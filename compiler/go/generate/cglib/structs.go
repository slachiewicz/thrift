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
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateStruct generates Thrift structs in C code, as GObjects.
func (g *Generator) generateStruct(tstruct *sema.Struct) {
	g.fTypes.WriteString("/* struct " + tstruct.Name() + " */\n")
	g.generateObject(tstruct)
}

func (g *Generator) generateXception(tstruct *sema.Struct) {
	name := tstruct.Name()
	nameU := initialCapsToUnderscores(name)
	nameLC := toLowerCase(nameU)
	nameUC := toUpperCase(nameU)

	g.generateObject(tstruct)

	g.fTypes.WriteString("/* exception */\n" +
		"typedef enum\n" +
		"{\n")
	g.indentUp()
	g.fTypes.WriteString(g.indent() + g.nspaceUC + nameUC + "_ERROR_CODE\n")
	g.indentDown()
	g.fTypes.WriteString("} " + g.nspace + name + "Error;\n" +
		"\n" +
		"GQuark " + g.nspaceLC + nameLC + "_error_quark (void);\n" +
		"#define " + g.nspaceUC + nameUC + "_ERROR (" +
		g.nspaceLC + nameLC + "_error_quark())\n" +
		"\n" +
		"\n")

	g.fTypesImpl.WriteString("/* define the GError domain for exceptions */\n" +
		"#define " + g.nspaceUC + nameUC + "_ERROR_DOMAIN \"" + g.nspaceLC + nameLC + "_error_quark\"\n" +
		"GQuark\n" + g.nspaceLC + nameLC + "_error_quark (void)\n" +
		"{\n" +
		"  return g_quark_from_static_string (" + g.nspaceUC + nameUC + "_ERROR_DOMAIN);\n" +
		"}\n\n")
}

// generateObject generates C code to represent a Thrift structure as a
// GObject.
func (g *Generator) generateObject(tstruct *sema.Struct) {
	name := tstruct.Name()
	nameU := initialCapsToUnderscores(name)
	nameUC := toUpperCase(nameU)

	className := g.nspace + name
	classNameLC := g.nspaceLC + initialCapsToUnderscores(name)
	classNameUC := toUpperCase(classNameLC)

	members := tstruct.Members()

	// write the instance definition
	g.fTypes.WriteString("struct _" + g.nspace + name + "\n" +
		"{ \n" +
		"  ThriftStruct parent; \n" +
		"\n" +
		"  /* public */\n")

	// for each field, add a member variable
	for _, m := range members {
		t := sema.TrueType(m.Type())
		g.fTypes.WriteString("  " + g.typeName(t, false, false) + " " + m.Name() + ";\n")
		if m.Req() != sema.Required {
			g.fTypes.WriteString("  gboolean __isset_" + m.Name() + ";\n")
		}
	}

	// close the structure definition and create a typedef
	g.fTypes.WriteString("};\ntypedef struct _" + g.nspace + name + " " + g.nspace + name + ";\n\n")

	// write the class definition
	g.fTypes.WriteString("struct _" + g.nspace + name + "Class\n" +
		"{\n" +
		"  ThriftStructClass parent;\n" +
		"};\ntypedef struct _" + g.nspace + name + "Class " + g.nspace + name + "Class;\n\n")

	// write the standard GObject boilerplate
	g.fTypes.WriteString("GType " + g.nspaceLC + nameU + "_get_type (void);\n" +
		"#define " + g.nspaceUC + "TYPE_" + nameUC + " (" + g.nspaceLC + nameU + "_get_type())\n" +
		"#define " + g.nspaceUC + nameUC +
		"(obj) (G_TYPE_CHECK_INSTANCE_CAST ((obj), " + g.nspaceUC + "TYPE_" + nameUC +
		", " + g.nspace + name + "))\n" +
		"#define " + g.nspaceUC + nameUC +
		"_CLASS(c) (G_TYPE_CHECK_CLASS_CAST ((c), " + g.nspaceUC + "_TYPE_" +
		nameUC + ", " + g.nspace + name + "Class))\n" +
		"#define " + g.nspaceUC + "IS_" + nameUC + "(obj) (G_TYPE_CHECK_INSTANCE_TYPE ((obj), " +
		g.nspaceUC + "TYPE_" + nameUC + "))\n" +
		"#define " + g.nspaceUC + "IS_" + nameUC + "_CLASS(c) (G_TYPE_CHECK_CLASS_TYPE ((c), " + g.nspaceUC +
		"TYPE_" + nameUC + "))\n" +
		"#define " + g.nspaceUC + nameUC +
		"_GET_CLASS(obj) (G_TYPE_INSTANCE_GET_CLASS ((obj), " + g.nspaceUC + "TYPE_" +
		nameUC + ", " + g.nspace + name + "Class))\n\n")

	// start writing the object implementation .c file

	// generate properties enum
	if len(members) > 0 {
		g.fTypesImpl.WriteString("enum _" + className + "Properties\n{\n")
		g.indentUp()
		g.fTypesImpl.WriteString(g.indent() + "PROP_" + classNameUC + "_0")
		for _, m := range members {
			memberNameUC := toUpperCase(toLowerCase(initialCapsToUnderscores(m.Name())))
			g.fTypesImpl.WriteString(",\n" + g.indent() + "PROP_" + classNameUC + "_" + memberNameUC)
		}
		g.fTypesImpl.WriteString("\n")
		g.indentDown()
		g.fTypesImpl.WriteString("};\n\n")
	}

	// generate struct I/O methods
	thisGet := g.nspace + name + " * this_object = " + g.nspaceUC + nameUC + "(object);"
	g.generateStructReader(&g.fTypesImpl, tstruct, "this_object->", thisGet, true)
	g.generateStructWriter(&g.fTypesImpl, tstruct, "this_object->", thisGet, true)

	// generate property setter and getter
	if len(members) > 0 {
		g.generateObjectPropertySetter(className, classNameLC, classNameUC, tstruct)
		g.generateObjectPropertyGetter(className, classNameLC, classNameUC, tstruct)
	}

	g.generateObjectInstanceInit(name, nameU, tstruct)
	g.generateObjectFinalize(name, nameU, nameUC, tstruct)
	g.generateObjectClassInit(name, className, classNameLC, classNameUC, tstruct)
}

