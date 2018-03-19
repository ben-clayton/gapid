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

package test

import (
	"context"

	"github.com/google/gapid/core/image"
	"github.com/google/gapid/gapil/constset"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/replay/builder"
)

type CustomState struct{}

// ConstantSets returns the constant set pack for the API.
func (API) ConstantSets() *constset.Pack { return nil }

// GetFramebufferAttachmentInfo returns the width, height, and format of the
// specified framebuffer attachment.
// It also returns an API specific index that maps the given attachment into
// an API specific representation.
func (API) GetFramebufferAttachmentInfo(
	ctx context.Context,
	after []uint64,
	state *api.GlobalState,
	thread uint64,
	attachment api.FramebufferAttachment) (width, height, index uint32, format *image.Format, err error) {
	return 0, 0, 0, nil, nil
}

// Context returns the active context for the given state.
func (API) Context(state *api.GlobalState, thread uint64) api.Context {
	return nil
}

func (State) InitializeCustomState() {}

func (*Foo) Mutate(context.Context, api.CmdID, *api.GlobalState, *builder.Builder) error { return nil }
