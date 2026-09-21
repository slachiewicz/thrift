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

package delphi

import (
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

// generateProperty is t_delphi_generator::generate_property.
func (g *Generator) generateProperty(out *strings.Builder, tfield *sema.Field, isPublic, isXception bool) {
	g.generateDelphiProperty(out, isXception, tfield, isPublic)
}

// generateDelphiProperty is t_delphi_generator::generate_delphi_property.
func (g *Generator) generateDelphiProperty(out *strings.Builder, structIsXception bool, tfield *sema.Field, isPublic bool) {
	_ = isPublic
	ftype := tfield.Type()
	g.generateDelphiDocField(out, tfield)
	g.wr(out, "property "+g.propNameF(tfield, structIsXception, "")+": "+
		g.typeName(ftype, false, true)+
		" read "+g.propNameF(tfield, structIsXception, "Get")+
		" write "+g.propNameF(tfield, structIsXception, "Set")+
		";"+renderDeprecationAttribute(tfield.Annotations(), " {", "}")+"\n")
}

// generateDelphiPropertyWriterDefinition is
// t_delphi_generator::generate_delphi_property_writer_definition.
func (g *Generator) generateDelphiPropertyWriterDefinition(out *strings.Builder, tfield *sema.Field, isXceptionClass bool) {
	ftype := tfield.Type()
	g.wr(out, "procedure "+g.propNameF(tfield, isXceptionClass, "Set")+
		"( const Value: "+g.typeName(ftype, false, true)+");"+
		renderDeprecationAttribute(tfield.Annotations(), " ", ";")+"\n")
}

// generateDelphiPropertyReaderDefinition is
// t_delphi_generator::generate_delphi_property_reader_definition.
func (g *Generator) generateDelphiPropertyReaderDefinition(out *strings.Builder, tfield *sema.Field, isXceptionClass bool) {
	ftype := tfield.Type()
	g.wr(out, "function "+g.propNameF(tfield, isXceptionClass, "Get")+": "+
		g.typeName(ftype, false, true)+";"+
		renderDeprecationAttribute(tfield.Annotations(), " ", ";")+"\n")
}

// generateDelphiIssetReaderWriterDefinition is
// t_delphi_generator::generate_delphi_isset_reader_writer_definition.
func (g *Generator) generateDelphiIssetReaderWriterDefinition(out *strings.Builder, tfield *sema.Field, isXception bool) {
	_ = isXception
	g.ln(out, "function "+g.propNameF(tfield, false, "Get__isset_")+": System.Boolean;")
	g.ln(out, "procedure "+g.propNameF(tfield, false, "Set__isset_")+"( const value : System.Boolean);")
}

// generateDelphiClearUnionValue is
// t_delphi_generator::generate_delphi_clear_union_value.
func (g *Generator) generateDelphiClearUnionValue(out *strings.Builder, tfield *sema.Field, fieldPrefix string, isXceptionClass bool) {
	ftype := tfield.Type()
	g.lnI(out, "if "+g.propNameF(tfield, isXceptionClass, "F__isset_")+" then begin")
	g.indentUpImpl()
	g.lnI(out, g.propNameF(tfield, isXceptionClass, "F__isset_")+" := False;")
	g.lnI(out, g.propNameF(tfield, isXceptionClass, fieldPrefix)+" := "+"Default( "+g.typeName(ftype, false, true)+");")
	g.indentDownImpl()
	g.lnI(out, "end;")
}

// generateDelphiPropertyWriterImpl is
// t_delphi_generator::generate_delphi_property_writer_impl.
func (g *Generator) generateDelphiPropertyWriterImpl(out *strings.Builder, clsPrefix, name string, tfield *sema.Field, fieldPrefix string, isXceptionClass, isUnion bool) {
	ftype := tfield.Type()
	g.lnI(out, "procedure "+clsPrefix+name+"."+g.propNameF(tfield, isXceptionClass, "Set")+
		"( const Value: "+g.typeName(ftype, false, true)+");")
	g.lnI(out, "begin")
	g.indentUpImpl()
	if isUnion {
		g.lnI(out, "ClearUnionValues;")
	}
	if isXceptionClass {
		g.lnI(out, "FData."+g.propNameF(tfield, false, fieldPrefix)+" := Value;")
	} else {
		if tfield.Req() != sema.Required {
			g.lnI(out, g.propNameF(tfield, isXceptionClass, "F__isset_")+" := True;")
		}
		g.lnI(out, g.propNameF(tfield, isXceptionClass, fieldPrefix)+" := Value;")
	}
	g.indentDownImpl()
	g.lnI(out, "end;")
	g.raw(out, "\n")
}

// generateDelphiPropertyReaderImpl is
// t_delphi_generator::generate_delphi_property_reader_impl.
func (g *Generator) generateDelphiPropertyReaderImpl(out *strings.Builder, clsPrefix, name string, tfield *sema.Field, fieldPrefix string, isXceptionClass bool) {
	ftype := tfield.Type()
	g.lnI(out, "function "+clsPrefix+name+"."+g.propNameF(tfield, isXceptionClass, "Get")+": "+g.typeName(ftype, false, true)+";")
	g.lnI(out, "begin")
	g.indentUpImpl()
	if isXceptionClass {
		g.lnI(out, "Result := FData."+g.propNameF(tfield, false, fieldPrefix)+";")
	} else {
		g.lnI(out, "Result := "+g.propNameF(tfield, isXceptionClass, fieldPrefix)+";")
	}
	g.indentDownImpl()
	g.lnI(out, "end;")
	g.raw(out, "\n")
}

// generateDelphiIssetReaderWriterImpl is
// t_delphi_generator::generate_delphi_isset_reader_writer_impl.
func (g *Generator) generateDelphiIssetReaderWriterImpl(out *strings.Builder, clsPrefix, name string, tfield *sema.Field, fieldPrefix string, isXceptionClass bool) {
	issetName := g.propNameF(tfield, false, "__isset_")

	g.lnI(out, "function "+clsPrefix+name+"."+"Get"+issetName+": System.Boolean;")
	g.lnI(out, "begin")
	g.indentUpImpl()
	if isXceptionClass {
		g.lnI(out, "Result := FData."+fieldPrefix+issetName+";")
	} else {
		g.lnI(out, "Result := "+fieldPrefix+issetName+";")
	}
	g.indentDownImpl()
	g.lnI(out, "end;")
	g.raw(out, "\n")

	g.lnI(out, "procedure "+clsPrefix+name+"."+"Set"+issetName+"( const value: System.Boolean);")
	g.lnI(out, "begin")
	g.indentUpImpl()
	if isXceptionClass {
		g.lnI(out, "FData."+fieldPrefix+issetName+" := value;")
	} else {
		g.lnI(out, fieldPrefix+issetName+" := value;")
	}
	g.indentDownImpl()
	g.lnI(out, "end;")
	g.raw(out, "\n")
}

// argumentList is t_delphi_generator::argument_list.
func (g *Generator) argumentList(tstruct *sema.Struct) string {
	result := ""
	first := true
	for _, f := range tstruct.Members() {
		if first {
			first = false
		} else {
			result += "; "
		}
		tt := f.Type()
		result += g.inputArgPrefix(tt)
		result += g.normalizeNameSimple(f.Name()) + ": " + g.typeName(tt, false, true)
	}
	return result
}

// constructorArgumentList is t_delphi_generator::constructor_argument_list.
func (g *Generator) constructorArgumentList(tstruct *sema.Struct, currentIndent string) string {
	var result strings.Builder
	first := true
	line := ""
	newlineIndent := currentIndent + "  "
	firstline := true

	for _, f := range tstruct.Members() {
		if first {
			first = false
		} else {
			line += ";"
		}

		if len(line) > 80 {
			if firstline {
				result.WriteString("\n" + newlineIndent)
				firstline = false
			}
			result.WriteString(line + "\n")
			line = newlineIndent
		} else if len(line) > 0 {
			line += " "
		}

		tt := f.Type()
		line += g.inputArgPrefix(tt)
		line += g.constructorParamName(f.Name()) + ": " + g.typeName(tt, false, true)
	}

	if len(line) > 0 {
		result.WriteString(line)
	}

	if firstline {
		return " " + result.String()
	}
	return result.String()
}

// generateDelphiStructDefinition is
// t_delphi_generator::generate_delphi_struct_definition.
func (g *Generator) generateDelphiStructDefinition(out *strings.Builder, tstruct *sema.Struct) {
	isFinal := tstruct.Annotations().Has("final")
	structIntfName := g.typeName(tstruct, false, false)
	structName := g.typeName(tstruct, true, false)
	members := tstruct.Members()

	g.generateDelphiDoc(out, tstruct)
	if g.opts.RTTI {
		g.ln(out, "{$TYPEINFO ON}")
		g.ln(out, "{$RTTI EXPLICIT METHODS([vcPublic, vcPublished]) PROPERTIES([vcPublic, vcPublished])}")
		g.ln(out, structIntfName+" = interface(IBaseWithTypeInfo)")
	} else {
		g.ln(out, structIntfName+" = interface(IBase)")
	}
	g.indentUp()

	if g.opts.GuidV4 {
		g.generateGuid(out)
	} else {
		g.generateGuidV8Struct(out, tstruct)
	}

	for _, f := range members {
		g.generateDelphiPropertyReaderDefinition(out, f, false)
		g.generateDelphiPropertyWriterDefinition(out, f, false)
	}

	if len(members) > 0 {
		g.raw(out, "\n")
		for _, f := range members {
			g.generateProperty(out, f, true, false)
		}

		g.raw(out, "\n")
		for _, f := range members {
			if f.Req() != sema.Required {
				g.generateDelphiIssetReaderWriterDefinition(out, f, false)
			}
		}

		g.raw(out, "\n")
		for _, f := range members {
			if f.Req() != sema.Required {
				issetName := g.propNameF(f, false, "__isset_")
				g.ln(out, "property "+issetName+": System.Boolean read Get"+issetName+" write Set"+issetName+";")
			}
		}
	}

	g.raw(out, "\n")
	g.ln(out, "function Equal(const other: IInterface): System.Boolean;")

	g.indentDown()
	g.wr(out, "end;"+renderDeprecationAttribute(tstruct.Annotations(), " {", "}")+"\n")
	if g.opts.RTTI {
		g.ln(out, "{$IFNDEF TYPEINFO_WAS_ON} {$TYPEINFO OFF} {$ENDIF}")
	}
	g.ln(out, "")

	g.generateDelphiDoc(out, tstruct)
	g.wr(out, structName+" = ")
	if isFinal {
		g.raw(out, "sealed ")
	}
	g.raw(out, "class(TInterfacedObject, IBase, ISupportsToString, "+structIntfName+")\n")

	g.ln(out, "private")
	g.indentUp()

	for _, f := range members {
		g.ln(out, g.declareField(f, "F", false))
	}

	if len(members) > 0 {
		g.ln(out, "")
		for _, f := range members {
			if f.Req() != sema.Required {
				issetName := g.propNameF(f, false, "F__isset_")
				g.ln(out, issetName+": System.Boolean;")
			}
		}
	}

	g.ln(out, "")

	for _, f := range members {
		g.generateDelphiPropertyReaderDefinition(out, f, false)
		g.generateDelphiPropertyWriterDefinition(out, f, false)
	}

	if tstruct.IsUnion() {
		g.raw(out, "\n")
		g.ln(out, "// Clear values(for union's property setter)")
		g.ln(out, "procedure ClearUnionValues;")
	}

	if len(members) > 0 {
		g.raw(out, "\n")
		for _, f := range members {
			if f.Req() != sema.Required {
				issetName := g.propNameF(f, false, "__isset_")
				g.ln(out, "function Get"+issetName+": System.Boolean;")
				g.ln(out, "procedure Set"+issetName+"( const value : System.Boolean);")
			}
		}
	}

	g.indentDown()
	g.ln(out, "public")
	g.indentUp()
	g.ln(out, "constructor Create;"+renderDeprecationAttribute(tstruct.Annotations(), " ", ";"))
	g.ln(out, "destructor Destroy; override;")

	g.raw(out, "\n")
	g.ln(out, "function ToString: string; override;")
	g.ln(out, "function Equal(const other: IInterface): System.Boolean;")
	g.ln(out, "function Equals(Obj: TObject): System.Boolean; override;")

	g.raw(out, "\n")
	g.ln(out, "// IBase")
	g.ln(out, "procedure Read( const iprot: IProtocol);")
	g.ln(out, "procedure Write( const oprot: IProtocol);")

	if len(members) > 0 {
		g.raw(out, "\n")
		g.ln(out, "// Properties")
		for _, f := range members {
			g.generateProperty(out, f, true, false)
		}

		g.raw(out, "\n")
		g.ln(out, "// isset")
		for _, f := range members {
			if f.Req() != sema.Required {
				issetName := g.propNameF(f, false, "__isset_")
				g.ln(out, "property "+issetName+": System.Boolean read Get"+issetName+" write Set"+issetName+";")
			}
		}
	}

	g.indentDown()
	g.ln(out, "end;")
	g.raw(out, "\n")
}

// generateDelphiExceptionDefinition is
// t_delphi_generator::generate_delphi_exception_definition.
func (g *Generator) generateDelphiExceptionDefinition(out *strings.Builder, tstruct *sema.Struct) {
	isFinal := tstruct.Annotations().Has("final")
	structIntfName := g.typeName(tstruct, false, false)
	structName := g.typeName(tstruct, true, true)
	members := tstruct.Members()

	g.generateDelphiDoc(out, tstruct)
	g.wr(out, structName+" = ")
	if isFinal {
		g.raw(out, "sealed ")
	}
	g.raw(out, "class(TException, IInterface, IBase, ISupportsToString)\n")

	g.ln(out, "private")
	g.indentUp()
	g.ln(out, "FData : "+structIntfName+";")
	g.ln(out, "")

	for _, f := range members {
		g.generateDelphiPropertyReaderDefinition(out, f, true)
		g.generateDelphiPropertyWriterDefinition(out, f, true)
	}

	if len(members) > 0 {
		g.raw(out, "\n")
		for _, f := range members {
			if f.Req() != sema.Required {
				issetName := g.propNameF(f, true, "__isset_")
				g.ln(out, "function Get"+issetName+": System.Boolean;")
				g.ln(out, "procedure Set"+issetName+"( const value : System.Boolean);")
			}
		}
	}

	g.raw(out, "\n")
	g.indentDown()
	g.ln(out, "strict protected")
	g.indentUp()
	g.ln(out, "// non-refcounted instance")
	g.ln(out, "function QueryInterface(const IID: TGUID; out Obj): HRESULT; stdcall;")
	g.ln(out, "function _AddRef: Integer; stdcall;")
	g.ln(out, "function _Release: Integer; stdcall;")
	g.raw(out, "\n")

	g.indentDown()
	g.ln(out, "public")
	g.indentUp()

	g.ln(out, "constructor Create; overload;"+renderDeprecationAttribute(tstruct.Annotations(), " ", ";"))
	if len(members) > 0 {
		g.ln(out, "constructor Create("+g.constructorArgumentList(tstruct, g.indent())+"); overload;"+renderDeprecationAttribute(tstruct.Annotations(), " ", ";"))
	}
	g.ln(out, "constructor Create( const aData: "+structIntfName+"); overload;"+renderDeprecationAttribute(tstruct.Annotations(), " ", ";"))

	g.ln(out, "destructor Destroy; override;")

	g.raw(out, "\n")
	g.ln(out, "property ExceptionData : "+structIntfName+" read FData;")
	g.ln(out, "function ToString: string; override;")
	g.ln(out, "function Equals(Obj: TObject): System.Boolean; override;")

	g.raw(out, "\n")
	g.ln(out, "// IBase")
	g.ln(out, "procedure Read( const iprot: IProtocol);")
	g.ln(out, "procedure Write( const oprot: IProtocol);")

	if len(members) > 0 {
		g.raw(out, "\n")
		g.ln(out, "// Properties")
		for _, f := range members {
			g.generateProperty(out, f, true, true)
		}

		g.raw(out, "\n")
		g.ln(out, "// isset")
		for _, f := range members {
			if f.Req() != sema.Required {
				issetName := g.propNameF(f, true, "__isset_")
				g.ln(out, "property "+issetName+": System.Boolean read Get"+issetName+" write Set"+issetName+";")
			}
		}
	}

	g.indentDown()
	g.ln(out, "end;")
	g.raw(out, "\n")
}
