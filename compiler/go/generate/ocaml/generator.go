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

// Package ocaml is a method-for-method port of
// compiler/cpp/src/thrift/generate/t_ocaml_generator.cc. It writes the same
// bytes as the C++ generator, which is checked against the C++ compiler by
// the parity tests. It produces "<program>_types.ml/.mli",
// "<program>_consts.ml" and, per service, "<Service>.ml/.mli", using an
// indent-tracked writer the way t_ocaml_generator uses its ofstream and
// indent()/indent_up()/indent_down(): a single indentLevel field, shared
// across every output file exactly like the C++ generator's one indent_
// member, and a single tmpCounter field, shared like tmp_.
//
// A few call sites in the C++ source build a value (a t_field local, a
// tmp() name, or the "_x" from generate_ocaml_struct_sig) that is never
// read. Because tmp() and its Go equivalent both mutate a shared counter
// that later variable names depend on, those dead calls are reproduced
// here too, or later temporary names would drift out of parity; each is
// flagged with a comment at its call site.
package ocaml

import (
	"math"
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

// Generator is t_ocaml_generator for one program.
type Generator struct {
	program *sema.Program
	opts    Options

	programName string
	serviceName string

	// indentLevel is indent_: the current code indentation level, shared
	// mutable state across the whole generation of the program and every
	// output file, exactly like the C++ member.
	indentLevel int
	// tmpCounter is tmp_: the running number tmp() appends to a name.
	tmpCounter int

	fTypes  strings.Builder
	fTypesI strings.Builder
	fConsts strings.Builder
}

// New creates a generator for the program.
func New(program *sema.Program, opts Options) *Generator {
	return &Generator{program: program, opts: opts, programName: program.Name()}
}

// Generate writes the OCaml code for the program. It is
// t_ocaml_generator::generate_program.
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

	// generate_program's order: unlike t_generator::generate_program, the
	// OCaml generator writes structs and xceptions before typedefs, and
	// services before consts.
	for _, e := range g.program.Enums() {
		g.generateEnum(e)
	}
	for _, s := range g.program.Structs() {
		g.generateStruct(s)
	}
	for _, x := range g.program.Xceptions() {
		g.generateXception(x)
	}
	for _, t := range g.program.Typedefs() {
		g.generateTypedef(t)
	}
	// f_service_/f_service_i_ are reopened per service without ever being
	// flushed in between (see the comment on generateService), so only
	// the last service in the program actually reaches disk.
	var lastServiceName, lastServiceIName, lastService, lastServiceI string
	haveService := false
	for _, s := range g.program.Services() {
		g.serviceName = s.Name()
		lastServiceName, lastServiceIName, lastService, lastServiceI = g.generateService(s)
		haveService = true
	}
	// generate_consts (the t_generator default): a plain loop over
	// generate_const.
	for _, c := range g.program.Consts() {
		g.generateConst(c)
	}

	// ~t_ocaml_generator() closes f_types_/f_types_i_/f_consts_/
	// f_service_/f_service_i_, which is what flushes an
	// ofstream_with_content_based_conditional_update to disk.
	outDir := g.outDir()
	emit.WriteFile(outDir+g.programName+"_types.ml", g.fTypes.String())
	emit.WriteFile(outDir+g.programName+"_types.mli", g.fTypesI.String())
	emit.WriteFile(outDir+g.programName+"_consts.ml", g.fConsts.String())
	if haveService {
		emit.WriteFile(lastServiceName, lastService)
		emit.WriteFile(lastServiceIName, lastServiceI)
	}
	return nil
}

// ---- t_generator helpers ----

func (g *Generator) indentUp()   { g.indentLevel++ }
func (g *Generator) indentDown() { g.indentLevel-- }

// indent is t_generator::indent() with the default indent_str() of "  ".
func (g *Generator) indent() string {
	return strings.Repeat("  ", g.indentLevel)
}

// outDir is t_generator::get_out_dir with out_dir_base_ == "gen-ocaml".
func (g *Generator) outDir() string {
	if g.program.IsOutPathAbsolute() {
		return g.program.OutPath() + "/"
	}
	return g.program.OutPath() + "gen-ocaml" + "/"
}

// tmp is t_generator::tmp: a name with a running number appended.
func (g *Generator) tmp(name string) string {
	s := name + strconv.Itoa(g.tmpCounter)
	g.tmpCounter++
	return s
}

