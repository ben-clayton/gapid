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

package builder

import (
	"testing"

	"github.com/google/gapid/core/assert"
	"github.com/google/gapid/core/data/binary"
	"github.com/google/gapid/core/fault"
	"github.com/google/gapid/core/log"
	"github.com/google/gapid/core/os/device"
	"github.com/google/gapid/gapis/memory"
	"github.com/google/gapid/gapis/replay/asm"
	"github.com/google/gapid/gapis/replay/protocol"
	"github.com/google/gapid/gapis/replay/value"
)

func TestCommitAtom(t *testing.T) {
	ctx := log.Testing(t)
	for _, test := range []struct {
		name     string
		f        func(*Builder)
		expected []asm.Instruction
	}{
		{
			"Call with used return value",
			func(b *Builder) {
				b.BeginAtom(10)
				b.Push(value.U8(1))
				b.Call(FunctionInfo{0, 123, protocol.Type_Uint8, 1})
				b.Store(value.AbsolutePointer(0x10000))
				b.CommitAtom()
			},
			[]asm.Instruction{
				asm.Label{Value: 10},
				asm.Push{Value: value.U8(1)},
				asm.Call{PushReturn: true, ApiIndex: 0, FunctionID: 123},
				asm.Store{Destination: value.AbsolutePointer(0x10000)},
			},
		},
		{
			"Call with unused return value",
			func(b *Builder) {
				b.BeginAtom(10)
				b.Push(value.U8(1))
				b.Call(FunctionInfo{1, 123, protocol.Type_Uint8, 1})
				b.CommitAtom()
			},
			[]asm.Instruction{
				asm.Label{Value: 10},
				asm.Push{Value: value.U8(1)},
				asm.Call{PushReturn: false, ApiIndex: 1, FunctionID: 123},
			},
		},
		{
			"Remove unused push",
			func(b *Builder) {
				b.BeginAtom(10)
				b.Push(value.U32(12))
				b.CommitAtom()
			},
			[]asm.Instruction{
				asm.Label{Value: 10},
			},
		},
		{
			"Unused pushes",
			func(b *Builder) {
				b.BeginAtom(10)
				b.Push(value.U32(12))
				b.Push(value.U32(34))
				b.Push(value.U32(56))
				b.CommitAtom()
			},
			[]asm.Instruction{
				asm.Label{Value: 10},
			},
		},
		{
			"Unused clone",
			func(b *Builder) {
				b.BeginAtom(10)
				b.Call(FunctionInfo{0, 123, protocol.Type_Uint8, 0})
				b.Clone(0)
				b.CommitAtom()
			},
			[]asm.Instruction{
				asm.Label{Value: 10},
				asm.Call{PushReturn: true, ApiIndex: 0, FunctionID: 123},
				asm.Pop{Count: 1},
			},
		},
		{
			"Unused clone",
			func(b *Builder) {
				b.BeginAtom(10)
				b.Call(FunctionInfo{1, 123, protocol.Type_Uint8, 0})
				b.Clone(0)
				b.Store(value.AbsolutePointer(0x10000))
				b.CommitAtom()
			},
			[]asm.Instruction{
				asm.Label{Value: 10},
				asm.Call{PushReturn: true, ApiIndex: 1, FunctionID: 123},
				asm.Nop{},
				asm.Store{Destination: value.AbsolutePointer(0x10000)},
			},
		},
		{
			"Unused clone of return value",
			func(b *Builder) {
				b.BeginAtom(10)
				b.Call(FunctionInfo{0, 123, protocol.Type_Uint8, 0})
				b.Clone(0)
				b.CommitAtom()
			},
			[]asm.Instruction{
				asm.Label{Value: 10},
				// TODO: Could be optimised further.
				asm.Call{PushReturn: true, ApiIndex: 0, FunctionID: 123},
				asm.Pop{Count: 1},
			},
		},
		{
			"Use one of three return values",
			func(b *Builder) {
				b.BeginAtom(10)
				b.Call(FunctionInfo{0, 123, protocol.Type_Uint8, 0})
				b.Call(FunctionInfo{0, 123, protocol.Type_Uint8, 0})
				b.Call(FunctionInfo{0, 123, protocol.Type_Uint8, 0})
				b.Clone(1)
				b.Store(value.AbsolutePointer(0x10000))
				b.CommitAtom()
			},
			[]asm.Instruction{
				asm.Label{Value: 10},
				asm.Call{PushReturn: false, ApiIndex: 0, FunctionID: 123},
				asm.Call{PushReturn: true, ApiIndex: 0, FunctionID: 123},
				asm.Call{PushReturn: false, ApiIndex: 0, FunctionID: 123},
				asm.Nop{},
				asm.Store{Destination: value.AbsolutePointer(0x10000)},
			},
		},
	} {
		b := New(device.Little32)
		test.f(b)
		assert.For(ctx, test.name).ThatSlice(b.instructions).Equals(test.expected)
	}
}

