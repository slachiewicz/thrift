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

// Package rs is t_rs_generator.cc: it renders a resolved program as the
// Rust source file the C++ compiler's --gen rs writes, byte for byte. It
// writes a single "<program>.rs" file, using an indent-tracked writer the
// way t_rs_generator uses its ofstream and indent()/indent_up()/
// indent_down(), rather than the split multi-file, gofmt-through layout
// the golang package uses: the C++ generator disables rustfmt on its
// output, so nothing reformats the file afterwards and every space here
// has to be the one the C++ code writes.
package rs

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// Options are the rs:... generator options.
type Options struct {
	// CratePrefix is the Rust path prefix for cross-file imports
	// (default: "crate"). Use "super" when generated files are in a
	// submodule.
	CratePrefix string
}

// ParseOptions parses the part after "rs:" of a --gen argument, with the
// same splitting rules as t_generator::parse_options: options are comma
// separated and a value follows the first "=".
func ParseOptions(spec string) (Options, error) {
	o := Options{CratePrefix: "crate"}
	for _, option := range strings.Split(spec, ",") {
		key := option
		value := ""
		if i := strings.IndexByte(option, '='); i >= 0 {
			key, value = option[:i], option[i+1:]
		}
		switch key {
		case "":
		case "crate_prefix":
			if value == "" {
				return o, &emit.Error{Msg: "crate_prefix requires a non-empty value, e.g. rs:crate_prefix=super"}
			}
			o.CratePrefix = value
		default:
			return o, &emit.Error{Msg: "unknown option rs:" + key}
		}
	}
	return o, nil
}

func init() {
	generate.Register(generate.Info{
		Name:     "rs",
		LongName: "Rust",
		Options: []generate.Option{
			{Name: "crate_prefix", Value: "p", Help: "Rust path prefix for cross-file imports (default: crate).\nUse 'super' when generated files are in a submodule."},
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
			if e, ok := r.(*emit.Error); ok {
				err = e
				return
			}
			panic(r)
		}
	}()
	program.Scope.ResolveAllConsts()
	sema.ValidateInput(program)
	return newGenerator(program, opts).generate()
}
