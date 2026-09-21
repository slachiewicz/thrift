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

package php

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateDeserializeField deserializes a field of any type.
func (g *Generator) generateDeserializeField(out *strings.Builder, field *sema.Field, prefix string, inclass bool) {
	t := sema.TrueType(field.Type())

	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE DESERIALIZE CODE FOR void TYPE: %s%s", prefix, field.Name())
	}

	name := prefix + field.Name()

	switch {
	case t.IsStruct() || t.IsXception():
		g.generateDeserializeStruct(out, t.(*sema.Struct), name)
	case t.IsContainer():
		g.generateDeserializeContainer(out, t, name)
	case t.IsBaseType() || t.IsEnum():
		if g.opts.Inlined {
			itrans := "$input"
			if inclass {
				itrans = "$this->input"
			}
			if t.IsBaseType() {
				bt := t.(*sema.BaseType)
				switch bt.Base() {
				case sema.TypeVoid:
					emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
				case sema.TypeString:
					out.WriteString(g.indent() + "$len = unpack('N', " + itrans + "->readAll(4));\n" +
						g.indent() + "$len = $len[1];\n" + g.indent() + "if ($len > 0x7fffffff) {\n" +
						g.indent() + "    $len = 0 - (($len - 1) ^ 0xffffffff);\n" + g.indent() +
						"}\n" + g.indent() + "$" + name + " = " + itrans + "->readAll($len);\n")
				case sema.TypeBool:
					// Stage through a local; the typed property would
					// reject the intermediate `array|false` returned
					// by unpack().
					out.WriteString(g.indent() + "$val = unpack('c', " + itrans + "->readAll(1));\n" +
						g.indent() + "$" + name + " = (bool)$val[1];\n")
				case sema.TypeI8:
					out.WriteString(g.indent() + "$val = unpack('c', " + itrans + "->readAll(1));\n" +
						g.indent() + "$" + name + " = $val[1];\n")
				case sema.TypeI16:
					out.WriteString(g.indent() + "$val = unpack('n', " + itrans + "->readAll(2));\n" +
						g.indent() + "$val = $val[1];\n" + g.indent() + "if ($val > 0x7fff) {\n" +
						g.indent() + "    $val = 0 - (($val - 1) ^ 0xffff);\n" + g.indent() +
						"}\n" + g.indent() + "$" + name + " = $val;\n")
				case sema.TypeI32:
					out.WriteString(g.indent() + "$val = unpack('N', " + itrans + "->readAll(4));\n" +
						g.indent() + "$val = $val[1];\n" + g.indent() + "if ($val > 0x7fffffff) {\n" +
						g.indent() + "    $val = 0 - (($val - 1) ^ 0xffffffff);\n" + g.indent() +
						"}\n" + g.indent() + "$" + name + " = $val;\n")
				case sema.TypeI64:
					out.WriteString(g.indent() + "$arr = unpack('N2', " + itrans + "->readAll(8));\n" +
						g.indent() + "if ($arr[1] & 0x80000000) {\n" + g.indent() +
						"    $arr[1] = $arr[1] ^ 0xFFFFFFFF;\n" + g.indent() +
						"    $arr[2] = $arr[2] ^ 0xFFFFFFFF;\n" + g.indent() + "    $" + name +
						" = 0 - $arr[1] * 4294967296 - $arr[2] - 1;\n" + g.indent() + "} else {\n" +
						g.indent() + "    $" + name + " = $arr[1] * 4294967296 + $arr[2];\n" +
						g.indent() + "}\n")
				case sema.TypeDouble:
					out.WriteString(g.indent() + "$arr = unpack('d', strrev(" + itrans + "->readAll(8)));\n" +
						g.indent() + "$" + name + " = $arr[1];\n")
				case sema.TypeUUID:
					out.WriteString(g.indent() + "$uuidBin = " + itrans + "->readAll(16);\n" +
						g.indent() + "$uuidHex = bin2hex($uuidBin);\n" +
						g.indent() + "$" + name + " = substr($uuidHex, 0, 8) . '-' . " +
						"substr($uuidHex, 8, 4) . '-' . " +
						"substr($uuidHex, 12, 4) . '-' . " +
						"substr($uuidHex, 16, 4) . '-' . " +
						"substr($uuidHex, 20, 12);\n")
				default:
					emit.Throw("compiler error: no PHP name for base type %s%s", sema.BaseName(bt.Base()), field.Name())
				}
			} else if t.IsEnum() {
				out.WriteString(g.indent() + "$val = unpack('N', " + itrans + "->readAll(4));\n" + g.indent() +
					"$val = $val[1];\n" + g.indent() + "if ($val > 0x7fffffff) {\n" +
					g.indent() + "    $val = 0 - (($val - 1) ^ 0xffffffff);\n" + g.indent() + "}\n" +
					g.indent() + "$" + name + " = $val;\n")
			}
		} else {
			out.WriteString(g.indent() + "$xfer += $input->")
			if t.IsBaseType() {
				bt := t.(*sema.BaseType)
				switch bt.Base() {
				case sema.TypeVoid:
					emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
				case sema.TypeString:
					out.WriteString("readString($" + name + ");")
				case sema.TypeBool:
					out.WriteString("readBool($" + name + ");")
				case sema.TypeI8:
					out.WriteString("readByte($" + name + ");")
				case sema.TypeI16:
					out.WriteString("readI16($" + name + ");")
				case sema.TypeI32:
					out.WriteString("readI32($" + name + ");")
				case sema.TypeI64:
					out.WriteString("readI64($" + name + ");")
				case sema.TypeDouble:
					out.WriteString("readDouble($" + name + ");")
				case sema.TypeUUID:
					out.WriteString("readUuid($" + name + ");")
				default:
					emit.Throw("compiler error: no PHP name for base type %s", sema.BaseName(bt.Base()))
				}
			} else if t.IsEnum() {
				out.WriteString("readI32($" + name + ");")
			}
			out.WriteString("\n")
		}
	}
}

