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

package decode

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
	"github.com/apache/thrift/lib/go/thrift"
)

// Schema-aware decoding annotates a raw tree with what the IDL says:
// field names, IDL type names, enum member names, union and exception
// kinds, and binary versus string. The wire is decoded first and the
// annotation never fails: a field the IDL does not know or whose wire
// type differs from the declaration is marked, not rejected.

// Annotation is what the IDL adds to a Value.
type Annotation struct {
	// Name is the field name, or the struct name on a struct value.
	Name string
	// TypeName is the IDL type as written, such as "MyEnum" or
	// "list<Foo>".
	TypeName string
	// Kind is "struct", "union" or "exception" on a struct value.
	Kind string
	// EnumName is the member name of an enum value, or empty when the
	// value is not a declared member.
	EnumName string
	// Unknown marks a field the IDL does not declare.
	Unknown bool
	// Mismatch names the declared type when the wire type differs.
	Mismatch string
	// Binary marks a string field the IDL declares as binary.
	Binary bool
}

// typeName renders an IDL type the way the IDL writes it.
func idlTypeName(t sema.Type) string {
	switch {
	case t == nil:
		return "?"
	case t.IsMap():
		m := t.(*sema.Map)
		return "map<" + idlTypeName(m.KeyType()) + ", " + idlTypeName(m.ValType()) + ">"
	case t.IsList():
		return "list<" + idlTypeName(t.(*sema.List).ElemType()) + ">"
	case t.IsSet():
		return "set<" + idlTypeName(t.(*sema.Set).ElemType()) + ">"
	}
	return t.Name()
}

// wireType is the TType a declared type is written as.
func wireType(t sema.Type) thrift.TType {
	t = sema.TrueType(t)
	switch {
	case t.IsEnum():
		return thrift.I32
	case t.IsStruct() || t.IsXception():
		return thrift.STRUCT
	case t.IsMap():
		return thrift.MAP
	case t.IsList():
		return thrift.LIST
	case t.IsSet():
		return thrift.SET
	case t.IsBaseType():
		switch t.(*sema.BaseType).Base() {
		case sema.TypeBool:
			return thrift.BOOL
		case sema.TypeI8:
			return thrift.BYTE
		case sema.TypeI16:
			return thrift.I16
		case sema.TypeI32:
			return thrift.I32
		case sema.TypeI64:
			return thrift.I64
		case sema.TypeDouble:
			return thrift.DOUBLE
		case sema.TypeString:
			return thrift.STRING
		case sema.TypeUUID:
			return thrift.UUID
		}
	}
	return thrift.STOP
}

// Annotate decorates v, decoded from the wire, with the declaration t.
func Annotate(v *Value, t sema.Type) {
	if v == nil || t == nil {
		return
	}
	v.Annotation.TypeName = idlTypeName(t)
	true_ := sema.TrueType(t)
	if want := wireType(t); want != thrift.STOP && want != v.Type {
		v.Annotation.Mismatch = idlTypeName(t)
		return
	}
	switch {
	case true_.IsEnum():
		if member := true_.(*sema.Enum).ConstantByValue(v.Int); member != nil {
			v.Annotation.EnumName = member.Name()
		}
	case true_.IsBaseType() && true_.IsBinary():
		v.Annotation.Binary = true
	case true_.IsStruct() || true_.IsXception():
		annotateStruct(v, true_.(*sema.Struct))
	case true_.IsMap():
		m := true_.(*sema.Map)
		for _, e := range v.Entries {
			Annotate(e.Key, m.KeyType())
			Annotate(e.Value, m.ValType())
		}
	case true_.IsList():
		for _, e := range v.Elems {
			Annotate(e, true_.(*sema.List).ElemType())
		}
	case true_.IsSet():
		for _, e := range v.Elems {
			Annotate(e, true_.(*sema.Set).ElemType())
		}
	}
}

func annotateStruct(v *Value, s *sema.Struct) {
	v.Annotation.Name = s.Name()
	switch {
	case s.IsUnion():
		v.Annotation.Kind = "union"
	case s.IsXception():
		v.Annotation.Kind = "exception"
	default:
		v.Annotation.Kind = "struct"
	}
	byKey := map[int16]*sema.Field{}
	for _, f := range s.Members() {
		byKey[int16(f.Key())] = f
	}
	for i := range v.Fields {
		f := &v.Fields[i]
		decl, ok := byKey[f.ID]
		if !ok {
			f.Value.Annotation.Unknown = true
			continue
		}
		f.Value.Annotation.Name = decl.Name()
		Annotate(f.Value, decl.Type())
		// A struct field's own name is the field's, not the type's.
		if f.Value.Type == thrift.STRUCT {
			f.Value.Annotation.Name = decl.Name()
		}
	}
}

// AnnotateMessage decorates a message with the service that defines its
// method: the arguments struct for a call, the result struct for a reply,
// and TApplicationException for an exception.
func AnnotateMessage(m *Message, service *sema.Service) error {
	if m.Type == thrift.EXCEPTION {
		annotateApplicationException(m.Body)
		return nil
	}
	fn := findFunction(service, m.Name)
	if fn == nil {
		return fmt.Errorf("service %s has no method %q", service.Name(), m.Name)
	}
	switch m.Type {
	case thrift.CALL, thrift.ONEWAY:
		annotateStruct(m.Body, fn.Arglist())
		m.Body.Annotation.Name = fn.Name() + "_args"
	case thrift.REPLY:
		annotateResult(m.Body, fn)
	}
	return nil
}

