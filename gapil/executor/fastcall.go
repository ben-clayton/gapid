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
	"reflect"
	"unsafe"
)

// #include "cgo.h"
// #include "env.h"
import "C"

const fastcallEnabled = false

func (e *Env) call(cmds *C.cmd_data, count C.uint64_t, res *C.uint64_t) {
	ctx := e.cCtx
	m := e.Executor.module

	if fastcallEnabled {
		ctx.stack = e.cStackHigh
		fastcallC((unsafe.Pointer)(C.call), ctx, m, cmds, count, res)
	} else {
		C.call(ctx, m, cmds, count, res)
	}
}

func callbacks() *C.callbacks {
	if fastcallEnabled {
		return fastcallCallbacks()
	}
	return cgoCallbacks()
}

func fastcallCallbacks() *C.callbacks {
	return &C.callbacks{
		apply_reads:       funcPtr(applyReadsFC),
		apply_writes:      funcPtr(applyWritesFC),
		resolve_pool_data: funcPtr(resolvePoolDataFC),
		call_extern:       funcPtr(callExternFC),
		copy_slice:        funcPtr(copySliceFC),
		cstring_to_slice:  funcPtr(cstringToSliceFC),
		store_in_database: funcPtr(storeInDatabaseFC),
		make_pool:         funcPtr(makePoolFC),
		pool_reference:    funcPtr(poolReferenceFC),
		pool_release:      funcPtr(poolReleaseFC),
	}
}

func cgoCallbacks() *C.callbacks {
	return &C.callbacks{
		apply_reads:       C.applyReadsCgo,
		apply_writes:      C.applyWritesCgo,
		resolve_pool_data: C.resolvePoolDataCgo,
		call_extern:       C.callExternCgo,
		copy_slice:        C.copySliceCgo,
		cstring_to_slice:  C.cstringToSliceCgo,
		store_in_database: C.storeInDatabaseCgo,
		make_pool:         C.makePoolCgo,
		pool_reference:    C.poolReferenceCgo,
		pool_release:      C.poolReleaseCgo,
	}
}

// funcPtr returns the function address of f.
func funcPtr(f interface{}) unsafe.Pointer {
	return unsafe.Pointer(reflect.ValueOf(f).Pointer())
}

func fastcallC(pfn unsafe.Pointer, ctx *C.context, mod *C.gapil_module, cmds *C.cmd_data, cnt C.uint64_t, res *C.uint64_t)

func applyReadsFC()
func applyWritesFC()
func resolvePoolDataFC()
func copySliceFC()
func cstringToSliceFC()
func storeInDatabaseFC()
func makePoolFC()
func poolReferenceFC()
func poolReleaseFC()
func callExternFC()

//export applyReadsCgo
func applyReadsCgo(c *C.context) { applyReads(c) }

//export applyWritesCgo
func applyWritesCgo(c *C.context) { applyWrites(c) }

//export resolvePoolDataCgo
func resolvePoolDataCgo(c *C.context, pool C.uint64_t, ptr C.uint64_t, access C.gapil_data_access, size C.uint64_t) unsafe.Pointer {
	return resolvePoolData(c, pool, ptr, access, size)
}

//export copySliceCgo
func copySliceCgo(c *C.context, dst, src *C.slice) { copySlice(c, dst, src) }

//export cstringToSliceCgo
func cstringToSliceCgo(c *C.context, ptr C.uint64_t, out *C.slice) { cstringToSlice(c, ptr, out) }

//export storeInDatabaseCgo
func storeInDatabaseCgo(c *C.context, ptr unsafe.Pointer, size C.uint64_t, idOut *C.uint8_t) {
	storeInDatabase(c, ptr, size, idOut)
}

//export makePoolCgo
func makePoolCgo(c *C.context, size C.uint64_t) C.uint64_t { return makePool(c, size) }

//export poolReferenceCgo
func poolReferenceCgo(c *C.context, pool C.uint64_t) { poolReference(c, pool) }

//export poolReleaseCgo
func poolReleaseCgo(c *C.context, pool C.uint64_t) { poolRelease(c, pool) }

//export callExternCgo
func callExternCgo(c *C.context, name *C.uint8_t, args, res unsafe.Pointer) {
	callExtern(c, name, args, res)
}
