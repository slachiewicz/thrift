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

// Package delphi generates Delphi/Pascal code from a resolved Thrift
// program.
//
// It is a method-for-method port of
// compiler/cpp/src/thrift/generate/t_delphi_generator.cc. The port writes
// exactly the bytes the C++ generator writes, which is checked against the
// C++ compiler by the parity tests; that is why the emitter builds strings
// the way an ostream would rather than through templates, and why it
// tracks two independent indentation counters the way the C++ generator's
// indent_/indent_impl_ pair does.
package delphi

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

// Options are the "--gen delphi:" generator options.
type Options struct {
	RegisterTypes bool
	ConstPrefix   bool
	OldNames      bool
	Events        bool
	XMLDoc        bool
	Async         bool
	ComTypes      bool
	RTTI          bool
	GuidV4        bool
}

// ParseOptions parses the part after "delphi:" of a --gen argument, with
// the same splitting rules as t_generator::parse_options: options are
// comma separated and a value follows the first "=", though every Delphi
// option is a flag and the value part (if any) is ignored, as the C++
// constructor's option loop only inspects iter->first.
func ParseOptions(spec string) (Options, error) {
	var o Options
	for _, option := range strings.Split(spec, ",") {
		key := option
		if i := strings.IndexByte(option, '='); i >= 0 {
			key = option[:i]
		}
		switch key {
		case "":
		case "register_types":
			o.RegisterTypes = true
		case "old_names":
			o.OldNames = true
		case "constprefix":
			o.ConstPrefix = true
		case "events":
			o.Events = true
		case "xmldoc":
			o.XMLDoc = true
		case "async":
			o.Async = true
		case "com_types":
			o.ComTypes = true
		case "rtti":
			o.RTTI = true
		case "guid_v4":
			o.GuidV4 = true
		default:
			return o, &emit.Error{Msg: "unknown option delphi:" + key}
		}
	}
	return o, nil
}

// Generator is t_delphi_generator for one program.
type Generator struct {
	program *sema.Program
	opts    Options

	namespaceName string
	programName   string
	serviceName   string

	// Section buffers, assembled into one file in closeGenerator, as the
	// C++ generator's s_forward_decr/s_enum/.../s_type_factory_funcs
	// ostringstream members are.
	sForwardDecr             strings.Builder
	sEnum                    strings.Builder
	sConst                   strings.Builder
	sStruct                  strings.Builder
	sService                 strings.Builder
	sConstImpl               strings.Builder
	sStructImpl              strings.Builder
	sServiceImpl             strings.Builder
	sTypeFactoryRegistration strings.Builder
	sTypeFactoryFuncs        strings.Builder

	hasForward bool
	hasEnum    bool
	hasConst   bool

	// level is indent_: the indentation counter for interface-section
	// output (declarations). implLevel is indent_impl_: the separate
	// counter for implementation-section output.
	level     int
	implLevel int

	tmpCounter int

	typesKnown      map[string]bool
	typedefsPending []*sema.Typedef

	usesList []string
}

// New creates a generator for the program.
func New(program *sema.Program, opts Options) *Generator {
	return &Generator{
		program:     program,
		opts:        opts,
		programName: program.Name(),
		typesKnown:  map[string]bool{},
	}
}

// Generate writes the Delphi unit for the program. It is
// t_generator::generate_program restricted to what t_delphi_generator
// overrides.
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
		g.generateForwardDeclaration(s)
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

// ---- t_generator helpers ----

// tmp is t_generator::tmp: a name with a running number appended,
// starting at 0.
func (g *Generator) tmp(name string) string {
	s := name + strconv.Itoa(g.tmpCounter)
	g.tmpCounter++
	return s
}

// indent is t_delphi_generator's inherited indent(): two spaces per
// level, tracked by g.level (indent_).
func (g *Generator) indent() string {
	return strings.Repeat("  ", g.level)
}

func (g *Generator) indentUp()   { g.level++ }
func (g *Generator) indentDown() { g.level-- }

// indentImpl is indent_impl(): the separate counter used while writing
// the implementation-section code (bodies), tracked by g.implLevel.
func (g *Generator) indentImpl() string {
	return strings.Repeat("  ", g.implLevel)
}

