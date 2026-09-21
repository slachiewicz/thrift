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

// Package erl is a method-for-method port of
// compiler/cpp/src/thrift/generate/t_erl_generator.cc. It writes the same
// bytes as the C++ generator, which is checked against the C++ compiler by
// the parity tests; that is why the emitter builds strings the way an
// ostream would rather than through templates, and reproduces C++ quirks
// (including two places where the C++ source calls indent_down() with no
// matching indent_up(), permanently shifting the shared indent level for
// the rest of the file: see the comments on renderConstValue's struct
// branch and generateConstFunction's list branch).
package erl

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

// Generator is t_erl_generator for one program.
type Generator struct {
	program *sema.Program
	opts    Options

	programName string
	serviceName string

	// indentLevel is indent_: the current code indentation level. It is
	// shared mutable state across the whole generation of the program,
	// exactly like the C++ member, including the two places that leave it
	// permanently shifted (see the package doc).
	indentLevel int

	exportLinesFirst bool
	exportLines      strings.Builder

	exportTypesLinesFirst bool
	exportTypesLines      strings.Builder

	fInfo    strings.Builder
	fInfoExt strings.Builder

	fTypesFile     strings.Builder
	fTypesName     string
	fTypesHrlFile  strings.Builder
	fTypesHrlName  string
	fConstsFile    strings.Builder
	fConstsName    string
	fConstsHrlFile strings.Builder
	fConstsHrlName string

	fService strings.Builder

	vStructNames    []string
	vEnumNames      []string
	vExceptionNames []string
	vEnums          []*sema.Enum
	vConsts         []*sema.Const
}

// New creates a generator for the program.
func New(program *sema.Program, opts Options) *Generator {
	return &Generator{program: program, opts: opts, programName: program.Name()}
}

// Generate writes the Erlang code for the program. It is
// t_generator::generate_program restricted to the calls the Erlang
// generator overrides.
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
		g.serviceName = s.Name()
		g.generateService(s)
	}
	g.closeGenerator()
	return nil
}

// ---- t_generator helpers ----

func (g *Generator) indentUp()   { g.indentLevel++ }
func (g *Generator) indentDown() { g.indentLevel-- }

// indent is t_generator::indent(): the cumulative indentation string for
// the current level. A negative level (see the package doc) yields "",
// exactly like the C++ loop "for (i = 0; i < indent_; ++i)".
func (g *Generator) indent() string {
	var sb strings.Builder
	for i := 0; i < g.indentLevel; i++ {
		sb.WriteString("  ")
	}
	return sb.String()
}

// outDir is t_generator::get_out_dir with out_dir_base_ == "gen-erl".
func (g *Generator) outDir() string {
	if g.program.IsOutPathAbsolute() {
		return g.program.OutPath() + "/"
	}
	return g.program.OutPath() + "gen-erl" + "/"
}

// ---- naming helpers (t_generator::capitalize/decapitalize/lowercase/
// uppercase/underscore) ----

func isUpper(c byte) bool { return c >= 'A' && c <= 'Z' }
func isLower(c byte) bool { return c >= 'a' && c <= 'z' }

func toUpperByte(c byte) byte {
	if isLower(c) {
		return c - 'a' + 'A'
	}
	return c
}

func toLowerByte(c byte) byte {
	if isUpper(c) {
		return c - 'A' + 'a'
	}
	return c
}

// capitalize is t_generator::capitalize; guarded for the empty string,
// which the C++ version would index out of bounds.
func capitalize(in string) string {
	if in == "" {
		return in
	}
	b := []byte(in)
	b[0] = toUpperByte(b[0])
	return string(b)
}

// decapitalize is t_generator::decapitalize.
func decapitalize(in string) string {
	if in == "" {
		return in
	}
	b := []byte(in)
	b[0] = toLowerByte(b[0])
	return string(b)
}

// lowercase is t_generator::lowercase.
func lowercase(in string) string {
	b := []byte(in)
	for i := range b {
		b[i] = toLowerByte(b[i])
	}
	return string(b)
}

// uppercase is t_generator::uppercase.
func uppercase(in string) string {
	b := []byte(in)
	for i := range b {
		b[i] = toUpperByte(b[i])
	}
	return string(b)
}

// underscore is t_generator::underscore: aMultiWord -> a_multi_word.
func underscore(in string) string {
	b := []byte(in)
	if len(b) > 0 {
		b[0] = toLowerByte(b[0])
	}
	var out []byte
	for i, c := range b {
		if i > 0 && isUpper(c) {
			out = append(out, '_', toLowerByte(c))
		} else {
			out = append(out, c)
		}
	}
	return string(out)
}

// ---- init and close ----

