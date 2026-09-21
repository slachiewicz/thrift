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

// Package xml is t_xml_generator.cc: it renders an XML model of the parsed
// IDL tree, designed to make it easy to use the file as the input for other
// template engines, such as XSLT. To that end the generated XML is slightly
// more verbose than one might expect - for example, references to "id"
// types (structs, unions, etc) always specify the name of the IDL document,
// even if the type is defined in the same document as the reference.
package xml

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

const defaultNSPrefix = "http://thrift.apache.org/xml/ns/"

// Options are the xml:... generator options.
type Options struct {
	// Merge is "merge": the included programs are written as nested
	// <document> elements, depth first, instead of each getting its own
	// output file.
	Merge bool
	// NoDefaultNS is "no_default_ns": the default xmlns is left out and
	// every element gets an idl: prefix instead.
	NoDefaultNS bool
	// NoNamespaces is "no_namespaces": no namespace definitions or
	// prefixes are added to the XML model at all.
	NoNamespaces bool
}

// ParseOptions parses the part after "xml:" of a --gen argument.
func ParseOptions(spec string) (Options, error) {
	var o Options
	for _, option := range strings.Split(spec, ",") {
		key := option
		if i := strings.IndexByte(option, '='); i >= 0 {
			key = option[:i]
		}
		switch key {
		case "":
		case "merge":
			o.Merge = true
		case "no_default_ns":
			o.NoDefaultNS = true
		case "no_namespaces":
			o.NoNamespaces = true
		default:
			return o, &emit.Error{Msg: "unknown option xml:" + key}
		}
	}
	return o, nil
}