// generateObjectPropertySetter generates the property setter.
func (g *Generator) generateObjectPropertySetter(className, classNameLC, classNameUC string, tstruct *sema.Struct) {
	members := tstruct.Members()

	functionName := classNameLC + "_set_property"
	argsIndent := strings.Repeat(" ", len(functionName)+2)
	g.fTypesImpl.WriteString("static void\n" + functionName + " (GObject *object,\n" +
		argsIndent + "guint property_id,\n" +
		argsIndent + "const GValue *value,\n" +
		argsIndent + "GParamSpec *pspec)\n")
	g.scopeUp(&g.fTypesImpl)
	g.fTypesImpl.WriteString(g.indent() + className + " *self = " + classNameUC + " (object);\n" +
		"\n" + g.indent() + "switch (property_id)\n")
	g.scopeUp(&g.fTypesImpl)

	for _, member := range members {
		memberName := member.Name()
		memberNameUC := toUpperCase(toLowerCase(initialCapsToUnderscores(memberName)))
		memberType := sema.TrueType(member.Type())

		propertyIdentifier := "PROP_" + classNameUC + "_" + memberNameUC

		g.fTypesImpl.WriteString(g.indent() + "case " + propertyIdentifier + ":\n")
		g.indentUp()

		assignFunctionName := ""
		if memberType.IsBaseType() {
			bt := memberType.(*sema.BaseType)
			if bt.Base() == sema.TypeString {
				g.fTypesImpl.WriteString(g.indent() + "if (self->" + memberName + " != NULL)\n")
				g.indentUp()
				var releaseFunctionName string
				if bt.IsBinary() {
					releaseFunctionName = "g_byte_array_unref"
					assignFunctionName = "g_value_dup_boxed"
				} else {
					releaseFunctionName = "g_free"
					assignFunctionName = "g_value_dup_string"
				}
				g.fTypesImpl.WriteString(g.indent() + releaseFunctionName + " (self->" + memberName + ");\n")
				g.indentDown()
			} else {
				switch bt.Base() {
				case sema.TypeBool:
					assignFunctionName = "g_value_get_boolean"
				case sema.TypeI8, sema.TypeI16, sema.TypeI32:
					assignFunctionName = "g_value_get_int"
				case sema.TypeI64:
					assignFunctionName = "g_value_get_int64"
				case sema.TypeDouble:
					assignFunctionName = "g_value_get_double"
				default:
					emit.Throw("compiler error: unrecognized base type \"%s\" for struct member \"%s\"",
						memberType.Name(), memberName)
				}
			}
			g.fTypesImpl.WriteString(g.indent() + "self->" + memberName + " = " + assignFunctionName + " (value);\n")
		} else if memberType.IsEnum() {
			g.fTypesImpl.WriteString(g.indent() + "self->" + memberName + " = g_value_get_int (value);\n")
		} else if memberType.IsContainer() {
			var releaseFunctionName, assignFunctionName string
			if memberType.IsList() {
				elemType := memberType.(*sema.List).ElemType()
				if g.isNumeric(elemType) {
					releaseFunctionName = "g_array_unref"
				} else {
					releaseFunctionName = "g_ptr_array_unref"
				}
				assignFunctionName = "g_value_dup_boxed"
			} else if memberType.IsSet() || memberType.IsMap() {
				releaseFunctionName = "g_hash_table_unref"
				assignFunctionName = "g_value_dup_boxed"
			}

			g.fTypesImpl.WriteString(g.indent() + "if (self->" + memberName + " != NULL)\n")
			g.indentUp()
			g.fTypesImpl.WriteString(g.indent() + releaseFunctionName + " (self->" + memberName + ");\n")
			g.indentDown()
			g.fTypesImpl.WriteString(g.indent() + "self->" + memberName + " = " + assignFunctionName + " (value);\n")
		} else if memberType.IsStruct() || memberType.IsXception() {
			g.fTypesImpl.WriteString(g.indent() + "if (self->" + memberName + " != NULL)\n")
			g.indentUp()
			g.fTypesImpl.WriteString(g.indent() + "g_object_unref (self->" + memberName + ");\n")
			g.indentDown()
			g.fTypesImpl.WriteString(g.indent() + "self->" + memberName + " = g_value_dup_object (value);\n")
		}

		if member.Req() != sema.Required {
			g.fTypesImpl.WriteString(g.indent() + "self->__isset_" + memberName + " = TRUE;\n")
		}

		g.fTypesImpl.WriteString(g.indent() + "break;\n\n")
		g.indentDown()
	}
	g.fTypesImpl.WriteString(g.indent() + "default:\n")
	g.indentUp()
	g.fTypesImpl.WriteString(g.indent() + "G_OBJECT_WARN_INVALID_PROPERTY_ID (object, property_id, pspec);\n" +
		g.indent() + "break;\n")
	g.indentDown()
	g.scopeDown(&g.fTypesImpl)
	g.scopeDown(&g.fTypesImpl)
	g.fTypesImpl.WriteString("\n")
}

