// Copyright (C) 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package executor

import (
	"context"
	"fmt"
	"sync"
	"unsafe"

	"github.com/google/gapid/core/app/status"
	"github.com/google/gapid/core/data/slice"
	"github.com/google/gapid/core/math/u64"
	"github.com/google/gapid/core/memory/arena"
	"github.com/google/gapid/gapil/compiler"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/database"
	"github.com/google/gapid/gapis/memory"
)

// #include "env.h"
//
// #include <string.h> // memset
// #include <stdlib.h> // free
import "C"

// Env is the go execution environment.
type Env struct {
	// Arena is the memory arena used by this execution environment.
	Arena arena.Arena // TODO: Remove - already stored as State.Arena.

	// Executor is the parent executor.
	Executor *Executor

	// State is the global state for the environment.
	State *api.GlobalState

	// Arena to use for buffers
	bufferArena arena.Arena
	buffers     []unsafe.Pointer
	lastCmdID   api.CmdID

	id    envID
	cCtx  *C.context      // The gapil C context.
	goCtx context.Context // The go context.
	cmds  []api.Cmd       // The currently executing commands.
}

// Dispose releases the memory used by the environment.
// Call after the env is no longer needed to avoid leaking memory.
func (e *Env) Dispose() {
	C.destroy_context(e.Executor.module, e.cCtx)
	e.bufferArena.Dispose()
	e.Arena.Dispose()
}

type envID uint32

var (
	envMutex  sync.RWMutex
	nextEnvID envID
	envs      = map[envID]*Env{}
)

// env returns the environment for the given context c.
func env(c *C.context) *Env {
	return envFromID(envID(c.id))
}

// envFromID returns the environment for the given envID.
func envFromID(id envID) *Env {
	envMutex.RLock()
	out, ok := envs[id]
	envMutex.RUnlock()
	if !ok {
		panic(fmt.Errorf("Unknown envID %v", id))
	}
	return out
}

// EnvFromNative returns the environment for the given context c.
func EnvFromNative(c unsafe.Pointer) *Env {
	return env((*C.context)(c))
}

// NewEnv creates a new execution environment for the given capture.
func (e *Executor) NewEnv(ctx context.Context) *Env {
	var id envID
	var env *Env

	func() {
		envMutex.Lock()
		defer envMutex.Unlock()

		id = nextEnvID
		nextEnvID++

		env = &Env{
			Executor: e,
			id:       envID(id),
		}
		envs[id] = env
	}()

	env.Arena = arena.New()
	env.State = &api.GlobalState{
		Arena:  env.Arena,
		APIs:   map[api.ID]api.State{},
		Memory: memory.NewPools(),
	}
	env.bufferArena = arena.New()

	// Create the context and initialize the globals.
	status.Do(ctx, "Create Context", func(ctx context.Context) {
		env.goCtx = ctx
		env.cCtx = C.create_context(e.module, (*C.arena)(env.Arena.Pointer))
		env.cCtx.id = (C.uint32_t)(id)
		env.goCtx = nil
	})

	// Prime the state objects.
	if env.cCtx.globals != nil {
		globalsBase := uintptr(unsafe.Pointer(env.cCtx.globals))
		for _, api := range api.All() {
			if m := C.get_api_module(e.module, C.uint32_t(api.Index())); m != nil {
				addr := uintptr(m.globals_offset) + globalsBase
				env.State.APIs[api.ID()] = api.State(env.Arena, unsafe.Pointer(addr))
			}
		}
	}

	return env
}

// Execute executes the all the commands in l.
func (e *Env) Execute(ctx context.Context, cmdID api.CmdID, cmd api.Cmd) error {
	return e.ExecuteN(ctx, cmdID, []api.Cmd{cmd})[0]
}

// ExecuteN executes the all the commands in cmds.
func (e *Env) ExecuteN(ctx context.Context, firstID api.CmdID, cmds []api.Cmd) []error {
	ctx = status.Start(ctx, "Execute<%v>", len(cmds))
	defer status.Finish(ctx)

	dataBuf := e.Arena.Allocate(len(cmds)*int(unsafe.Sizeof(C.cmd_data{})), int(unsafe.Alignof(C.cmd_data{})))
	defer e.Arena.Free(dataBuf)

	data := (*(*[1 << 40]C.cmd_data)(dataBuf))[:len(cmds)]
	for i, cmd := range cmds {
		flags := C.uint64_t(0)
		if extras := cmd.Extras(); extras != nil {
			if o := extras.Observations(); o != nil {
				if len(o.Reads) > 0 {
					flags |= C.CMD_FLAGS_HAS_READS
				}
				if len(o.Writes) > 0 {
					flags |= C.CMD_FLAGS_HAS_WRITES
				}
			}
		}
		data[i] = C.cmd_data{
			api_idx: C.uint32_t(cmd.API().Index()),
			cmd_idx: C.uint32_t(cmd.CmdIndex()),
			args:    cmd.ExecData(),
			id:      C.uint64_t(firstID) + C.uint64_t(i),
			flags:   flags,
			thread:  C.uint64_t(cmd.Thread()),
		}
	}

	res := make([]C.uint64_t, len(cmds))

	e.cmds = cmds
	e.goCtx = ctx

	call(
		e.cCtx,
		e.Executor.module,
		&data[0],
		C.uint64_t(len(cmds)),
		&res[0],
	)

	e.goCtx = nil
	e.cmds = nil

	out := make([]error, len(cmds))
	for i, r := range res {
		out[i] = compiler.ErrorCode(r).Err()
	}
	return out
}

// CContext returns the pointer to the c context.
func (e *Env) CContext() unsafe.Pointer {
	return (unsafe.Pointer)(e.cCtx)
}