func (g *Generator) indentUpImpl()   { g.implLevel++ }
func (g *Generator) indentDownImpl() { g.implLevel-- }

// ---- output-builder helpers ----
//
// These mirror `indent(out) << s` / `indent(out) << s << '\n'` / `out << s`
// call sites in the C++ source, parameterized over the target builder
// since the C++ code passes different ostream& targets (s_struct_impl,
// s_service_impl, ...) into the same shared functions.

// wr writes indent()+s with no trailing newline: `indent(out) << s`.
func (g *Generator) wr(out *strings.Builder, s string) {
	out.WriteString(g.indent())
	out.WriteString(s)
}

// ln writes indent()+s+"\n": `indent(out) << s << '\n'`.
func (g *Generator) ln(out *strings.Builder, s string) {
	out.WriteString(g.indent())
	out.WriteString(s)
	out.WriteString("\n")
}

// raw writes s verbatim: `out << s`.
func (g *Generator) raw(out *strings.Builder, s string) {
	out.WriteString(s)
}

// wrI writes indentImpl()+s with no trailing newline: `indent_impl(out) << s`.
func (g *Generator) wrI(out *strings.Builder, s string) {
	out.WriteString(g.indentImpl())
	out.WriteString(s)
}

// lnI writes indentImpl()+s+"\n": `indent_impl(out) << s << '\n'`.
func (g *Generator) lnI(out *strings.Builder, s string) {
	out.WriteString(g.indentImpl())
	out.WriteString(s)
	out.WriteString("\n")
}

func lowercaseASCII(in string) string {
	b := []byte(in)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c - 'A' + 'a'
		}
	}
	return string(b)
}

// formatDouble mimics `ostream << double`, the default (general) float
// format at the default precision of six significant digits.
func formatDouble(f float64) string {
	return strconv.FormatFloat(f, 'g', 6, 64)
}

func itoa64(i int64) string { return strconv.FormatInt(i, 10) }
func itoa32(i int32) string { return strconv.FormatInt(int64(i), 10) }

// ---- files ----

func (g *Generator) outDir() string {
	if g.program.IsOutPathAbsolute() {
		return g.program.OutPath() + "/"
	}
	return g.program.OutPath() + "gen-delphi" + "/"
}

// autogenComment is t_delphi_generator::autogen_comment.
func (g *Generator) autogenComment() string {
	return "(**\n" +
		" * Autogenerated by Thrift Compiler (" + version.Version + ")\n" +
		" *\n" +
		" * DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING\n" +
		" *)\n"
}

// initGenerator is t_delphi_generator::init_generator.
func (g *Generator) initGenerator() {
	g.level = 0
	g.implLevel = 0
	g.namespaceName = g.program.Namespace("delphi")
	g.hasForward = false
	g.hasEnum = false
	g.hasConst = false

	g.addDelphiUsesList("Classes")
	g.addDelphiUsesList("SysUtils")
	g.addDelphiUsesList("Generics.Collections")
	if g.opts.Async {
		g.addDelphiUsesList("System.Threading")
	}

	g.addDelphiUsesList("Thrift")
	g.addDelphiUsesList("Thrift.Utils")
	g.addDelphiUsesList("Thrift.Collections")
	g.addDelphiUsesList("Thrift.Protocol")
	g.addDelphiUsesList("Thrift.Transport")
	if g.opts.RegisterTypes {
		g.addDelphiUsesList("Thrift.TypeRegistry")
	}

	g.initKnownTypesList()

	for _, include := range g.program.Includes() {
		unitname := include.Name()
		nsname := include.Namespace("delphi")
		if nsname != "" {
			unitname = nsname
		}
		unitname = g.normalizeName(unitname, false, false, true)
		g.addDelphiUsesList(unitname)
	}

	emit.Mkdir(g.outDir())
}

