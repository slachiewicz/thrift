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

package cglib

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateSerializeField serializes a field of any type.
func (g *Generator) generateSerializeField(out *strings.Builder, tfield *sema.Field, prefix, suffix, errorRet string) {
	t := sema.TrueType(tfield.Type())
	name := prefix + tfield.Name() + suffix

	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE SERIALIZE CODE FOR void TYPE: %s", name)
	}

	if t.IsStruct() || t.IsXception() {
		g.generateSerializeStruct(out, name, errorRet)
	} else if t.IsContainer() {
		g.generateSerializeContainer(out, t, name, errorRet)
	} else if t.IsBaseType() || t.IsEnum() {
		out.WriteString(g.indent() + "if ((ret = thrift_protocol_write_")

		if t.IsBaseType() {
			bt := t.(*sema.BaseType)
			switch bt.Base() {
			case sema.TypeVoid:
				emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
			case sema.TypeBool:
				out.WriteString("bool (protocol, " + name)
			case sema.TypeI8:
				out.WriteString("byte (protocol, " + name)
			case sema.TypeI16:
				out.WriteString("i16 (protocol, " + name)
			case sema.TypeI32:
				out.WriteString("i32 (protocol, " + name)
			case sema.TypeI64:
				out.WriteString("i64 (protocol, " + name)
			case sema.TypeDouble:
				out.WriteString("double (protocol, " + name)
			case sema.TypeString:
				if bt.IsBinary() {
					out.WriteString("binary (protocol, " + name + " ? ((GByteArray *) " + name +
						")->data : NULL, " + name + " ? ((GByteArray *) " + name + ")->len : 0")
				} else {
					out.WriteString("string (protocol, " + name)
				}
			default:
				emit.Throw("compiler error: no C writer for base type %s%s", sema.BaseName(bt.Base()), name)
			}
		} else {
			out.WriteString("i32 (protocol, (gint32) " + name)
		}
		out.WriteString(", error)) < 0)\n" +
			g.indent() + "  return " + errorRet + ";\n" +
			g.indent() + "xfer += ret;\n\n")
	} else {
		emit.Throw("DO NOT KNOW HOW TO SERIALIZE FIELD '%s' TYPE '%s", name, g.typeName(t, false, false))
	}
}

// generateSerializeStruct is generate_serialize_struct.
func (g *Generator) generateSerializeStruct(out *strings.Builder, prefix, errorRet string) {
	out.WriteString(g.indent() + "if ((ret = thrift_struct_write (THRIFT_STRUCT (" + prefix +
		"), protocol, error)) < 0)\n" + g.indent() + "  return " + errorRet + ";\n" +
		g.indent() + "xfer += ret;\n\n")
}

