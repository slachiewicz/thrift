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

package netstd

import (
	"fmt"
	"os"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateDeserializeField is generate_deserialize_field.
func (g *Generator) generateDeserializeField(out *strings.Builder, f *sema.Field, prefix string, isPropertyless bool) {
	t := trueType(f.Type())

	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE DESERIALIZE CODE FOR void TYPE: %s%s", prefix, f.Name())
	}

	name := prefix
	if !isPropertyless {
		name += g.propName(f, false)
	}

	switch {
	case t.IsStruct() || t.IsXception():
		g.generateDeserializeStruct(out, t.(*sema.Struct), name)
	case t.IsContainer():
		g.generateDeserializeContainer(out, t, name)
	case t.IsBaseType() || t.IsEnum():
		out.WriteString(g.indent() + name + " = ")

		if t.IsEnum() {
			out.WriteString("(" + g.tn(t) + ")")
		}

		out.WriteString("await iprot.")

		if t.IsBaseType() {
			switch baseOf(t) {
			case sema.TypeVoid:
				emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
			case sema.TypeString:
				if t.IsBinary() {
					out.WriteString("ReadBinaryAsync(" + cancellationTokenName + ");")
				} else {
					out.WriteString("ReadStringAsync(" + cancellationTokenName + ");")
				}
			case sema.TypeUUID:
				out.WriteString("ReadUuidAsync(" + cancellationTokenName + ");")
			case sema.TypeBool:
				out.WriteString("ReadBoolAsync(" + cancellationTokenName + ");")
			case sema.TypeI8:
				out.WriteString("ReadByteAsync(" + cancellationTokenName + ");")
			case sema.TypeI16:
				out.WriteString("ReadI16Async(" + cancellationTokenName + ");")
			case sema.TypeI32:
				out.WriteString("ReadI32Async(" + cancellationTokenName + ");")
			case sema.TypeI64:
				out.WriteString("ReadI64Async(" + cancellationTokenName + ");")
			case sema.TypeDouble:
				out.WriteString("ReadDoubleAsync(" + cancellationTokenName + ");")
			default:
				emit.Throw("compiler error: no C# name for base type %s", sema.BaseName(baseOf(t)))
			}
		} else if t.IsEnum() {
			out.WriteString("ReadI32Async(" + cancellationTokenName + ");")
		}
		out.WriteString("\n")
	default:
		fmt.Fprintf(os.Stdout, "DO NOT KNOW HOW TO DESERIALIZE FIELD '%s' TYPE '%s'\n", f.Name(), g.tn(t))
	}
}

// generateDeserializeStruct is generate_deserialize_struct.
func (g *Generator) generateDeserializeStruct(out *strings.Builder, s *sema.Struct, prefix string) {
	if g.isUnionEnabled() && s.IsUnion() {
		out.WriteString(g.indent() + prefix + " = await " + g.tn(s) + ".ReadAsync(iprot, " + cancellationTokenName + ");\n")
	} else {
		out.WriteString(g.indent() + prefix + " = new " + g.tn(s) + "();\n")
		out.WriteString(g.indent() + "await " + prefix + ".ReadAsync(iprot, " + cancellationTokenName + ");\n")
	}
}

