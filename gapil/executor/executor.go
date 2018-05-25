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

// Package executor provides an interface for executing compiled API programs.
package executor

//#include "gapil/runtime/cc/runtime.h"
import "C"

import (
	"context"
	"fmt"
	"os"
	"sync"
	"unsafe"

	"github.com/google/gapid/core/app/status"
	"github.com/google/gapid/core/os/device"
	"github.com/google/gapid/gapil/compiler"
	"github.com/google/gapid/gapil/compiler/plugins/replay"
	"github.com/google/gapid/gapil/semantic"
	"github.com/google/gapid/gapis/api"
)

//#include "gapil/runtime/cc/runtime.h"
import "C"

// Executor is used to create execution environments for a compiled program.
// Use New() or For() to create Executors, do not create directly.
type Executor struct {
	cfg     Config
	module  *C.gapil_module
	symbols map[string]unsafe.Pointer
}

var cache sync.Map

type apiExec struct {
	exec  *Executor
	ready chan struct{}
}

// Config is a configuration for an executor.
type Config struct {
	CaptureABI *device.ABI
	ReplayABI  *device.ABI
	Execute    bool
	Optimize   bool

	// APIs to compile for. If empty, then all registered APIs will be compiled.
	APIs []api.API
}

func (c Config) key() string {
	key := fmt.Sprintf("capture: %+v replay: %+v exec: %v opt: %v",
		c.CaptureABI,
		c.ReplayABI,
		c.Execute,
		c.Optimize)
	fmt.Fprintln(os.Stderr, key)
	return key
}

// NewEnv returns a new environment for an executor with the given config.
func NewEnv(ctx context.Context, cfg Config) *Env {
	ctx = status.Start(ctx, "NewEnv")
	defer status.Finish(ctx)

	obj, existing := cache.LoadOrStore(cfg.key(), &apiExec{ready: make(chan struct{})})
	ae := obj.(*apiExec)
	if !existing {
		ae.exec = Compile(ctx, cfg)
		close(ae.ready)
	} else {
		<-ae.ready
	}
	return ae.exec.NewEnv(ctx)
}

// Compile returns a new and initialized Executor for the given config.
func Compile(ctx context.Context, cfg Config) *Executor {
	ctx = status.Start(ctx, "executor.Compile")
	defer status.Finish(ctx)

	apis := cfg.APIs
	if len(apis) == 0 {
		apis = api.All()
	}

	sems := make([]*semantic.API, len(apis))
	mappings := &semantic.Mappings{}
	for i, api := range apis {
		def := api.Definition()
		if def.Semantic == nil {
			panic(fmt.Errorf("API %v has no semantic definition", api.Name()))
		}
		sems[i] = def.Semantic
		if def.Mappings != nil {
			mappings.MergeIn(def.Mappings)
		}
	}

	settings := compiler.Settings{
		CaptureABI:  cfg.CaptureABI,
		EmitContext: true,
		EmitExec:    cfg.Execute,
	}

	if cfg.ReplayABI != nil {
		settings.Plugins = append(settings.Plugins, replay.Plugin(cfg.ReplayABI.MemoryLayout))
	}

	prog, err := compiler.Compile(sems, mappings, settings)
	if err != nil {
		panic(err)
	}

	e, err := prog.Codegen.Executor(cfg.Optimize)
	if err != nil {
		panic(err)
	}

	module := e.GlobalAddress(prog.Module)

	return New(ctx, cfg, module)
}

// New returns a new and initialized Executor for the given program.
func New(ctx context.Context, cfg Config, module unsafe.Pointer) *Executor {
	ctx = status.Start(ctx, "executor.New")
	defer status.Finish(ctx)

	m := (*C.gapil_module)(module)

	if m.create_context == nil || m.destroy_context == nil {
		panic(fmt.Errorf("Program has no context functions. Was EmitContext not set to true?\nmodule: %+v", m))
	}

	exec := &Executor{
		cfg:     cfg,
		module:  m,
		symbols: map[string]unsafe.Pointer{},
	}

	symbols := (*[65536]C.gapil_symbol)(unsafe.Pointer(m.symbols))[:m.num_symbols]
	for _, s := range symbols {
		exec.symbols[C.GoString(s.name)] = s.addr
	}

	return exec
}

// Symbol returns the address of the symnol with the given name or nil if the
// symbol was not found.
func (e *Executor) Symbol(name string) unsafe.Pointer {
	return e.symbols[name]
}
