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

package perl

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateDeserializeField deserializes a field of any type.
func (g *Generator) generateDeserializeField(out *strings.Builder, tfield *sema.Field, prefix string, inclass bool) {
	_ = inclass
	t := sema.TrueType(tfield.Type())

	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE DESERIALIZE CODE FOR void TYPE: %s%s", prefix, tfield.Name())
	}

	name := tfield.Name()

	// Hack for when prefix is defined (always a hash ref).
	if prefix != "" {
		name = prefix + "{" + tfield.Name() + "}"
	}

	switch {
	case t.IsStruct() || t.IsXception():
		g.generateDeserializeStruct(out, t.(*sema.Struct), name)
	case t.IsContainer():
		g.generateDeserializeContainer(out, t, name)
	case t.IsBaseType() || t.IsEnum():
		out.WriteString(g.indent() + "$xfer += $input->")

		if t.IsBaseType() {
			switch t.(*sema.BaseType).Base() {
			case sema.TypeVoid:
				emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
			case sema.TypeString:
				out.WriteString("readString(\\$" + name + ");")
			case sema.TypeBool:
				out.WriteString("readBool(\\$" + name + ");")
			case sema.TypeI8:
				out.WriteString("readByte(\\$" + name + ");")
			case sema.TypeI16:
				out.WriteString("readI16(\\$" + name + ");")
			case sema.TypeI32:
				out.WriteString("readI32(\\$" + name + ");")
			case sema.TypeI64:
				out.WriteString("readI64(\\$" + name + ");")
			case sema.TypeDouble:
				out.WriteString("readDouble(\\$" + name + ");")
			default:
				emit.Throw("compiler error: no PERL name for base type %s", sema.BaseName(t.(*sema.BaseType).Base()))
			}
		} else if t.IsEnum() {
			out.WriteString("readI32(\\$" + name + ");")
		}
		out.WriteString("\n")
	default:
		// DO NOT KNOW HOW TO DESERIALIZE FIELD: unreachable for a
		// well-formed program; the C++ generator only printf's to
		// stdout here, which is not part of the generated file.
	}
}

// generateDeserializeStruct generates an unserializer for a variable. This
// makes two key assumptions, first that there is a const char* variable
// named data that points to the buffer for deserialization, and that
// there is a variable protocol which is a reference to a TProtocol
// serialization object.
func (g *Generator) generateDeserializeStruct(out *strings.Builder, tstruct *sema.Struct, prefix string) {
	out.WriteString(g.indent() + "$" + prefix + " = " + perlNamespace(tstruct.Program()) +
		tstruct.Name() + "->new();\n" + g.indent() + "$xfer += $" + prefix + "->read($input);\n")
}

func (g *Generator) generateDeserializeContainer(out *strings.Builder, ttype sema.Type, prefix string) {
	g.scopeUp(out)

	size := g.tmp("_size")
	ktype := g.tmp("_ktype")
	vtype := g.tmp("_vtype")
	etype := g.tmp("_etype")

	out.WriteString(g.indent() + "my $" + size + " = 0;\n")

	// Declare variables, read header.
	switch {
	case ttype.IsMap():
		out.WriteString(g.indent() + "$" + prefix + " = {};\n" + g.indent() + "my $" + ktype + " = 0;\n" +
			g.indent() + "my $" + vtype + " = 0;\n")

		out.WriteString(g.indent() + "$xfer += $input->readMapBegin(" +
			"\\$" + ktype + ", \\$" + vtype + ", \\$" + size + ");\n")

	case ttype.IsSet():
		out.WriteString(g.indent() + "$" + prefix + " = {};\n" + g.indent() + "my $" + etype + " = 0;\n" +
			g.indent() + "$xfer += $input->readSetBegin(" +
			"\\$" + etype + ", \\$" + size + ");\n")

	case ttype.IsList():
		out.WriteString(g.indent() + "$" + prefix + " = [];\n" + g.indent() + "my $" + etype + " = 0;\n" +
			g.indent() + "$xfer += $input->readListBegin(" +
			"\\$" + etype + ", \\$" + size + ");\n")
	}

	// For loop iterates over elements.
	i := g.tmp("_i")
	out.WriteString(g.indent() + "for (my $" + i + " = 0; $" + i + " < $" + size + "; ++$" + i + ")\n")

	g.scopeUp(out)

	switch {
	case ttype.IsMap():
		g.generateDeserializeMapElement(out, ttype.(*sema.Map), prefix)
	case ttype.IsSet():
		g.generateDeserializeSetElement(out, ttype.(*sema.Set), prefix)
	case ttype.IsList():
		g.generateDeserializeListElement(out, ttype.(*sema.List), prefix)
	}

	g.scopeDown(out)

	// Read container end.
	switch {
	case ttype.IsMap():
		out.WriteString(g.indent() + "$xfer += $input->readMapEnd();\n")
	case ttype.IsSet():
		out.WriteString(g.indent() + "$xfer += $input->readSetEnd();\n")
	case ttype.IsList():
		out.WriteString(g.indent() + "$xfer += $input->readListEnd();\n")
	}

	g.scopeDown(out)
}