// ---- naming helpers (t_generator::capitalize/decapitalize) ----

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

// ---- init ----

// initGenerator is t_ocaml_generator::init_generator. Opening the actual
// files is left to Generate, which writes the accumulated builders out
// once generation is complete, the way ~t_ocaml_generator() flushes them.
func (g *Generator) initGenerator() {
	emit.Mkdir(g.outDir())

	g.fTypes.WriteString(g.ocamlAutogenComment() + "\n" + g.ocamlImports() + "\n")
	g.fTypesI.WriteString(g.ocamlAutogenComment() + "\n" + g.ocamlImports() + "\n")
	g.fConsts.WriteString(g.ocamlAutogenComment() + "\n" + g.ocamlImports() + "\n" + "open " +
		capitalize(g.programName) + "_types" + "\n")
}

// ocamlAutogenComment is t_ocaml_generator::ocaml_autogen_comment.
func (g *Generator) ocamlAutogenComment() string {
	return "(*\n" +
		" Autogenerated by Thrift Compiler (" + version.Version + ")\n" +
		"\n" +
		" DO NOT EDIT UNLESS YOU ARE SURE YOU KNOW WHAT YOU ARE DOING\n" +
		"*)\n"
}

// ocamlImports is t_ocaml_generator::ocaml_imports: it always prints
// standard thrift imports.
func (g *Generator) ocamlImports() string {
	return "open Thrift"
}

// ---- helper rendering functions ----

// typeName is t_ocaml_generator::type_name.
func (g *Generator) typeName(typ sema.Type) string {
	prefix := ""
	program := typ.Program()
	if program != nil && program != g.program {
		if !typ.IsService() {
			prefix = capitalize(program.Name()) + "_types."
		}
	}

	name := typ.Name()
	if typ.IsService() {
		name = capitalize(name)
	} else {
		name = decapitalize(name)
	}
	return prefix + name
}

// exceptionCtor is t_ocaml_generator::exception_ctor.
func (g *Generator) exceptionCtor(typ sema.Type) string {
	prefix := ""
	program := typ.Program()
	if program != nil && program != g.program {
		if !typ.IsService() {
			prefix = capitalize(program.Name()) + "_types."
		}
	}
	return prefix + capitalize(typ.Name())
}

// functionSignature is t_ocaml_generator::function_signature: renders a
// function signature of the form 'name args'. It takes the function's name
// and arglist directly rather than a *sema.Function, since the client's
// synthetic recv_ function (generateServiceClient) has no t_function of its
// own in the Go model, exactly as the C++ source builds one locally there.
func (g *Generator) functionSignature(name string, arglist *sema.Struct, prefix string) string {
	return prefix + decapitalize(name) + " " + g.argumentList(arglist)
}

// functionType is t_ocaml_generator::function_type. It takes the arglist
// and return type directly, for the same reason as functionSignature.
func (g *Generator) functionType(arglist *sema.Struct, returnType sema.Type, method, options bool) string {
	result := ""
	fields := arglist.Members()
	for _, f := range fields {
		result += g.renderOcamlType(f.Type())
		if options {
			result += " option"
		}
		result += " -> "
	}
	if len(fields) == 0 && !method {
		result += "unit -> "
	}
	result += g.renderOcamlType(returnType)
	return result
}

// argumentList is t_ocaml_generator::argument_list: renders a field list.
func (g *Generator) argumentList(s *sema.Struct) string {
	result := ""
	first := true
	for _, f := range s.Members() {
		if first {
			first = false
		} else {
			result += " "
		}
		result += f.Name()
	}
	return result
}

// typeToEnum is t_ocaml_generator::type_to_enum: converts the parse type to
// a Protocol.t_type enum.
func (g *Generator) typeToEnum(typ sema.Type) string {
	typ = sema.TrueType(typ)

	if typ.IsBaseType() {
		b := typ.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeVoid:
			return "Protocol.T_VOID"
		case sema.TypeString:
			return "Protocol.T_STRING"
		case sema.TypeBool:
			return "Protocol.T_BOOL"
		case sema.TypeI8:
			return "Protocol.T_BYTE"
		case sema.TypeI16:
			return "Protocol.T_I16"
		case sema.TypeI32:
			return "Protocol.T_I32"
		case sema.TypeI64:
			return "Protocol.T_I64"
		case sema.TypeDouble:
			return "Protocol.T_DOUBLE"
		default:
			emit.Throw("compiler error: unhandled type")
		}
	} else if typ.IsEnum() {
		return "Protocol.T_I32"
	} else if typ.IsStruct() || typ.IsXception() {
		return "Protocol.T_STRUCT"
	} else if typ.IsMap() {
		return "Protocol.T_MAP"
	} else if typ.IsSet() {
		return "Protocol.T_SET"
	} else if typ.IsList() {
		return "Protocol.T_LIST"
	}

	emit.Throw("INVALID TYPE IN type_to_enum: %s", typ.Name())
	return ""
}

