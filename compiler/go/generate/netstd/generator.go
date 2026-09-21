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

// Package netstd is a port of t_netstd_generator.cc. Every function keeps
// the name and the emission order of its C++ original, so that the output
// is byte-identical; the parity test in internal/parity holds it to that.
package netstd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

// deepCopyMethodName is DEEP_COPY_METHOD_NAME.
const deepCopyMethodName = "DeepCopy"

// cancellationTokenName is CANCELLATION_TOKEN_NAME.
const cancellationTokenName = "cancellationToken"

// Options are the parsed "--gen netstd:..." options.
type Options struct {
	Union            bool
	Serialize        bool
	WCF              bool
	Pascal           bool
	NoDeepcopy       bool
	AsyncPostfix     bool
	TargetNetVersion int // 0 = any, 8, 9, 10
	// WCFNamespace is wcf_namespace_: the value of whichever of "serial"
	// or "wcf" was parsed last (the C++ compiler iterates a sorted
	// std::map<string,string>, so "wcf" wins over "serial" when both are
	// given, since 's' < 'w').
	WCFNamespace string
}

// ParseOptions is the t_netstd_generator constructor's option loop. The
// C++ compiler iterates a std::map, so options apply in sorted order.
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
		case "union":
			o.Union = true
		case "serial":
			o.Serialize = true
			o.WCFNamespace = value // since there can be only one namespace
		case "wcf":
			o.WCF = true
			o.WCFNamespace = value
		case "pascal":
			o.Pascal = true
		case "no_deepcopy":
			o.NoDeepcopy = true
		case "net10":
			o.TargetNetVersion = 10
		case "net9":
			o.TargetNetVersion = 9
		case "net8":
			o.TargetNetVersion = 8
		case "async_postfix":
			o.AsyncPostfix = true
		default:
			return o, &emit.Error{Msg: "unknown option netstd:" + key}
		}
	}
	return o, nil
}

// OutDirBase is out_dir_base_.
func (o Options) OutDirBase() string { return "gen-netstd" }

// netstdKeywords is CSHARP_KEYWORDS.
var netstdKeywords = map[string]bool{}

func init() {
	for _, k := range []string{
		// C# keywords
		"abstract", "as", "base", "bool", "break", "byte", "case", "catch", "char", "checked", "class", "const", "continue",
		"decimal", "default", "delegate", "do", "double", "else", "enum", "event", "explicit", "extern", "false", "finally",
		"fixed", "float", "for", "foreach", "goto", "if", "implicit", "in", "int", "interface", "internal", "is", "lock",
		"long", "namespace", "new", "null", "object", "operator", "out", "override", "params", "private", "protected",
		"public", "readonly", "ref", "return", "sbyte", "sealed", "short", "sizeof", "stackalloc", "static", "string",
		"struct", "switch", "this", "throw", "true", "try", "typeof", "uint", "ulong", "unchecked", "unsafe", "ushort",
		"using", "virtual", "void", "volatile", "while",
		// C# contextual keywords
		"add", "alias", "ascending", "async", "await", "descending", "dynamic", "from", "get", "global", "group", "into",
		"join", "let", "orderby", "partial", "remove", "select", "set", "value", "var", "when", "where", "yield",
	} {
		netstdKeywords[k] = true
	}
}

// memberMappingScope is member_mapping_scope.
type memberMappingScope struct {
	mapping map[string]string
}

// Generator is t_netstd_generator.
type Generator struct {
	program     *sema.Program
	opts        Options
	programName string
	serviceName string
	indentLevel int
	tmpCounter  int

	namespaceName string
	namespaceDir  string

	memberMappingScopes []memberMappingScope

	collectedExtensionTypes  map[string]sema.Type
	checkedExtensionTypes    map[string]sema.Type
	inheritedExtensionOwners map[string]*sema.Program
	extensionsOwner          *sema.Program
}

// New is the t_netstd_generator constructor.
func New(program *sema.Program, opts Options) *Generator {
	return &Generator{
		program:                  program,
		opts:                     opts,
		programName:              program.Name(),
		extensionsOwner:          program,
		collectedExtensionTypes:  map[string]sema.Type{},
		checkedExtensionTypes:    map[string]sema.Type{},
		inheritedExtensionOwners: map[string]*sema.Program{},
	}
}