// generateDeserializeStruct generates an unserializer for a variable.
// This makes two key assumptions, first that there is a const char*
// variable named data that points to the buffer for deserialization,
// and that there is a variable protocol which is a reference to a
// TProtocol serialization object.
func (g *Generator) generateDeserializeStruct(out *strings.Builder, s *sema.Struct, prefix string) {
	out.WriteString(g.indent() + "$" + prefix + " = new " + g.phpNamespace(s.Program()) +
		s.Name() + "();\n" + g.indent() + "$xfer += $" + prefix + "->read($input);\n")
}

func (g *Generator) generateDeserializeContainer(out *strings.Builder, t sema.Type, prefix string) {
	size := g.tmp("_size")
	ktype := g.tmp("_ktype")
	vtype := g.tmp("_vtype")
	etype := g.tmp("_etype")

	fsize := newTempField(sema.GlobalI32, size)
	fktype := newTempField(sema.GlobalI8, ktype)
	fvtype := newTempField(sema.GlobalI8, vtype)
	fetype := newTempField(sema.GlobalI8, etype)

	out.WriteString(g.indent() + "$" + prefix + " = [];\n" + g.indent() + "$" + size + " = 0;\n")

	// Declare variables, read header.
	switch {
	case t.IsMap():
		out.WriteString(g.indent() + "$" + ktype + " = 0;\n" + g.indent() + "$" + vtype + " = 0;\n")
		if g.opts.Inlined {
			g.generateDeserializeField(out, fktype, "", false)
			g.generateDeserializeField(out, fvtype, "", false)
			g.generateDeserializeField(out, fsize, "", false)
		} else {
			out.WriteString(g.indent() + "$xfer += $input->readMapBegin(" +
				"$" + ktype + ", $" + vtype + ", $" + size + ");\n")
		}
	case t.IsSet():
		if g.opts.Inlined {
			g.generateDeserializeField(out, fetype, "", false)
			g.generateDeserializeField(out, fsize, "", false)
		} else {
			out.WriteString(g.indent() + "$" + etype + " = 0;\n" + g.indent() +
				"$xfer += $input->readSetBegin(" + "$" + etype + ", $" + size + ");\n")
		}
	case t.IsList():
		if g.opts.Inlined {
			g.generateDeserializeField(out, fetype, "", false)
			g.generateDeserializeField(out, fsize, "", false)
		} else {
			out.WriteString(g.indent() + "$" + etype + " = 0;\n" + g.indent() +
				"$xfer += $input->readListBegin(" + "$" + etype + ", $" + size + ");\n")
		}
	}

	// For loop iterates over elements.
	i := g.tmp("_i")
	out.WriteString(g.indent() + "for ($" + i + " = 0; $" + i + " < $" + size + "; ++$" + i + ") {\n")
	g.indentUp()

	switch {
	case t.IsMap():
		g.generateDeserializeMapElement(out, t.(*sema.Map), prefix)
	case t.IsSet():
		g.generateDeserializeSetElement(out, t.(*sema.Set), prefix)
	case t.IsList():
		g.generateDeserializeListElement(out, t.(*sema.List), prefix)
	}

	g.scopeDown(out)

	if !g.opts.Inlined {
		// Read container end.
		switch {
		case t.IsMap():
			out.WriteString(g.indent() + "$xfer += $input->readMapEnd();\n")
		case t.IsSet():
			out.WriteString(g.indent() + "$xfer += $input->readSetEnd();\n")
		case t.IsList():
			out.WriteString(g.indent() + "$xfer += $input->readListEnd();\n")
		}
	}
}

