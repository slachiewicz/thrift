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

package delphi

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateDeserializeField is t_delphi_generator::generate_deserialize_field.
func (g *Generator) generateDeserializeField(out *strings.Builder, isXception bool, tfield *sema.Field, prefix string, localVars *strings.Builder) {
	t := sema.TrueType(tfield.Type())
	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE DESERIALIZE CODE FOR void TYPE: %s%s", prefix, tfield.Name())
	}

	name := prefix + g.propNameF(tfield, isXception, "")

	if t.IsStruct() || t.IsXception() {
		g.generateDeserializeStruct(out, t.(*sema.Struct), name, "")
	} else if t.IsContainer() {
		g.generateDeserializeContainer(out, isXception, t, name, localVars)
	} else if t.IsBaseType() || t.IsEnum() {
		g.wrI(out, name+" := ")
		if t.IsEnum() {
			g.raw(out, g.typeName(t, false, false)+"(")
		}
		g.raw(out, "iprot.")
		if t.IsBaseType() {
			switch t.(*sema.BaseType).Base() {
			case sema.TypeVoid:
				emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
			case sema.TypeString:
				if t.IsBinary() {
					if g.opts.ComTypes {
						g.raw(out, "ReadBinaryCOM();")
					} else {
						g.raw(out, "ReadBinary();")
					}
				} else {
					g.raw(out, "ReadString();")
				}
			case sema.TypeUUID:
				g.raw(out, "ReadUuid();")
			case sema.TypeBool:
				g.raw(out, "ReadBool();")
			case sema.TypeI8:
				g.raw(out, "ReadByte();")
			case sema.TypeI16:
				g.raw(out, "ReadI16();")
			case sema.TypeI32:
				g.raw(out, "ReadI32();")
			case sema.TypeI64:
				g.raw(out, "ReadI64();")
			case sema.TypeDouble:
				g.raw(out, "ReadDouble();")
			default:
				emit.Throw("compiler error: no Delphi name for base type %s", sema.BaseName(t.(*sema.BaseType).Base()))
			}
		} else if t.IsEnum() {
			g.raw(out, "ReadI32()")
			g.raw(out, ");")
		}
		g.raw(out, "\n")
	}
	// else: the C++ source prints a stderr diagnostic here and emits
	// nothing to the generated file; every reachable type is covered
	// above, so this branch is unreachable.
}

// generateDeserializeStruct is t_delphi_generator::generate_deserialize_struct.
func (g *Generator) generateDeserializeStruct(out *strings.Builder, tstruct *sema.Struct, name, prefix string) {
	typName := g.typeName(tstruct, true, false)
	g.lnI(out, prefix+name+" := "+typName+".Create;")
	g.lnI(out, prefix+name+".Read(iprot);")
}

// generateDeserializeContainer is t_delphi_generator::generate_deserialize_container.
func (g *Generator) generateDeserializeContainer(out *strings.Builder, isXception bool, ttype sema.Type, name string, localVars *strings.Builder) {
	var obj string
	switch {
	case ttype.IsMap():
		obj = g.tmp("_map")
	case ttype.IsSet():
		obj = g.tmp("_set")
	case ttype.IsList():
		obj = g.tmp("_list")
	}

	switch {
	case ttype.IsMap():
		localVars.WriteString("  " + obj + ": TThriftMap;\n")
	case ttype.IsSet():
		localVars.WriteString("  " + obj + ": TThriftSet;\n")
	case ttype.IsList():
		localVars.WriteString("  " + obj + ": TThriftList;\n")
	}
	counter := g.tmp("_i")
	localVars.WriteString("  " + counter + ": System.Integer;\n")

	g.lnI(out, name+" := "+g.typeName(ttype, true, false)+".Create;")

	switch {
	case ttype.IsMap():
		g.lnI(out, obj+" := iprot.ReadMapBegin();")
	case ttype.IsSet():
		g.lnI(out, obj+" := iprot.ReadSetBegin();")
	case ttype.IsList():
		g.lnI(out, obj+" := iprot.ReadListBegin();")
	}

	g.lnI(out, "for "+counter+" := 0 to "+obj+".Count - 1 do begin")
	g.indentUpImpl()
	switch {
	case ttype.IsMap():
		g.generateDeserializeMapElement(out, isXception, ttype.(*sema.Map), name, localVars)
	case ttype.IsSet():
		g.generateDeserializeSetElement(out, isXception, ttype.(*sema.Set), name, localVars)
	case ttype.IsList():
		g.generateDeserializeListElement(out, isXception, ttype.(*sema.List), name, localVars)
	}
	g.indentDownImpl()
	g.lnI(out, "end;")

	switch {
	case ttype.IsMap():
		g.lnI(out, "iprot.ReadMapEnd();")
	case ttype.IsSet():
		g.lnI(out, "iprot.ReadSetEnd();")
	case ttype.IsList():
		g.lnI(out, "iprot.ReadListEnd();")
	}
}

