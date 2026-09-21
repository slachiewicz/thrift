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

// Package py generates Python code from a resolved Thrift program.
//
// It is a method-for-method port of
// compiler/cpp/src/thrift/generate/t_py_generator.cc: the emitter builds
// strings the way an ostream would, with an explicit indent level, rather
// than through templates, so that it writes exactly the bytes the C++
// generator writes. That is checked against the C++ compiler by the
// parity tests.
package py

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

// Error is a generator failure; the C++ generator throws a string.
type Error struct {
	Msg string
}

func (e *Error) Error() string { return e.Msg }

func throw(format string, args ...interface{}) {
	panic(&Error{Msg: fmt.Sprintf(format, args...)})
}

// Options are the "--gen py:" generator options, already fully derived the
// way the t_py_generator constructor's option loop derives its gen_*_
// member variables (including the interactions between options, such as
// "dynamic" implying "old_style" and filling in the dynbase class
// defaults).
type Options struct {
	NewStyle bool
	Enum     bool
	Dynamic  bool

	DynbaseClass          string
	DynbaseClassFrozen    string
	DynbaseClassExc       string
	DynbaseClassFrozenExc string
	ImportDynbase         string

	Slots     bool
	TypeHints bool

	ZopeInterface bool
	Twisted       bool
	Tornado       bool
	Utf8Strings   bool

	Coding        string
	PackagePrefix string
}

