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
	"context"
	"fmt"

	"github.com/google/gapid/core/data/protoconv"
	"github.com/google/gapid/gapis/memory/memory_pb"
)

// Nullptr is a zero-address pointer in the application pool.
var Nullptr = Pointer{Pool: ApplicationPool}

// Values smaller than this are not legal addresses.
const lowMem = uint64(1) << 16
const bits32 = uint64(1) << 32

// Pointer is the type representing a memory pointer.
type Pointer struct {
	Address uint64 // The memory address.
	Pool    PoolID // The memory pool.
}

// Offset returns the pointer offset by n bytes.
func (p Pointer) Offset(n uint64) Pointer {
	return Pointer{Address: p.Address + n, Pool: p.Pool}
}

// Range returns a Range of size s with the base of this pointer.
func (p Pointer) Range(s uint64) Range {
	return Range{Base: p.Address, Size: s}
}

func (p Pointer) String() string {
	if p.Pool == PoolID(0) {
		if p.Address < lowMem {
			return fmt.Sprint(p.Address)
		}
		if p.Address < bits32 {
			return fmt.Sprintf("0x%.8x", p.Address)
		}
		return fmt.Sprintf("0x%.16x", p.Address)
	}
	if p.Address < bits32 {
		return fmt.Sprintf("0x%.8x@%d", p.Address, p.Pool)
	}
	return fmt.Sprintf("0x%.16x@%d", p.Address, p.Pool)
}

func (p Pointer) ToProto() *memory_pb.Pointer {
	return &memory_pb.Pointer{
		Address: p.Address,
		Pool:    uint32(p.Pool),
	}
}

func PointerFrom(from *memory_pb.Pointer) Pointer {
	return Pointer{
		Address: from.Address,
		Pool:    PoolID(from.Pool),
	}
}

func init() {
	protoconv.Register(
		func(ctx context.Context, a Pointer) (*memory_pb.Pointer, error) {
			return a.ToProto(), nil
		},
		func(ctx context.Context, a *memory_pb.Pointer) (Pointer, error) {
			return PointerFrom(a), nil
		},
	)
}