// generateDeserializeContainer is generate_deserialize_container.
func (g *Generator) generateDeserializeContainer(out *strings.Builder, t sema.Type, prefix string) {
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	obj := ""
	switch {
	case t.IsMap():
		obj = g.tmp("_map")
	case t.IsSet():
		obj = g.tmp("_set")
	case t.IsList():
		obj = g.tmp("_list")
	}

	switch {
	case t.IsMap():
		out.WriteString(g.indent() + "var " + obj + " = await iprot.ReadMapBeginAsync(" + cancellationTokenName + ");\n")
	case t.IsSet():
		out.WriteString(g.indent() + "var " + obj + " = await iprot.ReadSetBeginAsync(" + cancellationTokenName + ");\n")
	case t.IsList():
		out.WriteString(g.indent() + "var " + obj + " = await iprot.ReadListBeginAsync(" + cancellationTokenName + ");\n")
	}

	if g.opts.TargetNetVersion < 5 && t.IsSet() {
		out.WriteString(g.indent() + prefix + " = new " + g.tn(t) + "();\n")
	} else {
		out.WriteString(g.indent() + prefix + " = new " + g.tn(t) + "(TProtocolUtil.PreallocSize(" + obj + ".Count));\n")
	}
	i := g.tmp("_i")
	out.WriteString(g.indent() + "for(int " + i + " = 0; " + i + " < " + obj + ".Count; ++" + i + ")\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	switch {
	case t.IsMap():
		g.generateDeserializeMapElement(out, t.(*sema.Map), prefix)
	case t.IsSet():
		g.generateDeserializeSetElement(out, t.(*sema.Set), prefix)
	case t.IsList():
		g.generateDeserializeListElement(out, t.(*sema.List), prefix)
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	switch {
	case t.IsMap():
		out.WriteString(g.indent() + "await iprot.ReadMapEndAsync(" + cancellationTokenName + ");\n")
	case t.IsSet():
		out.WriteString(g.indent() + "await iprot.ReadSetEndAsync(" + cancellationTokenName + ");\n")
	case t.IsList():
		out.WriteString(g.indent() + "await iprot.ReadListEndAsync(" + cancellationTokenName + ");\n")
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

// generateDeserializeMapElement is generate_deserialize_map_element.
func (g *Generator) generateDeserializeMapElement(out *strings.Builder, m *sema.Map, prefix string) {
	key := g.tmp("_key")
	val := g.tmp("_val")

	fkey := sema.NewField(m.KeyType(), key, 0)
	fval := sema.NewField(m.ValType(), val, 0)

	out.WriteString(g.indent() + g.declareField(fkey, false, false, "") + "\n")
	out.WriteString(g.indent() + g.declareField(fval, false, false, "") + "\n")

	g.generateDeserializeField(out, fkey, "", false)
	g.generateDeserializeField(out, fval, "", false)

	out.WriteString(g.indent() + prefix + "[" + key + "] = " + val + ";\n")
}

// generateDeserializeSetElement is generate_deserialize_set_element.
func (g *Generator) generateDeserializeSetElement(out *strings.Builder, s *sema.Set, prefix string) {
	elem := g.tmp("_elem")
	felem := sema.NewField(s.ElemType(), elem, 0)

	out.WriteString(g.indent() + g.declareField(felem, false, false, "") + "\n")

	g.generateDeserializeField(out, felem, "", false)

	out.WriteString(g.indent() + prefix + ".Add(" + elem + ");\n")
}

// generateDeserializeListElement is generate_deserialize_list_element.
func (g *Generator) generateDeserializeListElement(out *strings.Builder, l *sema.List, prefix string) {
	elem := g.tmp("_elem")
	felem := sema.NewField(l.ElemType(), elem, 0)

	out.WriteString(g.indent() + g.declareField(felem, false, false, "") + "\n")

	g.generateDeserializeField(out, felem, "", false)

	out.WriteString(g.indent() + prefix + ".Add(" + elem + ");\n")
}

// generateSerializeField is generate_serialize_field.
func (g *Generator) generateSerializeField(out *strings.Builder, f *sema.Field, prefix string, isPropertyless, allowNullable bool) {
	t := trueType(f.Type())

	name := prefix
	if !isPropertyless {
		name += g.propName(f, false)
	}
	nullableName := name
	if allowNullable {
		nullableName += g.nullableValueAccess(t)
	}

	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE SERIALIZE CODE FOR void TYPE: %s", name)
	}

	switch {
	case t.IsStruct() || t.IsXception():
		g.generateSerializeStruct(out, t.(*sema.Struct), name)
	case t.IsContainer():
		g.generateSerializeContainer(out, t, name)
	case t.IsBaseType() || t.IsEnum():
		out.WriteString(g.indent() + "await oprot.")

		if t.IsBaseType() {
			switch baseOf(t) {
			case sema.TypeVoid:
				emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
			case sema.TypeString:
				if t.IsBinary() {
					out.WriteString("WriteBinaryAsync(")
				} else {
					out.WriteString("WriteStringAsync(")
				}
				out.WriteString(name + ", " + cancellationTokenName + ");")
			case sema.TypeUUID:
				out.WriteString("WriteUuidAsync(" + nullableName + ", " + cancellationTokenName + ");")
			case sema.TypeBool:
				out.WriteString("WriteBoolAsync(" + nullableName + ", " + cancellationTokenName + ");")
			case sema.TypeI8:
				out.WriteString("WriteByteAsync(" + nullableName + ", " + cancellationTokenName + ");")
			case sema.TypeI16:
				out.WriteString("WriteI16Async(" + nullableName + ", " + cancellationTokenName + ");")
			case sema.TypeI32:
				out.WriteString("WriteI32Async(" + nullableName + ", " + cancellationTokenName + ");")
			case sema.TypeI64:
				out.WriteString("WriteI64Async(" + nullableName + ", " + cancellationTokenName + ");")
			case sema.TypeDouble:
				out.WriteString("WriteDoubleAsync(" + nullableName + ", " + cancellationTokenName + ");")
			default:
				emit.Throw("compiler error: no C# name for base type %s", sema.BaseName(baseOf(t)))
			}
		} else if t.IsEnum() {
			out.WriteString("WriteI32Async((int)" + name + ", " + cancellationTokenName + ");")
		}
		out.WriteString("\n")
	default:
		fmt.Fprintf(os.Stdout, "DO NOT KNOW HOW TO SERIALIZE '%s%s' TYPE '%s'\n", prefix, f.Name(), g.tn(t))
	}
}

// generateSerializeStruct is generate_serialize_struct.
func (g *Generator) generateSerializeStruct(out *strings.Builder, s *sema.Struct, prefix string) {
	_ = s
	out.WriteString(g.indent() + "await " + prefix + ".WriteAsync(oprot, " + cancellationTokenName + ");\n")
}

// generateSerializeContainer is generate_serialize_container.
func (g *Generator) generateSerializeContainer(out *strings.Builder, t sema.Type, prefix string) {
	switch {
	case t.IsMap():
		m := t.(*sema.Map)
		out.WriteString(g.indent() + "await oprot.WriteMapBeginAsync(new TMap(" + g.typeToEnum(m.KeyType()) +
			", " + g.typeToEnum(m.ValType()) + ", " + prefix + ".Count), " + cancellationTokenName + ");\n")
	case t.IsSet():
		s := t.(*sema.Set)
		out.WriteString(g.indent() + "await oprot.WriteSetBeginAsync(new TSet(" + g.typeToEnum(s.ElemType()) +
			", " + prefix + ".Count), " + cancellationTokenName + ");\n")
	case t.IsList():
		l := t.(*sema.List)
		out.WriteString(g.indent() + "await oprot.WriteListBeginAsync(new TList(" + g.typeToEnum(l.ElemType()) +
			", " + prefix + ".Count), " + cancellationTokenName + ");\n")
	}

	iter := g.tmp("_iter")
	switch {
	case t.IsMap():
		m := t.(*sema.Map)
		out.WriteString(g.indent() + "foreach (" + g.tn(m.KeyType()) + " " + iter + " in " + prefix + ".Keys)")
	case t.IsSet():
		s := t.(*sema.Set)
		out.WriteString(g.indent() + "foreach (" + g.tn(s.ElemType()) + " " + iter + " in " + prefix + ")")
	case t.IsList():
		l := t.(*sema.List)
		out.WriteString(g.indent() + "foreach (" + g.tn(l.ElemType()) + " " + iter + " in " + prefix + ")")
	}

	out.WriteString("\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	switch {
	case t.IsMap():
		g.generateSerializeMapElement(out, t.(*sema.Map), iter, prefix)
	case t.IsSet():
		g.generateSerializeSetElement(out, t.(*sema.Set), iter)
	case t.IsList():
		g.generateSerializeListElement(out, t.(*sema.List), iter)
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	switch {
	case t.IsMap():
		out.WriteString(g.indent() + "await oprot.WriteMapEndAsync(" + cancellationTokenName + ");\n")
	case t.IsSet():
		out.WriteString(g.indent() + "await oprot.WriteSetEndAsync(" + cancellationTokenName + ");\n")
	case t.IsList():
		out.WriteString(g.indent() + "await oprot.WriteListEndAsync(" + cancellationTokenName + ");\n")
	}
}

// generateSerializeMapElement is generate_serialize_map_element.
func (g *Generator) generateSerializeMapElement(out *strings.Builder, m *sema.Map, iter, mapExpr string) {
	kfield := sema.NewField(m.KeyType(), iter, 0)
	g.generateSerializeField(out, kfield, "", false, false)
	vfield := sema.NewField(m.ValType(), mapExpr+"["+iter+"]", 0)
	g.generateSerializeField(out, vfield, "", false, false)
}

// generateSerializeSetElement is generate_serialize_set_element.
func (g *Generator) generateSerializeSetElement(out *strings.Builder, s *sema.Set, iter string) {
	efield := sema.NewField(s.ElemType(), iter, 0)
	g.generateSerializeField(out, efield, "", false, false)
}

// generateSerializeListElement is generate_serialize_list_element.
func (g *Generator) generateSerializeListElement(out *strings.Builder, l *sema.List, iter string) {
	efield := sema.NewField(l.ElemType(), iter, 0)
	g.generateSerializeField(out, efield, "", false, false)
}