// initGenerator is t_erl_generator::init_generator.
func (g *Generator) initGenerator() {
	outDir := g.outDir()
	emit.Mkdir(outDir)

	g.exportLinesFirst = true
	g.exportTypesLinesFirst = true

	programModuleName := g.makeSafeForModuleName(g.programName)

	g.fTypesName = outDir + programModuleName + "_types.erl"
	g.fTypesHrlName = outDir + programModuleName + "_types.hrl"

	g.hrlHeader(&g.fTypesHrlFile, programModuleName+"_types")

	g.fTypesFile.WriteString(g.erlAutogenComment() + "\n" +
		"-module(" + programModuleName + "_types)." + "\n" +
		g.erlImports() + "\n")

	g.fTypesFile.WriteString("-include(\"" + programModuleName + "_types.hrl\")." + "\n" + "\n")

	g.fTypesHrlFile.WriteString(g.renderIncludes() + "\n")

	g.fConstsName = outDir + programModuleName + "_constants.erl"
	g.fConstsHrlName = outDir + programModuleName + "_constants.hrl"

	g.fConstsFile.WriteString(g.erlAutogenComment() + "\n" +
		"-module(" + programModuleName + "_constants)." + "\n" +
		g.erlImports() + "\n" +
		"-include(\"" + programModuleName + "_types.hrl\")." + "\n" + "\n")

	g.fConstsHrlFile.WriteString(g.erlAutogenComment() + "\n" + g.erlImports() + "\n" +
		"-include(\"" + programModuleName + "_types.hrl\")." + "\n" + "\n")
}

// hrlHeader is t_erl_generator::hrl_header: boilerplate at the beginning of
// a header file.
func (g *Generator) hrlHeader(out *strings.Builder, name string) {
	out.WriteString(g.erlAutogenComment() + "\n" +
		"-ifndef(_" + name + "_included)." + "\n" +
		"-define(_" + name + "_included, yeah)." + "\n")
}

// hrlFooter is t_erl_generator::hrl_footer.
func (g *Generator) hrlFooter(out *strings.Builder) {
	out.WriteString("-endif." + "\n")
}

// renderIncludes is t_erl_generator::render_includes: the imports for
// including another Thrift program.
func (g *Generator) renderIncludes() string {
	var sb strings.Builder
	includes := g.program.Includes()
	for _, inc := range includes {
		sb.WriteString("-include(\"" + g.makeSafeForModuleName(inc.Name()) + "_types.hrl\").\n")
	}
	if len(includes) > 0 {
		sb.WriteString("\n")
	}
	return sb.String()
}

// erlAutogenComment is t_erl_generator::erl_autogen_comment.
func (g *Generator) erlAutogenComment() string {
	return "%%\n" +
		"%% Autogenerated by Thrift Compiler (" + version.Version + ")\n" +
		"%%\n" +
		"%% DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING\n" +
		"%%\n"
}

// comment is t_erl_generator::comment. It is not called from anywhere in
// the generator's own output, exactly as in the C++ source, but is kept for
// parity of the class's public surface.
func comment(in string) string {
	out := "%% " + in
	pos := len("%% ")
	for {
		idx := strings.IndexByte(out[pos:], '\n')
		if idx < 0 {
			break
		}
		insertAt := pos + idx + 1
		out = out[:insertAt] + "%% " + out[insertAt:]
		pos = insertAt + len("%% ")
	}
	return out
}

// erlImports is t_erl_generator::erl_imports: it always prints standard
// thrift imports, which is to say none.
func (g *Generator) erlImports() string {
	return ""
}

// closeGenerator is t_erl_generator::close_generator: it closes the type
// files.
func (g *Generator) closeGenerator() {
	g.exportTypesString("struct_info", 1)
	g.exportTypesString("struct_info_ext", 1)
	g.exportTypesString("enum_info", 1)
	g.exportTypesString("enum_names", 0)
	g.exportTypesString("struct_names", 0)
	g.exportTypesString("exception_names", 0)

	g.fTypesFile.WriteString("-export([" + g.exportTypesLines.String() + "])." + "\n" + "\n")

	g.fTypesFile.WriteString(g.fInfo.String())
	g.fTypesFile.WriteString("struct_info(_) -> erlang:error(function_clause)." + "\n" + "\n")

	g.fTypesFile.WriteString(g.fInfoExt.String())
	g.fTypesFile.WriteString("struct_info_ext(_) -> erlang:error(function_clause)." + "\n" + "\n")

	g.generateConstFunctions()

	g.generateTypeMetadata("struct_names", g.vStructNames)
	g.generateEnumMetadata()
	g.generateTypeMetadata("enum_names", g.vEnumNames)
	g.generateTypeMetadata("exception_names", g.vExceptionNames)

	g.hrlFooter(&g.fTypesHrlFile)

	emit.WriteFile(g.fTypesName, g.fTypesFile.String())
	emit.WriteFile(g.fTypesHrlName, g.fTypesHrlFile.String())
	emit.WriteFile(g.fConstsName, g.fConstsFile.String())
	emit.WriteFile(g.fConstsHrlName, g.fConstsHrlFile.String())
}

// generateTypeMetadata is t_erl_generator::generate_type_metadata.
func (g *Generator) generateTypeMetadata(functionName string, names []string) {
	numStructs := len(names)

	g.fTypesFile.WriteString(g.indent() + functionName + "() ->\n")
	g.indentUp()
	g.fTypesFile.WriteString(g.indent() + "[")

	for i := 0; i < numStructs; i++ {
		g.fTypesFile.WriteString(names[i])
		if i < numStructs-1 {
			g.fTypesFile.WriteString(", ")
		}
	}

	g.fTypesFile.WriteString("].\n\n")
	g.indentDown()
}

// generateTypedef is t_erl_generator::generate_typedef.
func (g *Generator) generateTypedef(t *sema.Typedef) {
	g.fTypesHrlFile.WriteString(g.renderTypeDeclaration() + g.typeName(t) +
		"() :: " + g.renderTypedefType(t) + ".\n" + "\n")
}