func TestRevertAtom(t *testing.T) {
	ctx := log.Testing(t)
	for _, test := range []struct {
		name     string
		f        func(*Builder)
		expected []asm.Instruction
	}{
		{
			"Revert atom",
			func(b *Builder) {
				b.BeginAtom(10)
				b.Push(value.U8(1))
				b.Call(FunctionInfo{1, 123, protocol.Type_Uint8, 1})
				b.Store(value.AbsolutePointer(0x10000))
				b.RevertAtom(nil)
			},
			[]asm.Instruction{},
		},
		{
			"Commit atom, revert atom",
			func(b *Builder) {
				b.BeginAtom(10)
				b.Push(value.U8(1))
				b.Call(FunctionInfo{1, 123, protocol.Type_Uint8, 1})
				b.Store(value.AbsolutePointer(0x10000))
				b.CommitAtom()
				b.BeginAtom(20)
				b.Push(value.U8(2))
				b.Call(FunctionInfo{1, 234, protocol.Type_Uint8, 1})
				b.Store(value.AbsolutePointer(0x10000))
				b.RevertAtom(nil)
			},
			[]asm.Instruction{
				asm.Label{Value: 10},
				asm.Push{Value: value.U8(1)},
				asm.Call{PushReturn: true, ApiIndex: 1, FunctionID: 123},
				asm.Store{Destination: value.AbsolutePointer(0x10000)},
			},
		},
	} {
		b := New(device.Little32)
		test.f(b)

		assert.For(ctx, test.name).ThatSlice(b.instructions).Equals(test.expected)
	}
}

func TestRevertPostbackAtom(t *testing.T) {
	ctx := log.Testing(t)
	const expectedErr = fault.Const("Oh noes!")
	postbackErr := error(nil)
	postback := Postback(func(r binary.Reader, err error) error {
		assert.For(ctx, "Postback reader").That(r).IsNil()
		postbackErr = err
		return nil
	})

	for _, test := range []struct {
		name     string
		f        func(*Builder)
		expected []asm.Instruction
	}{
		{
			"Revert postback atom",
			func(b *Builder) {
				b.BeginAtom(10)
				b.Post(value.AbsolutePointer(0x10000), 100, postback)
				b.RevertAtom(expectedErr)
			},
			[]asm.Instruction{},
		},
	} {
		ctx := log.Enter(ctx, test.name)
		b := New(device.Little32)
		test.f(b)
		assert.With(ctx).ThatSlice(b.instructions).Equals(test.expected)
	}
	assert.For(ctx, "Postback was not informed of RevertAtom").ThatError(postbackErr).Equals(expectedErr)
}

func TestMapMemory(t *testing.T) {
	ctx := log.Testing(t)
	for _, test := range []struct {
		name     string
		f        func(*Builder)
		expected []asm.Instruction
	}{
		{
			"No mapping",
			func(b *Builder) {
				b.BeginAtom(10)
				b.Push(value.ObservedPointer(0x100004))
				b.Call(FunctionInfo{0, 123, protocol.Type_VolatilePointer, 1})
				b.CommitAtom()
			},
			[]asm.Instruction{
				asm.Label{Value: 10},
				asm.Push{Value: value.ObservedPointer(0x100004)},
				asm.Call{ApiIndex: 0, FunctionID: 123},
			},
		},
		{
			"MapMemory",
			func(b *Builder) {
				b.BeginAtom(10)
				b.Call(FunctionInfo{0, 100, protocol.Type_AbsolutePointer, 0})
				b.MapMemory(memory.Range{Base: 0x100000, Size: 0x10})
				b.CommitAtom()

				b.BeginAtom(20)
				b.Push(value.ObservedPointer(0x100004))
				b.Call(FunctionInfo{0, 123, protocol.Type_Void, 1})
				b.CommitAtom()
			},
			[]asm.Instruction{
				asm.Label{Value: 10},
				asm.Call{ApiIndex: 0, FunctionID: 100, PushReturn: true},
				asm.Store{Destination: value.VolatilePointer(0x0)}, // mapping-storage address

				asm.Label{Value: 20},
				asm.Load{DataType: protocol.Type_AbsolutePointer, Source: value.VolatilePointer(0x0)}, // mapping-storage address
				asm.Push{Value: value.AbsolutePointer(0x4)},
				asm.Add{Count: 2},
				asm.Call{ApiIndex: 0, FunctionID: 123},
			},
		},
		{
			"UnmapMemory",
			func(b *Builder) {
				b.BeginAtom(10)
				b.Call(FunctionInfo{0, 100, protocol.Type_AbsolutePointer, 0})
				b.MapMemory(memory.Range{Base: 0x100000, Size: 0x10})
				b.CommitAtom()

				b.BeginAtom(20)
				b.UnmapMemory(memory.Range{Base: 0x100000, Size: 0x10})
				b.CommitAtom()

				b.BeginAtom(30)
				b.Push(value.ObservedPointer(0x100004))
				b.Call(FunctionInfo{0, 123, protocol.Type_Void, 1})
				b.CommitAtom()
			},
			[]asm.Instruction{
				asm.Label{Value: 10},
				asm.Call{ApiIndex: 0, FunctionID: 100, PushReturn: true},
				asm.Store{Destination: value.VolatilePointer(0x0)}, // mapping-storage address

				asm.Label{Value: 20},

				asm.Label{Value: 30},
				asm.Push{Value: value.ObservedPointer(0x100004)},
				asm.Call{ApiIndex: 0, FunctionID: 123},
			},
		},
	} {
		ctx := log.Enter(ctx, test.name)
		b := New(device.Little32)
		test.f(b)
		assert.With(ctx).ThatSlice(b.instructions).Equals(test.expected)
	}
}
