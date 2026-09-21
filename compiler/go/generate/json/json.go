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

// Package json is t_json_generator.cc: it renders a resolved program as
// the JSON document the C++ compiler's --gen json writes, byte for byte.
// The front-end parity test also uses it, through Dump, to compare the
// Go front end with the C++ front end without any other generator.
package json

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// Options are the json:... generator options.
type Options struct {
	// Merge is "merge": the included programs' declarations are written
	// into the program's own sections and the namespaces and includes
	// sections are left out.
	Merge bool
}

// ParseOptions parses the part after "json:" of a --gen argument.
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
		default:
			return o, &emit.Error{Msg: "unknown option json:" + key}
		}
	}
	return o, nil
}

func init() {
	generate.Register(generate.Info{
		Name:     "json",
		LongName: "JSON",
		Options: []generate.Option{
			{Name: "merge", Help: "Generate output with included files merged"},
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
	outDir := program.OutPath() + "gen-json/"
	if program.IsOutPathAbsolute() {
		outDir = program.OutPath() + "/"
	}
	emit.Mkdir(outDir)
	emit.WriteFile(outDir+program.Name()+".json", Render(program, opts))
	return nil
}

type writer struct {
	sb          strings.Builder
	indentLevel int
	commaNeeded []bool
	program     *sema.Program
	merge       bool
}

// Dump renders the program without merging, as the front-end parity
// test compares it.
func Dump(p *sema.Program) string {
	return Render(p, Options{})
}

// Render renders the program as the JSON document, with a trailing
// newline like the file the C++ compiler writes.
func Render(p *sema.Program, opts Options) string {
	w := &writer{program: p, merge: opts.Merge}
	w.generateProgram()
	return w.sb.String()
}

// merged returns the program's declarations of one kind followed by
// those of its includes, depth first, as merge_includes appends them; an
// include reached twice is appended twice, as in C++.
func mergedEnums(p *sema.Program) []*sema.Enum {
	out := append([]*sema.Enum{}, p.Enums()...)
	for _, inc := range p.Includes() {
		out = append(out, mergedEnums(inc)...)
	}
	return out
}

func mergedTypedefs(p *sema.Program) []*sema.Typedef {
	out := append([]*sema.Typedef{}, p.Typedefs()...)
	for _, inc := range p.Includes() {
		out = append(out, mergedTypedefs(inc)...)
	}
	return out
}

func mergedObjects(p *sema.Program) []*sema.Struct {
	out := append([]*sema.Struct{}, p.Objects()...)
	for _, inc := range p.Includes() {
		out = append(out, mergedObjects(inc)...)
	}
	return out
}

func mergedConsts(p *sema.Program) []*sema.Const {
	out := append([]*sema.Const{}, p.Consts()...)
	for _, inc := range p.Includes() {
		out = append(out, mergedConsts(inc)...)
	}
	return out
}

func mergedServices(p *sema.Program) []*sema.Service {
	out := append([]*sema.Service{}, p.Services()...)
	for _, inc := range p.Includes() {
		out = append(out, mergedServices(inc)...)
	}
	return out
}

func (w *writer) indent() string {
	return strings.Repeat("  ", w.indentLevel)
}

func escapeJSONString(input string) string {
	var sb strings.Builder
	for i := 0; i < len(input); i++ {
		c := input[i]
		switch c {
		case '\\':
			sb.WriteString(`\\`)
		case '"':
			sb.WriteString(`\"`)
		case '/':
			sb.WriteString(`\/`)
		case '\b':
			sb.WriteString(`\b`)
		case '\f':
			sb.WriteString(`\f`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

func jsonStr(s string) string {
	return `"` + escapeJSONString(s) + `"`
}

// numberToString mimics an ostream with precision set to
// numeric_limits<T>::digits10, which for double is 15 significant digits
// in the default (general) float format.
func doubleToString(f float64) string {
	return strconv.FormatFloat(f, 'g', 15, 64)
}

func (w *writer) startObject(shouldIndent bool) {
	if shouldIndent {
		w.sb.WriteString(w.indent())
	}
	w.sb.WriteString("{\n")
	w.indentLevel++
	w.commaNeeded = append(w.commaNeeded, false)
}

func (w *writer) startArray() {
	w.sb.WriteString("[\n")
	w.indentLevel++
	w.commaNeeded = append(w.commaNeeded, false)
}

func (w *writer) writeCommaIfNeeded() {
	if w.commaNeeded[len(w.commaNeeded)-1] {
		w.sb.WriteString(",\n")
	}
}

func (w *writer) indicateCommaNeeded() {
	w.commaNeeded[len(w.commaNeeded)-1] = true
}

func (w *writer) writeKeyAnd(key string) {
	w.writeCommaIfNeeded()
	w.sb.WriteString(w.indent() + jsonStr(key) + ": ")
	w.indicateCommaNeeded()
}

func (w *writer) writeKeyAndInteger(key string, val int64) {
	w.writeCommaIfNeeded()
	w.sb.WriteString(w.indent() + jsonStr(key) + ": " + strconv.FormatInt(val, 10))
	w.indicateCommaNeeded()
}

func (w *writer) writeKeyAndString(key, val string) {
	w.writeCommaIfNeeded()
	w.sb.WriteString(w.indent() + jsonStr(key) + ": " + jsonStr(val))
	w.indicateCommaNeeded()
}

func (w *writer) writeKeyAndBool(key string, val bool) {
	w.writeCommaIfNeeded()
	s := "false"
	if val {
		s = "true"
	}
	w.sb.WriteString(w.indent() + jsonStr(key) + ": " + s)
	w.indicateCommaNeeded()
}

func (w *writer) endObject() {
	w.indentLevel--
	w.sb.WriteString("\n" + w.indent() + "}")
	w.commaNeeded = w.commaNeeded[:len(w.commaNeeded)-1]
}

func (w *writer) endArray() {
	w.indentLevel--
	if w.commaNeeded[len(w.commaNeeded)-1] {
		w.sb.WriteString("\n")
	}
	w.sb.WriteString(w.indent() + "]")
	w.commaNeeded = w.commaNeeded[:len(w.commaNeeded)-1]
}

func (w *writer) writeString(value string) {
	w.sb.WriteString(jsonStr(value))
}

func (w *writer) writeAnnotations(a sema.Annotations) {
	if len(a) == 0 {
		return
	}
	w.writeKeyAnd("annotations")
	w.startObject(true)
	for _, k := range a.Keys() {
		for _, v := range a[k] {
			w.writeKeyAndString(k, v)
		}
	}
	w.endObject()
}

func (w *writer) writeTypeSpecObject(name string, t sema.Type) {
	t = sema.TrueType(t)
	if t.IsStruct() || t.IsXception() || t.IsContainer() || t.IsEnum() {
		w.writeKeyAnd(name)
		w.startObject(false)
		w.writeKeyAnd("typeId")
		w.writeTypeSpec(t)
		w.endObject()
	}
}

func (w *writer) writeTypeSpec(t sema.Type) {
	t = sema.TrueType(t)
	w.writeString(w.typeName(t))
	w.writeAnnotations(t.Annotations())
	switch {
	case t.IsStruct() || t.IsXception() || t.IsEnum():
		w.writeKeyAndString("class", w.qualifiedName(t))
	case t.IsMap():
		m := t.(*sema.Map)
		w.writeKeyAndString("keyTypeId", w.typeName(m.KeyType()))
		w.writeKeyAndString("valueTypeId", w.typeName(m.ValType()))
		w.writeTypeSpecObject("keyType", m.KeyType())
		w.writeTypeSpecObject("valueType", m.ValType())
	case t.IsList():
		l := t.(*sema.List)
		w.writeKeyAndString("elemTypeId", w.typeName(l.ElemType()))
		w.writeTypeSpecObject("elemType", l.ElemType())
	case t.IsSet():
		s := t.(*sema.Set)
		w.writeKeyAndString("elemTypeId", w.typeName(s.ElemType()))
		w.writeTypeSpecObject("elemType", s.ElemType())
	}
}

func (w *writer) generateProgram() {
	p := w.program
	w.startObject(true)
	w.writeKeyAndString("name", p.Name())
	if p.HasDoc() {
		w.writeKeyAndString("doc", p.Doc())
	}

	// When merging includes, the namespaces and includes sections become
	// ambiguous, so they are left out.
	enums, typedefs, objects, consts, services := p.Enums(), p.Typedefs(), p.Objects(), p.Consts(), p.Services()
	if w.merge {
		enums, typedefs, objects, consts, services = mergedEnums(p), mergedTypedefs(p), mergedObjects(p), mergedConsts(p), mergedServices(p)
	} else {
		w.writeKeyAnd("namespaces")
		w.startObject(false)
		for _, k := range p.NamespaceKeys() {
			w.writeKeyAndString(k, p.Namespaces()[k])
			w.indicateCommaNeeded()
		}
		w.endObject()

		w.writeKeyAnd("includes")
		w.startArray()
		for _, inc := range p.Includes() {
			w.writeCommaIfNeeded()
			w.sb.WriteString(w.indent())
			w.writeString(inc.Name())
			w.indicateCommaNeeded()
		}
		w.endArray()
	}

	w.writeKeyAnd("enums")
	w.startArray()
	for _, e := range enums {
		w.writeCommaIfNeeded()
		w.generateEnum(e)
		w.indicateCommaNeeded()
	}
	w.endArray()

	w.writeKeyAnd("typedefs")
	w.startArray()
	for _, t := range typedefs {
		w.writeCommaIfNeeded()
		w.generateTypedef(t)
		w.indicateCommaNeeded()
	}
	w.endArray()

	w.writeKeyAnd("structs")
	w.startArray()
	for _, s := range objects {
		w.writeCommaIfNeeded()
		w.generateStruct(s)
		w.indicateCommaNeeded()
	}
	w.endArray()

	w.writeKeyAnd("constants")
	w.startArray()
	for _, c := range consts {
		w.writeCommaIfNeeded()
		w.generateConstant(c)
		w.indicateCommaNeeded()
	}
	w.endArray()

	w.writeKeyAnd("services")
	w.startArray()
	for _, s := range services {
		w.writeCommaIfNeeded()
		w.generateService(s)
		w.indicateCommaNeeded()
	}
	w.endArray()

	w.endObject()
	w.sb.WriteString("\n")
}

func (w *writer) generateTypedef(t *sema.Typedef) {
	w.startObject(true)
	w.writeKeyAndString("name", w.qualifiedName(t))
	w.writeKeyAndString("typeId", w.typeName(sema.TrueType(t)))
	w.writeTypeSpecObject("type", sema.TrueType(t))
	if t.HasDoc() {
		w.writeKeyAndString("doc", t.Doc())
	}
	w.writeAnnotations(t.Annotations())
	w.endObject()
}

func (w *writer) writeConstValue(v *sema.ConstValue, forceString bool) {
	switch v.Kind() {
	case sema.CVIdentifier, sema.CVInteger:
		s := strconv.FormatInt(v.Integer(), 10)
		if forceString {
			w.writeString(s)
		} else {
			w.sb.WriteString(s)
		}
	case sema.CVDouble:
		s := doubleToString(v.Double())
		if forceString {
			w.writeString(s)
		} else {
			w.sb.WriteString(s)
		}
	case sema.CVString:
		w.writeString(v.String())
	case sema.CVList:
		w.startArray()
		for _, e := range v.List() {
			w.writeCommaIfNeeded()
			w.sb.WriteString(w.indent())
			w.writeConstValue(e, false)
			w.indicateCommaNeeded()
		}
		w.endArray()
	case sema.CVMap:
		w.startObject(false)
		for _, e := range v.Map() {
			w.writeCommaIfNeeded()
			w.sb.WriteString(w.indent())
			w.writeConstValue(e.Key, true)
			w.sb.WriteString(": ")
			w.writeConstValue(e.Value, false)
			w.indicateCommaNeeded()
		}
		w.endObject()
	default:
		w.sb.WriteString("null")
	}
}

func (w *writer) generateConstant(c *sema.Const) {
	w.startObject(true)
	w.writeKeyAndString("name", c.Name())
	w.writeKeyAndString("typeId", w.typeName(c.Type()))
	w.writeTypeSpecObject("type", c.Type())
	if c.HasDoc() {
		w.writeKeyAndString("doc", c.Doc())
	}
	w.writeKeyAnd("value")
	w.writeConstValue(c.Value(), false)
	w.endObject()
}

func (w *writer) generateEnum(e *sema.Enum) {
	w.startObject(true)
	w.writeKeyAndString("name", e.Name())
	if e.HasDoc() {
		w.writeKeyAndString("doc", e.Doc())
	}
	w.writeAnnotations(e.Annotations())
	w.writeKeyAnd("members")
	w.startArray()
	for _, v := range e.Constants() {
		w.writeCommaIfNeeded()
		w.startObject(true)
		w.writeKeyAndString("name", v.Name())
		w.writeKeyAndInteger("value", int64(v.Value()))
		if v.HasDoc() {
			w.writeKeyAndString("doc", v.Doc())
		}
		w.endObject()
		w.indicateCommaNeeded()
	}
	w.endArray()
	w.endObject()
}

func (w *writer) generateStruct(s *sema.Struct) {
	w.startObject(true)
	w.writeKeyAndString("name", s.Name())
	if s.HasDoc() {
		w.writeKeyAndString("doc", s.Doc())
	}
	w.writeAnnotations(s.Annotations())
	w.writeKeyAndBool("isException", s.IsXception())
	w.writeKeyAndBool("isUnion", s.IsUnion())
	w.writeKeyAnd("fields")
	w.startArray()
	for _, f := range s.Members() {
		w.writeCommaIfNeeded()
		w.generateField(f)
		w.indicateCommaNeeded()
	}
	w.endArray()
	w.endObject()
}

func (w *writer) generateService(s *sema.Service) {
	w.startObject(true)
	w.writeKeyAndString("name", w.qualifiedName(s))
	if s.Extends() != nil {
		w.writeKeyAndString("extends", w.qualifiedName(s.Extends()))
	}
	if s.HasDoc() {
		w.writeKeyAndString("doc", s.Doc())
	}
	w.writeAnnotations(s.Annotations())
	w.writeKeyAnd("functions")
	w.startArray()
	for _, f := range s.Functions() {
		w.writeCommaIfNeeded()
		w.generateFunction(f)
		w.indicateCommaNeeded()
	}
	w.endArray()
	w.endObject()
}

func (w *writer) generateFunction(f *sema.Function) {
	w.startObject(true)
	w.writeKeyAndString("name", f.Name())
	w.writeKeyAndString("returnTypeId", w.typeName(f.ReturnType()))
	w.writeTypeSpecObject("returnType", f.ReturnType())
	w.writeKeyAndBool("oneway", f.IsOneway())
	if f.HasDoc() {
		w.writeKeyAndString("doc", f.Doc())
	}
	w.writeAnnotations(f.Annotations())
	w.writeKeyAnd("arguments")
	w.startArray()
	for _, a := range f.Arglist().Members() {
		w.writeCommaIfNeeded()
		w.generateField(a)
		w.indicateCommaNeeded()
	}
	w.endArray()
	w.writeKeyAnd("exceptions")
	w.startArray()
	for _, x := range f.Xceptions().Members() {
		w.writeCommaIfNeeded()
		w.generateField(x)
		w.indicateCommaNeeded()
	}
	w.endArray()
	w.endObject()
}

func (w *writer) generateField(f *sema.Field) {
	w.startObject(true)
	w.writeKeyAndInteger("key", int64(f.Key()))
	w.writeKeyAndString("name", f.Name())
	w.writeKeyAndString("typeId", w.typeName(f.Type()))
	w.writeTypeSpecObject("type", f.Type())
	if f.HasDoc() {
		w.writeKeyAndString("doc", f.Doc())
	}
	w.writeAnnotations(f.Annotations())
	w.writeKeyAnd("required")
	switch f.Req() {
	case sema.Required:
		w.writeString("required")
	case sema.OptInReqOut:
		w.writeString("req_out")
	default:
		w.writeString("optional")
	}
	if f.Value() != nil {
		w.writeKeyAnd("default")
		w.writeConstValue(f.Value(), false)
	}
	w.endObject()
}

func (w *writer) typeName(t sema.Type) string {
	t = sema.TrueType(t)
	switch {
	case t.IsList():
		return "list"
	case t.IsSet():
		return "set"
	case t.IsMap():
		return "map"
	case t.IsEnum():
		return "enum"
	case t.IsStruct():
		if t.(*sema.Struct).IsUnion() {
			return "union"
		}
		return "struct"
	case t.IsXception():
		return "exception"
	case t.IsBaseType():
		b := t.(*sema.BaseType)
		if b.IsBinary() {
			return "binary"
		}
		return sema.BaseName(b.Base())
	}
	return "(unknown)"
}

func (w *writer) qualifiedName(t sema.Type) string {
	if w.merge || t.Program() == w.program {
		return t.Name()
	}
	return t.Program().Name() + "." + t.Name()
}
