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

package rs

import (
	"sort"
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

// structType is t_rs_generator::e_struct_type.
//
//   - regular: a user-defined struct in the IDL.
//   - args: a struct used to hold all service-call parameters.
//   - result: a struct used to hold all service-call returns and exceptions.
//   - exception: a user-defined exception in the IDL.
type structType int

const (
	structRegular structType = iota
	structArgs
	structResult
	structException
)

// serviceResultVariable is SERVICE_RESULT_VARIABLE.
const serviceResultVariable = "result_value"

// resultStructSuffix is RESULT_STRUCT_SUFFIX.
const resultStructSuffix = "Result"

// syncClientGenericBoundVars is SYNC_CLIENT_GENERIC_BOUND_VARS.
const syncClientGenericBoundVars = "<IP, OP>"

// syncClientGenericBounds is SYNC_CLIENT_GENERIC_BOUNDS.
const syncClientGenericBounds = "where IP: TInputProtocol, OP: TOutputProtocol"

// rustReservedWords is RUST_RESERVED_WORDS_SET.
var rustReservedWords = map[string]bool{}

func init() {
	for _, w := range []string{
		"abstract", "alignof", "as", "become", "box", "break", "const", "continue", "crate",
		"do", "else", "enum", "extern", "false", "final", "fn", "for", "if",
		"impl", "in", "let", "loop", "macro", "match", "mod", "move", "mut",
		"offsetof", "override", "priv", "proc", "pub", "pure", "ref", "return", "Self",
		"self", "sizeof", "static", "struct", "super", "trait", "true", "type", "typeof",
		"unsafe", "unsized", "use", "virtual", "where", "while", "yield",
	} {
		rustReservedWords[w] = true
	}
}

// generator is t_rs_generator for one program.
type generator struct {
	program     *sema.Program
	opts        Options
	cratePrefix string

	buf         strings.Builder
	level       int
	tmpCounter  int
	serviceName string
}

func newGenerator(program *sema.Program, opts Options) *generator {
	crate := opts.CratePrefix
	if crate == "" {
		crate = "crate"
	}
	return &generator{program: program, opts: opts, cratePrefix: crate}
}

// generate is t_generator::generate_program restricted to what
// t_rs_generator overrides.
func (g *generator) generate() (err error) {
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

// ---- indent-tracked writer ----

// ind is indent(): two spaces per level.
func (g *generator) ind() string {
	return strings.Repeat("  ", g.level)
}

func (g *generator) up()   { g.level++ }
func (g *generator) down() { g.level-- }

// raw appends s verbatim, with no indent and no trailing newline.
func (g *generator) raw(s string) { g.buf.WriteString(s) }

// wl is `f_gen_ << s << '\n'` with no leading indent().
func (g *generator) wl(s string) { g.buf.WriteString(s); g.buf.WriteString("\n") }

// line is `f_gen_ << indent() << s << '\n'`.
func (g *generator) line(s string) {
	g.buf.WriteString(g.ind())
	g.buf.WriteString(s)
	g.buf.WriteString("\n")
}

// indentRaw is `f_gen_ << indent() << s` with no trailing newline.
func (g *generator) indentRaw(s string) {
	g.buf.WriteString(g.ind())
	g.buf.WriteString(s)
}

// tmp is t_generator::tmp: a name with a running number appended.
func (g *generator) tmp(name string) string {
	s := name + strconv.Itoa(g.tmpCounter)
	g.tmpCounter++
	return s
}

func throwf(format string, args ...interface{}) {
	emit.Throw(format, args...)
}

// ---- t_generator string helpers shared with the golang port ----

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

// capitalize is t_generator::capitalize.
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

// uppercase is t_generator::uppercase.
func uppercase(in string) string { return strings.ToUpper(in) }

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

// camelcase is the base t_generator::camelcase (not the go generator's
// initialism-aware override): a_multi_word -> aMultiWord.
func camelcase(in string) string {
	var out strings.Builder
	under := false
	for i := 0; i < len(in); i++ {
		c := in[i]
		if c == '_' {
			under = true
			continue
		}
		if under {
			out.WriteByte(toUpperByte(c))
			under = false
			continue
		}
		out.WriteByte(c)
	}
	return out.String()
}

// stringReplace is t_rs_generator::string_replace: every occurrence of
// search is replaced by replace, continuing the scan after the inserted
// replacement.
func stringReplace(target, search, replace string) string {
	if target == "" || search == "" {
		return target
	}
	var out strings.Builder
	rest := target
	for {
		idx := strings.Index(rest, search)
		if idx < 0 {
			out.WriteString(rest)
			break
		}
		out.WriteString(rest[:idx])
		out.WriteString(replace)
		rest = rest[idx+len(search):]
	}
	return out.String()
}

// formatDouble mimics `ostream << double` with the default precision of
// six significant digits, as used by render_const_value's TYPE_DOUBLE
// case (no setprecision call is in effect there).
func formatDouble(f float64) string {
	return strconv.FormatFloat(f, 'g', 6, 64)
}

// ---- naming ----

// isReserved is t_rs_generator::is_reserved.
func isReserved(name string) bool { return rustReservedWords[name] }

// rustSafeName is t_rs_generator::rust_safe_name.
func rustSafeName(name string) string {
	if isReserved(name) {
		return name + "_"
	}
	return name
}

// rustStructName is t_rs_generator::rust_struct_name.
func rustStructName(s *sema.Struct) string {
	return rustSafeName(rustCamelCase(s.Name()))
}

// rustFieldName is t_rs_generator::rust_field_name.
func rustFieldName(f *sema.Field) string {
	return rustSafeName(rustSnakeCase(f.Name()))
}

// rustUnionFieldName is t_rs_generator::rust_union_field_name.
func rustUnionFieldName(f *sema.Field) string {
	return rustSafeName(rustCamelCase(f.Name()))
}

// rustEnumVariantName is t_rs_generator::rust_enum_variant_name.
func rustEnumVariantName(name string) string {
	if isAllUppercase(name) {
		return name
	}
	return stringReplace(uppercase(underscore(name)), "__", "_")
}

// rustUpperCase is t_rs_generator::rust_upper_case.
func rustUpperCase(name string) string {
	if isAllUppercase(name) {
		return name
	}
	return stringReplace(uppercase(underscore(name)), "__", "_")
}

func isAllUppercase(name string) bool {
	for i := 0; i < len(name); i++ {
		c := name[i]
		if isLower(c) {
			return false
		}
	}
	return true
}

// rustSnakeCase is t_rs_generator::rust_snake_case.
func rustSnakeCase(name string) string {
	return stringReplace(decapitalize(underscore(name)), "__", "_")
}

// rustCamelCase is t_rs_generator::rust_camel_case.
func rustCamelCase(name string) string {
	return stringReplace(capitalize(camelcase(name)), "_", "")
}

// rustSafeFieldID is t_rs_generator::rust_safe_field_id.
func rustSafeFieldID(id int32) string {
	n := id
	if n < 0 {
		n = -n
	}
	s := strconv.FormatInt(int64(n), 10)
	if id >= 0 {
		return s
	}
	return "neg" + s
}

// rustNamespaceService is the t_service overload of rust_namespace.
func (g *generator) rustNamespaceService(s *sema.Service) string {
	if s.Program().Name() != g.program.Name() {
		return rustSnakeCase(s.Program().Name()) + "::"
	}
	return ""
}

// rustNamespaceType is the t_type overload of rust_namespace.
func (g *generator) rustNamespaceType(t sema.Type) string {
	if t.Program().Name() != g.program.Name() {
		return rustSnakeCase(t.Program().Name()) + "::"
	}
	return ""
}

// ---- type mapping ----

// isDouble is t_rs_generator::is_double.
func isDouble(t sema.Type) bool {
	t = sema.TrueType(t)
	return t.IsBaseType() && t.(*sema.BaseType).Base() == sema.TypeDouble
}

// isVoidType is t_rs_generator::is_void.
func isVoidType(t sema.Type) bool {
	return t.IsBaseType() && t.(*sema.BaseType).Base() == sema.TypeVoid
}

// isOptional is t_rs_generator::is_optional.
func isOptional(req sema.Requiredness) bool {
	return req == sema.Optional || req == sema.OptInReqOut
}

// actualFieldReq is t_rs_generator::actual_field_req.
func actualFieldReq(f *sema.Field, st structType) sema.Requiredness {
	if st == structArgs {
		return sema.Required
	}
	return f.Req()
}

// hasArgs is t_rs_generator::has_args.
func hasArgs(fn *sema.Function) bool {
	return fn.Arglist() != nil && len(fn.Arglist().SortedMembers()) != 0
}

// hasNonVoidArgs is t_rs_generator::has_non_void_args.
func hasNonVoidArgs(fn *sema.Function) bool {
	for _, f := range fn.Arglist().SortedMembers() {
		if !f.Type().IsVoid() {
			return true
		}
	}
	return false
}

// visibilityQualifier is t_rs_generator::visibility_qualifier.
func visibilityQualifier(st structType) string {
	switch st {
	case structArgs, structResult:
		return ""
	default:
		return "pub "
	}
}

// toRustType is t_rs_generator::to_rust_type.
func (g *generator) toRustType(t sema.Type) string {
	switch {
	case t.IsBaseType():
		b := t.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeVoid:
			return "()"
		case sema.TypeString:
			if b.IsBinary() {
				return "Vec<u8>"
			}
			return "String"
		case sema.TypeUUID:
			return "uuid::Uuid"
		case sema.TypeBool:
			return "bool"
		case sema.TypeI8:
			return "i8"
		case sema.TypeI16:
			return "i16"
		case sema.TypeI32:
			return "i32"
		case sema.TypeI64:
			return "i64"
		case sema.TypeDouble:
			return "OrderedFloat<f64>"
		default:
			throwf("compiler error: unhandled type")
		}
	case t.IsTypedef():
		td := t.(*sema.Typedef)
		rustType := g.rustNamespaceType(t) + td.Symbolic()
		if td.IsForwardTypedef() {
			rustType = "Box<" + rustType + ">"
		}
		return rustType
	case t.IsEnum():
		return g.rustNamespaceType(t) + rustCamelCase(t.Name())
	case t.IsStruct(), t.IsXception():
		return g.rustNamespaceType(t) + rustCamelCase(t.Name())
	case t.IsMap():
		m := t.(*sema.Map)
		return "BTreeMap<" + g.toRustType(m.KeyType()) + ", " + g.toRustType(m.ValType()) + ">"
	case t.IsSet():
		s := t.(*sema.Set)
		return "BTreeSet<" + g.toRustType(s.ElemType()) + ">"
	case t.IsList():
		l := t.(*sema.List)
		return "Vec<" + g.toRustType(l.ElemType()) + ">"
	}
	throwf("cannot find rust type for %s", t.Name())
	return ""
}

// toRustConstType is t_rs_generator::to_rust_const_type.
func (g *generator) toRustConstType(t sema.Type) string {
	if t.IsBaseType() {
		b := t.(*sema.BaseType)
		if b.Base() == sema.TypeString {
			if b.IsBinary() {
				return "&[u8]"
			}
			return "&str"
		}
	}
	return g.toRustType(t)
}

// toRustFieldTypeEnum is t_rs_generator::to_rust_field_type_enum.
func toRustFieldTypeEnum(t sema.Type) string {
	t = sema.TrueType(t)
	switch {
	case t.IsBaseType():
		switch t.(*sema.BaseType).Base() {
		case sema.TypeVoid:
			throwf("will not generate protocol::TType for TYPE_VOID")
		case sema.TypeString:
			return "TType::String"
		case sema.TypeUUID:
			return "TType::Uuid"
		case sema.TypeBool:
			return "TType::Bool"
		case sema.TypeI8:
			return "TType::I08"
		case sema.TypeI16:
			return "TType::I16"
		case sema.TypeI32:
			return "TType::I32"
		case sema.TypeI64:
			return "TType::I64"
		case sema.TypeDouble:
			return "TType::Double"
		default:
			throwf("compiler error: unhandled type")
		}
	case t.IsEnum():
		return "TType::I32"
	case t.IsStruct(), t.IsXception():
		return "TType::Struct"
	case t.IsMap():
		return "TType::Map"
	case t.IsSet():
		return "TType::Set"
	case t.IsList():
		return "TType::List"
	}
	throwf("cannot find TType for %s", t.Name())
	return ""
}

// optInReqOutValue is t_rs_generator::opt_in_req_out_value.
func optInReqOutValue(t sema.Type) string {
	t = sema.TrueType(t)
	switch {
	case t.IsBaseType():
		b := t.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeVoid:
			throwf("cannot generate OPT_IN_REQ_OUT value for void")
		case sema.TypeString:
			if b.IsBinary() {
				return "Some(Vec::new())"
			}
			return `Some("".to_owned())`
		case sema.TypeUUID:
			return "Some(uuid::Uuid::nil())"
		case sema.TypeBool:
			return "Some(false)"
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			return "Some(0)"
		case sema.TypeDouble:
			return "Some(OrderedFloat::from(0.0))"
		default:
			throwf("compiler error: unhandled type")
		}
	case t.IsEnum(), t.IsStruct(), t.IsXception():
		return "None"
	case t.IsList():
		return "Some(Vec::new())"
	case t.IsSet():
		return "Some(BTreeSet::new())"
	case t.IsMap():
		return "Some(BTreeMap::new())"
	}
	throwf("cannot generate opt-in-req-out value for type %s", t.Name())
	return ""
}