// Generate is t_generator::generate_program.
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

// ---- t_generator / t_oop_generator helpers ----

func (g *Generator) indentUp()   { g.indentLevel++ }
func (g *Generator) indentDown() { g.indentLevel-- }

func (g *Generator) indent() string {
	return strings.Repeat("  ", g.indentLevel)
}

func (g *Generator) scopeUp(out *strings.Builder) {
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
}

func (g *Generator) scopeDown(out *strings.Builder) {
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

// resetIndent is reset_indent().
func (g *Generator) resetIndent() {
	g.indentLevel = 0
}

func (g *Generator) tmp(name string) string {
	s := name + strconv.Itoa(g.tmpCounter)
	g.tmpCounter++
	return s
}

func (g *Generator) outDir() string {
	if g.program.IsOutPathAbsolute() {
		return g.program.OutPath() + "/"
	}
	return g.program.OutPath() + g.opts.OutDirBase() + "/"
}

func autogenSummary() string {
	return "Autogenerated by Thrift Compiler (" + version.Version + ")"
}

// autogenComment is the netstd override of autogen_comment().
func (g *Generator) autogenComment() string {
	comment := "/**\n"
	if g.opts.TargetNetVersion < 6 {
		comment += " * <auto-generated>\n"
	}
	comment += " * " + autogenSummary() + "\n"
	comment += " * DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING\n"
	if g.opts.TargetNetVersion < 6 {
		comment += " * </auto-generated>\n"
	}
	comment += " */\n"
	return comment
}

func trueType(t sema.Type) sema.Type { return sema.TrueType(t) }

func baseOf(t sema.Type) sema.BaseKind { return t.(*sema.BaseType).Base() }

func itoa(i int64) string { return strconv.FormatInt(i, 10) }

// escapedString is get_escaped_string.
func escapedString(cv *sema.ConstValue) string { return emit.EscapeString(cv.String()) }

// formatDouble mimics `ostream << double` with the default precision of
// six significant digits.
func formatDouble(f float64) string {
	return strconv.FormatFloat(f, 'g', 6, 64)
}

// ---- predicates ----

// fieldHasDefault is field_has_default.
func fieldHasDefault(f *sema.Field) bool { return f.Value() != nil }

// fieldIsRequired is field_is_required.
func fieldIsRequired(f *sema.Field) bool { return f.Req() == sema.Required }

// typeCanBeNull is type_can_be_null.
func typeCanBeNull(t sema.Type) bool {
	t = trueType(t)
	return t.IsContainer() || t.IsStruct() || t.IsXception() || t.IsString()
}

func isDeprecated(a sema.Annotations) bool { return a.Has("deprecated") }

func (g *Generator) isWCFEnabled() bool       { return g.opts.WCF }
func (g *Generator) isSerializeEnabled() bool { return g.opts.Serialize }
func (g *Generator) isUnionEnabled() bool     { return g.opts.Union }

// isNullableType is is_nullable_type.
func isNullableType(t sema.Type) bool {
	t = trueType(t)
	if t.IsEnum() {
		return false
	}
	if t.IsBaseType() {
		return baseOf(t) == sema.TypeString
	}
	return true
}

// nullableSuffix is nullable_suffix(): unconditionally "?" from net6 up.
func (g *Generator) nullableSuffix() string {
	if g.opts.TargetNetVersion >= 6 {
		return "?"
	}
	return ""
}

// nullableFieldSuffixField is nullable_field_suffix(t_field*).
func (g *Generator) nullableFieldSuffixField(f *sema.Field) string {
	if fieldIsRequired(f) && !g.forceMemberNullable(f) {
		return ""
	}
	return g.nullableFieldSuffixType(f.Type())
}

// nullableFieldSuffixType is nullable_field_suffix(t_type*).
func (g *Generator) nullableFieldSuffixType(t sema.Type) string {
	if g.opts.TargetNetVersion < 6 {
		return ""
	}
	t = trueType(t)
	if t.IsEnum() {
		return ""
	}
	if t.IsBaseType() {
		if baseOf(t) == sema.TypeString {
			return g.nullableSuffix()
		}
		return ""
	}
	return g.nullableSuffix()
}

// nullableValueAccess is nullable_value_access.
func (g *Generator) nullableValueAccess(t sema.Type) string {
	if g.opts.TargetNetVersion < 6 {
		return ""
	}
	t = trueType(t)
	// This uses the null-forgiving operator and therefore assumes that
	// the variable has been properly checked against an isset guard or
	// null.
	if t.IsBaseType() {
		if baseOf(t) == sema.TypeString {
			return "!"
		}
		return ""
	}
	if t.IsContainer() || t.IsStruct() || t.IsXception() {
		return "!"
	}
	return ""
}

// forceMemberNullable is force_member_nullable.
//
// IMPORTANT: if tfield is a struct that contains a required field of the
// same type (directly or indirectly), auto-initializing such a member
// field would immediately produce an OOM, or at least unexpectedly
// allocate potentially large amounts of memory -> ALWAYS leave containers
// and struct members nullable.
func (g *Generator) forceMemberNullable(f *sema.Field) bool {
	t := trueType(f.Type())
	return t.IsStruct() || t.IsContainer()
}

// ---- naming ----

// normalizeName is normalize_name.
func (g *Generator) normalizeName(name string, isArgName bool) string {
	tmpLower := strings.ToLower(name)

	// check for reserved argument names
	if isArgName && name == cancellationTokenName {
		name += "_"
	}

	// un-conflict keywords by prefixing with "@"
	if netstdKeywords[tmpLower] {
		return "@" + name
	}

	// prevent CS8981 "The type name only contains lower-cased ascii characters"
	allLower := true
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c < 'a' || c > 'z' {
			allLower = false
			break
		}
	}
	if allLower {
		return "@" + name
	}

	// no changes necessary
	return name
}

