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

package dart

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

// Generator is t_dart_generator for one program.
type Generator struct {
	program     *sema.Program
	opts        Options
	programName string // program_name_
	serviceName string // service_name_

	libraryName string // library_name_

	baseDir string // base_dir_
	srcDir  string // src_dir_

	libraryExports strings.Builder // library_exports_

	// fService is f_service_: reopened at the start of every
	// generateService call and shared by generateServiceInterface,
	// generateServiceClient, generateServiceServer, generateServiceHelpers
	// and everything they call.
	fService *strings.Builder

	indentLevel int
	tmpCounter  int
}

func newGenerator(program *sema.Program, opts Options) *Generator {
	return &Generator{
		program:     program,
		opts:        opts,
		programName: program.Name(),
		libraryName: opts.LibraryName,
	}
}

// generate is t_generator::generate_program restricted to what
// t_dart_generator overrides: init, enums, typedefs (no-op), structs and
// exceptions in declared order (forward declarations are a no-op in the
// base class and dart never overrides them), constants, services, close.
func (g *Generator) generate() (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*emit.Error); ok {
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
	g.generateConsts(g.program.Consts())
	for _, s := range g.program.Services() {
		g.serviceName = s.Name()
		g.generateService(s)
	}
	g.closeGenerator()
	return nil
}

// ---- indentation and scoping ----

// ind is indent(): two spaces per level (t_dart_generator does not
// override indent_str, so the t_generator default applies).
func (g *Generator) ind() string { return strings.Repeat("  ", g.indentLevel) }

func (g *Generator) indentUp()   { g.indentLevel++ }
func (g *Generator) indentDown() { g.indentLevel-- }

// scopeUp is t_dart_generator::scope_up with its default prefix " ": it
// continues whatever was already written to out with " {\n" and does not
// itself indent, since the caller has usually already written the line
// this scope opens.
func (g *Generator) scopeUp(out *strings.Builder) { g.scopeUpPrefix(out, " ") }

func (g *Generator) scopeUpPrefix(out *strings.Builder, prefix string) {
	out.WriteString(prefix + "{\n")
	g.indentUp()
}

// scopeDown is t_dart_generator::scope_down with its default postfix
// "\n".
func (g *Generator) scopeDown(out *strings.Builder) { g.scopeDownPostfix(out, "\n") }

func (g *Generator) scopeDownPostfix(out *strings.Builder, postfix string) {
	g.indentDown()
	out.WriteString(g.ind() + "}" + postfix)
}

// tmp is t_generator::tmp: a name with a running number appended.
func (g *Generator) tmp(name string) string {
	s := name + strconv.Itoa(g.tmpCounter)
	g.tmpCounter++
	return s
}

// ---- init / close / library / pubspec ----

// outDir is get_out_dir with out_dir_base_ "gen-dart".
func (g *Generator) outDir() string {
	if g.program.IsOutPathAbsolute() {
		return g.program.OutPath() + "/"
	}
	return g.program.OutPath() + "gen-dart/"
}

func (g *Generator) initGenerator() {
	emit.Mkdir(g.outDir())

	if g.libraryName == "" {
		g.libraryName = findLibraryName(g.program)
	}

	subdir := g.outDir() + g.libraryName
	emit.Mkdir(subdir)
	g.baseDir = subdir

	if g.opts.LibraryPrefix == "" {
		subdir += "/lib"
		emit.Mkdir(subdir)
		subdir += "/src"
		emit.Mkdir(subdir)
		g.srcDir = subdir
	} else {
		g.srcDir = g.baseDir
	}
}

// findLibraryName is t_dart_generator::find_library_name.
func findLibraryName(program *sema.Program) string {
	name := program.Namespace("dart")
	if name == "" {
		name = program.Name()
	}
	name = replaceAll(name, ".", "_")
	name = replaceAll(name, "-", "_")
	return name
}

// dartLibrary is t_dart_generator::dart_library: "library myservice;".
func (g *Generator) dartLibrary(fileName string) string {
	out := "library " + g.opts.LibraryPrefix + g.libraryName
	if fileName != "" {
		if g.opts.LibraryPrefix == "" {
			out += ".src." + fileName
		} else {
			out += "." + fileName
		}
	}
	return out + ";\n"
}