// generateObjectPropertyGetter generates the property getter.
func (g *Generator) generateObjectPropertyGetter(className, classNameLC, classNameUC string, tstruct *sema.Struct) {
	members := tstruct.Members()

	functionName := classNameLC + "_get_property"
	argsIndent := strings.Repeat(" ", len(functionName)+2)
	g.fTypesImpl.WriteString("static void\n" + functionName + " (GObject *object,\n" +
		argsIndent + "guint property_id,\n" +
		argsIndent + "GValue *value,\n" +
		argsIndent + "GParamSpec *pspec)\n")
	g.scopeUp(&g.fTypesImpl)
	g.fTypesImpl.WriteString(g.indent() + className + " *self = " + classNameUC + " (object);\n" +
		"\n" + g.indent() + "switch (property_id)\n")
	g.scopeUp(&g.fTypesImpl)

	for _, member := range members {
		memberName := member.Name()
		memberNameUC := toUpperCase(toLowerCase(initialCapsToUnderscores(memberName)))
		memberType := sema.TrueType(member.Type())

		propertyIdentifier := "PROP_" + classNameUC + "_" + memberNameUC

		setterFunctionName := ""
		if memberType.IsBaseType() {
			bt := memberType.(*sema.BaseType)
			switch bt.Base() {
			case sema.TypeBool:
				setterFunctionName = "g_value_set_boolean"
			case sema.TypeI8, sema.TypeI16, sema.TypeI32:
				setterFunctionName = "g_value_set_int"
			case sema.TypeI64:
				setterFunctionName = "g_value_set_int64"
			case sema.TypeDouble:
				setterFunctionName = "g_value_set_double"
			case sema.TypeString:
				if bt.IsBinary() {
					setterFunctionName = "g_value_set_boxed"
				} else {
					setterFunctionName = "g_value_set_string"
				}
			default:
				emit.Throw("compiler error: unrecognized base type \"%s\" for struct member \"%s\"",
					memberType.Name(), memberName)
			}
		} else if memberType.IsEnum() {
			setterFunctionName = "g_value_set_int"
		} else if memberType.IsStruct() || memberType.IsXception() {
			setterFunctionName = "g_value_set_object"
		} else if memberType.IsContainer() {
			setterFunctionName = "g_value_set_boxed"
		} else {
			emit.Throw("compiler error: unrecognized type for struct member \"%s\"", memberName)
		}

		g.fTypesImpl.WriteString(g.indent() + "case " + propertyIdentifier + ":\n")
		g.indentUp()
		g.fTypesImpl.WriteString(g.indent() + setterFunctionName + " (value, self->" + memberName + ");\n" +
			g.indent() + "break;\n\n")
		g.indentDown()
	}
	g.fTypesImpl.WriteString(g.indent() + "default:\n")
	g.indentUp()
	g.fTypesImpl.WriteString(g.indent() + "G_OBJECT_WARN_INVALID_PROPERTY_ID (object, property_id, pspec);\n" +
		g.indent() + "break;\n")
	g.indentDown()
	g.scopeDown(&g.fTypesImpl)
	g.scopeDown(&g.fTypesImpl)
	g.fTypesImpl.WriteString("\n")
}

