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
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateStruct is generate_struct.
func (g *Generator) generateStruct(s *sema.Struct) {
	g.collectExtensionsTypesStruct(s)

	if g.isUnionEnabled() && s.IsUnion() {
		g.generateNetstdUnion(s)
	} else {
		g.generateNetstdStruct(s, false)
	}
}

// generateXception is generate_xception.
func (g *Generator) generateXception(x *sema.Struct) {
	g.generateNetstdStruct(x, true)
}

// generateNetstdStruct is generate_netstd_struct: one file per struct.
func (g *Generator) generateNetstdStruct(s *sema.Struct, isException bool) {
	fStructName := g.namespaceDir + "/" + s.Name() + ".cs"
	var f strings.Builder

	g.resetIndent()
	f.WriteString(g.autogenComment() + g.netstdTypeUsings() + g.netstdThriftUsings() + "\n\n")

	g.pragmasAndDirectives(&f)
	g.generateNetstdStructDefinition(&f, s, isException, false, false)

	emit.WriteFile(fStructName, f.String())
}

// generateNetstdStructDefinition is generate_netstd_struct_definition.
func (g *Generator) generateNetstdStructDefinition(out *strings.Builder, s *sema.Struct, isException, inClass, isResult bool) {
	if !inClass {
		g.startNetstdNamespace(out)
	}

	out.WriteString("\n")

	g.netstdDoc(out, s)
	g.collectExtensionsTypesStruct(s)
	g.prepareMemberNameMappingStruct(s)

	if (g.isSerializeEnabled() || g.isWCFEnabled()) && !isException {
		out.WriteString(g.indent() + "[DataContract(Namespace=\"" + g.opts.WCFNamespace + "\")]\n")
	}

	isFinal := s.Annotations().Has("final")

	sharpStructName := g.typeName(s, false)

	g.generateDeprecationAttribute(out, s.Annotations())
	out.WriteString(g.indent() + "public ")
	if isFinal {
		out.WriteString("sealed ")
	}
	out.WriteString("partial class " + sharpStructName + " : ")

	if isException {
		out.WriteString("TException, ")
	}

	out.WriteString("TBase\n" + g.indent() + "{\n")
	g.indentUp()

	members := s.Members()

	// make private members with public Properties
	for _, m := range members {
		// if the field is required, then we use auto-properties
		if !fieldIsRequired(m) {
			out.WriteString(g.indent() + "private " + g.declareField(m, false, true, "_") + "\n")
		}
	}
	out.WriteString("\n")

	hasNonRequiredFields := false
	hasRequiredFields := false
	for _, m := range members {
		g.netstdDocField(out, m)
		g.generateProperty(out, m, true, true)
		if fieldIsRequired(m) {
			hasRequiredFields = true
		} else {
			hasNonRequiredFields = true
		}
	}

	generateIsset := hasNonRequiredFields
	if generateIsset {
		out.WriteString("\n")
		if g.isSerializeEnabled() || g.isWCFEnabled() {
			out.WriteString(g.indent() + "[DataMember(Order = 1)]\n")
		}
		out.WriteString(g.indent() + "public Isset __isset;\n")
		if g.isSerializeEnabled() || g.isWCFEnabled() {
			out.WriteString(g.indent() + "[DataContract]\n")
		}

		out.WriteString(g.indent() + "public struct Isset\n" + g.indent() + "{\n")
		g.indentUp()

		for _, m := range members {
			// if it is required, don't need Isset for that variable
			// if it is not required, if it has a default value, we need to generate Isset
			if !fieldIsRequired(m) {
				if g.isSerializeEnabled() || g.isWCFEnabled() {
					out.WriteString(g.indent() + "[DataMember]\n")
				}
				out.WriteString(g.indent() + "public bool " + getIssetName(g.normalizeName(m.Name(), false)) + ";\n")
			}
		}

		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")

		if generateIsset && (g.isSerializeEnabled() || g.isWCFEnabled()) {
			out.WriteString(g.indent() + "#region XmlSerializer support\n\n")

			for _, m := range members {
				if !fieldIsRequired(m) {
					out.WriteString(g.indent() + "public bool ShouldSerialize" + g.propName(m, false) + "()\n" + g.indent() + "{\n")
					g.indentUp()
					out.WriteString(g.indent() + "return __isset." + getIssetName(g.normalizeName(m.Name(), false)) + ";\n")
					g.indentDown()
					out.WriteString(g.indent() + "}\n\n")
				}
			}

			out.WriteString(g.indent() + "#endregion XmlSerializer support\n\n")
		}
	}

	// We always want a default, no argument constructor for Reading
	out.WriteString(g.indent() + "public " + sharpStructName + "()\n" + g.indent() + "{\n")
	g.indentUp()

	for _, m := range members {
		t := trueType(m.Type())

		if m.Value() != nil {
			if fieldIsRequired(m) {
				g.printConstValue(out, "this."+g.normalizeName(g.propName(m, false), false), t, m.Value(), true, true, false)
			} else {
				g.printConstValue(out, "this._"+m.Name(), t, m.Value(), true, true, false)
				// Optionals with defaults are marked set
				out.WriteString(g.indent() + "this.__isset." + getIssetName(g.normalizeName(m.Name(), false)) + " = true;\n")
			}
		}
	}
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	// if we have required fields, we add that CTOR too
	if hasRequiredFields {
		out.WriteString(g.indent() + "public " + sharpStructName + "(")
		first := true
		for _, m := range members {
			if fieldIsRequired(m) {
				if first {
					first = false
				} else {
					out.WriteString(", ")
				}
				out.WriteString(g.typeName(m.Type(), true) + g.nullableFieldSuffixField(m) + " " + g.normalizeName(m.Name(), false))
			}
		}
		out.WriteString(") : this()\n" + g.indent() + "{\n")
		g.indentUp()

		for _, m := range members {
			if fieldIsRequired(m) {
				out.WriteString(g.indent() + "this." + g.propName(m, false) + " = " + g.normalizeName(m.Name(), false) + ";\n")
			}
		}

		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
	}

	// DeepCopy()
	g.generateNetstdDeepcopyMethod(out, s, sharpStructName)

	g.generateNetstdStructReader(out, s)
	if isResult {
		g.generateNetstdStructResultWriter(out, s)
	} else {
		g.generateNetstdStructWriter(out, s)
	}
	g.generateNetstdStructEquals(out, s)
	g.generateNetstdStructHashcode(out, s)
	g.generateNetstdStructTostring(out, s)

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	// generate a corresponding WCF fault to wrap the exception
	if (g.isSerializeEnabled() || g.isWCFEnabled()) && isException {
		g.generateNetstdWcffault(out, s)
	}

	g.cleanupMemberNameMapping()
	if !inClass {
		g.endNetstdNamespace(out)
	}
}