// serviceImports is t_dart_generator::service_imports.
func serviceImports() string {
	return "import 'dart:async';\n"
}

// dartThriftImports is t_dart_generator::dart_thrift_imports.
func (g *Generator) dartThriftImports() string {
	imports := "import 'dart:typed_data' show Uint8List;\n" +
		"import 'package:thrift/thrift.dart';\n"

	if g.opts.PackagePrefix == "" {
		imports += "import 'package:" + g.libraryName + "/" + g.libraryName + ".dart';\n"
	} else {
		imports += "import 'package:" + g.opts.PackagePrefix + g.libraryName + ".dart';\n"
	}

	for _, inc := range g.program.Includes() {
		includeName := findLibraryName(inc)
		namedImport := "t_" + includeName
		if g.opts.PackagePrefix == "" {
			imports += "import 'package:" + includeName + "/" + includeName + ".dart' as " + namedImport + ";\n"
		} else {
			imports += "import 'package:" + g.opts.PackagePrefix + includeName + ".dart' as " + namedImport + ";\n"
		}
	}

	return imports
}

// closeGenerator is t_dart_generator::close_generator.
func (g *Generator) closeGenerator() {
	g.generateDartLibrary()

	if g.opts.LibraryPrefix == "" {
		g.generateDartPubspec()
	}
}

func (g *Generator) generateDartLibrary() {
	var fLibraryName string
	if g.opts.LibraryPrefix == "" {
		fLibraryName = g.baseDir + "/lib/" + g.libraryName + ".dart"
	} else {
		fLibraryName = g.outDir() + g.libraryName + ".dart"
	}

	var f strings.Builder
	f.WriteString(autogenComment() + "\n")
	f.WriteString("library " + g.opts.LibraryPrefix + g.libraryName + ";\n\n")
	f.WriteString(g.libraryExports.String())

	emit.WriteFile(fLibraryName, f.String())
}

// exportClassToLibrary is t_dart_generator::export_class_to_library.
func (g *Generator) exportClassToLibrary(fileName, className string) {
	subdir := "src"
	if g.opts.LibraryPrefix != "" {
		subdir = g.libraryName
	}
	g.libraryExports.WriteString("export '" + subdir + "/" + fileName + ".dart' show " + className + ";\n")
}

// generateDartPubspec is t_dart_generator::generate_dart_pubspec.
func (g *Generator) generateDartPubspec() {
	fPubspecName := g.baseDir + "/pubspec.yaml"
	var f strings.Builder

	f.WriteString(g.ind() + "name: " + g.libraryName + "\n")
	f.WriteString(g.ind() + "version: 0.0.1\n")
	f.WriteString(g.ind() + "description: Autogenerated by Thrift Compiler\n")
	f.WriteString("\n")

	f.WriteString(g.ind() + "environment:\n")
	g.indentUp()
	f.WriteString(g.ind() + "sdk: '>=2.12.0 <4.0.0'\n")
	g.indentDown()
	f.WriteString("\n")

	f.WriteString(g.ind() + "dependencies:\n")
	g.indentUp()

	if g.opts.PubspecLib == "" {
		// default to relative path within working directory, which works for tests
		f.WriteString(g.ind() + "thrift:  # ^" + version.Version + "\n")
		g.indentUp()
		f.WriteString(g.ind() + "path: ../../../../lib/dart\n")
		g.indentDown()
	} else {
		for _, line := range splitPipe(g.opts.PubspecLib) {
			f.WriteString(g.ind() + line + "\n")
		}
	}

	// add included thrift files as dependencies
	for _, inc := range g.program.Includes() {
		includeName := findLibraryName(inc)
		f.WriteString(g.ind() + includeName + ":\n")
		g.indentUp()
		f.WriteString(g.ind() + "path: ../" + includeName + "\n")
		g.indentDown()
	}

	g.indentDown()
	f.WriteString("\n")

	emit.WriteFile(fPubspecName, f.String())
}

