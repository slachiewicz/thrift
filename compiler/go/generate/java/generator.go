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

// Package java is a port of t_java_generator.cc. Every function keeps the
// name and the emission order of its C++ original, so that the output is
// byte-identical; the parity test in internal/parity holds it to that.
package java

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

const (
	thriftOptionClass = "org.apache.thrift.Option"
	jdkOptionClass    = "java.util.Optional"
)

// Options are the parsed "--gen java:..." options.
type Options struct {
	BeanStyle                   bool
	AndroidStyle                bool
	PrivateMembers              bool
	NocamelStyle                bool
	FullcamelStyle              bool
	AndroidLegacy               bool
	Java5                       bool
	SortedContainers            bool
	ReuseObjects                bool
	GenerateFutureIface         bool
	UseOptionType               bool
	UseJdk8OptionType           bool
	UndatedGeneratedAnnotations bool
	SuppressGeneratedAnnotation bool
	RethrowUnhandledExceptions  bool
	UnsafeBinaries              bool
	AnnotationsAsMetadata       bool
	JakartaAnnotations          bool
}

// ParseOptions is the t_java_generator constructor's option loop. The
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
		case "beans":
			o.BeanStyle = true
		case "android":
			o.AndroidStyle = true
		case "private_members", "private-members":
			o.PrivateMembers = true
		case "nocamel":
			o.NocamelStyle = true
		case "fullcamel":
			o.FullcamelStyle = true
		case "android_legacy":
			o.AndroidLegacy = true
		case "sorted_containers":
			o.SortedContainers = true
		case "java5":
			o.Java5 = true
		case "future_iface":
			o.GenerateFutureIface = true
		case "reuse_objects", "reuse-objects":
			o.ReuseObjects = true
		case "option_type":
			o.UseOptionType = true
			switch value {
			case "jdk8":
				o.UseJdk8OptionType = true
			case "thrift", "":
				o.UseJdk8OptionType = false
			default:
				return o, &emit.Error{Msg: "option_type must be 'jdk8' or 'thrift'"}
			}
		case "rethrow_unhandled_exceptions":
			o.RethrowUnhandledExceptions = true
		case "generated_annotations":
			switch value {
			case "undated":
				o.UndatedGeneratedAnnotations = true
			case "suppress":
				o.SuppressGeneratedAnnotation = true
			default:
				return o, &emit.Error{Msg: "unknown option java:" + key + "=" + value}
			}
		case "unsafe_binaries":
			o.UnsafeBinaries = true
		case "annotations_as_metadata":
			o.AnnotationsAsMetadata = true
		case "jakarta_annotations":
			o.JakartaAnnotations = true
		default:
			return o, &emit.Error{Msg: "unknown option java:" + key}
		}
	}
	if o.Java5 {
		o.AndroidLegacy = true
	}
	return o, nil
}

// OutDirBase is out_dir_base_: the gen-* directory used without -out.
func (o Options) OutDirBase() string {
	if o.BeanStyle {
		return "gen-javabean"
	}
	return "gen-java"
}

var javaKeywords = map[string]bool{}

func init() {
	for _, k := range []string{
		"abstract", "assert", "boolean", "break", "byte", "case", "catch", "char", "class", "const", "continue",
		"default", "do", "double", "else", "enum", "extends", "final", "finally", "float", "for", "goto", "if",
		"implements", "import", "instanceof", "int", "interface", "long", "native", "new", "package", "private",
		"protected", "public", "return", "short", "static", "strictfp", "super", "switch", "synchronized", "this",
		"throw", "throws", "transient", "try", "void", "volatile", "while", "true", "false", "null",
	} {
		javaKeywords[k] = true
	}
}

type issetType int

const (
	issetNone issetType = iota
	issetPrimitive
	issetBitset
)

// Generator is t_java_generator.
type Generator struct {
	program     *sema.Program
	opts        Options
	programName string
	serviceName string
	indentLevel int
	tmpCounter  int

	packageName  string
	packageDir   string
	fService     strings.Builder
	fServiceName string
}

