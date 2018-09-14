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

package memory

import (
	"unsafe"

	"github.com/google/gapid/core/memory/arena"

	// #include "gapis/memory/cc/memory.h"
	"C"
)
import (
	"bytes"
	"context"
	"io"

	"github.com/google/gapid/core/data/id"
	"github.com/google/gapid/core/data/slice"
)

// PoolID is the identifier of a pool in a memory.
type PoolID uint64

// Pool represents an unbounded and isolated memory space. Pool can be used
// to represent the application address space, or hidden GPU Pool.
//
// Pool can be read or written to.
// All writes to Pool or its slices do not actually perform binary data
// copies, but instead all writes are stored as lightweight records.
type Pool struct {
	c  *C.memory
	id PoolID
}

// Write writes the data d to address ptr.
func (p Pool) Write(ptr uint64, d Data) {
	switch d := d.(type) {
	case *blob:
		size := len(d.data)
		bytes := unsafe.Pointer(&d.data[0])
		C.memory_write(p.c, C.pool_id(p.id), C.uint64_t(ptr), C.uint64_t(size), bytes)
	}
}

// Slice returns a slice of the pool p.
func (p Pool) Slice(r Range) Data {
	return poolSlice{p.c, p.id, r}
}

type poolSlice struct {
	c *C.memory
	p PoolID
	r Range
}

// Get writes the bytes representing the slice to out, starting at offset
// bytes. This is equivalent to: copy(out, data[offset:]).
func (s poolSlice) Get(ctx context.Context, offset uint64, out []byte) error {
	var freePtr C.GAPIL_BOOL
	ptr := C.memory_read(
		s.c,
		C.pool_id(s.p),
		C.uint64_t(s.r.Base+offset),
		C.uint64_t(s.r.Size-offset),
		&freePtr,
	)
	data := slice.Bytes(ptr, s.r.Size)
	copy(out, data)
	if freePtr == C.GAPIL_TRUE {
		// TODO: Free the pointer!
	}
	return nil
}

// NewReader returns an io.Reader to efficiently read from the slice.
// There shouldn't be a need to wrap this in additional buffers.
func (s poolSlice) NewReader(ctx context.Context) io.Reader {
	buf := make([]byte, s.r.Size)
	s.Get(ctx, 0, buf)
	return bytes.NewReader(buf)
}

// ResourceID returns the identifier of the resource representing the slice,
// creating a new resource if it isn't already backed by one.
func (s poolSlice) ResourceID(ctx context.Context) (id.ID, error) {
	panic("todo")
}

// Size returns the number of bytes that would be returned by calling Get.
func (s poolSlice) Size() uint64 {
	return s.r.Size
}

// Slice returns a new Data referencing a subset range of the data.
// The range r is relative to the base of the Slice. For example a slice of
// [0, 4] would return a Slice referencing the first 5 bytes of this Slice.
// Attempting to slice outside the range of this Slice will result in a
// panic.
func (s poolSlice) Slice(r Range) Data {
	s.r.Base += r.Base
	s.r.Size = r.Size
	return s
}

// ValidRanges returns the list of slice-relative memory ranges that contain
// valid (non-zero) data that can be read with Get.
func (s poolSlice) ValidRanges() RangeList {
	panic("todo")
}

// Strlen returns the number of bytes before the first zero byte in the
// data.
// If the Data does not contain a zero byte, then -1 is returned.
func (s poolSlice) Strlen(ctx context.Context) (int, error) {
	panic("todo")
}

// Memory holds a memory model.
type Memory struct {
	c *C.memory
}

// New returns a new memory using the provided allocator.
func New(a arena.Arena) *Memory {
	return &Memory{C.memory_create((*C.arena)(a.Pointer))}
}

// Dispose releases the memory object.
func (m *Memory) Dispose() {
	C.memory_destroy(m.c)
	m.c = nil
}

// NewPool creates and returns a new Pool in m.
func (m *Memory) NewPool() Pool {
	id := PoolID(C.memory_new_pool(m.c))
	return Pool{m.c, id}
}