// generateDeserializeMapElement generates code to deserialize a map.
func (g *Generator) generateDeserializeMapElement(out *strings.Builder, m *sema.Map, prefix string) {
	key := g.tmp("key")
	val := g.tmp("val")
	fkey := newTempField(m.KeyType(), key)
	fval := newTempField(m.ValType(), val)

	out.WriteString(g.indent() + g.declareField(fkey, true, true) + "\n")
	out.WriteString(g.indent() + g.declareField(fval, true, true) + "\n")

	g.generateDeserializeField(out, fkey, "", false)
	g.generateDeserializeField(out, fval, "", false)

	out.WriteString(g.indent() + "$" + prefix + "[$" + key + "] = $" + val + ";\n")
}

func (g *Generator) generateDeserializeSetElement(out *strings.Builder, s *sema.Set, prefix string) {
	elem := g.tmp("elem")
	felem := newTempField(s.ElemType(), elem)

	out.WriteString(g.indent() + "$" + elem + " = null;\n")

	g.generateDeserializeField(out, felem, "", false)

	elemType := s.ElemType()
	if phpIsScalar(elemType) {
		out.WriteString(g.indent() + "$" + prefix + "[$" + elem + "] = true;\n")
	} else {
		out.WriteString(g.indent() + "$" + prefix + "[] = $" + elem + ";\n")
	}
}

func (g *Generator) generateDeserializeListElement(out *strings.Builder, l *sema.List, prefix string) {
	elem := g.tmp("elem")
	felem := newTempField(l.ElemType(), elem)

	out.WriteString(g.indent() + "$" + elem + " = null;\n")

	g.generateDeserializeField(out, felem, "", false)

	out.WriteString(g.indent() + "$" + prefix + "[] = $" + elem + ";\n")
}