// generateSerializeContainer is generate_serialize_container.
func (g *Generator) generateSerializeContainer(out *strings.Builder, ttype sema.Type, prefix, errorRet string) {
	g.scopeUp(out)

	if ttype.IsMap() {
		m := ttype.(*sema.Map)
		tkey, tval := m.KeyType(), m.ValType()
		tkeyName := g.typeName(tkey, false, false)
		tvalName := g.typeName(tval, false, false)
		keyname := g.tmp("key")
		valname := g.tmp("val")

		g.declareLocalVariableForWrite(out, tkey, keyname)
		g.declareLocalVariableForWrite(out, tval, valname)

		// If either the key or value type is a typedef, find its underlying type so
		// we can correctly determine how to generate a pointer to it
		tkey = sema.TrueType(tkey)
		tval = sema.TrueType(tval)

		tkeyPtr := ""
		if !tkey.IsString() && tkey.IsBaseType() {
			tkeyPtr = "*"
		}
		tvalPtr := ""
		if !tval.IsString() && tval.IsBaseType() {
			tvalPtr = "*"
		}

		// Some ugliness here. To maximize backwards compatibility, we
		// avoid using GHashTableIter and instead get a GList of all keys,
		// then copy it into a array on the stack, and free it.
		// This is because we may exit early before we get a chance to free the
		// GList.
		out.WriteString(g.indent() + "GList *key_list = NULL, *iter = NULL;\n" +
			g.indent() + tkeyName + tkeyPtr + "* keys;\n" +
			g.indent() + "int i = 0, key_count;\n\n" +
			g.indent() + "if ((ret = thrift_protocol_write_map_begin (protocol, " +
			g.typeToEnum(tkey) + ", " + g.typeToEnum(tval) + ", " + prefix + " ? " +
			"(gint32) g_hash_table_size ((GHashTable *) " + prefix + ") : 0" +
			", error)) < 0)\n")
		g.indentUp()
		out.WriteString(g.indent() + "return " + errorRet + ";\n")
		g.indentDown()
		out.WriteString(g.indent() + "xfer += ret;\n" +
			g.indent() + "if (" + prefix + ")\n" +
			g.indent() + "  g_hash_table_foreach ((GHashTable *) " + prefix +
			", thrift_hash_table_get_keys, &key_list);\n" +
			g.indent() + "key_count = g_list_length (key_list);\n" +
			g.indent() + "keys = g_newa (" + tkeyName + tkeyPtr + ", key_count);\n" +
			g.indent() + "for (iter = g_list_first (key_list); iter; iter = iter->next)\n")
		g.indentUp()
		out.WriteString(g.indent() + "keys[i++] = (" + tkeyName + tkeyPtr + ") iter->data;\n")
		g.indentDown()
		out.WriteString(g.indent() + "g_list_free (key_list);\n\n" +
			g.indent() + "for (i = 0; i < key_count; ++i)\n")
		g.scopeUp(out)
		out.WriteString(g.indent() + keyname + " = keys[i];\n" +
			g.indent() + valname + " = (" + tvalName + tvalPtr +
			") g_hash_table_lookup (((GHashTable *) " + prefix + "), (gpointer) " + keyname + ");\n\n")
		g.generateSerializeMapElement(out, m, tkeyPtr+" "+keyname, tvalPtr+" "+valname, errorRet)
		g.scopeDown(out)
		out.WriteString(g.indent() + "if ((ret = thrift_protocol_write_map_end (protocol, error)) < 0)\n")
		g.indentUp()
		out.WriteString(g.indent() + "return " + errorRet + ";\n")
		g.indentDown()
		out.WriteString(g.indent() + "xfer += ret;\n")
	} else if ttype.IsSet() {
		s := ttype.(*sema.Set)
		telem := s.ElemType()
		telemName := g.typeName(telem, false, false)
		telemPtr := ""
		if !telem.IsString() && telem.IsBaseType() {
			telemPtr = "*"
		}
		out.WriteString(g.indent() + "GList *key_list = NULL, *iter = NULL;\n" +
			g.indent() + telemName + telemPtr + "* keys;\n" +
			g.indent() + "int i = 0, key_count;\n" +
			g.indent() + telemName + telemPtr + " elem;\n" +
			g.indent() + "gpointer value;\n" +
			g.indent() + "THRIFT_UNUSED_VAR (value);\n\n" +
			g.indent() + "if ((ret = thrift_protocol_write_set_begin (protocol, " +
			g.typeToEnum(telem) + ", " + prefix + " ? " +
			"(gint32) g_hash_table_size ((GHashTable *) " + prefix + ") : 0" +
			", error)) < 0)\n")
		g.indentUp()
		out.WriteString(g.indent() + "return " + errorRet + ";\n")
		g.indentDown()
		out.WriteString(g.indent() + "xfer += ret;\n" +
			g.indent() + "if (" + prefix + ")\n" +
			g.indent() + "  g_hash_table_foreach ((GHashTable *) " + prefix +
			", thrift_hash_table_get_keys, &key_list);\n" +
			g.indent() + "key_count = g_list_length (key_list);\n" +
			g.indent() + "keys = g_newa (" + telemName + telemPtr + ", key_count);\n" +
			g.indent() + "for (iter = g_list_first (key_list); iter; iter = iter->next)\n")
		g.indentUp()
		out.WriteString(g.indent() + "keys[i++] = (" + telemName + telemPtr + ") iter->data;\n")
		g.indentDown()
		out.WriteString(g.indent() + "g_list_free (key_list);\n\n" +
			g.indent() + "for (i = 0; i < key_count; ++i)\n")
		g.scopeUp(out)
		out.WriteString(g.indent() + "elem = keys[i];\n" +
			g.indent() + "value = (gpointer) g_hash_table_lookup (((GHashTable *) " + prefix +
			"), (gpointer) elem);\n\n")
		g.generateSerializeSetElement(out, s, telemPtr+"elem", errorRet)
		g.scopeDown(out)
		out.WriteString(g.indent() + "if ((ret = thrift_protocol_write_set_end (protocol, error)) < 0)\n")
		g.indentUp()
		out.WriteString(g.indent() + "return " + errorRet + ";\n")
		g.indentDown()
		out.WriteString(g.indent() + "xfer += ret;\n")
	} else if ttype.IsList() {
		l := ttype.(*sema.List)
		length := "(" + prefix + " ? " + prefix + "->len : 0)"
		i := g.tmp("i")
		out.WriteString(g.indent() + "guint " + i + ";\n\n" +
			g.indent() + "if ((ret = thrift_protocol_write_list_begin (protocol, " +
			g.typeToEnum(l.ElemType()) + ", (gint32) " + length + ", error)) < 0)\n")
		g.indentUp()
		out.WriteString(g.indent() + "return " + errorRet + ";\n")
		g.indentDown()
		out.WriteString(g.indent() + "xfer += ret;\n" +
			g.indent() + "for (" + i + " = 0; " + i + " < " + length + "; " + i + "++)\n")
		g.scopeUp(out)
		g.generateSerializeListElement(out, l, prefix, i, errorRet)
		g.scopeDown(out)
		out.WriteString(g.indent() + "if ((ret = thrift_protocol_write_list_end (protocol, error)) < 0)\n")
		g.indentUp()
		out.WriteString(g.indent() + "return " + errorRet + ";\n")
		g.indentDown()
		out.WriteString(g.indent() + "xfer += ret;\n")
	}

	g.scopeDown(out)
}

