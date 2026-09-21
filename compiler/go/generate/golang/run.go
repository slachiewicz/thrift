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

package golang

import (
	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/sema"
)

// The option table of THRIFT_REGISTER_GENERATOR(go, ...).
func init() {
	generate.Register(generate.Info{
		Name:     "go",
		LongName: "Go",
		Options: []generate.Option{
			{Name: "package_prefix", Value: "prefix", Help: "Package prefix for generated files."},
			{Name: "thrift_import", Value: "path", Help: "Override thrift package import path (default:" + DefaultThriftImport + ")"},
			{Name: "package", Value: "name", Help: "Package name (default: inferred from thrift file name)"},
			{Name: "ignore_initialisms", Help: "Disable automatic spelling correction of initialisms (e.g. \"URL\")"},
			{Name: "read_write_private", Help: "Make read/write methods private, default is public Read/Write"},
			{Name: "skip_remote", Help: "Skip the generating of -remote folders for the client binaries for services"},
			{Name: "struct_key_entries", Help: "Generate maps keyed by a struct, union or exception as []thrift.MapEntry[*K, V] instead of map[*K]V"},
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

// runner is the registry's view of one parsed --gen go argument.
type runner struct{ opts Options }

func (r runner) Run(program *sema.Program, recurse bool) error {
	return Run(program, r.opts, recurse)
}

// Run generates the program and, when recurse is set, every program it
// includes first, each inheriting the output path. It is generate() in
// main.cc restricted to the Go generator.
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
			panic(r)
		}
	}()
	program.Scope.ResolveAllConsts()
	sema.ValidateInput(program)
	return New(program, opts).Generate()
}