// makeValidCSharpIdentifier is make_valid_csharp_identifier.
func makeValidCSharpIdentifier(from string) string {
	if from == "" {
		return from
	}
	b := []byte(from)
	if c := b[0]; '0' <= c && c <= '9' {
		b = append([]byte{'_'}, b...)
	}
	for i, c := range b {
		if !(('A' <= c && c <= 'Z') || ('a' <= c && c <= 'z') || ('0' <= c && c <= '9') || c == '_') {
			b[i] = '_'
		}
	}
	return string(b)
}

// makeCSharpStringLiteral is make_csharp_string_literal.
func makeCSharpStringLiteral(value string) string {
	if value == "" {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(`"`)
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c < 32:
			// convert ctrl chars, but leave UTF-8 alone
			fmt.Fprintf(&sb, "\\x%04x", c)
		case c == '\\' || c == '"':
			sb.WriteByte('\\')
			sb.WriteByte(c)
		default:
			sb.WriteByte(c)
		}
	}
	sb.WriteString(`"`)
	return sb.String()
}

// convertToPascalCase is convert_to_pascal_case.
func convertToPascalCase(str string) string {
	var out []byte
	mustCapitalize := true
	firstCharacter := true
	for i := 0; i < len(str); i++ {
		c := str[i]
		if isAlnum(c) {
			if mustCapitalize {
				out = append(out, toUpper(c))
				mustCapitalize = false
			} else {
				out = append(out, c)
			}
		} else {
			if firstCharacter {
				// this is a private variable and should not be PascalCased
				return str
			}
			mustCapitalize = true
		}
		firstCharacter = false
	}
	return string(out)
}

func isAlnum(c byte) bool {
	return ('0' <= c && c <= '9') || ('A' <= c && c <= 'Z') || ('a' <= c && c <= 'z')
}

func toUpper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 'a' + 'A'
	}
	return c
}

// getIssetName is get_isset_name.
func getIssetName(str string) string {
	if str != "Isset" {
		return str
	}
	return str + "_"
}

// propName is prop_name.
func (g *Generator) propName(f *sema.Field, suppressMapping bool) string {
	name := f.Name()
	if suppressMapping {
		b := []byte(name)
		if len(b) > 0 {
			b[0] = toUpper(b[0])
		}
		name = string(b)
		if g.opts.Pascal {
			name = convertToPascalCase(name)
		}
	} else {
		name = g.getMappedMemberName(name)
	}
	return name
}