// generateDeserializeMapElement generates code to deserialize a map.
func (g *Generator) generateDeserializeMapElement(out *strings.Builder, tmap *sema.Map, prefix string) {
	key := g.tmp("key")
	val := g.tmp("val")
	fkey := sema.NewField(tmap.KeyType(), key, 0)
	fval := sema.NewField(tmap.ValType(), val, 0)

	out.WriteString(g.indent() + declareField(fkey, true, true) + "\n")
	out.WriteString(g.indent() + declareField(fval, true, true) + "\n")

	g.generateDeserializeField(out, fkey, "", false)
	g.generateDeserializeField(out, fval, "", false)

	out.WriteString(g.indent() + "$" + prefix + "->{$" + key + "} = $" + val + ";\n")
}

func (g *Generator) generateDeserializeSetElement(out *strings.Builder, tset *sema.Set, prefix string) {
	elem := g.tmp("elem")
	felem := sema.NewField(tset.ElemType(), elem, 0)

	out.WriteString(g.indent() + "my $" + elem + " = undef;\n")

	g.generateDeserializeField(out, felem, "", false)

	out.WriteString(g.indent() + "$" + prefix + "->{$" + elem + "} = 1;\n")
}

func (g *Generator) generateDeserializeListElement(out *strings.Builder, tlist *sema.List, prefix string) {
	elem := g.tmp("elem")
	felem := sema.NewField(tlist.ElemType(), elem, 0)

	out.WriteString(g.indent() + "my $" + elem + " = undef;\n")

	g.generateDeserializeField(out, felem, "", false)

	out.WriteString(g.indent() + "push(@{$" + prefix + "},$" + elem + ");\n")
}

// generateSerializeField serializes a field of any type.
func (g *Generator) generateSerializeField(out *strings.Builder, tfield *sema.Field, prefix string) {
	t := sema.TrueType(tfield.Type())

	// Do nothing for void types.
	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE SERIALIZE CODE FOR void TYPE: %s%s", prefix, tfield.Name())
	}

	switch {
	case t.IsStruct() || t.IsXception():
		g.generateSerializeStruct(out, t.(*sema.Struct), prefix+"{"+tfield.Name()+"}")
	case t.IsContainer():
		g.generateSerializeContainer(out, t, prefix+"{"+tfield.Name()+"}")
	case t.IsBaseType() || t.IsEnum():
		name := tfield.Name()

		// Hack for when prefix is defined (always a hash ref).
		if prefix != "" {
			name = prefix + "{" + tfield.Name() + "}"
		}

		out.WriteString(g.indent() + "$xfer += $output->")

		if t.IsBaseType() {
			switch t.(*sema.BaseType).Base() {
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
			default:
				emit.Throw("compiler error: no PERL name for base type %s", sema.BaseName(t.(*sema.BaseType).Base()))
			}
		} else if t.IsEnum() {
			out.WriteString("writeI32($" + name + ");")
		}
		out.WriteString("\n")
	default:
		// DO NOT KNOW HOW TO SERIALIZE FIELD: unreachable for a
		// well-formed program; see generateDeserializeField.
	}
}

