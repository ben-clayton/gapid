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

	"github.com/google/gapid/core/data/slice"
	"github.com/google/gapid/core/math/u64"
	"github.com/google/gapid/core/memory/arena"
	"github.com/google/gapid/gapil/compiler"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/database"
	"github.com/google/gapid/gapis/memory"
)

// #include "gapil/runtime/cc/runtime.h"
//
// #include <string.h>
//
// typedef context* (TCreateContext) (arena*);
// typedef void     (TDestroyContext) (context*);
// typedef uint32_t (TFunc) (void* ctx);
//
// static context* create_context(TCreateContext* func, arena* a) { return func(a); }
// static void destroy_context(TDestroyContext* func, context* ctx) { func(ctx); }
// static uint32_t call(context* ctx, TFunc* func) { return func(ctx); }
//
// // Implemented below.
// extern void apply_reads(context*);
// extern void apply_writes(context*);
// extern void* resolve_pool_data(context*, uint64_t, uint64_t, gapil_data_access, uint64_t);
// extern void copy_slice(context* ctx, slice* dst, slice* src);
// extern void store_in_database(context*, void*, uint64_t, uint8_t*);
// extern uint64_t make_pool(context*, uint64_t);
// extern void pool_reference(context*, uint64_t);
// extern void pool_release(context*, uint64_t);
//
// static void set_callbacks() {
//   gapil_runtime_callbacks callbacks = {
//     .apply_reads       = &apply_reads,
//     .apply_writes      = &apply_writes,
//     .resolve_pool_data = &resolve_pool_data,
//     .copy_slice        = &copy_slice,
//     .store_in_database = &store_in_database,
//     .make_pool         = &make_pool,
//     .pool_reference    = &pool_reference,
//     .pool_release      = &pool_release,
//   };
//   gapil_set_runtime_callbacks(&callbacks);
// }
import "C"

func init() {
	// Setup the gapil runtime environment.
	C.set_callbacks()
}

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

	id    envID
	cCtx  *C.context      // The gapil C context.
	goCtx context.Context // The go context.
	cmd   api.Cmd         // The currently executing command.
}

// Dispose releases the memory used by the environment.
// Call after the env is no longer needed to avoid leaking memory.
func (e *Env) Dispose() {
	C.destroy_context((*C.TDestroyContext)(e.Executor.destroyContext), e.cCtx)
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
	env.goCtx = ctx
	env.cCtx = C.create_context((*C.TCreateContext)(e.createContext), (*C.arena)(env.Arena.Pointer))
	env.cCtx.id = (C.uint32_t)(id)
	env.goCtx = nil

	// Prime the state objects.
	if env.cCtx.globals != nil {
		globalsBase := uintptr(unsafe.Pointer(env.cCtx.globals))
		for api, offset := range e.globalsAPIOffset {
			addr := globalsBase + offset
			env.State.APIs[api.ID()] = api.State(env.Arena, unsafe.Pointer(addr))
		}
	}

	return env
}

// Execute executes the command cmd.
func (e *Env) Execute(ctx context.Context, cmd api.Cmd, id api.CmdID) error {
	name := cmd.CmdName()
	fptr, ok := e.Executor.cmdFunctions[name]
	if !ok {
		return fmt.Errorf("Program has no command '%v'", name)
	}

	e.cmd = cmd
	e.cCtx.thread = (C.uint64_t)(cmd.Thread())
	e.cCtx.cmd_id = (C.uint64_t)(id)
	res := e.call(ctx, fptr, cmd.ExecData())
	e.cmd = nil

	return res
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
	return e.cmd
}

// CmdID returns the currently executing command identifer.
func (e *Env) CmdID() api.CmdID {
	return api.CmdID(e.cCtx.cmd_id)
}

// Globals returns the memory of the global state.
func (e *Env) Globals() []byte {
	return slice.Bytes((unsafe.Pointer)(e.cCtx.globals), e.Executor.globalsSize)
}

func (e *Env) call(ctx context.Context, fptr, args unsafe.Pointer) error {
	e.goCtx = ctx
	e.cCtx.arguments = args
	err := compiler.ErrorCode(C.call(e.cCtx, (*C.TFunc)(fptr)))
	e.goCtx = nil

	return err.Err()
}

//export apply_reads
func apply_reads(c *C.context) {
	e := env(c)
	if extras := e.cmd.Extras(); extras != nil {
		if o := extras.Observations(); o != nil {
			o.ApplyReads(e.State.Memory.ApplicationPool())
		}
	}
}

//export apply_writes
func apply_writes(c *C.context) {
	e := env(c)
	if extras := e.cmd.Extras(); extras != nil {
		if o := extras.Observations(); o != nil {
			o.ApplyWrites(e.State.Memory.ApplicationPool())
		}
	}
}

//export resolve_pool_data
func resolve_pool_data(c *C.context, pool C.uint64_t, ptr C.uint64_t, access C.gapil_data_access, size C.uint64_t) unsafe.Pointer {
	env := EnvFromNative((unsafe.Pointer)(c))
	ctx := env.goCtx
	p := env.State.Memory.MustGet(memory.PoolID(pool))
	switch access {
	case C.GAPIL_READ:
		buf := env.bufferArena.Allocate(int(size), 1) // TODO: Free these!
		C.memset(buf, 0, C.size_t(size))
		rng := memory.Range{Base: uint64(ptr), Size: uint64(size)}
		sli := p.Slice(rng)
		if err := sli.Get(ctx, 0, slice.Bytes(buf, uint64(size))); err != nil {
			panic(err)
		}
		return buf
	case C.GAPIL_WRITE:
		buf := env.bufferArena.Allocate(int(size), 1) // TODO: Free these!
		C.memset(buf, 0, C.size_t(size))
		blob := memory.Blob(slice.Bytes(buf, uint64(size)))
		p.Write(uint64(ptr), blob)
		return buf
	default:
		panic(fmt.Errorf("Unexpected access: %v", access))
	}
}

//export copy_slice
func copy_slice(c *C.context, dst, src *C.slice) {
	env := EnvFromNative((unsafe.Pointer)(c))
	pDst := env.State.Memory.MustGet(memory.PoolID(dst.pool))
	pSrc := env.State.Memory.MustGet(memory.PoolID(src.pool))
	size := u64.Min(uint64(dst.size), uint64(src.size))
	pDst.Write(uint64(dst.base), pSrc.Slice(memory.Range{Base: uint64(src.base), Size: size}))
}

//export store_in_database
func store_in_database(c *C.context, ptr unsafe.Pointer, size C.uint64_t, idOut *C.uint8_t) {
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

//export make_pool
func make_pool(c *C.context, size C.uint64_t) C.uint64_t {
	env := EnvFromNative((unsafe.Pointer)(c))
	id, _ := env.State.Memory.New()
	return C.uint64_t(id)
}

//export pool_reference
func pool_reference(c *C.context, pool C.uint64_t) {
	// TODO: Refcounting
}

//export pool_release
func pool_release(c *C.context, pool C.uint64_t) {
	// TODO: Refcounting
}