// generateTypedef is t_dart_generator::generate_typedef: not used.
func (g *Generator) generateTypedef(ttypedef *sema.Typedef) { _ = ttypedef }

// generateEnum is t_dart_generator::generate_enum: enums are a class with
// a set of static constants.
func (g *Generator) generateEnum(e *sema.Enum) {
	fileName := getFileName(e.Name())
	fEnumName := g.srcDir + "/" + fileName + ".dart"
	var f strings.Builder

	// Comment and add library
	f.WriteString(autogenComment() + g.dartLibrary(fileName) + "\n")

	className := e.Name()
	g.exportClassToLibrary(fileName, className)
	f.WriteString("class " + className)
	g.scopeUp(&f)

	constants := e.Constants()
	for _, c := range constants {
		f.WriteString(g.ind() + "static const int " + c.Name() + " = " + strconv.Itoa(int(c.Value())) + ";\n")
	}

	// Create a static Set with all valid values for this enum
	f.WriteString("\n")

	f.WriteString(g.ind() + "static final Set<int> VALID_VALUES = new Set.from([\n")
	g.indentUp()
	firstValue := true
	for _, c := range constants {
		// populate set
		f.WriteString(g.ind())
		if !firstValue {
			f.WriteString(", ")
		}
		f.WriteString(c.Name() + "\n")
		firstValue = false
	}
	g.indentDown()
	f.WriteString(g.ind() + "]);\n")

	f.WriteString(g.ind() + "static final Map<int, String> VALUES_TO_NAMES = {\n")
	g.indentUp()
	firstValue = true
	for _, c := range constants {
		f.WriteString(g.ind())
		if !firstValue {
			f.WriteString(", ")
		}
		f.WriteString(c.Name() + ": '" + c.Name() + "'\n")
		firstValue = false
	}
	g.indentDown()
	f.WriteString(g.ind() + "};\n")

	g.scopeDown(&f) // end class

	emit.WriteFile(fEnumName, f.String())
}

// ---- type mapping ----

// getDartTypeString is t_dart_generator::get_dart_type_string. It is
// declared and defined in the C++ source but never called; ported for
// fidelity.
func getDartTypeString(t sema.Type) string {
	switch {
	case t.IsList():
		return "TType.LIST"
	case t.IsMap():
		return "TType.MAP"
	case t.IsSet():
		return "TType.SET"
	case t.IsStruct(), t.IsXception():
		return "TType.STRUCT"
	case t.IsEnum():
		return "TType.I32"
	case t.IsTypedef():
		return getDartTypeString(t.(*sema.Typedef).Type())
	case t.IsBaseType():
		b := t.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeVoid:
			return "TType.VOID"
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
		default:
			emit.Throw("Unknown thrift type \"%s\" passed to t_dart_generator::get_dart_type_string!", t.Name())
		}
	default:
		emit.Throw("Unknown thrift type \"%s\" passed to t_dart_generator::get_dart_type_string!", t.Name())
	}
	return ""
}

// typeName is t_dart_generator::type_name.
func (g *Generator) typeName(t sema.Type) string {
	t = sema.TrueType(t)
	switch {
	case t.IsBaseType():
		return baseTypeName(t.(*sema.BaseType))
	case t.IsEnum():
		return "int"
	case t.IsMap():
		m := t.(*sema.Map)
		return "Map<" + g.typeName(m.KeyType()) + ", " + g.typeName(m.ValType()) + ">"
	case t.IsSet():
		s := t.(*sema.Set)
		return "Set<" + g.typeName(s.ElemType()) + ">"
	case t.IsList():
		l := t.(*sema.List)
		return "List<" + g.typeName(l.ElemType()) + ">"
	}
	return g.getTtypeClassName(t)
}

