/*
 * Copyright (c) 2008- Patrick Collison <patrick@collison.ie>
 * Copyright (c) 2006- Facebook
 *
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

// Package cl is a method-for-method port of
// compiler/cpp/src/thrift/generate/t_cl_generator.cc. It writes the same
// bytes as the C++ generator, which is checked against the C++ compiler by
// the parity tests; that is why the emitter builds strings the way an
// ostream would rather than through templates, and reproduces C++ quirks
// (including the map and list/set branches of renderConstValue, whose
// indent_up/indent_down calls straddle the closing paren differently from
// each other and from the struct branch: see the comments there).
package cl

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// Generator is t_cl_generator for one program.
type Generator struct {
	program *sema.Program
	opts    Options

	// copyOptions is copy_options_: the full "--gen" argument text (e.g.
	// "cl:no_asd,sys_pref=foo"), printed verbatim in every output file's
	// autogen comment.
	copyOptions string

	programName string

	// indentLevel is indent_: the current code indentation level, shared
	// mutable state across the whole generation of the program, exactly
	// like the C++ member.
	indentLevel int

	fTypes     strings.Builder
	fTypesName string
	fVars      strings.Builder
	fVarsName  string
	fASD       strings.Builder
	fASDName   string
	// hasASD is whether f_asd_ was ever opened (no_asd is false): closing
	// an unopened ofstream in the C++ source writes nothing.
	hasASD bool
}

// New creates a generator for the program. copyOptions is the exact text
// t_cl_generator's copy_options_ holds.
func New(program *sema.Program, opts Options, copyOptions string) *Generator {
	return &Generator{program: program, opts: opts, copyOptions: copyOptions, programName: program.Name()}
}

// Generate writes the Common Lisp code for the program. It is
// t_generator::generate_program restricted to the calls the Common Lisp
// generator overrides; t_cl_generator does not override generate_program,
// generate_forward_declaration or validate_input.
func (g *Generator) Generate() (err error) {
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
	g.initGenerator()
	for _, e := range g.program.Enums() {
		g.generateEnum(e)
	}
	for _, t := range g.program.Typedefs() {
		g.generateTypedef(t)
	}
	for _, s := range g.program.Objects() {
		if s.IsXception() {
			g.generateXception(s)
		} else {
			g.generateStruct(s)
		}
	}
	// generate_consts (the t_generator default): a plain loop over
	// generate_const.
	for _, c := range g.program.Consts() {
		g.generateConst(c)
	}
	for _, s := range g.program.Services() {
		g.generateService(s)
	}
	g.closeGenerator()
	return nil
}

// ---- t_generator helpers ----

func (g *Generator) indentUp()   { g.indentLevel++ }
func (g *Generator) indentDown() { g.indentLevel-- }

// indent is t_generator::indent(): the cumulative indentation string for
// the current level.
func (g *Generator) indent() string {
	return strings.Repeat("  ", g.indentLevel)
}

// outDir is t_generator::get_out_dir with out_dir_base_ == "gen-cl".
func (g *Generator) outDir() string {
	if g.program.IsOutPathAbsolute() {
		return g.program.OutPath() + "/"
	}
	return g.program.OutPath() + "gen-cl" + "/"
}

// lowercase is t_generator::lowercase.
func lowercase(in string) string {
	b := []byte(in)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

// underscore is t_generator::underscore: aMultiWord -> a_multi_word.
func underscore(in string) string {
	b := []byte(in)
	if len(b) > 0 && b[0] >= 'A' && b[0] <= 'Z' {
		b[0] += 'a' - 'A'
	}
	var out []byte
	for i, c := range b {
		if i > 0 && c >= 'A' && c <= 'Z' {
			out = append(out, '_', c+'a'-'A')
		} else {
			out = append(out, c)
		}
	}
	return string(out)
}

// formatDouble mimics `ostream << double` with the default precision of
// six significant digits.
func formatDouble(f float64) string {
	return strconv.FormatFloat(f, 'g', 6, 64)
}

// prefix is t_cl_generator::prefix: a double-quoted CL symbol name.
func prefix(symbol string) string {
	return "\"" + symbol + "\""
}

// clDocstring is t_cl_generator::cl_docstring.
func clDocstring(raw string) string {
	return strings.ReplaceAll(raw, "\"", "'")
}

// clAutogenComment is t_cl_generator::cl_autogen_comment.
func (g *Generator) clAutogenComment() string {
	return ";;; Autogenerated by Thrift\n" +
		";;; DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING\n" +
		";;; options string: " + g.copyOptions + "\n"
}

// packageOf is t_cl_generator::package_of.
func packageOf(p *sema.Program) string {
	ns := p.Namespace("cl")
	if ns == "" {
		return "thrift-generated"
	}
	return ns
}

// pkg is t_cl_generator::package.
func (g *Generator) pkg() string {
	return packageOf(g.program)
}

// renderIncludes is t_cl_generator::render_includes: the imports necessary
// for including another Thrift program.
func (g *Generator) renderIncludes() string {
	var sb strings.Builder
	sb.WriteString(":depends-on (:thrift")
	for _, inc := range g.program.Includes() {
		sb.WriteString(" :" + g.opts.SysPref + underscore(inc.Name()))
	}
	sb.WriteString(")\n")
	return sb.String()
}

// asdfDef is t_cl_generator::asdf_def.
func (g *Generator) asdfDef(out *strings.Builder) {
	out.WriteString("(asdf:defsystem #:" + g.opts.SysPref + g.programName + "\n")
	g.indentUp()
	out.WriteString(g.indent() + g.renderIncludes() +
		g.indent() + ":serial t" + "\n" +
		g.indent() + ":components (" + "(:file \"" + g.programName + "-types\") " + "(:file \"" + g.programName + "-vars\")))" + "\n")
	g.indentDown()
}

// packageDef is t_cl_generator::package_def: a package definition, with
// use references equivalent to the idl file's include statements.
func (g *Generator) packageDef(out *strings.Builder) {
	includes := g.program.Includes()
	out.WriteString("(thrift:def-package :" + g.pkg())
	if len(includes) > 0 {
		out.WriteString(" :use (")
		for _, inc := range includes {
			out.WriteString(" :" + inc.Name())
		}
		out.WriteString(")")
	}
	out.WriteString(")\n\n")
}

// packageIn is t_cl_generator::package_in.
func (g *Generator) packageIn(out *strings.Builder) {
	out.WriteString("(cl:in-package :" + g.pkg() + ")\n\n")
}

// ---- init and close ----

// initGenerator is t_cl_generator::init_generator.
func (g *Generator) initGenerator() {
	outDir := g.outDir()
	emit.Mkdir(outDir)
	programDir := outDir + g.programName + "/"
	emit.Mkdir(programDir)

	g.fTypesName = programDir + g.programName + "-types.lisp"
	g.fVarsName = programDir + g.programName + "-vars.lisp"

	g.fTypes.WriteString(g.clAutogenComment() + "\n")
	g.fVars.WriteString(g.clAutogenComment() + "\n")

	g.packageDef(&g.fTypes)
	g.packageIn(&g.fTypes)
	g.packageIn(&g.fVars)

	if !g.opts.NoASD {
		g.hasASD = true
		g.fASDName = programDir + g.opts.SysPref + g.programName + ".asd"
		g.fASD.WriteString(g.clAutogenComment() + "\n")
		g.asdfDef(&g.fASD)
	}
}

// closeGenerator is t_cl_generator::close_generator.
func (g *Generator) closeGenerator() {
	if g.hasASD {
		emit.WriteFile(g.fASDName, g.fASD.String())
	}
	emit.WriteFile(g.fTypesName, g.fTypes.String())
	emit.WriteFile(g.fVarsName, g.fVars.String())
}

// generateTypedef is t_cl_generator::generate_typedef: this is not done in
// Common Lisp, types are all implicit.
func (g *Generator) generateTypedef(t *sema.Typedef) {}

// generateEnum is t_cl_generator::generate_enum.
func (g *Generator) generateEnum(e *sema.Enum) {
	g.fTypes.WriteString("(thrift:def-enum " + prefix(e.Name()) + "\n")

	constants := e.Constants()

	g.indentUp()
	g.fTypes.WriteString(g.indent() + "(")
	for i, c := range constants {
		if i != 0 {
			g.fTypes.WriteString("\n" + g.indent() + " ")
		}
		g.fTypes.WriteString("(\"" + c.Name() + "\" . " + strconv.FormatInt(int64(c.Value()), 10) + ")")
	}
	g.indentDown()
	g.fTypes.WriteString("))\n\n")
}

// generateConst is t_cl_generator::generate_const: generates a constant
// value.
func (g *Generator) generateConst(c *sema.Const) {
	g.fVars.WriteString("(thrift:def-constant " + prefix(c.Name()) + " " + g.renderConstValue(c.Type(), c.Value()) + ")" + "\n" + "\n")
}

// renderConstValue is t_cl_generator::render_const_value. Note that type
// checking is NOT performed in this function, as it is always run
// beforehand using validateInput.
func (g *Generator) renderConstValue(typ sema.Type, value *sema.ConstValue) string {
	typ = sema.TrueType(typ)
	var out strings.Builder

	switch {
	case typ.IsBaseType():
		b := typ.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeString:
			// Note (t_cl_generator.cc): the string is embedded raw, with
			// no escaping of '"' or '\\'.
			out.WriteString("\"" + value.String() + "\"")
		case sema.TypeBool:
			if value.Integer() > 0 {
				out.WriteString("t")
			} else {
				out.WriteString("nil")
			}
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			out.WriteString(strconv.FormatInt(value.Integer(), 10))
		case sema.TypeDouble:
			if value.Kind() == sema.CVInteger {
				out.WriteString(strconv.FormatInt(value.Integer(), 10))
			} else {
				out.WriteString(formatDouble(value.Double()))
			}
		default:
			emit.Throw("compiler error: no const of base type %s", sema.BaseName(b.Base()))
		}
	case typ.IsEnum():
		out.WriteString(g.indent() + strconv.FormatInt(value.Integer(), 10))
	case typ.IsStruct() || typ.IsXception():
		if typ.IsStruct() {
			out.WriteString("(make-instance '" + lowercase(typ.Name()) + " " + "\n")
		} else {
			out.WriteString("(make-exception '" + lowercase(typ.Name()) + " " + "\n")
		}
		g.indentUp()

		fields := typ.(*sema.Struct).Members()
		for _, e := range value.Map() {
			var fieldType sema.Type
			for _, f := range fields {
				if f.Name() == e.Key.String() {
					fieldType = f.Type()
				}
			}
			if fieldType == nil {
				emit.Throw("type error: %s has no field %s", typ.Name(), e.Key.String())
			}

			out.WriteString(g.indent() + ":" + e.Key.String() + " " + g.renderConstValue(fieldType, e.Value) + "\n")
		}
		out.WriteString(g.indent() + ")")

		g.indentDown()
	case typ.IsMap():
		// Emit a hash form with both keys and values to be evaluated.
		m := typ.(*sema.Map)
		ktype, vtype := m.KeyType(), m.ValType()
		out.WriteString("(thrift:map ")
		g.indentUp()
		for _, e := range value.Map() {
			out.WriteString("\n" + g.indent() +
				"(cl:cons " + g.renderConstValue(ktype, e.Key) + " " + g.renderConstValue(vtype, e.Value) + ")")
		}
		// Quirk (t_cl_generator.cc): indent_down() runs before the closing
		// paren is appended, unlike the struct branch above and the
		// list/set branch below, so the paren lands right after the last
		// entry with no newline between them.
		g.indentDown()
		out.WriteString(g.indent() + ")")
	case typ.IsList() || typ.IsSet():
		var etype sema.Type
		if typ.IsList() {
			etype = typ.(*sema.List).ElemType()
		} else {
			etype = typ.(*sema.Set).ElemType()
		}
		if typ.IsSet() {
			out.WriteString("(thrift:set" + "\n")
		} else {
			out.WriteString("(thrift:list" + "\n")
		}
		g.indentUp()
		g.indentUp()
		for _, v := range value.List() {
			out.WriteString(g.indent() + g.renderConstValue(etype, v) + "\n")
		}
		// Quirk (t_cl_generator.cc): the closing paren is appended, at the
		// doubly-incremented indent, before either indent_down() call.
		out.WriteString(g.indent() + ")")
		g.indentDown()
		g.indentDown()
	default:
		emit.Throw("CANNOT GENERATE CONSTANT FOR TYPE: %s", typ.Name())
	}
	return out.String()
}

// generateStruct is t_cl_generator::generate_struct.
func (g *Generator) generateStruct(s *sema.Struct) {
	g.generateClStruct(&g.fTypes, s, false)
}

// generateXception is t_cl_generator::generate_xception.
func (g *Generator) generateXception(s *sema.Struct) {
	g.generateClStruct(&g.fTypes, s, true)
}

// generateClStructInternal is t_cl_generator::generate_cl_struct_internal.
// isException is unused, exactly as in the C++ source ("(void)is_exception").
func (g *Generator) generateClStructInternal(out *strings.Builder, s *sema.Struct, isException bool) {
	_ = isException
	members := s.Members()

	out.WriteString("(")

	for i, m := range members {
		value := m.Value()
		typ := m.Type()

		if i != 0 {
			out.WriteString("\n" + g.indent() + " ")
		}
		valStr := "nil"
		if value != nil {
			valStr = g.renderConstValue(typ, value)
		}
		out.WriteString("(" + prefix(m.Name()) + " " + valStr + " :id " + strconv.FormatInt(int64(m.Key()), 10))
		// This mirrors the C++ source's dangling-else nesting exactly; the
		// outer condition can only be true when typespec(type) == "string",
		// which typespec never returns for a binary type (it answers
		// "binary" first), so the is_binary() arm below is unreachable and
		// the else arm always fires. Kept as ported.
		if typ.IsBaseType() && g.typespec(typ) == "string" {
			if typ.(*sema.BaseType).IsBinary() {
				out.WriteString(" :type binary")
			} else {
				out.WriteString(" :type string")
			}
		} else {
			out.WriteString(" :type " + g.typespec(typ))
		}
		if m.Req() == sema.Optional {
			out.WriteString(" :optional t")
		}
		if m.HasDoc() {
			out.WriteString(" :documentation \"" + clDocstring(m.Doc()) + "\"")
		}
		out.WriteString(")")
	}

	out.WriteString(")")
}

// generateClStruct is t_cl_generator::generate_cl_struct.
func (g *Generator) generateClStruct(out *strings.Builder, s *sema.Struct, isException bool) {
	name := g.typeName(s)
	if isException {
		out.WriteString("(thrift:def-exception " + prefix(name) + "\n")
	} else {
		out.WriteString("(thrift:def-struct " + prefix(name) + "\n")
	}
	g.indentUp()
	if s.HasDoc() {
		out.WriteString(g.indent())
		out.WriteString("\"" + clDocstring(s.Doc()) + "\"" + "\n")
	}
	out.WriteString(g.indent())
	g.generateClStructInternal(out, s, isException)
	g.indentDown()
	out.WriteString(")" + "\n" + "\n")
}

// generateExceptionSig is t_cl_generator::generate_exception_sig.
func (g *Generator) generateExceptionSig(out *strings.Builder, f *sema.Function) {
	g.generateClStructInternal(out, f.Xceptions(), true)
}

// generateService is t_cl_generator::generate_service.
func (g *Generator) generateService(s *sema.Service) {
	extendsClient := ""
	if s.Extends() != nil {
		extendsClient = g.typeName(s.Extends())
	}
	if extendsClient == "" {
		extendsClient = "nil"
	} else {
		extendsClient = prefix(extendsClient)
	}

	g.fTypes.WriteString("(thrift:def-service " + prefix(s.Name()) + " " + extendsClient)

	g.indentUp()

	if s.HasDoc() {
		g.fTypes.WriteString("\n" + g.indent() + "(:documentation \"" + clDocstring(s.Doc()) + "\")")
	}

	for _, f := range s.Functions() {
		signature := g.functionSignature(f)
		xmembers := f.Xceptions().Members()

		g.fTypes.WriteString("\n" + g.indent() + "(:method " + prefix(f.Name()))
		g.fTypes.WriteString(" (" + signature + " " + g.typespec(f.ReturnType()) + ")")
		if len(xmembers) > 0 {
			g.fTypes.WriteString("\n" + g.indent() + " :exceptions ")
			g.generateExceptionSig(&g.fTypes, f)
		}
		if f.IsOneway() {
			g.fTypes.WriteString("\n" + g.indent() + " :oneway t")
		}
		if f.HasDoc() {
			g.fTypes.WriteString("\n" + g.indent() + " :documentation \"" + clDocstring(f.Doc()) + "\"")
		}
		g.fTypes.WriteString(")")
	}

	g.fTypes.WriteString(")" + "\n" + "\n")

	g.indentDown()
}

// typespec is t_cl_generator::typespec.
func (g *Generator) typespec(t sema.Type) string {
	t = sema.TrueType(t)

	switch {
	case t.IsBinary():
		return "binary"
	case t.IsBaseType():
		return g.typeName(t)
	case t.IsMap():
		m := t.(*sema.Map)
		return "(thrift:map " + g.typespec(m.KeyType()) + " " + g.typespec(m.ValType()) + ")"
	case t.IsStruct() || t.IsXception():
		return "(struct " + prefix(g.typeName(t)) + ")"
	case t.IsList():
		return "(thrift:list " + g.typespec(t.(*sema.List).ElemType()) + ")"
	case t.IsSet():
		return "(thrift:set " + g.typespec(t.(*sema.Set).ElemType()) + ")"
	case t.IsEnum():
		return "(enum \"" + t.Name() + "\")"
	default:
		emit.Throw("Sorry, I don't know how to generate this: %s", g.typeName(t))
		return ""
	}
}

// functionSignature is t_cl_generator::function_signature.
func (g *Generator) functionSignature(f *sema.Function) string {
	return g.argumentList(f.Arglist())
}

// argumentList is t_cl_generator::argument_list.
func (g *Generator) argumentList(s *sema.Struct) string {
	var res strings.Builder
	res.WriteString("(")

	first := true
	for _, f := range s.Members() {
		if first {
			first = false
		} else {
			res.WriteString(" ")
		}
		res.WriteString("(" + prefix(f.Name()) + " " + g.typespec(f.Type()) + " " + strconv.FormatInt(int64(f.Key()), 10) + ")")
	}
	res.WriteString(")")
	return res.String()
}

// typeName is t_cl_generator::type_name.
func (g *Generator) typeName(t sema.Type) string {
	pfx := ""
	if p := t.Program(); p != nil && p != g.program {
		if po := packageOf(p); po != g.pkg() {
			pfx = po + ":"
		}
	}

	name := t.Name()
	if t.IsStruct() || t.IsXception() {
		name = lowercase(t.Name())
	}

	return pfx + name
}

// displayName is t_cl_generator::display_name. It is not called from
// anywhere in the generator's own output, exactly as in the C++ source
// (validate_id, the only caller, is never reached since
// lang_keywords_for_validation is empty), but is kept for parity of the
// class's public surface.
func (g *Generator) displayName() string {
	return "Common Lisp"
}
