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

// Package gv is t_gv_generator.cc: it renders a resolved program as a
// Graphviz "dot" file describing its types and services, byte for byte
// like the C++ compiler's --gen gv.
package gv

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// Options are the gv:... generator options.
type Options struct {
	// Exceptions is "exceptions": whether to draw arrows from functions
	// to the exceptions they throw.
	Exceptions bool
}

// ParseOptions parses the part after "gv:" of a --gen argument.
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
			return o, &emit.Error{Msg: "unknown option gv:" + key}
		}
	}
	return o, nil
}

func init() {
	generate.Register(generate.Info{
		Name:     "gv",
		LongName: "Graphviz",
		Options: []generate.Option{
			{Name: "exceptions", Help: "Whether to draw arrows from functions to exception."},
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
	outDir := program.OutPath() + "gen-gv/"
	if program.IsOutPathAbsolute() {
		outDir = program.OutPath() + "/"
	}
	emit.Mkdir(outDir)
	emit.WriteFile(outDir+program.Name()+".gv", Render(program, opts))
	return nil
}

// generator is t_gv_generator: it accumulates the dot file body in sb and
// the edges list to print at the end, exactly as f_out_ and the C++
// class's edges member do.
type generator struct {
	program *sema.Program
	opts    Options
	sb      strings.Builder
	edges   []string
}

// Render renders the program as the dot file, with a trailing newline
// like the file the C++ compiler writes.
func Render(p *sema.Program, opts Options) string {
	g := &generator{program: p, opts: opts}
	g.generateProgram()
	return g.sb.String()
}

// generateProgram is t_generator::generate_program's dispatch order,
// which t_gv_generator does not override: init, enums, typedefs, structs
// and exceptions in declared order, consts, services, close.
func (g *generator) generateProgram() {
	g.initGenerator()
	for _, e := range g.program.Enums() {
		g.generateEnum(e)
	}
	for _, t := range g.program.Typedefs() {
		g.generateTypedef(t)
	}
	for _, s := range g.program.Objects() {
		g.generateStruct(s)
	}
	for _, c := range g.program.Consts() {
		g.generateConst(c)
	}
	for _, s := range g.program.Services() {
		g.generateService(s)
	}
	g.closeGenerator()
}

/**
 * Init generator:
 * - Adds some escaping for the Graphviz domain.
 * - Create output directory and open file for writting.
 * - Write the file header.
 */
func (g *generator) initGenerator() {
	g.sb.WriteString("digraph \"" + gvEscape(g.program.Name()) + "\" {\n")
	g.sb.WriteString("node [style=filled, shape=record];\n")
	g.sb.WriteString("edge [arrowsize=0.5];\n")
	g.sb.WriteString("rankdir=LR\n")
}

/**
 * Closes generator:
 * - Print accumulated nodes connections.
 * - Print footnote.
 * - Closes file.
 */
func (g *generator) closeGenerator() {
	for _, e := range g.edges {
		g.sb.WriteString(e + "\n")
	}
	g.sb.WriteString("}\n")
}

func (g *generator) generateTypedef(t *sema.Typedef) {
	name := t.Name()
	g.sb.WriteString("node [fillcolor=azure];\n")
	g.sb.WriteString(name + " [label=\"")

	g.sb.WriteString(gvEscape(name))
	g.sb.WriteString(" :: ")
	g.printType(t.Type(), name)

	g.sb.WriteString("\"];\n")
}

func (g *generator) generateEnum(e *sema.Enum) {
	name := e.Name()
	g.sb.WriteString("node [fillcolor=white];\n")
	g.sb.WriteString(name + " [label=\"enum " + gvEscape(name))

	for _, v := range e.Constants() {
		g.sb.WriteString("|" + v.Name())
		g.sb.WriteString(" = ")
		g.sb.WriteString(strconv.FormatInt(int64(v.Value()), 10))
	}

	g.sb.WriteString("\"];\n")
}

func (g *generator) generateConst(c *sema.Const) {
	name := c.Name()

	g.sb.WriteString("node [fillcolor=aliceblue];\n")
	g.sb.WriteString("const_" + name + " [label=\"")

	g.sb.WriteString(gvEscape(name))
	g.sb.WriteString(" = ")
	g.printConstValue(c.Type(), c.Value())
	g.sb.WriteString(" :: ")
	g.printType(c.Type(), "const_"+name)

	g.sb.WriteString("\"];\n")
}

func (g *generator) generateStruct(s *sema.Struct) {
	name := s.Name()

	switch {
	case s.IsXception():
		g.sb.WriteString("node [fillcolor=lightpink];\n")
		g.sb.WriteString(name + " [label=\"")
		g.sb.WriteString("exception " + gvEscape(name))
	case s.IsUnion():
		g.sb.WriteString("node [fillcolor=lightcyan];\n")
		g.sb.WriteString(name + " [label=\"")
		g.sb.WriteString("union " + gvEscape(name))
	default:
		g.sb.WriteString("node [fillcolor=beige];\n")
		g.sb.WriteString(name + " [label=\"")
		g.sb.WriteString("struct " + gvEscape(name))
	}

	for _, f := range s.Members() {
		fieldName := f.Name()

		// print port (anchor reference)
		g.sb.WriteString("|<field_" + fieldName + ">")

		// field name :: field type
		g.sb.WriteString(f.Name())
		g.sb.WriteString(" :: ")
		g.printType(f.Type(), name+":field_"+fieldName)
	}

	g.sb.WriteString("\"];\n")
}

func (g *generator) printType(t sema.Type, structFieldRef string) {
	switch {
	case t.IsContainer():
		switch {
		case t.IsList():
			g.sb.WriteString(`list\<`)
			g.printType(t.(*sema.List).ElemType(), structFieldRef)
			g.sb.WriteString(`\>`)
		case t.IsSet():
			g.sb.WriteString(`set\<`)
			g.printType(t.(*sema.Set).ElemType(), structFieldRef)
			g.sb.WriteString(`\>`)
		case t.IsMap():
			m := t.(*sema.Map)
			g.sb.WriteString(`map\<`)
			g.printType(m.KeyType(), structFieldRef)
			g.sb.WriteString(", ")
			g.printType(m.ValType(), structFieldRef)
			g.sb.WriteString(`\>`)
		}
	case t.IsBaseType():
		if t.IsBinary() {
			g.sb.WriteString("binary")
		} else {
			g.sb.WriteString(t.Name())
		}
	default:
		g.sb.WriteString(t.Name())
		g.edges = append(g.edges, structFieldRef+" -> "+t.Name())
	}
}

/**
 * Prints out an string representation of the provided constant value
 *
 * The C++ generator computes a CV_MAP's key/value type, or a CV_LIST's
 * element type, by casting the constant's declared type to t_map or
 * t_list/t_set without checking it actually is one. A struct literal is
 * itself a CV_MAP (the parser has no other way to represent one), so
 * that cast runs on a real but wrongly typed t_struct whenever a
 * struct-valued constant turns up where a container was assumed: one
 * element of a const list<Bar>, a struct-typed map key, or a struct
 * field whose own value is a further container. Reading a member out of
 * a real (if wrongly typed) C++ object this way does not fault, so a
 * leaf value under it -- an integer, a double or a string -- prints
 * exactly as it would with the correct type, and t_gv_generator has
 * emitted exactly this output for every corpus file with such a
 * constant. Only a further identifier needs the type for its name, and
 * only then does the C++ generator dereference the bogus type it
 * derived; checked empirically (lib/go/test/ConstOptionalField.thrift),
 * that is undefined behavior in the C++ binary, which crashes on most
 * runs and reproduces one fixed, semantically wrong output on the rest.
 * typ is nil here for exactly the cases where the C++ generator would be
 * working from such a bogus type. Using it to name an identifier always
 * throws. Using it as the type of a further CV_MAP or CV_LIST also
 * throws: that reads a key/value/element type out of a pointer that was
 * itself only ever a misread of unrelated memory, not a real object,
 * which is unsafe on the C++ side too (checked: lib/go/test/
 * StructKeyTest.thrift's struct-keyed map with a struct-typed default
 * value crashes the C++ binary on most runs without ever reaching an
 * identifier). One level of "real object read as the wrong type" is
 * where the two generators still agree; a second level is where the Go
 * generator rejects the file instead of gambling on which of the C++
 * binary's undefined outcomes to match.
 */
func (g *generator) printConstValue(typ sema.Type, tvalue *sema.ConstValue) {
	first := true
	switch tvalue.Kind() {
	case sema.CVInteger:
		g.sb.WriteString(strconv.FormatInt(tvalue.Integer(), 10))
	case sema.CVDouble:
		g.sb.WriteString(formatDouble(tvalue.Double()))
	case sema.CVString:
		g.sb.WriteString(`\"` + gvEscape(tvalue.String()) + `\"`)
	case sema.CVMap:
		if typ == nil {
			emit.Throw("gv: nested map constant value has no usable declared type here")
		}
		g.sb.WriteString(`\{ `)
		var keyType, valType sema.Type
		if m, ok := sema.TrueType(typ).(*sema.Map); ok {
			keyType, valType = m.KeyType(), m.ValType()
		}
		for _, e := range tvalue.Map() {
			if !first {
				g.sb.WriteString(", ")
			}
			first = false
			g.printConstValue(keyType, e.Key)
			g.sb.WriteString(" = ")
			g.printConstValue(valType, e.Value)
		}
		g.sb.WriteString(` \}`)
	case sema.CVList:
		if typ == nil {
			emit.Throw("gv: nested list constant value has no usable declared type here")
		}
		g.sb.WriteString(`\{ `)
		var elemType sema.Type
		tt := sema.TrueType(typ)
		if l, ok := tt.(*sema.List); ok {
			elemType = l.ElemType()
		} else if s, ok := tt.(*sema.Set); ok {
			elemType = s.ElemType()
		}
		for _, e := range tvalue.List() {
			if !first {
				g.sb.WriteString(", ")
			}
			first = false
			g.printConstValue(elemType, e)
		}
		g.sb.WriteString(` \}`)
	case sema.CVIdentifier:
		if typ == nil {
			emit.Throw("gv: identifier constant value has no usable declared type here")
		}
		g.sb.WriteString(gvEscape(typ.Name()) + "." + gvEscape(tvalue.IdentifierName()))
	default:
		g.sb.WriteString("UNKNOWN")
	}
}

func (g *generator) generateService(s *sema.Service) {
	serviceName := s.Name()
	g.sb.WriteString("subgraph cluster_" + serviceName + " {\n")
	g.sb.WriteString("node [fillcolor=bisque];\n")
	g.sb.WriteString("style=dashed;\n")
	g.sb.WriteString("label = \"" + gvEscape(serviceName) + " service\";\n")

	// TODO: service extends

	for _, fn := range s.Functions() {
		fnName := fn.Name()

		g.sb.WriteString("function_" + serviceName + fnName)
		g.sb.WriteString("[label=\"<return_type>function " + gvEscape(fnName))
		g.sb.WriteString(" :: ")
		g.printType(fn.ReturnType(), "function_"+serviceName+fnName+":return_type")

		for _, a := range fn.Arglist().Members() {
			g.sb.WriteString("|<param_" + a.Name() + ">")
			g.sb.WriteString(a.Name())
			if a.Value() != nil {
				g.sb.WriteString(" = ")
				g.printConstValue(a.Type(), a.Value())
			}
			g.sb.WriteString(" :: ")
			g.printType(a.Type(), "function_"+serviceName+fnName+":param_"+a.Name())
		}
		// end of node
		g.sb.WriteString("\"];\n")

		// Exception edges
		if g.opts.Exceptions {
			for _, x := range fn.Xceptions().Members() {
				g.edges = append(g.edges, "function_"+serviceName+fnName+" -> "+x.Type().Name()+" [color=red]")
			}
		}
	}

	g.sb.WriteString(" }\n")
}

// gvEscape is t_generator::escape_string with the two extra entries
// init_generator adds for the Graphviz domain: the record label syntax
// uses braces for field ports, so a literal brace in a name or string
// needs escaping like the control characters and quotes t_generator
// escapes by default.
func gvEscape(in string) string {
	var sb strings.Builder
	for i := 0; i < len(in); i++ {
		switch c := in[i]; c {
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '{':
			sb.WriteString(`\{`)
		case '}':
			sb.WriteString(`\}`)
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

// formatDouble mimics `ostream << double` with the default precision of
// six significant digits, as print_const_value's unformatted f_out_ <<
// tvalue->get_double() uses.
func formatDouble(f float64) string {
	return strconv.FormatFloat(f, 'g', 6, 64)
}