// typeNameInstantiate is t_dart_generator::type_name_instantiate.
func (g *Generator) typeNameInstantiate(t sema.Type) string {
	t = sema.TrueType(t)
	switch {
	case t.IsBaseType():
		return baseTypeName(t.(*sema.BaseType))
	case t.IsEnum():
		return "int"
	case t.IsMap():
		m := t.(*sema.Map)
		return "<" + g.typeName(m.KeyType()) + ", " + g.typeName(m.ValType()) + ">{}"
	case t.IsSet():
		s := t.(*sema.Set)
		return "<" + g.typeName(s.ElemType()) + ">{}"
	case t.IsList():
		l := t.(*sema.List)
		return "<" + g.typeName(l.ElemType()) + ">[]"
	}
	return g.getTtypeClassName(t)
}

// baseTypeName is t_dart_generator::base_type_name.
func baseTypeName(t *sema.BaseType) string {
	switch t.Base() {
	case sema.TypeVoid:
		return "void"
	case sema.TypeString:
		if t.IsBinary() {
			return "Uint8List"
		}
		return "String"
	case sema.TypeBool:
		return "bool"
	case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
		return "int"
	case sema.TypeDouble:
		return "double"
	default:
		emit.Throw("compiler error: no Dart name for base type %s", sema.BaseName(t.Base()))
	}
	return ""
}

// declareField is t_dart_generator::declare_field: declares a field,
// which may include initialization as necessary.
func (g *Generator) declareField(field *sema.Field, initv bool) string {
	fieldName := getMemberName(field.Name())
	result := g.typeName(field.Type()) + " " + fieldName
	if initv {
		ttype := sema.TrueType(field.Type())
		switch {
		case ttype.IsBaseType() && field.Value() != nil:
			var dummy strings.Builder
			result += " = " + g.renderConstValue(&dummy, fieldName, ttype, field.Value())
		case ttype.IsBaseType():
			b := ttype.(*sema.BaseType)
			switch b.Base() {
			case sema.TypeVoid:
				emit.Throw("NO T_VOID CONSTRUCT")
			case sema.TypeString:
				result += " = null"
			case sema.TypeBool:
				result += " = false"
			case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
				result += " = 0"
			case sema.TypeDouble:
				result += " = 0.0"
			default:
				emit.Throw("compiler error: unhandled type")
			}
		case ttype.IsEnum():
			result += " = 0"
		case ttype.IsContainer():
			result += " = new " + g.typeName(ttype) + "()"
		default:
			result += " = new " + g.typeName(ttype) + "()"
		}
	}
	return result + ";"
}

// functionSignature is t_dart_generator::function_signature: renders a
// function signature of the form 'type name(args)'.
func (g *Generator) functionSignature(f *sema.Function) string {
	arguments := g.argumentList(f.Arglist())

	var returntype string
	if f.ReturnType().IsVoid() {
		returntype = "Future<void>"
	} else {
		returntype = "Future<" + g.typeName(f.ReturnType()) + getTypeSuffix(f.ReturnType()) + ">"
	}

	return returntype + " " + getMemberName(f.Name()) + "(" + arguments + ")"
}

// argumentList is t_dart_generator::argument_list: renders a comma
// separated field list, with type names.
func (g *Generator) argumentList(s *sema.Struct) string {
	var result strings.Builder
	first := true
	for _, f := range s.Members() {
		if first {
			first = false
		} else {
			result.WriteString(", ")
		}
		fieldName := getMemberName(f.Name())
		suffix := ""
		if typeCanBeNull(f.Type()) {
			suffix = "?"
		}
		result.WriteString(g.typeName(f.Type()) + suffix + " " + fieldName)
	}
	return result.String()
}