// generateConstFunction is t_erl_generator::generate_const_function.
func (g *Generator) generateConstFunction(tconst *sema.Const, exports, functions *strings.Builder) {
	typ := sema.TrueType(tconst.Type())
	name := tconst.Name()
	value := tconst.Value()

	if typ.IsMap() {
		m := typ.(*sema.Map)
		ktype, vtype := m.KeyType(), m.ValType()
		constFunName := lowercase(name)

		// Emit const function export.
		if exports.Len() > 0 {
			exports.WriteString(", ")
		}
		exports.WriteString(constFunName + "/1, " + constFunName + "/2")

		// Emit const function definition.
		entries := value.Map()
		// The one-argument form throws an error if the key does not exist in the map.
		for i, e := range entries {
			functions.WriteString(constFunName + "(" + g.renderConstValue(ktype, e.Key) + ") -> " +
				g.renderConstValue(vtype, e.Value))
			if i != len(entries)-1 {
				functions.WriteString(";\n")
			} else {
				functions.WriteString(".\n\n")
			}
		}

		// The two-argument form returns a default value if the key does not exist in the map.
		for _, e := range entries {
			functions.WriteString(constFunName + "(" + g.renderConstValue(ktype, e.Key) + ", _) -> " +
				g.renderConstValue(vtype, e.Value) + ";\n")
		}
		functions.WriteString(constFunName + "(_, Default) -> Default.\n\n")
	} else if typ.IsList() {
		constFunName := lowercase(name)

		if exports.Len() > 0 {
			exports.WriteString(", ")
		}
		exports.WriteString(constFunName + "/1, " + constFunName + "/2")

		listSize := len(value.List())
		renderedList := g.renderConstListValues(typ, value)
		functions.WriteString(constFunName + "(N) when N >= 1, N =< " + strconv.Itoa(listSize) + " ->\n" +
			"  " + "element(N, {" + renderedList + "}).\n")
		functions.WriteString(constFunName + "(N, _) when N >= 1, N =< " + strconv.Itoa(listSize) + " ->\n" +
			"  " + "element(N, {" + renderedList + "});\n" +
			constFunName + "(_, Default) -> Default.\n\n")
		// Quirk (t_erl_generator.cc): indent_down() with no matching
		// indent_up() in this branch, permanently shifting the shared
		// indent level for everything generated after this constant.
		g.indentDown()
	}
}

// generateConstFunctions is t_erl_generator::generate_const_functions.
func (g *Generator) generateConstFunctions() {
	var exports, functions strings.Builder
	for _, c := range g.vConsts {
		g.generateConstFunction(c, &exports, &functions)
	}
	if exports.Len() > 0 {
		g.fConstsFile.WriteString("-export([" + exports.String() + "]).\n\n" + functions.String())
	}
}

// generateEnum is t_erl_generator::generate_enum: generates code for an
// enumerated type.
func (g *Generator) generateEnum(e *sema.Enum) {
	constants := e.Constants()

	g.vEnums = append(g.vEnums, e)
	g.vEnumNames = append(g.vEnumNames, g.atomify(e.Name()))

	g.fTypesHrlFile.WriteString("%% enum " + e.Name() + "\n" + "\n")

	var constNames []string
	for _, c := range constants {
		value := c.Value()
		name := c.Name()
		constName := g.renderConstName2(e.Name(), name)
		g.fTypesHrlFile.WriteString(g.indent() + "-define(" + constName + ", " + strconv.Itoa(int(value)) + ")." + "\n")
		constNames = append(constNames, constName)
	}
	g.fTypesHrlFile.WriteString("\n")

	enumDefinition := g.renderTypeDeclaration() + g.typeName(e) + "() :: "
	valueIndent := strings.Repeat(" ", len(enumDefinition))
	g.fTypesHrlFile.WriteString(enumDefinition)
	namesIterFirst := false
	for _, cn := range constNames {
		if namesIterFirst {
			g.fTypesHrlFile.WriteString(" |" + "\n" + valueIndent)
		} else {
			namesIterFirst = true
		}
		g.fTypesHrlFile.WriteString(g.indent() + "?" + cn)
	}
	g.fTypesHrlFile.WriteString(".\n" + "\n")
}

// renderConstName is the one-argument t_erl_generator::render_const_name.
func (g *Generator) renderConstName(name string) string {
	return g.constify(g.makeSafeForModuleName(g.programName)) + "_" + g.constify(name)
}

// renderConstName2 is the two-argument t_erl_generator::render_const_name.
func (g *Generator) renderConstName2(sname, name string) string {
	return g.renderConstName(sname) + "_" + g.constify(name)
}

// generateEnumInfo is t_erl_generator::generate_enum_info.
func (g *Generator) generateEnumInfo(e *sema.Enum) {
	constants := e.Constants()
	numConstants := len(constants)

	g.fTypesFile.WriteString(g.indent() + "enum_info(" + g.atomify(e.Name()) + ") ->\n")
	g.indentUp()
	g.fTypesFile.WriteString(g.indent() + "[\n")

	for i := 0; i < numConstants; i++ {
		g.indentUp()
		value := constants[i]
		g.fTypesFile.WriteString(g.indent() + "{" + g.atomify(value.Name()) + ", " + strconv.Itoa(int(value.Value())) + "}")
		if i < numConstants-1 {
			g.fTypesFile.WriteString(",\n")
		}
		g.indentDown()
	}
	g.fTypesFile.WriteString("\n")
	g.fTypesFile.WriteString(g.indent() + "];\n\n")
	g.indentDown()
}