// generateObjectInstanceInit generates the instance init function.
func (g *Generator) generateObjectInstanceInit(name, nameU string, tstruct *sema.Struct) {
	members := tstruct.Members()

	g.fTypesImpl.WriteString("static void \n" + g.nspaceLC + nameU + "_instance_init (" +
		g.nspace + name + " * object)\n{\n")
	g.indentUp()

	// generate default-value structures for container-type members
	constantDeclarationOutput := false
	stringListConstantOutput := false
	for _, member := range members {
		memberValue := member.Value()
		if memberValue != nil {
			memberName := member.Name()
			memberType := sema.TrueType(member.Type())

			if memberType.IsList() {
				list := memberValue.List()
				elemType := memberType.(*sema.List).ElemType()

				// Generate an array with the list literal
				g.fTypesImpl.WriteString(g.indent() + "static " + g.typeName(elemType, false, true) +
					" __default_" + memberName + "[" + strconv.Itoa(len(list)) + "] = \n")
				g.indentUp()
				g.fTypesImpl.WriteString(g.indent() + g.constantLiteral(memberType, memberValue) + ";\n")
				g.indentDown()

				constantDeclarationOutput = true

				// If we are generating values for a pointer array (i.e. a list of
				// strings), set a flag so we know to also declare an index variable to
				// use in pre-populating the array
				if elemType.IsString() {
					stringListConstantOutput = true
				}
			}

			// TODO: Handle container types other than list
		}
	}
	if constantDeclarationOutput {
		if stringListConstantOutput {
			g.fTypesImpl.WriteString(g.indent() + "unsigned int list_index;\n")
		}
		g.fTypesImpl.WriteString("\n")
	}

	// satisfy compilers with -Wall turned on
	g.fTypesImpl.WriteString(g.indent() + "/* satisfy -Wall */\n" + g.indent() + "THRIFT_UNUSED_VAR (object);\n")

	for _, member := range members {
		memberType := member.Type()
		t := sema.TrueType(memberType)
		if t.IsBaseType() {
			dval := " = "
			if t.IsEnum() {
				dval += "(" + g.typeName(t, false, false) + ")"
			}
			cv := member.Value()
			if cv != nil {
				dval += g.constantValue("", t, cv)
			} else if t.IsString() {
				dval += "NULL"
			} else {
				dval += "0"
			}
			g.fTypesImpl.WriteString(g.indent() + "object->" + member.Name() + dval + ";\n")
		} else if t.IsStruct() {
			name2 := member.Name()
			typeProgram := memberType.Program()
			typeNspace := ""
			if typeProgram != nil {
				typeNspace = typeProgram.Namespace("c_glib")
			}
			typeNspacePrefix := ""
			if typeNspace != "" {
				typeNspacePrefix = initialCapsToUnderscores(typeNspace) + "_"
			}
			typeNameUC := toUpperCase(initialCapsToUnderscores(memberType.Name()))
			g.fTypesImpl.WriteString(g.indent() + "object->" + name2 + " = g_object_new (" +
				toUpperCase(typeNspacePrefix) + "TYPE_" + typeNameUC + ", NULL);\n")
		} else if t.IsXception() {
			name2 := member.Name()
			g.fTypesImpl.WriteString(g.indent() + "object->" + name2 + " = NULL;\n")
		} else if t.IsContainer() {
			name2 := member.Name()
			var initFunction string
			var etype sema.Type

			if t.IsMap() {
				m := t.(*sema.Map)
				initFunction = g.generateNewHashFromType(m.KeyType(), m.ValType())
			} else if t.IsSet() {
				etype = t.(*sema.Set).ElemType()
				initFunction = g.generateNewHashFromType(etype, nil)
			} else if t.IsList() {
				etype = t.(*sema.List).ElemType()
				initFunction = g.generateNewArrayFromType(etype)
			}

			g.fTypesImpl.WriteString(g.indent() + "object->" + name2 + " = " + initFunction + "\n")

			// Pre-populate the container with the specified default values, if any
			if member.Value() != nil {
				memberValue := member.Value()

				if t.IsList() {
					list := memberValue.List()

					if g.isNumeric(etype) {
						g.fTypesImpl.WriteString(g.indent() + "g_array_append_vals (object->" + name2 +
							", &__default_" + name2 + ", " + strconv.Itoa(len(list)) + ");\n")
					} else {
						g.fTypesImpl.WriteString(g.indent() + "for (list_index = 0; list_index < " +
							strconv.Itoa(len(list)) + "; list_index += 1)\n")
						g.indentUp()
						g.fTypesImpl.WriteString(g.indent() + "g_ptr_array_add (object->" + name2 + ",\n" +
							g.indent() + strings.Repeat(" ", 17) + "g_strdup (__default_" + name2 + "[list_index]));\n")
						g.indentDown()
					}
				}

				// TODO: Handle container types other than list
			}
		}

		/* if not required, initialize the __isset variable */
		if member.Req() != sema.Required {
			g.fTypesImpl.WriteString(g.indent() + "object->__isset_" + member.Name() + " = FALSE;\n")
		}
	}

	g.indentDown()
	g.fTypesImpl.WriteString("}\n\n")
}

