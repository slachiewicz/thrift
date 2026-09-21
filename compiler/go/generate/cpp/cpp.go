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

// Package cpp generates C++ code from a resolved Thrift program.
//
// It is a method-for-method port of
// compiler/cpp/src/thrift/generate/t_cpp_generator.cc. The port writes
// exactly the bytes the C++ generator writes, which is checked against the
// C++ compiler by the parity tests; that is why the emitter builds strings
// the way an ostream would rather than through templates.
package cpp

import (
	"os"
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

// thriftVersion is the version string the autogen comment carries.
const thriftVersion = version.Version

// Options are the "--gen cpp:" generator options, read from the doc string
// of THRIFT_REGISTER_GENERATOR and the constructor's option loop in
// t_cpp_generator.cc.
type Options struct {
	PureEnums          bool
	EnumClass          bool // pure_enums=enum_class
	UseIncludePrefix   bool
	CobStyle           bool
	NoClientCompletion bool
	NoDefaultOperators bool
	Templates          bool
	TemplatesOnly      bool // templates=only
	Moveable           bool
	ForwardSetter      bool // moveable_types=forward_setter
	TemplateStreamop   bool
	NoOstreamOperators bool
	NoSkeleton         bool
	NoConstructors     bool
	PrivateOptional    bool
}

// ParseOptions parses the part after "cpp:" of a --gen argument, with the
// same splitting rules as t_generator::parse_options: options are comma
// separated and a value follows the first "=". A repeated key keeps the
// last value, as the C++ parser's std::map<string,string> does.
func ParseOptions(spec string) (Options, error) {
	var o Options
	for _, option := range strings.Split(spec, ",") {
		key, value := option, ""
		if i := strings.IndexByte(option, '='); i >= 0 {
			key, value = option[:i], option[i+1:]
		}
		switch key {
		case "":
		case "pure_enums":
			o.PureEnums = true
			if value == "enum_class" {
				o.EnumClass = true
			}
		case "include_prefix":
			o.UseIncludePrefix = true
		case "cob_style":
			o.CobStyle = true
		case "no_client_completion":
			o.NoClientCompletion = true
		case "no_default_operators":
			o.NoDefaultOperators = true
		case "templates":
			o.Templates = true
			o.TemplatesOnly = value == "only"
		case "moveable_types":
			o.Moveable = true
			if value == "forward_setter" {
				o.ForwardSetter = true
			}
		case "no_ostream_operators":
			o.NoOstreamOperators = true
		case "template_streamop":
			o.TemplateStreamop = true
		case "no_skeleton":
			o.NoSkeleton = true
		case "no_constructors":
			o.NoConstructors = true
		case "private_optional":
			o.PrivateOptional = true
		default:
			return o, &emit.Error{Msg: "unknown option cpp:" + key}
		}
	}
	return o, nil
}

func init() {
	generate.Register(generate.Info{
		Name:     "cpp",
		LongName: "C++",
		Options: []generate.Option{
			{Name: "cob_style", Help: "Generate \"Continuation OBject\"-style classes."},
			{Name: "no_client_completion", Help: "Omit calls to completion__() in CobClient class."},
			{Name: "no_default_operators", Help: "Omits generation of default operators ==, != and <"},
			{Name: "templates", Help: "Generate templatized reader/writer methods."},
			{Name: "pure_enums", Help: "Generate pure enums instead of wrapper classes.\nWhen 'pure_enums=enum_class', generate C++ 11 enum class."},
			{Name: "include_prefix", Help: "Use full include paths in generated files."},
			{Name: "moveable_types", Help: "Generate move constructors and assignment operators.\nWhen 'moveable_types=forward_setter', also generate setters\nwith perfect forwarding for non-primitive types."},
			{Name: "no_ostream_operators", Help: "Omit generation of ostream definitions."},
			{Name: "no_skeleton", Help: "Omits generation of skeleton."},
			{Name: "template_streamop", Help: "Generate operator<< and printTo with a generic stream type template."},
			{Name: "no_constructors", Help: "Omits generation of constructors/destructors."},
			{Name: "private_optional", Help: "Generate optional fields as private members with getters."},
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

// Generator is t_cpp_generator for one program.
type Generator struct {
	program     *sema.Program
	opts        Options
	programName string
	serviceName string

	tmpCounter  int
	indentLevel int

	outDirBase string // out_dir_base_, always "gen-cpp"

	nsOpen  string
	nsClose string

	hasMembers bool

	fTypes         strings.Builder
	fTypesImpl     strings.Builder
	fTypesTcc      strings.Builder
	typesTccOpened bool

	// Per-service streams, valid only while generateService runs.
	fHeader          *strings.Builder
	fService         *strings.Builder
	fServiceTcc      *strings.Builder
	serviceTccOpened bool
}

func newGenerator(program *sema.Program, opts Options) *Generator {
	return &Generator{
		program:     program,
		opts:        opts,
		programName: program.Name(),
		outDirBase:  "gen-cpp",
	}
}

// generateProgram is t_generator::generate_program, specialized to the
// cpp generator's overrides.
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
		g.generateCppStruct(s, s.IsXception())
	}
	g.generateConsts(g.program.Consts())
	for _, s := range g.program.Services() {
		g.serviceName = s.Name()
		g.generateService(s)
	}
	g.closeGenerator()
}

// generateForwardDeclaration is t_cpp_generator::generate_forward_declaration.
func (g *Generator) generateForwardDeclaration(s *sema.Struct) {
	g.fTypes.WriteString(g.indent() + "class " + s.Name() + ";\n\n")
}

// initGenerator is t_cpp_generator::init_generator.
func (g *Generator) initGenerator() {
	mkdir(g.outDir())
	g.programName = getLegalProgramName(g.programName)

	g.fTypes.WriteString(g.autogenComment())
	g.fTypesImpl.WriteString(g.autogenComment())
	g.fTypesTcc.WriteString(g.autogenComment())

	g.fTypes.WriteString("#ifndef " + g.programName + "_TYPES_H\n#define " + g.programName + "_TYPES_H\n\n")
	g.fTypesTcc.WriteString("#ifndef " + g.programName + "_TYPES_TCC\n#define " + g.programName + "_TYPES_TCC\n\n")

	g.fTypes.WriteString("#include <iosfwd>\n\n" +
		"#include <thrift/Thrift.h>\n" +
		"#include <thrift/TApplicationException.h>\n" +
		"#include <thrift/TBase.h>\n" +
		"#include <thrift/protocol/TProtocol.h>\n" +
		"#include <thrift/transport/TTransport.h>\n\n")
	g.fTypes.WriteString("#include <functional>\n")
	g.fTypes.WriteString("#include <memory>\n")

	for _, inc := range g.program.Includes() {
		g.fTypes.WriteString("#include \"" + g.getIncludePrefix(inc) + inc.Name() + "_types.h\"\n")
		// XXX(simpkins): If gen_templates_ is enabled, this currently
		// assumes all included files were also generated with templates
		// enabled.
		g.fTypesTcc.WriteString("#include \"" + g.getIncludePrefix(inc) + inc.Name() + "_types.tcc\"\n")
	}
	g.fTypes.WriteString("\n")

	for _, cppInclude := range g.program.CppIncludes() {
		if strings.HasPrefix(cppInclude, "<") {
			g.fTypes.WriteString("#include " + cppInclude + "\n")
		} else {
			g.fTypes.WriteString("#include \"" + cppInclude + "\"\n")
		}
	}
	g.fTypes.WriteString("\n")

	g.fTypesImpl.WriteString("#include \"" + g.getIncludePrefix(g.program) + g.programName + "_types.h\"\n\n")
	g.fTypesTcc.WriteString("#include \"" + g.getIncludePrefix(g.program) + g.programName + "_types.h\"\n\n")

	// The swap() code needs <algorithm> for std::swap()
	g.fTypesImpl.WriteString("#include <algorithm>\n")
	// for operator<<
	g.fTypesImpl.WriteString("#include <ostream>\n\n")
	g.fTypesImpl.WriteString("#include <thrift/TToString.h>\n\n")

	// For template_streamop, TPrintTo.h is needed in the .tcc file for
	// direct streaming; it avoids the overhead of to_string, which uses
	// ostringstream internally.
	if g.opts.TemplateStreamop {
		g.fTypesTcc.WriteString("#include <thrift/TPrintTo.h>\n\n")
	}

	g.nsOpen = namespaceOpen(g.program.Namespace("cpp"))
	g.nsClose = namespaceClose(g.program.Namespace("cpp"))

	g.fTypes.WriteString(g.nsOpen + "\n\n")
	g.fTypesImpl.WriteString(g.nsOpen + "\n\n")
	g.fTypesTcc.WriteString(g.nsOpen + "\n\n")

	g.typesTccOpened = g.opts.Templates || g.opts.ForwardSetter || g.opts.TemplateStreamop
}

// closeGenerator is t_cpp_generator::close_generator.
func (g *Generator) closeGenerator() {
	g.fTypes.WriteString(g.nsClose + "\n\n")
	g.fTypesImpl.WriteString(g.nsClose + "\n")
	g.fTypesTcc.WriteString(g.nsClose + "\n\n")

	// Include the types.tcc file from the types header file, so clients
	// don't have to explicitly include the tcc file.
	if g.opts.Templates || g.opts.ForwardSetter || g.opts.TemplateStreamop {
		g.fTypes.WriteString("#include \"" + g.getIncludePrefix(g.program) + g.programName + "_types.tcc\"\n\n")
	}

	g.fTypes.WriteString("#endif\n")
	g.fTypesTcc.WriteString("#endif\n")

	outDir := g.outDir()
	writeFile(outDir+g.programName+"_types.h", g.fTypes.String())
	implPath := outDir + g.programName + "_types.cpp"
	if g.hasMembers {
		writeFile(implPath, g.fTypesImpl.String())
	} else {
		removeIfExists(implPath)
	}
	if g.typesTccOpened {
		writeFile(outDir+g.programName+"_types.tcc", g.fTypesTcc.String())
	}
}

func removeIfExists(path string) { _ = os.Remove(path) }

// ---- t_generator helpers ----

// tmp is t_generator::tmp: a name with a running number appended.
func (g *Generator) tmp(name string) string {
	s := name + strconv.Itoa(g.tmpCounter)
	g.tmpCounter++
	return s
}

func (g *Generator) indentUp()   { g.indentLevel++ }
func (g *Generator) indentDown() { g.indentLevel-- }

// indent is t_generator::indent.
func (g *Generator) indent() string {
	return strings.Repeat("  ", g.indentLevel)
}

// scopeUp is t_oop_generator::scope_up.
func (g *Generator) scopeUp(out *strings.Builder) {
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
}

// scopeDown is t_oop_generator::scope_down.
func (g *Generator) scopeDown(out *strings.Builder) {
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

// escapeString is t_generator::escape_string, via the shared table.
func escapeString(in string) string { return emit.EscapeString(in) }

// doubleAsString is t_generator::emit_double_as_string.
func doubleAsString(v float64) string { return emit.DoubleFixed16(v) }

func (g *Generator) autogenComment() string {
	return "/**\n * Autogenerated by Thrift Compiler (" + thriftVersion + ")\n *\n" +
		" * DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING\n" +
		" *  @generated\n */\n"
}

// generateJavaDoc is the generic t_doc overload of generate_java_doc.
func (g *Generator) generateJavaDoc(out *strings.Builder, hasDoc bool, doc string) {
	if hasDoc {
		g.generateJavaDocstringComment(out, doc)
	}
}

// generateJavaDocField is the t_field overload of generate_java_doc, which
// appends an @see for an enum-typed field.
func (g *Generator) generateJavaDocField(out *strings.Builder, f *sema.Field) {
	if sema.TrueType(f.Type()).IsEnum() {
		combined := f.Doc() + "\n@see " + g.getEnumClassName(f.Type())
		g.generateJavaDocstringComment(out, combined)
	} else {
		g.generateJavaDoc(out, f.HasDoc(), f.Doc())
	}
}

// generateJavaDocFunction is the t_function overload of generate_java_doc.
func (g *Generator) generateJavaDocFunction(out *strings.Builder, f *sema.Function) {
	if !f.HasDoc() {
		return
	}
	var ss strings.Builder
	ss.WriteString(f.Doc())
	for _, p := range f.Arglist().Members() {
		ss.WriteString("\n@param " + p.Name())
		if p.HasDoc() {
			ss.WriteString(" " + p.Doc())
		}
	}
	emit.DocstringComment(out, g.indent(), "/**\n", " * ", ss.String(), " */\n")
}

func (g *Generator) generateJavaDocstringComment(out *strings.Builder, contents string) {
	emit.DocstringComment(out, g.indent(), "/**\n", " * ", contents, " */\n")
}

// getEnumClassName is t_oop_generator::get_enum_class_name.
func (g *Generator) getEnumClassName(t sema.Type) string {
	pkg := ""
	if p := t.Program(); p != nil && p != g.program {
		pkg = p.Namespace("java") + "."
	}
	return pkg + t.Name()
}

// ---- files ----

// outDir is t_generator::get_out_dir.
func (g *Generator) outDir() string {
	if g.program.IsOutPathAbsolute() {
		return g.program.OutPath() + "/"
	}
	return g.program.OutPath() + g.outDirBase + "/"
}

func writeFile(path, content string) { emit.WriteFile(path, content) }
func mkdir(path string)              { emit.Mkdir(path) }

// getLegalProgramName is t_cpp_generator::get_legal_program_name: dots in
// the program name become underscores.
func getLegalProgramName(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}

// getIncludePrefix is t_cpp_generator::get_include_prefix.
func (g *Generator) getIncludePrefix(program *sema.Program) string {
	includePrefix := program.IncludePrefix()
	if !g.opts.UseIncludePrefix || (len(includePrefix) > 0 && includePrefix[0] == '/') {
		return ""
	}
	if lastSlash := strings.LastIndexByte(includePrefix, '/'); lastSlash >= 0 {
		if g.program.IsOutPathAbsolute() {
			return includePrefix[:lastSlash] + "/"
		}
		return includePrefix[:lastSlash] + "/" + g.outDirBase + "/"
	}
	return ""
}

// ---- namespaces ----

// namespacePrefix is t_cpp_generator::namespace_prefix.
func namespacePrefix(ns string) string {
	result := " ::"
	for {
		i := strings.IndexByte(ns, '.')
		if i < 0 {
			break
		}
		result += ns[:i] + "::"
		ns = ns[i+1:]
	}
	if ns != "" {
		result += ns + "::"
	}
	return result
}

// namespaceOpen is t_cpp_generator::namespace_open.
func namespaceOpen(ns string) string {
	if ns == "" {
		return ""
	}
	result := ""
	sep := ""
	for {
		i := strings.IndexByte(ns, '.')
		if i < 0 {
			break
		}
		result += sep + "namespace " + ns[:i] + " {"
		sep = " "
		ns = ns[i+1:]
	}
	if ns != "" {
		result += sep + "namespace " + ns + " {"
	}
	return result
}

// namespaceClose is t_cpp_generator::namespace_close.
func namespaceClose(ns string) string {
	if ns == "" {
		return ""
	}
	result := "}"
	for {
		i := strings.IndexByte(ns, '.')
		if i < 0 {
			break
		}
		result += "}"
		ns = ns[i+1:]
	}
	result += " // namespace"
	return result
}

// ---- types ----

// cppNamed is t_container's has_cpp_name/get_cpp_name, for the container
// types that embed containerBase.
type cppNamed interface {
	HasCppName() bool
	CppName() string
}

// isComplexType is t_cpp_generator::is_complex_type.
func (g *Generator) isComplexType(t sema.Type) bool {
	t = sema.TrueType(t)
	if t.IsContainer() || t.IsStruct() || t.IsXception() {
		return true
	}
	if t.IsBaseType() {
		b := t.(*sema.BaseType)
		return b.Base() == sema.TypeString || b.Base() == sema.TypeUUID
	}
	return false
}

// isReference is t_cpp_generator::is_reference.
func isReference(f *sema.Field) bool { return f.Reference() }

// hasCustomOstream is t_cpp_generator::has_custom_ostream.
func (g *Generator) hasCustomOstream(annotations sema.Annotations) bool {
	return g.opts.NoOstreamOperators || annotations.Has("cpp.customostream")
}

// baseTypeName is t_cpp_generator::base_type_name.
func baseTypeName(k sema.BaseKind) string {
	switch k {
	case sema.TypeVoid:
		return "void"
	case sema.TypeString:
		return "std::string"
	case sema.TypeBool:
		return "bool"
	case sema.TypeI8:
		return "int8_t"
	case sema.TypeI16:
		return "int16_t"
	case sema.TypeI32:
		return "int32_t"
	case sema.TypeI64:
		return "int64_t"
	case sema.TypeDouble:
		return "double"
	case sema.TypeUUID:
		return "apache::thrift::TUuid"
	}
	emit.Throw("compiler error: no C++ base type name for base type %s", sema.BaseName(k))
	return ""
}

// typeName is t_cpp_generator::type_name.
func (g *Generator) typeName(ttype sema.Type, inTypedef, arg bool) string {
	if ttype.IsBaseType() {
		b := ttype.(*sema.BaseType)
		bname := baseTypeName(b.Base())
		if vals, ok := ttype.Annotations()["cpp.type"]; ok && len(vals) > 0 {
			bname = vals[len(vals)-1]
		}
		if !arg {
			return bname
		}
		if b.Base() == sema.TypeString || b.Base() == sema.TypeUUID {
			return "const " + bname + "&"
		}
		return "const " + bname
	}

	if ttype.IsContainer() {
		var cname string
		if cn, ok := ttype.(cppNamed); ok && cn.HasCppName() {
			cname = cn.CppName()
		} else if ttype.IsMap() {
			m := ttype.(*sema.Map)
			cname = "std::map<" + g.typeName(m.KeyType(), inTypedef, false) + ", " + g.typeName(m.ValType(), inTypedef, false) + "> "
		} else if ttype.IsSet() {
			s := ttype.(*sema.Set)
			cname = "std::set<" + g.typeName(s.ElemType(), inTypedef, false) + "> "
		} else if ttype.IsList() {
			l := ttype.(*sema.List)
			cname = "std::vector<" + g.typeName(l.ElemType(), inTypedef, false) + "> "
		}
		if arg {
			return "const " + cname + "&"
		}
		return cname
	}

	classPrefix := ""
	if inTypedef && (ttype.IsStruct() || ttype.IsXception()) {
		classPrefix = "class "
	}

	var pname string
	program := ttype.Program()
	if program != nil && program != g.program {
		pname = classPrefix + namespacePrefix(program.Namespace("cpp")) + ttype.Name()
	} else {
		pname = classPrefix + ttype.Name()
	}

	if ttype.IsEnum() && !g.opts.PureEnums {
		pname += "::type"
	}

	if arg {
		if g.isComplexType(ttype) {
			return "const " + pname + "&"
		}
		return "const " + pname
	}
	return pname
}

// declareField is t_cpp_generator::declare_field.
func (g *Generator) declareField(f *sema.Field, init, pointer, constant, reference bool) string {
	result := ""
	if constant {
		result += "const "
	}
	result += g.typeName(f.Type(), false, false)
	if isReference(f) {
		result = "::std::shared_ptr<" + result + ">"
	}
	if pointer {
		result += "*"
	}
	if reference {
		result += "&"
	}
	result += " " + f.Name()
	if init {
		t := sema.TrueType(f.Type())
		if cv := f.Value(); cv != nil {
			result += " = " + g.renderConstValue(nil, f.Name(), t, cv)
		} else if t.IsBaseType() {
			b := t.(*sema.BaseType)
			switch b.Base() {
			case sema.TypeVoid, sema.TypeString, sema.TypeUUID:
			case sema.TypeBool:
				result += " = false"
			case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
				result += " = 0"
			case sema.TypeDouble:
				result += " = 0.0"
			default:
				emit.Throw("compiler error: no C++ initializer for base type %s", sema.BaseName(b.Base()))
			}
		} else if t.IsEnum() {
			result += " = static_cast<" + g.typeName(t, false, false) + ">(0)"
		}
	}
	if !reference {
		result += ";"
	}
	return result
}

// typeToEnum is t_cpp_generator::type_to_enum.
func typeToEnum(t sema.Type) string {
	t = sema.TrueType(t)
	if t.IsBaseType() {
		b := t.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeVoid:
			emit.Throw("NO T_VOID CONSTRUCT")
		case sema.TypeString:
			return "::apache::thrift::protocol::T_STRING"
		case sema.TypeBool:
			return "::apache::thrift::protocol::T_BOOL"
		case sema.TypeI8:
			return "::apache::thrift::protocol::T_BYTE"
		case sema.TypeI16:
			return "::apache::thrift::protocol::T_I16"
		case sema.TypeI32:
			return "::apache::thrift::protocol::T_I32"
		case sema.TypeI64:
			return "::apache::thrift::protocol::T_I64"
		case sema.TypeDouble:
			return "::apache::thrift::protocol::T_DOUBLE"
		case sema.TypeUUID:
			return "::apache::thrift::protocol::T_UUID"
		}
	} else if t.IsEnum() {
		return "::apache::thrift::protocol::T_I32"
	} else if t.IsStruct() {
		return "::apache::thrift::protocol::T_STRUCT"
	} else if t.IsXception() {
		return "::apache::thrift::protocol::T_STRUCT"
	} else if t.IsMap() {
		return "::apache::thrift::protocol::T_MAP"
	} else if t.IsSet() {
		return "::apache::thrift::protocol::T_SET"
	} else if t.IsList() {
		return "::apache::thrift::protocol::T_LIST"
	}
	emit.Throw("INVALID TYPE IN type_to_enum: %s", t.Name())
	return ""
}

// isStructStorageNotThrowing is t_cpp_generator::is_struct_storage_not_throwing.
func isStructStorageNotThrowing(tstruct *sema.Struct) bool {
	members := append([]*sema.Field{}, tstruct.Members()...)
	for i := 0; i < len(members); i++ {
		t := sema.TrueType(members[i].Type())
		if t.IsEnum() {
			continue
		}
		if t.IsXception() {
			return false
		}
		if t.IsBaseType() {
			b := t.(*sema.BaseType)
			switch b.Base() {
			case sema.TypeBool, sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64, sema.TypeDouble, sema.TypeUUID:
				continue
			default:
				return false
			}
		}
		if t.IsStruct() {
			more := t.(*sema.Struct).Members()
			for _, m := range more {
				found := false
				for _, existing := range members {
					if existing == m {
						found = true
						break
					}
				}
				if !found {
					members = append(members, m)
				}
			}
			continue
		}
		return false
	}
	return true
}

// hasFieldWithDefaultValue is t_cpp_generator::has_field_with_default_value.
func hasFieldWithDefaultValue(tstruct *sema.Struct) bool {
	for _, m := range tstruct.Members() {
		t := sema.TrueType(m.Type())
		if isReference(m) || t.IsString() || t.IsUUID() {
			if m.Value() != nil {
				return true
			}
		}
	}
	return false
}