// Context returns the go context of the environment.
func (e *Env) Context() context.Context {
	return e.goCtx
}

// Cmd returns the currently executing command.
func (e *Env) Cmd() api.Cmd {
	return e.cmds[e.cCtx.cmd_idx]
}

// CmdID returns the currently executing command identifer.
func (e *Env) CmdID() api.CmdID {
	return api.CmdID(e.cCtx.cmd_id)
}

// Globals returns the memory of the global state.
func (e *Env) Globals() []byte {
	return slice.Bytes((unsafe.Pointer)(e.cCtx.globals), uint64(e.Executor.module.globals_size))
}

func (e *Env) changedCommand() bool {
	cur := api.CmdID(e.cCtx.cmd_id)
	changed := cur != e.lastCmdID
	e.lastCmdID = cur
	return changed
}

func (e *Env) readPoolData(pool memory.PoolID, ptr, size uint64) unsafe.Pointer {
	if e.changedCommand() {
		for _, b := range e.buffers {
			e.bufferArena.Free(b)
		}
		e.buffers = e.buffers[:0]
	}

	ctx := e.goCtx
	p := e.State.Memory.MustGet(pool)

	rng := memory.Range{Base: ptr, Size: size}
	sli := p.Slice(rng)

	switch sli := sli.(type) {
	case *memory.Native:
		return sli.Data()
	default:
		buf := e.bufferArena.Allocate(int(size), 1) // TODO: Free these!
		C.memset(buf, 0, C.size_t(size))            // TODO: Fix Get() to zero gaps
		if err := sli.Get(ctx, 0, slice.Bytes(buf, size)); err != nil {
			panic(err)
		}
		e.buffers = append(e.buffers, buf)
		return buf
	}
}

func (e *Env) writePoolData(pool memory.PoolID, ptr, size uint64) unsafe.Pointer {
	native := memory.NewNative(e.bufferArena, size)
	p := e.State.Memory.MustGet(pool)
	p.Write(ptr, native)
	return native.Data()
}

func applyReads(c *C.context) {
	e := env(c)
	if extras := e.Cmd().Extras(); extras != nil {
		if o := extras.Observations(); o != nil {
			o.ApplyReads(e.State.Memory.ApplicationPool())
		}
	}
}

func applyWrites(c *C.context) {
	e := env(c)
	if extras := e.Cmd().Extras(); extras != nil {
		if o := extras.Observations(); o != nil {
			o.ApplyWrites(e.State.Memory.ApplicationPool())
		}
	}
}

func resolvePoolData(c *C.context, pool C.uint64_t, ptr C.uint64_t, access C.gapil_data_access, size C.uint64_t) unsafe.Pointer {
	env := EnvFromNative((unsafe.Pointer)(c))
	switch access {
	case C.GAPIL_READ:
		return env.readPoolData(memory.PoolID(pool), uint64(ptr), uint64(size))
	case C.GAPIL_WRITE:
		return env.writePoolData(memory.PoolID(pool), uint64(ptr), uint64(size))
	default:
		panic(fmt.Errorf("Unexpected access: %v", access))
	}
}

func copySlice(c *C.context, dst, src *C.slice) {
	env := EnvFromNative((unsafe.Pointer)(c))
	pDst := env.State.Memory.MustGet(memory.PoolID(dst.pool))
	pSrc := env.State.Memory.MustGet(memory.PoolID(src.pool))
	size := u64.Min(uint64(dst.size), uint64(src.size))
	pDst.Write(uint64(dst.base), pSrc.Slice(memory.Range{Base: uint64(src.base), Size: size}))
}

func cstringToSlice(c *C.context, ptr C.uint64_t, out *C.slice) {
	env := EnvFromNative((unsafe.Pointer)(c))
	pool := env.State.Memory.ApplicationPool()
	size, err := pool.Strlen(env.goCtx, uint64(ptr))
	if err != nil {
		panic(err)
	}

	size++ // Include null terminator

	out.pool = C.uint64_t(memory.ApplicationPool)
	out.root = C.uint64_t(ptr)
	out.base = C.uint64_t(ptr)
	out.size = C.uint64_t(size)
	out.count = C.uint64_t(size)
}

func storeInDatabase(c *C.context, ptr unsafe.Pointer, size C.uint64_t, idOut *C.uint8_t) {
	env := EnvFromNative((unsafe.Pointer)(c))
	ctx := env.Context()
	sli := slice.Bytes(ptr, uint64(size))
	id, err := database.Store(ctx, sli)
	if err != nil {
		panic(err)
	}
	out := slice.Bytes((unsafe.Pointer)(idOut), 20)
	copy(out, id[:])
}

func makePool(c *C.context, size C.uint64_t) C.uint64_t {
	env := EnvFromNative((unsafe.Pointer)(c))
	id, _ := env.State.Memory.New()
	return C.uint64_t(id)
}

func poolReference(c *C.context, pool C.uint64_t) {
	// TODO: Refcounting
}

func poolRelease(c *C.context, pool C.uint64_t) {
	// TODO: Refcounting
}

func callExtern(c *C.context, name *C.uint8_t, args, res unsafe.Pointer) {
	env := EnvFromNative((unsafe.Pointer)(c))
	n := C.GoString((*C.char)((unsafe.Pointer)(name)))
	f, ok := externs[n]
	if !ok {
		panic(fmt.Sprintf("No handler for extern '%v'", n))
	}
	f(env, args, res)
}

func init() {
	C.set_callbacks(callbacks())
}

func registerCExtern(name string, e unsafe.Pointer) {
	n := C.CString(name)
	C.register_c_extern(n, (*C.gapil_extern)(e))
	C.free(unsafe.Pointer(n))
}