// typeToEnum converts the parse type to a Dart TType enum string for the
// given type. It is t_dart_generator::type_to_enum.
func typeToEnum(t sema.Type) string {
	t = sema.TrueType(t)
	if t.IsBaseType() {
		b := t.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeVoid:
			emit.Throw("NO T_VOID CONSTRUCT")
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
	emit.Throw("INVALID TYPE IN type_to_enum: %s", t.Name())
	return ""
}

// initValue is t_dart_generator::init_value.
func initValue(field *sema.Field) string {
	ttype := field.Type()

	if ttype.IsEnum() {
		return " = 0"
	}

	// Get the actual type for a typedef (one level only, as the C++ code
	// does).
	if ttype.IsTypedef() {
		ttype = ttype.(*sema.Typedef).Type()
	}

	// Only consider base types for default initialization
	if !ttype.IsBaseType() {
		return ""
	}
	b := ttype.(*sema.BaseType)

	switch b.Base() {
	case sema.TypeBool:
		return " = false"
	case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
		return " = 0"
	case sema.TypeDouble:
		return " = 0.0"
	case sema.TypeVoid, sema.TypeString:
		return ""
	default:
		emit.Throw("compiler error: unhandled type")
	}
	return ""
}

// typeCanBeNull is t_dart_generator::type_can_be_null.
func typeCanBeNull(t sema.Type) bool {
	t = sema.TrueType(t)
	return t.IsContainer() || t.IsStruct() || t.IsXception() || t.IsString()
}

// getTypeSuffix is t_dart_generator::get_type_suffix.
func getTypeSuffix(t sema.Type) string {
	if typeCanBeNull(t) {
		return "?"
	}
	return ""
}

// getTtypeClassName is t_dart_generator::get_ttype_class_name.
func (g *Generator) getTtypeClassName(t sema.Type) string {
	if g.program == t.Program() {
		return t.Name()
	}
	namedImport := "t_" + findLibraryName(t.Program())
	return namedImport + "." + t.Name()
}

func (g *Generator) displayName() string { return "Dart" }

// ---- naming helpers ----

// getCapName is t_dart_generator::get_cap_name.
func getCapName(name string) string {
	if name == "" {
		return name
	}
	b := []byte(name)
	b[0] = toUpperByte(b[0])
	return string(b)
}

// getMemberName is t_dart_generator::get_member_name.
func getMemberName(name string) string {
	if name == "" {
		return name
	}
	b := []byte(name)
	b[0] = toLowerByte(b[0])
	return string(b)
}

// getArgsClassName is t_dart_generator::get_args_class_name.
func getArgsClassName(name string) string { return name + "_args" }

// getResultClassName is t_dart_generator::get_result_class_name.
func getResultClassName(name string) string { return name + "_result" }

// getFileName is t_dart_generator::get_file_name, e.g. APIForFileIO
// becomes api_for_file_io.
func getFileName(name string) string {
	var ret strings.Builder
	n := len(name)
	var first byte
	if n > 0 {
		first = name[0]
	}
	isPrevLC := true
	isCurrentLC := first == toLowerByte(first)
	isNextLC := false

	for i := 0; i < n; i++ {
		c := name[i]
		lc := toLowerByte(c)

		if i == n-1 {
			isNextLC = false
		} else {
			next := name[i+1]
			isNextLC = next == toLowerByte(next)
		}

		if i != 0 && !isCurrentLC && (isPrevLC || isNextLC) {
			ret.WriteByte('_')
		}
		ret.WriteByte(lc)

		isPrevLC = isCurrentLC
		isCurrentLC = isNextLC
	}

	return ret.String()
}

// getConstantsClassName is t_dart_generator::get_constants_class_name,
// e.g. my_great_model becomes MyGreatModelConstants.
func getConstantsClassName(name string) string {
	var ret strings.Builder
	isPrevUnderscore := true
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c == '_' {
			isPrevUnderscore = true
			continue
		}
		if isPrevUnderscore {
			ret.WriteByte(toUpperByte(c))
		} else {
			ret.WriteByte(c)
		}
		isPrevUnderscore = false
	}
	return ret.String() + "Constants"
}

// constantName is t_dart_generator::constant_name.
func constantName(name string) string {
	var out strings.Builder
	isFirst := true
	wasPreviousCharUpper := false
	for i := 0; i < len(name); i++ {
		c := name[i]
		isUpper := isUpperByte(c)
		if isUpper && !isFirst && !wasPreviousCharUpper {
			out.WriteByte('_')
		}
		out.WriteByte(toUpperByte(c))
		isFirst = false
		wasPreviousCharUpper = isUpper
	}
	return out.String()
}

// upcaseString is t_oop_generator::upcase_string.
func upcaseString(s string) string {
	b := []byte(s)
	for i, c := range b {
		b[i] = toUpperByte(c)
	}
	return string(b)
}

