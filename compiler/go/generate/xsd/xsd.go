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

// Package xsd is t_xsd_generator.cc: it creates an XSD for the base types
// and structs, plus a PHP file mapping the list element names the XSD
// invents back to their thrift types.
package xsd

import (
	"sort"
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

// Options are the xsd:... generator options. There are none yet.
type Options struct{}

// ParseOptions parses the part after "xsd:" of a --gen argument.
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
			return o, &emit.Error{Msg: "unknown option xsd:" + key}
		}
	}
	return o, nil
}

func init() {
	generate.Register(generate.Info{
		Name:     "xsd",
		LongName: "XSD",
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

	outDir := program.OutPath() + "gen-xsd/"
	if program.IsOutPathAbsolute() {
		outDir = program.OutPath() + "/"
	}

	g := &generator{program: program, outDir: outDir}
	g.generateProgram()
	return nil
}

// generator is t_xsd_generator: one instance generates one program, the
// way the C++ driver constructs a fresh generator per program even under
// -r, so its accumulated state (the type definitions and the PHP map)
// never crosses a program boundary.
type generator struct {
	program *sema.Program
	outDir  string

	// indentLevel is t_generator's indent_, shared by every buffer this
	// generator writes to, exactly as the C++ member is.
	indentLevel int

	// sXsdTypes is s_xsd_types_: the accumulated complexType/simpleType
	// definitions, written once and copied into every service's XSD file.
	sXsdTypes strings.Builder

	// phpBuf is f_php_: the PHP file mapping list element names to their
	// thrift type names, opened in initGenerator and closed in
	// closeGenerator.
	phpBuf strings.Builder
}

// generateProgram is t_generator::generate_program: init, then enums,
// typedefs, structs and exceptions in declared order, consts (a no-op for
// xsd) and services, then close.
func (g *generator) generateProgram() {
	g.initGenerator()

	for _, e := range g.program.Enums() {
		g.generateEnum(e)
	}

	for _, t := range g.program.Typedefs() {
		g.generateTypedef(t)
	}

	// generate_xception defaults to generate_struct, and t_xsd_generator
	// does not override it, so structs and exceptions in declared order
	// (get_objects) all go through generateStruct.
	for _, s := range g.program.Objects() {
		g.generateStruct(s)
	}

	// generate_const defaults to a no-op, and t_xsd_generator does not
	// override it either, so constants produce nothing.

	for _, s := range g.program.Services() {
		g.generateService(s)
	}

	g.closeGenerator()
}

func (g *generator) initGenerator() {
	// Make output directory
	emit.Mkdir(g.outDir)

	g.phpBuf.WriteString("<?php" + "\n" + g.autogenComment() + "\n")
}

func (g *generator) closeGenerator() {
	g.phpBuf.WriteString("?>" + "\n")
	emit.WriteFile(g.outDir+g.program.Name()+"_xsd.php", g.phpBuf.String())
}

func (g *generator) displayName() string {
	return "XSD"
}

func (g *generator) generateTypedef(ttypedef *sema.Typedef) {
	g.writeIndent(&g.sXsdTypes)
	g.sXsdTypes.WriteString("<xsd:simpleType name=\"" + ttypedef.Name() + "\">\n")
	g.indentUp()
	g.writeIndent(&g.sXsdTypes)
	g.sXsdTypes.WriteString("<xsd:restriction base=\"" + g.typeName(ttypedef.Type()) + "\" />\n")
	g.indentDown()
	g.writeIndent(&g.sXsdTypes)
	g.sXsdTypes.WriteString("</xsd:simpleType>\n\n")
}

func (g *generator) generateEnum(tenum *sema.Enum) {
	_ = tenum
}

func (g *generator) generateStruct(tstruct *sema.Struct) {
	members := tstruct.Members()
	xsdAll := tstruct.XsdAll()

	g.writeIndent(&g.sXsdTypes)
	g.sXsdTypes.WriteString("<xsd:complexType name=\"" + tstruct.Name() + "\">\n")
	g.indentUp()
	if xsdAll {
		g.writeIndent(&g.sXsdTypes)
		g.sXsdTypes.WriteString("<xsd:all>\n")
	} else {
		g.writeIndent(&g.sXsdTypes)
		g.sXsdTypes.WriteString("<xsd:sequence>\n")
	}
	g.indentUp()

	for _, m := range members {
		g.generateElement(&g.sXsdTypes, m.Name(), m.Type(), m.XsdAttrs(), m.XsdOptional() || xsdAll, m.XsdNillable(), false)
	}

	g.indentDown()
	if xsdAll {
		g.writeIndent(&g.sXsdTypes)
		g.sXsdTypes.WriteString("</xsd:all>\n")
	} else {
		g.writeIndent(&g.sXsdTypes)
		g.sXsdTypes.WriteString("</xsd:sequence>\n")
	}
	g.indentDown()
	g.writeIndent(&g.sXsdTypes)
	g.sXsdTypes.WriteString("</xsd:complexType>\n\n")
}

// generateElement is t_xsd_generator::generate_element. It writes into
// out, which is either sXsdTypes (a struct member) or a service's own
// XSD buffer (a response or exception element), and recurses on a list's
// element type using out unchanged.
func (g *generator) generateElement(out *strings.Builder, name string, ttype sema.Type, attrs *sema.Struct, optional, nillable, listElement bool) {
	sMinOccurs := ""
	if optional || listElement {
		sMinOccurs = " minOccurs=\"0\""
	}
	sMaxOccurs := ""
	if listElement {
		sMaxOccurs = " maxOccurs=\"unbounded\""
	}
	sOptional := sMinOccurs + sMaxOccurs
	sNillable := ""
	if nillable {
		sNillable = " nillable=\"true\""
	}

	// ttype is used as-is here, never resolved through TrueType: a
	// typedef aliasing void or a list does not take this branch, because
	// t_typedef does not override is_void/is_list in the C++ model
	// either, and the Go Typedef does not override them.
	if ttype.IsVoid() || ttype.IsList() {
		g.writeIndent(out)
		out.WriteString("<xsd:element name=\"" + name + "\"" + sOptional + sNillable + ">\n")
		g.indentUp()
		if attrs == nil && ttype.IsVoid() {
			g.writeIndent(out)
			out.WriteString("<xsd:complexType />\n")
		} else {
			g.writeIndent(out)
			out.WriteString("<xsd:complexType>\n")
			g.indentUp()
			if ttype.IsList() {
				g.writeIndent(out)
				out.WriteString("<xsd:sequence minOccurs=\"0\" maxOccurs=\"unbounded\">\n")
				g.indentUp()
				var subname string
				subtype := ttype.(*sema.List).ElemType()
				if subtype.IsBaseType() || subtype.IsContainer() {
					subname = name + "_elt"
				} else {
					subname = g.typeName(subtype)
				}
				g.phpBuf.WriteString("$GLOBALS['" + g.program.Name() + "_xsd_elt_" + name + "'] = '" + subname + "';\n")
				g.generateElement(out, subname, subtype, nil, false, false, true)
				g.indentDown()
				g.writeIndent(out)
				out.WriteString("</xsd:sequence>\n")
				g.writeIndent(out)
				out.WriteString("<xsd:attribute name=\"list\" type=\"xsd:boolean\" />\n")
			}
			if attrs != nil {
				for _, a := range attrs.Members() {
					g.writeIndent(out)
					out.WriteString("<xsd:attribute name=\"" + a.Name() + "\" type=\"" + g.typeName(a.Type()) + "\" />\n")
				}
			}
			g.indentDown()
			g.writeIndent(out)
			out.WriteString("</xsd:complexType>\n")
		}
		g.indentDown()
		g.writeIndent(out)
		out.WriteString("</xsd:element>\n")
	} else {
		if attrs == nil {
			g.writeIndent(out)
			out.WriteString("<xsd:element name=\"" + name + "\"" + " type=\"" + g.typeName(ttype) + "\"" + sOptional + sNillable + " />\n")
		} else {
			// Wow, all this work for a SIMPLE TYPE with attributes?!?!?!
			g.writeIndent(out)
			out.WriteString("<xsd:element name=\"" + name + "\"" + sOptional + sNillable + ">\n")
			g.indentUp()
			g.writeIndent(out)
			out.WriteString("<xsd:complexType>\n")
			g.indentUp()
			g.writeIndent(out)
			out.WriteString("<xsd:complexContent>\n")
			g.indentUp()
			g.writeIndent(out)
			out.WriteString("<xsd:extension base=\"" + g.typeName(ttype) + "\">\n")
			g.indentUp()
			for _, a := range attrs.Members() {
				g.writeIndent(out)
				out.WriteString("<xsd:attribute name=\"" + a.Name() + "\" type=\"" + g.typeName(a.Type()) + "\" />\n")
			}
			g.indentDown()
			g.writeIndent(out)
			out.WriteString("</xsd:extension>\n")
			g.indentDown()
			g.writeIndent(out)
			out.WriteString("</xsd:complexContent>\n")
			g.indentDown()
			g.writeIndent(out)
			out.WriteString("</xsd:complexType>\n")
			g.indentDown()
			g.writeIndent(out)
			out.WriteString("</xsd:element>\n")
		}
	}
}

func (g *generator) generateService(tservice *sema.Service) {
	var xsdBuf strings.Builder
	fXsdName := g.outDir + tservice.Name() + ".xsd"

	ns := g.program.Namespace("xsd")
	annot := g.program.NamespaceAnnotations("xsd")
	if uri, ok := annot["uri"]; ok && len(uri) > 0 {
		ns = uri[len(uri)-1]
	}
	if len(ns) > 0 {
		ns = " targetNamespace=\"" + ns + "\" xmlns=\"" + ns + "\" " + "elementFormDefault=\"qualified\""
	}

	// Print the XSD header
	xsdBuf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\" ?>\n")
	xsdBuf.WriteString("<xsd:schema xmlns:xsd=\"http://www.w3.org/2001/XMLSchema\"" + ns + ">\n")
	xsdBuf.WriteString(g.xmlAutogenComment())
	xsdBuf.WriteString("\n")

	// Print out the type definitions
	g.writeIndent(&xsdBuf)
	xsdBuf.WriteString(g.sXsdTypes.String())

	// Keep a list of all the possible exceptions that might get thrown.
	// The C++ code stores this as t_struct*, via a cast that only ever
	// holds up because type_name and generate_element check is_typedef
	// before touching anything struct-specific; kept as sema.Type here,
	// which lets the same field type (typedef or struct) go through the
	// same generateElement/typeName calls with no assertion to panic.
	allXceptions := map[string]sema.Type{}

	// List the elements that you might actually get
	for _, f := range tservice.Functions() {
		elemname := f.Name() + "_response"
		returntype := f.ReturnType()
		g.generateElement(&xsdBuf, elemname, returntype, nil, false, false, false)
		xsdBuf.WriteString("\n")

		for _, x := range f.Xceptions().Members() {
			allXceptions[x.Name()] = x.Type()
		}
	}

	// map<string, t_struct*> iterates in sorted key order.
	names := make([]string, 0, len(allXceptions))
	for k := range allXceptions {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, name := range names {
		g.generateElement(&xsdBuf, name, allXceptions[name], nil, false, false, false)
	}

	// Close the XSD document
	xsdBuf.WriteString("\n</xsd:schema>\n")
	emit.WriteFile(fXsdName, xsdBuf.String())
}

func ns(in, namespace string) string { return namespace + ":" + in }

func xsd(in string) string { return ns(in, "xsd") }

// typeName is t_xsd_generator::type_name. It never resolves through
// TrueType: a typedef returns its own name, matching t_typedef not
// overriding get_name.
func (g *generator) typeName(ttype sema.Type) string {
	if ttype.IsTypedef() {
		return ttype.Name()
	}

	if ttype.IsBaseType() {
		return xsd(g.baseTypeName(ttype.(*sema.BaseType).Base()))
	}

	if ttype.IsEnum() {
		return xsd("int")
	}

	if ttype.IsStruct() || ttype.IsXception() {
		return ttype.Name()
	}

	return "container"
}

// baseTypeName is t_xsd_generator::base_type_name: the XSD type that
// corresponds to the thrift base type.
func (g *generator) baseTypeName(tbase sema.BaseKind) string {
	switch tbase {
	case sema.TypeVoid:
		return "void"
	case sema.TypeString:
		return "string"
	case sema.TypeBool:
		return "boolean"
	case sema.TypeI8:
		return "byte"
	case sema.TypeI16:
		return "short"
	case sema.TypeI32:
		return "int"
	case sema.TypeI64:
		return "long"
	case sema.TypeDouble:
		return "decimal"
	}
	emit.Throw("compiler error: no XSD base type name for base type %s", sema.BaseName(tbase))
	return ""
}

func (g *generator) xmlAutogenComment() string {
	return "<!--\n" +
		" * Autogenerated by Thrift Compiler (" + version.Version + ")\n" +
		" *\n" +
		" * DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING\n" +
		" -->\n"
}

func (g *generator) autogenComment() string {
	return "/**\n" +
		" * " + g.autogenSummary() + "\n" +
		" *\n" +
		" * DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING\n" +
		" *  @generated\n" +
		" */\n"
}

func (g *generator) autogenSummary() string {
	return "Autogenerated by Thrift Compiler (" + version.Version + ")"
}

func (g *generator) indent() string {
	return strings.Repeat("  ", g.indentLevel)
}

func (g *generator) writeIndent(out *strings.Builder) {
	out.WriteString(g.indent())
}

func (g *generator) indentUp() {
	g.indentLevel++
}

func (g *generator) indentDown() {
	g.indentLevel--
}
