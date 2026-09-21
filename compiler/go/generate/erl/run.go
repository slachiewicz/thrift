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

package erl

import (
	"sort"
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// StringTo is the string=... option: how the string() type is rendered and
// how a string constant is emitted.
type StringTo int

// StringToBoth is the default: string() | binary(), and a plain "quoted"
// constant.
const (
	StringToBoth StringTo = iota
	StringToString
	StringToBinary
)

// SetsTo is the set=... option: which sets implementation default value
// expressions use.
type SetsTo int

// SetsToV1 is the default.
const (
	SetsToV1 SetsTo = iota
	SetsToV2
)

// TypeDeclaration is the type=... option: which Erlang declaration keyword
// -type/-nominal generated type specs use.
type TypeDeclaration int

// TypeDeclType is the default.
const (
	TypeDeclType TypeDeclaration = iota
	TypeDeclNominal
)

// Options are the erl:... generator options.
type Options struct {
	// LegacyNames retains the naming conventions of Thrift 0.9.1 and
	// earlier (legacynames).
	LegacyNames bool
	// Maps generates maps instead of dicts (maps).
	Maps bool
	// Delimiter separates a namespace prefix from a record name
	// (delimiter=). Default is ".".
	Delimiter string
	// AppPrefix is prepended to generated module and record names
	// (app_prefix=).
	AppPrefix string
	// StringTo is the string=... option.
	StringTo StringTo
	// SetsTo is the set=... option.
	SetsTo SetsTo
	// TypeDeclaration is the type=... option.
	TypeDeclaration TypeDeclaration
}

// ParseOptions parses the part after "erl:" of a --gen argument, with the
// same splitting rules as the t_erl_generator constructor's option loop: the
// C++ compiler stores options in a std::map, so options apply in sorted
// order (which does not matter here, since every option is independent).
func ParseOptions(spec string) (Options, error) {
	o := Options{Delimiter: "."}
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
		case "legacynames":
			o.LegacyNames = true
		case "maps":
			o.Maps = true
		case "delimiter":
			o.Delimiter = value
		case "app_prefix":
			o.AppPrefix = value
		case "string":
			switch value {
			case "string":
				o.StringTo = StringToString
			case "binary":
				o.StringTo = StringToBinary
			case "both":
				o.StringTo = StringToBoth
			default:
				return o, &emit.Error{Msg: "unknown string option value:" + value}
			}
		case "set":
			switch value {
			case "v1":
				o.SetsTo = SetsToV1
			case "v2":
				o.SetsTo = SetsToV2
			default:
				return o, &emit.Error{Msg: "unknown set option value:" + value}
			}
		case "type":
			switch value {
			case "type":
				o.TypeDeclaration = TypeDeclType
			case "nominal":
				o.TypeDeclaration = TypeDeclNominal
			default:
				return o, &emit.Error{Msg: "unknown type option value:" + value}
			}
		default:
			return o, &emit.Error{Msg: "unknown option erl:" + key}
		}
	}
	return o, nil
}

// The option table of THRIFT_REGISTER_GENERATOR(erl, ...).
func init() {
	generate.Register(generate.Info{
		Name:     "erl",
		LongName: "Erlang",
		Options: []generate.Option{
			{Name: "legacynames", Help: "Output files retain naming conventions of Thrift 0.9.1 and earlier."},
			{Name: "delimiter", Value: "delimiter", Help: "Delimiter between namespace prefix and record name. Default is '.'."},
			{Name: "app_prefix", Value: "prefix", Help: "Application prefix for generated Erlang files."},
			{Name: "maps", Help: "Generate maps instead of dicts."},
			{Name: "string", Value: "[string|binary|both]", Help: "Define string as 'string', 'binary' or 'both'. Default is 'both'."},
			{Name: "set", Value: "[v1|v2]", Help: "Define sets implementation, supported 'v1' and 'v2'. Default is 'v1'."},
			{Name: "type", Value: "[type|nominal]", Help: "Define type declaration, supported 'type' and 'nominal'. Default is 'type'."},
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

// runner is the registry's view of one parsed --gen erl argument.
type runner struct{ opts Options }

func (r runner) Run(program *sema.Program, recurse bool) error {
	return Run(program, r.opts, recurse)
}

// Run generates the program and, when recurse is set, every program it
// includes first, each inheriting the output path. It is generate() in
// main.cc restricted to the Erlang generator.
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