// ParseOptions parses the part after "py:" of a --gen argument and applies
// it the way the t_py_generator constructor does: options are collected
// into a map (so a repeated key keeps its last value, as an assignment
// into a std::map would) and then applied in sorted key order.
func ParseOptions(spec string) (Options, error) {
	o := Options{NewStyle: true, Utf8Strings: true}
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
	sortStrings(keys)
	for _, key := range keys {
		value := parsed[key]
		switch key {
		case "enum":
			o.Enum = true
		case "new_style":
			// Deprecated no-op: new_style is enabled by default.
		case "old_style":
			o.NewStyle = false
		case "utf8strings":
			// Deprecated no-op: utf8strings is enabled by default.
		case "no_utf8strings":
			o.Utf8Strings = false
		case "slots":
			o.Slots = true
		case "package_prefix":
			o.PackagePrefix = value
		case "dynamic":
			o.Dynamic = true
			o.NewStyle = false // dynamic is newstyle
			if o.DynbaseClass == "" {
				o.DynbaseClass = "TBase"
			}
			if o.DynbaseClassFrozen == "" {
				o.DynbaseClassFrozen = "TFrozenBase"
			}
			if o.DynbaseClassExc == "" {
				o.DynbaseClassExc = "TExceptionBase"
			}
			if o.DynbaseClassFrozenExc == "" {
				o.DynbaseClassFrozenExc = "TFrozenExceptionBase"
			}
			if o.ImportDynbase == "" {
				o.ImportDynbase = "from thrift.protocol.TBase import TBase, TFrozenBase, TExceptionBase, TFrozenExceptionBase, TTransport\n"
			}
		case "dynbase":
			o.DynbaseClass = value
		case "dynfrozen":
			o.DynbaseClassFrozen = value
		case "dynexc":
			o.DynbaseClassExc = value
		case "dynfrozenexc":
			o.DynbaseClassFrozenExc = value
		case "dynimport":
			o.ImportDynbase = value
		case "zope.interface":
			o.ZopeInterface = true
		case "twisted":
			o.Twisted = true
			o.ZopeInterface = true
		case "tornado":
			o.Tornado = true
		case "coding":
			o.Coding = value
		case "type_hints":
			if !o.Enum {
				return o, &emit.Error{Msg: "the type_hints py option requires the enum py option"}
			}
			o.TypeHints = true
		default:
			return o, &emit.Error{Msg: "unknown option py:" + key}
		}
	}
	if o.Twisted && o.Tornado {
		return o, &emit.Error{Msg: "at most one of 'twisted' and 'tornado' are allowed"}
	}
	return o, nil
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// pythonKeywords is python_keywords_ from the constructor.
var pythonKeywords = map[string]bool{
	"False": true, "None": true, "True": true, "and": true, "as": true,
	"assert": true, "break": true, "class": true, "continue": true, "def": true,
	"del": true, "elif": true, "else": true, "except": true, "exec": true,
	"finally": true, "for": true, "from": true, "global": true, "if": true,
	"import": true, "in": true, "is": true, "lambda": true, "nonlocal": true,
	"not": true, "or": true, "pass": true, "print": true, "raise": true,
	"return": true, "try": true, "while": true, "with": true, "yield": true,
}

// maybeEscapeIdentifier is maybe_escape_identifier.
func maybeEscapeIdentifier(identifier string) string {
	if pythonKeywords[identifier] {
		return identifier + "_"
	}
	return identifier
}

// isImmutable is the static is_immutable helper.
func isImmutable(t sema.Type) bool {
	values, ok := t.Annotations()["python.immutable"]
	if !ok {
		// Exceptions are immutable by default.
		return t.IsXception()
	}
	if len(values) > 0 && values[len(values)-1] == "false" {
		return false
	}
	return true
}

// Generator is t_py_generator for one program.
type Generator struct {
	program     *sema.Program
	opts        Options
	copyOptions string
	serviceName string

	ind        int
	tmpCounter int

	fTypes      strings.Builder
	fTypesName  string
	fConsts     strings.Builder
	fConstsName string
	fService    strings.Builder

	packageDir string
	module     string
}

// New creates a generator for the program. copyOptions is the full
// "lang:spec" text (option_string in the C++ constructor), used verbatim
// in the "options string:" line of the autogenerated comment.
func New(program *sema.Program, opts Options, copyOptions string) *Generator {
	return &Generator{program: program, opts: opts, copyOptions: copyOptions}
}

// Generate writes the Python code for the program. It is
// t_generator::generate_program restricted to what t_py_generator
// overrides.
func (g *Generator) Generate() (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*Error); ok {
				err = e
				return
			}
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
	objects := g.program.Objects()
	for _, o := range objects {
		g.generateForwardDeclaration(o)
	}
	for _, o := range objects {
		if o.IsXception() {
			g.generateXception(o)
		} else {
			g.generateStruct(o)
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
	return nil
}

// ---- indentation, matching t_generator's indent_/indent_str() ----

// indentStr is indent_str(): the py generator uses four spaces.
const indentStr = "    "

func (g *Generator) indentUp()   { g.ind++ }
func (g *Generator) indentDown() { g.ind-- }

// indent is indent(): the current indentation prefix.
func (g *Generator) indent() string { return strings.Repeat(indentStr, g.ind) }

// tmp is t_generator::tmp.
func (g *Generator) tmp(name string) string {
	s := name + strconv.Itoa(g.tmpCounter)
	g.tmpCounter++
	return s
}

// ---- files ----

func (g *Generator) outDirBase() string {
	if g.opts.Twisted {
		return "gen-py.twisted"
	}
	if g.opts.Tornado {
		return "gen-py.tornado"
	}
	return "gen-py"
}

// outDir is t_generator::get_out_dir.
func (g *Generator) outDir() string {
	if g.program.IsOutPathAbsolute() {
		return g.program.OutPath() + "/"
	}
	return g.program.OutPath() + g.outDirBase() + "/"
}

// touchInitPy opens dir/__init__.py for append, creating it if missing and
// leaving existing content untouched, like the ofstream opened with
// ios_base::app in init_generator's directory-building loop.
func (g *Generator) touchInitPy(dir string) {
	f, err := os.OpenFile(dir+"/__init__.py", os.O_APPEND|os.O_CREATE, 0o644)
	if err == nil {
		f.Close()
	}
}

// realPyModule is the static get_real_py_module helper.
func (g *Generator) realPyModule(program *sema.Program, packageDir string) string {
	if g.opts.Twisted {
		if tw := program.Namespace("py.twisted"); tw != "" {
			return tw
		}
	}
	real := program.Namespace("py")
	if real == "" {
		return program.Name()
	}
	return packageDir + real
}

func (g *Generator) initGenerator() {
	module := g.realPyModule(g.program, "")
	g.module = module
	g.packageDir = g.outDir()
	for {
		emit.Mkdir(g.packageDir)
		g.touchInitPy(g.packageDir)
		if module == "" {
			break
		}
		if pos := strings.IndexByte(module, '.'); pos < 0 {
			g.packageDir += "/" + module
			module = ""
		} else {
			g.packageDir += "/" + module[:pos]
			module = module[pos+1:]
		}
	}

	g.fTypesName = g.packageDir + "/ttypes.py"
	g.fConstsName = g.packageDir + "/constants.py"

	var fInit strings.Builder
	fInit.WriteString("__all__ = ['ttypes', 'constants'")
	for _, s := range g.program.Services() {
		fInit.WriteString(", '" + maybeEscapeIdentifier(s.Name()) + "'")
	}
	fInit.WriteString("]\n")
	emit.WriteFile(g.packageDir+"/__init__.py", fInit.String())

	g.fTypes.WriteString(g.pyAutogenComment() + "\n" + g.pyImports() + "\n" + g.renderIncludes() + "\n" +
		"from thrift.transport import TTransport" + "\n" + g.opts.ImportDynbase)
	if g.opts.TypeHints {
		g.fTypes.WriteString("all_structs: list[typing.Any] = []\n")
	} else {
		g.fTypes.WriteString("all_structs = []\n")
	}

	g.fConsts.WriteString(g.pyAutogenComment() + "\n" + g.pyImports() + "\n" + "from .ttypes import *\n")
}

func (g *Generator) closeGenerator() {
	g.fTypes.WriteString("fix_spec(all_structs)\n")
	g.fTypes.WriteString("del all_structs\n")

	emit.WriteFile(g.fTypesName, g.fTypes.String())
	emit.WriteFile(g.fConstsName, g.fConsts.String())
}

// renderIncludes is render_includes.
func (g *Generator) renderIncludes() string {
	var sb strings.Builder
	for _, inc := range g.program.Includes() {
		sb.WriteString("import " + g.realPyModule(inc, g.opts.PackagePrefix) + ".ttypes\n")
	}
	return sb.String()
}

// pyAutogenComment is py_autogen_comment.
func (g *Generator) pyAutogenComment() string {
	coding := ""
	if g.opts.Coding != "" {
		coding = "# -*- coding: " + g.opts.Coding + " -*-\n"
	}
	return coding + "#\n" + "# Autogenerated by Thrift Compiler (" + version.Version + ")\n" +
		"#\n" + "# DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING\n" + "#\n" +
		"#  options string: " + g.copyOptions + "\n" + "#\n"
}

// pyImports is py_imports.
func (g *Generator) pyImports() string {
	var sb strings.Builder
	if g.opts.TypeHints {
		sb.WriteString("from __future__ import annotations\n")
		sb.WriteString("import typing\n")
	}
	sb.WriteString("from thrift.Thrift import TType, TMessageType, TFrozenDict, TException, TApplicationException\n")
	sb.WriteString("from thrift.protocol.TProtocol import TProtocolException\n")
	sb.WriteString("from thrift.TRecursive import fix_spec\n")
	sb.WriteString("from uuid import UUID\n")
	if g.opts.Enum {
		sb.WriteString("from enum import IntEnum\n")
	}
	if g.opts.Utf8Strings {
		sb.WriteString("\nimport sys")
	}
	return sb.String()
}

// ---- program-level generation ----

// generateTypedef is a no-op: types are all implicit in Python.
func (g *Generator) generateTypedef(*sema.Typedef) {}

func (g *Generator) generateEnum(e *sema.Enum) {
	var toStringMapping, fromStringMapping strings.Builder
	baseClass := ""
	if g.opts.Enum {
		baseClass = "IntEnum"
	} else if g.opts.NewStyle {
		baseClass = "object"
	} else if g.opts.Dynamic {
		baseClass = g.opts.DynbaseClass
	}

	g.fTypes.WriteString("\n\nclass " + maybeEscapeIdentifier(e.Name()))
	if baseClass != "" {
		g.fTypes.WriteString("(" + baseClass + ")")
	}
	g.fTypes.WriteString(":\n")
	g.indentUp()
	g.generatePythonDocstring(&g.fTypes, e)

	toStringMapping.WriteString(g.indent() + "_VALUES_TO_NAMES = {\n")
	fromStringMapping.WriteString(g.indent() + "_NAMES_TO_VALUES = {\n")

	for _, c := range e.Constants() {
		value := strconv.FormatInt(int64(c.Value()), 10)
		name := maybeEscapeIdentifier(c.Name())
		g.fTypes.WriteString(g.indent() + name + " = " + value + "\n")

		toStringMapping.WriteString(g.indent() + g.indent() + value + ": \"" + emit.EscapeString(c.Name()) + "\",\n")
		fromStringMapping.WriteString(g.indent() + g.indent() + "\"" + emit.EscapeString(c.Name()) + "\": " + value + ",\n")
	}
	toStringMapping.WriteString(g.indent() + "}\n")
	fromStringMapping.WriteString(g.indent() + "}\n")

	g.indentDown()
	g.fTypes.WriteString("\n")
	if !g.opts.Enum {
		g.fTypes.WriteString(toStringMapping.String() + "\n" + fromStringMapping.String())
	}
}

func (g *Generator) generateConst(c *sema.Const) {
	name := maybeEscapeIdentifier(c.Name())
	g.fConsts.WriteString(g.indent() + name + " = " + g.renderConstValue(c.Type(), c.Value()))
	g.fConsts.WriteString("\n")
}

// renderConstValue is render_const_value. Type checking is not performed
// here, as in the C++ generator: it relies on validate_types having run
// beforehand.
func (g *Generator) renderConstValue(typ sema.Type, value *sema.ConstValue) string {
	typ = sema.TrueType(typ)
	var out strings.Builder

	switch {
	case typ.IsBaseType():
		bt := typ.(*sema.BaseType)
		switch bt.Base() {
		case sema.TypeString:
			if bt.IsBinary() {
				out.WriteString("b")
			}
			out.WriteString("\"" + emit.EscapeString(value.String()) + "\"")
		case sema.TypeBool:
			if value.Integer() > 0 {
				out.WriteString("True")
			} else {
				out.WriteString("False")
			}
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			out.WriteString(strconv.FormatInt(value.Integer(), 10))
		case sema.TypeDouble:
			if value.Kind() == sema.CVInteger {
				out.WriteString("float(" + strconv.FormatInt(value.Integer(), 10) + ")")
			} else {
				out.WriteString(emit.DoubleFixed16(value.Double()))
			}
		case sema.TypeUUID:
			out.WriteString("UUID(\"" + emit.EscapeString(value.String()) + "\")")
		default:
			throw("compiler error: no const of base type %s", sema.BaseName(bt.Base()))
		}
	case typ.IsEnum():
		out.WriteString(g.indent())
		intVal := value.Integer()
		if g.opts.Enum {
			en := typ.(*sema.Enum)
			enumVal := en.ConstantByValue(intVal)
			out.WriteString(g.typeName(typ) + "." + maybeEscapeIdentifier(enumVal.Name()))
		} else {
			out.WriteString(strconv.FormatInt(intVal, 10))
		}
	case typ.IsStruct() || typ.IsXception():
		out.WriteString(g.typeName(typ) + "(**{\n")
		g.indentUp()
		s := typ.(*sema.Struct)
		for _, e := range value.Map() {
			var fieldType sema.Type
			for _, f := range s.Members() {
				if f.Name() == e.Key.String() {
					fieldType = f.Type()
				}
			}
			if fieldType == nil {
				throw("type error: %s has no field %s", typ.Name(), e.Key.String())
			}
			out.WriteString(g.indent() + g.renderConstValue(sema.GlobalString, e.Key) + ": " + g.renderConstValue(fieldType, e.Value) + ",\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "})")
	case typ.IsMap():
		m := typ.(*sema.Map)
		immutable := isImmutable(typ)
		if immutable {
			out.WriteString("TFrozenDict(")
		}
		out.WriteString("{\n")
		g.indentUp()
		for _, e := range value.Map() {
			out.WriteString(g.indent() + g.renderConstValue(m.KeyType(), e.Key) + ": " + g.renderConstValue(m.ValType(), e.Value) + ",\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "}")
		if immutable {
			out.WriteString(")")
		}
	case typ.IsList() || typ.IsSet():
		var etype sema.Type
		isSet := typ.IsSet()
		if typ.IsList() {
			etype = typ.(*sema.List).ElemType()
		} else {
			etype = typ.(*sema.Set).ElemType()
		}
		immutable := isImmutable(typ)
		if isSet {
			if immutable {
				out.WriteString("frozen")
			}
			out.WriteString("set(")
		}
		if immutable || isSet {
			out.WriteString("(\n")
		} else {
			out.WriteString("[\n")
		}
		g.indentUp()
		for _, e := range value.List() {
			out.WriteString(g.indent() + g.renderConstValue(etype, e) + ",\n")
		}
		g.indentDown()
		if immutable || isSet {
			out.WriteString(g.indent() + ")")
		} else {
			out.WriteString(g.indent() + "]")
		}
		if isSet {
			out.WriteString(")")
		}
	default:
		throw("CANNOT GENERATE CONSTANT FOR TYPE: %s", typ.Name())
	}

	return out.String()
}

// ---- helper rendering functions ----

// typeName is type_name.
func (g *Generator) typeName(t sema.Type) string {
	for t.IsTypedef() {
		t = t.(*sema.Typedef).Type()
	}
	program := t.Program()
	if t.IsService() {
		return g.realPyModule(program, g.opts.PackagePrefix) + "." + maybeEscapeIdentifier(t.Name())
	}
	if program != nil && program != g.program {
		return g.realPyModule(program, g.opts.PackagePrefix) + ".ttypes." + maybeEscapeIdentifier(t.Name())
	}
	return maybeEscapeIdentifier(t.Name())
}

func (g *Generator) argHint(t sema.Type) string {
	if g.opts.TypeHints {
		return ": " + g.typeToPyType(t)
	}
	return ""
}

func (g *Generator) memberHint(t sema.Type, req sema.Requiredness) string {
	if g.opts.TypeHints {
		if req != sema.Required {
			return ": typing.Optional[" + g.typeToPyType(t) + "]"
		}
		return ": " + g.typeToPyType(t)
	}
	return ""
}

func (g *Generator) funcHint(t sema.Type) string {
	if g.opts.TypeHints {
		return " -> " + g.typeToPyType(t)
	}
	return ""
}

// typeToPyType is type_to_py_type.
func (g *Generator) typeToPyType(t sema.Type) string {
	t = sema.TrueType(t)
	if t.IsBinary() {
		return "bytes"
	}
	if t.IsBaseType() {
		bt := t.(*sema.BaseType)
		switch bt.Base() {
		case sema.TypeVoid:
			return "None"
		case sema.TypeString:
			return "str"
		case sema.TypeBool:
			return "bool"
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			return "int"
		case sema.TypeDouble:
			return "float"
		case sema.TypeUUID:
			return "UUID"
		}
	} else if t.IsEnum() || t.IsStruct() || t.IsXception() {
		return g.typeName(t)
	} else if t.IsMap() {
		m := t.(*sema.Map)
		return "dict[" + g.typeToPyType(m.KeyType()) + ", " + g.typeToPyType(m.ValType()) + "]"
	} else if t.IsSet() {
		return "set[" + g.typeToPyType(t.(*sema.Set).ElemType()) + "]"
	} else if t.IsList() {
		return "list[" + g.typeToPyType(t.(*sema.List).ElemType()) + "]"
	}
	throw("INVALID TYPE IN type_to_py_type: %s", t.Name())
	return ""
}

// typeToEnum is type_to_enum.
func (g *Generator) typeToEnum(t sema.Type) string {
	t = sema.TrueType(t)
	if t.IsBaseType() {
		bt := t.(*sema.BaseType)
		switch bt.Base() {
		case sema.TypeVoid:
			throw("NO T_VOID CONSTRUCT")
		case sema.TypeString:
			return "TType.STRING"
		case sema.TypeBool:
			return "TType.BOOL"
		case sema.TypeI8:
			return "TType.BYTE"
		case sema.TypeI16:
			return "TType.I16"
		case sema.TypeI32:
			return "TType.I32"
		case sema.TypeI64:
			return "TType.I64"
		case sema.TypeDouble:
			return "TType.DOUBLE"
		case sema.TypeUUID:
			return "TType.UUID"
		default:
			throw("compiler error: unhandled type")
		}
	} else if t.IsEnum() {
		return "TType.I32"
	} else if t.IsStruct() || t.IsXception() {
		return "TType.STRUCT"
	} else if t.IsMap() {
		return "TType.MAP"
	} else if t.IsSet() {
		return "TType.SET"
	} else if t.IsList() {
		return "TType.LIST"
	}
	throw("INVALID TYPE IN type_to_enum: %s", t.Name())
	return ""
}

// typeToSpecArgs is type_to_spec_args; see the comment inside
// generatePyStructDefinition for what this is.
func (g *Generator) typeToSpecArgs(t sema.Type) string {
	for t.IsTypedef() {
		t = t.(*sema.Typedef).Type()
	}

	imm := func(v sema.Type) string {
		if isImmutable(v) {
			return "True"
		}
		return "False"
	}

	if t.IsBinary() {
		return "'BINARY'"
	} else if g.opts.Utf8Strings && t.IsBaseType() && t.IsString() {
		return "'UTF8'"
	} else if t.IsBaseType() || t.IsEnum() {
		return "None"
	} else if t.IsStruct() || t.IsXception() {
		return "[" + maybeEscapeIdentifier(g.typeName(t)) + ", None]"
	} else if t.IsMap() {
		m := t.(*sema.Map)
		return "(" + g.typeToEnum(m.KeyType()) + ", " + g.typeToSpecArgs(m.KeyType()) + ", " +
			g.typeToEnum(m.ValType()) + ", " + g.typeToSpecArgs(m.ValType()) + ", " + imm(t) + ")"
	} else if t.IsSet() {
		s := t.(*sema.Set)
		return "(" + g.typeToEnum(s.ElemType()) + ", " + g.typeToSpecArgs(s.ElemType()) + ", " + imm(t) + ")"
	} else if t.IsList() {
		l := t.(*sema.List)
		return "(" + g.typeToEnum(l.ElemType()) + ", " + g.typeToSpecArgs(l.ElemType()) + ", " + imm(t) + ")"
	}
	throw("INVALID TYPE IN type_to_spec_args: %s", t.Name())
	return ""
}

// declareArgument is declare_argument.
func (g *Generator) declareArgument(f *sema.Field) string {
	result := maybeEscapeIdentifier(f.Name()) + g.memberHint(f.Type(), f.Req())
	result += " = "
	if f.Value() != nil {
		result += g.renderFieldDefaultValue(f)
	} else {
		result += "None"
	}
	return result
}

// renderFieldDefaultValue is render_field_default_value.
func (g *Generator) renderFieldDefaultValue(f *sema.Field) string {
	typ := sema.TrueType(f.Type())
	if f.Value() != nil {
		return g.renderConstValue(typ, f.Value())
	}
	return "None"
}

// functionSignature is function_signature. It takes the pieces of a
// t_function directly rather than a *sema.Function so that it can also
// render the client's synthetic "recv_xxx" signature, which the C++
// generator builds from a throwaway t_function of its own.
func (g *Generator) functionSignature(name string, arglist *sema.Struct, returnType sema.Type, interfaceFlag, sendPart bool) string {
	var pre, post []string
	signature := maybeEscapeIdentifier(name) + "("

	if !(g.opts.ZopeInterface && interfaceFlag) {
		pre = append(pre, "self")
	}

	signature += g.argumentList(arglist, pre, post) + ")"
	if !sendPart {
		signature += g.funcHint(returnType)
	}

	return signature
}

// argumentList is argument_list.
func (g *Generator) argumentList(s *sema.Struct, pre, post []string) string {
	var result strings.Builder
	first := true
	for _, p := range pre {
		if first {
			first = false
		} else {
			result.WriteString(", ")
		}
		result.WriteString(p)
	}
	for _, f := range s.Members() {
		if first {
			first = false
		} else {
			result.WriteString(", ")
		}
		result.WriteString(maybeEscapeIdentifier(f.Name()) + g.argHint(f.Type()))
	}
	for _, p := range post {
		if first {
			first = false
		} else {
			result.WriteString(", ")
		}
		result.WriteString(p)
	}
	return result.String()
}

// ---- docstrings ----

type docHaver interface {
	HasDoc() bool
	Doc() string
}

func (g *Generator) generatePythonDocstringStruct(out *strings.Builder, s *sema.Struct) {
	g.generatePythonDocstringFor(out, s, s, "Attributes")
}

func (g *Generator) generatePythonDocstringFunction(out *strings.Builder, f *sema.Function) {
	g.generatePythonDocstringFor(out, f, f.Arglist(), "Parameters")
}

func (g *Generator) generatePythonDocstringFor(out *strings.Builder, doc docHaver, s *sema.Struct, subheader string) {
	hasDoc := false
	var ss strings.Builder
	if doc.HasDoc() {
		hasDoc = true
		ss.WriteString(doc.Doc())
	}

	fields := s.Members()
	if len(fields) > 0 {
		if hasDoc {
			ss.WriteString("\n")
		}
		hasDoc = true
		ss.WriteString(subheader + ":\n")
		for _, p := range fields {
			ss.WriteString(" - " + p.Name())
			if p.HasDoc() {
				ss.WriteString(": " + p.Doc())
			} else {
				ss.WriteString("\n")
			}
		}
	}

	if hasDoc {
		g.pyDocstringComment(out, "\"\"\"\n", "", ss.String(), "\"\"\"\n")
	}
}

// generatePythonDocstring is the generic, one-argument overload.
func (g *Generator) generatePythonDocstring(out *strings.Builder, doc docHaver) {
	if doc.HasDoc() {
		g.pyDocstringComment(out, "\"\"\"\n", "", doc.Doc(), "\"\"\"\n")
	}
}

// pyDocstringComment is generate_docstring_comment, ported precisely
// enough to reproduce a real quirk of the C++ getline loop it drives: when
// contents ends with a newline and linePrefix is empty, the loop runs one
// extra iteration that hits end-of-file with zero characters extracted
// (std::istream::getline still sets an empty buffer in that case) and,
// because the empty-prefix branch is not gated on eof, prints one more
// bare blank line before the closing comment. emit.DocstringComment does
// not reproduce this, because it stops as soon as its remaining-string
// slice becomes empty; the generators that share it never call it with an
// empty linePrefix and trailing-newline content the way this one does.
func (g *Generator) pyDocstringComment(out *strings.Builder, commentStart, linePrefix, contents, commentEnd string) {
	if commentStart != "" {
		out.WriteString(g.indent() + commentStart)
	}
	pos := 0
	n := len(contents)
	eof := false
	for {
		if pos >= n {
			// getline() finds nothing left to extract: it sets both
			// eofbit and failbit but still leaves an empty buffer, and
			// that iteration's body still runs once before the loop
			// condition is re-checked.
			if linePrefix == "" {
				out.WriteString("\n")
			}
			break
		}
		nl := strings.IndexByte(contents[pos:], '\n')
		var line string
		truncate := false
		switch {
		case nl >= 0 && nl < 1023:
			line = contents[pos : pos+nl]
			pos = pos + nl + 1
		case nl < 0 && n-pos < 1024:
			line = contents[pos:]
			pos = n
			eof = true // ran out of input while searching for the delimiter
		default:
			line = contents[pos : pos+1023]
			pos += 1023
			truncate = true
		}
		if len(line) > 0 {
			out.WriteString(g.indent() + linePrefix + line + "\n")
		} else if linePrefix == "" {
			out.WriteString("\n")
		} else if !eof {
			out.WriteString(g.indent() + linePrefix + "\n")
		}
		if truncate || eof {
			break
		}
	}
	if commentEnd != "" {
		out.WriteString(g.indent() + commentEnd)
	}
}
