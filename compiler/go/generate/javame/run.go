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

package javame

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// Options are the javame:... generator options. THRIFT_REGISTER_GENERATOR
// registers no options for javame ("" as its third argument), and the
// constructor throws "unknown option javame:<key>" for anything it is
// handed, so Options has no fields.
type Options struct{}

// ParseOptions parses the part after "javame:" of a --gen argument. The
// C++ constructor's loop over parsed_options unconditionally throws on
// the first entry, since there is nothing it recognizes.
func ParseOptions(spec string) (Options, error) {
	var o Options
	for _, option := range strings.Split(spec, ",") {
		if option == "" {
			continue
		}
		key := option
		if i := strings.IndexByte(option, '='); i >= 0 {
			key = option[:i]
		}
		return o, &emit.Error{Msg: "unknown option javame:" + key}
	}
	return o, nil
}

// The option table of THRIFT_REGISTER_GENERATOR(javame, "Java ME", "").
func init() {
	generate.Register(generate.Info{
		Name:     "javame",
		LongName: "Java ME",
		Options:  nil,
		Parse: func(spec string) (generate.Runner, error) {
			opts, err := ParseOptions(spec)
			if err != nil {
				return nil, err
			}
			return runner{opts}, nil
		},
	})
}

// runner is the registry's view of one parsed --gen javame argument.
type runner struct{ opts Options }

func (r runner) Run(program *sema.Program, recurse bool) error {
	return Run(program, r.opts, recurse)
}

// Run generates the program and, when recurse is set, every program it
// includes first, each inheriting the output path. It is generate() in
// main.cc restricted to the Java ME generator.
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
