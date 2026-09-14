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

package java

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

func (g *Generator) generateStruct(s *sema.Struct) {
	if s.IsUnion() {
		g.generateJavaUnion(s)
	} else {
		g.generateJavaStruct(s, false)
	}
}

func (g *Generator) generateXception(s *sema.Struct) {
	g.generateJavaStruct(s, true)
}

// generateJavaStruct is generate_java_struct: one file per struct.
func (g *Generator) generateJavaStruct(s *sema.Struct, isException bool) {
	fStructName := g.packageDir + "/" + makeValidJavaFilename(s.Name()) + ".java"
	var f strings.Builder

	f.WriteString(autogenComment() + g.javaPackage())

	g.generateJavaStructDefinition(&f, s, isException, false, false)
	emit.WriteFile(fStructName, f.String())
}

// generateJavaStructDefinition is generate_java_struct_definition.
func (g *Generator) generateJavaStructDefinition(out *strings.Builder, s *sema.Struct, isException, inClass, isResult bool) {
	g.javaDoc(out, s)
	out.WriteString(g.indent() + javaSuppressions())

	isFinal := s.Annotations().Has("final")
	deprecated := isDeprecated(s.Annotations())

	if !inClass && !g.opts.SuppressGeneratedAnnotation {
		g.generateJavaxGeneratedAnnotation(out)
	}

	if deprecated {
		out.WriteString(g.indent() + "@Deprecated\n")
	}
	out.WriteString(g.indent() + "public ")
	if isFinal {
		out.WriteString("final ")
	}
	if inClass {
		out.WriteString("static ")
	}
	out.WriteString("class " + makeValidJavaIdentifier(s.Name()) + " ")

	if isException {
		out.WriteString("extends org.apache.thrift.TException ")
	}
	out.WriteString("implements org.apache.thrift.TBase<" + makeValidJavaIdentifier(s.Name()) +
		", " + makeValidJavaIdentifier(s.Name()) +
		"._Fields>, java.io.Serializable, Cloneable, Comparable<" + makeValidJavaIdentifier(s.Name()) + ">")

	if g.opts.AndroidStyle {
		out.WriteString(", android.os.Parcelable")
	}

	out.WriteString(" ")

	g.scopeUp(out)

	g.generateStructDesc(out, s)

	members := s.Members()

	out.WriteString("\n")

	g.generateFieldDescs(out, s)

	out.WriteString("\n")

	g.generateSchemeMap(out, s)

	out.WriteString("\n")

	for _, m := range members {
		if g.opts.BeanStyle || g.opts.PrivateMembers {
			out.WriteString(g.indent() + "private ")
		} else {
			g.javaDocField(out, m)
			out.WriteString(g.indent() + "public ")
		}
		out.WriteString(g.declareField(m, false, true) + "\n")
	}

	out.WriteString("\n")

	if g.opts.AndroidStyle {
		g.generateJavaStructParcelable(out, s)
	}

	g.generateFieldNameConstants(out, s)

	// isset data
	if len(members) > 0 {
		out.WriteString("\n")

		out.WriteString(g.indent() + "// isset id assignments\n")

		i := 0
		optionals := 0
		for _, m := range members {
			if m.Req() == sema.Optional {
				optionals++
			}
			if !typeCanBeNull(m.Type()) {
				out.WriteString(g.indent() + "private static final int " + issetFieldID(m) + " = " + itoa(int64(i)) + ";\n")
				i++
			}
		}

		switch kind, primitive := g.needsIsset(s); kind {
		case issetNone:
		case issetPrimitive:
			out.WriteString(g.indent() + "private " + primitive + " __isset_bitfield = 0;\n")
		case issetBitset:
			out.WriteString(g.indent() + "private java.util.BitSet __isset_bit_vector = new java.util.BitSet(" + itoa(int64(i)) + ");\n")
		}

		if optionals > 0 {
			outputString := "private static final _Fields[] optionals = {"
			for _, m := range members {
				if m.Req() == sema.Optional {
					outputString = outputString + "_Fields." + constantName(m.Name()) + ","
				}
			}
			out.WriteString(g.indent() + outputString[:len(outputString)-1] + "};\n")
		}
	}

	g.generateJavaMetaDataMap(out, s)

	allOptionalMembers := true

	// Default constructor
	out.WriteString(g.indent() + "public " + makeValidJavaIdentifier(s.Name()) + "() {\n")
	g.indentUp()
	for _, m := range members {
		t := trueType(m.Type())
		if m.Value() != nil {
			g.printConstValue(out, "this."+m.Name(), t, m.Value(), true, true)
		}
		if m.Req() != sema.Optional {
			allOptionalMembers = false
		}
	}
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	if len(members) > 0 && !allOptionalMembers {
		// Full constructor for all fields
		out.WriteString(g.indent() + "public " + makeValidJavaIdentifier(s.Name()) + "(\n")
		g.indentUp()
		first := true
		for _, m := range members {
			if m.Req() != sema.Optional {
				if !first {
					out.WriteString(",\n")
				}
				first = false
				out.WriteString(g.indent() + g.tn(m.Type()) + " " + makeValidJavaIdentifier(m.Name()))
			}
		}
		out.WriteString(")\n")
		g.indentDown()
		out.WriteString(g.indent() + "{\n")
		g.indentUp()
		out.WriteString(g.indent() + "this();\n")
		for _, m := range members {
			if m.Req() != sema.Optional {
				t := trueType(m.Type())
				id := makeValidJavaIdentifier(m.Name())
				if t.IsBinary() {
					if g.opts.UnsafeBinaries {
						out.WriteString(g.indent() + "this." + id + " = " + id + ";\n")
					} else {
						out.WriteString(g.indent() + "this." + id + " = org.apache.thrift.TBaseHelper.copyBinary(" + id + ");\n")
					}
				} else {
					out.WriteString(g.indent() + "this." + id + " = " + id + ";\n")
				}
				g.issetSet(out, m, "")
			}
		}
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
	}

	// copy constructor
	out.WriteString(g.indent() + "/**\n")
	out.WriteString(g.indent() + " * Performs a deep copy on <i>other</i>.\n")
	out.WriteString(g.indent() + " */\n")
	out.WriteString(g.indent() + "public " + makeValidJavaIdentifier(s.Name()) + "(" + makeValidJavaIdentifier(s.Name()) + " other) {\n")
	g.indentUp()

	switch kind, _ := g.needsIsset(s); kind {
	case issetNone:
	case issetPrimitive:
		out.WriteString(g.indent() + "__isset_bitfield = other.__isset_bitfield;\n")
	case issetBitset:
		out.WriteString(g.indent() + "__isset_bit_vector.clear();\n")
		out.WriteString(g.indent() + "__isset_bit_vector.or(other.__isset_bit_vector);\n")
	}

	for _, m := range members {
		fieldName := m.Name()
		t := trueType(m.Type())
		canBeNull := typeCanBeNull(t)

		if canBeNull {
			out.WriteString(g.indent() + "if (other." + g.issetCheck(m) + ") {\n")
			g.indentUp()
		}

		if t.IsContainer() {
			g.generateDeepCopyContainer(out, "other", fieldName, "__this__"+fieldName, t)
			out.WriteString(g.indent() + "this." + makeValidJavaIdentifier(fieldName) + " = __this__" + fieldName + ";\n")
		} else {
			out.WriteString(g.indent() + "this." + makeValidJavaIdentifier(fieldName) + " = ")
			g.generateDeepCopyNonContainer(out, "other."+makeValidJavaIdentifier(fieldName), fieldName, t)
			out.WriteString(";\n")
		}

		if canBeNull {
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	// clone method
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public " + makeValidJavaIdentifier(s.Name()) + " deepCopy() {\n")
	out.WriteString(g.indent() + "  return new " + makeValidJavaIdentifier(s.Name()) + "(this);\n")
	out.WriteString(g.indent() + "}\n\n")

	g.generateJavaStructClear(out, s)

	g.generateJavaBeanBoilerplate(out, s)
	g.generateGenericFieldGettersSetters(out, s)
	g.generateGenericIssetMethod(out, s)

	g.generateJavaStructEquality(out, s)
	g.generateJavaStructCompareTo(out, s)
	g.generateJavaStructFieldByID(out, s)

	g.generateJavaStructReader(out, s)
	if isResult {
		g.generateJavaStructResultWriter(out, s)
	} else {
		g.generateJavaStructWriter(out, s)
	}
	g.generateJavaStructTostring(out, s)
	g.generateJavaValidator(out, s)

	g.generateJavaStructWriteObject(out, s)
	g.generateJavaStructReadObject(out, s)

	g.generateJavaStructStandardScheme(out, s, isResult)
	g.generateJavaStructTupleScheme(out, s)
	g.generateJavaSchemeLookup(out)

	g.scopeDown(out)
	out.WriteString("\n")
}

// generateJavaStructParcelable is generate_java_struct_parcelable.
func (g *Generator) generateJavaStructParcelable(out *strings.Builder, s *sema.Struct) {
	tname := s.Name()
	members := s.Members()

	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n" +
		g.indent() + "public void writeToParcel(android.os.Parcel out, int flags) {\n")
	g.indentUp()
	kind, bitsetPrimitiveType := g.needsIsset(s)
	switch kind {
	case issetNone:
	case issetPrimitive:
		out.WriteString(g.indent() + "//primitive bitfield of type: " + bitsetPrimitiveType + "\n")
		switch bitsetPrimitiveType {
		case "byte":
			out.WriteString(g.indent() + "out.writeByte(__isset_bitfield);\n")
		case "short":
			out.WriteString(g.indent() + "out.writeInt(new Short(__isset_bitfield).intValue());\n")
		case "int":
			out.WriteString(g.indent() + "out.writeInt(__isset_bitfield);\n")
		case "long":
			out.WriteString(g.indent() + "out.writeLong(__isset_bitfield);\n")
		}
		out.WriteString("\n")
	case issetBitset:
		out.WriteString(g.indent() + "//BitSet\n")
		out.WriteString(g.indent() + "out.writeSerializable(__isset_bit_vector);\n")
		out.WriteString("\n")
	}
	for _, m := range members {
		t := trueType(m.Type())
		name := m.Name()

		switch {
		case t.IsStruct():
			out.WriteString(g.indent() + "out.writeParcelable(" + name + ", flags);\n")
		case g.tn(t) == "float":
			out.WriteString(g.indent() + "out.writeFloat(" + name + ");\n")
		case t.IsEnum():
			out.WriteString(g.indent() + "out.writeInt(" + name + " != null ? " + name + ".getValue() : -1);\n")
		case t.IsList():
			if trueType(t.(*sema.List).ElemType()).IsStruct() {
				out.WriteString(g.indent() + "out.writeTypedList(" + name + ");\n")
			} else {
				out.WriteString(g.indent() + "out.writeList(" + name + ");\n")
			}
		case t.IsMap():
			out.WriteString(g.indent() + "out.writeMap(" + name + ");\n")
		case t.IsBaseType():
			if t.IsBinary() {
				out.WriteString(g.indent() + "out.writeInt(" + name + "!=null ? 1 : 0);\n")
				out.WriteString(g.indent() + "if(" + name + " != null) { \n")
				g.indentUp()
				out.WriteString(g.indent() + "out.writeByteArray(" + name + ".array(), " + name + ".position() + " +
					name + ".arrayOffset(), " + name + ".limit() - " + name + ".position() );\n")
				g.scopeDown(out)
			} else {
				switch baseOf(t) {
				case sema.TypeI16:
					out.WriteString(g.indent() + "out.writeInt(new Short(" + name + ").intValue());\n")
				case sema.TypeUUID:
					out.WriteString(g.indent() + "out.writeUuid(" + name + ");\n")
				case sema.TypeI32:
					out.WriteString(g.indent() + "out.writeInt(" + name + ");\n")
				case sema.TypeI64:
					out.WriteString(g.indent() + "out.writeLong(" + name + ");\n")
				case sema.TypeBool:
					out.WriteString(g.indent() + "out.writeInt(" + name + " ? 1 : 0);\n")
				case sema.TypeI8:
					out.WriteString(g.indent() + "out.writeByte(" + name + ");\n")
				case sema.TypeDouble:
					out.WriteString(g.indent() + "out.writeDouble(" + name + ");\n")
				case sema.TypeString:
					out.WriteString(g.indent() + "out.writeString(" + name + ");\n")
				case sema.TypeVoid:
				default:
					emit.Throw("compiler error: unhandled type")
				}
			}
		}
	}
	g.scopeDown(out)
	out.WriteString("\n")

	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n" +
		g.indent() + "public int describeContents() {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return 0;\n")
	g.scopeDown(out)
	out.WriteString("\n")

	out.WriteString(g.indent() + "public " + tname + "(android.os.Parcel in) {\n")
	g.indentUp()
	switch kind {
	case issetNone:
	case issetPrimitive:
		out.WriteString(g.indent() + "//primitive bitfield of type: " + bitsetPrimitiveType + "\n")
		switch bitsetPrimitiveType {
		case "byte":
			out.WriteString(g.indent() + "__isset_bitfield = in.readByte();\n")
		case "short":
			out.WriteString(g.indent() + "__isset_bitfield = (short) in.readInt();\n")
		case "int":
			out.WriteString(g.indent() + "__isset_bitfield = in.readInt();\n")
		case "long":
			out.WriteString(g.indent() + "__isset_bitfield = in.readLong();\n")
		}
		out.WriteString("\n")
	case issetBitset:
		out.WriteString(g.indent() + "//BitSet\n")
		out.WriteString(g.indent() + "__isset_bit_vector = (java.util.BitSet) in.readSerializable();\n")
		out.WriteString("\n")
	}
	for _, m := range members {
		t := trueType(m.Type())
		name := m.Name()
		prefix := "this." + name

		switch {
		case t.IsStruct():
			out.WriteString(g.indent() + prefix + "= in.readParcelable(" + tname + ".class.getClassLoader());\n")
		case t.IsEnum():
			out.WriteString(g.indent() + prefix + " = " + g.tn(t) + ".findByValue(in.readInt());\n")
		case t.IsList():
			l := t.(*sema.List)
			out.WriteString(g.indent() + prefix + " = new " + g.typeName(t, false, true, false, false) + "();\n")
			if trueType(l.ElemType()).IsStruct() {
				out.WriteString(g.indent() + "in.readTypedList(" + prefix + ", " + g.tn(l.ElemType()) + ".CREATOR);\n")
			} else {
				out.WriteString(g.indent() + "in.readList(" + prefix + ", " + tname + ".class.getClassLoader());\n")
			}
		case t.IsMap():
			out.WriteString(g.indent() + prefix + " = new " + g.typeName(t, false, true, false, false) + "();\n")
			out.WriteString(g.indent() + " in.readMap(" + prefix + ", " + tname + ".class.getClassLoader());\n")
		case g.tn(t) == "float":
			out.WriteString(g.indent() + prefix + " = in.readFloat();\n")
		case t.IsBaseType():
			if t.IsBinary() {
				out.WriteString(g.indent() + "if(in.readInt()==1) {\n")
				g.indentUp()
				out.WriteString(g.indent() + prefix + " = java.nio.ByteBuffer.wrap(in.createByteArray());\n")
				g.scopeDown(out)
			} else {
				switch baseOf(t) {
				case sema.TypeI8:
					out.WriteString(g.indent() + prefix + " = in.readByte();\n")
				case sema.TypeI16:
					out.WriteString(g.indent() + prefix + " = (short) in.readInt();\n")
				case sema.TypeI32:
					out.WriteString(g.indent() + prefix + " = in.readInt();\n")
				case sema.TypeI64:
					out.WriteString(g.indent() + prefix + " = in.readLong();\n")
				case sema.TypeUUID:
					out.WriteString(g.indent() + prefix + " = in.readUuid();\n")
				case sema.TypeBool:
					out.WriteString(g.indent() + prefix + " = (in.readInt()==1);\n")
				case sema.TypeDouble:
					out.WriteString(g.indent() + prefix + " = in.readDouble();\n")
				case sema.TypeString:
					out.WriteString(g.indent() + prefix + "= in.readString();\n")
				case sema.TypeVoid:
				default:
					emit.Throw("compiler error: unhandled type")
				}
			}
		}
	}

	g.scopeDown(out)
	out.WriteString("\n")

	out.WriteString(g.indent() + "public static final android.os.Parcelable.Creator<" + tname +
		"> CREATOR = new android.os.Parcelable.Creator<" + tname + ">() {\n")
	g.indentUp()

	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n" +
		g.indent() + "public " + tname + "[] newArray(int size) {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return new " + tname + "[size];\n")
	g.scopeDown(out)
	out.WriteString("\n")

	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n" +
		g.indent() + "public " + tname + " createFromParcel(android.os.Parcel in) {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return new " + tname + "(in);\n")
	g.scopeDown(out)

	g.indentDown()
	out.WriteString(g.indent() + "};\n")
	out.WriteString("\n")
}

