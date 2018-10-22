// Copyright (C) 2018 Google Inc.
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
	"reflect"
	"unsafe"

	"github.com/google/gapid/core/data/id"
	"github.com/google/gapid/core/data/slice"
	"github.com/google/gapid/core/os/device"
	"github.com/google/gapid/gapil/compiler/plugins/replay"
	gapir "github.com/google/gapid/gapir/client"
	replaysrv "github.com/google/gapid/gapir/replay_service"
	"github.com/google/gapid/gapis/memory"
	"github.com/google/gapid/gapis/replay/builder"
	"github.com/google/gapid/gapis/replay/protocol"
	"github.com/google/gapid/gapis/replay/value"
)

// #include "gapil/runtime/cc/replay/replay.h"
//
// typedef gapil_replay_data* (TGetReplayData) (gapil_context*);
// gapil_replay_data* get_replay_data(TGetReplayData* func, gapil_context* ctx) { return func(ctx); }
import "C"

// ReplayBuilder returns a replay builder.
func (e *Env) ReplayBuilder() builder.Builder {
	return &replayBuilder{e}
}

type replayBuilder struct {
	e *Env
}

func (b *replayBuilder) MemoryLayout() *device.MemoryLayout {
	panic("MemoryLayout not implemented")
}
func (b *replayBuilder) AllocateMemory(size uint64) value.Pointer {
	panic("AllocateMemory not implemented")
}
func (b *replayBuilder) AllocateTemporaryMemory(size uint64) value.Pointer {
	panic("AllocateTemporaryMemory not implemented")
}
func (b *replayBuilder) BeginCommand(cmdID, threadID uint64) {
	panic("BeginCommand not implemented")
}
func (b *replayBuilder) CommitCommand() {
	panic("CommitCommand not implemented")
}
func (b *replayBuilder) RevertCommand(err error) {
	panic("RevertCommand not implemented")
}
func (b *replayBuilder) Buffer(count int) value.Pointer {
	panic("Buffer not implemented")
}
func (b *replayBuilder) String(s string) value.Pointer {
	panic("String not implemented")
}
func (b *replayBuilder) Call(f builder.FunctionInfo) {
	panic("Call not implemented")
}
func (b *replayBuilder) Copy(size uint64) {
	panic("Copy not implemented")
}
func (b *replayBuilder) Clone(index int) {
	panic("Clone not implemented")
}
func (b *replayBuilder) Load(ty protocol.Type, addr value.Pointer) {
	panic("Load not implemented")
}
func (b *replayBuilder) Store(addr value.Pointer) {
	panic("Store not implemented")
}
func (b *replayBuilder) StorePointer(idx value.PointerIndex, ptr value.Pointer) {
	panic("StorePointer not implemented")
}
func (b *replayBuilder) Strcpy(maxCount uint64) {
	panic("Strcpy not implemented")
}
func (b *replayBuilder) Post(addr value.Pointer, size uint64, p builder.Postback) {
	panic("Post not implemented")
}
func (b *replayBuilder) Push(val value.Value) {
	panic("Push not implemented")
}
func (b *replayBuilder) Pop(count uint32) {
	panic("Pop not implemented")
}
func (b *replayBuilder) ReserveMemory(rng memory.Range) {
	panic("ReserveMemory not implemented")
}
func (b *replayBuilder) MapMemory(rng memory.Range) {
	panic("MapMemory not implemented")
}
func (b *replayBuilder) UnmapMemory(rng memory.Range) {
	panic("UnmapMemory not implemented")
}
func (b *replayBuilder) Write(rng memory.Range, resourceID id.ID) {
	panic("Write not implemented")
}
func (b *replayBuilder) Remappings() map[interface{}]value.Pointer {
	panic("Remappings not implemented")
}
func (b *replayBuilder) RegisterNotificationReader(reader builder.NotificationReader) {
	panic("RegisterNotificationReader not implemented")
}
func (b *replayBuilder) Export(ctx context.Context) (gapir.Payload, error) {
	panic("Export not implemented")
}
func (b *replayBuilder) Build(ctx context.Context) (gapir.Payload, builder.PostDataHandler, builder.NotificationHandler, error) {
	panic("Build not implemented")
}

// BuildReplay builds the replay payload for execution.
func (e *Env) BuildReplay(ctx context.Context) (replaysrv.Payload, error) {
	pfn := e.Executor.Symbol(replay.GetReplayData)
	if pfn == nil {
		return replaysrv.Payload{}, fmt.Errorf("Program did not export the function to get the replay opcodes")
	}

	gro := (*C.TGetReplayData)(pfn)
	c := (*C.gapil_context)(e.CContext())

	data := C.get_replay_data(gro, c)

	C.gapil_replay_build(c, data)

	resources := slice.Cast(
		slice.Bytes(unsafe.Pointer(data.resources.data), uint64(data.resources.size)),
		reflect.TypeOf([]C.gapil_replay_resource_info_t{})).([]C.gapil_replay_resource_info_t)

	payload := replaysrv.Payload{
		Opcodes:   slice.Bytes(unsafe.Pointer(data.stream.data), uint64(data.stream.size)),
		Resources: make([]*replaysrv.ResourceInfo, len(resources)),
		Constants: slice.Bytes(unsafe.Pointer(data.constants.data), uint64(data.constants.size)),
	}

	for i, r := range resources {
		id := slice.Bytes(unsafe.Pointer(&r.id[0]), 20)
		payload.Resources[i] = &replaysrv.ResourceInfo{
			Id:   string(id),
			Size: uint32(r.size),
		}
	}

	return payload, nil
}
