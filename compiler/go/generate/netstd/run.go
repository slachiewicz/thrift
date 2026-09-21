/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements. See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership. The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package netstd

import (
	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// The option table of THRIFT_REGISTER_GENERATOR(netstd, ...).
func init() {
	generate.Register(generate.Info{
		Name:     "netstd",
		LongName: "C#",
		Options: []generate.Option{
			{Name: "wcf", Help: "Adds bindings for WCF to generated classes."},
			{Name: "serial", Help: "Add serialization support to generated classes."},
			{Name: "union", Help: "Use new union typing, which includes a static read function for union types."},
			{Name: "pascal", Help: "Generate Pascal Case property names according to Microsoft naming convention."},
			{Name: "net8", Help: "Enable features that require net8 and C# 12 or higher."},
			{Name: "net9", Help: "Enable features that require net9 and C# 13 or higher."},
			{Name: "net10", Help: "Enable features that require net10 and C# 14 or higher."},
			{Name: "no_deepcopy", Help: "Suppress generation of " + deepCopyMethodName + "() method."},
			{Name: "async_postfix", Help: "Append \"Async\" to all service methods (maintains compatibility with existing code)."},
		},
		Parse: func(spec string) (generate.Runner, error) {
			opts, err := ParseOptions(spec)
			if err != nil {
				return nil, err
			}
			return runner{opts}, nil
		},
	})
}

// runner is the registry's view of one parsed --gen netstd argument.
type runner struct{ opts Options }

func (r runner) Run(program *sema.Program, recurse bool) error {
	return Run(program, r.opts, recurse)
}

// Run generates the program and, when recurse is set, every program it
// includes first, each inheriting the output path. It is generate() in
// main.cc restricted to the netstd generator.
func Run(program *sema.Program, opts Options, recurse bool) error {
	if recurse {
		program.SetRecursive(true)
		for _, inc := range program.Includes() {
			inc.SetOutPath(program.OutPath(), program.IsOutPathAbsolute())
			if err := Run(inc, opts, recurse); err != nil {
				return err
			}
		}
	}
	return generateOne(program, opts)
}

func generateOne(program *sema.Program, opts Options) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*sema.Error); ok {
				err = e
				return
			}
			if e, ok := r.(*emit.Error); ok {
				err = e
				return
			}
			panic(r)
		}
	}()
	program.Scope.ResolveAllConsts()
	sema.ValidateInput(program)
	return New(program, opts).Generate()
}
