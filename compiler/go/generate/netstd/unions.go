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

// generateNetstdUnion is generate_netstd_union: one file per union.
func (g *Generator) generateNetstdUnion(u *sema.Struct) {
	fUnionName := g.namespaceDir + "/" + u.Name() + ".cs"
	var f strings.Builder

	g.resetIndent()
	f.WriteString(g.autogenComment() + g.netstdTypeUsings() + g.netstdThriftUsings() + "\n\n")

	g.pragmasAndDirectives(&f)
	g.generateNetstdUnionDefinition(&f, u)

	emit.WriteFile(fUnionName, f.String())
}

// generateNetstdUnionDefinition is generate_netstd_union_definition.
func (g *Generator) generateNetstdUnionDefinition(out *strings.Builder, u *sema.Struct) {
	// Let's define the class first
	g.startNetstdNamespace(out)

	g.generateDeprecationAttribute(out, u.Annotations())
	out.WriteString(g.indent() + "public abstract partial class " + g.normalizeName(u.Name(), false) + " : TUnionBase\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	out.WriteString(g.indent() + "public abstract global::System.Threading.Tasks.Task WriteAsync(TProtocol tProtocol, CancellationToken " + cancellationTokenName + ");\n")
	out.WriteString(g.indent() + "public readonly int Isset;\n")
	out.WriteString(g.indent() + "public abstract object" + g.nullableSuffix() + " Data { get; }\n")
	out.WriteString(g.indent() + "protected " + g.normalizeName(u.Name(), false) + "(int isset)\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	out.WriteString(g.indent() + "Isset = isset;\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	fields := u.Members()

	out.WriteString(g.indent() + "public override bool Equals(object" + g.nullableSuffix() + " that)\n")
	g.scopeUp(out)
	if g.opts.TargetNetVersion >= 6 {
		out.WriteString(g.indent() + "if (that is not " + u.Name() + " other) return false;\n")
	} else {
		out.WriteString(g.indent() + "if (!(that is " + u.Name() + " other)) return false;\n")
	}
	out.WriteString(g.indent() + "if (ReferenceEquals(this, other)) return true;\n")
	out.WriteString("\n")
	out.WriteString(g.indent() + "if(this.Isset != other.Isset) return false;\n")
	out.WriteString("\n")
	if g.opts.TargetNetVersion >= 6 {
		out.WriteString(g.indent() + "return Isset switch\n")
		g.scopeUp(out)
		for _, f := range fields {
			out.WriteString(g.indent() + itoa(int64(f.Key())) + " => Equals(As_" + f.Name() + ", other.As_" + f.Name() + "),\n")
		}
		out.WriteString(g.indent() + "_ => true,\n")
		g.indentDown()
		out.WriteString(g.indent() + "};\n")
	} else {
		out.WriteString(g.indent() + "switch (Isset)\n")
		g.scopeUp(out)
		for _, f := range fields {
			out.WriteString(g.indent() + "case " + itoa(int64(f.Key())) + ":\n")
			g.indentUp()
			out.WriteString(g.indent() + "return Equals(As_" + f.Name() + ", other.As_" + f.Name() + ");\n")
			g.indentDown()
		}
		out.WriteString(g.indent() + "default:\n")
		g.indentUp()
		out.WriteString(g.indent() + "return true;\n")
		g.indentDown()
		g.scopeDown(out)
	}
	g.scopeDown(out)
	out.WriteString("\n")

	out.WriteString(g.indent() + "public override int GetHashCode()\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	if g.opts.TargetNetVersion >= 6 {
		out.WriteString(g.indent() + "return Isset switch\n")
		out.WriteString(g.indent() + "{\n")
		g.indentUp()
		for _, f := range fields {
			nullCoalesce := ""
			if isNullableType(f.Type()) {
				nullCoalesce = "?"
			}
			out.WriteString(g.indent() + itoa(int64(f.Key())) + " => As_" + f.Name() + nullCoalesce + ".GetHashCode()")
			if nullCoalesce != "" {
				out.WriteString(" ?? 0")
			}
			out.WriteString(",\n")
		}
		out.WriteString(g.indent() + "_ =>  (new ___undefined()).GetHashCode()\n")
		g.indentDown()
		out.WriteString(g.indent() + "};\n")
	} else {
		out.WriteString(g.indent() + "switch (Isset)\n")
		out.WriteString(g.indent() + "{\n")
		g.indentUp()
		for _, f := range fields {
			nullCoalesce := ""
			if isNullableType(f.Type()) {
				nullCoalesce = "?"
			}
			out.WriteString(g.indent() + "case " + itoa(int64(f.Key())) + ":\n")
			g.indentUp()
			out.WriteString(g.indent() + "return As_" + f.Name() + nullCoalesce + ".GetHashCode()")
			if nullCoalesce != "" {
				out.WriteString(" ?? 0")
			}
			out.WriteString(";\n")
			g.indentDown()
		}
		out.WriteString(g.indent() + "default:\n")
		g.indentUp()
		out.WriteString(g.indent() + "return (new ___undefined()).GetHashCode();\n")
		g.indentDown()
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	if !g.opts.NoDeepcopy {
		out.WriteString(g.indent() + "public " + u.Name() + " " + deepCopyMethodName + "()\n")
		out.WriteString(g.indent() + "{\n")
		g.indentUp()
		if g.opts.TargetNetVersion >= 6 {
			out.WriteString(g.indent() + "return Isset switch\n")
			out.WriteString(g.indent() + "{\n")
			g.indentUp()
			for _, f := range fields {
				needsTypecast := false
				copyOp := g.deepCopyExpression("As_"+f.Name(), f.Type(), false, &needsTypecast, true)
				out.WriteString(g.indent() + itoa(int64(f.Key())) + " => new " + f.Name() + "(" + copyOp + "),\n")
			}
			out.WriteString(g.indent() + "_ => new ___undefined()\n")
			g.indentDown()
			out.WriteString(g.indent() + "};\n")
		} else {
			out.WriteString(g.indent() + "switch (Isset)\n")
			out.WriteString(g.indent() + "{\n")
			g.indentUp()
			for _, f := range fields {
				needsTypecast := false
				copyOp := g.deepCopyExpression("As_"+f.Name(), f.Type(), false, &needsTypecast, true)
				out.WriteString(g.indent() + "case " + itoa(int64(f.Key())) + ":\n")
				g.indentUp()
				out.WriteString(g.indent() + "return new " + f.Name() + "(" + copyOp + ");\n")
				g.indentDown()
			}
			out.WriteString(g.indent() + "default:\n")
			g.indentUp()
			out.WriteString(g.indent() + "return new ___undefined();\n")
			g.indentDown()
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
	}

	out.WriteString(g.indent() + "public class ___undefined : " + u.Name() + "\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	out.WriteString(g.indent() + "public override object" + g.nullableSuffix() + " Data { get { return null; } }\n")
	out.WriteString(g.indent() + "public ___undefined() : base(0) {}\n\n")

	if !g.opts.NoDeepcopy {
		out.WriteString(g.indent() + "public new ___undefined " + deepCopyMethodName + "()\n")
		out.WriteString(g.indent() + "{\n")
		g.indentUp()
		out.WriteString(g.indent() + "return new ___undefined();\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
	}

	undefinedStruct := sema.NewStruct(g.program)
	undefinedStruct.SetName("___undefined")
	g.generateNetstdStructEquals(out, undefinedStruct)
	g.generateNetstdStructHashcode(out, undefinedStruct)

	out.WriteString(g.indent() + "public override global::System.Threading.Tasks.Task WriteAsync(TProtocol oprot, CancellationToken " + cancellationTokenName + ")\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	out.WriteString(g.indent() + "throw new TProtocolException( TProtocolException.INVALID_DATA, \"Cannot persist an union type which is not set.\");\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	for _, f := range fields {
		g.generateNetstdUnionClass(out, u, f)
	}

	g.generateNetstdUnionReader(out, u)

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	g.endNetstdNamespace(out)
}

// generateNetstdUnionClass is generate_netstd_union_class.
func (g *Generator) generateNetstdUnionClass(out *strings.Builder, u *sema.Struct, f *sema.Field) {
	out.WriteString(g.indent() + "public " + g.typeName(f.Type(), true) + g.nullableFieldSuffixField(f) + " As_" + f.Name() + "\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	out.WriteString(g.indent() + "get\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	def := "default"
	if g.opts.TargetNetVersion < 6 {
		def = "default(" + g.typeName(f.Type(), true) + ")"
	}
	out.WriteString(g.indent() + "return (" + itoa(int64(f.Key())) + " == Isset) && (Data != null)" +
		" ? (" + g.typeName(f.Type(), true) + g.nullableFieldSuffixField(f) + ")Data" +
		" : " + def + ";\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	out.WriteString(g.indent() + "public class " + g.normalizeName(f.Name(), false) + " : " + g.normalizeName(u.Name(), false) + "\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	out.WriteString(g.indent() + "private readonly " + g.typeName(f.Type(), true) + " _data;\n")
	out.WriteString(g.indent() + "public override object" + g.nullableSuffix() + " Data { get { return _data; } }\n")
	out.WriteString(g.indent() + "public " + g.normalizeName(f.Name(), false) + "(" + g.typeName(f.Type(), true) + " data) : base(" + itoa(int64(f.Key())) + ")\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	out.WriteString(g.indent() + "this._data = data;\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	if !g.opts.NoDeepcopy {
		out.WriteString(g.indent() + "public new " + g.normalizeName(f.Name(), false) + " " + deepCopyMethodName + "()\n")
		out.WriteString(g.indent() + "{\n")
		g.indentUp()
		needsTypecast := false
		copyOp := g.deepCopyExpression("_data", f.Type(), true, &needsTypecast, false)
		out.WriteString(g.indent() + "return new " + g.normalizeName(f.Name(), false) + "(" + copyOp + ");\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
	}

	out.WriteString(g.indent() + "public override bool Equals(object" + g.nullableSuffix() + " that)\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	if g.opts.TargetNetVersion >= 6 {
		out.WriteString(g.indent() + "if (that is not " + u.Name() + " other) return false;\n")
	} else {
		out.WriteString(g.indent() + "if (!(that is " + u.Name() + " other)) return false;\n")
	}
	out.WriteString(g.indent() + "if (ReferenceEquals(this, other)) return true;\n")
	out.WriteString("\n")
	out.WriteString(g.indent() + "return Equals( _data, other.As_" + f.Name() + ");\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	out.WriteString(g.indent() + "public override int GetHashCode()\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	out.WriteString(g.indent() + "return _data.GetHashCode();\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	out.WriteString(g.indent() + "public override async global::System.Threading.Tasks.Task WriteAsync(TProtocol oprot, CancellationToken " + cancellationTokenName + ") {\n")
	g.indentUp()

	out.WriteString(g.indent() + "oprot.IncrementRecursionDepth();\n")
	out.WriteString(g.indent() + "try\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	out.WriteString(g.indent() + "var struc = new TStruct(\"" + u.Name() + "\");\n")
	out.WriteString(g.indent() + "await oprot.WriteStructBeginAsync(struc, " + cancellationTokenName + ");\n")

	out.WriteString(g.indent() + "var field = new TField()\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	out.WriteString(g.indent() + "Name = \"" + f.Name() + "\",\n")
	out.WriteString(g.indent() + "Type = " + g.typeToEnum(f.Type()) + ",\n")
	out.WriteString(g.indent() + "ID = " + itoa(int64(f.Key())) + "\n")
	g.indentDown()
	out.WriteString(g.indent() + "};\n")
	out.WriteString(g.indent() + "await oprot.WriteFieldBeginAsync(field, " + cancellationTokenName + ");\n")

	g.generateSerializeField(out, f, "_data", true, false)

	out.WriteString(g.indent() + "await oprot.WriteFieldEndAsync(" + cancellationTokenName + ");\n")
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
	out.WriteString(g.indent() + "}\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateNetstdUnionReader is generate_netstd_union_reader.
func (g *Generator) generateNetstdUnionReader(out *strings.Builder, u *sema.Struct) {
	// Thanks to THRIFT-1768, we don't need to check for required fields in
	// the union
	fields := u.Members()

	out.WriteString(g.indent() + "public static async Task<" + u.Name() + "> ReadAsync(TProtocol iprot, CancellationToken " + cancellationTokenName + ")\n")
	g.scopeUp(out)

	out.WriteString(g.indent() + "iprot.IncrementRecursionDepth();\n")
	out.WriteString(g.indent() + "try\n")
	g.scopeUp(out)

	tmpRetval := g.tmp("tmp")
	out.WriteString(g.indent() + u.Name() + " " + tmpRetval + ";\n")
	out.WriteString(g.indent() + "await iprot.ReadStructBeginAsync(" + cancellationTokenName + ");\n")
	out.WriteString(g.indent() + "TField field = await iprot.ReadFieldBeginAsync(" + cancellationTokenName + ");\n")
	// we cannot have the first field be a stop -- we must have a single
	// field defined
	out.WriteString(g.indent() + "if (field.Type == TType.Stop)\n")
	g.scopeUp(out)
	out.WriteString(g.indent() + "await iprot.ReadFieldEndAsync(" + cancellationTokenName + ");\n")
	out.WriteString(g.indent() + "" + tmpRetval + " = new ___undefined();\n")
	g.scopeDown(out)
	out.WriteString(g.indent() + "else\n")
	g.scopeUp(out)
	out.WriteString(g.indent() + "switch (field.ID)\n")
	g.scopeUp(out)

	for _, f := range fields {
		out.WriteString(g.indent() + "case " + itoa(int64(f.Key())) + ":\n")
		g.indentUp()
		out.WriteString(g.indent() + "if (field.Type == " + g.typeToEnum(f.Type()) + ") {\n")
		g.indentUp()

		tmpvar := g.tmp("tmp")
		out.WriteString(g.indent() + g.typeName(f.Type(), true) + " " + tmpvar + ";\n")
		g.generateDeserializeField(out, f, tmpvar, true)
		out.WriteString(g.indent() + tmpRetval + " = new " + f.Name() + "(" + tmpvar + ");\n")

		g.indentDown()
		out.WriteString(g.indent() + "} else { \n" + g.indent() + " await TProtocolUtil.SkipAsync(iprot, field.Type, " + cancellationTokenName + ");" +
			"\n" + g.indent() + "  " + tmpRetval + " = new ___undefined();\n" + g.indent() + "}\n")
		out.WriteString(g.indent() + "break;\n")
		g.indentDown()
	}

	out.WriteString(g.indent() + "default: \n")
	g.indentUp()
	out.WriteString(g.indent() + "await TProtocolUtil.SkipAsync(iprot, field.Type, " + cancellationTokenName + ");\n" + g.indent() +
		tmpRetval + " = new ___undefined();\n")
	out.WriteString(g.indent() + "break;\n")
	g.indentDown()

	g.scopeDown(out)

	out.WriteString(g.indent() + "await iprot.ReadFieldEndAsync(" + cancellationTokenName + ");\n")

	out.WriteString(g.indent() + "if ((await iprot.ReadFieldBeginAsync(" + cancellationTokenName + ")).Type != TType.Stop)\n")
	g.scopeUp(out)
	out.WriteString(g.indent() + "throw new TProtocolException(TProtocolException.INVALID_DATA);\n")
	g.scopeDown(out)

	// end of else for TStop
	g.scopeDown(out)
	out.WriteString(g.indent() + "await iprot.ReadStructEndAsync(" + cancellationTokenName + ");\n")
	out.WriteString(g.indent() + "return " + tmpRetval + ";\n")
	g.indentDown()

	g.scopeDown(out)
	out.WriteString(g.indent() + "finally\n")
	g.scopeUp(out)
	out.WriteString(g.indent() + "iprot.DecrementRecursionDepth();\n")
	g.scopeDown(out)

	out.WriteString(g.indent() + "}\n\n")
}
