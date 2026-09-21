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

// Package st is a method-for-method port of
// compiler/cpp/src/thrift/generate/t_st_generator.cc, the Smalltalk
// generator. The port writes exactly the bytes the C++ generator writes,
// which is checked against the C++ compiler by the parity tests; that is
// why the emitter builds strings the way an ostream would rather than
// through templates.
package st

import (
	"strconv"
	"strings"
	"time"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

// Now is time.Now, overridden by the golden tests so the "generated on"
// stamp st_method writes is reproducible.
var Now = time.Now

// Options are the st:... generator options. There are none: the
// constructor throws on any parsed option.
type Options struct{}

// ParseOptions parses the part after "st:" of a --gen argument.
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
			return o, &emit.Error{Msg: "unknown option st:" + key}
		}
	}
	return o, nil
}

func init() {
	generate.Register(generate.Info{
		Name:     "st",
		LongName: "Smalltalk",
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
	_ = opts // no options yet
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
	outDir := program.OutPath() + "gen-st/"
	if program.IsOutPathAbsolute() {
		outDir = program.OutPath() + "/"
	}
	emit.Mkdir(outDir)
	emit.WriteFile(outDir+program.Name()+".st", Render(program))
	return nil
}

// generator is t_st_generator. sb is f_: the single output file every
// top-level generation function writes into (some indirectly, through the
// out parameter they were ported with). indentLevel is the class's
// indent_ counter: it is shared by every helper that calls indent()/
// indentUp()/indentDown(), including the ones that build a piece of code
// into a local buffer rather than sb, exactly as the C++ methods share
// their instance's indent_ while writing into a local ostringstream.
type generator struct {
	program      *sema.Program
	sb           strings.Builder
	indentLevel  int
	temporaryVar int
	serviceName  string
}

// Render renders the program as the .st file, exactly as t_st_generator
// does through t_generator::generate_program.
func Render(p *sema.Program) string {
	g := &generator{program: p}
	g.generateProgram()
	return g.sb.String()
}

func (g *generator) indent() string { return strings.Repeat("  ", g.indentLevel) }
func (g *generator) indentUp()      { g.indentLevel++ }
func (g *generator) indentDown()    { g.indentLevel-- }

// tempName is t_st_generator::temp_name.
func (g *generator) tempName() string {
	s := "temp" + strconv.Itoa(g.temporaryVar)
	g.temporaryVar++
	return s
}

// generateProgram is t_generator::generate_program's dispatch order,
// which t_st_generator does not override. init_generator's own enum loop
// runs first, and generate_program's enum loop runs again right after it:
// the C++ generator emits every enum twice, and that is reproduced here
// rather than fixed.
func (g *generator) generateProgram() {
	g.initGenerator()

	for _, e := range g.program.Enums() {
		g.generateEnum(e)
	}
	for _, t := range g.program.Typedefs() {
		g.generateTypedef(t)
	}
	// generate_forward_declaration has no override in t_st_generator; the
	// base t_generator's default does nothing.
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

// initGenerator is t_st_generator::init_generator, minus the MKDIR/open
// that generateOne performs once for the whole program.
func (g *generator) initGenerator() {
	g.sb.WriteString(g.stAutogenComment() + "\n")

	g.stClassDef(&g.sb, g.program.Name())
	g.generateClassSideDefinition()

	for _, e := range g.program.Enums() {
		g.generateEnum(e)
	}
}

// closeGenerator is t_st_generator::close_generator, minus the file
// close that generateOne performs.
func (g *generator) closeGenerator() {
	g.generateForceConsts()
}

// generateForceConsts is t_st_generator::generate_force_consts.
func (g *generator) generateForceConsts() {
	g.sb.WriteString(g.prefix(g.className()) + " enums keysAndValuesDo: [:k :v | " + g.prefix(g.className()) +
		" enums at: k put: v value].!" + "\n")
	g.sb.WriteString(g.prefix(g.className()) + " constants keysAndValuesDo: [:k :v | " + g.prefix(g.className()) +
		" constants at: k put: v value].!" + "\n")
}

// className is t_st_generator::class_name.
func (g *generator) className() string { return capitalize(g.program.Name()) }

// isValidNamespace is t_st_generator::is_valid_namespace. It is called by
// the C++ parser when checking a "namespace smalltalk.<sub>" declaration
// and is not otherwise wired into generation; kept for parity with the
// class it was ported from.
func isValidNamespace(subNamespace string) bool {
	return subNamespace == "prefix" || subNamespace == "category"
}

// prefix is t_st_generator::prefix: the smalltalk.prefix namespace,
// prepended to the capitalized name.
func (g *generator) prefix(className string) string {
	pfx := g.program.Namespace("smalltalk.prefix")
	name := capitalize(className)
	if pfx != "" {
		name = pfx + name
	}
	return name
}

// clientClassName is t_st_generator::client_class_name.
func (g *generator) clientClassName() string { return capitalize(g.serviceName) + "Client" }

// stAutogenComment is t_st_generator::st_autogen_comment.
func (g *generator) stAutogenComment() string {
	return "'" + "Autogenerated by Thrift Compiler (" + version.Version + ")\n" +
		"\n" + "DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING\n" + "'!\n"
}

// generatedCategory is t_st_generator::generated_category. For
// compatibility with the Thrift grammar, the category must be punctuated
// by dots; they are replaced with dashes here.
func (g *generator) generatedCategory() string {
	cat := strings.ReplaceAll(g.program.Namespace("smalltalk.category"), ".", "-")
	if cat != "" {
		return cat
	}
	return "Generated-" + g.className()
}

// generateTypedef is t_st_generator::generate_typedef: typedefs are not
// represented in Smalltalk, types are all implicit.
func (g *generator) generateTypedef(t *sema.Typedef) {}

// stClassDef is t_st_generator::st_class_def. It calls indentUp with no
// matching indentDown, so the level it leaves behind is the floor for
// every indent() call for the rest of generation; that leak is in the
// C++ source and is reproduced rather than fixed.
func (g *generator) stClassDef(out *strings.Builder, name string) {
	out.WriteString("Object subclass: #" + g.prefix(name) + "\n")
	g.indentUp()
	out.WriteString(g.indent() + "instanceVariableNames: ''" + "\n" + g.indent() + "classVariableNames: ''" +
		"\n" + g.indent() + "poolDictionaries: ''" + "\n" + g.indent() + "category: '" +
		g.generatedCategory() + "'!" + "\n" + "\n")
}

// stMethod is t_st_generator::st_method, the four-argument overload; the
// three-argument overload's default category ("as yet uncategorized") is
// passed explicitly at every call site instead.
func (g *generator) stMethod(out *strings.Builder, cls, name, category string) {
	timestr := Now().Local().Format("01/02/2006 15:04")
	out.WriteString("!" + g.prefix(cls) + " methodsFor: '" + category + "' stamp: 'thrift " + timestr +
		"'!\n" + name + "\n")
	g.indentUp()
	out.WriteString(g.indent())
}

// stCloseMethod is t_st_generator::st_close_method.
func (g *generator) stCloseMethod(out *strings.Builder) {
	out.WriteString("! !" + "\n" + "\n")
	g.indentDown()
}

// stClassMethod is t_st_generator::st_class_method, the two-argument
// overload (the only one ever called). The three-argument overload does
// not append " class" to cls and is otherwise a plain forward to
// stMethod; it is not ported since it is never called.
func (g *generator) stClassMethod(out *strings.Builder, cls, name string) {
	g.stMethod(out, cls+" class", name, "as yet uncategorized")
}

// stSetter is t_st_generator::st_setter.
func (g *generator) stSetter(out *strings.Builder, cls, name, typ string) {
	g.stMethod(out, cls, name+": "+typ, "as yet uncategorized")
	out.WriteString(name + " := " + typ)
	g.stCloseMethod(out)
}

// stGetter is t_st_generator::st_getter.
func (g *generator) stGetter(out *strings.Builder, cls, name string) {
	g.stMethod(out, cls, name, "as yet uncategorized")
	out.WriteString("^ " + name)
	g.stCloseMethod(out)
}

// stAccessors is t_st_generator::st_accessors.
func (g *generator) stAccessors(out *strings.Builder, cls, name, typ string) {
	g.stSetter(out, cls, name, typ)
	g.stGetter(out, cls, name)
}

// generateClassSideDefinition is t_st_generator::generate_class_side_definition.
func (g *generator) generateClassSideDefinition() {
	g.sb.WriteString(g.prefix(g.className()) + " class" + "\n" + "\tinstanceVariableNames: 'constants enums'!" +
		"\n" + "\n")

	g.stAccessors(&g.sb, g.className()+" class", "enums", "anObject")
	g.stAccessors(&g.sb, g.className()+" class", "constants", "anObject")

	g.sb.WriteString(g.prefix(g.className()) + " enums: Dictionary new!" + "\n")
	g.sb.WriteString(g.prefix(g.className()) + " constants: Dictionary new!" + "\n")

	g.sb.WriteString("\n")
}

// generateEnum is t_st_generator::generate_enum: a class-scoped
// dictionary of the enum's values.
func (g *generator) generateEnum(e *sema.Enum) {
	g.sb.WriteString(g.prefix(g.className()) + " enums at: '" + e.Name() + "' put: [" +
		"(Dictionary new " + "\n")

	for _, c := range e.Constants() {
		g.sb.WriteString("\tat: '" + c.Name() + "' put: " + strconv.FormatInt(int64(c.Value()), 10) + ";" + "\n")
	}

	g.sb.WriteString("\tyourself)]!" + "\n" + "\n")
}

// generateConst is t_st_generator::generate_const.
func (g *generator) generateConst(c *sema.Const) {
	g.sb.WriteString(g.prefix(g.className()) + " constants at: '" + c.Name() + "' put: [" +
		g.renderConstValue(c.Type(), c.Value()) + "]!" + "\n" + "\n")
}

// renderConstValue is t_st_generator::render_const_value. Type checking
// is not performed here, as it is always run beforehand by
// sema.ValidateInput.
func (g *generator) renderConstValue(typ sema.Type, value *sema.ConstValue) string {
	typ = sema.TrueType(typ)
	var out strings.Builder

	if typ.IsBaseType() {
		base := typ.(*sema.BaseType)
		switch base.Base() {
		case sema.TypeString:
			out.WriteString("\"" + emit.EscapeString(value.String()) + "\"")
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
				out.WriteString(strconv.FormatInt(value.Integer(), 10))
			} else {
				out.WriteString(formatDouble(value.Double()))
			}
		default:
			emit.Throw("compiler error: no const of base type %s", sema.BaseName(base.Base()))
		}
	} else if typ.IsEnum() {
		out.WriteString(g.indent() + strconv.FormatInt(value.Integer(), 10))
	} else if typ.IsStruct() || typ.IsXception() {
		s := typ.(*sema.Struct)
		out.WriteString("(" + capitalize(typ.Name()) + " new " + "\n")
		g.indentUp()
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
			out.WriteString(g.indent() + entry.Key.String() + ": " + g.renderConstValue(fieldType, entry.Value) + ";" + "\n")
		}
		out.WriteString(g.indent() + "yourself)")
		g.indentDown()
	} else if typ.IsMap() {
		m := typ.(*sema.Map)
		out.WriteString("(Dictionary new" + "\n")
		g.indentUp()
		g.indentUp()
		for _, entry := range value.Map() {
			out.WriteString(g.indent() + g.indent())
			out.WriteString("at: " + g.renderConstValue(m.KeyType(), entry.Key))
			out.WriteString(" put: ")
			out.WriteString(g.renderConstValue(m.ValType(), entry.Value))
			out.WriteString(";" + "\n")
		}
		out.WriteString(g.indent() + g.indent() + "yourself)")
		g.indentDown()
		g.indentDown()
	} else if typ.IsList() || typ.IsSet() {
		var elemType sema.Type
		if typ.IsList() {
			elemType = typ.(*sema.List).ElemType()
		} else {
			elemType = typ.(*sema.Set).ElemType()
		}
		if typ.IsSet() {
			out.WriteString("(Set new" + "\n")
		} else {
			out.WriteString("(OrderedCollection new" + "\n")
		}
		g.indentUp()
		g.indentUp()
		for _, e := range value.List() {
			out.WriteString(g.indent() + g.indent())
			out.WriteString("add: " + g.renderConstValue(elemType, e))
			out.WriteString(";" + "\n")
		}
		out.WriteString(g.indent() + g.indent() + "yourself)")
		g.indentDown()
		g.indentDown()
	} else {
		emit.Throw("CANNOT GENERATE CONSTANT FOR TYPE: %s", typ.Name())
	}

	return out.String()
}

// generateStruct is t_st_generator::generate_struct.
func (g *generator) generateStruct(s *sema.Struct) {
	g.generateStStruct(&g.sb, s, false)
}

// generateXception is t_st_generator::generate_xception: a struct
// definition that extends Error instead of Object.
func (g *generator) generateXception(s *sema.Struct) {
	g.generateStStruct(&g.sb, s, true)
}

// generateStStruct is t_st_generator::generate_st_struct: a Smalltalk
// class to represent a struct.
func (g *generator) generateStStruct(out *strings.Builder, tstruct *sema.Struct, isException bool) {
	members := tstruct.Members()

	if isException {
		out.WriteString("Error")
	} else {
		out.WriteString("Object")
	}

	out.WriteString(" subclass: #" + g.prefix(g.typeName(tstruct)) + "\n" + "\tinstanceVariableNames: '")

	for i, m := range members {
		if i != 0 {
			out.WriteString(" ")
		}
		out.WriteString(camelcase(m.Name()))
	}

	out.WriteString("'\n" +
		"\tclassVariableNames: ''\n" +
		"\tpoolDictionaries: ''\n" +
		"\tcategory: '" + g.generatedCategory() + "'!\n\n")

	g.generateAccessors(out, tstruct)
	g.generateSerialization(out, tstruct)
}

// generateAccessors is t_st_generator::generate_accessors.
func (g *generator) generateAccessors(out *strings.Builder, tstruct *sema.Struct) {
	members := tstruct.Members()
	if len(members) > 0 {
		for _, m := range members {
			g.stAccessors(out, capitalize(g.typeName(tstruct)), camelcase(m.Name()), g.aType(m.Type()))
		}
		out.WriteString("\n")
	}
}

// generateSerialization is t_st_generator::generate_serialization. See
// the C++ source for why the recursion is emitted once per struct and
// called, rather than expanded inline (THRIFT-6062).
func (g *generator) generateSerialization(out *strings.Builder, tstruct *sema.Struct) {
	cls := capitalize(g.typeName(tstruct))

	g.stMethod(out, cls, "writeTo: oprot", "as yet uncategorized")
	out.WriteString(g.structWriter(tstruct, "self"))
	g.stCloseMethod(out)

	g.stClassMethod(out, cls, "readFrom: iprot")
	out.WriteString("^ " + g.structReader(tstruct, tstruct.Name()))
	g.stCloseMethod(out)

	out.WriteString("\n")
}

// isVowel is t_st_generator::is_vowel.
func isVowel(c byte) bool {
	switch toLowerByte(c) {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

// aType is t_st_generator::a_type. type_name(type) is empty for an
// anonymous container type (list/set/map get no name_ unless typedef'd);
// C++'s std::string::operator[] at pos == size() reads the null
// terminator there, which is_vowel treats as not a vowel, so an empty
// name renders as just "a" rather than indexing out of bounds.
func (g *generator) aType(t sema.Type) string {
	name := g.typeName(t)
	article := "a"
	if name != "" && isVowel(name[0]) {
		article = "an"
	}
	return article + capitalize(name)
}

// generateService is t_st_generator::generate_service.
func (g *generator) generateService(s *sema.Service) {
	g.generateServiceClient(s)
}

// generateServiceClient is t_st_generator::generate_service_client. The
// superclass name is written as-is, without going through prefix, even
// though the subclass name does; that asymmetry is in the C++ source.
func (g *generator) generateServiceClient(tservice *sema.Service) {
	extendsClient := "TClient"
	functions := tservice.Functions()

	if tservice.Extends() != nil {
		extendsClient = g.typeName(tservice.Extends()) + "Client"
	}

	g.sb.WriteString(extendsClient + " subclass: #" + g.prefix(g.clientClassName()) + "\n" +
		"\tinstanceVariableNames: ''\n" +
		"\tclassVariableNames: ''\n" +
		"\tpoolDictionaries: ''\n" +
		"\tcategory: '" + g.generatedCategory() + "'!\n\n")

	for _, fn := range functions {
		funname := camelcase(fn.Name())
		signature := g.functionSignature(fn)

		g.stMethod(&g.sb, g.clientClassName(), signature, "as yet uncategorized")
		g.sb.WriteString(g.functionTypesComment(fn) + "\n" + g.indent() + "self send" + capitalize(signature) + "." + "\n")

		if !fn.IsOneway() {
			g.sb.WriteString(g.indent() + "^ self recv" + capitalize(funname) + " success " + "\n")
		}

		g.stCloseMethod(&g.sb)

		g.generateSendMethod(fn)
		if !fn.IsOneway() {
			g.generateRecvMethod(fn)
		}
	}
}

// generateSendMethod is t_st_generator::generate_send_method.
func (g *generator) generateSendMethod(fn *sema.Function) {
	funname := fn.Name()
	signature := g.functionSignature(fn)
	fields := fn.Arglist().Members()

	g.stMethod(&g.sb, g.clientClassName(), "send"+capitalize(signature), "as yet uncategorized")
	g.sb.WriteString("oprot writeMessageBegin:" + "\n")
	g.indentUp()

	g.sb.WriteString(g.indent() + "(TCallMessage new" + "\n")
	g.indentUp()

	g.sb.WriteString(g.indent() + "name: '" + funname + "'; " + "\n" + g.indent() + "seqid: self nextSeqid)." + "\n")
	g.indentDown()
	g.indentDown()

	g.sb.WriteString(g.indent() + "oprot writeStructBegin: " +
		"(TStruct new name: '" + capitalize(camelcase(funname)) + "_args')." + "\n")

	for _, fld := range fields {
		fname := camelcase(fld.Name())

		g.sb.WriteString(g.indent() + "oprot writeFieldBegin: (TField new name: '" + fname +
			"'; type: " + g.typeToEnum(fld.Type()) + "; id: " + strconv.Itoa(int(fld.Key())) + ")." + "\n")

		g.sb.WriteString(g.indent() + g.writeVal(fld.Type(), fname) + "." + "\n" + g.indent() + "oprot writeFieldEnd." + "\n")
	}

	g.sb.WriteString(g.indent() + "oprot writeFieldStop; writeStructEnd; writeMessageEnd." + "\n")
	g.sb.WriteString(g.indent() + "oprot transport flush")

	g.stCloseMethod(&g.sb)
}

// generateRecvMethod is t_st_generator::generate_recv_method. Only
// receiving a TResult structure is supported, so this will not work on
// the server side, as the C++ comment says.
//
// The envelope is read with iprot, like the struct inside it. A client
// that never calls outProtocol: has oprot and iprot pointing at the same
// object, which is why reading the envelope from oprot went unnoticed;
// one that does would read the reply from the protocol it writes to. The
// flush is a write side operation and stays on oprot.
func (g *generator) generateRecvMethod(fn *sema.Function) {
	funname := camelcase(fn.Name())

	result := sema.NewStruct(g.program)
	result.SetName("TResult")
	success := sema.NewField(fn.ReturnType(), "success", 0)
	result.Append(success)

	xs := fn.Xceptions()
	for _, f := range xs.Members() {
		// duplicate the field, but call it "exception"... we don't need a
		// dynamic name. Append silently drops one past the first, since
		// they all share that name; the C++ source ignores the same
		// return value.
		exception := sema.NewField(f.Type(), "exception", f.Key())
		result.Append(exception)
	}

	g.stMethod(&g.sb, g.clientClassName(), "recv"+capitalize(funname), "as yet uncategorized")
	g.sb.WriteString("| msg res | " + "\n" + g.indent() + "msg := iprot readMessageBegin." + "\n" + g.indent() +
		"self validateRemoteMessage: msg." + "\n" + g.indent() +
		"res := " + g.structReader(result, "") + "." + "\n" + g.indent() + "iprot readMessageEnd." +
		"\n" + g.indent() + "oprot transport flush." + "\n" + g.indent() +
		"res exception ifNotNil: [res exception signal]." + "\n" + g.indent() + "^ res")
	g.stCloseMethod(&g.sb)
}

// functionTypesComment is t_st_generator::function_types_comment.
func (g *generator) functionTypesComment(fn *sema.Function) string {
	var out strings.Builder
	fields := fn.Arglist().Members()

	out.WriteString("\"")
	for i, f := range fields {
		out.WriteString(camelcase(f.Name()) + ": " + g.typeName(f.Type()))
		if i != len(fields)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString("\"")

	return out.String()
}

// functionSignature is t_st_generator::function_signature: a rendered
// function signature of the form 'type name(args)'.
func (g *generator) functionSignature(fn *sema.Function) string {
	return camelcase(fn.Name()) + capitalize(g.argumentList(fn.Arglist()))
}

// argumentList is t_st_generator::argument_list: a rendered field list.
func (g *generator) argumentList(tstruct *sema.Struct) string {
	var result strings.Builder
	first := true
	for _, f := range tstruct.Members() {
		if first {
			first = false
		} else {
			result.WriteString(" ")
		}
		name := camelcase(f.Name())
		result.WriteString(name + ": " + name)
	}
	return result.String()
}

// typeName is t_st_generator::type_name.
func (g *generator) typeName(t sema.Type) string {
	prefix := ""
	if p := t.Program(); p != nil && p != g.program {
		if !t.IsService() {
			prefix = p.Name() + "_types."
		}
	}

	name := t.Name()
	if t.IsStruct() || t.IsXception() {
		name = capitalize(t.Name())
	}

	return prefix + name
}

// typeToEnum converts the parse type to a Smalltalk TType, as
// t_st_generator::type_to_enum.
func (g *generator) typeToEnum(t sema.Type) string {
	t = sema.TrueType(t)
	if t.IsBaseType() {
		base := t.(*sema.BaseType)
		switch base.Base() {
		case sema.TypeVoid:
			emit.Throw("NO T_VOID CONSTRUCT")
		case sema.TypeString:
			return "TType string"
		case sema.TypeBool:
			return "TType bool"
		case sema.TypeI8:
			return "TType byte"
		case sema.TypeI16:
			return "TType i16"
		case sema.TypeI32:
			return "TType i32"
		case sema.TypeI64:
			return "TType i64"
		case sema.TypeDouble:
			return "TType double"
		default:
			emit.Throw("compiler error: unhandled type")
		}
	} else if t.IsEnum() {
		return "TType i32"
	} else if t.IsStruct() || t.IsXception() {
		return "TType struct"
	} else if t.IsMap() {
		return "TType map"
	} else if t.IsSet() {
		return "TType set"
	} else if t.IsList() {
		return "TType list"
	}
	emit.Throw("INVALID TYPE IN type_to_enum: %s", t.Name())
	return ""
}

// mapWriter is t_st_generator::map_writer.
func (g *generator) mapWriter(tmap *sema.Map, fname string) string {
	var out strings.Builder
	key := g.tempName()
	val := g.tempName()

	out.WriteString("[oprot writeMapBegin: (TMap new keyType: " + g.typeToEnum(tmap.KeyType()) +
		"; valueType: " + g.typeToEnum(tmap.ValType()) + "; size: " + fname + " size)." + "\n")
	g.indentUp()

	out.WriteString(g.indent() + fname + " keysAndValuesDo: [:" + key + " :" + val + " |" + "\n")
	g.indentUp()

	out.WriteString(g.indent() + g.writeVal(tmap.KeyType(), key) + "." + "\n" + g.indent() +
		g.writeVal(tmap.ValType(), val))
	g.indentDown()

	out.WriteString("]." + "\n" + g.indent() + "oprot writeMapEnd] value")
	g.indentDown()

	return out.String()
}

// mapReader is t_st_generator::map_reader.
func (g *generator) mapReader(tmap *sema.Map) string {
	var out strings.Builder
	desc := g.tempName()
	val := g.tempName()

	out.WriteString("[|" + desc + " " + val + "| " + "\n")
	g.indentUp()

	out.WriteString(g.indent() + desc + " := iprot readMapBegin." + "\n" + g.indent() + val +
		" := Dictionary new." + "\n" + g.indent() + desc + " size timesRepeat: [" + "\n")

	g.indentUp()
	out.WriteString(g.indent() + val + " at: " + g.readVal(tmap.KeyType()) +
		" put: " + g.readVal(tmap.ValType()))
	g.indentDown()

	out.WriteString("]." + "\n" + g.indent() + "iprot readMapEnd." + "\n" + g.indent() + val + "] value")
	g.indentDown()

	return out.String()
}

// listWriter is t_st_generator::list_writer.
func (g *generator) listWriter(tlist *sema.List, fname string) string {
	var out strings.Builder
	val := g.tempName()

	out.WriteString("[oprot writeListBegin: (TList new elemType: " + g.typeToEnum(tlist.ElemType()) +
		"; size: " + fname + " size)." + "\n")
	g.indentUp()

	out.WriteString(g.indent() + fname + " do: [:" + val + "|" + "\n")
	g.indentUp()

	out.WriteString(g.indent() + g.writeVal(tlist.ElemType(), val) + "\n")
	g.indentDown()

	out.WriteString("]." + "\n" + g.indent() + "oprot writeListEnd] value")
	g.indentDown()

	return out.String()
}

// listReader is t_st_generator::list_reader.
func (g *generator) listReader(tlist *sema.List) string {
	var out strings.Builder
	desc := g.tempName()
	val := g.tempName()

	out.WriteString("[|" + desc + " " + val + "| " + desc + " := iprot readListBegin." + "\n")
	g.indentUp()

	out.WriteString(g.indent() + val + " := OrderedCollection new." + "\n" + g.indent() + desc +
		" size timesRepeat: [" + "\n")

	g.indentUp()
	out.WriteString(g.indent() + val + " add: " + g.readVal(tlist.ElemType()))
	g.indentDown()

	out.WriteString("]." + "\n" + g.indent() + "iprot readListEnd." + "\n" + g.indent() + val + "] value")
	g.indentDown()

	return out.String()
}

// setWriter is t_st_generator::set_writer.
func (g *generator) setWriter(tset *sema.Set, fname string) string {
	var out strings.Builder
	val := g.tempName()

	out.WriteString("[oprot writeSetBegin: (TSet new elemType: " + g.typeToEnum(tset.ElemType()) +
		"; size: " + fname + " size)." + "\n")
	g.indentUp()

	out.WriteString(g.indent() + fname + " do: [:" + val + "|" + "\n")
	g.indentUp()

	out.WriteString(g.indent() + g.writeVal(tset.ElemType(), val) + "\n")
	g.indentDown()

	out.WriteString("]." + "\n" + g.indent() + "oprot writeSetEnd] value")
	g.indentDown()

	return out.String()
}

// setReader is t_st_generator::set_reader.
func (g *generator) setReader(tset *sema.Set) string {
	var out strings.Builder
	desc := g.tempName()
	val := g.tempName()

	out.WriteString("[|" + desc + " " + val + "| " + desc + " := iprot readSetBegin." + "\n")
	g.indentUp()

	out.WriteString(g.indent() + val + " := Set new." + "\n" + g.indent() + desc + " size timesRepeat: [" + "\n")

	g.indentUp()
	out.WriteString(g.indent() + val + " add: " + g.readVal(tset.ElemType()))
	g.indentDown()

	out.WriteString("]." + "\n" + g.indent() + "iprot readSetEnd." + "\n" + g.indent() + val + "] value")
	g.indentDown()

	return out.String()
}

// structWriter is t_st_generator::struct_writer.
func (g *generator) structWriter(tstruct *sema.Struct, sname string) string {
	var out strings.Builder
	fields := tstruct.SortedMembers()

	out.WriteString("[oprot incrementRecursionDepth." + "\n")
	g.indentUp()
	out.WriteString(g.indent() + "[oprot writeStructBegin: " +
		"(TStruct new name: '" + tstruct.Name() + "')." + "\n")

	for _, fld := range fields {
		optional := fld.Req() == sema.Optional
		fname := camelcase(fld.Name())
		accessor := sname + " " + camelcase(fname)

		if optional {
			out.WriteString(g.indent() + accessor + " ifNotNil: [" + "\n")
			g.indentUp()
		}

		out.WriteString(g.indent() + "oprot writeFieldBegin: (TField new name: '" + fname +
			"'; type: " + g.typeToEnum(fld.Type()) + "; id: " + strconv.Itoa(int(fld.Key())) + ")." + "\n")

		out.WriteString(g.indent() + g.writeVal(fld.Type(), accessor) + "." + "\n" + g.indent() + "oprot writeFieldEnd")

		if optional {
			out.WriteString("]")
			g.indentDown()
		}

		out.WriteString("." + "\n")
	}

	out.WriteString(g.indent() + "oprot writeFieldStop; writeStructEnd] ensure: [oprot decrementRecursionDepth]] value")
	g.indentDown()

	return out.String()
}

// structReader is t_st_generator::struct_reader. clsName is passed "" at
// the one call site that relied on the C++ default argument.
func (g *generator) structReader(tstruct *sema.Struct, clsName string) string {
	var out strings.Builder
	fields := tstruct.Members()
	val := g.tempName()
	desc := g.tempName()
	found := g.tempName()

	if clsName == "" {
		clsName = tstruct.Name()
	}

	out.WriteString("[|" + desc + " " + val + "|" + "\n")
	g.indentUp()

	// This is nasty, but without it we'll break things by prefixing TResult.
	name := g.prefix(clsName)
	if capitalize(clsName) == "TResult" {
		name = capitalize(clsName)
	}
	out.WriteString(g.indent() + val + " := " + name + " new." + "\n")
	out.WriteString(g.indent() + "iprot incrementRecursionDepth." + "\n")

	out.WriteString(g.indent() + "[iprot readStructBegin." + "\n" +
		g.indent() + "[" + desc + " := iprot readFieldBegin." + "\n" +
		g.indent() + desc + " type = TType stop] whileFalse: [|" + found + "|" + "\n")
	g.indentUp()

	for _, fld := range fields {
		out.WriteString(g.indent() + desc + " id = " + strconv.Itoa(int(fld.Key())) + " ifTrue: [" + "\n")
		g.indentUp()

		out.WriteString(g.indent() + found + " := true." + "\n" + g.indent() + val + " " +
			camelcase(fld.Name()) + ": " + g.readVal(fld.Type()))
		g.indentDown()

		out.WriteString("]." + "\n")
	}

	out.WriteString(g.indent() + found + " ifNil: [iprot skip: " + desc + " type]]." + "\n")
	g.indentDown()

	out.WriteString(g.indent() + "iprot readStructEnd] ensure: [iprot decrementRecursionDepth]." + "\n" +
		g.indent() + val + "] value")
	g.indentDown()

	return out.String()
}

// writeVal is t_st_generator::write_val.
func (g *generator) writeVal(t sema.Type, fname string) string {
	t = sema.TrueType(t)

	if t.IsBaseType() {
		base := t.(*sema.BaseType)
		switch base.Base() {
		case sema.TypeDouble:
			return "oprot writeDouble: " + fname + " asFloat"
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			return "oprot write" + capitalize(g.typeName(t)) + ": " + fname + " asInteger"
		default:
			return "oprot write" + capitalize(g.typeName(t)) + ": " + fname
		}
	} else if t.IsMap() {
		return g.mapWriter(t.(*sema.Map), fname)
	} else if t.IsStruct() || t.IsXception() {
		return fname + " writeTo: oprot"
	} else if t.IsList() {
		return g.listWriter(t.(*sema.List), fname)
	} else if t.IsSet() {
		return g.setWriter(t.(*sema.Set), fname)
	} else if t.IsEnum() {
		return "oprot writeI32: " + fname
	}
	emit.Throw("Sorry, I don't know how to write this: %s", g.typeName(t))
	return ""
}

// readVal is t_st_generator::read_val.
func (g *generator) readVal(t sema.Type) string {
	t = sema.TrueType(t)

	if t.IsBaseType() {
		return "iprot read" + capitalize(g.typeName(t))
	} else if t.IsMap() {
		return g.mapReader(t.(*sema.Map))
	} else if t.IsStruct() || t.IsXception() {
		// Parenthesised: a read_val result is used as a keyword argument, and
		// "coll add: Foo readFrom: iprot" would parse as one add:readFrom: send.
		// Every other branch returns a primary or a bracketed block, so this is
		// the only one that needs it.
		return "(" + g.prefix(g.typeName(t)) + " readFrom: iprot)"
	} else if t.IsList() {
		return g.listReader(t.(*sema.List))
	} else if t.IsSet() {
		return g.setReader(t.(*sema.Set))
	} else if t.IsEnum() {
		return "iprot readI32"
	}
	emit.Throw("Sorry, I don't know how to read this: %s", g.typeName(t))
	return ""
}

// ---- t_generator string helpers shared with the other ports ----

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

// camelcase is the base t_generator::camelcase (not a language-specific
// override): a_multi_word -> aMultiWord.
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

// formatDouble mimics `ostream << double` with the default precision of
// six significant digits.
func formatDouble(f float64) string {
	return strconv.FormatFloat(f, 'g', 6, 64)
}
