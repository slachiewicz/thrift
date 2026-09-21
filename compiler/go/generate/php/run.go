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

// Package php is a port of t_php_generator.cc. Every function keeps the
// name and the emission order of its C++ original, so that the output is
// byte-identical; the parity test in internal/parity holds it to that.
package php

import (
	"sort"
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// Options are the parsed "--gen php:..." options.
type Options struct {
	// Inlined is binary_inline_: generate protocol-independent template
	// code (false) or binary-inline code (true).
	Inlined bool
	// Rest is rest_: generate a REST handler class.
	Rest bool
	// Server is phps_: generate stubs for a PHP server.
	Server bool
	// Oop is oop_: use the OOP base class TBase.
	Oop bool
	// Classmap is classmap_: generate old-style PHP files for classmap
	// autoloading, instead of one file per class under psr4.
	Classmap bool
	// Validate is validate_: generate validator methods.
	Validate bool
	// JSON is json_serializable_: generate JsonSerializable classes.
	JSON bool
	// NSGlobal is nsglobal_: the global namespace for PHP 5.3.
	NSGlobal string
	// GettersSetters is getters_setters_: generate getters and setters.
	GettersSetters bool
}

// ParseOptions is the t_php_generator constructor's option loop. The C++
// compiler iterates a std::map, so options apply in sorted order.
func ParseOptions(spec string) (Options, error) {
	var o Options
	parsed := map[string]string{}
	if spec != "" {
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
	}
	keys := make([]string, 0, len(parsed))
	for k := range parsed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := parsed[key]
		switch key {
		case "inlined":
			o.Inlined = true
		case "rest":
			o.Rest = true
		case "server":
			o.Server = true
		case "oop":
			o.Oop = true
		case "validate":
			o.Validate = true
		case "json":
			o.JSON = true
		case "nsglobal":
			o.NSGlobal = value
		case "classmap":
			o.Classmap = true
		case "psr4":
			// psr4 is the default output layout; the C++ generator
			// only warns that the option is unnecessary, unless
			// classmap was also given, which is a hard error.
			if o.Classmap {
				return o, &emit.Error{Msg: "psr4 and classmap are mutually exclusive."}
			}
		case "getters_setters":
			o.GettersSetters = true
		default:
			return o, &emit.Error{Msg: "unknown option php:" + key}
		}
	}
	if o.Oop && o.Inlined {
		return o, &emit.Error{Msg: "oop and inlined are mutually exclusive."}
	}
	return o, nil
}

// OutDirBase is out_dir_base_: the gen-* directory used without -out.
func (o Options) OutDirBase() string {
	if o.Inlined {
		return "gen-phpi"
	}
	return "gen-php"
}

// The option table of THRIFT_REGISTER_GENERATOR(php, ...).
func init() {
	generate.Register(generate.Info{
		Name:     "php",
		LongName: "PHP",
		Options: []generate.Option{
			{Name: "inlined", Help: "Generate PHP inlined files"},
			{Name: "server", Help: "Generate PHP server stubs"},
			{Name: "oop", Help: "Generate PHP with object oriented subclasses"},
			{Name: "classmap", Help: "Generate old-style PHP files (use classmap autoloading)"},
			{Name: "rest", Help: "Generate PHP REST processors"},
			{Name: "nsglobal", Value: "NAME", Help: "Set global namespace"},
			{Name: "validate", Help: "Generate PHP validator methods"},
			{Name: "json", Help: "Generate JsonSerializable classes (requires PHP >= 5.4)"},
			{Name: "getters_setters", Help: "Generate Getters and Setters for struct variables"},
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

// runner is the registry's view of one parsed --gen php argument.
type runner struct{ opts Options }

func (r runner) Run(program *sema.Program, recurse bool) error {
	return Run(program, r.opts, recurse)
}

// Run generates the program and, when recurse is set, every program it
// includes first, each inheriting the output path. It is generate() in
// main.cc restricted to the PHP generator.
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
	if err := validatePHPIdentifiers(program); err != nil {
		return err
	}
	return New(program, opts).Generate()
}