func findFunction(service *sema.Service, name string) *sema.Function {
	for s := service; s != nil; s = s.Extends() {
		for _, fn := range s.Functions() {
			if fn.Name() == name {
				return fn
			}
		}
	}
	return nil
}

// annotateResult handles the generated <method>_result struct: field 0
// is the return value and the rest are the declared exceptions.
func annotateResult(v *Value, fn *sema.Function) {
	v.Annotation.Name = fn.Name() + "_result"
	v.Annotation.Kind = "struct"
	byKey := map[int16]*sema.Field{}
	for _, f := range fn.Xceptions().Members() {
		byKey[int16(f.Key())] = f
	}
	for i := range v.Fields {
		f := &v.Fields[i]
		if f.ID == 0 && !fn.ReturnType().IsVoid() {
			f.Value.Annotation.Name = "success"
			Annotate(f.Value, fn.ReturnType())
			if f.Value.Type == thrift.STRUCT {
				f.Value.Annotation.Name = "success"
			}
			continue
		}
		decl, ok := byKey[f.ID]
		if !ok {
			f.Value.Annotation.Unknown = true
			continue
		}
		f.Value.Annotation.Name = decl.Name()
		Annotate(f.Value, decl.Type())
		if f.Value.Type == thrift.STRUCT {
			f.Value.Annotation.Name = decl.Name()
		}
	}
}

// applicationExceptionTypes are TApplicationException's type codes.
var applicationExceptionTypes = map[int64]string{
	0: "UNKNOWN", 1: "UNKNOWN_METHOD", 2: "INVALID_MESSAGE_TYPE_EXCEPTION", 3: "WRONG_METHOD_NAME",
	4: "BAD_SEQUENCE_ID", 5: "MISSING_RESULT", 6: "INTERNAL_ERROR", 7: "PROTOCOL_ERROR",
	8: "INVALID_TRANSFORM", 9: "INVALID_PROTOCOL", 10: "UNSUPPORTED_CLIENT_TYPE",
}

func annotateApplicationException(v *Value) {
	v.Annotation.Name = "TApplicationException"
	v.Annotation.Kind = "exception"
	for i := range v.Fields {
		f := &v.Fields[i]
		switch {
		case f.ID == 1 && f.Value.Type == thrift.STRING:
			f.Value.Annotation.Name = "message"
			f.Value.Annotation.TypeName = "string"
		case f.ID == 2 && f.Value.Type == thrift.I32:
			f.Value.Annotation.Name = "type"
			f.Value.Annotation.TypeName = "i32"
			f.Value.Annotation.EnumName = applicationExceptionTypes[f.Value.Int]
		default:
			f.Value.Annotation.Unknown = true
		}
	}
}

// Schema is the IDL side of a decode: the program and what to decode
// the input as.
type Schema struct {
	Program *sema.Program
	// Type names the struct a bare value is, as "Name" or "Included.Name".
	Type string
	// Service names the service whose method a message calls; empty
	// picks the program's only service.
	Service string
}

// Apply annotates every value of the result. It reports an IDL lookup
// that fails; a wire that does not match the IDL is marked in the tree.
func (s *Schema) Apply(r *Result) error {
	if s == nil || s.Program == nil {
		return nil
	}
	if len(r.Values) > 0 {
		if s.Type == "" {
			return fmt.Errorf("the input is a bare struct; name its type with --type")
		}
		t := s.Program.Scope.GetType(s.Type)
		if t == nil {
			return fmt.Errorf("type %q is not declared in %s", s.Type, s.Program.Name())
		}
		if tt := sema.TrueType(t); !tt.IsStruct() && !tt.IsXception() {
			return fmt.Errorf("type %q is a %s, not a struct", s.Type, idlTypeName(t))
		}
		for _, v := range r.Values {
			Annotate(v, t)
		}
	}
	if len(r.Messages) > 0 {
		service, err := s.service()
		if err != nil {
			return err
		}
		for _, m := range r.Messages {
			if err := AnnotateMessage(m, service); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Schema) service() (*sema.Service, error) {
	if s.Service != "" {
		if sv := s.Program.Scope.GetService(s.Service); sv != nil {
			return sv, nil
		}
		return nil, fmt.Errorf("service %q is not declared in %s", s.Service, s.Program.Name())
	}
	services := s.Program.Services()
	switch len(services) {
	case 1:
		return services[0], nil
	case 0:
		return nil, fmt.Errorf("%s declares no service; the input is a message", s.Program.Name())
	}
	var names []string
	for _, sv := range services {
		names = append(names, sv.Name())
	}
	return nil, fmt.Errorf("%s declares several services (%s); name one with --service", s.Program.Name(), strings.Join(names, ", "))
}

// binaryBytes returns the bytes of a string field the IDL declares
// binary: on the JSON protocol they arrive base64-encoded.
func binaryBytes(v *Value, json bool) []byte {
	if json {
		if b, err := base64.StdEncoding.DecodeString(string(v.Bytes)); err == nil {
			return b
		}
	}
	return v.Bytes
}