// funcName is func_name(t_function*, bool) / func_name(string, bool).
func (g *Generator) funcName(name string, suppressMapping bool) string {
	if suppressMapping {
		return name
	}
	return g.getMappedMemberName(name)
}

// ---- member name mapping ----

func (g *Generator) getMappedMemberName(name string) string {
	if len(g.memberMappingScopes) > 0 {
		top := g.memberMappingScopes[len(g.memberMappingScopes)-1]
		if v, ok := top.mapping[name]; ok {
			return v
		}
	}
	return name
}

// prepareMemberNameMappingStruct is prepare_member_name_mapping(t_struct*).
func (g *Generator) prepareMemberNameMappingStruct(s *sema.Struct) {
	g.prepareMemberNameMappingFields(s.Members(), s.Name())
}

// prepareMemberNameMappingService is prepare_member_name_mapping(t_service*).
func (g *Generator) prepareMemberNameMappingService(s *sema.Service) {
	g.prepareMemberNameMappingFunctions(s.Functions(), s.Name())
}

// prepareMemberNameMappingFields is
// prepare_member_name_mapping(t_struct*, vector<t_field*>&, string&).
func (g *Generator) prepareMemberNameMappingFields(members []*sema.Field, structName string) {
	scope := memberMappingScope{mapping: map[string]string{}}

	// current C# generator policy:
	// - prop names are always rendered with an Uppercase first letter
	// - struct names are used as given
	used := map[string]bool{}

	// prevent name conflicts with struct (CS0542 error + THRIFT-2942)
	used[structName] = true
	used["Isset"] = true
	used["Read"] = true
	used["Write"] = true

	for _, f := range members {
		oldname := f.Name()
		newname := g.propName(f, true)
		for used[newname] {
			// new name conflicts with another member
			newname += "_"
		}
		// add always, this helps us to detect edge cases like different
		// spellings ("foo" and "Foo") within the same struct
		scope.mapping[oldname] = newname
		used[newname] = true
	}

	g.memberMappingScopes = append(g.memberMappingScopes, scope)
}

// prepareMemberNameMappingFunctions is
// prepare_member_name_mapping(t_service*, vector<t_function*>&, string&).
func (g *Generator) prepareMemberNameMappingFunctions(members []*sema.Function, structName string) {
	scope := memberMappingScope{mapping: map[string]string{}}

	used := map[string]bool{}

	// prevent name conflicts with service/intf
	used[structName] = true
	used["Client"] = true
	used["IAsync"] = true
	used["AsyncProcessor"] = true
	used["InternalStructs"] = true

	for _, f := range members {
		oldname := f.Name()
		newname := g.funcName(f.Name(), true)
		for used[newname] {
			// new name conflicts with another method
			newname += "_"
		}
		scope.mapping[oldname] = newname
		used[newname] = true
	}

	g.memberMappingScopes = append(g.memberMappingScopes, scope)
}

// cleanupMemberNameMapping is cleanup_member_name_mapping.
func (g *Generator) cleanupMemberNameMapping() {
	g.memberMappingScopes = g.memberMappingScopes[:len(g.memberMappingScopes)-1]
}

// ---- namespace ----

func (g *Generator) initGenerator() {
	emit.Mkdir(g.outDir())

	g.namespaceName = g.program.Namespace("netstd")

	dir := g.namespaceName
	subdir := g.outDir()
	for {
		loc := strings.IndexByte(dir, '.')
		if loc < 0 {
			break
		}
		subdir = subdir + "/" + dir[:loc]
		emit.Mkdir(subdir)
		dir = dir[loc+1:]
	}
	if len(dir) > 0 {
		subdir = subdir + "/" + dir
		emit.Mkdir(subdir)
	}
	g.namespaceDir = subdir

	// has to be known before the first call site is written, not just
	// before the extensions file is
	visited := map[*sema.Program]bool{}
	g.inheritedExtensionOwners = map[string]*sema.Program{}
	g.collectInheritedExtensionsTypes(g.program, visited, g.inheritedExtensionOwners)
}

func (g *Generator) closeGenerator() {
	// right at the end, after everything else
	g.generateExtensionsFile()
}