func init() {
	generate.Register(generate.Info{
		Name:     "xml",
		LongName: "XML",
		Options: []generate.Option{
			{Name: "merge", Help: "Generate output with included files merged"},
			{Name: "no_default_ns", Help: "Omit default xmlns and add idl: prefix to all elements"},
			{Name: "no_namespaces", Help: "Do not add namespace definitions to the XML model"},
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
	outDir := program.OutPath() + "gen-xml/"
	if program.IsOutPathAbsolute() {
		outDir = program.OutPath() + "/"
	}
	emit.Mkdir(outDir)
	emit.WriteFile(outDir+program.Name()+".xml", Render(program, opts))
	return nil
}

// docHaver is t_doc: every declaration write_doc can be called on.
type docHaver interface {
	HasDoc() bool
	Doc() string
}

// xmlWriter is t_xml_generator's mutable generation state: f_xml_,
// elements_, top_element_is_empty and top_element_is_open, plus the
// programs_ set used while merging includes.
type xmlWriter struct {
	sb          strings.Builder
	indentLevel int

	elements          []string
	topElementIsEmpty bool
	topElementIsOpen  bool

	shouldMergeIncludes bool
	shouldUseDefaultNS  bool
	shouldUseNamespaces bool

	programs map[string]bool
}

// Render renders the program as the XML document the C++ compiler's
// --gen xml writes, byte for byte.
func Render(p *sema.Program, opts Options) string {
	w := &xmlWriter{
		shouldMergeIncludes: opts.Merge,
		shouldUseDefaultNS:  !opts.NoDefaultNS,
		shouldUseNamespaces: !opts.NoNamespaces,
		programs:            map[string]bool{},
	}
	w.generateProgram(p)
	return w.sb.String()
}

func (w *xmlWriter) indent() string {
	return strings.Repeat("  ", w.indentLevel)
}

func escapeXMLString(input string) string {
	var sb strings.Builder
	for i := 0; i < len(input); i++ {
		switch input[i] {
		case '&':
			sb.WriteString("&amp;")
		case '"':
			sb.WriteString("&quot;")
		case '\'':
			sb.WriteString("&apos;")
		case '<':
			sb.WriteString("&lt;")
		case '>':
			sb.WriteString("&gt;")
		default:
			sb.WriteByte(input[i])
		}
	}
	return sb.String()
}

// doubleToString mimics an ostream with precision set to
// numeric_limits<double>::digits10, which is 15 significant digits in the
// default (general) float format.
func doubleToString(f float64) string {
	return strconv.FormatFloat(f, 'g', 15, 64)
}

func (w *xmlWriter) writeXMLComment(msg string) {
	w.closeTopElement()
	// TODO: indent any EOLs that may occur with msg
	// TODO: proper msg escaping needed?
	w.sb.WriteString(w.indent() + "<!-- " + msg + " -->\n")
	w.topElementIsEmpty = false
}

func (w *xmlWriter) closeTopElement() {
	if w.topElementIsOpen {
		w.topElementIsOpen = false
		if len(w.elements) > 0 && w.topElementIsEmpty {
			w.sb.WriteString(">\n")
		}
	}
}

func (w *xmlWriter) qualifiedElementName(name string) string {
	if w.shouldUseNamespaces && !w.shouldUseDefaultNS {
		return "idl:" + name
	}
	return name
}

func (w *xmlWriter) writeElementStart(name string) {
	name = w.qualifiedElementName(name)
	w.closeTopElement()
	w.sb.WriteString(w.indent() + "<" + name)
	w.elements = append(w.elements, name)
	w.topElementIsEmpty = true
	w.topElementIsOpen = true
	w.indentLevel++
}

func (w *xmlWriter) writeElementEnd() {
	w.indentLevel--
	if w.topElementIsEmpty && w.topElementIsOpen {
		w.sb.WriteString(" />\n")
	} else {
		w.sb.WriteString(w.indent() + "</" + w.elements[len(w.elements)-1] + ">\n")
	}
	w.topElementIsEmpty = false
	w.elements = w.elements[:len(w.elements)-1]
}

func (w *xmlWriter) writeAttribute(key, val string) {
	w.sb.WriteString(" " + key + "=\"" + escapeXMLString(val) + "\"")
}

func (w *xmlWriter) writeIntAttribute(key string, val int32) {
	w.writeAttribute(key, strconv.FormatInt(int64(val), 10))
}

func (w *xmlWriter) writeElementString(name, val string) {
	name = w.qualifiedElementName(name)
	w.closeTopElement()
	w.topElementIsEmpty = false
	w.sb.WriteString(w.indent() + "<" + name + ">" + escapeXMLString(val) + "</" + name + ">\n")
}

func (w *xmlWriter) writeElementNumberInt(name string, n int64) {
	w.writeElementString(name, strconv.FormatInt(n, 10))
}

func (w *xmlWriter) writeElementNumberDouble(name string, n float64) {
	w.writeElementString(name, doubleToString(n))
}

// writeDoc is write_doc: for some reason there always seems to be a
// trailing newline on doc comments, so it strips a run of trailing '\n'
// characters. The strip only happens once a character other than '\n' is
// found scanning backwards; a doc that is nothing but newlines is left
// untouched, since the C++ loop then never reaches its erase.
func (w *xmlWriter) writeDoc(d docHaver) {
	if !d.HasDoc() {
		return
	}
	doc := d.Doc()
	n := 0
	for i := len(doc) - 1; i >= 0; i-- {
		if doc[i] != '\n' {
			if n > 0 {
				doc = doc[:len(doc)-n]
			}
			break
		}
		n++
	}
	w.writeAttribute("doc", doc)
}

func (w *xmlWriter) writeAnnotations(a sema.Annotations) {
	for _, k := range a.Keys() {
		for _, v := range a[k] {
			w.writeElementStart("annotation")
			w.writeAttribute("key", k)
			w.writeAttribute("value", v)
			w.writeElementEnd()
		}
	}
}

// getTypeName is get_type_name.
func getTypeName(t sema.Type) string {
	if t.IsList() {
		return "list"
	}
	if t.IsSet() {
		return "set"
	}
	if t.IsMap() {
		return "map"
	}
	if t.IsEnum() || t.IsStruct() || t.IsTypedef() || t.IsXception() {
		return "id"
	}
	if t.IsBaseType() {
		b := t.(*sema.BaseType)
		if b.IsBinary() {
			return "binary"
		}
		return sema.BaseName(b.Base())
	}
	return "(unknown)"
}

// writeType is write_type. It is deliberately given the declared type, not
// its true type: a field or const referencing a typedef writes the
// typedef's own name as the "id", the way generate_typedef itself resolves
// only when it renders the alias.
func (w *xmlWriter) writeType(t sema.Type) {
	typeName := getTypeName(t)
	w.writeAttribute("type", typeName)
	switch typeName {
	case "id":
		w.writeAttribute("type-module", t.Program().Name())
		w.writeAttribute("type-id", t.Name())
	case "list":
		etype := t.(*sema.List).ElemType()
		w.writeElementStart("elemType")
		w.writeType(etype)
		w.writeElementEnd()
	case "set":
		etype := t.(*sema.Set).ElemType()
		w.writeElementStart("elemType")
		w.writeType(etype)
		w.writeElementEnd()
	case "map":
		ktype := t.(*sema.Map).KeyType()
		w.writeElementStart("keyType")
		w.writeType(ktype)
		w.writeElementEnd()
		vtype := t.(*sema.Map).ValType()
		w.writeElementStart("valueType")
		w.writeType(vtype)
		w.writeElementEnd()
	}
}

// targetNamespace is target_namespace.
func targetNamespace(p *sema.Program) string {
	if a := p.NamespaceAnnotations("xml"); a != nil {
		if vals, ok := a["targetNamespace"]; ok && len(vals) > 0 {
			return vals[len(vals)-1]
		}
	}
	if ns, ok := p.Namespaces()["xml"]; ok {
		return defaultNSPrefix + ns
	}
	if a := p.NamespaceAnnotations("*"); a != nil {
		if vals, ok := a["xml.targetNamespace"]; ok && len(vals) > 0 {
			return vals[len(vals)-1]
		}
	}
	if ns, ok := p.Namespaces()["*"]; ok {
		return defaultNSPrefix + ns
	}
	return defaultNSPrefix + p.Name()
}

func (w *xmlWriter) writeConstValue(value *sema.ConstValue) {
	switch value.Kind() {
	case sema.CVIdentifier, sema.CVInteger:
		w.writeElementNumberInt("int", value.Integer())

	case sema.CVDouble:
		w.writeElementNumberDouble("double", value.Double())

	case sema.CVString:
		w.writeElementString("string", value.String())

	case sema.CVList:
		w.writeElementStart("list")
		for _, e := range value.List() {
			w.writeElementStart("entry")
			w.writeConstValue(e)
			w.writeElementEnd()
		}
		w.writeElementEnd()

	case sema.CVMap:
		w.writeElementStart("map")
		for _, e := range value.Map() {
			w.writeElementStart("entry")
			w.writeElementStart("key")
			w.writeConstValue(e.Key)
			w.writeElementEnd()
			w.writeElementStart("value")
			w.writeConstValue(e.Value)
			w.writeElementEnd()
			w.writeElementEnd()
		}
		w.writeElementEnd()

	default:
		w.indentLevel++
		w.sb.WriteString(w.indent() + "<null />\n")
		w.indentLevel--
	}
}

func (w *xmlWriter) generateConstant(con *sema.Const) {
	w.writeElementStart("const")
	w.writeAttribute("name", con.Name())
	w.writeDoc(con)
	w.writeType(con.Type())
	w.writeConstValue(con.Value())
	w.writeElementEnd()
}

func (w *xmlWriter) generateTypedef(ttypedef *sema.Typedef) {
	w.writeElementStart("typedef")
	w.writeAttribute("name", ttypedef.Name())
	w.writeDoc(ttypedef)
	w.writeType(sema.TrueType(ttypedef))
	w.writeAnnotations(ttypedef.Annotations())
	w.writeElementEnd()
}

func (w *xmlWriter) generateEnum(tenum *sema.Enum) {
	w.writeElementStart("enum")
	w.writeAttribute("name", tenum.Name())
	w.writeDoc(tenum)

	for _, val := range tenum.Constants() {
		w.writeElementStart("member")
		w.writeAttribute("name", val.Name())
		w.writeIntAttribute("value", val.Value())
		w.writeDoc(val)
		w.writeAnnotations(val.Annotations())
		w.writeElementEnd()
	}

	w.writeAnnotations(tenum.Annotations())

	w.writeElementEnd()
}

func (w *xmlWriter) generateField(field *sema.Field) {
	w.writeAttribute("name", field.Name())
	w.writeIntAttribute("field-id", field.Key())
	w.writeDoc(field)
	requiredness := ""
	switch field.Req() {
	case sema.Required:
		requiredness = "required"
	case sema.Optional:
		requiredness = "optional"
	}
	if requiredness != "" {
		w.writeAttribute("required", requiredness)
	}
	w.writeType(field.Type())
	if field.Value() != nil {
		w.writeElementStart("default")
		w.writeConstValue(field.Value())
		w.writeElementEnd()
	}
	w.writeAnnotations(field.Annotations())
}

func (w *xmlWriter) generateStruct(tstruct *sema.Struct) {
	tagname := "struct"
	if tstruct.IsUnion() {
		tagname = "union"
	} else if tstruct.IsXception() {
		tagname = "exception"
	}

	w.writeElementStart(tagname)
	w.writeAttribute("name", tstruct.Name())
	w.writeDoc(tstruct)
	for _, member := range tstruct.Members() {
		w.writeElementStart("field")
		w.generateField(member)
		w.writeElementEnd()
	}

	w.writeAnnotations(tstruct.Annotations())

	w.writeElementEnd()
}

func (w *xmlWriter) generateFunction(tfunc *sema.Function) {
	w.writeElementStart("method")

	w.writeAttribute("name", tfunc.Name())
	if tfunc.IsOneway() {
		w.writeAttribute("oneway", "true")
	}

	w.writeDoc(tfunc)

	w.writeElementStart("returns")
	w.writeType(tfunc.ReturnType())
	w.writeElementEnd()

	for _, member := range tfunc.Arglist().Members() {
		w.writeElementStart("arg")
		w.generateField(member)
		w.writeElementEnd()
	}

	for _, except := range tfunc.Xceptions().Members() {
		w.writeElementStart("throws")
		w.generateField(except)
		w.writeElementEnd()
	}

	w.writeAnnotations(tfunc.Annotations())

	w.writeElementEnd()
}

func (w *xmlWriter) generateService(tservice *sema.Service) {
	w.writeElementStart("service")
	w.writeAttribute("name", tservice.Name())

	if w.shouldUseNamespaces {
		progNS := targetNamespace(tservice.Program())
		if !strings.HasSuffix(progNS, "/") {
			progNS += "/"
		}
		tns := progNS + tservice.Name()
		w.writeAttribute("targetNamespace", tns)
		w.writeAttribute("xmlns:tns", tns)
	}

	if tservice.Extends() != nil {
		extends := tservice.Extends()
		w.writeAttribute("parent-module", extends.Program().Name())
		w.writeAttribute("parent-id", extends.Name())
	}

	w.writeDoc(tservice)

	for _, fn := range tservice.Functions() {
		w.generateFunction(fn)
	}

	w.writeAnnotations(tservice.Annotations())

	w.writeElementEnd()
}

func (w *xmlWriter) iterateProgram(program *sema.Program) {
	w.writeElementStart("document")
	w.writeAttribute("name", program.Name())
	if w.shouldUseNamespaces {
		targetNS := targetNamespace(program)
		w.writeAttribute("targetNamespace", targetNS)
		w.writeAttribute("xmlns:"+program.Name(), targetNS)
	}
	w.writeDoc(program)

	for _, inc := range program.Includes() {
		w.writeElementStart("include")
		w.writeAttribute("name", inc.Name())
		w.writeElementEnd()
	}

	for _, k := range program.NamespaceKeys() {
		w.writeElementStart("namespace")
		w.writeAttribute("name", k)
		w.writeAttribute("value", program.Namespaces()[k])
		w.writeAnnotations(program.NamespaceAnnotations(k))
		w.writeElementEnd()
	}

	// TODO: can constants have annotations?
	for _, con := range program.Consts() {
		w.generateConstant(con)
	}

	for _, ttypedef := range program.Typedefs() {
		w.generateTypedef(ttypedef)
	}

	for _, tenum := range program.Enums() {
		w.generateEnum(tenum)
	}

	for _, obj := range program.Objects() {
		if obj.IsXception() {
			// generate_xception has no override in t_xml_generator; the
			// base class's default implementation forwards to
			// generate_struct, so exceptions render the same way as
			// structs.
			w.generateStruct(obj)
		} else {
			w.generateStruct(obj)
		}
	}

	for _, tservice := range program.Services() {
		w.generateService(tservice)
	}

	w.writeElementEnd()

	if w.shouldMergeIncludes {
		w.programs[program.Name()] = true
		for _, prog := range program.Includes() {
			if !w.programs[prog.Name()] {
				w.iterateProgram(prog)
			}
		}
	}
}

// xmlAutogenComment is xml_autogen_comment.
func xmlAutogenComment() string {
	return "\n" + " * Autogenerated by Thrift Compiler (" + version.Version + ")\n" +
		" *\n" + " * DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING\n"
}

func (w *xmlWriter) generateProgram(program *sema.Program) {
	w.writeElementStart("idl")
	if w.shouldUseNamespaces {
		if w.shouldUseDefaultNS {
			w.writeAttribute("xmlns", "http://thrift.apache.org/xml/idl")
		}
		w.writeAttribute("xmlns:idl", "http://thrift.apache.org/xml/idl")
	}

	w.writeXMLComment(xmlAutogenComment())

	w.iterateProgram(program)

	w.writeElementEnd()
}