// generateEnumMetadata is t_erl_generator::generate_enum_metadata.
func (g *Generator) generateEnumMetadata() {
	for _, e := range g.vEnums {
		g.generateEnumInfo(e)
	}
	g.fTypesFile.WriteString(g.indent() + "enum_info(_) -> erlang:error(function_clause).\n\n")
}

// generateConst is t_erl_generator::generate_const: generates a constant
// value.
func (g *Generator) generateConst(tconst *sema.Const) {
	typ := tconst.Type()
	name := tconst.Name()
	value := tconst.Value()

	// Save the tconst so that function can be emitted in generateConstFunctions.
	g.vConsts = append(g.vConsts, tconst)

	g.fConstsHrlFile.WriteString("-define(" + g.renderConstName(name) + ", " +
		g.renderConstValue(typ, value) + ")." + "\n" + "\n")
}

// renderConstValue is t_erl_generator::render_const_value. Note that type
// checking is NOT performed in this function, as it is always run
// beforehand using validateInput.
func (g *Generator) renderConstValue(typ sema.Type, value *sema.ConstValue) string {
	typ = sema.TrueType(typ)
	var out strings.Builder

	if typ.IsBaseType() {
		b := typ.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeString:
			out.WriteString(g.renderConstStringValue(value))
		case sema.TypeBool:
			if value.Integer() > 0 {
				out.WriteString("true")
			} else {
				out.WriteString("false")
			}
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			out.WriteString(strconv.FormatInt(value.Integer(), 10))
		case sema.TypeDouble:
			if value.Kind() == sema.CVInteger {
				out.WriteString("float(" + strconv.FormatInt(value.Integer(), 10) + ")")
			} else {
				out.WriteString(emit.DoubleFixed16(value.Double()))
			}
		default:
			emit.Throw("compiler error: no const of base type %s", sema.BaseName(b.Base()))
		}
	} else if typ.IsEnum() {
		e := typ.(*sema.Enum)
		ev := e.ConstantByValue(value.Integer())
		out.WriteString(g.indent() + "?" + g.renderConstName2(typ.Name(), ev.Name()))
	} else if typ.IsStruct() || typ.IsXception() {
		s := typ.(*sema.Struct)
		out.WriteString("#" + g.typeName(typ) + "{")
		fields := s.Members()
		val := value.Map()

		first := true
		for _, e := range val {
			var fieldType sema.Type
			for _, f := range fields {
				if f.Name() == e.Key.String() {
					fieldType = f.Type()
				}
			}
			if fieldType == nil {
				emit.Throw("type error: %s has no field %s", typ.Name(), e.Key.String())
			}

			if first {
				first = false
			} else {
				out.WriteString(",")
			}
			out.WriteString(e.Key.String())
			out.WriteString(" = ")
			out.WriteString(g.renderConstValue(fieldType, e.Value))
		}
		// Quirk (t_erl_generator.cc): indent_down() with no matching
		// indent_up() in this branch, permanently shifting the shared
		// indent level for everything generated after this constant.
		g.indentDown()
		out.WriteString(g.indent() + "}")

	} else if typ.IsMap() {
		m := typ.(*sema.Map)
		ktype, vtype := m.KeyType(), m.ValType()

		if g.opts.Maps {
			out.WriteString("maps:from_list([")
		} else {
			out.WriteString("dict:from_list([")
		}
		entries := value.Map()
		for i, e := range entries {
			out.WriteString("{" + g.renderConstValue(ktype, e.Key) + "," + g.renderConstValue(vtype, e.Value) + "}")
			if i != len(entries)-1 {
				out.WriteString(",")
			}
		}
		out.WriteString("])")
	} else if typ.IsSet() {
		s := typ.(*sema.Set)
		etype := s.ElemType()
		out.WriteString("sets:from_list([")
		list := value.List()
		for i, v := range list {
			out.WriteString(g.renderConstValue(etype, v))
			if i != len(list)-1 {
				out.WriteString(",")
			}
		}
		out.WriteString("]")
		if g.opts.SetsTo == SetsToV2 {
			out.WriteString(", [{version, 2}]")
		}
		out.WriteString(")")
	} else if typ.IsList() {
		out.WriteString("[" + g.renderConstListValues(typ, value) + "]")
	} else {
		emit.Throw("CANNOT GENERATE CONSTANT FOR TYPE: %s", typ.Name())
	}
	return out.String()
}

// renderConstListValues is t_erl_generator::render_const_list_values. typ
// must already be the true type of a list, as at both call sites.
func (g *Generator) renderConstListValues(typ sema.Type, value *sema.ConstValue) string {
	var out strings.Builder
	etype := typ.(*sema.List).ElemType()

	first := true
	for _, v := range value.List() {
		if first {
			first = false
		} else {
			out.WriteString(",")
		}
		out.WriteString(g.renderConstValue(etype, v))
	}
	return out.String()
}

// renderConstStringValue is t_erl_generator::render_const_string_value.
func (g *Generator) renderConstStringValue(value *sema.ConstValue) string {
	if g.opts.StringTo == StringToBinary {
		return "<<\"" + emit.EscapeString(value.String()) + "\">>"
	}
	return "\"" + emit.EscapeString(value.String()) + "\""
}