// generateObjectFinalize creates the destructor.
func (g *Generator) generateObjectFinalize(name, nameU, nameUC string, tstruct *sema.Struct) {
	members := tstruct.Members()

	g.fTypesImpl.WriteString("static void \n" + g.nspaceLC + nameU + "_finalize (GObject *object)\n{\n")
	g.indentUp()

	g.fTypesImpl.WriteString(g.indent() + g.nspace + name + " *tobject = " + g.nspaceUC + nameUC + " (object);\n\n")

	g.fTypesImpl.WriteString(g.indent() + "/* satisfy -Wall in case we don't use tobject */\n" +
		g.indent() + "THRIFT_UNUSED_VAR (tobject);\n")

	for _, member := range members {
		t := sema.TrueType(member.Type())
		if t.IsContainer() {
			name2 := member.Name()
			if t.IsMap() || t.IsSet() {
				g.fTypesImpl.WriteString(g.indent() + "if (tobject->" + name2 + " != NULL)\n")
				g.fTypesImpl.WriteString(g.indent() + "{\n")
				g.indentUp()
				g.fTypesImpl.WriteString(g.indent() + "g_hash_table_destroy (tobject->" + name2 + ");\n")
				g.fTypesImpl.WriteString(g.indent() + "tobject->" + name2 + " = NULL;\n")
				g.indentDown()
				g.fTypesImpl.WriteString(g.indent() + "}\n")
			} else if t.IsList() {
				etype := t.(*sema.List).ElemType()
				destructorFunction := "g_ptr_array_unref"

				if etype.IsBaseType() {
					bt := etype.(*sema.BaseType)
					switch bt.Base() {
					case sema.TypeVoid:
						emit.Throw("compiler error: cannot determine array type")
					case sema.TypeBool, sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64, sema.TypeDouble:
						destructorFunction = "g_array_unref"
					case sema.TypeString:
					default:
						emit.Throw("compiler error: no array info for type")
					}
				} else if etype.IsEnum() {
					destructorFunction = "g_array_unref"
				}

				g.fTypesImpl.WriteString(g.indent() + "if (tobject->" + name2 + " != NULL)\n")
				g.fTypesImpl.WriteString(g.indent() + "{\n")
				g.indentUp()
				g.fTypesImpl.WriteString(g.indent() + destructorFunction + " (tobject->" + name2 + ");\n")
				g.fTypesImpl.WriteString(g.indent() + "tobject->" + name2 + " = NULL;\n")
				g.indentDown()
				g.fTypesImpl.WriteString(g.indent() + "}\n")
			}
		} else if t.IsStruct() || t.IsXception() {
			name2 := member.Name()
			g.fTypesImpl.WriteString(g.indent() + "if (tobject->" + name2 + " != NULL)\n")
			g.fTypesImpl.WriteString(g.indent() + "{\n")
			g.indentUp()
			g.fTypesImpl.WriteString(g.indent() + "g_object_unref(tobject->" + name2 + ");\n")
			g.fTypesImpl.WriteString(g.indent() + "tobject->" + name2 + " = NULL;\n")
			g.indentDown()
			g.fTypesImpl.WriteString(g.indent() + "}\n")
		} else if t.IsString() {
			name2 := member.Name()
			g.fTypesImpl.WriteString(g.indent() + "if (tobject->" + name2 + " != NULL)\n")
			g.fTypesImpl.WriteString(g.indent() + "{\n")
			g.indentUp()
			g.fTypesImpl.WriteString(g.indent() + g.generateFreeFuncFromType(t) + "(tobject->" + name2 + ");\n")
			g.fTypesImpl.WriteString(g.indent() + "tobject->" + name2 + " = NULL;\n")
			g.indentDown()
			g.fTypesImpl.WriteString(g.indent() + "}\n")
		}
	}

	g.indentDown()
	g.fTypesImpl.WriteString("}\n\n")
}

