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

package js

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// jsEscapeString is get_escaped_string / t_generator::escape_string with
// the js generator's escape table, which adds a single-quote entry to the
// default one (the js generator constructor does
// escape_[quote-char] = "\\'", since string constants are single-quoted).
func jsEscapeString(in string) string {
	var sb strings.Builder
	for i := 0; i < len(in); i++ {
		switch c := in[i]; c {
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\'':
			sb.WriteString(`\'`)
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

// generateTypedef is generate_typedef: typedefs are not emitted, types are
// all implicit in JS.
func (g *Generator) generateTypedef(_ *sema.Typedef) {}

// generateEnum is generate_enum. Since define is expensive to look up in
// JS, it emits a global map instead.
func (g *Generator) generateEnum(e *sema.Enum) {
	if g.opts.ESM {
		g.fTypes.WriteString("export const " + e.Name() + " = {" + "\n")
	} else {
		g.fTypes.WriteString(g.jsTypeNamespace(e.Program()) + e.Name() + " = {" + "\n")
	}
	if g.opts.TS {
		g.fTypesTS.WriteString(g.tsPrintDoc(e) + g.tsIndent() + g.tsDeclare() + "enum " + e.Name() + " {" + "\n")
	}

	g.indentUp()
	constants := e.Constants()
	for i, c := range constants {
		value := c.Value()
		if g.opts.TS {
			g.fTypesTS.WriteString(g.tsIndent() + c.Name() + " = " + strconv.FormatInt(int64(value), 10) + "," + "\n")
			g.fTypes.WriteString(g.indent() + "'" + strconv.FormatInt(int64(value), 10) + "' : '" + c.Name() + "'," + "\n")
		}
		g.fTypes.WriteString(g.indent() + "'" + c.Name() + "' : " + strconv.FormatInt(int64(value), 10))
		if i != len(constants)-1 {
			g.fTypes.WriteString(",")
		}
		g.fTypes.WriteString("\n")
	}
	g.indentDown()

	g.fTypes.WriteString("};" + "\n")
	if g.opts.TS {
		g.fTypesTS.WriteString(g.tsIndent() + "}" + "\n")
	}
}

// generateConst is generate_const.
func (g *Generator) generateConst(c *sema.Const) {
	if g.opts.ESM {
		g.fTypes.WriteString("export const " + c.Name() + " = ")
	} else {
		g.fTypes.WriteString(g.jsTypeNamespace(g.program) + c.Name() + " = ")
	}
	g.fTypes.WriteString(g.renderConstValue(c.Type(), c.Value()) + ";" + "\n")

	if g.opts.TS {
		g.fTypesTS.WriteString(g.tsPrintDoc(c) + g.tsIndent() + g.tsDeclare() + g.jsConstType + c.Name() + ": " + g.tsGetType(c.Type()) + ";" + "\n")
	}
}

// renderConstValue is render_const_value. Note that type checking is NOT
// performed here, as it is always run beforehand by sema.ValidateInput.
func (g *Generator) renderConstValue(t sema.Type, v *sema.ConstValue) string {
	var out strings.Builder
	t = sema.TrueType(t)

	switch {
	case t.IsBaseType():
		base := t.(*sema.BaseType).Base()
		switch base {
		case sema.TypeString:
			out.WriteString("'" + jsEscapeString(v.String()) + "'")
		case sema.TypeUUID:
			// The C++ source writes `out << "'" << value << "'"` where
			// value is the t_const_value*, not get_escaped_string(value):
			// it prints the pointer's address, which is not reproducible
			// between runs. See undefinedInCpp["js"] in the parity test;
			// this port renders the UUID string instead.
			out.WriteString("'" + v.String() + "'")
		case sema.TypeBool:
			if v.Integer() > 0 {
				out.WriteString("true")
			} else {
				out.WriteString("false")
			}
		case sema.TypeI8, sema.TypeI16, sema.TypeI32:
			out.WriteString(strconv.FormatInt(v.Integer(), 10))
		case sema.TypeI64:
			iv := v.Integer()
			switch {
			case g.opts.Bigint:
				// Native BigInt literal - handles the full signed 64-bit range.
				out.WriteString(strconv.FormatInt(iv, 10) + "n")
			case iv <= maxSafeInteger && iv >= minSafeInteger:
				out.WriteString("new Int64(" + strconv.FormatInt(iv, 10) + ")")
			default:
				// std::hex on a negative int64_t prints the digits of its
				// two's complement representation, not a minus sign.
				out.WriteString("new Int64('" + strconv.FormatUint(uint64(iv), 16) + "')")
			}
		case sema.TypeDouble:
			if v.Kind() == sema.CVInteger {
				out.WriteString(strconv.FormatInt(v.Integer(), 10))
			} else {
				out.WriteString(emit.DoubleFixed16(v.Double()))
			}
		default:
			emit.Throw("compiler error: no const of base type %s", sema.BaseName(base))
		}
	case t.IsEnum():
		out.WriteString(strconv.FormatInt(v.Integer(), 10))
	case t.IsStruct(), t.IsXception():
		out.WriteString("new " + g.jsTypeNamespace(t.Program()) + t.Name() + "({")
		g.indentUp()
		fields := t.(*sema.Struct).Members()
		for i, entry := range v.Map() {
			fieldName := entry.Key.String()
			var fieldType sema.Type
			for _, f := range fields {
				if f.Name() == fieldName {
					fieldType = f.Type()
				}
			}
			if fieldType == nil {
				emit.Throw("type error: %s has no field %s", t.Name(), fieldName)
			}
			if i != 0 {
				out.WriteString(",")
			}
			out.WriteString("\n" + g.indent() + g.renderConstValue(sema.GlobalString, entry.Key))
			out.WriteString(" : ")
			out.WriteString(g.renderConstValue(fieldType, entry.Value))
		}
		g.indentDown()
		out.WriteString("\n" + g.indent() + "})")
	case t.IsMap():
		m := t.(*sema.Map)
		out.WriteString("{" + "\n")
		g.indentUp()
		for i, entry := range v.Map() {
			if i != 0 {
				out.WriteString("," + "\n")
			}
			if m.KeyType().IsBaseType() && sema.TrueType(m.KeyType()).(*sema.BaseType).Base() == sema.TypeI64 {
				out.WriteString(g.indent() + "\"" + strconv.FormatInt(entry.Key.Integer(), 10) + "\"")
			} else {
				out.WriteString(g.indent() + g.renderConstValue(m.KeyType(), entry.Key))
			}
			out.WriteString(" : ")
			out.WriteString(g.renderConstValue(m.ValType(), entry.Value))
		}
		g.indentDown()
		out.WriteString("\n" + g.indent() + "}")
	case t.IsList(), t.IsSet():
		var etype sema.Type
		if t.IsList() {
			etype = t.(*sema.List).ElemType()
		} else {
			etype = t.(*sema.Set).ElemType()
		}
		out.WriteString("[")
		for i, e := range v.List() {
			if i != 0 {
				out.WriteString(",")
			}
			out.WriteString(g.renderConstValue(etype, e))
		}
		out.WriteString("]")
	}
	return out.String()
}

// tsGetType is ts_get_type.
func (g *Generator) tsGetType(t sema.Type) string {
	t = sema.TrueType(t)
	switch {
	case t.IsBaseType():
		bt := t.(*sema.BaseType)
		switch bt.Base() {
		case sema.TypeString:
			// Only the node runtime exchanges binary as a Buffer. The
			// browser library has no Buffer: TProtocol.writeBinary() takes
			// a string and readBinary() returns one, as lib/ts/thrift.d.ts
			// declares.
			if bt.IsBinary() && g.opts.Node {
				return "Buffer"
			}
			return "string"
		case sema.TypeUUID:
			return "uuid"
		case sema.TypeBool:
			return "boolean"
		case sema.TypeI8:
			return "any"
		case sema.TypeI16, sema.TypeI32, sema.TypeDouble:
			return "number"
		case sema.TypeI64:
			if g.opts.Bigint {
				return "bigint"
			}
			return "Int64"
		case sema.TypeVoid:
			return "void"
		default:
			emit.Throw("compiler error: unhandled js type")
		}
	case t.IsEnum(), t.IsStruct(), t.IsXception():
		typeName := ""
		if p := t.Program(); p != nil {
			typeName = g.jsNamespace(p)
			// If the type is not defined within the current program, we
			// need to prefix it with the same name as the generated
			// "import" statement for the types containing program.
			if p != g.program {
				if prefix, ok := g.include2ImportName[p]; ok {
					typeName += prefix + "."
				}
			}
		}
		typeName += t.Name()
		return typeName
	case t.IsList(), t.IsSet():
		var etype sema.Type
		if t.IsList() {
			etype = t.(*sema.List).ElemType()
		} else {
			etype = t.(*sema.Set).ElemType()
		}
		return g.tsGetType(etype) + "[]"
	case t.IsMap():
		m := t.(*sema.Map)
		ktype := g.tsGetType(m.KeyType())
		vtype := g.tsGetType(m.ValType())
		switch {
		case ktype == "number" || ktype == "string":
			return "{ [k: " + ktype + "]: " + vtype + "; }"
		case ktype == "bigint":
			// TS index signatures only support string / number / symbol -
			// JS coerces object keys to strings at runtime, so a
			// `map<i64, ...>` is really a string-keyed object even in
			// bigint mode.
			return "{ [k: string /*bigint*/]: " + vtype + "; }"
		case m.KeyType().IsEnum():
			// Not yet supported (enum map):
			// https://github.com/Microsoft/TypeScript/pull/2652
			return "{ [k: number /*" + ktype + "*/]: " + vtype + "; }"
		default:
			return "any"
		}
	}
	return ""
}