// renderOcamlType is t_ocaml_generator::render_ocaml_type: converts the
// parse type to an OCaml type.
func (g *Generator) renderOcamlType(typ sema.Type) string {
	typ = sema.TrueType(typ)

	if typ.IsBaseType() {
		b := typ.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeVoid:
			return "unit"
		case sema.TypeString:
			return "string"
		case sema.TypeBool:
			return "bool"
		case sema.TypeI8:
			return "int"
		case sema.TypeI16:
			return "int"
		case sema.TypeI32:
			return "Int32.t"
		case sema.TypeI64:
			return "Int64.t"
		case sema.TypeDouble:
			return "float"
		default:
			emit.Throw("compiler error: unhandled type")
		}
	} else if typ.IsEnum() {
		return capitalize(typ.Name()) + ".t"
	} else if typ.IsStruct() || typ.IsXception() {
		return g.typeName(typ)
	} else if typ.IsMap() {
		m := typ.(*sema.Map)
		return "(" + g.renderOcamlType(m.KeyType()) + "," + g.renderOcamlType(m.ValType()) + ") Hashtbl.t"
	} else if typ.IsSet() {
		s := typ.(*sema.Set)
		return "(" + g.renderOcamlType(s.ElemType()) + ",bool) Hashtbl.t"
	} else if typ.IsList() {
		l := typ.(*sema.List)
		return g.renderOcamlType(l.ElemType()) + " list"
	}

	emit.Throw("INVALID TYPE IN type_to_enum: %s", typ.Name())
	return ""
}

// displayName is t_ocaml_generator::display_name. It is not called from
// anywhere in the generator's own output, exactly as in the C++ source, but
// is kept for parity of the class's public surface.
func (g *Generator) displayName() string {
	return "OCaml"
}

// formatDoubleShowpoint mimics `ostream << double` with ios::showpoint set
// and the default precision of 6 significant digits, which is what
// render_const_value's TYPE_DOUBLE case uses ("OCaml requires all floating
// point numbers contain a decimal point"). Unlike the plain %g formatDouble
// helper the other language ports use, showpoint keeps trailing zeros and
// always prints a decimal point: it follows the same fixed-vs-scientific
// choice as %g (fixed unless the base-10 exponent is < -4 or >= the
// precision), but never strips the zeros padding out to the precision.
func formatDoubleShowpoint(f float64) string {
	const prec = 6
	neg := math.Signbit(f)
	af := math.Abs(f)

	// Round to prec significant digits in scientific form first, to read
	// off the exponent the fixed-vs-scientific choice needs; strconv's
	// correctly-rounded formatter guarantees this agrees with the fixed
	// formatting below, which asks for exactly the digits that make up
	// the same prec significant digits.
	sci := strconv.FormatFloat(af, 'e', prec-1, 64)
	eIdx := strings.IndexByte(sci, 'e')
	mantissa, expPart := sci[:eIdx], sci[eIdx+1:]
	exp, _ := strconv.Atoi(expPart)

	var out string
	if exp < -4 || exp >= prec {
		out = mantissa + "e" + fmt3Sign(exp)
	} else {
		decDigits := prec - 1 - exp
		if decDigits < 0 {
			decDigits = 0
		}
		out = strconv.FormatFloat(af, 'f', decDigits, 64)
		if !strings.Contains(out, ".") {
			out += "."
		}
	}
	if neg {
		out = "-" + out
	}
	return out
}

// fmt3Sign renders a signed exponent with a minimum of two digits, as
// ostream's scientific notation does (e.g. "+06", "-05", "+123").
func fmt3Sign(exp int) string {
	sign := "+"
	if exp < 0 {
		sign = "-"
		exp = -exp
	}
	digits := strconv.Itoa(exp)
	if len(digits) < 2 {
		digits = "0" + digits
	}
	return sign + digits
}