// startNetstdNamespace is start_netstd_namespace.
func (g *Generator) startNetstdNamespace(out *strings.Builder) {
	if g.namespaceName == "" {
		return
	}
	var normalized string
	for _, part := range strings.Split(g.namespaceName, ".") {
		if len(normalized) > 0 {
			normalized += "."
		}
		normalized += g.normalizeName(part, false)
	}
	out.WriteString("namespace " + normalized + "\n")
	g.scopeUp(out)
}

// endNetstdNamespace is end_netstd_namespace.
func (g *Generator) endNetstdNamespace(out *strings.Builder) {
	if g.namespaceName == "" {
		return
	}
	g.scopeDown(out)
}

func (g *Generator) netstdTypeUsings() string {
	namespaces := "using System;\n" +
		"using System.Collections;\n" +
		"using System.Collections.Generic;\n" +
		"using System.Text;\n" +
		"using System.IO;\n" +
		"using System.Linq;\n" +
		"using System.Threading;\n" +
		"using System.Threading.Tasks;\n" +
		"using Microsoft.Extensions.Logging;\n" +
		"using Thrift;\n" +
		"using Thrift.Collections;\n"

	if g.isWCFEnabled() {
		namespaces += "using System.ServiceModel;\n"
	}
	if g.isWCFEnabled() || g.isSerializeEnabled() {
		namespaces += "using System.Runtime.Serialization;\n"
	}

	return namespaces
}

func (g *Generator) netstdThriftUsings() string {
	return "using Thrift.Protocol;\n" +
		"using Thrift.Protocol.Entities;\n" +
		"using Thrift.Protocol.Utilities;\n" +
		"using Thrift.Transport;\n" +
		"using Thrift.Transport.Client;\n" +
		"using Thrift.Transport.Server;\n" +
		"using Thrift.Processor;\n"
}

// pragmasAndDirectives is pragmas_and_directives.
func (g *Generator) pragmasAndDirectives(out *strings.Builder) {
	v := g.opts.TargetNetVersion
	switch {
	case v >= 10:
		out.WriteString("// targeting net 10\n")
		out.WriteString("#if( !NET10_0_OR_GREATER)\n")
	case v >= 9:
		out.WriteString("// targeting net 9\n")
		out.WriteString("#if( NET10_0_OR_GREATER || !NET9_0_OR_GREATER)\n")
	case v >= 8:
		out.WriteString("// targeting net 8\n")
		out.WriteString("#if( NET9_0_OR_GREATER || !NET8_0_OR_GREATER)\n")
	default:
		out.WriteString("// targeting netstandard 2.x\n")
		out.WriteString("#if(! NETSTANDARD2_0_OR_GREATER)\n")
	}
	out.WriteString("#error Unexpected target platform. See 'thrift --help' for details.\n")
	out.WriteString("#endif\n")
	out.WriteString("\n")

	if v >= 6 {
		out.WriteString("// Thrift code generated for net" + itoa(int64(v)) + "\n")
		out.WriteString("#nullable enable                 // requires C# 8.0\n")
	}

	// this one must be first
	out.WriteString("#pragma warning disable IDE0079  // remove unnecessary pragmas\n")

	if v >= 9 {
		out.WriteString("#pragma warning disable IDE0130  // unexpected folder structure\n")
	}

	if v < 8 {
		out.WriteString("#pragma warning disable IDE0017  // object init can be simplified\n")
		out.WriteString("#pragma warning disable IDE0028  // collection init can be simplified\n")
		out.WriteString("#pragma warning disable IDE0305  // collection init can be simplified\n")
		out.WriteString("#pragma warning disable IDE0034  // simplify default expression\n")
		out.WriteString("#pragma warning disable IDE0066  // use switch expression\n")
		out.WriteString("#pragma warning disable IDE0090  // simplify new expression\n")
	}

	out.WriteString("#pragma warning disable IDE0290  // use primary CTOR\n")
	out.WriteString("#pragma warning disable IDE1006  // parts of the code use IDL spelling\n")
	out.WriteString("#pragma warning disable CA1822   // empty " + deepCopyMethodName + "() methods still non-static\n")

	if g.anyDeprecations() {
		out.WriteString("#pragma warning disable CS0618   // silence our own deprecation warnings\n")
	}

	if v < 6 {
		out.WriteString("#pragma warning disable IDE0083  // pattern matching \"that is not SomeType\" requires net5.0 but we still support earlier versions\n")
	}
	out.WriteString("\n")
}

