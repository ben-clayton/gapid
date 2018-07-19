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
	"unsafe"

	"github.com/google/gapid/core/codegen"
	"github.com/google/gapid/gapil/compiler"
	"github.com/google/gapid/gapil/semantic"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/capture"
)

// Executor is used to create execution environments for a compiled program.
// Use New() or For() to create Executors, do not create directly.
type Executor struct {
	program        *compiler.Program
	exec           *codegen.Executor
	createContext  unsafe.Pointer
	destroyContext unsafe.Pointer
	globalsSize    uint64
	cmdFunctions   map[string]unsafe.Pointer
}

var cache sync.Map

type apiExec struct {
	exec  *Executor
	ready chan struct{}
}

// Config is a configuration for an executor.
type Config struct{}

// NewEnv returns a new environment for an executor with the given config.
func NewEnv(ctx context.Context, capture *capture.Capture, cfg Config) *Env {
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
			EmitContext: true,
			EmitExec:    true,
		}
		prog, err := compiler.Compile(sems, mappings, settings)
		if err != nil {
			panic(err)
		}
		ae.exec = New(prog, true)
		close(ae.ready)
	} else {
		<-ae.ready
	}
	return ae.exec.NewEnv(ctx, capture)
}

// New returns a new and initialized Executor for the given program.
func New(prog *compiler.Program, optimize bool) *Executor {
	e, err := prog.Module.Executor(optimize)
	if err != nil {
		panic(err)
	}

	if prog.CreateContext == nil || prog.DestroyContext == nil {
		panic("Program has no context functions. Was EmitContext not set to true?")
	}

	exec := &Executor{
		program:        prog,
		exec:           e,
		createContext:  e.FunctionAddress(prog.CreateContext),
		destroyContext: e.FunctionAddress(prog.DestroyContext),
		globalsSize:    uint64(e.SizeOf(prog.Globals.Type)),
		cmdFunctions:   map[string]unsafe.Pointer{},
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