// generateSerializeStruct serializes all the members of a struct.
func (g *Generator) generateSerializeStruct(out *strings.Builder, tstruct *sema.Struct, prefix string) {
	_ = tstruct
	out.WriteString(g.indent() + "$xfer += $" + prefix + "->write($output);\n")
}

// generateSerializeContainer writes out a container.
func (g *Generator) generateSerializeContainer(out *strings.Builder, ttype sema.Type, prefix string) {
	g.scopeUp(out)

	switch {
	case ttype.IsMap():
		m := ttype.(*sema.Map)
		out.WriteString(g.indent() + "$xfer += $output->writeMapBegin(" +
			typeToEnum(m.KeyType()) + ", " + typeToEnum(m.ValType()) + ", " +
			"scalar(keys %{$" + prefix + "}));\n")
	case ttype.IsSet():
		s := ttype.(*sema.Set)
		out.WriteString(g.indent() + "$xfer += $output->writeSetBegin(" +
			typeToEnum(s.ElemType()) + ", " + "scalar(@{$" + prefix + "}));\n")
	case ttype.IsList():
		l := ttype.(*sema.List)
		out.WriteString(g.indent() + "$xfer += $output->writeListBegin(" +
			typeToEnum(l.ElemType()) + ", " + "scalar(@{$" + prefix + "}));\n")
	}

	g.scopeUp(out)

	switch {
	case ttype.IsMap():
		m := ttype.(*sema.Map)
		kiter := g.tmp("kiter")
		viter := g.tmp("viter")
		out.WriteString(g.indent() + "while( my ($" + kiter + ",$" + viter + ") = each %{$" + prefix + "}) \n")

		g.scopeUp(out)
		g.generateSerializeMapElement(out, m, kiter, viter)
		g.scopeDown(out)

	case ttype.IsSet():
		s := ttype.(*sema.Set)
		iter := g.tmp("iter")
		out.WriteString(g.indent() + "foreach my $" + iter + " (@{$" + prefix + "})\n")
		g.scopeUp(out)
		g.generateSerializeSetElement(out, s, iter)
		g.scopeDown(out)

	case ttype.IsList():
		l := ttype.(*sema.List)
		iter := g.tmp("iter")
		out.WriteString(g.indent() + "foreach my $" + iter + " (@{$" + prefix + "}) \n")
		g.scopeUp(out)
		g.generateSerializeListElement(out, l, iter)
		g.scopeDown(out)
	}

	g.scopeDown(out)

	switch {
	case ttype.IsMap():
		out.WriteString(g.indent() + "$xfer += $output->writeMapEnd();\n")
	case ttype.IsSet():
		out.WriteString(g.indent() + "$xfer += $output->writeSetEnd();\n")
	case ttype.IsList():
		out.WriteString(g.indent() + "$xfer += $output->writeListEnd();\n")
	}

	g.scopeDown(out)
}

// generateSerializeMapElement serializes the members of a map.
func (g *Generator) generateSerializeMapElement(out *strings.Builder, tmap *sema.Map, kiter, viter string) {
	kfield := sema.NewField(tmap.KeyType(), kiter, 0)
	g.generateSerializeField(out, kfield, "")

	vfield := sema.NewField(tmap.ValType(), viter, 0)
	g.generateSerializeField(out, vfield, "")
}

// generateSerializeSetElement serializes the members of a set.
func (g *Generator) generateSerializeSetElement(out *strings.Builder, tset *sema.Set, iter string) {
	efield := sema.NewField(tset.ElemType(), iter, 0)
	g.generateSerializeField(out, efield, "")
}

// generateSerializeListElement serializes the members of a list.
func (g *Generator) generateSerializeListElement(out *strings.Builder, tlist *sema.List, iter string) {
	efield := sema.NewField(tlist.ElemType(), iter, 0)
	g.generateSerializeField(out, efield, "")
}