// generateSerializeField serializes a field of any type.
func (g *Generator) generateSerializeField(out *strings.Builder, field *sema.Field, prefix string) {
	t := sema.TrueType(field.Type())

	// Do nothing for void types.
	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE SERIALIZE CODE FOR void TYPE: %s%s", prefix, field.Name())
	}

	switch {
	case t.IsStruct() || t.IsXception():
		g.generateSerializeStruct(out, t.(*sema.Struct), prefix+field.Name())
	case t.IsContainer():
		g.generateSerializeContainer(out, t, prefix+field.Name())
	case t.IsBaseType() || t.IsEnum():
		name := prefix + field.Name()

		if g.opts.Inlined {
			if t.IsBaseType() {
				bt := t.(*sema.BaseType)
				switch bt.Base() {
				case sema.TypeVoid:
					emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
				case sema.TypeString:
					out.WriteString(g.indent() + "$output .= pack('N', strlen($" + name + "));\n" + g.indent() +
						"$output .= $" + name + ";\n")
				case sema.TypeBool:
					out.WriteString(g.indent() + "$output .= pack('c', $" + name + " ? 1 : 0);\n")
				case sema.TypeI8:
					out.WriteString(g.indent() + "$output .= pack('c', $" + name + ");\n")
				case sema.TypeI16:
					out.WriteString(g.indent() + "$output .= pack('n', $" + name + ");\n")
				case sema.TypeI32:
					out.WriteString(g.indent() + "$output .= pack('N', $" + name + ");\n")
				case sema.TypeI64:
					out.WriteString(g.indent() + "$output .= pack('N2', $" + name + " >> 32, $" + name +
						" & 0xFFFFFFFF);\n")
				case sema.TypeDouble:
					out.WriteString(g.indent() + "$output .= strrev(pack('d', $" + name + "));\n")
				case sema.TypeUUID:
					out.WriteString(g.indent() + "$output .= hex2bin(str_replace('-', '', $" + name + "));\n")
				default:
					emit.Throw("compiler error: no PHP name for base type %s", sema.BaseName(bt.Base()))
				}
			} else if t.IsEnum() {
				out.WriteString(g.indent() + "$output .= pack('N', $" + name + ");\n")
			}
		} else {
			out.WriteString(g.indent() + "$xfer += $output->")
			if t.IsBaseType() {
				bt := t.(*sema.BaseType)
				switch bt.Base() {
				case sema.TypeVoid:
					emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
				case sema.TypeString:
					out.WriteString("writeString($" + name + ");")
				case sema.TypeBool:
					out.WriteString("writeBool($" + name + ");")
				case sema.TypeI8:
					out.WriteString("writeByte($" + name + ");")
				case sema.TypeI16:
					out.WriteString("writeI16($" + name + ");")
				case sema.TypeI32:
					out.WriteString("writeI32($" + name + ");")
				case sema.TypeI64:
					out.WriteString("writeI64($" + name + ");")
				case sema.TypeDouble:
					out.WriteString("writeDouble($" + name + ");")
				case sema.TypeUUID:
					out.WriteString("writeUuid($" + name + ");")
				default:
					emit.Throw("compiler error: no PHP name for base type %s", sema.BaseName(bt.Base()))
				}
			} else if t.IsEnum() {
				out.WriteString("writeI32($" + name + ");")
			}
			out.WriteString("\n")
		}
	}
}

// generateSerializeStruct serializes all the members of a struct.
func (g *Generator) generateSerializeStruct(out *strings.Builder, s *sema.Struct, prefix string) {
	_ = s
	out.WriteString(g.indent() + "$xfer += $" + prefix + "->write($output);\n")
}