// New is the t_java_generator constructor.
func New(program *sema.Program, opts Options) *Generator {
	return &Generator{program: program, opts: opts, programName: program.Name()}
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

// ---- t_generator and t_oop_generator helpers ----

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

func autogenComment() string {
	return "/**\n" + " * " + autogenSummary() + "\n" + " *\n" +
		" * DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING\n" +
		" *  @generated\n" + " */\n"
}

func upcaseString(s string) string {
	b := []byte(s)
	for i, c := range b {
		b[i] = toUpper(c)
	}
	return string(b)
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

func itoa(i int64) string { return strconv.FormatInt(i, 10) }

func escapedString(cv *sema.ConstValue) string { return emit.EscapeString(cv.String()) }

func trueType(t sema.Type) sema.Type { return sema.TrueType(t) }

func baseOf(t sema.Type) sema.BaseKind { return t.(*sema.BaseType).Base() }

// ---- docs (t_oop_generator) ----

func (g *Generator) enumClassName(t sema.Type) string {
	pkg := ""
	if p := t.Program(); p != nil && p != g.program {
		pkg = p.Namespace("java") + "."
	}
	return pkg + t.Name()
}

func (g *Generator) javaDocstringComment(out *strings.Builder, contents string) {
	emit.DocstringComment(out, g.indent(), "/**\n", " * ", contents, " */\n")
}

// javaDocField is generate_java_doc(ostream&, t_field*).
func (g *Generator) javaDocField(out *strings.Builder, f *sema.Field) {
	if trueType(f.Type()).IsEnum() {
		g.javaDocstringComment(out, f.Doc()+"\n@see "+g.enumClassName(f.Type()))
	} else {
		g.javaDoc(out, f)
	}
}

// javaDoc is generate_java_doc(ostream&, t_doc*).
func (g *Generator) javaDoc(out *strings.Builder, doc interface {
	HasDoc() bool
	Doc() string
}) {
	if doc.HasDoc() {
		g.javaDocstringComment(out, doc.Doc())
	}
}

// javaDocFunction is generate_java_doc(ostream&, t_function*).
func (g *Generator) javaDocFunction(out *strings.Builder, f *sema.Function) {
	if f.HasDoc() {
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
}

// ---- init and close ----

func (g *Generator) initGenerator() {
	emit.Mkdir(g.outDir())
	g.packageName = g.program.Namespace("java")

	dir := g.packageName
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
	g.packageDir = subdir
}

func (g *Generator) javaPackage() string {
	if g.packageName != "" {
		return "package " + g.packageName + ";\n\n"
	}
	return ""
}

func javaSuppressions() string {
	return "@SuppressWarnings({\"cast\", \"rawtypes\", \"serial\", \"unchecked\", \"unused\"})\n"
}

func javaNullableAnnotation() string { return "@org.apache.thrift.annotation.Nullable" }

func javaOverrideAnnotation() string { return "@Override" }

func normalizeName(name string) string {
	if javaKeywords[name] {
		return "$" + name
	}
	return name
}

func (g *Generator) closeGenerator() {}

// ---- predicates ----

func typeCanBeNull(t sema.Type) bool {
	t = trueType(t)
	return t.IsContainer() || t.IsStruct() || t.IsXception() || t.IsString() || t.IsUUID() || t.IsEnum()
}

func isDeprecated(a sema.Annotations) bool { return a.Has("deprecated") }

func (g *Generator) isEnumSet(t sema.Type) bool {
	if !g.opts.SortedContainers {
		t = trueType(t)
		if t.IsSet() {
			return trueType(t.(*sema.Set).ElemType()).IsEnum()
		}
	}
	return false
}

func (g *Generator) isEnumMap(t sema.Type) bool {
	if !g.opts.SortedContainers {
		t = trueType(t)
		if t.IsMap() {
			return trueType(t.(*sema.Map).KeyType()).IsEnum()
		}
	}
	return false
}

func (g *Generator) innerEnumTypeName(t sema.Type) string {
	t = trueType(t)
	if t.IsMap() {
		return g.typeName(trueType(t.(*sema.Map).KeyType()), true, false, false, false) + ".class"
	} else if t.IsSet() {
		return g.typeName(trueType(t.(*sema.Set).ElemType()), true, false, false, false) + ".class"
	}
	return ""
}

// ---- type names ----

// typeName is type_name(ttype, in_container, in_init, skip_generic, force_namespace).
func (g *Generator) typeName(t sema.Type, inContainer, inInit, skipGeneric, forceNamespace bool) string {
	t = trueType(t)
	prefix := ""
	if t.IsBaseType() {
		return baseTypeName(t.(*sema.BaseType), inContainer)
	} else if t.IsMap() {
		m := t.(*sema.Map)
		if inInit {
			if g.isEnumMap(m) {
				prefix = "java.util.EnumMap"
			} else if g.opts.SortedContainers {
				prefix = "java.util.TreeMap"
			} else {
				prefix = "java.util.HashMap"
			}
		} else {
			prefix = "java.util.Map"
		}
		if skipGeneric {
			return prefix
		}
		return prefix + "<" + g.typeName(m.KeyType(), true, false, false, false) + "," + g.typeName(m.ValType(), true, false, false, false) + ">"
	} else if t.IsSet() {
		s := t.(*sema.Set)
		if inInit {
			if g.isEnumSet(s) {
				prefix = "java.util.EnumSet"
			} else if g.opts.SortedContainers {
				prefix = "java.util.TreeSet"
			} else {
				prefix = "java.util.HashSet"
			}
		} else {
			prefix = "java.util.Set"
		}
		if skipGeneric {
			return prefix
		}
		return prefix + "<" + g.typeName(s.ElemType(), true, false, false, false) + ">"
	} else if t.IsList() {
		l := t.(*sema.List)
		if inInit {
			prefix = "java.util.ArrayList"
		} else {
			prefix = "java.util.List"
		}
		if skipGeneric {
			return prefix
		}
		return prefix + "<" + g.typeName(l.ElemType(), true, false, false, false) + ">"
	}

	if p := t.Program(); p != nil && (p != g.program || forceNamespace) {
		if pkg := p.Namespace("java"); pkg != "" {
			return pkg + "." + makeValidJavaIdentifier(t.Name())
		}
	}
	return makeValidJavaIdentifier(t.Name())
}

func (g *Generator) tn(t sema.Type) string { return g.typeName(t, false, false, false, false) }

func baseTypeName(t *sema.BaseType, inContainer bool) string {
	pick := func(boxed, plain string) string {
		if inContainer {
			return boxed
		}
		return plain
	}
	switch t.Base() {
	case sema.TypeVoid:
		return pick("Void", "void")
	case sema.TypeString:
		if t.IsBinary() {
			return "java.nio.ByteBuffer"
		}
		return "java.lang.String"
	case sema.TypeUUID:
		return "java.util.UUID"
	case sema.TypeBool:
		return pick("java.lang.Boolean", "boolean")
	case sema.TypeI8:
		return pick("java.lang.Byte", "byte")
	case sema.TypeI16:
		return pick("java.lang.Short", "short")
	case sema.TypeI32:
		return pick("java.lang.Integer", "int")
	case sema.TypeI64:
		return pick("java.lang.Long", "long")
	case sema.TypeDouble:
		return pick("java.lang.Double", "double")
	}
	emit.Throw("compiler error: no Java name for base type %s", t.Name())
	return ""
}

// declareField is declare_field(tfield, init, comment).
func (g *Generator) declareField(f *sema.Field, init, comment bool) string {
	result := ""
	tt := trueType(f.Type())
	if typeCanBeNull(tt) {
		result += javaNullableAnnotation() + " "
	}
	result += g.tn(f.Type()) + " " + makeValidJavaIdentifier(f.Name())
	if init {
		if tt.IsBaseType() && f.Value() != nil {
			var dummy strings.Builder
			result += " = " + g.renderConstValue(&dummy, tt, f.Value())
		} else if tt.IsBaseType() {
			switch baseOf(tt) {
			case sema.TypeVoid:
				emit.Throw("NO T_VOID CONSTRUCT")
			case sema.TypeString, sema.TypeUUID:
				result += " = null"
			case sema.TypeBool:
				result += " = false"
			case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
				result += " = 0"
			case sema.TypeDouble:
				result += " = (double)0"
			default:
				emit.Throw("compiler error: unhandled type")
			}
		} else if tt.IsEnum() {
			result += " = null"
		} else {
			result += " = new " + g.typeName(tt, false, true, false, false) + "()"
		}
	}
	result += ";"
	if comment {
		result += " // "
		if f.Req() == sema.Optional {
			result += "optional"
		} else {
			result += "required"
		}
	}
	return result
}

func (g *Generator) functionSignature(f *sema.Function, prefix string) string {
	result := g.tn(f.ReturnType()) + " " + prefix + g.rpcMethodName(f.Name()) + "(" + g.argumentList(f.Arglist(), true) + ") throws "
	for _, x := range f.Xceptions().Members() {
		result += g.typeName(x.Type(), false, false, false, false) + ", "
	}
	result += "org.apache.thrift.TException"
	return result
}

func (g *Generator) functionSignatureAsync(f *sema.Function, useBaseMethod bool, prefix string) string {
	arglist := g.asyncFunctionCallArglist(f, useBaseMethod, true)
	return prefix + "void " + g.rpcMethodName(f.Name()) + "(" + arglist + ")"
}

func (g *Generator) functionSignatureFuture(f *sema.Function, prefix string) string {
	return "java.util.concurrent.CompletableFuture<" + g.typeName(f.ReturnType(), true, false, false, false) + "> " +
		prefix + g.rpcMethodName(f.Name()) + "(" + g.argumentList(f.Arglist(), true) + ")"
}

func (g *Generator) asyncFunctionCallArglist(f *sema.Function, useBaseMethod, includeTypes bool) string {
	_ = useBaseMethod
	arglist := ""
	if len(f.Arglist().Members()) > 0 {
		arglist = g.argumentList(f.Arglist(), includeTypes) + ", "
	}
	if includeTypes {
		arglist += "org.apache.thrift.async.AsyncMethodCallback<"
		arglist += g.typeName(f.ReturnType(), true, false, false, false) + "> "
	}
	arglist += "resultHandler"
	return arglist
}

func (g *Generator) argumentList(s *sema.Struct, includeTypes bool) string {
	result := ""
	first := true
	for _, f := range s.Members() {
		if first {
			first = false
		} else {
			result += ", "
		}
		if includeTypes {
			result += g.tn(f.Type()) + " "
		}
		result += makeValidJavaIdentifier(f.Name())
	}
	return result
}

func (g *Generator) asyncArgumentList(f *sema.Function, s *sema.Struct, includeTypes bool) string {
	result := ""
	first := true
	for _, m := range s.Members() {
		if first {
			first = false
		} else {
			result += ", "
		}
		if includeTypes {
			result += g.tn(m.Type()) + " "
		}
		result += makeValidJavaIdentifier(m.Name())
	}
	if !first {
		result += ", "
	}
	if includeTypes {
		result += "org.apache.thrift.async.AsyncMethodCallback<"
		result += g.typeName(f.ReturnType(), true, false, false, false) + "> "
	}
	result += "resultHandler"
	return result
}

func (g *Generator) typeToEnum(t sema.Type) string {
	t = trueType(t)
	if t.IsBaseType() {
		switch baseOf(t) {
		case sema.TypeVoid:
			emit.Throw("NO T_VOID CONSTRUCT")
		case sema.TypeString:
			return "org.apache.thrift.protocol.TType.STRING"
		case sema.TypeBool:
			return "org.apache.thrift.protocol.TType.BOOL"
		case sema.TypeI8:
			return "org.apache.thrift.protocol.TType.BYTE"
		case sema.TypeI16:
			return "org.apache.thrift.protocol.TType.I16"
		case sema.TypeI32:
			return "org.apache.thrift.protocol.TType.I32"
		case sema.TypeI64:
			return "org.apache.thrift.protocol.TType.I64"
		case sema.TypeUUID:
			return "org.apache.thrift.protocol.TType.UUID"
		case sema.TypeDouble:
			return "org.apache.thrift.protocol.TType.DOUBLE"
		default:
			emit.Throw("compiler error: unhandled type")
		}
	} else if t.IsEnum() {
		return "org.apache.thrift.protocol.TType.I32"
	} else if t.IsStruct() || t.IsXception() {
		return "org.apache.thrift.protocol.TType.STRUCT"
	} else if t.IsMap() {
		return "org.apache.thrift.protocol.TType.MAP"
	} else if t.IsSet() {
		return "org.apache.thrift.protocol.TType.SET"
	} else if t.IsList() {
		return "org.apache.thrift.protocol.TType.LIST"
	}
	emit.Throw("INVALID TYPE IN type_to_enum: %s", t.Name())
	return ""
}

func makeValidJavaFilename(from string) string { return makeValidJavaIdentifier(from) }

func makeValidJavaIdentifier(from string) string {
	if from == "" {
		return from
	}
	str := []byte(from)
	if c := str[0]; '0' <= c && c <= '9' {
		str = append([]byte{'_'}, str...)
	}
	for i, c := range str {
		if !(('A' <= c && c <= 'Z') || ('a' <= c && c <= 'z') || ('0' <= c && c <= '9') || c == '_') {
			str[i] = '_'
		}
	}
	return normalizeName(string(str))
}

func asCamelCase(name string, ucfirst bool) string {
	var b []byte
	i := 0
	for i = 0; i < len(name); i++ {
		if name[i] != '_' {
			break
		}
	}
	// The C++ code reads name[i] without a bounds check; on an
	// all-underscore name it reads the terminating NUL.
	if i < len(name) {
		if ucfirst {
			b = append(b, toUpper(name[i]))
		} else {
			b = append(b, toLower(name[i]))
		}
	}
	i++
	for ; i < len(name); i++ {
		if name[i] == '_' {
			if i < len(name)-1 {
				i++
				b = append(b, toUpper(name[i]))
			}
		} else {
			b = append(b, name[i])
		}
	}
	return string(b)
}

func (g *Generator) rpcMethodName(name string) string {
	if g.opts.FullcamelStyle {
		return makeValidJavaIdentifier(asCamelCase(name, false))
	}
	return makeValidJavaIdentifier(name)
}

func (g *Generator) capName(name string) string {
	if g.opts.NocamelStyle {
		return "_" + name
	} else if g.opts.FullcamelStyle {
		return asCamelCase(name, true)
	}
	b := []byte(name)
	if len(b) > 0 {
		b[0] = toUpper(b[0])
	}
	return string(b)
}

func constantName(name string) string {
	var b []byte
	isFirst := true
	wasPreviousUpper := false
	for i := 0; i < len(name); i++ {
		c := name[i]
		up := isUpper(c)
		if up && !isFirst && !wasPreviousUpper {
			b = append(b, '_')
		}
		b = append(b, toUpper(c))
		isFirst = false
		wasPreviousUpper = up
	}
	return string(b)
}

func (g *Generator) issetCheck(f *sema.Field) string { return g.issetCheckName(f.Name()) }

func issetFieldID(f *sema.Field) string { return "__" + upcaseString(f.Name()+"_isset_id") }

func (g *Generator) issetCheckName(name string) string {
	return "is" + g.capName("set") + g.capName(name) + "()"
}

func (g *Generator) issetSet(out *strings.Builder, f *sema.Field, prefix string) {
	if !typeCanBeNull(f.Type()) {
		out.WriteString(g.indent() + prefix + "set" + g.capName(f.Name()) + g.capName("isSet") + "(true);\n")
	}
}

func (g *Generator) needsIsset(s *sema.Struct) (issetType, string) {
	count := 0
	for _, m := range s.Members() {
		if !typeCanBeNull(trueType(m.Type())) {
			count++
		}
	}
	if count == 0 {
		return issetNone, ""
	} else if count <= 64 {
		primitive := ""
		switch {
		case count <= 8:
			primitive = "byte"
		case count <= 16:
			primitive = "short"
		case count <= 32:
			primitive = "int"
		default:
			primitive = "long"
		}
		return issetPrimitive, primitive
	}
	return issetBitset, ""
}

// Now supplies the date of the @Generated annotation. Tests that compare
// the output against stored files fix it.
var Now = time.Now

func (g *Generator) generateJavaxGeneratedAnnotation(out *strings.Builder) {
	now := Now()
	if g.opts.JakartaAnnotations {
		out.WriteString(g.indent() + "@jakarta.annotation.Generated(value = \"" + autogenSummary() + "\"")
	} else {
		out.WriteString(g.indent() + "@javax.annotation.Generated(value = \"" + autogenSummary() + "\"")
	}
	if g.opts.UndatedGeneratedAnnotations {
		out.WriteString(")\n")
	} else {
		out.WriteString(g.indent() + ", date = \"" + now.Format("2006-01-02") + "\")\n")
	}
}