// generateDeserializeMapElement is t_delphi_generator::generate_deserialize_map_element.
func (g *Generator) generateDeserializeMapElement(out *strings.Builder, isXception bool, tmap *sema.Map, prefix string, localVars *strings.Builder) {
	key := g.tmp("_key")
	val := g.tmp("_val")

	fkey := sema.NewField(tmap.KeyType(), key, 0)
	fval := sema.NewField(tmap.ValType(), val, 0)

	localVars.WriteString("  " + g.declareField(fkey, "", false) + "\n")
	localVars.WriteString("  " + g.declareField(fval, "", false) + "\n")

	g.generateDeserializeField(out, isXception, fkey, "", localVars)
	g.generateDeserializeField(out, isXception, fval, "", localVars)

	g.lnI(out, prefix+".AddOrSetValue( "+key+", "+val+");")
}

// generateDeserializeSetElement is t_delphi_generator::generate_deserialize_set_element.
func (g *Generator) generateDeserializeSetElement(out *strings.Builder, isXception bool, tset *sema.Set, prefix string, localVars *strings.Builder) {
	elem := g.tmp("_elem")
	felem := sema.NewField(tset.ElemType(), elem, 0)
	localVars.WriteString("  " + g.declareField(felem, "", false) + "\n")
	g.generateDeserializeField(out, isXception, felem, "", localVars)
	g.lnI(out, prefix+".Add("+elem+");")
}

// generateDeserializeListElement is t_delphi_generator::generate_deserialize_list_element.
func (g *Generator) generateDeserializeListElement(out *strings.Builder, isXception bool, tlist *sema.List, prefix string, localVars *strings.Builder) {
	elem := g.tmp("_elem")
	felem := sema.NewField(tlist.ElemType(), elem, 0)
	localVars.WriteString("  " + g.declareField(felem, "", false) + "\n")
	g.generateDeserializeField(out, isXception, felem, "", localVars)
	g.lnI(out, prefix+".Add("+elem+");")
}

// generateSerializeField is t_delphi_generator::generate_serialize_field.
func (g *Generator) generateSerializeField(out *strings.Builder, isXception bool, tfield *sema.Field, prefix string, localVars *strings.Builder) {
	t := sema.TrueType(tfield.Type())
	name := prefix + g.propNameF(tfield, isXception, "")

	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE SERIALIZE CODE FOR void TYPE: %s", name)
	}

	if t.IsStruct() || t.IsXception() {
		g.generateSerializeStruct(out, name, localVars)
	} else if t.IsContainer() {
		g.generateSerializeContainer(out, isXception, t, name, localVars)
	} else if t.IsBaseType() || t.IsEnum() {
		g.wrI(out, "oprot.")
		if t.IsBaseType() {
			switch t.(*sema.BaseType).Base() {
			case sema.TypeVoid:
				emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
			case sema.TypeString:
				if t.IsBinary() {
					g.raw(out, "WriteBinary(")
				} else {
					g.raw(out, "WriteString(")
				}
				g.raw(out, name+");")
			case sema.TypeUUID:
				g.raw(out, "WriteUuid("+name+");")
			case sema.TypeBool:
				g.raw(out, "WriteBool("+name+");")
			case sema.TypeI8:
				g.raw(out, "WriteByte("+name+");")
			case sema.TypeI16:
				g.raw(out, "WriteI16("+name+");")
			case sema.TypeI32:
				g.raw(out, "WriteI32("+name+");")
			case sema.TypeI64:
				g.raw(out, "WriteI64("+name+");")
			case sema.TypeDouble:
				g.raw(out, "WriteDouble("+name+");")
			default:
				emit.Throw("compiler error: no Delphi name for base type %s", sema.BaseName(t.(*sema.BaseType).Base()))
			}
		} else if t.IsEnum() {
			g.raw(out, "WriteI32(System.Integer("+name+"));")
		}
		g.raw(out, "\n")
	}
}

// generateSerializeStruct is t_delphi_generator::generate_serialize_struct.
func (g *Generator) generateSerializeStruct(out *strings.Builder, prefix string, localVars *strings.Builder) {
	_ = localVars
	g.lnI(out, prefix+".Write(oprot);")
}