// closeGenerator is t_delphi_generator::close_generator.
func (g *Generator) closeGenerator() {
	unitname := g.programName
	if g.namespaceName != "" {
		unitname = g.namespaceName
	}

	unitname = strings.ReplaceAll(unitname, " ", "_")
	unitname = g.normalizeName(unitname, false, false, true)

	fName := g.outDir() + "/" + unitname + ".pas"
	var f strings.Builder

	f.WriteString(g.autogenComment() + "\n")
	g.generateDelphiDoc(&f, g.program)
	f.WriteString("unit " + unitname + ";" + "\n\n")
	f.WriteString("{$WARN SYMBOL_DEPRECATED OFF}\n")
	if g.opts.ComTypes {
		f.WriteString("{$MINENUMSIZE 4}\n")
	}
	if g.opts.RTTI {
		f.WriteString("{$IFOPT M+} {$DEFINE TYPEINFO_WAS_ON} {$ELSE} {$UNDEF TYPEINFO_WAS_ON} {$ENDIF}\n")
	}
	f.WriteString("\n")
	f.WriteString("interface\n\n")
	f.WriteString("uses\n")

	g.indentUp()

	for i, u := range g.usesList {
		if i != 0 {
			f.WriteString(",")
			f.WriteString("\n")
		}
		g.wr(&f, u)
	}

	f.WriteString(";\n\n")

	g.indentDown()

	tmpUnit := strings.ReplaceAll(unitname, ".", "_")

	f.WriteString("const\n")
	g.indentUp()
	g.ln(&f, "c"+tmpUnit+"_Option_Register_Types = "+boolStr(g.opts.RegisterTypes)+";")
	g.ln(&f, "c"+tmpUnit+"_Option_ConstPrefix    = "+boolStr(g.opts.ConstPrefix)+";")
	g.ln(&f, "c"+tmpUnit+"_Option_Events         = "+boolStr(g.opts.Events)+";")
	g.ln(&f, "c"+tmpUnit+"_Option_XmlDoc         = "+boolStr(g.opts.XMLDoc)+";")
	g.ln(&f, "c"+tmpUnit+"_Option_Async          = "+boolStr(g.opts.Async)+";")
	g.ln(&f, "c"+tmpUnit+"_Option_COM_types      = "+boolStr(g.opts.ComTypes)+";")
	g.ln(&f, "c"+tmpUnit+"_Option_Old_Names      = "+boolStr(g.opts.OldNames)+";")
	g.ln(&f, "c"+tmpUnit+"_Option_RTTI           = "+boolStr(g.opts.RTTI)+";")
	g.indentDown()

	f.WriteString("\n")
	f.WriteString("type\n")
	if g.hasForward {
		f.WriteString(g.sForwardDecr.String() + "\n")
	}
	if g.hasEnum {
		g.ln(&f, "")
		g.ln(&f, "{$SCOPEDENUMS ON}\n")
		f.WriteString(g.sEnum.String())
		g.ln(&f, "{$SCOPEDENUMS OFF}\n")
	}
	f.WriteString(g.sStruct.String())
	f.WriteString(g.sService.String())
	f.WriteString(g.sConst.String())
	f.WriteString("implementation\n\n")
	f.WriteString(g.sStructImpl.String())
	f.WriteString(g.sServiceImpl.String())
	f.WriteString(g.sConstImpl.String())

	if g.opts.RegisterTypes {
		f.WriteString("\n")
		f.WriteString("// Type factory methods and registration\n")
		f.WriteString(g.sTypeFactoryFuncs.String())
		f.WriteString("procedure RegisterTypeFactories;\n")
		f.WriteString("begin\n")
		f.WriteString(g.sTypeFactoryRegistration.String())
		f.WriteString("end;\n")
	}
	f.WriteString("\n")

	constantsClass := g.makeConstantsClassname()

	f.WriteString("initialization\n")
	if g.hasConst {
		f.WriteString("{$IF CompilerVersion < 21.0}  // D2010\n")
		f.WriteString("  " + constantsClass + "_Initialize;\n")
		f.WriteString("{$IFEND}\n")
	}
	if g.opts.RegisterTypes {
		f.WriteString("  RegisterTypeFactories;\n")
	}
	f.WriteString("\n")

	f.WriteString("finalization\n")
	if g.hasConst {
		f.WriteString("{$IF CompilerVersion < 21.0}  // D2010\n")
		f.WriteString("  " + constantsClass + "_Finalize;\n")
		f.WriteString("{$IFEND}\n")
	}
	f.WriteString("\n\n")

	f.WriteString("end.\n")

	emit.WriteFile(fName, f.String())
}

func boolStr(b bool) string {
	if b {
		return "True"
	}
	return "False"
}