// generateNetstdWcffault is generate_netstd_wcffault.
func (g *Generator) generateNetstdWcffault(out *strings.Builder, s *sema.Struct) {
	out.WriteString("\n")
	out.WriteString(g.indent() + "[DataContract]\n")

	isFinal := s.Annotations().Has("final")

	g.generateDeprecationAttribute(out, s.Annotations())
	out.WriteString(g.indent() + "public ")
	if isFinal {
		out.WriteString("sealed ")
	}
	out.WriteString("partial class " + g.typeName(s, false) + "Fault\n" + g.indent() + "{\n")
	g.indentUp()

	members := s.Members()

	// make private members with public Properties
	for _, m := range members {
		if !fieldIsRequired(m) {
			out.WriteString(g.indent() + "private " + g.declareField(m, false, true, "_") + "\n")
		}
	}
	out.WriteString("\n")

	for _, m := range members {
		g.generateProperty(out, m, true, false)
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateNetstdDeepcopyMethod is generate_netstd_deepcopy_method.
func (g *Generator) generateNetstdDeepcopyMethod(out *strings.Builder, s *sema.Struct, sharpStructName string) {
	if g.opts.NoDeepcopy {
		return // feature disabled
	}

	members := s.Members()

	out.WriteString(g.indent() + "public " + sharpStructName + " " + deepCopyMethodName + "()\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	// return directly if there are only required fields
	tmpInstance := g.tmp("tmp")
	out.WriteString(g.indent() + "var " + tmpInstance + " = new " + sharpStructName + "()")
	inlineAssignment := g.opts.TargetNetVersion >= 6
	if inlineAssignment {
		out.WriteString("\n" + g.indent() + "{\n")
		g.indentUp()
	} else {
		out.WriteString(";\n")
	}

	for _, m := range members {
		needsTypecast := false
		ttype := m.Type()
		copyOp := g.deepCopyExpression("this."+g.propName(m, false), ttype, true, &needsTypecast, false)

		isRequired := fieldIsRequired(m)
		nullAllowed := typeCanBeNull(m.Type())

		if inlineAssignment {
			if nullAllowed || !isRequired { // = has isset
				g.indentDown()
				out.WriteString(g.indent() + "};\n")
				inlineAssignment = false
			}
		}

		g.generateNullCheckBegin(out, m)

		out.WriteString(g.indent())
		if !inlineAssignment {
			out.WriteString(tmpInstance + ".")
		}
		out.WriteString(g.propName(m, false) + " = ")
		if needsTypecast {
			out.WriteString("(" + g.tn(ttype) + ")")
		}
		out.WriteString(copyOp)
		if inlineAssignment {
			out.WriteString(",\n")
		} else {
			out.WriteString(";\n")
		}

		g.generateNullCheckEnd(out, m)
		if !isRequired {
			out.WriteString(g.indent())
			if !inlineAssignment {
				out.WriteString(tmpInstance + ".")
			}
			out.WriteString("__isset." + getIssetName(g.normalizeName(m.Name(), false)))
			out.WriteString(" = this.__isset." + getIssetName(g.normalizeName(m.Name(), false)))
			if inlineAssignment {
				out.WriteString(",\n")
			} else {
				out.WriteString(";\n")
			}
		}
	}

	if inlineAssignment {
		g.indentDown()
		out.WriteString(g.indent() + "};\n")
	}

	out.WriteString(g.indent() + "return " + tmpInstance + ";\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateNetstdStructReader is generate_netstd_struct_reader.
func (g *Generator) generateNetstdStructReader(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "public async global::System.Threading.Tasks.Task ReadAsync(TProtocol iprot, CancellationToken " + cancellationTokenName + ")\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	out.WriteString(g.indent() + "iprot.IncrementRecursionDepth();\n")
	out.WriteString(g.indent() + "try\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	fields := s.Members()

	// Required variables aren't in __isset, so we need tmp vars to check them
	for _, f := range fields {
		if fieldIsRequired(f) {
			out.WriteString(g.indent() + "bool isset_" + f.Name() + " = false;\n")
		}
	}

	out.WriteString(g.indent() + "TField field;\n")
	out.WriteString(g.indent() + "await iprot.ReadStructBeginAsync(" + cancellationTokenName + ");\n")
	out.WriteString(g.indent() + "while (true)\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	out.WriteString(g.indent() + "field = await iprot.ReadFieldBeginAsync(" + cancellationTokenName + ");\n")
	out.WriteString(g.indent() + "if (field.Type == TType.Stop)\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	out.WriteString(g.indent() + "break;\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
	out.WriteString(g.indent() + "switch (field.ID)\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	for _, f := range fields {
		isRequired := fieldIsRequired(f)
		out.WriteString(g.indent() + "case " + itoa(int64(f.Key())) + ":\n")
		g.indentUp()
		out.WriteString(g.indent() + "if (field.Type == " + g.typeToEnum(f.Type()) + ")\n")
		out.WriteString(g.indent() + "{\n")
		g.indentUp()

		g.generateDeserializeField(out, f, "", false)
		if isRequired {
			out.WriteString(g.indent() + "isset_" + f.Name() + " = true;\n")
		}

		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "else\n")
		out.WriteString(g.indent() + "{\n")
		g.indentUp()
		out.WriteString(g.indent() + "await TProtocolUtil.SkipAsync(iprot, field.Type, " + cancellationTokenName + ");\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "break;\n")
		g.indentDown()
	}

	out.WriteString(g.indent() + "default: \n")
	g.indentUp()
	out.WriteString(g.indent() + "await TProtocolUtil.SkipAsync(iprot, field.Type, " + cancellationTokenName + ");\n")
	out.WriteString(g.indent() + "break;\n")
	g.indentDown()
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
	out.WriteString(g.indent() + "await iprot.ReadFieldEndAsync(" + cancellationTokenName + ");\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
	out.WriteString(g.indent() + "await iprot.ReadStructEndAsync(" + cancellationTokenName + ");\n")

	for _, f := range fields {
		if fieldIsRequired(f) {
			out.WriteString(g.indent() + "if (!isset_" + f.Name() + ")\n")
			out.WriteString(g.indent() + "{\n")
			g.indentUp()
			out.WriteString(g.indent() + "throw new TProtocolException(TProtocolException.INVALID_DATA);\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "finally\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	out.WriteString(g.indent() + "iprot.DecrementRecursionDepth();\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateNullCheckBegin is generate_null_check_begin.
func (g *Generator) generateNullCheckBegin(out *strings.Builder, f *sema.Field) {
	isRequired := fieldIsRequired(f)
	nullAllowed := typeCanBeNull(f.Type())

	if nullAllowed || !isRequired {
		first := true
		out.WriteString(g.indent() + "if(")

		if nullAllowed {
			out.WriteString("(" + g.propName(f, false) + " != null)")
			first = false
		}

		if !isRequired {
			if !first {
				out.WriteString(" && ")
			}
			out.WriteString("__isset." + getIssetName(g.normalizeName(f.Name(), false)))
		}

		out.WriteString(")\n" + g.indent() + "{\n")
		g.indentUp()
	}
}

// generateNullCheckEnd is generate_null_check_end.
func (g *Generator) generateNullCheckEnd(out *strings.Builder, f *sema.Field) {
	isRequired := fieldIsRequired(f)
	nullAllowed := typeCanBeNull(f.Type())

	if nullAllowed || !isRequired {
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}
}

// generateNetstdStructWriter is generate_netstd_struct_writer.
func (g *Generator) generateNetstdStructWriter(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "public async global::System.Threading.Tasks.Task WriteAsync(TProtocol oprot, CancellationToken " + cancellationTokenName + ")\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	out.WriteString(g.indent() + "oprot.IncrementRecursionDepth();\n")
	out.WriteString(g.indent() + "try\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	name := s.Name()
	fields := s.SortedMembers()

	tmpvar := g.tmp("tmp")
	out.WriteString(g.indent() + "var " + tmpvar + " = new TStruct(\"" + name + "\");\n")
	out.WriteString(g.indent() + "await oprot.WriteStructBeginAsync(" + tmpvar + ", " + cancellationTokenName + ");\n")

	if len(fields) > 0 {
		tmpvar = g.tmp("tmp")
		if g.opts.TargetNetVersion >= 8 {
			out.WriteString(g.indent() + "#pragma warning disable IDE0017  // simplified init\n")
		}
		out.WriteString(g.indent() + "var " + tmpvar + " = new TField();\n")
		for _, f := range fields {
			g.generateNullCheckBegin(out, f)
			out.WriteString(g.indent() + tmpvar + ".Name = \"" + f.Name() + "\";\n")
			out.WriteString(g.indent() + tmpvar + ".Type = " + g.typeToEnum(f.Type()) + ";\n")
			out.WriteString(g.indent() + tmpvar + ".ID = " + itoa(int64(f.Key())) + ";\n")
			out.WriteString(g.indent() + "await oprot.WriteFieldBeginAsync(" + tmpvar + ", " + cancellationTokenName + ");\n")

			g.generateSerializeField(out, f, "", false, true)

			out.WriteString(g.indent() + "await oprot.WriteFieldEndAsync(" + cancellationTokenName + ");\n")
			g.generateNullCheckEnd(out, f)
		}
		if g.opts.TargetNetVersion >= 8 {
			out.WriteString(g.indent() + "#pragma warning restore IDE0017  // simplified init\n")
		}
	}

	out.WriteString(g.indent() + "await oprot.WriteFieldStopAsync(" + cancellationTokenName + ");\n")
	out.WriteString(g.indent() + "await oprot.WriteStructEndAsync(" + cancellationTokenName + ");\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "finally\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	out.WriteString(g.indent() + "oprot.DecrementRecursionDepth();\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateNetstdStructResultWriter is generate_netstd_struct_result_writer.
func (g *Generator) generateNetstdStructResultWriter(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "public async global::System.Threading.Tasks.Task WriteAsync(TProtocol oprot, CancellationToken " + cancellationTokenName + ")\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	out.WriteString(g.indent() + "oprot.IncrementRecursionDepth();\n")
	out.WriteString(g.indent() + "try\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	name := s.Name()
	fields := s.SortedMembers()

	tmpvar := g.tmp("tmp")
	out.WriteString(g.indent() + "var " + tmpvar + " = new TStruct(\"" + name + "\");\n")
	out.WriteString(g.indent() + "await oprot.WriteStructBeginAsync(" + tmpvar + ", " + cancellationTokenName + ");\n")

	if len(fields) > 0 {
		tmpvar = g.tmp("tmp")
		if g.opts.TargetNetVersion >= 8 {
			out.WriteString(g.indent() + "#pragma warning disable IDE0017  // simplified init\n")
		}
		out.WriteString(g.indent() + "var " + tmpvar + " = new TField();\n")
		first := true
		for _, f := range fields {
			if first {
				first = false
				out.WriteString("\n" + g.indent() + "if")
			} else {
				out.WriteString(g.indent() + "else if")
			}

			out.WriteString("(this.__isset." + getIssetName(g.normalizeName(f.Name(), false)) + ")\n")
			out.WriteString(g.indent() + "{\n")
			g.indentUp()

			nullAllowed := typeCanBeNull(f.Type())
			if nullAllowed {
				out.WriteString(g.indent() + "if (" + g.propName(f, false) + " != null)\n")
				out.WriteString(g.indent() + "{\n")
				g.indentUp()
			}

			out.WriteString(g.indent() + tmpvar + ".Name = \"" + g.propName(f, false) + "\";\n")
			out.WriteString(g.indent() + tmpvar + ".Type = " + g.typeToEnum(f.Type()) + ";\n")
			out.WriteString(g.indent() + tmpvar + ".ID = " + itoa(int64(f.Key())) + ";\n")
			out.WriteString(g.indent() + "await oprot.WriteFieldBeginAsync(" + tmpvar + ", " + cancellationTokenName + ");\n")

			g.generateSerializeField(out, f, "", false, true)

			out.WriteString(g.indent() + "await oprot.WriteFieldEndAsync(" + cancellationTokenName + ");\n")

			if nullAllowed {
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
			}

			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
		if g.opts.TargetNetVersion >= 8 {
			out.WriteString(g.indent() + "#pragma warning restore IDE0017  // simplified init\n")
		}
	}

	out.WriteString(g.indent() + "await oprot.WriteFieldStopAsync(" + cancellationTokenName + ");\n")
	out.WriteString(g.indent() + "await oprot.WriteStructEndAsync(" + cancellationTokenName + ");\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "finally\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	out.WriteString(g.indent() + "oprot.DecrementRecursionDepth();\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateNetstdStructTostring is generate_netstd_struct_tostring.
func (g *Generator) generateNetstdStructTostring(out *strings.Builder, s *sema.Struct) {
	tmpvar := g.tmp("tmp")
	out.WriteString(g.indent() + "public override string ToString()\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	out.WriteString(g.indent() + "var " + tmpvar + " = new StringBuilder(\"" + s.Name() + "(\");\n")

	fields := s.Members()

	useFirstFlag := false
	tmpCount := g.tmp("tmp")
	for _, f := range fields {
		if !fieldIsRequired(f) {
			out.WriteString(g.indent() + "int " + tmpCount + " = 0;\n")
			useFirstFlag = true
		}
		break
	}

	hadRequired := false // set to true after first required field has been processed

	for _, f := range fields {
		isRequired := fieldIsRequired(f)
		g.generateNullCheckBegin(out, f)

		if useFirstFlag && !hadRequired {
			incr := ""
			if !isRequired {
				incr = "++"
			}
			out.WriteString(g.indent() + "if(0 < " + tmpCount + incr + ") { " + tmpvar + ".Append(\", \"); }\n")
			out.WriteString(g.indent() + tmpvar + ".Append(\"" + g.propName(f, false) + ": \");\n")
		} else {
			out.WriteString(g.indent() + tmpvar + ".Append(\", " + g.propName(f, false) + ": \");\n")
		}

		out.WriteString(g.indent() + g.propName(f, false) + ".ToString(" + tmpvar + ");\n")

		g.generateNullCheckEnd(out, f)
		if isRequired {
			hadRequired = true // now __count must be > 0, so we don't need to check it anymore
		}
	}

	out.WriteString(g.indent() + tmpvar + ".Append(')');\n")
	out.WriteString(g.indent() + "return " + tmpvar + ".ToString();\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

// generateNetstdStructEquals is generate_netstd_struct_equals.
func (g *Generator) generateNetstdStructEquals(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "public override bool Equals(object" + g.nullableSuffix() + " that)\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	if g.opts.TargetNetVersion >= 6 {
		out.WriteString(g.indent() + "if (that is not " + g.typeName(s, false) + " other) return false;\n")
	} else {
		out.WriteString(g.indent() + "if (!(that is " + g.typeName(s, false) + " other)) return false;\n")
	}
	out.WriteString(g.indent() + "if (ReferenceEquals(this, other)) return true;\n")

	fields := s.Members()

	first := true

	for _, f := range fields {
		if first {
			first = false
			out.WriteString(g.indent() + "return ")
			g.indentUp()
		} else {
			out.WriteString("\n")
			out.WriteString(g.indent() + "&& ")
		}
		if !fieldIsRequired(f) {
			isset := getIssetName(g.normalizeName(f.Name(), false))
			out.WriteString("((__isset." + isset + " == other.__isset." + isset + ") && ((!__isset." + isset + ") || (")
		}
		t := f.Type()
		if t.IsContainer() || t.IsBinary() {
			out.WriteString("TCollections.Equals(")
		} else {
			out.WriteString("global::System.Object.Equals(")
		}
		out.WriteString(g.propName(f, false) + ", other." + g.propName(f, false) + ")")
		if !fieldIsRequired(f) {
			out.WriteString(")))")
		}
	}
	if first {
		out.WriteString(g.indent() + "return true;\n")
	} else {
		out.WriteString(";\n")
		g.indentDown()
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateNetstdStructHashcode is generate_netstd_struct_hashcode.
func (g *Generator) generateNetstdStructHashcode(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "public override int GetHashCode() {\n")
	g.indentUp()

	out.WriteString(g.indent() + "int hashcode = 157;\n")
	out.WriteString(g.indent() + "unchecked {\n")
	g.indentUp()

	fields := s.Members()

	for _, f := range fields {
		t := f.Type()

		g.generateNullCheckBegin(out, f)
		out.WriteString(g.indent() + "hashcode = (hashcode * 397) + ")
		if t.IsContainer() {
			out.WriteString("TCollections.GetHashCode(" + g.propName(f, false) + ")")
		} else {
			out.WriteString(g.propName(f, false) + ".GetHashCode()")
		}
		out.WriteString(";\n")

		g.generateNullCheckEnd(out, f)
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "return hashcode;\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateProperty is generate_property.
func (g *Generator) generateProperty(out *strings.Builder, f *sema.Field, isPublic, generateIsset bool) {
	g.generateNetstdProperty(out, f, isPublic, generateIsset, "_")
}

// generateNetstdProperty is generate_netstd_property.
func (g *Generator) generateNetstdProperty(out *strings.Builder, f *sema.Field, isPublic, generateIsset bool, fieldPrefix string) {
	if (g.isSerializeEnabled() || g.isWCFEnabled()) && isPublic {
		out.WriteString(g.indent() + "[DataMember(Order = 0)]\n")
	}
	g.generateDeprecationAttribute(out, f.Annotations())

	access := "private "
	if isPublic {
		access = "public "
	}
	out.WriteString(g.indent() + access + g.typeName(f.Type(), true) + g.nullableFieldSuffixField(f) + " " + g.propName(f, false))

	isRequired := fieldIsRequired(f)
	if isRequired {
		out.WriteString(" { get; set; }")
		if g.opts.TargetNetVersion >= 6 && !g.forceMemberNullable(f) {
			out.WriteString(g.initializeField(f) + ";")
		}
		out.WriteString("\n")
	} else {
		out.WriteString("\n" + g.indent() + "{\n")
		g.indentUp()

		out.WriteString(g.indent() + "get\n" + g.indent() + "{\n")
		g.indentUp()

		out.WriteString(g.indent() + "return " + fieldPrefix + f.Name() + ";\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n" + g.indent() + "set\n" + g.indent() + "{\n")
		g.indentUp()

		if generateIsset {
			out.WriteString(g.indent() + "__isset." + getIssetName(g.normalizeName(f.Name(), false)) + " = true;\n")
		}
		out.WriteString(g.indent() + "this." + fieldPrefix + f.Name() + " = value;\n")

		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}
	out.WriteString("\n")
}
