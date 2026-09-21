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

// Package perl is a method-for-method port of
// compiler/cpp/src/thrift/generate/t_perl_generator.cc. The port writes
// exactly the bytes the C++ generator writes, which is checked against the
// C++ compiler by the parity tests; that is why the emitter builds strings
// the way an ostream would rather than through templates. t_perl_generator
// keeps a single indent_ counter (inherited from t_generator) shared across
// every output stream it writes to in turn, rather than a per-stream
// counter as some other ports use, so Generator.indentLevel below is one
// field shared the same way.
package perl

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

// Options are the perl:... generator options. The C++ constructor accepts
// none: any parsed option is a hard error ("no options yet").
type Options struct{}

// ParseOptions parses the part after "perl:" of a --gen argument.
func ParseOptions(spec string) (Options, error) {
	var o Options
	for _, option := range strings.Split(spec, ",") {
		key := option
		if i := strings.IndexByte(option, '='); i >= 0 {
			key = option[:i]
		}
		switch key {
		case "":
		default:
			return o, &emit.Error{Msg: "unknown option perl:" + key}
		}
	}
	return o, nil
}

func init() {
	generate.Register(generate.Info{
		Name:     "perl",
		LongName: "Perl",
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
	_ = opts
	program.Scope.ResolveAllConsts()
	sema.ValidateInput(program)
	g := newGenerator(program)
	g.generateProgram()
	return nil
}

// Generator is t_perl_generator for one program.
type Generator struct {
	program     *sema.Program
	serviceName string // service_name_
	indentLevel int    // indent_
	tmpCounter  int    // tmp_

	fTypes  strings.Builder // f_types_
	fConsts strings.Builder // f_consts_
	// fService is f_service_, reopened for every service.
	fService strings.Builder

	fTypesName  string
	fConstsName string

	// fTypesUseIncludesEmitted is f_types_use_includes_emitted_: debounces
	// generate_use_includes for f_types_.
	fTypesUseIncludesEmitted bool
}

func newGenerator(program *sema.Program) *Generator {
	return &Generator{program: program}
}

// generateProgram is t_generator::generate_program: init, enums,
// typedefs (no-op), structs and exceptions in declared order (perl has no
// forward-declaration override), constants, services, close.
func (g *Generator) generateProgram() {
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
}

// ---- indenting, shared across every stream this generator writes to ----

func (g *Generator) indentUp()   { g.indentLevel++ }
func (g *Generator) indentDown() { g.indentLevel-- }

// indent is t_generator::indent() with the default indent_str() of two
// spaces (t_perl_generator does not override it).
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

// tmp is t_generator::tmp.
func (g *Generator) tmp(name string) string {
	s := name + strconv.Itoa(g.tmpCounter)
	g.tmpCounter++
	return s
}

// outDir is t_generator::get_out_dir with out_dir_base_ "gen-perl".
func (g *Generator) outDir() string {
	if g.program.IsOutPathAbsolute() {
		return g.program.OutPath() + "/"
	}
	return g.program.OutPath() + "gen-perl/"
}

// ---- init and close ----

// initGenerator is t_perl_generator::init_generator: prepares the output
// directory (including any namespace subdirectories) and opens the types
// and constants files.
func (g *Generator) initGenerator() {
	emit.Mkdir(g.outDir())

	outdir := g.outDir()
	dirs := perlNamespaceDirs(g.program.Namespace("perl"))
	for _, d := range dirs {
		outdir += d + "/"
		emit.Mkdir(outdir)
	}

	g.fTypesName = outdir + "Types.pm"
	g.fConstsName = outdir + "Constants.pm"

	g.fTypes.WriteString(autogenComment() + perlIncludes())

	g.fConsts.WriteString(autogenComment() + "package " + perlNamespace(g.program) +
		"Constants;  ## no critic (RequireFilenameMatchesPackage)\n" + perlIncludes() + "\n")
}

// closeGenerator is t_perl_generator::close_generator.
func (g *Generator) closeGenerator() {
	g.fTypes.WriteString("1;\n")
	g.fConsts.WriteString("1;\n")
	emit.WriteFile(g.fTypesName, g.fTypes.String())
	emit.WriteFile(g.fConstsName, g.fConsts.String())
}

// perlIncludes is t_perl_generator::perl_includes: prints standard perl
// imports.
func perlIncludes() string {
	return "use 5.10.0;\n" +
		"use strict;\n" +
		"use warnings;\n" +
		"use Thrift::Exception;\n" +
		"use Thrift::MessageType;\n" +
		"use Thrift::Type;\n\n"
}

// autogenComment is t_perl_generator::autogen_comment.
func autogenComment() string {
	return "#\n" +
		"# Autogenerated by Thrift Compiler (" + version.Version + ")\n" +
		"#\n" +
		"# DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING\n" +
		"#\n"
}

// perlNamespaceDirs is t_perl_generator::perl_namespace_dirs: the "perl"
// namespace split on '.', with the same edge-case handling as the C++
// while-loop (a trailing dot yields no trailing empty segment).
func perlNamespaceDirs(ns string) []string {
	var dirs []string
	for {
		loc := strings.IndexByte(ns, '.')
		if loc < 0 {
			break
		}
		dirs = append(dirs, ns[:loc])
		ns = ns[loc+1:]
	}
	if len(ns) > 0 {
		dirs = append(dirs, ns)
	}
	return dirs
}

// perlNamespace is t_perl_generator::perl_namespace: my.namespace becomes
// my::namespace::.
func perlNamespace(p *sema.Program) string {
	ns := p.Namespace("perl")
	result := ""
	for {
		loc := strings.IndexByte(ns, '.')
		if loc < 0 {
			break
		}
		result += ns[:loc] + "::"
		ns = ns[loc+1:]
	}
	if len(ns) > 0 {
		result += ns + "::"
	}
	return result
}

// namespaceOutDir is t_perl_generator::get_namespace_out_dir.
func (g *Generator) namespaceOutDir() string {
	outdir := g.outDir()
	for _, d := range perlNamespaceDirs(g.program.Namespace("perl")) {
		outdir += d + "/"
	}
	return outdir
}

// ---- typedefs, enums, consts ----

// generateTypedef does nothing. This is not done in PERL, types are all
// implicit.
func (g *Generator) generateTypedef(ttypedef *sema.Typedef) { _ = ttypedef }

// generateEnum generates code for an enumerated type. Since define is
// expensive to lookup in PERL, we use a global array for this.
func (g *Generator) generateEnum(tenum *sema.Enum) {
	g.fTypes.WriteString("package " + perlNamespace(g.program) + tenum.Name() +
		";  ## no critic (RequireFilenameMatchesPackage)\n")

	for _, c := range tenum.Constants() {
		g.fTypes.WriteString("use constant " + c.Name() + " => " + strconv.FormatInt(int64(c.Value()), 10) + ";\n")
	}
}

// generateConst generates a constant value.
func (g *Generator) generateConst(tconst *sema.Const) {
	g.fConsts.WriteString("use constant " + tconst.Name() + " => ")
	g.fConsts.WriteString(g.renderConstValue(tconst.Type(), tconst.Value()))
	g.fConsts.WriteString(";\n\n")
}

// renderConstValue prints the value of a constant with the given type.
// Note that type checking is NOT performed in this function as it is
// always run beforehand by sema.ValidateInput.
func (g *Generator) renderConstValue(typ sema.Type, value *sema.ConstValue) string {
	var out strings.Builder
	typ = sema.TrueType(typ)

	switch {
	case typ.IsBaseType():
		base := typ.(*sema.BaseType)
		switch base.Base() {
		case sema.TypeString:
			out.WriteString(`"` + perlEscapeString(value.String()) + `"`)
		case sema.TypeBool:
			if value.Integer() > 0 {
				out.WriteString("1")
			} else {
				out.WriteString("0")
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
			emit.Throw("compiler error: no const of base type %s", sema.BaseName(base.Base()))
		}
	case typ.IsEnum():
		out.WriteString(strconv.FormatInt(value.Integer(), 10))
	case typ.IsStruct() || typ.IsXception():
		out.WriteString(perlNamespace(typ.Program()) + typ.Name() + "->new({\n")
		g.indentUp()
		s := typ.(*sema.Struct)
		for _, entry := range value.Map() {
			var fieldType sema.Type
			for _, f := range s.Members() {
				if f.Name() == entry.Key.String() {
					fieldType = f.Type()
				}
			}
			if fieldType == nil {
				emit.Throw("type error: %s has no field %s", typ.Name(), entry.Key.String())
			}
			out.WriteString(g.indent() + g.renderConstValue(sema.GlobalString, entry.Key))
			out.WriteString(" => ")
			out.WriteString(g.renderConstValue(fieldType, entry.Value))
			out.WriteString(",")
			out.WriteString("\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "})")
	case typ.IsMap():
		m := typ.(*sema.Map)
		out.WriteString("{\n")
		g.indentUp()
		for _, entry := range value.Map() {
			out.WriteString(g.indent() + g.renderConstValue(m.KeyType(), entry.Key))
			out.WriteString(" => ")
			out.WriteString(g.renderConstValue(m.ValType(), entry.Value))
			out.WriteString(",\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "}")
	case typ.IsList() || typ.IsSet():
		var etype sema.Type
		if typ.IsList() {
			etype = typ.(*sema.List).ElemType()
		} else {
			etype = typ.(*sema.Set).ElemType()
		}
		out.WriteString("[\n")
		g.indentUp()
		for _, elem := range value.List() {
			out.WriteString(g.indent() + g.renderConstValue(etype, elem))
			if typ.IsSet() {
				out.WriteString(" => 1")
			}
			out.WriteString(",\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "]")
	}
	return out.String()
}

// ---- helper rendering functions ----

// perlEscapeString is t_perl_generator's escape_string: the base
// t_generator table plus '$' -> "\\$" and '@' -> "\\@", set in the
// constructor.
func perlEscapeString(in string) string {
	var sb strings.Builder
	for i := 0; i < len(in); i++ {
		switch c := in[i]; c {
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '$':
			sb.WriteString(`\$`)
		case '@':
			sb.WriteString(`\@`)
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

// formatDouble mimics `ostream << double` with the default precision of
// six significant digits, as the other generator ports' formatDouble
// functions do.
func formatDouble(f float64) string {
	return strconv.FormatFloat(f, 'g', 6, 64)
}

// declareField is t_perl_generator::declare_field: declares a field,
// which may include initialization as necessary.
func declareField(field *sema.Field, initv, obj bool) string {
	result := "my $" + field.Name()
	if initv {
		t := sema.TrueType(field.Type())
		switch {
		case t.IsBaseType():
			switch t.(*sema.BaseType).Base() {
			case sema.TypeVoid:
			case sema.TypeString:
				result += " = ''"
			case sema.TypeBool:
				result += " = 0"
			case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
				result += " = 0"
			case sema.TypeDouble:
				result += " = 0.0"
			default:
				emit.Throw("compiler error: no PERL initializer for base type %s", sema.BaseName(t.(*sema.BaseType).Base()))
			}
		case t.IsEnum():
			result += " = 0"
		case t.IsContainer():
			result += " = []"
		case t.IsStruct() || t.IsXception():
			if obj {
				result += " = " + perlNamespace(t.Program()) + t.Name() + "->new()"
			} else {
				result += " = undef"
			}
		}
	}
	return result + ";"
}

// functionSignature is t_perl_generator::function_signature: renders a
// function signature of the form 'name{\n  my $self = shift;\n...'.
func functionSignature(tfunction *sema.Function, prefix string) string {
	str := prefix + tfunction.Name() + "{\n"
	str += "  my $self = shift;\n"

	for _, f := range tfunction.Arglist().Members() {
		str += "  my $" + f.Name() + " = shift;\n"
	}

	return str
}

// argumentList is t_perl_generator::argument_list: renders a field list.
func argumentList(tstruct *sema.Struct) string {
	result := ""
	first := true
	for _, f := range tstruct.Members() {
		if first {
			first = false
		} else {
			result += ", "
		}
		result += "$" + f.Name()
	}
	return result
}

// typeToEnum is t_perl_generator::type_to_enum: converts the parse type to
// a Thrift::TType constant string.
func typeToEnum(t sema.Type) string {
	t = sema.TrueType(t)
	switch {
	case t.IsBaseType():
		switch t.(*sema.BaseType).Base() {
		case sema.TypeVoid:
			emit.Throw("NO T_VOID CONSTRUCT")
		case sema.TypeString:
			return "Thrift::TType::STRING"
		case sema.TypeBool:
			return "Thrift::TType::BOOL"
		case sema.TypeI8:
			return "Thrift::TType::BYTE"
		case sema.TypeI16:
			return "Thrift::TType::I16"
		case sema.TypeI32:
			return "Thrift::TType::I32"
		case sema.TypeI64:
			return "Thrift::TType::I64"
		case sema.TypeDouble:
			return "Thrift::TType::DOUBLE"
		default:
			emit.Throw("compiler error: unhandled type")
		}
	case t.IsEnum():
		return "Thrift::TType::I32"
	case t.IsStruct() || t.IsXception():
		return "Thrift::TType::STRUCT"
	case t.IsMap():
		return "Thrift::TType::MAP"
	case t.IsSet():
		return "Thrift::TType::SET"
	case t.IsList():
		return "Thrift::TType::LIST"
	}
	emit.Throw("INVALID TYPE IN type_to_enum: %s", t.Name())
	return ""
}

// displayName is t_perl_generator::display_name.
func (g *Generator) displayName() string { return "Perl" }