// generateObjectClassInit generates the class init function.
func (g *Generator) generateObjectClassInit(name, className, classNameLC, classNameUC string, tstruct *sema.Struct) {
	members := tstruct.Members()

	g.fTypesImpl.WriteString("static void\n" + classNameLC + "_class_init (" + className + "Class * cls)\n")
	g.scopeUp(&g.fTypesImpl)

	g.fTypesImpl.WriteString(g.indent() + "GObjectClass *gobject_class = G_OBJECT_CLASS (cls);\n" +
		g.indent() + "ThriftStructClass *struct_class = " + "THRIFT_STRUCT_CLASS (cls);\n\n" + g.indent() +
		"struct_class->read = " + classNameLC + "_read;\n" + g.indent() +
		"struct_class->write = " + classNameLC + "_write;\n\n" +
		g.indent() + "gobject_class->finalize = " + classNameLC + "_finalize;\n")
	if len(members) > 0 {
		g.fTypesImpl.WriteString(g.indent() + "gobject_class->get_property = " + classNameLC + "_get_property;\n" +
			g.indent() + "gobject_class->set_property = " + classNameLC + "_set_property;\n")

		for _, member := range members {
			memberName := member.Name()
			memberNameUC := toUpperCase(toLowerCase(initialCapsToUnderscores(memberName)))
			memberType := sema.TrueType(member.Type())
			memberValue := member.Value()

			propertyIdentifier := "PROP_" + classNameUC + "_" + memberNameUC

			g.fTypesImpl.WriteString("\n" + g.indent() + "g_object_class_install_property\n")
			g.indentUp()
			argsIndent := g.indent() + " "
			g.fTypesImpl.WriteString(g.indent() + "(gobject_class,\n" + argsIndent + propertyIdentifier + ",\n" + argsIndent)

			if memberType.IsBaseType() {
				bt := memberType.(*sema.BaseType)
				switch bt.Base() {
				case sema.TypeString:
					if bt.IsBinary() {
						ai := argsIndent + strings.Repeat(" ", 20)
						g.fTypesImpl.WriteString("g_param_spec_boxed (\"" + memberName + "\",\n" + ai +
							"NULL,\n" + ai + "NULL,\n" + ai + "G_TYPE_BYTE_ARRAY,\n" + ai + "G_PARAM_READWRITE));\n")
					} else {
						ai := argsIndent + strings.Repeat(" ", 21)
						defaultStr := "NULL"
						if memberValue != nil {
							defaultStr = "\"" + memberValue.String() + "\""
						}
						g.fTypesImpl.WriteString("g_param_spec_string (\"" + memberName + "\",\n" + ai +
							"NULL,\n" + ai + "NULL,\n" + ai + defaultStr + ",\n" + ai + "G_PARAM_READWRITE));\n")
					}
				case sema.TypeBool:
					ai := argsIndent + strings.Repeat(" ", 22)
					defaultStr := "FALSE"
					if memberValue != nil && memberValue.Integer() != 0 {
						defaultStr = "TRUE"
					}
					g.fTypesImpl.WriteString("g_param_spec_boolean (\"" + memberName + "\",\n" + ai +
						"NULL,\n" + ai + "NULL,\n" + ai + defaultStr + ",\n" + ai + "G_PARAM_READWRITE));\n")
				case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64, sema.TypeDouble:
					paramSpecFunctionName := "g_param_spec_int"
					var minValue, maxValue string
					switch bt.Base() {
					case sema.TypeI8:
						minValue, maxValue = "G_MININT8", "G_MAXINT8"
					case sema.TypeI16:
						minValue, maxValue = "G_MININT16", "G_MAXINT16"
					case sema.TypeI32:
						minValue, maxValue = "G_MININT32", "G_MAXINT32"
					case sema.TypeI64:
						paramSpecFunctionName = "g_param_spec_int64"
						minValue, maxValue = "G_MININT64", "G_MAXINT64"
					case sema.TypeDouble:
						paramSpecFunctionName = "g_param_spec_double"
						minValue, maxValue = "-INFINITY", "INFINITY"
					}

					// The C++ source computes this with
					// `base_type == TYPE_DOUBLE ? member_value->get_double() :
					// member_value->get_integer()`: since the two ternary
					// branches have different types (double and int64_t), C++
					// promotes the whole expression to double, so an integer
					// default is also formatted the ostream way a double is,
					// e.g. an i64 default of 10000000000 renders as "1e+10".
					var defaultValue string
					if memberValue != nil {
						v := memberValue.Integer()
						if bt.Base() == sema.TypeDouble {
							defaultValue = formatDouble(memberValue.Double())
						} else {
							defaultValue = formatDouble(float64(v))
						}
					} else {
						defaultValue = "0"
					}

					ai := argsIndent + strings.Repeat(" ", len(paramSpecFunctionName)+2)
					g.fTypesImpl.WriteString(paramSpecFunctionName + " (\"" + memberName + "\",\n" +
						ai + "NULL,\n" + ai + "NULL,\n" +
						ai + minValue + ",\n" + ai + maxValue + ",\n" +
						ai + defaultValue + ",\n" +
						ai + "G_PARAM_READWRITE));\n")
				default:
					emit.Throw("compiler error: unrecognized base type \"%s\" for struct member \"%s\"",
						memberType.Name(), memberName)
				}
				g.indentDown()
			} else if memberType.IsEnum() {
				e := memberType.(*sema.Enum)
				minValue, maxValue := int32(0), int32(0)
				if v := e.MinValue(); v != nil {
					minValue = v.Value()
				}
				if v := e.MaxValue(); v != nil {
					maxValue = v.Value()
				}

				ai := argsIndent + strings.Repeat(" ", 18)
				g.fTypesImpl.WriteString("g_param_spec_int (\"" + memberName + "\",\n" + ai +
					"NULL,\n" + ai + "NULL,\n" +
					ai + strconv.FormatInt(int64(minValue), 10) + ",\n" +
					ai + strconv.FormatInt(int64(maxValue), 10) + ",\n" +
					ai + strconv.FormatInt(int64(minValue), 10) + ",\n" +
					ai + "G_PARAM_READWRITE));\n")
				g.indentDown()
			} else if memberType.IsStruct() || memberType.IsXception() {
				typeProgram := memberType.Program()
				typeNspace := ""
				if typeProgram != nil {
					typeNspace = typeProgram.Namespace("c_glib")
				}
				typeNspacePrefix := ""
				if typeNspace != "" {
					typeNspacePrefix = initialCapsToUnderscores(typeNspace) + "_"
				}
				paramType := toUpperCase(typeNspacePrefix) + "TYPE_" + toUpperCase(initialCapsToUnderscores(memberType.Name()))

				ai := argsIndent + strings.Repeat(" ", 20)
				g.fTypesImpl.WriteString("g_param_spec_object (\"" + memberName + "\",\n" + ai +
					"NULL,\n" + ai + "NULL,\n" + ai + paramType + ",\n" + ai + "G_PARAM_READWRITE));\n")
				g.indentDown()
			} else if memberType.IsList() {
				elemType := memberType.(*sema.List).ElemType()
				paramType := "G_TYPE_PTR_ARRAY"
				if elemType.IsBaseType() && !elemType.IsString() {
					paramType = "G_TYPE_ARRAY"
				}

				ai := argsIndent + strings.Repeat(" ", 20)
				g.fTypesImpl.WriteString("g_param_spec_boxed (\"" + memberName + "\",\n" + ai +
					"NULL,\n" + ai + "NULL,\n" + ai + paramType + ",\n" + ai + "G_PARAM_READWRITE));\n")
				g.indentDown()
			} else if memberType.IsSet() || memberType.IsMap() {
				ai := argsIndent + strings.Repeat(" ", 20)
				g.fTypesImpl.WriteString("g_param_spec_boxed (\"" + memberName + "\",\n" + ai +
					"NULL,\n" + ai + "NULL,\n" + ai + "G_TYPE_HASH_TABLE,\n" + ai + "G_PARAM_READWRITE));\n")
				g.indentDown()
			}
		}
	}
	g.scopeDown(&g.fTypesImpl)
	g.fTypesImpl.WriteString("\n")

	nameU := initialCapsToUnderscores(name)
	g.fTypesImpl.WriteString("GType\n" + g.nspaceLC + nameU + "_get_type (void)\n{\n" +
		"  static GType type = 0;\n\n" +
		"  if (type == 0) \n" +
		"  {\n" +
		"    static const GTypeInfo type_info = \n" +
		"    {\n" +
		"      sizeof (" + g.nspace + name + "Class),\n" +
		"      NULL, /* base_init */\n" +
		"      NULL, /* base_finalize */\n" +
		"      (GClassInitFunc) " + g.nspaceLC + nameU + "_class_init,\n" +
		"      NULL, /* class_finalize */\n" +
		"      NULL, /* class_data */\n" +
		"      sizeof (" + g.nspace + name + "),\n" +
		"      0, /* n_preallocs */\n" +
		"      (GInstanceInitFunc) " + g.nspaceLC + nameU + "_instance_init,\n" +
		"      NULL, /* value_table */\n" +
		"    };\n\n" +
		"    type = g_type_register_static (THRIFT_TYPE_STRUCT, \n" +
		"                                   \"" + g.nspace + name + "Type\",\n" +
		"                                   &type_info, 0);\n" +
		"  }\n\n" +
		"  return type;\n}\n\n")
}

