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

package golang

import (
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

func (g *Generator) generateTypedef(td *sema.Typedef) {
	newTypeName := g.publicize(td.Symbolic())
	baseType := g.typeToGoType(td.Type())
	if baseType == newTypeName {
		return
	}
	// A struct is used through a pointer and carries its Read, Write and Equals
	// methods on that pointer, so a typedef of one has to be a Go alias: a
	// defined type would be a distinct type with no methods, and one whose
	// underlying type is already a pointer, so every use of it would need a
	// second pointer to reach the struct.
	resolved := sema.TrueType(td.Type())
	aliasOfStruct := resolved.IsStruct() || resolved.IsXception()
	aliasTarget := strings.TrimPrefix(baseType, "*")
	if !aliasOfStruct {
		aliasTarget = baseType
	}

	g.beginTypesDeclaration()
	out := &g.fTypes
	g.generateDocstring(out, td)
	g.generateDeprecationComment(out, td.Annotations())
	if aliasOfStruct {
		out.WriteString("type " + newTypeName + " = " + aliasTarget + "\n")
		// The alias is the struct, used through a pointer like the struct itself,
		// and structs get no Ptr helper; one here would only copy a value that
		// nobody holds by value.
		return
	}
	out.WriteString("type " + newTypeName + " " + baseType + "\n")
	out.WriteString("\n")
	if g.generateDeprecationComment(out, td.Annotations()) {
		out.WriteString("//\n")
	}
	out.WriteString("//go:fix inline\n")
	out.WriteString("func " + newTypeName + "Ptr(v " + newTypeName + ") *" + newTypeName + " { return new(v) }\n")
}

func (g *Generator) generateEnum(e *sema.Enum) {
	g.beginTypesDeclaration()
	out := &g.fTypes
	var toString, fromString, knownValues, isDefined strings.Builder
	enumName := g.publicize(e.Name())
	g.generateDocstring(out, e)
	g.generateDeprecationComment(out, e.Annotations())
	out.WriteString("type " + enumName + " int64\n\n")
	knownValues.WriteString("var known" + enumName + "Values" + " = []" + enumName + "{\n")
	toString.WriteString("func (p " + enumName + ") String() string {\n")
	toString.WriteString("switch p {\n")
	g.generateDeprecationComment(&fromString, e.Annotations())
	fromString.WriteString("func " + enumName + "FromString(s string) (" + enumName + ", error) {\n")
	fromString.WriteString("switch s {\n")
	g.generateDeprecationComment(&isDefined, e.Annotations())
	isDefined.WriteString("func (p " + enumName + ") IsDefined() bool {\n")
	isDefined.WriteString("switch p {\n")
	constants := e.Constants()
	if len(constants) == 0 {
		out.WriteString("const ()\n\n")
	} else {
		out.WriteString("const (\n")
	}
	for _, c := range constants {
		value := c.Value()
		iterStdName := escapeString(c.Name())
		iterName := c.Name()
		goEnumName := enumName + "_" + iterName
		g.generateDeprecationComment(out, c.Annotations())
		out.WriteString(goEnumName + " " + enumName + " = " + itoa(int64(value)) + "\n")
		knownValues.WriteString(goEnumName + ",\n")
		toString.WriteString("case " + goEnumName + ":\n")
		toString.WriteString("return \"" + iterStdName + "\"\n")
		if iterStdName != escapeString(iterName) {
			fromString.WriteString("case \"" + iterStdName + "\", \"" + escapeString(iterName) + "\":\n")
		} else {
			fromString.WriteString("case \"" + iterStdName + "\":\n")
		}
		fromString.WriteString("return " + goEnumName + ", nil\n")
		isDefined.WriteString("case " + goEnumName + ":\n")
		isDefined.WriteString("return true\n")
	}
	toString.WriteString("}\n")
	toString.WriteString("return fmt.Sprintf(\"" + enumName + "(%d)\", p)\n")
	toString.WriteString("}\n")
	fromString.WriteString("}\n")
	fromString.WriteString("return " + enumName + "(0)," + " fmt.Errorf(\"not a valid " + enumName + " string\")\n")
	fromString.WriteString("}\n")
	isDefined.WriteString("}\n")
	isDefined.WriteString("return false\n")
	isDefined.WriteString("}\n")
	if len(constants) == 0 {
		knownValues.Reset()
		knownValues.WriteString("var known" + enumName + "Values" + " = []" + enumName + "{}\n\n")
	} else {
		knownValues.WriteString("}\n\n")
	}
	knownValues.WriteString("func " + enumName + "Values() iter.Seq[" + enumName + "] {\n")
	knownValues.WriteString("return func(yield func(" + enumName + ") bool) {\n")
	knownValues.WriteString("for _, v := range known" + enumName + "Values {\n")
	knownValues.WriteString("if !yield(v) {\n")
	knownValues.WriteString("return\n")
	knownValues.WriteString("}\n")
	knownValues.WriteString("}\n")
	knownValues.WriteString("}\n")
	knownValues.WriteString("}\n")
	if len(constants) != 0 {
		out.WriteString(")\n\n")
	}
	out.WriteString(knownValues.String() + toString.String() + "\n" + fromString.String() + "\n" + isDefined.String() + "\n")
	if g.generateDeprecationComment(out, e.Annotations()) {
		out.WriteString("//\n")
	}
	out.WriteString("//go:fix inline\n")
	out.WriteString("func " + enumName + "Ptr(v " + enumName + ") *" + enumName + " { return new(v) }\n")
	out.WriteString("\n")
	out.WriteString("func (p " + enumName + ") MarshalText() ([]byte, error) {\n")
	out.WriteString("return []byte(p.String()), nil\n")
	out.WriteString("}\n\n")
	out.WriteString("func (p *" + enumName + ") UnmarshalText(text []byte) error {\n")
	out.WriteString("q, err := " + enumName + "FromString(string(text))\n")
	out.WriteString("if err != nil {\n")
	out.WriteString("return err\n")
	out.WriteString("}\n")
	out.WriteString("*p = q\n")
	out.WriteString("return nil\n")
	out.WriteString("}\n\n")
	out.WriteString("func (p *" + enumName + ") Scan(value interface{}) error {\n")
	out.WriteString("v, ok := value.(int64)\n")
	out.WriteString("if !ok {\n")
	out.WriteString("return errors.New(\"Scan value is not int64\")\n")
	out.WriteString("}\n")
	out.WriteString("*p = " + enumName + "(v)\n")
	out.WriteString("return nil\n")
	out.WriteString("}\n\n")
	out.WriteString("func (p *" + enumName + ") Value() (driver.Value, error) {\n")
	out.WriteString("if p == nil {\n")
	out.WriteString("return nil, nil\n")
	out.WriteString("}\n")
	out.WriteString("return int64(*p), nil\n")
	out.WriteString("}\n")
}

func (g *Generator) generateConst(c *sema.Const) {
	typ := c.Type()
	name := g.publicize(c.Name())
	value := c.Value()
	if typ.IsEnum() || (typ.IsBaseType() && typ.(*sema.BaseType).Base() != sema.TypeUUID) {
		if g.lastConstBlock == 2 {
			g.fConsts.WriteString("\n")
		}
		g.lastConstBlock = 1
		g.fConsts.WriteString("const " + name + " = " + g.renderConstValue(typ, value, name, false) + "\n")
	} else {
		if g.lastConstBlock == 1 {
			g.fConsts.WriteString("\n")
		}
		g.lastConstBlock = 2
		rendered := g.renderConstValue(typ, value, name, false)
		g.fConstValues.WriteString("\t" + name + " = " + rendered + "\n")
		g.fConsts.WriteString("var " + name + " " + g.typeToGoType(typ) + "\n")
	}
}

// renderConstValue is render_const_value.
// constContainerLiteral returns the type that opens a container literal in a
// constant. A container field with a default value is a pointer field, so
// the literal has to be addressed, and a typedef'd one has to use the typedef
// name for the address to have the field's type.
func constContainerLiteral(containerType, typedefName string, pointer bool) string {
	if !pointer {
		return containerType
	}
	if typedefName == "" {
		return "&" + containerType
	}
	return "&" + typedefName
}

func (g *Generator) renderConstValue(typ sema.Type, value *sema.ConstValue, name string, opt bool) string {
	typedefOpt := ""
	if typ.IsTypedef() {
		typedefOpt = g.publicize(g.typeName(typ))
	}
	typ = sema.TrueType(typ)
	var out strings.Builder
	switch {
	case typ.IsBaseType():
		tbase := typ.(*sema.BaseType).Base()
		if opt {
			switch tbase {
			case sema.TypeBool:
				out.WriteString("new(")
				if typedefOpt != "" {
					out.WriteString(typedefOpt + "(")
				}
				if value.Integer() > 0 {
					out.WriteString("true")
				} else {
					out.WriteString("false")
				}
				if typedefOpt != "" {
					out.WriteString(")")
				}
			case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
				out.WriteString("new(")
				if typedefOpt != "" {
					out.WriteString(typedefOpt)
				} else {
					out.WriteString(map[sema.BaseKind]string{sema.TypeI8: "int8", sema.TypeI16: "int16", sema.TypeI32: "int32", sema.TypeI64: "int64"}[tbase])
				}
				out.WriteString("(" + itoa(value.Integer()) + ")")
			case sema.TypeDouble:
				out.WriteString("new(")
				if typedefOpt != "" {
					out.WriteString(typedefOpt)
				} else {
					out.WriteString("float64")
				}
				out.WriteString("(")
				if value.Kind() == sema.CVInteger {
					out.WriteString(itoa(value.Integer()))
				} else {
					out.WriteString(formatDouble(value.Double()))
				}
				out.WriteString(")")
			case sema.TypeString:
				out.WriteString("new(")
				if typedefOpt != "" {
					out.WriteString(typedefOpt + "(")
				}
				out.WriteString("\"" + escapeString(value.String()) + "\"")
				if typedefOpt != "" {
					out.WriteString(")")
				}
			case sema.TypeUUID:
				out.WriteString("new(")
				if typedefOpt != "" {
					out.WriteString(typedefOpt + "(")
				}
				out.WriteString("thrift.Must(thrift.ParseTuuid(\"" + escapeString(value.String()) + "\"))")
				if typedefOpt != "" {
					out.WriteString(")")
				}
			default:
				throw("compiler error: no const of base type %s", sema.BaseName(tbase))
			}
			out.WriteString(")")
		} else {
			switch tbase {
			case sema.TypeString:
				if typ.IsBinary() {
					out.WriteString("[]byte(\"" + escapeString(value.String()) + "\")")
				} else {
					out.WriteString("\"" + escapeString(value.String()) + "\"")
				}
			case sema.TypeBool:
				if value.Integer() > 0 {
					out.WriteString("true")
				} else {
					out.WriteString("false")
				}
			case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
				out.WriteString(itoa(value.Integer()))
			case sema.TypeDouble:
				if value.Kind() == sema.CVInteger {
					out.WriteString(itoa(value.Integer()))
				} else {
					out.WriteString(formatDouble(value.Double()))
				}
			case sema.TypeUUID:
				if typedefOpt != "" {
					out.WriteString(typedefOpt + "(")
				}
				out.WriteString("thrift.Must(thrift.ParseTuuid(\"" + escapeString(value.String()) + "\"))")
				if typedefOpt != "" {
					out.WriteString(")")
				}
			default:
				throw("compiler error: no const of base type %s", sema.BaseName(tbase))
			}
		}
	case typ.IsEnum():
		if opt {
			out.WriteString("new(")
			if typedefOpt != "" {
				out.WriteString(typedefOpt)
			} else {
				out.WriteString(g.typeName(typ))
			}
			out.WriteString("(")
		}
		out.WriteString(itoa(value.Integer()))
		if opt {
			out.WriteString("))")
		}
	case typ.IsStruct() || typ.IsXception():
		val := value.Map()
		if len(val) == 0 {
			return "&" + g.publicize(g.typeName(typ)) + "{}"
		}
		out.WriteString("&" + g.publicize(g.typeName(typ)) + "{")
		fields := typ.(*sema.Struct).Members()
		var fieldNames, fieldValues []string
		for _, e := range val {
			var fieldType sema.Type
			isOptional := false
			for _, f := range fields {
				if f.Name() == e.Key.String() {
					fieldType = f.Type()
					isOptional = isPointerField(f)
				}
			}
			if fieldType == nil {
				throw("type error: %s has no field %s", typ.Name(), e.Key.String())
			}
			rendered := g.renderConstValue(fieldType, e.Value, name, isOptional)
			fieldNames = append(fieldNames, g.publicize(e.Key.String()))
			fieldValues = append(fieldValues, rendered)
		}
		writeFields(&out, fieldNames, fieldValues)
		out.WriteString("\n" + "}")
	case typ.IsMap():
		m := typ.(*sema.Map)
		ktype, vtype := m.KeyType(), m.ValType()
		val := value.Map()
		if g.isContainerKeyedMap(typ) {
			literal := constContainerLiteral("[]"+g.mapEntryType(m), typedefOpt, opt)
			if len(val) == 0 {
				return literal + "{}"
			}
			out.WriteString(literal + "{\n")
			for _, e := range val {
				out.WriteString("{Key: " + g.renderConstValue(ktype, e.Key, name, false) +
					", Value: " + g.renderConstValue(vtype, e.Value, name, false) + "},\n")
			}
			out.WriteString("}")
			return out.String()
		}
		literal := constContainerLiteral("map["+g.typeToGoKeyType(ktype)+"]"+g.typeToGoType(vtype), typedefOpt, opt)
		if len(val) == 0 {
			return literal + "{}"
		}
		out.WriteString(literal + "{\n")
		for _, e := range val {
			key := g.renderConstValue(ktype, e.Key, name, false)
			rendered := g.renderConstValue(vtype, e.Value, name, false)
			out.WriteString(key + ": " + rendered + ",\n")
		}
		out.WriteString("}")
	case typ.IsList() || typ.IsSet():
		var etype sema.Type
		if typ.IsList() {
			etype = typ.(*sema.List).ElemType()
		} else {
			etype = typ.(*sema.Set).ElemType()
		}
		val := value.List()
		literal := constContainerLiteral("[]"+g.typeToGoType(etype), typedefOpt, opt)
		if len(val) == 0 {
			return literal + "{}"
		}
		out.WriteString(literal + "{\n")
		for _, v := range val {
			out.WriteString(g.renderConstValue(etype, v, name, false) + ",\n")
		}
		out.WriteString("}")
	default:
		throw("CANNOT GENERATE CONSTANT FOR TYPE: %s", typ.Name())
	}
	return out.String()
}

// writeFields writes "name: value," lines; gofmt aligns them on write.
func writeFields(out *strings.Builder, names, values []string) {
	for i := range names {
		out.WriteString("\n" + names[i] + ": " + values[i] + ",")
	}
}
