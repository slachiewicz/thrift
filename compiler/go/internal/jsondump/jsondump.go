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

// Package jsondump renders a resolved program in the exact format of the
// C++ compiler's JSON generator (--gen json). It exists so that the Go
// front end can be compared with the C++ front end byte for byte, without
// involving any code generator.
package jsondump

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

type writer struct {
	sb          strings.Builder
	indentLevel int
	commaNeeded []bool
	program     *sema.Program
}

// Dump renders the program.
func Dump(p *sema.Program) string {
	w := &writer{program: p}
	w.generateProgram()
	return w.sb.String()
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

	w.writeKeyAnd("enums")
	w.startArray()
	for _, e := range p.Enums() {
		w.writeCommaIfNeeded()
		w.generateEnum(e)
		w.indicateCommaNeeded()
	}
	w.endArray()

	w.writeKeyAnd("typedefs")
	w.startArray()
	for _, t := range p.Typedefs() {
		w.writeCommaIfNeeded()
		w.generateTypedef(t)
		w.indicateCommaNeeded()
	}
	w.endArray()

	w.writeKeyAnd("structs")
	w.startArray()
	for _, s := range p.Objects() {
		w.writeCommaIfNeeded()
		w.generateStruct(s)
		w.indicateCommaNeeded()
	}
	w.endArray()

	w.writeKeyAnd("constants")
	w.startArray()
	for _, c := range p.Consts() {
		w.writeCommaIfNeeded()
		w.generateConstant(c)
		w.indicateCommaNeeded()
	}
	w.endArray()

	w.writeKeyAnd("services")
	w.startArray()
	for _, s := range p.Services() {
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
	if t.Program() == w.program {
		return t.Name()
	}
	return t.Program().Name() + "." + t.Name()
}
