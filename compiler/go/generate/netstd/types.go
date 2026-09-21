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

// typeName is type_name(ttype, with_namespace = true).
func (g *Generator) typeName(t sema.Type, withNamespace bool) string {
	t = trueType(t)

	if t.IsBaseType() {
		return g.baseTypeName(t.(*sema.BaseType))
	}

	if t.IsMap() {
		m := t.(*sema.Map)
		return "Dictionary<" + g.typeName(m.KeyType(), true) + ", " + g.typeName(m.ValType(), true) + ">"
	}

	if t.IsSet() {
		s := t.(*sema.Set)
		return "HashSet<" + g.typeName(s.ElemType(), true) + ">"
	}

	if t.IsList() {
		l := t.(*sema.List)
		return "List<" + g.typeName(l.ElemType(), true) + ">"
	}

	theName := g.normalizeName(t.Name(), false)

	if withNamespace {
		if p := t.Program(); p != nil {
			if ns := p.Namespace("netstd"); ns != "" {
				return "global::" + ns + "." + theName
			}
		}
	}

	return theName
}

// tn is type_name(ttype) with the default with_namespace = true.
func (g *Generator) tn(t sema.Type) string { return g.typeName(t, true) }

// baseTypeName is base_type_name.
func (g *Generator) baseTypeName(t *sema.BaseType) string {
	switch t.Base() {
	case sema.TypeVoid:
		return "void"
	case sema.TypeString:
		if t.IsBinary() {
			return "byte[]"
		}
		return "string"
	case sema.TypeUUID:
		return "global::System.Guid"
	case sema.TypeBool:
		return "bool"
	case sema.TypeI8:
		return "sbyte"
	case sema.TypeI16:
		return "short"
	case sema.TypeI32:
		return "int"
	case sema.TypeI64:
		return "long"
	case sema.TypeDouble:
		return "double"
	}
	emit.Throw("compiler error: no C# name for base type %s", sema.BaseName(t.Base()))
	return ""
}

// declareField is declare_field.
func (g *Generator) declareField(f *sema.Field, init, allowNullable bool, prefix string) string {
	result := g.typeName(f.Type(), true)
	if allowNullable {
		result += g.nullableFieldSuffixField(f)
	}
	result += " " + prefix + f.Name()
	if init {
		result += g.initializeField(f)
	}
	return result + ";"
}

// initializeField is initialize_field.
func (g *Generator) initializeField(f *sema.Field) string {
	t := trueType(f.Type())

	if t.IsBaseType() && fieldHasDefault(f) {
		var dummy strings.Builder
		return " = " + g.renderConstValue(&dummy, f.Name(), t, f.Value())
	} else if g.forceMemberNullable(f) {
		return "" // see force_member_nullable() why this is necessary
	} else if t.IsBaseType() {
		switch baseOf(t) {
		case sema.TypeVoid:
			emit.Throw("NO T_VOID CONSTRUCT")
		case sema.TypeString:
			if g.opts.TargetNetVersion >= 6 && fieldIsRequired(f) {
				if t.IsBinary() {
					if g.opts.TargetNetVersion >= 8 {
						return "= []"
					}
					return " = Array.Empty<byte>()"
				}
				return " = string.Empty"
			}
			return " = null"
		case sema.TypeUUID:
			return " = System.Guid.Empty"
		case sema.TypeBool:
			return " = false"
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			return " = 0"
		case sema.TypeDouble:
			return " = 0.0"
		}
	} else if t.IsEnum() {
		return " = default"
	} else if t.IsContainer() {
		if g.opts.TargetNetVersion >= 6 {
			return " = new()"
		}
		return " = new " + g.tn(t) + "()"
	} else if t.IsStruct() {
		s := t.(*sema.Struct)
		if g.opts.TargetNetVersion >= 6 {
			if s.IsUnion() {
				return " = new " + g.tn(t) + ".___undefined()"
			}
			return " = new()"
		}
		return " = new " + g.tn(t) + "()"
	}

	emit.Throw("UNEXPECTED TYPE IN initialize_field: %s", t.Name())
	return ""
}

// functionSignature is function_signature.
func (g *Generator) functionSignature(f *sema.Function, prefix string) string {
	t := f.ReturnType()
	return g.tn(t) + " " + g.funcName(g.normalizeName(prefix+f.Name(), false), false) + "(" + g.argumentList(f.Arglist(), true) + ")"
}

// Function signature modes: MODE_FULL_DECL, MODE_NO_RETURN, MODE_NO_ARGS.
const (
	modeFullDecl = 0x00
	modeNoReturn = 0x01
	modeNoArgs   = 0x02
)

// functionSignatureAsync is function_signature_async.
func (g *Generator) functionSignatureAsync(f *sema.Function, prefix string, mode int) string {
	t := f.ReturnType()
	task := "global::System.Threading.Tasks.Task"
	if !t.IsVoid() && (mode&modeNoReturn) == 0 {
		task += "<" + g.tn(t) + ">"
	}

	name := g.normalizeName(prefix+f.Name(), false)
	if g.opts.AsyncPostfix {
		name += "Async"
	}
	result := task + " " + g.funcName(name, false) + "("
	if mode&modeNoArgs == 0 {
		args := g.argumentList(f.Arglist(), true)
		result += args
		if args != "" {
			result += ", "
		}
	}
	result += "CancellationToken " + cancellationTokenName + " = default)"

	return result
}

// argumentList is argument_list.
func (g *Generator) argumentList(s *sema.Struct, withTypes bool) string {
	result := ""
	first := true
	for _, f := range s.Members() {
		if first {
			first = false
		} else {
			result += ", "
		}
		if withTypes {
			result += g.typeName(f.Type(), true) + g.nullableFieldSuffixField(f) + " "
		}
		result += g.normalizeName(f.Name(), true)
	}
	return result
}

// typeToEnum is type_to_enum.
func (g *Generator) typeToEnum(t sema.Type) string {
	t = trueType(t)

	if t.IsBaseType() {
		switch baseOf(t) {
		case sema.TypeVoid:
			emit.Throw("NO T_VOID CONSTRUCT")
		case sema.TypeString:
			return "TType.String"
		case sema.TypeUUID:
			return "TType.Uuid"
		case sema.TypeBool:
			return "TType.Bool"
		case sema.TypeI8:
			return "TType.Byte"
		case sema.TypeI16:
			return "TType.I16"
		case sema.TypeI32:
			return "TType.I32"
		case sema.TypeI64:
			return "TType.I64"
		case sema.TypeDouble:
			return "TType.Double"
		}
	} else if t.IsEnum() {
		return "TType.I32"
	} else if t.IsStruct() || t.IsXception() {
		return "TType.Struct"
	} else if t.IsMap() {
		return "TType.Map"
	} else if t.IsSet() {
		return "TType.Set"
	} else if t.IsList() {
		return "TType.List"
	}

	emit.Throw("INVALID TYPE IN type_to_enum: %s", t.Name())
	return ""
}
