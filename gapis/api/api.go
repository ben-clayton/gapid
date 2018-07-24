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

package api

import (
	"context"
	"fmt"
	"sort"
	"unsafe"

	"github.com/google/gapid/core/data/id"
	"github.com/google/gapid/core/image"
	"github.com/google/gapid/core/memory/arena"
	"github.com/google/gapid/gapil/constset"
	"github.com/google/gapid/gapil/semantic"
)

// API is the common interface to a graphics programming api.
type API interface {
	// Definition returns the API's semantic definition.
	Definition() Definition

	// State returns a Go state object for the native state at p.
	State(a arena.Arena, p unsafe.Pointer) State

	// Name returns the official name of the api.
	Name() string

	// Index returns the API index.
	Index() uint8

	// ID returns the unique API identifier.
	ID() ID

	// ConstantSets returns the constant set pack for the API.
	ConstantSets() *constset.Pack

	// GetFramebufferAttachmentInfo returns the width, height, and format of the
	// specified framebuffer attachment.
	// It also returns an API specific index that maps the given attachment into
	// an API specific representation.
	GetFramebufferAttachmentInfo(
		ctx context.Context,
		after []uint64,
		state *GlobalState,
		thread uint64,
		attachment FramebufferAttachment) (info FramebufferAttachmentInfo, err error)

	// Context returns the active context for the given state.
	Context(state *GlobalState, thread uint64) Context

	// CreateCmd constructs and returns a new command with the specified name.
	CreateCmd(a arena.Arena, name string) Cmd
}

// Definition holds the data from the semantic tree of the API definition.
type Definition struct {
	// Semantic is the API's semantic tree.
	Semantic *semantic.API
	// Mappings are the API's mappings between the semantic, abstract and
	// concrete trees.
	Mappings *semantic.Mappings
}

// FramebufferAttachmentInfo describes a framebuffer at a given point in the trace
type FramebufferAttachmentInfo struct {
	// Width in texels of the framebuffer
	Width uint32
	// Height in texels of the framebuffer
	Height uint32
	// Framebuffer index
	Index uint32
	// Format of the image
	Format *image.Format
	// CanResize is true if this can be efficiently resized during replay.
	CanResize bool
}

// ID is an API identifier
type ID id.ID

// IsValid returns true if the id is not the default zero value.
func (i ID) IsValid() bool  { return id.ID(i).IsValid() }
func (i ID) String() string { return id.ID(i).String() }

// APIObject is the interface implemented by types that belong to an API.
type APIObject interface {
	// API returns the API identifier that this type belongs to.
	API() API
}

var apis = map[ID]API{}
var indices = map[uint8]API{}

// Register adds an api to the understood set.
// It is illegal to register the same name twice.
func Register(api API) {
	id := api.ID()
	if existing, present := apis[id]; present {
		panic(fmt.Errorf("API %s registered more than once. First: %T, Second: %T", id, existing, api))
	}
	apis[id] = api

	index := api.Index()
	if existing, present := indices[index]; present {
		panic(fmt.Errorf("API %s used an occupied index %d. First: %T, Second: %T", id, index, existing, api))
	}
	indices[index] = api
}

// Find looks up a graphics API by identifier.
// If the id has not been registered, it returns nil.
func Find(id ID) API {
	return apis[id]
}

// All returns all the registered APIs.
func All() []API {
	out := make([]API, 0, len(apis))
	for _, api := range apis {
		out = append(out, api)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index() < out[j].Index() })
	return out
}