// generateStructWriter generates functions to write Thrift structures to
// a stream.
func (g *Generator) generateStructWriter(out *strings.Builder, tstruct *sema.Struct, thisName, thisGet string, isFunction bool) {
	name := tstruct.Name()
	nameU := initialCapsToUnderscores(name)

	fields := tstruct.Members()
	errorRet := "0"

	if isFunction {
		errorRet = "-1"
		out.WriteString(g.indent() + "static gint32\n" + g.nspaceLC + nameU +
			"_write (ThriftStruct *object, ThriftProtocol *protocol, GError **error)\n")
	}
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	out.WriteString(g.indent() + "gint32 ret;\n" + g.indent() + "gint32 xfer = 0;\n\n")

	out.WriteString(g.indent() + thisGet + "\n")
	// satisfy -Wall in the case of an empty struct
	if thisGet != "" {
		out.WriteString(g.indent() + "THRIFT_UNUSED_VAR (this_object);\n")
	}

	out.WriteString(g.indent() + "if ((ret = thrift_protocol_write_struct_begin (protocol, \"" + name +
		"\", error)) < 0)\n" + g.indent() + "  return " + errorRet + ";\n" +
		g.indent() + "xfer += ret;\n")

	for _, f := range fields {
		if f.Req() == sema.Optional {
			out.WriteString(g.indent() + "if (this_object->__isset_" + f.Name() + " == TRUE) {\n")
			g.indentUp()
		}

		out.WriteString(g.indent() + "if ((ret = thrift_protocol_write_field_begin (protocol, " +
			"\"" + f.Name() + "\", " + g.typeToEnum(f.Type()) + ", " +
			strconv.FormatInt(int64(f.Key()), 10) + ", error)) < 0)\n" +
			g.indent() + "  return " + errorRet + ";\n" + g.indent() + "xfer += ret;\n")
		g.generateSerializeField(out, f, thisName, "", errorRet)
		out.WriteString(g.indent() + "if ((ret = thrift_protocol_write_field_end (protocol, error)) < 0)\n" +
			g.indent() + "  return " + errorRet + ";\n" + g.indent() + "xfer += ret;\n")

		if f.Req() == sema.Optional {
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
	}

	// write the struct map
	out.WriteString(g.indent() + "if ((ret = thrift_protocol_write_field_stop (protocol, error)) < 0)\n" +
		g.indent() + "  return " + errorRet + ";\n" + g.indent() + "xfer += ret;\n" +
		g.indent() + "if ((ret = thrift_protocol_write_struct_end (protocol, error)) < 0)\n" +
		g.indent() + "  return " + errorRet + ";\n" + g.indent() + "xfer += ret;\n\n")

	if isFunction {
		out.WriteString(g.indent() + "return xfer;\n")
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateStructReader generates code to read Thrift structures from a
// stream.
func (g *Generator) generateStructReader(out *strings.Builder, tstruct *sema.Struct, thisName, thisGet string, isFunction bool) {
	name := tstruct.Name()
	nameU := initialCapsToUnderscores(name)
	errorRet := "0"
	fields := tstruct.Members()

	if isFunction {
		errorRet = "-1"
		out.WriteString(g.indent() + "/* reads a " + nameU + " object */\nstatic gint32\n" +
			g.nspaceLC + nameU +
			"_read (ThriftStruct *object, ThriftProtocol *protocol, GError **error)\n")
	}

	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	// declare stack temp variables
	out.WriteString(g.indent() + "gint32 ret;\n" + g.indent() + "gint32 xfer = 0;\n" +
		g.indent() + "gchar *name = NULL;\n" + g.indent() + "ThriftType ftype;\n" +
		g.indent() + "gint16 fid;\n" + g.indent() + "guint32 len = 0;\n" +
		g.indent() + "gpointer data = NULL;\n" + g.indent() + thisGet + "\n")

	for _, f := range fields {
		if f.Req() == sema.Required {
			out.WriteString(g.indent() + "gboolean isset_" + f.Name() + " = FALSE;\n")
		}
	}

	out.WriteString("\n")

	// satisfy -Wall in case we don't use some variables
	out.WriteString(g.indent() + "/* satisfy -Wall in case these aren't used */\n" +
		g.indent() + "THRIFT_UNUSED_VAR (len);\n" + g.indent() + "THRIFT_UNUSED_VAR (data);\n")

	if thisGet != "" {
		out.WriteString(g.indent() + "THRIFT_UNUSED_VAR (this_object);\n")
	}
	out.WriteString("\n")

	// read the beginning of the structure marker
	out.WriteString(g.indent() + "/* read the struct begin marker */\n" +
		g.indent() + "if ((ret = thrift_protocol_read_struct_begin (protocol, &name, error)) < 0)\n" +
		g.indent() + "{\n" + g.indent() + "  if (name) g_free (name);\n" +
		g.indent() + "  return " + errorRet + ";\n" + g.indent() + "}\n" +
		g.indent() + "xfer += ret;\n" + g.indent() + "if (name) g_free (name);\n" +
		g.indent() + "name = NULL;\n\n")

	// read the struct fields
	out.WriteString(g.indent() + "/* read the struct fields */\n" + g.indent() + "while (1)\n")
	g.scopeUp(out)

	// read beginning field marker
	out.WriteString(g.indent() + "/* read the beginning of a field */\n" +
		g.indent() + "if ((ret = thrift_protocol_read_field_begin (protocol, &name, &ftype, &fid, error)) < 0)\n" +
		g.indent() + "{\n" + g.indent() + "  if (name) g_free (name);\n" +
		g.indent() + "  return " + errorRet + ";\n" + g.indent() + "}\n" +
		g.indent() + "xfer += ret;\n" + g.indent() + "if (name) g_free (name);\n" +
		g.indent() + "name = NULL;\n\n")

	// check for field STOP marker
	out.WriteString(g.indent() + "/* break if we get a STOP field */\n" +
		g.indent() + "if (ftype == T_STOP)\n" + g.indent() + "{\n" +
		g.indent() + "  break;\n" + g.indent() + "}\n\n")

	// switch depending on the field type
	out.WriteString(g.indent() + "switch (fid)\n")

	// start switch
	g.scopeUp(out)

	// generate deserialization code for known types
	for _, f := range fields {
		out.WriteString(g.indent() + "case " + strconv.FormatInt(int64(f.Key()), 10) + ":\n")
		g.indentUp()
		out.WriteString(g.indent() + "if (ftype == " + g.typeToEnum(f.Type()) + ")\n")
		out.WriteString(g.indent() + "{\n")

		g.indentUp()
		// generate deserialize field
		g.generateDeserializeField(out, f, thisName, "", errorRet, false)
		g.indentDown()

		out.WriteString(g.indent() + "} else {\n" + g.indent() +
			"  if ((ret = thrift_protocol_skip (protocol, ftype, error)) < 0)\n" + g.indent() +
			"    return " + errorRet + ";\n" + g.indent() + "  xfer += ret;\n" +
			g.indent() + "}\n" + g.indent() + "break;\n")
		g.indentDown()
	}

	// create the default case
	out.WriteString(g.indent() + "default:\n" + g.indent() +
		"  if ((ret = thrift_protocol_skip (protocol, ftype, error)) < 0)\n" + g.indent() +
		"    return " + errorRet + ";\n" + g.indent() + "  xfer += ret;\n" +
		g.indent() + "  break;\n")

	// end switch
	g.scopeDown(out)

	// read field end marker
	out.WriteString(g.indent() + "if ((ret = thrift_protocol_read_field_end (protocol, error)) < 0)\n" +
		g.indent() + "  return " + errorRet + ";\n" + g.indent() + "xfer += ret;\n")

	// end while loop
	g.scopeDown(out)
	out.WriteString("\n")

	// read the end of the structure
	out.WriteString(g.indent() + "if ((ret = thrift_protocol_read_struct_end (protocol, error)) < 0)\n" +
		g.indent() + "  return " + errorRet + ";\n" + g.indent() + "xfer += ret;\n\n")

	// if a required field is missing, throw an error
	for _, f := range fields {
		if f.Req() == sema.Required {
			out.WriteString(g.indent() + "if (!isset_" + f.Name() + ")\n" + g.indent() + "{\n" +
				g.indent() + "  g_set_error (error, THRIFT_PROTOCOL_ERROR,\n" + g.indent() +
				"               THRIFT_PROTOCOL_ERROR_INVALID_DATA,\n" + g.indent() +
				"               \"missing field\");\n" + g.indent() + "  return -1;\n" +
				g.indent() + "}\n\n")
		}
	}

	if isFunction {
		out.WriteString(g.indent() + "return xfer;\n")
	}

	// end the function/structure
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}