// canGenerateSimpleConst is t_rs_generator::can_generate_simple_const.
func canGenerateSimpleConst(t sema.Type) bool {
	actual := sema.TrueType(t)
	if actual.IsBaseType() {
		return actual.(*sema.BaseType).Base() != sema.TypeDouble
	}
	return false
}

// canGenerateConstHolder is t_rs_generator::can_generate_const_holder.
func canGenerateConstHolder(t sema.Type) bool {
	actual := sema.TrueType(t)
	return !canGenerateSimpleConst(actual) && !actual.IsService()
}

// ---- service call / trait / struct names ----

func serviceCallClientFunctionName(fn *sema.Function) string {
	return rustSnakeCase(fn.Name())
}

func serviceCallHandlerFunctionName(fn *sema.Function) string {
	return "handle_" + rustSnakeCase(fn.Name())
}

func (g *generator) serviceCallArgsStructName(fn *sema.Function) string {
	// Thrift automatically appends `Args` to the arglist name. No need to do it here.
	return rustCamelCase(g.serviceName) + rustCamelCase(fn.Arglist().Name())
}

func (g *generator) serviceCallResultStructName(fn *sema.Function) string {
	return rustCamelCase(g.serviceName) + rustCamelCase(fn.Name()) + resultStructSuffix
}