// generateSerializeContainer is t_delphi_generator::generate_serialize_container.
func (g *Generator) generateSerializeContainer(out *strings.Builder, isXception bool, ttype sema.Type, prefix string, localVars *strings.Builder) {
	var obj string
	switch {
	case ttype.IsMap():
		m := ttype.(*sema.Map)
		obj = g.tmp("map")
		localVars.WriteString("  " + obj + " : TThriftMap;\n")
		g.lnI(out, "Thrift.Protocol.Init( "+obj+", "+g.typeToEnum(m.KeyType())+", "+g.typeToEnum(m.ValType())+", "+prefix+".Count);")
		g.lnI(out, "oprot.WriteMapBegin( "+obj+");")
	case ttype.IsSet():
		s := ttype.(*sema.Set)
		obj = g.tmp("set_")
		localVars.WriteString("  " + obj + " : TThriftSet;\n")
		g.lnI(out, "Thrift.Protocol.Init( "+obj+", "+g.typeToEnum(s.ElemType())+", "+prefix+".Count);")
		g.lnI(out, "oprot.WriteSetBegin( "+obj+");")
	case ttype.IsList():
		l := ttype.(*sema.List)
		obj = g.tmp("list_")
		localVars.WriteString("  " + obj + " : TThriftList;\n")
		g.lnI(out, "Thrift.Protocol.Init( "+obj+", "+g.typeToEnum(l.ElemType())+", "+prefix+".Count);")
		g.lnI(out, "oprot.WriteListBegin( "+obj+");")
	}

	iter := g.tmp("_iter")
	switch {
	case ttype.IsMap():
		m := ttype.(*sema.Map)
		localVars.WriteString("  " + iter + ": " + g.typeName(m.KeyType(), false, false) + ";\n")
		g.lnI(out, "for "+iter+" in "+prefix+".Keys do begin")
		g.indentUpImpl()
	case ttype.IsSet():
		s := ttype.(*sema.Set)
		localVars.WriteString("  " + iter + ": " + g.typeName(s.ElemType(), false, false) + ";\n")
		g.lnI(out, "for "+iter+" in "+prefix+" do begin")
		g.indentUpImpl()
	case ttype.IsList():
		l := ttype.(*sema.List)
		localVars.WriteString("  " + iter + ": " + g.typeName(l.ElemType(), false, false) + ";\n")
		g.lnI(out, "for "+iter+" in "+prefix+" do begin")
		g.indentUpImpl()
	}

	switch {
	case ttype.IsMap():
		g.generateSerializeMapElement(out, isXception, ttype.(*sema.Map), iter, prefix, localVars)
	case ttype.IsSet():
		g.generateSerializeSetElement(out, isXception, ttype.(*sema.Set), iter, localVars)
	case ttype.IsList():
		g.generateSerializeListElement(out, isXception, ttype.(*sema.List), iter, localVars)
	}

	g.indentDownImpl()
	g.lnI(out, "end;")

	switch {
	case ttype.IsMap():
		g.lnI(out, "oprot.WriteMapEnd();")
	case ttype.IsSet():
		g.lnI(out, "oprot.WriteSetEnd();")
	case ttype.IsList():
		g.lnI(out, "oprot.WriteListEnd();")
	}
}

// generateSerializeMapElement is t_delphi_generator::generate_serialize_map_element.
func (g *Generator) generateSerializeMapElement(out *strings.Builder, isXception bool, tmap *sema.Map, iter, mapName string, localVars *strings.Builder) {
	kfield := sema.NewField(tmap.KeyType(), iter, 0)
	g.generateSerializeField(out, isXception, kfield, "", localVars)
	vfield := sema.NewField(tmap.ValType(), mapName+"["+iter+"]", 0)
	g.generateSerializeField(out, isXception, vfield, "", localVars)
}

// generateSerializeSetElement is t_delphi_generator::generate_serialize_set_element.
func (g *Generator) generateSerializeSetElement(out *strings.Builder, isXception bool, tset *sema.Set, iter string, localVars *strings.Builder) {
	efield := sema.NewField(tset.ElemType(), iter, 0)
	g.generateSerializeField(out, isXception, efield, "", localVars)
}

// generateSerializeListElement is t_delphi_generator::generate_serialize_list_element.
func (g *Generator) generateSerializeListElement(out *strings.Builder, isXception bool, tlist *sema.List, iter string, localVars *strings.Builder) {
	efield := sema.NewField(tlist.ElemType(), iter, 0)
	g.generateSerializeField(out, isXception, efield, "", localVars)
}