// renderDefaultValue is t_erl_generator::render_default_value(t_field*). It
// deliberately checks field.Type() rather than its true type, exactly as
// the C++ source does: a required field whose declared type is a typedef of
// a container renders as "undefined", not the container's zero value.
func (g *Generator) renderDefaultValue(f *sema.Field) string {
	typ := f.Type()
	if typ.IsStruct() || typ.IsXception() {
		return "#" + g.typeName(typ) + "{}"
	} else if typ.IsMap() {
		if g.opts.Maps {
			return "#{}"
		}
		return "dict:new()"
	} else if typ.IsSet() {
		return g.renderDefaultSetsValue()
	} else if typ.IsList() {
		return "[]"
	}
	return "undefined"
}

// renderDefaultSetsValue is t_erl_generator::render_default_sets_value.
func (g *Generator) renderDefaultSetsValue() string {
	switch g.opts.SetsTo {
	case SetsToV1:
		return "sets:new()"
	case SetsToV2:
		return "sets:new([{version,2}])"
	default:
		emit.Throw("compiler error: unsupported set type")
		return ""
	}
}

// renderMemberType is t_erl_generator::render_member_type.
func (g *Generator) renderMemberType(f *sema.Field) string {
	return g.renderType(f.Type())
}

// renderType is t_erl_generator::render_type.
func (g *Generator) renderType(typ sema.Type) string {
	if typ.IsBaseType() {
		return g.renderBaseType(typ)
	} else if typ.IsEnum() {
		return g.typeName(typ) + "()"
	} else if typ.IsStruct() || typ.IsXception() || typ.IsTypedef() {
		return g.typeName(typ) + "()"
	} else if typ.IsMap() {
		if g.opts.Maps {
			return "maps:map()"
		}
		return "dict:dict()"
	} else if typ.IsSet() {
		return "sets:set()"
	} else if typ.IsList() {
		return "list()"
	}
	emit.Throw("compiler error: unsupported type %s", typ.Name())
	return ""
}

// renderBaseType is t_erl_generator::render_base_type. There is no case for
// uuid, exactly as in the C++ source: a uuid field makes generation fail.
func (g *Generator) renderBaseType(typ sema.Type) string {
	b := typ.(*sema.BaseType)
	switch b.Base() {
	case sema.TypeString:
		return g.renderStringType()
	case sema.TypeBool:
		return "boolean()"
	case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
		return "integer()"
	case sema.TypeDouble:
		return "float()"
	default:
		emit.Throw("compiler error: unsupported base type %s", sema.BaseName(b.Base()))
		return ""
	}
}

// renderStringType is t_erl_generator::render_string_type.
func (g *Generator) renderStringType() string {
	switch g.opts.StringTo {
	case StringToString:
		return "string()"
	case StringToBinary:
		return "binary()"
	case StringToBoth:
		return "string() | binary()"
	default:
		emit.Throw("compiler error: unsupported string type")
		return ""
	}
}

// renderTypeDeclaration is t_erl_generator::render_type_declaration.
func (g *Generator) renderTypeDeclaration() string {
	switch g.opts.TypeDeclaration {
	case TypeDeclType:
		return "-type "
	case TypeDeclNominal:
		return "-nominal "
	default:
		emit.Throw("compiler error: unsupported type declaration")
		return ""
	}
}

// renderMemberRequiredness is t_erl_generator::render_member_requiredness.
func (g *Generator) renderMemberRequiredness(f *sema.Field) string {
	switch f.Req() {
	case sema.Required:
		return "required"
	case sema.Optional:
		return "optional"
	default:
		return "undefined"
	}
}

// renderTypedefType is t_erl_generator::render_typedef_type.
func (g *Generator) renderTypedefType(t *sema.Typedef) string {
	return g.renderType(t.Type())
}

// generateStruct is t_erl_generator::generate_struct.
func (g *Generator) generateStruct(s *sema.Struct) {
	g.vStructNames = append(g.vStructNames, g.typeName(s))
	g.generateErlStruct(s, false)
}

// generateXception is t_erl_generator::generate_xception: generates a
// struct definition for a thrift exception. Basically the same as a
// struct but extends the Exception class.
func (g *Generator) generateXception(s *sema.Struct) {
	g.vExceptionNames = append(g.vExceptionNames, g.typeName(s))
	g.generateErlStruct(s, true)
}

// generateErlStruct is t_erl_generator::generate_erl_struct.
func (g *Generator) generateErlStruct(s *sema.Struct, isException bool) {
	_ = isException // (void)is_exception in the C++ source
	g.generateErlStructDefinition(&g.fTypesHrlFile, s)
	g.generateErlStructInfo(&g.fInfo, s)
	g.generateErlExtendedStructInfo(&g.fInfoExt, s)
}

// generateErlStructDefinition is
// t_erl_generator::generate_erl_struct_definition.
func (g *Generator) generateErlStructDefinition(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "%% ")
	if s.IsUnion() {
		out.WriteString("union ")
	} else if s.IsXception() {
		out.WriteString("exception ")
	} else {
		out.WriteString("struct ")
	}
	out.WriteString(s.Name() + "\n" + "\n")

	var buf strings.Builder
	buf.WriteString(g.indent() + "-record(" + g.typeName(s) + ", {")
	fieldIndent := strings.Repeat(" ", buf.Len())

	members := s.Members()
	for i, m := range members {
		g.generateErlStructMember(&buf, m)
		if i != len(members)-1 {
			buf.WriteString("," + "\n" + fieldIndent)
		}
	}
	buf.WriteString("}).")

	out.WriteString(buf.String() + "\n")
	out.WriteString(g.renderTypeDeclaration() + g.typeName(s) + "() :: #" + g.typeName(s) + "{}." + "\n" + "\n")
}