func isUpperByte(c byte) bool { return c >= 'A' && c <= 'Z' }
func isLowerByte(c byte) bool { return c >= 'a' && c <= 'z' }
func toUpperByte(c byte) byte {
	if isLowerByte(c) {
		return c - 'a' + 'A'
	}
	return c
}
func toLowerByte(c byte) byte {
	if isUpperByte(c) {
		return c - 'A' + 'a'
	}
	return c
}

// replaceAll is t_oop_generator::replace_all.
func replaceAll(contents, search, repl string) string {
	if search == "" {
		return contents
	}
	str := contents
	slen := len(search)
	rlen := len(repl)
	incr := rlen
	if incr == 0 {
		incr = 1
	}
	found := strings.Index(str, search)
	for found >= 0 && found < len(str) {
		str = str[:found] + repl + str[found+slen:]
		next := found + incr
		if next > len(str) {
			break
		}
		idx := strings.Index(str[next:], search)
		if idx < 0 {
			found = -1
		} else {
			found = next + idx
		}
	}
	return str
}

// splitPipe is the generator's split(s, '|') helper, used only to split
// pubspec_lib on the pipe delimiter. It mimics the C++ getline loop: a
// trailing empty segment (the string ends with the delimiter) is
// dropped, since std::getline's next call hits EOF with nothing
// extracted and fails; an empty segment between two delimiters is kept,
// since that getline call still succeeds by finding the delimiter
// immediately.
func splitPipe(s string) []string {
	parts := strings.Split(s, "|")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// formatDouble mimics `ostream << double` with the default precision of
// six significant digits, as used by renderConstValue's TYPE_DOUBLE
// case (no setprecision call is in effect there).
func formatDouble(f float64) string {
	return strconv.FormatFloat(f, 'g', 6, 64)
}

// autogenSummary is t_generator::autogen_summary.
func autogenSummary() string {
	return "Autogenerated by Thrift Compiler (" + version.Version + ")"
}

// autogenComment is t_generator::autogen_comment.
func autogenComment() string {
	return "/**\n" + " * " + autogenSummary() + "\n" + " *\n" +
		" * DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING\n" +
		" *  @generated\n" + " */\n"
}

// ---- doc comments ----

// docer is the doc-comment interface every commented IDL element
// satisfies (t_doc).
type docer interface {
	HasDoc() bool
	Doc() string
}

// generateDartDoc is t_dart_generator::generate_dart_doc(ostream&, t_doc*).
func (g *Generator) generateDartDoc(out *strings.Builder, d docer) {
	if d.HasDoc() {
		emit.DocstringComment(out, g.ind(), "", "/// ", d.Doc(), "")
	}
}

// generateDartDocFunction is
// t_dart_generator::generate_dart_doc(ostream&, t_function*).
func (g *Generator) generateDartDocFunction(out *strings.Builder, f *sema.Function) {
	if f.HasDoc() {
		var ss strings.Builder
		ss.WriteString(f.Doc())
		for _, p := range f.Arglist().Members() {
			fieldName := getMemberName(p.Name())
			ss.WriteString("\n@param " + fieldName)
			if p.HasDoc() {
				ss.WriteString(" " + p.Doc())
			}
		}
		emit.DocstringComment(out, g.ind(), "", "/// ", ss.String(), "")
	}
}

// generateIssetCheckField is
// t_dart_generator::generate_isset_check(t_field*).
func generateIssetCheckField(field *sema.Field) string {
	fieldName := getMemberName(field.Name())
	return generateIssetCheckName(fieldName)
}

// generateIssetCheckName is
// t_dart_generator::generate_isset_check(std::string).
func generateIssetCheckName(fieldName string) string {
	return "is" + getCapName("set") + getCapName(fieldName) + "()"
}

// generateIssetSet is t_dart_generator::generate_isset_set.
func (g *Generator) generateIssetSet(out *strings.Builder, field *sema.Field) {
	if !typeCanBeNull(field.Type()) {
		fieldName := getMemberName(field.Name())
		out.WriteString(g.ind() + "this.__isset_" + fieldName + " = true;\n")
	}
}
