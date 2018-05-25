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

	"github.com/google/gapid/core/data/slice"
	"github.com/google/gapid/gapil/compiler/plugins/replay"
	replaysrv "github.com/google/gapid/gapir/replay_service"
)

// #include "gapil/runtime/cc/replay/replay.h"
//
// typedef gapil_replay_data* (TGetReplayData) (gapil_context*);
// gapil_replay_data* get_replay_data(TGetReplayData* func, gapil_context* ctx) { return func(ctx); }
import "C"

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