// generateSerializeMapElement is generate_serialize_map_element.
func (g *Generator) generateSerializeMapElement(out *strings.Builder, tmap *sema.Map, key, value, errorRet string) {
	kfield := sema.NewField(tmap.KeyType(), key, 0)
	g.generateSerializeField(out, kfield, "", "", errorRet)

	vfield := sema.NewField(tmap.ValType(), value, 0)
	g.generateSerializeField(out, vfield, "", "", errorRet)
}

// generateSerializeSetElement is generate_serialize_set_element.
func (g *Generator) generateSerializeSetElement(out *strings.Builder, tset *sema.Set, element, errorRet string) {
	efield := sema.NewField(tset.ElemType(), element, 0)
	g.generateSerializeField(out, efield, "", "", errorRet)
}

// generateSerializeListElement is generate_serialize_list_element.
func (g *Generator) generateSerializeListElement(out *strings.Builder, tlist *sema.List, list, index, errorRet string) {
	ttype := sema.TrueType(tlist.ElemType())

	cast := ""
	name := "g_ptr_array_index ((GPtrArray *) " + list + ", " + index + ")"

	if ttype.IsVoid() {
		emit.Throw("compiler error: list element type cannot be void")
	} else if g.isNumeric(ttype) {
		name = "g_array_index (" + list + ", " + g.baseTypeName(ttype) + ", " + index + ")"
	} else if ttype.IsString() {
		cast = "(gchar*)"
	} else if ttype.IsMap() || ttype.IsSet() {
		cast = "(GHashTable*)"
	} else if ttype.IsList() {
		etype := ttype.(*sema.List).ElemType()
		if etype.IsVoid() {
			emit.Throw("compiler error: list element type cannot be void")
		}
		if g.isNumeric(etype) {
			cast = "(GArray*)"
		} else {
			cast = "(GPtrArray*)"
		}
	}

	efield := sema.NewField(ttype, "("+cast+name+")", 0)
	g.generateSerializeField(out, efield, "", "", errorRet)
}