func rustSyncClientMarkerTraitName(s *sema.Service) string {
	return "T" + rustCamelCase(s.Name()) + "SyncClientMarker"
}

func rustSyncClientTraitName(s *sema.Service) string {
	return "T" + rustCamelCase(s.Name()) + "SyncClient"
}

func rustSyncClientImplName(s *sema.Service) string {
	return rustCamelCase(s.Name()) + "SyncClient"
}

func rustSyncHandlerTraitName(s *sema.Service) string {
	return rustCamelCase(s.Name()) + "SyncHandler"
}

func rustSyncProcessorName(s *sema.Service) string {
	return rustCamelCase(s.Name()) + "SyncProcessor"
}

func rustSyncProcessorImplName(s *sema.Service) string {
	return "T" + rustCamelCase(s.Name()) + "ProcessFunctions"
}

// ---- utility rendering ----

// docer is the doc-comment interface every commented IDL element
// satisfies (t_doc).
type docer interface {
	HasDoc() bool
	Doc() string
}

// renderTypeComment is t_rs_generator::render_type_comment.
func (g *generator) renderTypeComment(name string) {
	g.wl("//")
	g.wl("// " + name)
	g.wl("//")
	g.wl("")
}

// renderRustdoc is t_rs_generator::render_rustdoc. Do *not* put in an
// extra newline after doc is generated: Rust docs have to abut the line
// they're documenting.
func (g *generator) renderRustdoc(d docer) {
	if !d.HasDoc() {
		return
	}
	emit.DocstringComment(&g.buf, g.ind(), "", "/// ", d.Doc(), "")
}

