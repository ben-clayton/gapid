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

import (
	"context"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/google/gapid/core/codegen"
	"github.com/google/gapid/core/log"
	"github.com/google/gapid/core/os/device"
	"github.com/google/gapid/gapil/compiler"
	"github.com/google/gapid/gapil/semantic"
	"github.com/google/gapid/gapis/api"
)

// Executor is used to create execution environments for a compiled program.
// Use New() or For() to create Executors, do not create directly.
type Executor struct {
	program          *compiler.Program
	exec             *codegen.Executor
	createContext    unsafe.Pointer
	destroyContext   unsafe.Pointer
	globalsSize      uint64
	globalsAPIOffset map[api.API]uintptr
	cmdFunctions     map[string]unsafe.Pointer
}

var cache sync.Map

type apiExec struct {
	exec  *Executor
	ready chan struct{}
}

// Config is a configuration for an executor.
type Config struct {
	CaptureABI *device.ABI
	Execute    bool
	Plugins    []compiler.Plugin
}

// NewEnv returns a new environment for an executor with the given config.
func NewEnv(ctx context.Context, abi *device.ABI, cfg Config) *Env {
	key := fmt.Sprintf("%v", cfg)
	obj, existing := cache.LoadOrStore(key, &apiExec{ready: make(chan struct{})})
	ae := obj.(*apiExec)
	if !existing {
		apis := api.All()
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
			CaptureABI:  abi,
			EmitContext: true,
			EmitExec:    cfg.Execute,
			Plugins:     cfg.Plugins,
		}

		log.I(ctx, "Compiling APIs with given settings: %+v", settings)
		start := time.Now()
		defer func() { log.I(ctx, "Compile finished in %v", time.Since(start)) }()

		prog, err := compiler.Compile(sems, mappings, settings)
		if err != nil {
			panic(err)
		}
		ae.exec = NewExecutor(prog, true)
		close(ae.ready)
	} else {
		<-ae.ready
	}
	return ae.exec.NewEnv(ctx)
}

// NewExecutor returns a new and initialized Executor for the given program.
func NewExecutor(prog *compiler.Program, optimize bool) *Executor {
	e, err := prog.Module.Executor(optimize)
	if err != nil {
		panic(err)
	}

	if prog.CreateContext == nil || prog.DestroyContext == nil {
		panic("Program has no context functions. Was EmitContext not set to true?")
	}

	// Gather all the API state offsets from the globals base pointer.
	apiOffsets := map[api.API]uintptr{}
	fieldOffsets := e.FieldOffsets(prog.Globals.Type)
	for _, api := range api.All() {
		name := api.Definition().Semantic.Name()
		if idx := prog.Globals.Type.FieldIndex(name); idx >= 0 {
			apiOffsets[api] = uintptr(fieldOffsets[idx])
		}
	}

	exec := &Executor{
		program:          prog,
		exec:             e,
		createContext:    e.FunctionAddress(prog.CreateContext),
		destroyContext:   e.FunctionAddress(prog.DestroyContext),
		globalsSize:      uint64(e.SizeOf(prog.Globals.Type)),
		globalsAPIOffset: apiOffsets,
		cmdFunctions:     map[string]unsafe.Pointer{},
	}

	for name, info := range prog.Commands {
		exec.cmdFunctions[name] = e.FunctionAddress(info.Execute)
	}

	return exec
}

// FunctionAddress returns the function address of the function with the given
// name or nil if the function was not found.
func (e *Executor) FunctionAddress(name string) unsafe.Pointer {
	f, ok := e.program.Functions[name]
	if !ok {
		return nil
	}
	return e.exec.FunctionAddress(f)
}