// generateErlStructMember is t_erl_generator::generate_erl_struct_member:
// generates the record field definition.
func (g *Generator) generateErlStructMember(out *strings.Builder, m *sema.Field) {
	out.WriteString(g.atomify(m.Name()))
	if g.hasDefaultValue(m) {
		out.WriteString(" = " + g.renderMemberValue(m))
	}
	out.WriteString(" :: " + g.renderMemberType(m))
	if m.Req() != sema.Required {
		out.WriteString(" | 'undefined'")
	}
}

// hasDefaultValue is t_erl_generator::has_default_value. Like
// renderDefaultValue, it checks the field's declared type rather than its
// true type.
func (g *Generator) hasDefaultValue(f *sema.Field) bool {
	typ := f.Type()
	if f.Value() == nil {
		if f.Req() == sema.Required {
			if typ.IsStruct() || typ.IsXception() || typ.IsMap() || typ.IsSet() || typ.IsList() {
				return true
			}
			return false
		}
		return false
	}
	return true
}

// renderMemberValue is t_erl_generator::render_member_value.
func (g *Generator) renderMemberValue(f *sema.Field) string {
	if f.Value() == nil {
		return g.renderDefaultValue(f)
	}
	return g.renderConstValue(f.Type(), f.Value())
}

// generateErlStructInfo is t_erl_generator::generate_erl_struct_info: the
// read method for a struct.
func (g *Generator) generateErlStructInfo(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "struct_info(" + g.typeName(s) + ") ->" + "\n")
	g.indentUp()
	out.WriteString(g.indent() + g.renderTypeTerm(s, true, false) + ";" + "\n")
	g.indentDown()
	out.WriteString("\n")
}

// generateErlExtendedStructInfo is
// t_erl_generator::generate_erl_extended_struct_info.
func (g *Generator) generateErlExtendedStructInfo(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "struct_info_ext(" + g.typeName(s) + ") ->" + "\n")
	g.indentUp()
	out.WriteString(g.indent() + g.renderTypeTerm(s, true, true) + ";" + "\n")
	g.indentDown()
	out.WriteString("\n")
}

// generateService is t_erl_generator::generate_service.
func (g *Generator) generateService(s *sema.Service) {
	g.serviceName = g.makeSafeForModuleName(g.serviceName)

	outDir := g.outDir()
	fServiceHrlName := outDir + g.serviceName + "_thrift.hrl"
	fServiceName := outDir + g.serviceName + "_thrift.erl"

	// Reset service text aggregating stream streams
	g.fService.Reset()
	g.exportLines.Reset()
	g.exportLinesFirst = true

	var fServiceHrl strings.Builder
	g.hrlHeader(&fServiceHrl, g.serviceName)

	if s.Extends() != nil {
		fServiceHrl.WriteString("-include(\"" + g.makeSafeForModuleName(s.Extends().Name()) +
			"_thrift.hrl\"). % inherit " + "\n")
	}

	fServiceHrl.WriteString("-include(\"" + g.makeSafeForModuleName(g.programName) + "_types.hrl\")." + "\n" + "\n")

	// Generate the three main parts of the service (well, two for now in PHP)
	g.generateServiceHelpers(s) // cpiro: New Erlang Order

	g.generateServiceInterface(s)

	g.generateServiceMetadata(s)

	var fServiceFile strings.Builder
	fServiceFile.WriteString(g.erlAutogenComment() + "\n" + "-module(" + g.serviceName + "_thrift)." +
		"\n" + "-behaviour(thrift_service)." + "\n" + "\n" + g.erlImports() + "\n")

	fServiceFile.WriteString("-include(\"" + g.makeSafeForModuleName(s.Name()) + "_thrift.hrl\")." + "\n" + "\n")

	fServiceFile.WriteString("-export([" + g.exportLines.String() + "])." + "\n" + "\n")

	fServiceFile.WriteString(g.fService.String())

	g.hrlFooter(&fServiceHrl)

	// Close service file
	emit.WriteFile(fServiceName, fServiceFile.String())
	emit.WriteFile(fServiceHrlName, fServiceHrl.String())
}

// generateServiceMetadata is t_erl_generator::generate_service_metadata.
func (g *Generator) generateServiceMetadata(s *sema.Service) {
	g.exportString("function_names", 0)
	functions := s.Functions()
	numFunctions := len(functions)

	g.fService.WriteString(g.indent() + "function_names() -> " + "\n")
	g.indentUp()
	g.fService.WriteString(g.indent() + "[")

	for i, f := range functions {
		g.fService.WriteString(g.atomify(f.Name()))
		if i < numFunctions-1 {
			g.fService.WriteString(", ")
		}
	}

	g.fService.WriteString("].\n\n")
	g.indentDown()
}

