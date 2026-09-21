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

// Package haxe is t_haxe_generator.cc: it renders a resolved program as the
// Haxe source files the C++ compiler's --gen haxe writes, byte for byte. It
// writes one ".hx" file per type and per service file (interface, callback
// interface, client, processor), using an indent-tracked writer the way
// t_haxe_generator uses its ofstream and indent()/indent_up()/indent_down(),
// rather than the split-file, gofmt-through layout the golang package uses:
// nothing reformats Haxe output, so every space here has to be the one the
// C++ code writes.
package haxe

import (
	"fmt"
	"sort"
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// Options are the haxe:... generator options.
type Options struct {
	// RTTI enables @:rtti on generated classes and interfaces.
	RTTI bool
	// BuildMacro, when non-empty, adds @:build/@:autoBuild macro calls to
	// generated classes and interfaces.
	BuildMacro string
}

// ParseOptions parses the part after "haxe:" of a --gen argument. Options
// are comma separated and a value follows the first "="; a key repeated
// keeps its last value and unknown keys are reported in sorted order, as
// t_generator::parse_options collects them into a std::map before the
// generator constructor's option loop runs.
func ParseOptions(spec string) (Options, error) {
	var o Options
	parsed := map[string]string{}
	for _, option := range strings.Split(spec, ",") {
		if option == "" {
			continue
		}
		key, value := option, ""
		if i := strings.IndexByte(option, '='); i >= 0 {
			key, value = option[:i], option[i+1:]
		}
		parsed[key] = value
	}
	keys := make([]string, 0, len(parsed))
	for k := range parsed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := parsed[key]
		switch key {
		case "callbacks":
			fmt.Println("Hint: The 'callbacks' option is no longer necessary.")
		case "rtti":
			o.RTTI = true
		case "buildmacro":
			o.BuildMacro = value
		default:
			return o, &emit.Error{Msg: "unknown option haxe:" + key}
		}
	}
	return o, nil
}

func init() {
	generate.Register(generate.Info{
		Name:     "haxe",
		LongName: "Haxe",
		Options: []generate.Option{
			{Name: "rtti", Help: "Enable @:rtti for generated classes and interfaces"},
			{Name: "buildmacro", Value: "my.macros.Class.method(args)", Help: "Add @:build macro calls to generated classes and interfaces"},
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
