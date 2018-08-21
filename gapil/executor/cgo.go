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
	"unsafe"
)

// #include "cgo.h"
// #include "env.h"
import "C"

func (e *Env) call(cmds *C.cmd_data, count C.uint64_t, res *C.uint64_t) {
	ctx := e.cCtx
	m := e.Executor.module
	C.call(ctx, m, cmds, count, res)
}

func callbacks() *C.callbacks {
	return &C.callbacks{
		apply_reads:       C.applyReads,
		apply_writes:      C.applyWrites,
		resolve_pool_data: C.resolvePoolData,
		call_extern:       C.callExtern,
		copy_slice:        C.copySlice,
		cstring_to_slice:  C.cstringToSlice,
		store_in_database: C.storeInDatabase,
		make_pool:         C.makePool,
		pool_reference:    C.poolReference,
		pool_release:      C.poolRelease,
	}
}

//export applyReads
func applyReads(c *C.context) {
	env(c).applyReads()
}

//export applyWrites
func applyWrites(c *C.context) {
	env(c).applyWrites()
}

//export resolvePoolData
func resolvePoolData(c *C.context, pool C.uint64_t, ptr C.uint64_t, access C.gapil_data_access, size C.uint64_t) unsafe.Pointer {
	return env(c).resolvePoolData(pool, ptr, access, size)
}

//export copySlice
func copySlice(c *C.context, dst, src *C.slice) {
	env(c).copySlice(dst, src)
}

//export cstringToSlice
func cstringToSlice(c *C.context, ptr C.uint64_t, out *C.slice) {
	env(c).cstringToSlice(ptr, out)
}

//export storeInDatabase
func storeInDatabase(c *C.context, ptr unsafe.Pointer, size C.uint64_t, idOut *C.uint8_t) {
	env(c).storeInDatabase(ptr, size, idOut)
}

//export makePool
func makePool(c *C.context, size C.uint64_t) C.uint64_t {
	return env(c).makePool(size)
}

//export poolReference
func poolReference(c *C.context, pool C.uint64_t) {
	env(c).poolReference(pool)
}

//export poolRelease
func poolRelease(c *C.context, pool C.uint64_t) {
	env(c).poolRelease(pool)
}

//export callExtern
func callExtern(c *C.context, name *C.uint8_t, args, res unsafe.Pointer) {
	env(c).callExtern(name, args, res)
}
