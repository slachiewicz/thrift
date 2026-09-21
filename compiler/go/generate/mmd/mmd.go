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

// Package mmd is t_mmd_generator.cc: it renders a resolved program as a
// Mermaid classDiagram, byte for byte with the C++ compiler's --gen mmd
// output. When invoked with -r, one .mmd file is produced per .thrift
// file, each containing only the types declared in that file.
package mmd

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// Options are the mmd:... generator options.
type Options struct {
	// Exceptions is "exceptions": draw dashed arrows from service
	// functions to their declared exceptions.
	Exceptions bool
}

// ParseOptions parses the part after "mmd:" of a --gen argument.
func ParseOptions(spec string) (Options, error) {
	var o Options
	for _, option := range strings.Split(spec, ",") {
		key := option
		if i := strings.IndexByte(option, '='); i >= 0 {
			key = option[:i]
		}
		switch key {
		case "":
		case "exceptions":
			o.Exceptions = true
		default:
			return o, &emit.Error{Msg: "unknown option mmd:" + key}
		}
	}
	return o, nil
}

func init() {
	generate.Register(generate.Info{
		Name:     "mmd",
		LongName: "Mermaid",
		Options: []generate.Option{
			{Name: "exceptions", Help: "Draw dashed arrows from service functions to their declared exceptions."},
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
	outDir := program.OutPath() + "gen-mmd/"
	if program.IsOutPathAbsolute() {
		outDir = program.OutPath() + "/"
	}
	emit.Mkdir(outDir)
	g := &generator{exceptionArrows: opts.Exceptions}
	g.generateProgram(program)
	emit.WriteFile(outDir+program.Name()+".mmd", g.sb.String())
	return nil
}

// generator is t_mmd_generator. It accumulates the class blocks in sb and
// the edges drawn between them in edges; close_generator's job of writing
// edges_ after every class block is done by generateProgram, once
// emitProgramTypes has filled both.
type generator struct {
	sb              strings.Builder
	edges           []string
	exceptionArrows bool
}

// generateProgram is t_mmd_generator::generate_program, with
// init_generator and close_generator inlined: it writes the diagram
// header, the class blocks, then the collected edges.
//
// Overriding the base t_generator::generate_program() routes exceptions
// through generateStruct (which renders the <<exception>> stereotype);
// the base class's generate_program calls generate_xception, which has
// no override here and would silently drop exceptions from the diagram.
func (g *generator) generateProgram(program *sema.Program) {
	g.sb.WriteString("classDiagram\n")
	g.sb.WriteString("  direction LR\n")
	g.emitProgramTypes(program)
	for _, edge := range g.edges {
		g.sb.WriteString(edge + "\n")
	}
}

// emitProgramTypes emits all diagram-relevant types for one program:
// enums, typedefs, structs/unions/exceptions (in declaration order), and
// services. Constants are intentionally omitted.
func (g *generator) emitProgramTypes(program *sema.Program) {
	for _, en := range program.Enums() {
		g.generateEnum(en)
	}
	for _, td := range program.Typedefs() {
		g.generateTypedef(td)
	}
	// Objects returns structs, unions and exceptions in declaration order.
	for _, obj := range program.Objects() {
		g.generateStruct(obj)
	}
	for _, svc := range program.Services() {
		g.generateService(svc)
	}
}

func (g *generator) generateEnum(tenum *sema.Enum) {
	name := tenum.Name()
	g.sb.WriteString("  class " + name + " {\n")
	g.sb.WriteString("    <<enumeration>>\n")
	for _, val := range tenum.Constants() {
		g.sb.WriteString("    " + val.Name() + " = " + strconv.FormatInt(int64(val.Value()), 10) + "\n")
	}
	g.sb.WriteString("  }\n")
}

func (g *generator) generateTypedef(ttypedef *sema.Typedef) {
	name := ttypedef.Name()
	base := ttypedef.Type()

	g.sb.WriteString("  class " + name + " {\n")
	g.sb.WriteString("    <<typedef>>\n")

	if base.IsBaseType() || base.IsContainer() {
		g.sb.WriteString("    " + mmdTypeStr(base) + "\n")
	} else {
		// named type: emit edge, no attribute line
		g.edges = append(g.edges, "  "+name+" --> "+base.Name())
	}

	g.sb.WriteString("  }\n")
}

func (g *generator) generateStruct(tstruct *sema.Struct) {
	name := tstruct.Name()
	g.sb.WriteString("  class " + name + " {\n")

	switch {
	case tstruct.IsXception():
		g.sb.WriteString("    <<exception>>\n")
	case tstruct.IsUnion():
		g.sb.WriteString("    <<union>>\n")
	default:
		g.sb.WriteString("    <<struct>>\n")
	}

	for _, mem := range tstruct.Members() {
		fieldName := mem.Name()
		g.sb.WriteString("    +")
		g.printType(mem.Type(), name, fieldName)
		g.sb.WriteString(" " + fieldName + "\n")
	}

	g.sb.WriteString("  }\n")
}

func (g *generator) generateService(tservice *sema.Service) {
	svcName := tservice.Name()
	g.sb.WriteString("  class " + svcName + " {\n")
	g.sb.WriteString("    <<service>>\n")

	functions := tservice.Functions()
	for _, fn := range functions {
		var ret string
		if fn.IsOneway() {
			ret = "oneway void"
		} else {
			ret = mmdTypeStr(fn.ReturnType())
		}
		g.sb.WriteString("    +" + fn.Name() + "(" + formatParams(fn) + ") " + ret + "\n")
	}

	g.sb.WriteString("  }\n")

	if tservice.Extends() != nil {
		g.edges = append(g.edges, "  "+svcName+" --|> "+tservice.Extends().Name())
	}

	if g.exceptionArrows {
		emitted := map[string]bool{}
		for _, fn := range functions {
			for _, ex := range fn.Xceptions().Members() {
				edge := "  " + svcName + " ..> " + ex.Type().Name()
				if !emitted[edge] {
					g.edges = append(g.edges, edge)
					emitted[edge] = true
				}
			}
		}
	}
}

// mmdTypeStr returns the Mermaid-escaped string representation of a
// type. Container generics use tilde syntax: list~T~, set~T~, map~K,V~.
func mmdTypeStr(ttype sema.Type) string {
	switch {
	case ttype.IsList():
		return "list~" + mmdTypeStr(ttype.(*sema.List).ElemType()) + "~"
	case ttype.IsSet():
		return "set~" + mmdTypeStr(ttype.(*sema.Set).ElemType()) + "~"
	case ttype.IsMap():
		m := ttype.(*sema.Map)
		return "map~" + mmdTypeStr(m.KeyType()) + "," + mmdTypeStr(m.ValType()) + "~"
	case ttype.IsBaseType():
		if ttype.IsBinary() {
			return "binary"
		}
		return ttype.Name()
	default:
		return ttype.Name()
	}
}

// printType writes the Mermaid type string to sb. When ownerRef is
// non-empty and the type is a named (non-primitive, non-container) type,
// it pushes a directed edge to edges. edgeLabel, when non-empty, is
// appended as " : label".
func (g *generator) printType(ttype sema.Type, ownerRef, edgeLabel string) {
	g.sb.WriteString(mmdTypeStr(ttype))
	if ownerRef != "" && !ttype.IsBaseType() && !ttype.IsContainer() {
		edge := "  " + ownerRef + " --> " + ttype.Name()
		if edgeLabel != "" {
			edge += " : " + edgeLabel
		}
		g.edges = append(g.edges, edge)
	}
}

// formatParams returns comma-separated "type name" pairs for all
// function arguments.
func formatParams(tfunc *sema.Function) string {
	args := tfunc.Arglist().Members()
	var result strings.Builder
	first := true
	for _, arg := range args {
		if !first {
			result.WriteString(", ")
		}
		first = false
		result.WriteString(mmdTypeStr(arg.Type()) + " " + arg.Name())
	}
	return result.String()
}