// renderThriftError is t_rs_generator::render_thrift_error.
func (g *generator) renderThriftError(errorKind, errorStruct, subErrorKind, errorMessage string) {
	g.line("Err(")
	g.up()
	g.line("thrift::Error::" + errorKind + "(")
	g.up()
	g.renderThriftErrorStruct(errorStruct, subErrorKind, errorMessage)
	g.down()
	g.line(")")
	g.down()
	g.line(")")
}

// renderThriftErrorStruct is t_rs_generator::render_thrift_error_struct.
func (g *generator) renderThriftErrorStruct(errorStruct, subErrorKind, errorMessage string) {
	g.line(errorStruct + "::new(")
	g.up()
	g.line(subErrorKind + ",")
	g.line(errorMessage)
	g.down()
	g.line(")")
}

// ---- init / includes / close ----

// outDir is get_out_dir(). t_rs_generator never sets out_dir_base_, so
// the base class formula (out_path + out_dir_base_ + "/") collapses to
// out_path + "/" whether or not the path is absolute.
func (g *generator) outDir() string {
	return g.program.OutPath() + "/"
}

func (g *generator) initGenerator() {
	genDir := g.outDir()
	emit.Mkdir(genDir)

	g.buf.WriteString("// Autogenerated by Thrift Compiler (" + version.Version + ")\n")
	g.wl("// DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING")
	g.wl("")

	g.renderAttributesAndIncludes()
}