// generateServiceHelpers is t_erl_generator::generate_service_helpers:
// generates helper functions for a service.
func (g *Generator) generateServiceHelpers(s *sema.Service) {
	functions := s.Functions()

	g.exportString("struct_info", 1)

	for _, f := range functions {
		g.generateErlFunctionHelpers(f)
	}
	g.fService.WriteString("struct_info(_) -> erlang:error(function_clause)." + "\n")
}

// generateErlFunctionHelpers is t_erl_generator::generate_erl_function_helpers:
// generates a struct and helpers for a function. Presently a no-op.
func (g *Generator) generateErlFunctionHelpers(f *sema.Function) {}

// generateServiceInterface is t_erl_generator::generate_service_interface.
func (g *Generator) generateServiceInterface(s *sema.Service) {
	g.exportString("function_info", 2)

	functions := s.Functions()
	g.fService.WriteString("%%% interface" + "\n")
	for _, f := range functions {
		g.fService.WriteString(g.indent() + "% " + g.functionSignature(f, "") + "\n")
		g.generateFunctionInfo(s, f)
	}

	// Inheritance - pass unknown functions to base class
	if s.Extends() != nil {
		g.fService.WriteString(g.indent() + "function_info(Function, InfoType) ->" + "\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + g.makeSafeForModuleName(s.Extends().Name()) +
			"_thrift:function_info(Function, InfoType)." + "\n")
		g.indentDown()
	} else {
		// return function_clause error for non-existent functions
		g.fService.WriteString(g.indent() + "function_info(_Func, _Info) -> erlang:error(function_clause)." + "\n")
	}

	g.fService.WriteString(g.indent() + "\n")
}

// generateFunctionInfo is t_erl_generator::generate_function_info:
// generates a function_info(FunctionName, params_type) and
// function_info(FunctionName, reply_type).
func (g *Generator) generateFunctionInfo(s *sema.Service, f *sema.Function) {
	nameAtom := g.atomify(f.Name())

	xs := f.Xceptions()
	argStruct := f.Arglist()

	// function_info(Function, params_type):
	g.fService.WriteString(g.indent() + "function_info(" + nameAtom + ", params_type) ->" + "\n")
	g.indentUp()

	g.fService.WriteString(g.indent() + g.renderTypeTerm(argStruct, true, false) + ";" + "\n")

	g.indentDown()

	// function_info(Function, reply_type):
	g.fService.WriteString(g.indent() + "function_info(" + nameAtom + ", reply_type) ->" + "\n")
	g.indentUp()

	if !f.ReturnType().IsVoid() {
		g.fService.WriteString(g.indent() + g.renderTypeTerm(f.ReturnType(), false, false) + ";" + "\n")
	} else if f.IsOneway() {
		g.fService.WriteString(g.indent() + "oneway_void;" + "\n")
	} else {
		g.fService.WriteString(g.indent() + "{struct, []}" + ";" + "\n")
	}
	g.indentDown()

	// function_info(Function, exceptions):
	g.fService.WriteString(g.indent() + "function_info(" + nameAtom + ", exceptions) ->" + "\n")
	g.indentUp()
	g.fService.WriteString(g.indent() + g.renderTypeTerm(xs, true, false) + ";" + "\n")
	g.indentDown()
}

// functionSignature is t_erl_generator::function_signature: renders a
// function signature of the form 'type name(args)'.
func (g *Generator) functionSignature(f *sema.Function, prefix string) string {
	return prefix + f.Name() + "(This" + capitalize(g.argumentList(f.Arglist())) + ")"
}

// exportString is t_erl_generator::export_string: adds a function to the
// exports list.
func (g *Generator) exportString(name string, num int) {
	if g.exportLinesFirst {
		g.exportLinesFirst = false
	} else {
		g.exportLines.WriteString(", ")
	}
	g.exportLines.WriteString(name + "/" + strconv.Itoa(num))
}

// exportTypesString is t_erl_generator::export_types_string.
func (g *Generator) exportTypesString(name string, num int) {
	if g.exportTypesLinesFirst {
		g.exportTypesLinesFirst = false
	} else {
		g.exportTypesLines.WriteString(", ")
	}
	g.exportTypesLines.WriteString(name + "/" + strconv.Itoa(num))
}

// exportFunction is t_erl_generator::export_function. It is not called from
// anywhere in the generator's own output, exactly as in the C++ source, but
// is kept for parity of the class's public surface.
func (g *Generator) exportFunction(f *sema.Function, prefix string) {
	num := len(f.Arglist().Members())
	g.exportString(prefix+f.Name(), 1+num)
}

// argumentList is t_erl_generator::argument_list: renders a field list.
func (g *Generator) argumentList(s *sema.Struct) string {
	result := ""
	for _, f := range s.Members() {
		// initial comma to compensate for initial This
		result += ", "
		result += capitalize(f.Name())
	}
	return result
}

// typeName is t_erl_generator::type_name.
func (g *Generator) typeName(t sema.Type) string {
	prefix := t.Program().Namespace("erl")
	prefixLength := len(prefix)
	if prefixLength > 0 && prefix[prefixLength-1] != '_' {
		delimiterLength := len(g.opts.Delimiter)
		if delimiterLength > 0 && delimiterLength < prefixLength {
			// prefix.compare(prefix_length - delimiter_length, prefix_length,
			// delimiter_) in the C++ source: std::string::compare clamps the
			// count to the characters actually available from pos, which is
			// exactly delimiter_length, so this is a suffix check.
			if !strings.HasSuffix(prefix, g.opts.Delimiter) {
				prefix += g.opts.Delimiter
			}
		}
	}

	name := t.Name()

	return g.atomify(prefix + name)
}