// generateDeserializeField deserializes a field of any type.
func (g *Generator) generateDeserializeField(out *strings.Builder, tfield *sema.Field, prefix, suffix, errorRet string, allocate bool) {
	t := sema.TrueType(tfield.Type())

	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE DESERIALIZE CODE FOR void TYPE: %s%s", prefix, tfield.Name())
	}

	name := prefix + tfield.Name() + suffix

	if t.IsStruct() || t.IsXception() {
		g.generateDeserializeStruct(out, t.(*sema.Struct), name, errorRet, allocate)
	} else if t.IsContainer() {
		g.generateDeserializeContainer(out, t, name, errorRet)
	} else if t.IsBaseType() {
		bt := t.(*sema.BaseType)
		if bt.Base() == sema.TypeString {
			out.WriteString(g.indent() + "if (" + name + " != NULL)\n" + g.indent() + "{\n")
			g.indentUp()
			if bt.IsBinary() {
				out.WriteString(g.indent() + "g_byte_array_free(" + name + ", TRUE);\n")
			} else {
				out.WriteString(g.indent() + "g_free(" + name + ");\n")
			}
			out.WriteString(g.indent() + name + " = NULL;\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n\n")
		}
		out.WriteString(g.indent() + "if ((ret = thrift_protocol_read_")

		switch bt.Base() {
		case sema.TypeVoid:
			emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
		case sema.TypeString:
			if bt.IsBinary() {
				out.WriteString("binary (protocol, &data, &len")
			} else {
				out.WriteString("string (protocol, &" + name)
			}
		case sema.TypeBool:
			out.WriteString("bool (protocol, &" + name)
		case sema.TypeI8:
			out.WriteString("byte (protocol, &" + name)
		case sema.TypeI16:
			out.WriteString("i16 (protocol, &" + name)
		case sema.TypeI32:
			out.WriteString("i32 (protocol, &" + name)
		case sema.TypeI64:
			out.WriteString("i64 (protocol, &" + name)
		case sema.TypeDouble:
			out.WriteString("double (protocol, &" + name)
		default:
			emit.Throw("compiler error: no C reader for base type %s%s", sema.BaseName(bt.Base()), name)
		}
		out.WriteString(", error)) < 0)\n")
		out.WriteString(g.indent() + "  return " + errorRet + ";\n" + g.indent() + "xfer += ret;\n")

		// load the byte array with the data
		if bt.Base() == sema.TypeString && bt.IsBinary() {
			out.WriteString(g.indent() + name + " = g_byte_array_new();\n")
			out.WriteString(g.indent() + "g_byte_array_append (" + name + ", (guint8 *) data, (guint) len);\n")
			out.WriteString(g.indent() + "g_free (data);\n")
		}
	} else if t.IsEnum() {
		tv := g.tmp("ecast")
		out.WriteString(g.indent() + "gint32 " + tv + ";\n" +
			g.indent() + "if ((ret = thrift_protocol_read_i32 (protocol, &" + tv + ", error)) < 0)\n" +
			g.indent() + "  return " + errorRet + ";\n" + g.indent() + "xfer += ret;\n" +
			g.indent() + name + " = (" + g.typeName(t, false, false) + ")" + tv + ";\n")
	} else {
		emit.Throw("DO NOT KNOW HOW TO SERIALIZE FIELD '%s' TYPE '%s", tfield.Name(), g.typeName(t, false, false))
	}

	// if the type is not required and this is a thrift struct (no prefix),
	// set the isset variable. if the type is required, then set the
	// local variable indicating the value was set, so that we can do
	// validation later.
	if prefix != "" && tfield.Req() != sema.Required {
		out.WriteString(g.indent() + prefix + "__isset_" + tfield.Name() + suffix + " = TRUE;\n")
	} else if prefix != "" && tfield.Req() == sema.Required {
		out.WriteString(g.indent() + "isset_" + tfield.Name() + " = TRUE;\n")
	}
}

