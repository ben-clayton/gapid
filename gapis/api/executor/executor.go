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

package executor

import (
	"unsafe"

	"github.com/google/gapid/core/codegen"
	"github.com/google/gapid/gapil/compiler"
)

type Executor struct {
	program *compiler.Program
	// Externs      Externs
	exec         *codegen.Executor
	initFunction unsafe.Pointer
	cmdFunctions map[string]unsafe.Pointer
}

// New returns a new and initialized Executor for the given program.
func New(prog *compiler.Program, optimize bool) *Executor {
	e, err := prog.Module.Executor(optimize)
	if err != nil {
		panic(err)
	}

	exec := &Executor{
		program: prog,
		// Externs:      make(Externs, len(prog.Externs)),
		exec:         e,
		initFunction: e.FunctionAddress(prog.Initializer),
		cmdFunctions: map[string]unsafe.Pointer{},
	}

	// Register each of the program's externs as binding points.
	// for name := range prog.Externs {
	// 	exec.Externs[name] = &extern{layout: e.Layout.Externs[name]}
	// }

	for name, info := range prog.Commands {
		exec.cmdFunctions[name] = e.FunctionAddress(info.Function)
	}

	return exec
}