func (g *generator) renderAttributesAndIncludes() {
	// turn off some compiler/clippy warnings

	// code may not be used
	g.wl("#![allow(dead_code)]")
	// code always includes BTreeMap/BTreeSet/OrderedFloat
	g.wl("#![allow(unused_imports)]")
	// code might not include imports from crates
	g.wl("#![allow(unused_extern_crates)]")
	// constructors take *all* struct parameters, which can trigger the "too many arguments" warning
	// some auto-gen'd types can be deeply nested. clippy recommends factoring them out which is hard
	// to autogen some methods may start with "is_"
	// FIXME: re-enable the 'vec_box' lint see:
	// [THRIFT-5364](https://issues.apache.org/jira/browse/THRIFT-5364) This can happen because we
	// automatically generate a Vec<Box<Type>> when the type is a typedef and it's a forward typedef.
	// This (typedef + forward typedef) can happen in two situations:
	// 1. When the type is recursive
	// 2. When you define types out of order
	g.wl("#![allow(clippy::too_many_arguments, clippy::type_complexity, clippy::vec_box, clippy::wrong_self_convention)]")
	// prevent rustfmt from running against this file
	// lines are too long, code is (thankfully!) not visual-indented, etc.
	// can't use #[rustfmt::skip] see: https://github.com/rust-lang/rust/issues/54726
	g.wl("#![cfg_attr(rustfmt, rustfmt_skip)]")
	g.wl("")

	// add standard includes
	g.wl("use std::cell::RefCell;")
	g.wl("use std::collections::{BTreeMap, BTreeSet};")
	g.wl("use std::convert::{From, TryFrom};")
	g.wl("use std::default::Default;")
	g.wl("use std::error::Error;")
	g.wl("use std::fmt;")
	g.wl("use std::fmt::{Display, Formatter};")
	g.wl("use std::rc::Rc;")
	g.wl("")
	g.wl("use thrift::OrderedFloat;")
	g.wl("use thrift::{ApplicationError, ApplicationErrorKind, ProtocolError, ProtocolErrorKind, TThriftClient};")
	g.wl("use thrift::protocol::{TFieldIdentifier, TListIdentifier, TMapIdentifier, TMessageIdentifier, TMessageType, TInputProtocol, TOutputProtocol, TSerializable, TSetIdentifier, TStructIdentifier, TType};")
	g.wl("use thrift::protocol::field_id;")
	g.wl("use thrift::protocol::verify_expected_message_type;")
	g.wl("use thrift::protocol::verify_expected_sequence_number;")
	g.wl("use thrift::protocol::verify_expected_service_call;")
	g.wl("use thrift::protocol::verify_required_field_exists;")
	g.wl("use thrift::server::TProcessor;")
	g.wl("")

	// add all the program includes
	// NOTE: this is more involved than you would expect because of service extension
	// Basically, I have to find the closure of all the services and include their modules at the
	// top-level

	referenced := g.referencedModules()

	// finally, write all the "pub use..." declarations
	if len(referenced) != 0 {
		for _, m := range referenced {
			moduleName := m.name
			moduleNamespace := stringReplace(m.ns, ".", "::")
			if moduleNamespace == "" {
				g.wl("use " + g.cratePrefix + "::" + rustSnakeCase(moduleName) + ";")
			} else {
				g.wl("use " + g.cratePrefix + "::" + moduleNamespace + "::" + rustSnakeCase(moduleName) + ";")
			}
		}
		g.wl("")
	}
}