// generateDeserializeStruct is generate_deserialize_struct.
func (g *Generator) generateDeserializeStruct(out *strings.Builder, tstruct *sema.Struct, prefix, errorRet string, allocate bool) {
	nameUC := toUpperCase(initialCapsToUnderscores(tstruct.Name()))
	if tstruct.IsXception() {
		out.WriteString(g.indent() + "/* This struct is an exception */\n")
		allocate = true
	}

	if allocate {
		out.WriteString(g.indent() + "if ( " + prefix + " != NULL)\n" + g.indent() + "{\n")
		g.indentUp()
		out.WriteString(g.indent() + "g_object_unref (" + prefix + ");\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n" + g.indent() + prefix + " = g_object_new (" + g.nspaceUC +
			"TYPE_" + nameUC + ", NULL);\n")
	}
	out.WriteString(g.indent() + "if ((ret = thrift_struct_read (THRIFT_STRUCT (" + prefix +
		"), protocol, error)) < 0)\n" + g.indent() + "{\n")
	g.indentUp()
	if allocate {
		out.WriteString(g.indent() + "g_object_unref (" + prefix + ");\n")
		if tstruct.IsXception() {
			out.WriteString(g.indent() + prefix + " = NULL;\n")
		}
	}
	out.WriteString(g.indent() + "return " + errorRet + ";\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n" + g.indent() + "xfer += ret;\n")
}

// generateDeserializeContainer is generate_deserialize_container.
func (g *Generator) generateDeserializeContainer(out *strings.Builder, ttype sema.Type, prefix, errorRet string) {
	g.scopeUp(out)

	if ttype.IsMap() {
		m := ttype.(*sema.Map)
		out.WriteString(g.indent() + "guint32 size;\n" +
			g.indent() + "guint32 i;\n" +
			g.indent() + "ThriftType key_type;\n" +
			g.indent() + "ThriftType value_type;\n\n" +
			g.indent() + "/* read the map begin marker */\n" +
			g.indent() + "if ((ret = thrift_protocol_read_map_begin (protocol, &key_type, &value_type, &size, error)) < 0)\n")
		g.indentUp()
		out.WriteString(g.indent() + "return " + errorRet + ";\n")
		g.indentDown()
		out.WriteString(g.indent() + "xfer += ret;\n\n")

		out.WriteString(g.indent() + "/* iterate through each of the map's fields */\n" +
			g.indent() + "for (i = 0; i < size; i++)\n")
		g.scopeUp(out)
		g.generateDeserializeMapElement(out, m, prefix, errorRet)
		g.scopeDown(out)
		out.WriteString("\n")

		out.WriteString(g.indent() + "/* read the map end marker */\n" +
			g.indent() + "if ((ret = thrift_protocol_read_map_end (protocol, error)) < 0)\n")
		g.indentUp()
		out.WriteString(g.indent() + "return " + errorRet + ";\n")
		g.indentDown()
		out.WriteString(g.indent() + "xfer += ret;\n")
	} else if ttype.IsSet() {
		s := ttype.(*sema.Set)
		out.WriteString(g.indent() + "guint32 size;\n" +
			g.indent() + "guint32 i;\n" +
			g.indent() + "ThriftType element_type;\n\n" +
			g.indent() + "if ((ret = thrift_protocol_read_set_begin (protocol, &element_type, &size, error)) < 0)\n")
		g.indentUp()
		out.WriteString(g.indent() + "return " + errorRet + ";\n")
		g.indentDown()
		out.WriteString(g.indent() + "xfer += ret;\n\n")

		out.WriteString(g.indent() + "/* iterate through the set elements */\n" +
			g.indent() + "for (i = 0; i < size; ++i)\n")
		g.scopeUp(out)
		g.generateDeserializeSetElement(out, s, prefix, errorRet)
		g.scopeDown(out)

		out.WriteString(g.indent() + "if ((ret = thrift_protocol_read_set_end (protocol, error)) < 0)\n")
		g.indentUp()
		out.WriteString(g.indent() + "return " + errorRet + ";\n")
		g.indentDown()
		out.WriteString(g.indent() + "xfer += ret;\n\n")
	} else if ttype.IsList() {
		l := ttype.(*sema.List)
		out.WriteString(g.indent() + "guint32 size;\n" +
			g.indent() + "guint32 i;\n" +
			g.indent() + "ThriftType element_type;\n\n" +
			g.indent() + "if ((ret = thrift_protocol_read_list_begin (protocol, &element_type,&size, error)) < 0)\n")
		g.indentUp()
		out.WriteString(g.indent() + "return " + errorRet + ";\n")
		g.indentDown()
		out.WriteString(g.indent() + "xfer += ret;\n\n")

		out.WriteString(g.indent() + "/* iterate through list elements */\n" +
			g.indent() + "for (i = 0; i < size; i++)\n")
		g.scopeUp(out)
		g.generateDeserializeListElement(out, l, prefix, "i", errorRet)
		g.scopeDown(out)

		out.WriteString(g.indent() + "if ((ret = thrift_protocol_read_list_end (protocol, error)) < 0)\n")
		g.indentUp()
		out.WriteString(g.indent() + "return " + errorRet + ";\n")
		g.indentDown()
		out.WriteString(g.indent() + "xfer += ret;\n")
	}

	g.scopeDown(out)
}

