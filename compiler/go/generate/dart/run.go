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

// Package dart is a port of t_dart_generator.cc. Every function keeps the
// name and the emission order of its C++ original, so that the output is
// byte-identical; the parity test in internal/parity holds it to that. The
// generator writes a package tree: a pubspec.yaml, a library file that
// exports every generated class, and one file per enum, struct, exception,
// constants holder and service, under lib/src/ (or under the library
// directory itself when library_prefix is given).
package dart

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// Options are the dart:... generator options, t_dart_generator's
// library_name_, library_prefix_, package_prefix_ and pubspec_lib_
// constructor fields.
type Options struct {
	// LibraryName overrides the library name; the default is the
	// program's "dart" namespace, or its name if that is unset.
	LibraryName string
	// LibraryPrefix generates code usable within an existing library,
	// e.g. "my_parent_lib.src.gen". Given, no pubspec.yaml is written.
	// Like library_prefix_, this already carries the trailing "." (so
	// its zero value, "", matches library_prefix_.empty() exactly,
	// including the edge case that a bare "library_prefix=" with an
	// empty value still sets it to "." and is thus non-empty).
	LibraryPrefix string
	// PackagePrefix is package_prefix_: LibraryPrefix with every "."
	// turned into "/", derived at the same time as LibraryPrefix.
	PackagePrefix string
	// PubspecLib overrides the thrift dependency stanza in pubspec.yaml,
	// pipe-separated lines, e.g. "thrift:|  git:|    url: git@foo.com".
	PubspecLib string
}

// ParseOptions parses the part after "dart:" of a --gen argument, with the
// same splitting rules as t_generator::parse_options: options are comma
// separated and a value follows the first "="; a key repeated in the spec
// keeps its last value, as t_generator::parse_options's map[key]=value
// does. It is also the t_dart_generator constructor's option loop, which
// derives LibraryPrefix and PackagePrefix as it goes.
func ParseOptions(spec string) (Options, error) {
	var o Options
	if spec == "" {
		return o, nil
	}
	for _, option := range strings.Split(spec, ",") {
		key := option
		value := ""
		if i := strings.IndexByte(option, '='); i >= 0 {
			key, value = option[:i], option[i+1:]
		}
		switch key {
		case "":
		case "library_name":
			o.LibraryName = value
		case "library_prefix":
			o.LibraryPrefix = value + "."
			o.PackagePrefix = replaceAll(o.LibraryPrefix, ".", "/")
		case "pubspec_lib":
			o.PubspecLib = value
		default:
			return o, &emit.Error{Msg: "unknown option dart:" + key}
		}
	}
	return o, nil
}

func init() {
	generate.Register(generate.Info{
		Name:     "dart",
		LongName: "Dart",
		Options: []generate.Option{
			{Name: "library_name", Value: "NAME", Help: "Optional override for library name."},
			{Name: "library_prefix", Value: "NAME", Help: "Generate code that can be used within an existing library.\nUse a dot-separated string, e.g. \"my_parent_lib.src.gen\""},
			{Name: "pubspec_lib", Value: "STR", Help: "Optional override for thrift lib dependency in pubspec.yaml,\ne.g. \"thrift: 0.x.x\".  Use a pipe delimiter to separate lines,\ne.g. \"thrift:|  git:|    url: git@foo.com\""},
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
// includes first, each inheriting the output path. It is generate() in
// main.cc restricted to the Dart generator.
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
			if e, ok := r.(*emit.Error); ok {
				err = e
				return
			}
			if e, ok := r.(*sema.Error); ok {
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
