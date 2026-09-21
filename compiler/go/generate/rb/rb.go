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

// Package rb is a method-for-method port of
// compiler/cpp/src/thrift/generate/t_rb_generator.cc. The port writes
// exactly the bytes the C++ generator writes, which is checked against the
// C++ compiler by the parity tests; that is why the emitter builds strings
// the way an ostream would rather than through templates.
package rb

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

// Options are the rb:... generator options.
type Options struct {
	// Rubygems adds a `require "rubygems"` line to the top of each
	// generated file.
	Rubygems bool
	// Namespaced generates files in idiomatic namespaced directories.
	Namespaced bool
}

// ParseOptions parses the part after "rb:" of a --gen argument.
func ParseOptions(spec string) (Options, error) {
	var o Options
	for _, option := range strings.Split(spec, ",") {
		key := option
		if i := strings.IndexByte(option, '='); i >= 0 {
			key = option[:i]
		}
		switch key {
		case "":
		case "rubygems":
			o.Rubygems = true
		case "namespaced":
			o.Namespaced = true
		default:
			return o, &emit.Error{Msg: "unknown option rb:" + key}
		}
	}
	return o, nil
}

func init() {
	generate.Register(generate.Info{
		Name:     "rb",
		LongName: "Ruby",
		Options: []generate.Option{
			{Name: "rubygems", Help: `Add a "require 'rubygems'" line to the top of each generated file.`},
			{Name: "namespaced", Help: "Generate files in idiomatic namespaced directories."},
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
	g := newGenerator(program, opts)
	g.generateProgram()
	return nil
}

// ofstream is t_rb_ofstream: an std::ofstream subclass with indenting
// functionality. It buffers instead of streaming to disk, like the other
// generator ports, and is written out with emit.WriteFile once complete.
type ofstream struct {
	sb          strings.Builder
	indentLevel int
}

func (o *ofstream) indent() {
	for i := 0; i < o.indentLevel; i++ {
		o.sb.WriteString("  ")
	}
}

func (o *ofstream) write(s string) { o.sb.WriteString(s) }

func (o *ofstream) indentUp()   { o.indentLevel++ }
func (o *ofstream) indentDown() { o.indentLevel-- }

func (o *ofstream) String() string { return o.sb.String() }

// Generator is t_rb_generator for one program.
type Generator struct {
	program     *sema.Program
	opts        Options
	programName string
	serviceName string

	fTypes  *ofstream
	fConsts *ofstream
	// fService is replaced for every service, like f_service_ being
	// reopened by generate_service.
	fService *ofstream

	fTypesName  string
	fConstsName string

	namespaceDir  string
	requirePrefix string

	typesNeedSeparator   bool
	constsNeedSeparator  bool
	serviceNeedSeparator bool
}

func newGenerator(program *sema.Program, opts Options) *Generator {
	return &Generator{
		program:     program,
		opts:        opts,
		programName: program.Name(),
		fTypes:      &ofstream{},
		fConsts:     &ofstream{},
	}
}

// generateProgram is t_generator::generate_program specialized with the rb
// overrides: init, enums, typedefs (no-op), forward declarations, structs
// and exceptions in declared order, constants, services, close.
func (g *Generator) generateProgram() {
	g.initGenerator()
	for _, e := range g.program.Enums() {
		g.generateEnum(e)
	}
	for _, t := range g.program.Typedefs() {
		g.generateTypedef(t)
	}
	for _, s := range g.program.Objects() {
		g.generateForwardDeclaration(s)
	}
	for _, s := range g.program.Objects() {
		if s.IsXception() {
			g.generateXception(s)
		} else {
			g.generateStruct(s)
		}
	}
	for _, c := range g.program.Consts() {
		g.generateConst(c)
	}
	for _, s := range g.program.Services() {
		g.serviceName = s.Name()
		g.generateService(s)
	}
	g.closeGenerator()
}

// initGenerator prepares the gen-rb (or namespaced) output directory and
// opens the types and constants files.
func (g *Generator) initGenerator() {
	modules := rubyModules(g.program)
	haveModules := len(modules) > 0

	subdir := g.outDir()

	if g.opts.Namespaced {
		g.requirePrefix = rbNamespaceToPathPrefix(g.program.Namespace("rb"))
		dir := g.requirePrefix
		for {
			loc := strings.IndexByte(dir, '/')
			if loc < 0 {
				break
			}
			subdir = subdir + dir[:loc] + "/"
			dir = dir[loc+1:]
		}
	}

	g.namespaceDir = subdir
	emit.Mkdir(g.namespaceDir)

	g.fTypesName = g.namespaceDir + underscore(g.programName) + "_types.rb"
	g.fConstsName = g.namespaceDir + underscore(g.programName) + "_constants.rb"

	g.fTypes.write(rbAutogenComment() + "\n" + g.renderRequireThrift() + g.renderIncludes())
	if haveModules {
		g.fTypes.write("\n")
	}
	beginNamespace(g.fTypes, modules)
	g.typesNeedSeparator = !haveModules

	g.fConsts.write(rbAutogenComment() + "\n" + g.renderRequireThrift() + "require \"" +
		g.requirePrefix + underscore(g.programName) + "_types\"\n")
	if haveModules {
		g.fConsts.write("\n")
	}
	beginNamespace(g.fConsts, modules)
	g.constsNeedSeparator = !haveModules
}

// closeGenerator closes the type files, i.e. writes them out.
func (g *Generator) closeGenerator() {
	endNamespace(g.fTypes, rubyModules(g.program))
	endNamespace(g.fConsts, rubyModules(g.program))
	emit.WriteFile(g.fTypesName, g.fTypes.String())
	emit.WriteFile(g.fConstsName, g.fConsts.String())
}

// outDir is t_generator::get_out_dir with out_dir_base_ "gen-rb".
func (g *Generator) outDir() string {
	if g.program.IsOutPathAbsolute() {
		return g.program.OutPath() + "/"
	}
	return g.program.OutPath() + "gen-rb/"
}

// renderRequireThrift renders the require of thrift itself, and possibly
// of the rubygems dependency.
func (g *Generator) renderRequireThrift() string {
	if g.opts.Rubygems {
		return "require \"rubygems\"\nrequire \"thrift\"\n"
	}
	return "require \"thrift\"\n"
}

// renderIncludes renders all the imports necessary for including another
// Thrift program.
func (g *Generator) renderIncludes() string {
	result := ""
	for _, inc := range g.program.Includes() {
		if g.opts.Namespaced {
			includedRequirePrefix := rbNamespaceToPathPrefix(inc.Namespace("rb"))
			result += "require \"" + includedRequirePrefix + underscore(inc.Name()) + "_types\"\n"
		} else {
			result += "require \"" + underscore(inc.Name()) + "_types\"\n"
		}
	}
	return result
}

// rbAutogenComment is the autogen'd comment.
func rbAutogenComment() string {
	return "# frozen_string_literal: true\n" +
		"#\n" +
		"# Autogenerated by Thrift Compiler (" + version.Version + ")\n" +
		"#\n" +
		"# DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING\n" +
		"#\n"
}

// rubyModules is ruby_modules: the program's "rb" namespace split on '.'
// and capitalized component by component.
func rubyModules(p *sema.Program) []string {
	ns := p.Namespace("rb")
	if ns == "" {
		return nil
	}
	parts := strings.Split(ns, ".")
	modules := make([]string, len(parts))
	for i, part := range parts {
		modules[i] = capitalize(part)
	}
	return modules
}

// rbNamespaceToPathPrefix mirrors t_rb_generator::rb_namespace_to_path_prefix.
func rbNamespaceToPathPrefix(rbNamespace string) string {
	namespacesLeft := rbNamespace
	pathPrefix := ""
	for {
		loc := strings.IndexByte(namespacesLeft, '.')
		if loc < 0 {
			break
		}
		pathPrefix += underscore(namespacesLeft[:loc]) + "/"
		namespacesLeft = namespacesLeft[loc+1:]
	}
	if len(namespacesLeft) > 0 {
		pathPrefix += underscore(namespacesLeft) + "/"
	}
	return pathPrefix
}

func beginNamespace(out *ofstream, modules []string) {
	for _, m := range modules {
		out.indent()
		out.write("module " + m + "\n")
		out.indentUp()
	}
}

func endNamespace(out *ofstream, modules []string) {
	for range modules {
		out.indentDown()
		out.indent()
		out.write("end\n")
	}
}

// topLevelSeparatorState is top_level_separator_state: it identifies which
// of the three output buffers out is, and returns the flag that tracks
// whether the next top-level declaration needs a blank line before it.
func (g *Generator) topLevelSeparatorState(out *ofstream) *bool {
	switch out {
	case g.fTypes:
		return &g.typesNeedSeparator
	case g.fConsts:
		return &g.constsNeedSeparator
	case g.fService:
		return &g.serviceNeedSeparator
	}
	emit.Throw("internal error: unknown ruby output stream")
	return nil
}

func (g *Generator) maybeSeparateTopLevel(out *ofstream) {
	if *g.topLevelSeparatorState(out) {
		out.write("\n")
	}
}

func (g *Generator) markTopLevelWritten(out *ofstream) {
	*g.topLevelSeparatorState(out) = true
}

// generateTypedef does nothing. This is not done in Ruby, types are all
// implicit.
func (g *Generator) generateTypedef(ttypedef *sema.Typedef) { _ = ttypedef }

// generateEnum generates code for an enumerated type, using a module to
// scope the values.
func (g *Generator) generateEnum(tenum *sema.Enum) {
	g.maybeSeparateTopLevel(g.fTypes)
	g.fTypes.indent()
	g.fTypes.write("module " + capitalize(tenum.Name()) + "\n")
	g.fTypes.indentUp()

	constants := tenum.Constants()
	for _, c := range constants {
		value := c.Value()
		name := capitalize(c.Name())
		g.generateRdoc(g.fTypes, c)
		g.fTypes.indent()
		g.fTypes.write(name + " = " + strconv.FormatInt(int64(value), 10) + "\n")
	}

	// Create a hash mapping values back to their names (as strings) since
	// ruby has no native enum type.
	g.fTypes.indent()
	g.fTypes.write("VALUE_MAP = {")
	for i, c := range constants {
		if i != 0 {
			g.fTypes.write(", ")
		}
		g.fTypes.write(strconv.FormatInt(int64(c.Value()), 10) + " => \"" + capitalize(c.Name()) + "\"")
	}
	g.fTypes.write("}\n")

	// Create a set with valid values for this enum.
	g.fTypes.indent()
	g.fTypes.write("VALID_VALUES = Set.new([")
	for i, c := range constants {
		if i != 0 {
			g.fTypes.write(", ")
		}
		g.fTypes.write(capitalize(c.Name()))
	}
	g.fTypes.write("]).freeze\n")

	g.fTypes.indentDown()
	g.fTypes.indent()
	g.fTypes.write("end\n")
	g.markTopLevelWritten(g.fTypes)
}

// generateConst generates a constant value.
func (g *Generator) generateConst(tconst *sema.Const) {
	g.maybeSeparateTopLevel(g.fConsts)
	typ := tconst.Type()
	name := capitalize(tconst.Name())
	value := tconst.Value()

	g.fConsts.indent()
	g.fConsts.write(name + " = ")
	renderConstValue(g.fConsts, typ, value)
	g.fConsts.write("\n")
	g.markTopLevelWritten(g.fConsts)
}

// generateRdoc is generate_rdoc: it prints the object's doc comment as a
// block of "#" lines, using the getline loop of the C++ code rather than
// emit.DocstringComment (which has a different quirk). std::getline splits
// on '\n' without a length limit, and if the doc text ends with a trailing
// newline no empty line is printed after it; a '\r' at the end of a line
// (from a CRLF source file) is stripped without ever reaching the output.
func (g *Generator) generateRdoc(out *ofstream, tdoc hasDoc) {
	if !tdoc.HasDoc() {
		return
	}
	rest := tdoc.Doc()
	for {
		idx := strings.IndexByte(rest, '\n')
		var line string
		if idx < 0 {
			if rest == "" {
				return
			}
			line = rest
			rest = ""
			writeRdocLine(out, line)
			return
		}
		line = rest[:idx]
		rest = rest[idx+1:]
		writeRdocLine(out, line)
	}
}

type hasDoc interface {
	HasDoc() bool
	Doc() string
}

func writeRdocLine(out *ofstream, line string) {
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	out.indent()
	out.write("#")
	if len(line) > 0 {
		out.write(" " + line)
	}
	out.write("\n")
}

// typeName is t_rb_generator::type_name.
func typeName(t sema.Type) string {
	name := t.Name()
	if t.IsStruct() || t.IsXception() || t.IsEnum() {
		name = capitalize(t.Name())
	}
	return name
}

// fullTypeName is t_rb_generator::full_type_name.
func fullTypeName(t sema.Type) string {
	prefix := "::"
	for _, m := range rubyModules(t.Program()) {
		prefix += m + "::"
	}
	return prefix + typeName(t)
}

// typeToEnum converts the parse type to a Ruby type, as
// t_rb_generator::type_to_enum.
func typeToEnum(t sema.Type) string {
	t = sema.TrueType(t)
	if t.IsBaseType() {
		base := t.(*sema.BaseType)
		switch base.Base() {
		case sema.TypeVoid:
			emit.Throw("NO T_VOID CONSTRUCT")
		case sema.TypeString:
			return "::Thrift::Types::STRING"
		case sema.TypeBool:
			return "::Thrift::Types::BOOL"
		case sema.TypeI8:
			return "::Thrift::Types::BYTE"
		case sema.TypeI16:
			return "::Thrift::Types::I16"
		case sema.TypeI32:
			return "::Thrift::Types::I32"
		case sema.TypeI64:
			return "::Thrift::Types::I64"
		case sema.TypeDouble:
			return "::Thrift::Types::DOUBLE"
		case sema.TypeUUID:
			return "::Thrift::Types::UUID"
		default:
			emit.Throw("compiler error: unhandled type")
		}
	} else if t.IsEnum() {
		return "::Thrift::Types::I32"
	} else if t.IsStruct() || t.IsXception() {
		return "::Thrift::Types::STRUCT"
	} else if t.IsMap() {
		return "::Thrift::Types::MAP"
	} else if t.IsSet() {
		return "::Thrift::Types::SET"
	} else if t.IsList() {
		return "::Thrift::Types::LIST"
	}
	emit.Throw("INVALID TYPE IN type_to_enum: %s", t.Name())
	return ""
}

// fieldIDConstantName is t_rb_generator::field_id_constant_name.
func fieldIDConstantName(fieldName string) string {
	return upcaseString(fieldName) + "_FIELD_ID"
}

// renderConstValue prints the value of a constant with the given type.
// Type checking is not performed here, as it is always run beforehand by
// sema.ValidateInput.
func renderConstValue(out *ofstream, typ sema.Type, value *sema.ConstValue) {
	typ = sema.TrueType(typ)
	if typ.IsBaseType() {
		base := typ.(*sema.BaseType)
		switch base.Base() {
		case sema.TypeString, sema.TypeUUID:
			out.write("%q\"" + emit.EscapeString(value.String()) + "\"")
		case sema.TypeBool:
			if value.Integer() > 0 {
				out.write("true")
			} else {
				out.write("false")
			}
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			out.write(strconv.FormatInt(value.Integer(), 10))
		case sema.TypeDouble:
			if value.Kind() == sema.CVInteger {
				out.write(strconv.FormatInt(value.Integer(), 10))
			} else {
				out.write(formatDouble(value.Double()))
			}
		default:
			emit.Throw("compiler error: no const of base type %s", sema.BaseName(base.Base()))
		}
	} else if typ.IsEnum() {
		out.write(strconv.FormatInt(value.Integer(), 10))
	} else if typ.IsStruct() || typ.IsXception() {
		out.write(fullTypeName(typ) + ".new({\n")
		out.indentUp()
		s := typ.(*sema.Struct)
		for _, entry := range value.Map() {
			var fieldType sema.Type
			for _, f := range s.Members() {
				if f.Name() == entry.Key.String() {
					fieldType = f.Type()
				}
			}
			if fieldType == nil {
				emit.Throw("type error: %s has no field %s", typ.Name(), entry.Key.String())
			}
			out.indent()
			renderConstValue(out, sema.GlobalString, entry.Key)
			out.write(" => ")
			renderConstValue(out, fieldType, entry.Value)
			out.write(",\n")
		}
		out.indentDown()
		out.indent()
		out.write("})")
	} else if typ.IsMap() {
		m := typ.(*sema.Map)
		out.write("{\n")
		out.indentUp()
		for _, entry := range value.Map() {
			out.indent()
			renderConstValue(out, m.KeyType(), entry.Key)
			out.write(" => ")
			renderConstValue(out, m.ValType(), entry.Value)
			out.write(",\n")
		}
		out.indentDown()
		out.indent()
		out.write("}")
	} else if typ.IsList() || typ.IsSet() {
		var elemType sema.Type
		if typ.IsList() {
			elemType = typ.(*sema.List).ElemType()
		} else {
			elemType = typ.(*sema.Set).ElemType()
		}
		if typ.IsSet() {
			out.write("Set.new([\n")
		} else {
			out.write("[\n")
		}
		out.indentUp()
		for _, elem := range value.List() {
			out.indent()
			renderConstValue(out, elemType, elem)
			out.write(",\n")
		}
		out.indentDown()
		out.indent()
		if typ.IsSet() {
			out.write("])")
		} else {
			out.write("]")
		}
	} else {
		emit.Throw("CANNOT GENERATE CONSTANT FOR TYPE: %s", typ.Name())
	}
}

// generateFieldData is t_rb_generator::generate_field_data: it renders one
// FIELDS entry, or a nested element/key/value/class field descriptor.
func generateFieldData(out *ofstream, fieldType sema.Type, fieldName string, fieldValue *sema.ConstValue, optional bool) {
	fieldType = sema.TrueType(fieldType)
	multiline := fieldValue != nil &&
		(fieldType.IsStruct() || fieldType.IsXception() || fieldType.IsMap() || fieldType.IsList() || fieldType.IsSet())

	// Begin this field's defn.
	out.write("{")
	if multiline {
		out.write("\n")
		out.indentUp()
		out.indent()
		out.write("type: " + typeToEnum(fieldType) + ",\n")
	} else {
		out.write("type: " + typeToEnum(fieldType))
	}

	if fieldName != "" {
		if multiline {
			out.indent()
			out.write("name: \"" + fieldName + "\",\n")
		} else {
			out.write(", name: \"" + fieldName + "\"")
		}
	}

	if fieldValue != nil {
		if multiline {
			out.indent()
			out.write("default: ")
			renderConstValue(out, fieldType, fieldValue)
			out.write(",\n")
		} else {
			out.write(", default: ")
			renderConstValue(out, fieldType, fieldValue)
		}
	}

	if !fieldType.IsBaseType() {
		if fieldType.IsStruct() || fieldType.IsXception() {
			if multiline {
				out.indent()
				out.write("class: " + fullTypeName(fieldType) + ",\n")
			} else {
				out.write(", class: " + fullTypeName(fieldType))
			}
		} else if fieldType.IsList() {
			if multiline {
				out.indent()
				out.write("element: ")
			} else {
				out.write(", element: ")
			}
			generateFieldData(out, fieldType.(*sema.List).ElemType(), "", nil, false)
			if multiline {
				out.write(",\n")
			}
		} else if fieldType.IsMap() {
			m := fieldType.(*sema.Map)
			if multiline {
				out.indent()
				out.write("key: ")
			} else {
				out.write(", key: ")
			}
			generateFieldData(out, m.KeyType(), "", nil, false)
			if multiline {
				out.write(",\n")
				out.indent()
				out.write("value: ")
			} else {
				out.write(", value: ")
			}
			generateFieldData(out, m.ValType(), "", nil, false)
			if multiline {
				out.write(",\n")
			}
		} else if fieldType.IsSet() {
			if multiline {
				out.indent()
				out.write("element: ")
			} else {
				out.write(", element: ")
			}
			generateFieldData(out, fieldType.(*sema.Set).ElemType(), "", nil, false)
			if multiline {
				out.write(",\n")
			}
		}
	} else if fieldType.(*sema.BaseType).IsBinary() {
		if multiline {
			out.indent()
			out.write("binary: true,\n")
		} else {
			out.write(", binary: true")
		}
	}

	if optional {
		if multiline {
			out.indent()
			out.write("optional: true,\n")
		} else {
			out.write(", optional: true")
		}
	}

	if fieldType.IsEnum() {
		if multiline {
			out.indent()
			out.write("enum_class: " + fullTypeName(fieldType) + ",\n")
		} else {
			out.write(", enum_class: " + fullTypeName(fieldType))
		}
	}

	// End of this field's defn.
	if multiline {
		out.indentDown()
		out.indent()
		out.write("}")
	} else {
		out.write("}")
	}
}

// ---- string helpers shared with the other t_generator ports ----

// capitalize is t_generator::capitalize: it upper-cases the first byte
// only.
func capitalize(in string) string {
	if in == "" {
		return in
	}
	b := []byte(in)
	b[0] = toUpper(b[0])
	return string(b)
}

// upcaseString is t_oop_generator::upcase_string.
func upcaseString(s string) string {
	b := []byte(s)
	for i, c := range b {
		b[i] = toUpper(c)
	}
	return string(b)
}

// underscore is t_oop_generator::underscore: aMultiWord -> a_multi_word.
func underscore(in string) string {
	b := []byte(in)
	if len(b) > 0 {
		b[0] = toLower(b[0])
	}
	var out []byte
	for i, c := range b {
		if i > 0 && isUpper(c) {
			out = append(out, '_', toLower(c))
		} else {
			out = append(out, c)
		}
	}
	return string(out)
}

func isUpper(c byte) bool { return c >= 'A' && c <= 'Z' }
func toUpper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 'a' + 'A'
	}
	return c
}
func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c - 'A' + 'a'
	}
	return c
}

// formatDouble mimics `ostream << double` with the default precision of
// six significant digits, as golang.formatDouble does.
func formatDouble(f float64) string {
	return strconv.FormatFloat(f, 'g', 6, 64)
}