// typeToEnum is t_erl_generator::type_to_enum: converts the parse type to
// an Erlang "type" (macro for int constants). It is not called from
// anywhere in the generator's own output, exactly as in the C++ source, but
// is kept for parity of the class's public surface.
func (g *Generator) typeToEnum(typ sema.Type) string {
	typ = sema.TrueType(typ)

	if typ.IsBaseType() {
		b := typ.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeVoid:
			emit.Throw("NO T_VOID CONSTRUCT")
		case sema.TypeString:
			return "?tType_STRING"
		case sema.TypeBool:
			return "?tType_BOOL"
		case sema.TypeI8:
			return "?tType_I8"
		case sema.TypeI16:
			return "?tType_I16"
		case sema.TypeI32:
			return "?tType_I32"
		case sema.TypeI64:
			return "?tType_I64"
		case sema.TypeDouble:
			return "?tType_DOUBLE"
		}
	} else if typ.IsEnum() {
		return "?tType_I32"
	} else if typ.IsStruct() || typ.IsXception() {
		return "?tType_STRUCT"
	} else if typ.IsMap() {
		return "?tType_MAP"
	} else if typ.IsSet() {
		return "?tType_SET"
	} else if typ.IsList() {
		return "?tType_LIST"
	}

	emit.Throw("INVALID TYPE IN type_to_enum: %s", typ.Name())
	return ""
}

// renderTypeTerm is t_erl_generator::render_type_term: generates an Erlang
// term which represents a thrift type.
func (g *Generator) renderTypeTerm(typ sema.Type, expandStructs, extendedInfo bool) string {
	typ = sema.TrueType(typ)

	if typ.IsBaseType() {
		b := typ.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeVoid:
			emit.Throw("NO T_VOID CONSTRUCT")
		case sema.TypeString:
			return "string"
		case sema.TypeBool:
			return "bool"
		case sema.TypeI8:
			return "byte"
		case sema.TypeI16:
			return "i16"
		case sema.TypeI32:
			return "i32"
		case sema.TypeI64:
			return "i64"
		case sema.TypeDouble:
			return "double"
		}
	} else if typ.IsEnum() {
		return "i32"
	} else if typ.IsStruct() || typ.IsXception() {
		if expandStructs {
			s := typ.(*sema.Struct)

			var buf strings.Builder
			buf.WriteString("{struct, [")
			fieldIndent := strings.Repeat(" ", buf.Len())

			fields := s.Members()
			for i, m := range fields {
				key := m.Key()
				mtype := g.renderTypeTerm(m.Type(), false, false) // recursive call

				if !extendedInfo {
					// Convert to format: {struct, [{Fid, Type}|...]}
					buf.WriteString("{" + strconv.Itoa(int(key)) + ", " + mtype + "}")
				} else {
					// Convert to format: {struct, [{Fid, Req, Type, Name, Def}|...]}
					name := m.Name()
					value := g.renderMemberValue(m)
					requiredness := g.renderMemberRequiredness(m)
					buf.WriteString("{" + strconv.Itoa(int(key)) + ", " + requiredness + ", " + mtype + ", " +
						g.atomify(name) + ", " + value + "}")
				}

				if i != len(fields)-1 {
					buf.WriteString("," + "\n" + fieldIndent)
				}
			}

			buf.WriteString("]}" + "\n")
			return buf.String()
		}
		return "{struct, {" + g.atomify(g.typeModule(typ)) + ", " + g.typeName(typ) + "}}"
	} else if typ.IsMap() {
		// {map, KeyType, ValType}
		m := typ.(*sema.Map)
		return "{map, " + g.renderTypeTerm(m.KeyType(), false, false) + ", " +
			g.renderTypeTerm(m.ValType(), false, false) + "}"

	} else if typ.IsSet() {
		s := typ.(*sema.Set)
		return "{set, " + g.renderTypeTerm(s.ElemType(), false, false) + "}"

	} else if typ.IsList() {
		l := typ.(*sema.List)
		return "{list, " + g.renderTypeTerm(l.ElemType(), false, false) + "}"
	}

	emit.Throw("INVALID TYPE IN type_to_enum: %s", typ.Name())
	return ""
}

// typeModule is t_erl_generator::type_module.
func (g *Generator) typeModule(t sema.Type) string {
	return g.makeSafeForModuleName(t.Program().Name()) + "_types"
}

// displayName is t_erl_generator::display_name.
func (g *Generator) displayName() string {
	return "Erlang"
}

// makeSafeForModuleName is t_erl_generator::make_safe_for_module_name.
func (g *Generator) makeSafeForModuleName(in string) string {
	if g.opts.LegacyNames {
		return decapitalize(g.opts.AppPrefix + in)
	}
	return underscore(g.opts.AppPrefix) + underscore(in)
}

// atomify is t_erl_generator::atomify.
func (g *Generator) atomify(in string) string {
	if g.opts.LegacyNames {
		return "'" + decapitalize(in) + "'"
	}
	return "'" + in + "'"
}

// constify is t_erl_generator::constify.
func (g *Generator) constify(in string) string {
	if g.opts.LegacyNames {
		return capitalize(in)
	}
	return uppercase(in)
}