// generateDeserializeMapElement is generate_deserialize_map_element.
func (g *Generator) generateDeserializeMapElement(out *strings.Builder, tmap *sema.Map, prefix, errorRet string) {
	tkey, tval := tmap.KeyType(), tmap.ValType()
	keyname := g.tmp("key")
	valname := g.tmp("val")

	g.declareLocalVariable(out, tkey, keyname, true)
	g.declareLocalVariable(out, tval, valname, true)

	// If either the key or value type is a typedef, find its underlying
	// type so we can correctly determine how to generate a pointer to it
	tkey = sema.TrueType(tkey)
	tval = sema.TrueType(tval)

	tkeyPtr := ""
	if !tkey.IsString() && tkey.IsBaseType() {
		tkeyPtr = "*"
	}
	tvalPtr := ""
	if !tval.IsString() && tval.IsBaseType() {
		tvalPtr = "*"
	}

	// deserialize the fields of the map element
	fkey := sema.NewField(tkey, tkeyPtr+keyname, 0)
	g.generateDeserializeField(out, fkey, "", "", errorRet, true)
	fval := sema.NewField(tval, tvalPtr+valname, 0)
	g.generateDeserializeField(out, fval, "", "", errorRet, true)

	out.WriteString(g.indent() + "if (" + prefix + " && " + keyname + ")\n")
	g.indentUp()
	out.WriteString(g.indent() + "g_hash_table_insert ((GHashTable *)" + prefix + ", (gpointer) " +
		keyname + ", (gpointer) " + valname + ");\n")
	g.indentDown()
}

// generateDeserializeSetElement is generate_deserialize_set_element.
func (g *Generator) generateDeserializeSetElement(out *strings.Builder, tset *sema.Set, prefix, errorRet string) {
	telem := tset.ElemType()
	elem := g.tmp("_elem")

	g.declareLocalVariable(out, telem, elem, true)

	telem = sema.TrueType(telem)
	telemPtr := ""
	if !telem.IsString() && telem.IsBaseType() {
		telemPtr = "*"
	}

	felem := sema.NewField(telem, telemPtr+elem, 0)
	g.generateDeserializeField(out, felem, "", "", errorRet, true)

	out.WriteString(g.indent() + "if (" + prefix + " && " + elem + ")\n")
	g.indentUp()
	out.WriteString(g.indent() + "g_hash_table_insert ((GHashTable *) " + prefix + ", (gpointer) " +
		elem + ", (gpointer) " + elem + ");\n")
	g.indentDown()
}

// generateDeserializeListElement is generate_deserialize_list_element.
func (g *Generator) generateDeserializeListElement(out *strings.Builder, tlist *sema.List, prefix, index, errorRet string) {
	ttype := sema.TrueType(tlist.ElemType())
	elem := g.tmp("_elem")
	telemPtr := ""
	if g.isNumeric(ttype) {
		telemPtr = "*"
	}

	g.declareLocalVariable(out, ttype, elem, false)

	felem := sema.NewField(ttype, telemPtr+elem, 0)
	g.generateDeserializeField(out, felem, "", "", errorRet, true)

	if ttype.IsVoid() {
		emit.Throw("compiler error: list element type cannot be void")
	} else if g.isNumeric(ttype) {
		out.WriteString(g.indent() + "g_array_append_vals (" + prefix + ", " + elem + ", 1);\n")
		out.WriteString(g.indent() + "g_free (" + elem + ");\n")
	} else {
		out.WriteString(g.indent() + "g_ptr_array_add (" + prefix + ", " + elem + ");\n")
	}
}
