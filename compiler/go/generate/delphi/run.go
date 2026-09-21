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

package delphi

import (
	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/sema"
)

// The option table of THRIFT_REGISTER_GENERATOR(delphi, ...).
func init() {
	generate.Register(generate.Info{
		Name:     "delphi",
		LongName: "Delphi",
		Options: []generate.Option{
			{Name: "register_types", Help: "Enable TypeRegistry, allows for creation of struct, union\nand container instances by interface or TypeInfo()"},
			{Name: "constprefix", Help: "Name TConstants classes after IDL to reduce ambiguities"},
			{Name: "events", Help: "Enable and use processing events in the generated code."},
			{Name: "xmldoc", Help: "Enable XMLDoc comments for Help Insight etc."},
			{Name: "async", Help: "Generate IAsync interface to use Parallel Programming Library (XE7+ only)."},
			{Name: "com_types", Help: "Use COM-compatible data types (e.g. WideString)."},
			{Name: "old_names", Help: "Compatibility: generate \"reserved\" identifiers with '_' postfix instead of '&' prefix."},
			{Name: "rtti", Help: "Activate {$TYPEINFO} and {$RTTI} at the generated API interfaces."},
			{Name: "guid_v4", Help: "Use random UUIDv4 (Windows only, legacy). Default: stable UUIDv8."},
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

// runner is the registry's view of one parsed --gen delphi argument.
type runner struct{ opts Options }

func (r runner) Run(program *sema.Program, recurse bool) error {
	return Run(program, r.opts, recurse)
}

// Run generates the program and, when recurse is set, every program it
// includes first, each inheriting the output path.
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