// generateJavaStructEquality is generate_java_struct_equality.
func (g *Generator) generateJavaStructEquality(out *strings.Builder, s *sema.Struct) {
	id := makeValidJavaIdentifier(s.Name())
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n" +
		g.indent() + "public boolean equals(java.lang.Object that) {\n")
	g.indentUp()
	out.WriteString(g.indent() + "if (that instanceof " + id + ")\n" +
		g.indent() + "  return this.equals((" + id + ")that);\n" +
		g.indent() + "return false;\n")
	g.scopeDown(out)
	out.WriteString("\n")

	out.WriteString(g.indent() + "public boolean equals(" + id + " that) {\n")
	g.indentUp()
	out.WriteString(g.indent() + "if (that == null)\n" +
		g.indent() + "  return false;\n" +
		g.indent() + "if (this == that)\n" +
		g.indent() + "  return true;\n")

	members := s.Members()
	for _, m := range members {
		out.WriteString("\n")

		t := trueType(m.Type())
		isOptional := m.Req() == sema.Optional
		canBeNull := typeCanBeNull(t)
		name := m.Name()

		thisPresent := "true"
		thatPresent := "true"
		var unequal string

		if isOptional || canBeNull {
			thisPresent += " && this." + g.issetCheck(m)
			thatPresent += " && that." + g.issetCheck(m)
		}

		out.WriteString(g.indent() + "boolean this_present_" + name + " = " + thisPresent + ";\n" +
			g.indent() + "boolean that_present_" + name + " = " + thatPresent + ";\n" +
			g.indent() + "if (" + "this_present_" + name + " || that_present_" + name + ") {\n")
		g.indentUp()
		out.WriteString(g.indent() + "if (!(" + "this_present_" + name + " && that_present_" + name + "))\n" +
			g.indent() + "  return false;\n")

		nid := makeValidJavaIdentifier(name)
		if t.IsBinary() {
			unequal = "!this." + nid + ".equals(that." + nid + ")"
		} else if canBeNull {
			unequal = "!this." + nid + ".equals(that." + nid + ")"
		} else {
			unequal = "this." + nid + " != that." + nid
		}

		out.WriteString(g.indent() + "if (" + unequal + ")\n" + g.indent() + "  return false;\n")

		g.scopeDown(out)
	}
	out.WriteString("\n")
	out.WriteString(g.indent() + "return true;\n")
	g.scopeDown(out)
	out.WriteString("\n")

	const mul = "8191"
	const bYes = "131071"
	const bNo = "524287"
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n" +
		g.indent() + "public int hashCode() {\n")
	g.indentUp()
	out.WriteString(g.indent() + "int hashCode = 1;\n")

	for _, m := range members {
		out.WriteString("\n")

		t := trueType(m.Type())
		isOptional := m.Req() == sema.Optional
		canBeNull := typeCanBeNull(t)
		name := makeValidJavaIdentifier(m.Name())

		if isOptional || canBeNull {
			out.WriteString(g.indent() + "hashCode = hashCode * " + mul + " + ((" + g.issetCheck(m) + ") ? " + bYes + " : " + bNo + ");\n")
		}

		if isOptional || canBeNull {
			out.WriteString(g.indent() + "if (" + g.issetCheck(m) + ")\n")
			g.indentUp()
		}

		if t.IsEnum() {
			out.WriteString(g.indent() + "hashCode = hashCode * " + mul + " + " + name + ".getValue();\n")
		} else if t.IsBaseType() {
			switch baseOf(t) {
			case sema.TypeString, sema.TypeUUID:
				out.WriteString(g.indent() + "hashCode = hashCode * " + mul + " + " + name + ".hashCode();\n")
			case sema.TypeBool:
				out.WriteString(g.indent() + "hashCode = hashCode * " + mul + " + ((" + name + ") ? " + bYes + " : " + bNo + ");\n")
			case sema.TypeI8:
				out.WriteString(g.indent() + "hashCode = hashCode * " + mul + " + (int) (" + name + ");\n")
			case sema.TypeI16, sema.TypeI32:
				out.WriteString(g.indent() + "hashCode = hashCode * " + mul + " + " + name + ";\n")
			case sema.TypeI64, sema.TypeDouble:
				out.WriteString(g.indent() + "hashCode = hashCode * " + mul + " + org.apache.thrift.TBaseHelper.hashCode(" + name + ");\n")
			case sema.TypeVoid:
				emit.Throw("compiler error: a struct field cannot be void")
			default:
				emit.Throw("compiler error: the following base type has no hashcode generator: %s", t.Name())
			}
		} else {
			out.WriteString(g.indent() + "hashCode = hashCode * " + mul + " + " + name + ".hashCode();\n")
		}

		if isOptional || canBeNull {
			g.indentDown()
		}
	}

	out.WriteString("\n")
	out.WriteString(g.indent() + "return hashCode;\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

func (g *Generator) generateJavaStructCompareTo(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public int compareTo(" + g.tn(s) + " other) {\n")
	g.indentUp()

	out.WriteString(g.indent() + "if (!getClass().equals(other.getClass())) {\n")
	out.WriteString(g.indent() + "  return getClass().getName().compareTo(other.getClass().getName());\n")
	out.WriteString(g.indent() + "}\n")
	out.WriteString("\n")

	out.WriteString(g.indent() + "int lastComparison = 0;\n")
	out.WriteString("\n")

	for _, m := range s.Members() {
		out.WriteString(g.indent() + "lastComparison = java.lang.Boolean.compare(" + g.issetCheck(m) + ", other." + g.issetCheck(m) + ");\n")
		out.WriteString(g.indent() + "if (lastComparison != 0) {\n")
		out.WriteString(g.indent() + "  return lastComparison;\n")
		out.WriteString(g.indent() + "}\n")

		out.WriteString(g.indent() + "if (" + g.issetCheck(m) + ") {\n")
		out.WriteString(g.indent() + "  lastComparison = org.apache.thrift.TBaseHelper.compareTo(this." +
			makeValidJavaIdentifier(m.Name()) + ", other." + makeValidJavaIdentifier(m.Name()) + ");\n")
		out.WriteString(g.indent() + "  if (lastComparison != 0) {\n")
		out.WriteString(g.indent() + "    return lastComparison;\n")
		out.WriteString(g.indent() + "  }\n")
		out.WriteString(g.indent() + "}\n")
	}

	out.WriteString(g.indent() + "return 0;\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

func (g *Generator) generateJavaStructReader(out *strings.Builder, s *sema.Struct) {
	_ = s
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public void read(org.apache.thrift.protocol.TProtocol iprot) throws org.apache.thrift.TException {\n")
	g.indentUp()
	out.WriteString(g.indent() + "scheme(iprot).read(iprot, this);\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

func (g *Generator) generateJavaValidator(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "public void validate() throws org.apache.thrift.TException {\n")
	g.indentUp()

	fields := s.Members()

	out.WriteString(g.indent() + "// check for required fields\n")
	for _, f := range fields {
		if f.Req() == sema.Required {
			if g.opts.BeanStyle {
				out.WriteString(g.indent() + "if (!" + g.issetCheck(f) + ") {\n" +
					g.indent() + "  throw new org.apache.thrift.protocol.TProtocolException(\"Required field '" +
					f.Name() + "' is unset! Struct:\" + toString());\n" +
					g.indent() + "}\n" + "\n")
			} else {
				if typeCanBeNull(f.Type()) {
					out.WriteString(g.indent() + "if (" + f.Name() + " == null) {\n")
					out.WriteString(g.indent() + "  throw new org.apache.thrift.protocol.TProtocolException(\"Required field '" +
						f.Name() + "' was not present! Struct: \" + toString());\n")
					out.WriteString(g.indent() + "}\n")
				} else {
					out.WriteString(g.indent() + "// alas, we cannot check '" + f.Name() +
						"' because it's a primitive and you chose the non-beans generator.\n")
				}
			}
		}
	}

	out.WriteString(g.indent() + "// check for sub-struct validity\n")
	for _, f := range fields {
		t := trueType(f.Type())
		if t.IsStruct() && !t.(*sema.Struct).IsUnion() {
			id := makeValidJavaIdentifier(f.Name())
			out.WriteString(g.indent() + "if (" + id + " != null) {\n")
			out.WriteString(g.indent() + "  " + id + ".validate();\n")
			out.WriteString(g.indent() + "}\n")
		}
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

func (g *Generator) generateJavaStructWriter(out *strings.Builder, s *sema.Struct) {
	_ = s
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public void write(org.apache.thrift.protocol.TProtocol oprot) throws org.apache.thrift.TException {\n")
	g.indentUp()
	out.WriteString(g.indent() + "scheme(oprot).write(oprot, this);\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

func (g *Generator) generateJavaStructResultWriter(out *strings.Builder, s *sema.Struct) {
	_ = s
	out.WriteString(g.indent() + "public void write(org.apache.thrift.protocol.TProtocol oprot) throws org.apache.thrift.TException {\n")
	g.indentUp()
	out.WriteString(g.indent() + "scheme(oprot).write(oprot, this);\n")

	g.indentDown()
	out.WriteString(g.indent() + "  }\n\n")
}

func (g *Generator) generateJavaStructFieldByID(out *strings.Builder, s *sema.Struct) {
	_ = s
	out.WriteString(g.indent() + javaNullableAnnotation() + "\n")
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public _Fields fieldForId(int fieldId) {\n")
	out.WriteString(g.indent() + "  return _Fields.findByThriftId(fieldId);\n")
	out.WriteString(g.indent() + "}\n\n")
}

func (g *Generator) generateReflectionGetters(out *strings.Builder, t sema.Type, fieldName, capName string) {
	out.WriteString(g.indent() + "case " + constantName(fieldName) + ":\n")
	g.indentUp()
	getter := "get"
	if t.IsBool() {
		getter = "is"
	}
	out.WriteString(g.indent() + "return " + getter + capName + "();\n\n")
	g.indentDown()
}

func (g *Generator) generateReflectionSetters(out *strings.Builder, t sema.Type, fieldName, capName string) {
	isBinary := t.IsBinary()
	out.WriteString(g.indent() + "case " + constantName(fieldName) + ":\n")
	g.indentUp()
	out.WriteString(g.indent() + "if (value == null) {\n")
	out.WriteString(g.indent() + "  unset" + g.capName(fieldName) + "();\n")
	out.WriteString(g.indent() + "} else {\n")
	if isBinary {
		g.indentUp()
		out.WriteString(g.indent() + "if (value instanceof byte[]) {\n")
		out.WriteString(g.indent() + "  set" + capName + "((byte[])value);\n")
		out.WriteString(g.indent() + "} else {\n")
	}
	out.WriteString(g.indent() + "  set" + capName + "((" + g.typeName(t, true, false, false, false) + ")value);\n")
	if isBinary {
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
	}
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "break;\n\n")

	g.indentDown()
}

func (g *Generator) generateGenericFieldGettersSetters(out *strings.Builder, s *sema.Struct) {
	var getterStream, setterStream strings.Builder

	for _, f := range s.Members() {
		t := trueType(f.Type())
		fieldName := f.Name()
		capName := g.capName(fieldName)

		g.indentUp()
		g.generateReflectionSetters(&setterStream, t, fieldName, capName)
		g.generateReflectionGetters(&getterStream, t, fieldName, capName)
		g.indentDown()
	}

	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public void setFieldValue(_Fields field, " + javaNullableAnnotation() + " java.lang.Object value) {\n")
	out.WriteString(g.indent() + "  switch (field) {\n")
	out.WriteString(setterStream.String())
	out.WriteString(g.indent() + "  }\n")
	out.WriteString(g.indent() + "}\n\n")

	out.WriteString(g.indent() + javaNullableAnnotation() + "\n")
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public java.lang.Object getFieldValue(_Fields field) {\n")
	g.indentUp()
	out.WriteString(g.indent() + "switch (field) {\n")
	out.WriteString(getterStream.String())
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "throw new java.lang.IllegalStateException();\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

func (g *Generator) generateGenericIssetMethod(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "/** Returns true if field corresponding to fieldID is set (has been assigned a value) and false otherwise */\n")
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public boolean isSet(_Fields field) {\n")
	g.indentUp()
	out.WriteString(g.indent() + "if (field == null) {\n")
	out.WriteString(g.indent() + "  throw new java.lang.IllegalArgumentException();\n")
	out.WriteString(g.indent() + "}\n\n")

	out.WriteString(g.indent() + "switch (field) {\n")

	for _, f := range s.Members() {
		out.WriteString(g.indent() + "case " + constantName(f.Name()) + ":\n")
		g.indentUp()
		out.WriteString(g.indent() + "return " + g.issetCheck(f) + ";\n")
		g.indentDown()
	}

	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "throw new java.lang.IllegalStateException();\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateJavaBeanBoilerplate is generate_java_bean_boilerplate.
func (g *Generator) generateJavaBeanBoilerplate(out *strings.Builder, s *sema.Struct) {
	issetKind, _ := g.needsIsset(s)
	for _, f := range s.Members() {
		t := trueType(f.Type())
		fieldName := f.Name()
		capName := g.capName(fieldName)
		optional := g.opts.UseOptionType && f.Req() == sema.Optional
		deprecated := isDeprecated(f.Annotations())
		id := makeValidJavaIdentifier(fieldName)

		optionClass := thriftOptionClass
		if g.opts.UseJdk8OptionType {
			optionClass = jdkOptionClass
		}
		optionNone := optionClass + ".none()"
		optionSome := optionClass + ".some(this."
		if g.opts.UseJdk8OptionType {
			optionNone = optionClass + ".empty()"
			optionSome = optionClass + ".of(this."
		}

		if t.IsContainer() {
			if optional {
				if deprecated {
					out.WriteString(g.indent() + "@Deprecated\n")
				}
				out.WriteString(g.indent() + "public " + optionClass + "<Integer> get" + capName)
				out.WriteString(g.capName("size() {") + "\n")

				g.indentUp()
				out.WriteString(g.indent() + "if (this." + fieldName + " == null) {\n")
				g.indentUp()
				out.WriteString(g.indent() + "return " + optionNone + ";\n")
				g.indentDown()
				out.WriteString(g.indent() + "} else {\n")
				g.indentUp()
				out.WriteString(g.indent() + "return " + optionSome)
				out.WriteString(fieldName + ".size());\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n\n")
			} else {
				if deprecated {
					out.WriteString(g.indent() + "@Deprecated\n")
				}
				out.WriteString(g.indent() + "public int get" + capName)
				out.WriteString(g.capName("size() {") + "\n")

				g.indentUp()
				out.WriteString(g.indent() + "return (this." + fieldName + " == null) ? 0 : " + "this." + fieldName + ".size();\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n\n")
			}
		}

		if t.IsSet() || t.IsList() {
			var elementType sema.Type
			if t.IsSet() {
				elementType = t.(*sema.Set).ElemType()
			} else {
				elementType = t.(*sema.List).ElemType()
			}

			if optional {
				if deprecated {
					out.WriteString(g.indent() + "@Deprecated\n")
				}
				out.WriteString(g.indent() + "public " + optionClass + "<")
				out.WriteString("java.util.Iterator<" + g.typeName(elementType, true, false, false, false) + ">> get" + capName)
				out.WriteString(g.capName("iterator() {") + "\n")

				g.indentUp()
				out.WriteString(g.indent() + "if (this." + fieldName + " == null) {\n")
				g.indentUp()
				out.WriteString(g.indent() + "return " + optionNone + ";\n")
				g.indentDown()
				out.WriteString(g.indent() + "} else {\n")
				g.indentUp()
				out.WriteString(g.indent() + "return " + optionSome)
				out.WriteString(fieldName + ".iterator());\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n\n")
			} else {
				if deprecated {
					out.WriteString(g.indent() + "@Deprecated\n")
				}
				out.WriteString(g.indent() + javaNullableAnnotation() + "\n")
				out.WriteString(g.indent() + "public java.util.Iterator<" + g.typeName(elementType, true, false, false, false) + "> get" + capName)
				out.WriteString(g.capName("iterator() {") + "\n")

				g.indentUp()
				out.WriteString(g.indent() + "return (this." + fieldName + " == null) ? null : " + "this." + fieldName + ".iterator();\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n\n")
			}

			if deprecated {
				out.WriteString(g.indent() + "@Deprecated\n")
			}
			out.WriteString(g.indent() + "public void add" + g.capName("to"))
			out.WriteString(capName + "(" + g.tn(elementType) + " elem) {\n")

			g.indentUp()
			out.WriteString(g.indent() + "if (this." + fieldName + " == null) {\n")
			g.indentUp()
			out.WriteString(g.indent() + "this." + fieldName)
			if g.isEnumSet(t) {
				out.WriteString(" = " + g.typeName(t, false, true, true, false) + ".noneOf(" + g.innerEnumTypeName(t) + ");\n")
			} else {
				out.WriteString(" = new " + g.typeName(t, false, true, false, false) + "();\n")
			}
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
			out.WriteString(g.indent() + "this." + fieldName + ".add(elem);\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n\n")
		} else if t.IsMap() {
			m := t.(*sema.Map)
			if deprecated {
				out.WriteString(g.indent() + "@Deprecated\n")
			}
			out.WriteString(g.indent() + "public void put" + g.capName("to"))
			out.WriteString(capName + "(" + g.tn(m.KeyType()) + " key, " + g.tn(m.ValType()) + " val) {\n")

			g.indentUp()
			out.WriteString(g.indent() + "if (this." + fieldName + " == null) {\n")
			g.indentUp()
			constructorArgs := ""
			if g.isEnumMap(t) {
				constructorArgs = g.innerEnumTypeName(t)
			}
			out.WriteString(g.indent() + "this." + fieldName + " = new " + g.typeName(t, false, true, false, false) + "(" + constructorArgs + ");\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
			out.WriteString(g.indent() + "this." + fieldName + ".put(key, val);\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n\n")
		}

		// Simple getter
		g.javaDocField(out, f)
		if t.IsBinary() {
			if deprecated {
				out.WriteString(g.indent() + "@Deprecated\n")
			}
			out.WriteString(g.indent() + "public byte[] get" + capName + "() {\n")
			out.WriteString(g.indent() + "  set" + capName + "(org.apache.thrift.TBaseHelper.rightSize(" + fieldName + "));\n")
			out.WriteString(g.indent() + "  return " + fieldName + " == null ? null : " + fieldName + ".array();\n")
			out.WriteString(g.indent() + "}\n\n")

			out.WriteString(g.indent() + "public java.nio.ByteBuffer buffer" + g.capName("for") + capName + "() {\n")
			if g.opts.UnsafeBinaries {
				out.WriteString(g.indent() + "  return " + fieldName + ";\n")
			} else {
				out.WriteString(g.indent() + "  return org.apache.thrift.TBaseHelper.copyBinary(" + fieldName + ");\n")
			}
			out.WriteString(g.indent() + "}\n\n")
		} else {
			if optional {
				if deprecated {
					out.WriteString(g.indent() + "@Deprecated\n")
				}
				out.WriteString(g.indent() + "public " + optionClass + "<" + g.typeName(t, true, false, false, false) + ">")
				if t.IsBaseType() && baseOf(t) == sema.TypeBool {
					out.WriteString(" is")
				} else {
					out.WriteString(" get")
				}
				out.WriteString(capName + "() {\n")
				g.indentUp()

				out.WriteString(g.indent() + "if (this.isSet" + capName + "()) {\n")
				g.indentUp()
				out.WriteString(g.indent() + "return " + optionSome)
				out.WriteString(fieldName + ");\n")
				g.indentDown()
				out.WriteString(g.indent() + "} else {\n")
				g.indentUp()
				out.WriteString(g.indent() + "return " + optionNone + ";\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n\n")
			} else {
				if deprecated {
					out.WriteString(g.indent() + "@Deprecated\n")
				}
				if typeCanBeNull(t) {
					out.WriteString(g.indent() + javaNullableAnnotation() + "\n")
				}
				out.WriteString(g.indent() + "public " + g.tn(t))
				if t.IsBaseType() && baseOf(t) == sema.TypeBool {
					out.WriteString(" is")
				} else {
					out.WriteString(" get")
				}
				out.WriteString(capName + "() {\n")
				g.indentUp()
				out.WriteString(g.indent() + "return this." + id + ";\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n\n")
			}
		}

		// Simple setter
		g.javaDocField(out, f)
		if t.IsBinary() {
			if deprecated {
				out.WriteString(g.indent() + "@Deprecated\n")
			}
			out.WriteString(g.indent() + "public ")
			if g.opts.BeanStyle {
				out.WriteString("void")
			} else {
				out.WriteString(g.tn(s))
			}
			out.WriteString(" set" + capName + "(byte[] " + id + ") {\n")
			out.WriteString(g.indent() + "  this." + id + " = " + id + " == null ? (java.nio.ByteBuffer)null")

			if g.opts.UnsafeBinaries {
				out.WriteString(g.indent() + " : java.nio.ByteBuffer.wrap(" + id + ");\n")
			} else {
				out.WriteString(g.indent() + " : java.nio.ByteBuffer.wrap(" + id + ".clone());\n")
			}

			if !g.opts.BeanStyle {
				out.WriteString(g.indent() + "  return this;\n")
			}
			out.WriteString(g.indent() + "}\n\n")
		}
		if deprecated {
			out.WriteString(g.indent() + "@Deprecated\n")
		}
		out.WriteString(g.indent() + "public ")
		if g.opts.BeanStyle {
			out.WriteString("void")
		} else {
			out.WriteString(g.tn(s))
		}
		nullable := ""
		if typeCanBeNull(t) {
			nullable = javaNullableAnnotation() + " "
		}
		out.WriteString(" set" + capName + "(" + nullable + g.tn(t) + " " + id + ") {\n")
		g.indentUp()
		out.WriteString(g.indent() + "this." + id + " = ")
		if t.IsBinary() && !g.opts.UnsafeBinaries {
			out.WriteString("org.apache.thrift.TBaseHelper.copyBinary(" + id + ")")
		} else {
			out.WriteString(id)
		}
		out.WriteString(";\n")
		g.issetSet(out, f, "")
		if !g.opts.BeanStyle {
			out.WriteString(g.indent() + "return this;\n")
		}

		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")

		// Unsetter
		if deprecated {
			out.WriteString(g.indent() + "@Deprecated\n")
		}
		out.WriteString(g.indent() + "public void unset" + capName + "() {\n")
		g.indentUp()
		if typeCanBeNull(t) {
			out.WriteString(g.indent() + "this." + id + " = null;\n")
		} else if issetKind == issetPrimitive {
			out.WriteString(g.indent() + "__isset_bitfield = org.apache.thrift.EncodingUtils.clearBit(__isset_bitfield, " + issetFieldID(f) + ");\n")
		} else {
			out.WriteString(g.indent() + "__isset_bit_vector.clear(" + issetFieldID(f) + ");\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")

		// isSet method
		out.WriteString(g.indent() + "/** Returns true if field " + fieldName + " is set (has been assigned a value) and false otherwise */\n")
		if deprecated {
			out.WriteString(g.indent() + "@Deprecated\n")
		}
		out.WriteString(g.indent() + "public boolean is" + g.capName("set") + capName + "() {\n")
		g.indentUp()
		if typeCanBeNull(t) {
			out.WriteString(g.indent() + "return this." + id + " != null;\n")
		} else if issetKind == issetPrimitive {
			out.WriteString(g.indent() + "return org.apache.thrift.EncodingUtils.testBit(__isset_bitfield, " + issetFieldID(f) + ");\n")
		} else {
			out.WriteString(g.indent() + "return __isset_bit_vector.get(" + issetFieldID(f) + ");\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")

		if deprecated {
			out.WriteString(g.indent() + "@Deprecated\n")
		}
		out.WriteString(g.indent() + "public void set" + capName + g.capName("isSet") + "(boolean value) {\n")
		g.indentUp()
		if typeCanBeNull(t) {
			out.WriteString(g.indent() + "if (!value) {\n")
			out.WriteString(g.indent() + "  this." + id + " = null;\n")
			out.WriteString(g.indent() + "}\n")
		} else if issetKind == issetPrimitive {
			out.WriteString(g.indent() + "__isset_bitfield = org.apache.thrift.EncodingUtils.setBit(__isset_bitfield, " + issetFieldID(f) + ", value);\n")
		} else {
			out.WriteString(g.indent() + "__isset_bit_vector.set(" + issetFieldID(f) + ", value);\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
	}
}

func (g *Generator) generateJavaStructTostring(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n" +
		g.indent() + "public java.lang.String toString() {\n")
	g.indentUp()

	out.WriteString(g.indent() + "java.lang.StringBuilder sb = new java.lang.StringBuilder(\"" + s.Name() + "(\");\n")
	out.WriteString(g.indent() + "boolean first = true;\n\n")

	first := true
	for _, f := range s.Members() {
		couldBeUnset := f.Req() == sema.Optional
		if couldBeUnset {
			out.WriteString(g.indent() + "if (" + g.issetCheck(f) + ") {\n")
			g.indentUp()
		}

		if !first {
			out.WriteString(g.indent() + "if (!first) sb.append(\", \");\n")
		}
		out.WriteString(g.indent() + "sb.append(\"" + f.Name() + ":\");\n")
		canBeNull := typeCanBeNull(f.Type())
		id := makeValidJavaIdentifier(f.Name())
		if canBeNull {
			out.WriteString(g.indent() + "if (this." + id + " == null) {\n")
			out.WriteString(g.indent() + "  sb.append(\"null\");\n")
			out.WriteString(g.indent() + "} else {\n")
			g.indentUp()
		}

		ft := f.Type()
		if trueType(ft).IsBinary() {
			out.WriteString(g.indent() + "org.apache.thrift.TBaseHelper.toString(this." + id + ", sb);\n")
		} else if ft.IsSet() && trueType(ft.(*sema.Set).ElemType()).IsBinary() {
			out.WriteString(g.indent() + "org.apache.thrift.TBaseHelper.toString(this." + id + ", sb);\n")
		} else if ft.IsList() && trueType(ft.(*sema.List).ElemType()).IsBinary() {
			out.WriteString(g.indent() + "org.apache.thrift.TBaseHelper.toString(this." + id + ", sb);\n")
		} else {
			out.WriteString(g.indent() + "sb.append(this." + id + ");\n")
		}

		if canBeNull {
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
		out.WriteString(g.indent() + "first = false;\n")

		if couldBeUnset {
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
		first = false
	}
	out.WriteString(g.indent() + "sb.append(\")\");\n" + g.indent() + "return sb.toString();\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateJavaMetaDataMap is generate_java_meta_data_map.
func (g *Generator) generateJavaMetaDataMap(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "public static final java.util.Map<_Fields, org.apache.thrift.meta_data.FieldMetaData> metaDataMap;\n")
	out.WriteString(g.indent() + "static {\n")
	g.indentUp()

	out.WriteString(g.indent() + "java.util.Map<_Fields, org.apache.thrift.meta_data.FieldMetaData> tmpMap = new java.util.EnumMap<_Fields, org.apache.thrift.meta_data.FieldMetaData>(_Fields.class);\n")

	for _, f := range s.Members() {
		fieldName := f.Name()
		out.WriteString(g.indent() + "tmpMap.put(_Fields." + constantName(fieldName) +
			", new org.apache.thrift.meta_data.FieldMetaData(\"" + fieldName + "\", ")

		switch f.Req() {
		case sema.Required:
			out.WriteString("org.apache.thrift.TFieldRequirementType.REQUIRED, ")
		case sema.Optional:
			out.WriteString("org.apache.thrift.TFieldRequirementType.OPTIONAL, ")
		default:
			out.WriteString("org.apache.thrift.TFieldRequirementType.DEFAULT, ")
		}

		g.generateFieldValueMetaData(out, f.Type())

		if g.opts.AnnotationsAsMetadata {
			g.generateMetadataForFieldAnnotations(out, f)
		}
		out.WriteString("));\n")
	}

	out.WriteString(g.indent() + "metaDataMap = java.util.Collections.unmodifiableMap(tmpMap);\n")

	out.WriteString(g.indent() + "org.apache.thrift.meta_data.FieldMetaData.addStructMetaDataMap(" + g.tn(s) + ".class, metaDataMap);\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

func (g *Generator) javaTypeString(t sema.Type) string {
	switch {
	case t.IsList():
		return "org.apache.thrift.protocol.TType.LIST"
	case t.IsMap():
		return "org.apache.thrift.protocol.TType.MAP"
	case t.IsSet():
		return "org.apache.thrift.protocol.TType.SET"
	case t.IsStruct() || t.IsXception():
		return "org.apache.thrift.protocol.TType.STRUCT"
	case t.IsEnum():
		return "org.apache.thrift.protocol.TType.ENUM"
	case t.IsTypedef():
		return g.javaTypeString(t.(*sema.Typedef).Type())
	case t.IsBaseType():
		switch baseOf(t) {
		case sema.TypeVoid:
			return "org.apache.thrift.protocol.TType.VOID"
		case sema.TypeString:
			return "org.apache.thrift.protocol.TType.STRING"
		case sema.TypeUUID:
			return "org.apache.thrift.protocol.TType.UUID"
		case sema.TypeBool:
			return "org.apache.thrift.protocol.TType.BOOL"
		case sema.TypeI8:
			return "org.apache.thrift.protocol.TType.BYTE"
		case sema.TypeI16:
			return "org.apache.thrift.protocol.TType.I16"
		case sema.TypeI32:
			return "org.apache.thrift.protocol.TType.I32"
		case sema.TypeI64:
			return "org.apache.thrift.protocol.TType.I64"
		case sema.TypeDouble:
			return "org.apache.thrift.protocol.TType.DOUBLE"
		}
	}
	emit.Throw("Unknown thrift type \"%s\" passed to t_java_generator::get_java_type_string!", t.Name())
	return ""
}

func (g *Generator) generateMetadataForFieldAnnotations(out *strings.Builder, f *sema.Field) {
	fieldAnn := f.Annotations()
	typeAnn := f.Type().Annotations()
	if len(fieldAnn) == 0 && len(typeAnn) == 0 {
		return
	}
	out.WriteString(", \n")
	g.indentUp()
	g.indentUp()
	out.WriteString(g.indent() + "java.util.stream.Stream.<java.util.Map.Entry<java.lang.String, java.lang.String>>builder()\n")

	g.indentUp()
	g.indentUp()
	for _, key := range fieldAnn.Keys() {
		values := fieldAnn[key]
		out.WriteString(g.indent() + ".add(new java.util.AbstractMap.SimpleImmutableEntry<>(\"" + key + "\", \"" + values[len(values)-1] + "\"))\n")
	}
	for _, key := range typeAnn.Keys() {
		if fieldAnn.Has(key) {
			continue
		}
		values := typeAnn[key]
		out.WriteString(g.indent() + ".add(new java.util.AbstractMap.SimpleImmutableEntry<>(\"" + key + "\", \"" + values[len(values)-1] + "\"))\n")
	}
	out.WriteString(g.indent() + ".build().collect(java.util.stream.Collectors.toMap(java.util.Map.Entry::getKey, java.util.Map.Entry::getValue))")
	g.indentDown()
	g.indentDown()

	g.indentDown()
	g.indentDown()
}

func (g *Generator) generateFieldValueMetaData(out *strings.Builder, t sema.Type) {
	tt := trueType(t)
	out.WriteString("\n")
	g.indentUp()
	g.indentUp()
	switch {
	case tt.IsStruct() || tt.IsXception():
		out.WriteString(g.indent() + "new org.apache.thrift.meta_data.StructMetaData(org.apache.thrift.protocol.TType.STRUCT, " + g.tn(tt) + ".class")
	case tt.IsContainer():
		switch {
		case tt.IsList():
			out.WriteString(g.indent() + "new org.apache.thrift.meta_data.ListMetaData(org.apache.thrift.protocol.TType.LIST, ")
			g.generateFieldValueMetaData(out, tt.(*sema.List).ElemType())
		case tt.IsSet():
			out.WriteString(g.indent() + "new org.apache.thrift.meta_data.SetMetaData(org.apache.thrift.protocol.TType.SET, ")
			g.generateFieldValueMetaData(out, tt.(*sema.Set).ElemType())
		default:
			m := tt.(*sema.Map)
			out.WriteString(g.indent() + "new org.apache.thrift.meta_data.MapMetaData(org.apache.thrift.protocol.TType.MAP, ")
			g.generateFieldValueMetaData(out, m.KeyType())
			out.WriteString(", ")
			g.generateFieldValueMetaData(out, m.ValType())
		}
	case tt.IsEnum():
		out.WriteString(g.indent() + "new org.apache.thrift.meta_data.EnumMetaData(org.apache.thrift.protocol.TType.ENUM, " + g.tn(tt) + ".class")
	default:
		out.WriteString(g.indent() + "new org.apache.thrift.meta_data.FieldValueMetaData(" + g.javaTypeString(tt))
		if tt.IsBinary() {
			out.WriteString(g.indent() + ", true")
		} else if t.IsTypedef() {
			out.WriteString(g.indent() + ", \"" + t.(*sema.Typedef).Symbolic() + "\"")
		}
	}
	out.WriteString(")")
	g.indentDown()
	g.indentDown()
}

func (g *Generator) generateStructDesc(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "private static final org.apache.thrift.protocol.TStruct STRUCT_DESC = new org.apache.thrift.protocol.TStruct(\"" + s.Name() + "\");\n")
}

func (g *Generator) generateFieldDescs(out *strings.Builder, s *sema.Struct) {
	for _, m := range s.Members() {
		out.WriteString(g.indent() + "private static final org.apache.thrift.protocol.TField " + constantName(m.Name()) +
			"_FIELD_DESC = new org.apache.thrift.protocol.TField(\"" + m.Name() + "\", " + g.typeToEnum(m.Type()) + ", " +
			"(short)" + itoa(int64(m.Key())) + ");\n")
	}
}

func (g *Generator) generateSchemeMap(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "private static final org.apache.thrift.scheme.SchemeFactory STANDARD_SCHEME_FACTORY = new " + s.Name() + "StandardSchemeFactory();\n")
	out.WriteString(g.indent() + "private static final org.apache.thrift.scheme.SchemeFactory TUPLE_SCHEME_FACTORY = new " + s.Name() + "TupleSchemeFactory();\n")
}

// generateFieldNameConstants is generate_field_name_constants: the _Fields enum.
func (g *Generator) generateFieldNameConstants(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "/** The set of fields this struct contains, along with convenience methods for finding and manipulating them. */\n")
	out.WriteString(g.indent() + "public enum _Fields implements org.apache.thrift.TFieldIdEnum {\n")

	g.indentUp()
	first := true
	members := s.Members()
	for _, m := range members {
		if !first {
			out.WriteString(",\n")
		}
		first = false
		g.javaDocField(out, m)
		out.WriteString(g.indent() + constantName(m.Name()) + "((short)" + itoa(int64(m.Key())) + ", \"" + m.Name() + "\")")
	}

	out.WriteString(";\n\n")

	out.WriteString(g.indent() + "private static final java.util.Map<java.lang.String, _Fields> byName = new java.util.HashMap<java.lang.String, _Fields>();\n")
	out.WriteString("\n")

	out.WriteString(g.indent() + "static {\n")
	out.WriteString(g.indent() + "  for (_Fields field : java.util.EnumSet.allOf(_Fields.class)) {\n")
	out.WriteString(g.indent() + "    byName.put(field.getFieldName(), field);\n")
	out.WriteString(g.indent() + "  }\n")
	out.WriteString(g.indent() + "}\n\n")

	out.WriteString(g.indent() + "/**\n")
	out.WriteString(g.indent() + " * Find the _Fields constant that matches fieldId, or null if its not found.\n")
	out.WriteString(g.indent() + " */\n")
	out.WriteString(g.indent() + javaNullableAnnotation() + "\n")
	out.WriteString(g.indent() + "public static _Fields findByThriftId(int fieldId) {\n")
	g.indentUp()
	out.WriteString(g.indent() + "switch(fieldId) {\n")
	g.indentUp()

	for _, m := range members {
		out.WriteString(g.indent() + "case " + itoa(int64(m.Key())) + ": // " + constantName(m.Name()) + "\n")
		out.WriteString(g.indent() + "  return " + constantName(m.Name()) + ";\n")
	}

	out.WriteString(g.indent() + "default:\n")
	out.WriteString(g.indent() + "  return null;\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	out.WriteString(g.indent() + "/**\n")
	out.WriteString(g.indent() + " * Find the _Fields constant that matches fieldId, throwing an exception\n")
	out.WriteString(g.indent() + " * if it is not found.\n")
	out.WriteString(g.indent() + " */\n")
	out.WriteString(g.indent() + "public static _Fields findByThriftIdOrThrow(int fieldId) {\n")
	out.WriteString(g.indent() + "  _Fields fields = findByThriftId(fieldId);\n")
	out.WriteString(g.indent() + "  if (fields == null) throw new java.lang.IllegalArgumentException(\"Field \" + fieldId + \" doesn't exist!\");\n")
	out.WriteString(g.indent() + "  return fields;\n")
	out.WriteString(g.indent() + "}\n\n")

	out.WriteString(g.indent() + "/**\n")
	out.WriteString(g.indent() + " * Find the _Fields constant that matches name, or null if its not found.\n")
	out.WriteString(g.indent() + " */\n")
	out.WriteString(g.indent() + javaNullableAnnotation() + "\n")
	out.WriteString(g.indent() + "public static _Fields findByName(java.lang.String name) {\n")
	out.WriteString(g.indent() + "  return byName.get(name);\n")
	out.WriteString(g.indent() + "}\n\n")

	out.WriteString(g.indent() + "private final short _thriftId;\n")
	out.WriteString(g.indent() + "private final java.lang.String _fieldName;\n\n")

	out.WriteString(g.indent() + "_Fields(short thriftId, java.lang.String fieldName) {\n")
	out.WriteString(g.indent() + "  _thriftId = thriftId;\n")
	out.WriteString(g.indent() + "  _fieldName = fieldName;\n")
	out.WriteString(g.indent() + "}\n\n")

	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public short getThriftFieldId() {\n")
	out.WriteString(g.indent() + "  return _thriftId;\n")
	out.WriteString(g.indent() + "}\n\n")

	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public java.lang.String getFieldName() {\n")
	out.WriteString(g.indent() + "  return _fieldName;\n")
	out.WriteString(g.indent() + "}\n")

	g.indentDown()

	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateJavaStructClear(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public void clear() {\n")

	g.indentUp()
	for _, f := range s.Members() {
		t := trueType(f.Type())
		id := makeValidJavaIdentifier(f.Name())

		if f.Value() != nil {
			g.printConstValue(out, "this."+id, t, f.Value(), true, true)
			continue
		}

		if typeCanBeNull(t) {
			if g.opts.ReuseObjects && (t.IsContainer() || t.IsStruct()) {
				out.WriteString(g.indent() + "if (this." + id + " != null) {\n")
				g.indentUp()
				out.WriteString(g.indent() + "this." + id + ".clear();\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
			} else {
				out.WriteString(g.indent() + "this." + id + " = null;\n")
			}
			continue
		}

		out.WriteString(g.indent() + "set" + g.capName(f.Name()) + g.capName("isSet") + "(false);\n")
		switch baseOf(t) {
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			out.WriteString(g.indent() + "this." + id + " = 0;\n")
		case sema.TypeDouble:
			out.WriteString(g.indent() + "this." + id + " = 0.0;\n")
		case sema.TypeBool:
			out.WriteString(g.indent() + "this." + id + " = false;\n")
		default:
			emit.Throw("unsupported type: %s for field %s", t.Name(), f.Name())
		}
	}
	g.indentDown()

	out.WriteString(g.indent() + "}\n\n")
}

func (g *Generator) generateJavaStructWriteObject(out *strings.Builder, s *sema.Struct) {
	_ = s
	legacy := ""
	if g.opts.AndroidLegacy {
		legacy = ".getMessage()"
	}
	out.WriteString(g.indent() + "private void writeObject(java.io.ObjectOutputStream out) throws java.io.IOException {\n")
	out.WriteString(g.indent() + "  try {\n")
	out.WriteString(g.indent() + "    write(new org.apache.thrift.protocol.TCompactProtocol(new org.apache.thrift.transport.TIOStreamTransport(out)));\n")
	out.WriteString(g.indent() + "  } catch (org.apache.thrift.TException te) {\n")
	out.WriteString(g.indent() + "    throw new java.io.IOException(te" + legacy + ");\n")
	out.WriteString(g.indent() + "  }\n")
	out.WriteString(g.indent() + "}\n\n")
}

func (g *Generator) generateJavaStructReadObject(out *strings.Builder, s *sema.Struct) {
	legacy := ""
	if g.opts.AndroidLegacy {
		legacy = ".getMessage()"
	}
	out.WriteString(g.indent() + "private void readObject(java.io.ObjectInputStream in) throws java.io.IOException, java.lang.ClassNotFoundException {\n")
	out.WriteString(g.indent() + "  try {\n")
	if !s.IsUnion() {
		switch kind, _ := g.needsIsset(s); kind {
		case issetNone:
		case issetPrimitive:
			out.WriteString(g.indent() + "    // it doesn't seem like you should have to do this, but java serialization is wacky, and doesn't call the default constructor.\n")
			out.WriteString(g.indent() + "    __isset_bitfield = 0;\n")
		case issetBitset:
			out.WriteString(g.indent() + "    // it doesn't seem like you should have to do this, but java serialization is wacky, and doesn't call the default constructor.\n")
			out.WriteString(g.indent() + "    __isset_bit_vector = new java.util.BitSet(1);\n")
		}
	}
	out.WriteString(g.indent() + "    read(new org.apache.thrift.protocol.TCompactProtocol(new org.apache.thrift.transport.TIOStreamTransport(in)));\n")
	out.WriteString(g.indent() + "  } catch (org.apache.thrift.TException te) {\n")
	out.WriteString(g.indent() + "    throw new java.io.IOException(te" + legacy + ");\n")
	out.WriteString(g.indent() + "  }\n")
	out.WriteString(g.indent() + "}\n\n")
}

func (g *Generator) generateStandardReader(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public void read(org.apache.thrift.protocol.TProtocol iprot, " + makeValidJavaIdentifier(s.Name()) + " struct) throws org.apache.thrift.TException {\n")
	g.indentUp()
	out.WriteString(g.indent() + "iprot.incrementRecursionDepth();\n")
	out.WriteString(g.indent() + "try {\n")
	g.indentUp()

	fields := s.Members()

	out.WriteString(g.indent() + "org.apache.thrift.protocol.TField schemeField;\n" +
		g.indent() + "iprot.readStructBegin();\n")

	out.WriteString(g.indent() + "while (true)\n")
	g.scopeUp(out)

	out.WriteString(g.indent() + "schemeField = iprot.readFieldBegin();\n")

	out.WriteString(g.indent() + "if (schemeField.type == org.apache.thrift.protocol.TType.STOP) { \n")
	g.indentUp()
	out.WriteString(g.indent() + "break;\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	out.WriteString(g.indent() + "switch (schemeField.id) {\n")

	g.indentUp()

	for _, f := range fields {
		out.WriteString(g.indent() + "case " + itoa(int64(f.Key())) + ": // " + constantName(f.Name()) + "\n")
		g.indentUp()
		out.WriteString(g.indent() + "if (schemeField.type == " + g.typeToEnum(f.Type()) + ") {\n")
		g.indentUp()

		g.generateDeserializeField(out, f, "struct.", true)
		out.WriteString(g.indent() + "struct." + "set" + g.capName(f.Name()) + g.capName("isSet") + "(true);\n")
		g.indentDown()
		out.WriteString(g.indent() + "} else { \n" +
			g.indent() + "  org.apache.thrift.protocol.TProtocolUtil.skip(iprot, schemeField.type);\n" +
			g.indent() + "}\n" +
			g.indent() + "break;\n")
		g.indentDown()
	}

	out.WriteString(g.indent() + "default:\n")
	out.WriteString(g.indent() + "  org.apache.thrift.protocol.TProtocolUtil.skip(iprot, schemeField.type);\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	out.WriteString(g.indent() + "iprot.readFieldEnd();\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	out.WriteString(g.indent() + "iprot.readStructEnd();\n")

	if !g.opts.BeanStyle {
		out.WriteString("\n" + g.indent() + "// check for required fields of primitive type, which can't be checked in the validate method\n")
		for _, f := range fields {
			if f.Req() == sema.Required && !typeCanBeNull(f.Type()) {
				out.WriteString(g.indent() + "if (!struct." + g.issetCheck(f) + ") {\n" +
					g.indent() + "  throw new org.apache.thrift.protocol.TProtocolException(\"Required field '" +
					f.Name() + "' was not found in serialized data! Struct: \" + toString());\n" +
					g.indent() + "}\n")
			}
		}
	}

	out.WriteString(g.indent() + "struct.validate();\n")

	g.indentDown()
	out.WriteString(g.indent() + "} finally {\n")
	g.indentUp()
	out.WriteString(g.indent() + "iprot.decrementRecursionDepth();\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateStandardWriter(out *strings.Builder, s *sema.Struct, isResult bool) {
	g.indentUp()
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public void write(org.apache.thrift.protocol.TProtocol oprot, " + makeValidJavaIdentifier(s.Name()) + " struct) throws org.apache.thrift.TException {\n")
	g.indentUp()
	fields := s.SortedMembers()

	out.WriteString(g.indent() + "struct.validate();\n\n")

	out.WriteString(g.indent() + "oprot.writeStructBegin(STRUCT_DESC);\n")

	for _, f := range fields {
		nullAllowed := typeCanBeNull(f.Type())
		if nullAllowed {
			out.WriteString(g.indent() + "if (struct." + makeValidJavaIdentifier(f.Name()) + " != null) {\n")
			g.indentUp()
		}
		optional := f.Req() == sema.Optional || (isResult && !nullAllowed)
		if optional {
			out.WriteString(g.indent() + "if (" + "struct." + g.issetCheck(f) + ") {\n")
			g.indentUp()
		}

		out.WriteString(g.indent() + "oprot.writeFieldBegin(" + constantName(f.Name()) + "_FIELD_DESC);\n")

		g.generateSerializeField(out, f, "struct.", "", true)

		out.WriteString(g.indent() + "oprot.writeFieldEnd();\n")

		if optional {
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
		if nullAllowed {
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
	}
	out.WriteString(g.indent() + "oprot.writeFieldStop();\n" +
		g.indent() + "oprot.writeStructEnd();\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
	g.indentDown()
}

func (g *Generator) generateJavaStructStandardScheme(out *strings.Builder, s *sema.Struct, isResult bool) {
	out.WriteString(g.indent() + "private static class " + s.Name() + "StandardSchemeFactory implements org.apache.thrift.scheme.SchemeFactory {\n")
	g.indentUp()
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public " + s.Name() + "StandardScheme getScheme() {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return new " + s.Name() + "StandardScheme();\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	out.WriteString(g.indent() + "private static class " + s.Name() +
		"StandardScheme extends org.apache.thrift.scheme.StandardScheme<" + makeValidJavaIdentifier(s.Name()) + "> {\n\n")
	g.indentUp()
	g.generateStandardReader(out, s)
	g.indentDown()
	out.WriteString("\n")
	g.generateStandardWriter(out, s, isResult)

	out.WriteString(g.indent() + "}\n\n")
}

func isOptionalOrDefault(f *sema.Field) bool {
	return f.Req() == sema.Optional || f.Req() == sema.OptInReqOut
}

func (g *Generator) generateJavaStructTupleReader(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public void read(org.apache.thrift.protocol.TProtocol prot, " + makeValidJavaIdentifier(s.Name()) + " struct) throws org.apache.thrift.TException {\n")
	g.indentUp()
	out.WriteString(g.indent() + "prot.incrementRecursionDepth();\n")
	out.WriteString(g.indent() + "try {\n")
	g.indentUp()
	out.WriteString(g.indent() + "org.apache.thrift.protocol.TTupleProtocol iprot = (org.apache.thrift.protocol.TTupleProtocol) prot;\n")
	optionalCount := 0
	fields := s.Members()
	for _, f := range fields {
		if isOptionalOrDefault(f) {
			optionalCount++
		}
		if f.Req() == sema.Required {
			g.generateDeserializeField(out, f, "struct.", false)
			out.WriteString(g.indent() + "struct.set" + g.capName(f.Name()) + g.capName("isSet") + "(true);\n")
		}
	}
	if optionalCount > 0 {
		out.WriteString(g.indent() + "java.util.BitSet incoming = iprot.readBitSet(" + itoa(int64(optionalCount)) + ");\n")
		i := 0
		for _, f := range fields {
			if isOptionalOrDefault(f) {
				out.WriteString(g.indent() + "if (incoming.get(" + itoa(int64(i)) + ")) {\n")
				g.indentUp()
				g.generateDeserializeField(out, f, "struct.", false)
				out.WriteString(g.indent() + "struct.set" + g.capName(f.Name()) + g.capName("isSet") + "(true);\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
				i++
			}
		}
	}
	g.indentDown()
	out.WriteString(g.indent() + "} finally {\n")
	g.indentUp()
	out.WriteString(g.indent() + "prot.decrementRecursionDepth();\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateJavaStructTupleWriter(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public void write(org.apache.thrift.protocol.TProtocol prot, " + makeValidJavaIdentifier(s.Name()) + " struct) throws org.apache.thrift.TException {\n")
	g.indentUp()
	out.WriteString(g.indent() + "org.apache.thrift.protocol.TTupleProtocol oprot = (org.apache.thrift.protocol.TTupleProtocol) prot;\n")

	fields := s.Members()
	hasOptional := false
	optionalCount := 0
	for _, f := range fields {
		if isOptionalOrDefault(f) {
			optionalCount++
			hasOptional = true
		}
		if f.Req() == sema.Required {
			g.generateSerializeField(out, f, "struct.", "", false)
		}
	}
	if hasOptional {
		out.WriteString(g.indent() + "java.util.BitSet optionals = new java.util.BitSet();\n")
		i := 0
		for _, f := range fields {
			if isOptionalOrDefault(f) {
				out.WriteString(g.indent() + "if (struct." + g.issetCheck(f) + ") {\n")
				g.indentUp()
				out.WriteString(g.indent() + "optionals.set(" + itoa(int64(i)) + ");\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
				i++
			}
		}

		out.WriteString(g.indent() + "oprot.writeBitSet(optionals, " + itoa(int64(optionalCount)) + ");\n")
		for _, f := range fields {
			if isOptionalOrDefault(f) {
				out.WriteString(g.indent() + "if (struct." + g.issetCheck(f) + ") {\n")
				g.indentUp()
				g.generateSerializeField(out, f, "struct.", "", false)
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
			}
		}
	}
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateJavaStructTupleScheme(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "private static class " + s.Name() + "TupleSchemeFactory implements org.apache.thrift.scheme.SchemeFactory {\n")
	g.indentUp()
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public " + s.Name() + "TupleScheme getScheme() {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return new " + s.Name() + "TupleScheme();\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
	out.WriteString(g.indent() + "private static class " + s.Name() +
		"TupleScheme extends org.apache.thrift.scheme.TupleScheme<" + makeValidJavaIdentifier(s.Name()) + "> {\n\n")
	g.indentUp()
	g.generateJavaStructTupleWriter(out, s)
	out.WriteString("\n")
	g.generateJavaStructTupleReader(out, s)
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

func (g *Generator) generateJavaSchemeLookup(out *strings.Builder) {
	out.WriteString(g.indent() + "private static <S extends org.apache.thrift.scheme.IScheme> S scheme(" + "org.apache.thrift.protocol.TProtocol proto) {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return (org.apache.thrift.scheme.StandardScheme.class.equals(proto.getScheme()) " +
		"? STANDARD_SCHEME_FACTORY " + ": TUPLE_SCHEME_FACTORY" + ").getScheme();\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}
