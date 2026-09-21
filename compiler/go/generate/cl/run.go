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

package cl

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// Options are the cl:... generator options.
type Options struct {
	// NoASD is no_asd: do not define ASDF systems for each generated
	// Thrift program.
	NoASD bool
	// SysPref is sys_pref=: the prefix given to ASDF system names.
	// Default: "thrift-gen-".
	SysPref string
}

// ParseOptions parses the part after "cl:" of a --gen argument, with the
// same splitting rules as the t_cl_generator constructor's option loop.
func ParseOptions(spec string) (Options, error) {
	o := Options{SysPref: "thrift-gen-"}
	for _, option := range strings.Split(spec, ",") {
		if option == "" {
			continue
		}
		key, value := option, ""
		if i := strings.IndexByte(option, '='); i >= 0 {
			key, value = option[:i], option[i+1:]
		}
		switch key {
		case "no_asd":
			o.NoASD = true
		case "sys_pref":
			o.SysPref = value
		default:
			return o, &emit.Error{Msg: "unknown option cl:" + key}
		}
	}
	return o, nil
}

// The option table of THRIFT_REGISTER_GENERATOR(cl, ...).
func init() {
	generate.Register(generate.Info{
		Name:     "cl",
		LongName: "Common Lisp",
		Options: []generate.Option{
			{Name: "no_asd", Help: "Do not define ASDF systems for each generated Thrift program."},
			{Name: "sys_pref", Value: "prefix", Help: "The prefix to give ASDF system names. Default: thrift-gen-"},
		},
		Parse: func(spec string) (generate.Runner, error) {
			opts, err := ParseOptions(spec)
			if err != nil {
				return nil, err
			}
			// copyOptions reconstructs the full "--gen" argument text
			// (main.cc's option_string / t_cl_generator's copy_options_),
			// which is what every caller in this codebase passes: the
			// registry only hands the Parse callback the part after the
			// first colon, always present (see generate.New).
			return runner{opts: opts, copyOptions: "cl:" + spec}, nil
		},
	})
}

// runner is the registry's view of one parsed --gen cl argument.
type runner struct {
	opts        Options
	copyOptions string
}

func (r runner) Run(program *sema.Program, recurse bool) error {
	return Run(program, r.opts, r.copyOptions, recurse)
}

// Run generates the program and, when recurse is set, every program it
// includes first, each inheriting the output path and the same
// copyOptions text, exactly as main.cc's generate() passes the same
// "--gen" spec string to a fresh generator for every recursively
// generated program.
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
			if e, ok := r.(*emit.Error); ok {
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