// anyDeprecations is any_deprecations.
func (g *Generator) anyDeprecations() bool {
	for _, e := range g.program.Enums() {
		if isDeprecated(e.Annotations()) {
			return true
		}
		for _, v := range e.Constants() {
			if isDeprecated(v.Annotations()) {
				return true
			}
		}
	}
	for _, td := range g.program.Typedefs() {
		if isDeprecated(td.Annotations()) {
			return true
		}
	}
	for _, o := range g.program.Objects() {
		if isDeprecated(o.Annotations()) {
			return true
		}
		for _, m := range o.Members() {
			if isDeprecated(m.Annotations()) {
				return true
			}
		}
	}
	for _, s := range g.program.Services() {
		if isDeprecated(s.Annotations()) {
			return true
		}
		for _, f := range s.Functions() {
			if isDeprecated(f.Annotations()) {
				return true
			}
		}
	}
	return false
}

// generateDeprecationAttribute is generate_deprecation_attribute.
func (g *Generator) generateDeprecationAttribute(out *strings.Builder, a sema.Annotations) {
	if !a.Has("deprecated") {
		return
	}
	values := a["deprecated"]
	last := values[len(values)-1]
	out.WriteString(g.indent() + "[Obsolete")
	// empty annotation values end up with "1" somewhere, ignore these as well
	if len(last) > 0 && last != "1" {
		out.WriteString("(" + makeCSharpStringLiteral(last) + ")")
	} else {
		out.WriteString("(" + makeCSharpStringLiteral("This code is deprecated.") + ")") // generic message to prevent CA1041
	}
	out.WriteString("]\n")
}

// isDeprecatedAnnotations is is_deprecated.
func isDeprecatedAnnotations(a sema.Annotations) bool { return a.Has("deprecated") }

// getEnumClassName is the netstd override of get_enum_class_name.
func (g *Generator) getEnumClassName(t sema.Type) string {
	pkg := ""
	if p := t.Program(); p != nil {
		pkg = p.Namespace("netstd") + "."
	}
	return "global::" + pkg + t.Name()
}

// ---- docs ----

func (g *Generator) netstdDocstringComment(out *strings.Builder, contents string) {
	emit.DocstringComment(out, g.indent(), "/// <summary>\n", "/// ", contents, "/// </summary>\n")
}

// netstdDocField is generate_netstd_doc(ostream&, t_field*). Note this
// checks the field's declared type directly, not its true type: a field
// typed through a typedef to an enum does not get the <seealso> comment,
// matching field->get_type()->is_enum() in the C++ source.
func (g *Generator) netstdDocField(out *strings.Builder, f *sema.Field) {
	if f.Type().IsEnum() {
		combined := f.Doc() + "\n" + "<seealso cref=\"" + g.getEnumClassName(f.Type()) + "\"/>"
		g.netstdDocstringComment(out, combined)
	} else {
		g.netstdDoc(out, f)
	}
}

// netstdDoc is generate_netstd_doc(ostream&, t_doc*).
func (g *Generator) netstdDoc(out *strings.Builder, doc interface {
	HasDoc() bool
	Doc() string
}) {
	if doc.HasDoc() {
		g.netstdDocstringComment(out, doc.Doc())
	}
}

// netstdDocFunction is generate_netstd_doc(ostream&, t_function*).
func (g *Generator) netstdDocFunction(out *strings.Builder, f *sema.Function) {
	if !f.HasDoc() {
		return
	}
	var ps strings.Builder
	for _, p := range f.Arglist().Members() {
		ps.WriteString("\n<param name=\"" + p.Name() + "\">")
		if p.HasDoc() {
			ps.WriteString(strings.ReplaceAll(p.Doc(), "\n", ""))
		}
		ps.WriteString("</param>")
	}
	emit.DocstringComment(out, g.indent(), "", "/// ", "<summary>\n"+f.Doc()+"</summary>"+ps.String(), "")
}