// modulePair is the set<pair<string,string>> entry of
// compute_service_referenced_modules: a module name and its namespace.
type modulePair struct{ name, ns string }

// referencedModules computes the closure of referenced modules exactly
// like render_attributes_and_includes / compute_service_referenced_modules,
// starting from the explicit includes and then walking every service's
// extends chain. The C++ code uses a std::set<pair<string,string>>, which
// dedupes and sorts by (first, then second).
func (g *generator) referencedModules() []modulePair {
	seen := map[modulePair]bool{}
	var out []modulePair
	add := func(p modulePair) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, inc := range g.program.Includes() {
		add(modulePair{inc.Name(), inc.Namespace("rs")})
	}
	for _, s := range g.program.Services() {
		g.computeServiceReferencedModules(s, add)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].name != out[j].name {
			return out[i].name < out[j].name
		}
		return out[i].ns < out[j].ns
	})
	return out
}

// computeServiceReferencedModules is
// t_rs_generator::compute_service_referenced_modules. It compares
// programs by identity, as the C++ pointer comparison does, unlike
// rustNamespace* which compare by name.
func (g *generator) computeServiceReferencedModules(s *sema.Service, add func(modulePair)) {
	extends := s.Extends()
	if extends != nil {
		if extends.Program() != g.program {
			add(modulePair{extends.Program().Name(), extends.Program().Namespace("rs")})
		}
		g.computeServiceReferencedModules(extends, add)
	}
}

func (g *generator) closeGenerator() {
	genDir := g.outDir()
	fileName := genDir + rustSnakeCase(g.program.Name()) + ".rs"
	emit.WriteFile(fileName, g.buf.String())
}