// generateSerializeContainer writes out a container.
func (g *Generator) generateSerializeContainer(out *strings.Builder, t sema.Type, prefix string) {
	switch {
	case t.IsMap():
		m := t.(*sema.Map)
		if g.opts.Inlined {
			out.WriteString(g.indent() + "$output .= pack('c', " + typeToEnum(m.KeyType()) +
				");\n" + g.indent() + "$output .= pack('c', " +
				typeToEnum(m.ValType()) + ");\n" + g.indent() +
				"$output .= strrev(pack('l', count($" + prefix + ")));\n")
		} else {
			out.WriteString(g.indent() + "$output->writeMapBegin(" + typeToEnum(m.KeyType()) +
				", " + typeToEnum(m.ValType()) + ", " +
				"count($" + prefix + "));\n")
		}
	case t.IsSet():
		s := t.(*sema.Set)
		if g.opts.Inlined {
			out.WriteString(g.indent() + "$output .= pack('c', " + typeToEnum(s.ElemType()) +
				");\n" + g.indent() + "$output .= strrev(pack('l', count($" + prefix + ")));\n")
		} else {
			out.WriteString(g.indent() + "$output->writeSetBegin(" + typeToEnum(s.ElemType()) +
				", " + "count($" + prefix + "));\n")
		}
	case t.IsList():
		l := t.(*sema.List)
		if g.opts.Inlined {
			out.WriteString(g.indent() + "$output .= pack('c', " + typeToEnum(l.ElemType()) +
				");\n" + g.indent() + "$output .= strrev(pack('l', count($" + prefix + ")));\n")
		} else {
			out.WriteString(g.indent() + "$output->writeListBegin(" + typeToEnum(l.ElemType()) +
				", " + "count($" + prefix + "));\n")
		}
	}

	switch {
	case t.IsMap():
		m := t.(*sema.Map)
		kiter := g.tmp("kiter")
		viter := g.tmp("viter")
		out.WriteString(g.indent() + "foreach ($" + prefix + " as " +
			"$" + kiter + " => $" + viter + ") {\n")
		g.indentUp()
		g.generateSerializeMapElement(out, m, kiter, viter)
		g.scopeDown(out)
	case t.IsSet():
		s := t.(*sema.Set)
		iter := g.tmp("iter")
		iterVal := g.tmp("iter")
		out.WriteString(g.indent() + "foreach ($" + prefix + " as $" + iter + " => $" + iterVal + ") {\n")
		g.indentUp()

		elemType := s.ElemType()
		if phpIsScalar(elemType) {
			g.generateSerializeSetElement(out, s, iter)
		} else {
			g.generateSerializeSetElement(out, s, iterVal)
		}
		g.scopeDown(out)
	case t.IsList():
		l := t.(*sema.List)
		iter := g.tmp("iter")
		out.WriteString(g.indent() + "foreach ($" + prefix + " as $" + iter + ") {\n")
		g.indentUp()
		g.generateSerializeListElement(out, l, iter)
		g.scopeDown(out)
	}

	if !g.opts.Inlined {
		switch {
		case t.IsMap():
			out.WriteString(g.indent() + "$output->writeMapEnd();\n")
		case t.IsSet():
			out.WriteString(g.indent() + "$output->writeSetEnd();\n")
		case t.IsList():
			out.WriteString(g.indent() + "$output->writeListEnd();\n")
		}
	}
}

// emitArrayKeyRecast: PHP foreach yields keys with their stored
// array-key type -- numeric strings are normalised to int at insertion
// (['123' => x] becomes [123 => x]), and bool keys collapse to int
// ([true => x] becomes [1 => x]). Under declare(strict_types=1) the
// typed writeXxx() library calls then refuse the coerced value. Emit a
// single-line cast back to the declared Thrift type before the write so
// the runtime contract holds.
//
// Reuses typeToCast, which already maps every Thrift base type and enum
// to its PHP cast prefix; non-castable types (struct/container/void)
// yield an empty string and are skipped.
func (g *Generator) emitArrayKeyRecast(out *strings.Builder, t sema.Type, varName string) {
	cast := typeToCast(sema.TrueType(t))
	if cast == "" {
		return
	}
	out.WriteString(g.indent() + "$" + varName + " = " + cast + "$" + varName + ";\n")
}

// generateSerializeMapElement serializes the members of a map.
func (g *Generator) generateSerializeMapElement(out *strings.Builder, m *sema.Map, kiter, viter string) {
	// PHP arrays silently coerce numeric-string keys to int (e.g. '123'
	// => 123). Cast back to the declared base type so the typed
	// writeXxx() call sites in strict-types generated files accept the
	// value.
	g.emitArrayKeyRecast(out, m.KeyType(), kiter)

	kfield := newTempField(m.KeyType(), kiter)
	g.generateSerializeField(out, kfield, "")

	vfield := newTempField(m.ValType(), viter)
	g.generateSerializeField(out, vfield, "")
}

// generateSerializeSetElement serializes the members of a set.
func (g *Generator) generateSerializeSetElement(out *strings.Builder, s *sema.Set, iter string) {
	// Set element used as PHP array key -- same coercion concern as map
	// keys; see comment on emitArrayKeyRecast. Helper no-ops for
	// non-castable element types.
	g.emitArrayKeyRecast(out, s.ElemType(), iter)

	efield := newTempField(s.ElemType(), iter)
	g.generateSerializeField(out, efield, "")
}

// generateSerializeListElement serializes the members of a list.
func (g *Generator) generateSerializeListElement(out *strings.Builder, l *sema.List, iter string) {
	efield := newTempField(l.ElemType(), iter)
	g.generateSerializeField(out, efield, "")
}
