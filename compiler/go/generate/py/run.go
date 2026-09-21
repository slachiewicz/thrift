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

package py

import (
	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/sema"
)

// The option table of THRIFT_REGISTER_GENERATOR(py, ...).
func init() {
	generate.Register(generate.Info{
		Name:     "py",
		LongName: "Python",
		Options: []generate.Option{
			{Name: "zope.interface", Help: "Generate code for use with zope.interface."},
			{Name: "twisted", Help: "Generate Twisted-friendly RPC services."},
			{Name: "tornado", Help: "Generate code for use with Tornado."},
			{Name: "no_utf8strings", Help: "Do not Encode/decode strings using utf8 in the generated code. Basically no effect for Python 3."},
			{Name: "coding", Value: "CODING", Help: "Add file encoding declare in generated file."},
			{Name: "slots", Help: "Generate code using slots for instance members."},
			{Name: "dynamic", Help: "Generate dynamic code, less code generated but slower."},
			{Name: "dynbase", Value: "CLS", Help: "Derive generated classes from class CLS instead of TBase."},
			{Name: "dynfrozen", Value: "CLS", Help: "Derive generated immutable classes from class CLS instead of TFrozenBase."},
			{Name: "dynexc", Value: "CLS", Help: "Derive generated exceptions from CLS instead of TExceptionBase."},
			{Name: "dynfrozenexc", Value: "CLS", Help: "Derive generated immutable exceptions from CLS instead of TFrozenExceptionBase."},
			{Name: "dynimport", Value: "'from foo.bar import CLS'", Help: "Add an import line to generated code to find the dynbase class."},
			{Name: "package_prefix", Value: "'top.package.'", Help: "Package prefix for generated files."},
			{Name: "old_style", Help: "Deprecated. Generate old-style classes."},
			{Name: "enum", Help: "Generates Python's IntEnum, connects thrift to python enums. Python 3.4 and higher."},
			{Name: "type_hints", Help: "Generate type hints and type checks in write method. Requires the enum option."},
		},
		Parse: func(spec string) (generate.Runner, error) {
			opts, err := ParseOptions(spec)
			if err != nil {
				return nil, err
			}
			// option_string in the C++ constructor is the full "--gen"
			// argument, i.e. always "py:" followed by the spec, which is
			// how both the CLI (cmd/thrift-go/generate.go) and the parity
			// tests build the string passed to generate.New.
			copyOptions := "py:" + spec
			return runner{opts, copyOptions}, nil
		},
	})
}

// runner is the registry's view of one parsed --gen py argument.
type runner struct {
	opts        Options
	copyOptions string
}

func (r runner) Run(program *sema.Program, recurse bool) error {
	return Run(program, r.opts, r.copyOptions, recurse)
}

// Run generates the program and, when recurse is set, every program it
// includes first, each inheriting the output path. It is generate() in
// main.cc restricted to the Python generator.
func Run(program *sema.Program, opts Options, copyOptions string, recurse bool) error {
	if recurse {
		program.SetRecursive(true)
		for _, inc := range program.Includes() {
			inc.SetOutPath(program.OutPath(), program.IsOutPathAbsolute())
			if err := Run(inc, opts, copyOptions, recurse); err != nil {
				return err
			}
		}
	}
	return generateOne(program, opts, copyOptions)
}

func generateOne(program *sema.Program, opts Options, copyOptions string) (err error) {
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
	return New(program, opts, copyOptions).Generate()
}
