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

// generateDelphiStructImpl is t_delphi_generator::generate_delphi_struct_impl.
func (g *Generator) generateDelphiStructImpl(out *strings.Builder, clsPrefix string, tstruct *sema.Struct, isResult, isConstClass bool) {
	_ = isResult
	clsNm := g.typeName(tstruct, true, false)

	var vars, code strings.Builder
	members := tstruct.Members()

	g.indentUpImpl()
	for _, f := range members {
		truetype := sema.TrueType(f.Type())
		if f.Value() != nil {
			g.initializeField(&vars, &code, g.propName(f.Name(), false, "F"), truetype, f.Value(), isConstClass)
			if f.Req() != sema.Required {
				g.lnI(&code, g.propNameF(f, false, "F__isset_")+" := True;")
			}
		}
	}
	g.indentDownImpl()

	g.lnI(out, "constructor "+clsPrefix+clsNm+".Create;")

	if vars.Len() > 0 {
		g.raw(out, "var\n")
		g.raw(out, vars.String())
	}

	g.lnI(out, "begin")
	g.indentUpImpl()
	g.lnI(out, "inherited;")

	if code.Len() > 0 {
		g.raw(out, code.String())
	}

	g.indentDownImpl()
	g.lnI(out, "end;")
	g.raw(out, "\n")

	g.lnI(out, "destructor "+clsPrefix+clsNm+".Destroy;")
	g.lnI(out, "begin")
	g.indentUpImpl()

	for range members {
		g.finalizeField()
	}

	g.lnI(out, "inherited;")
	g.indentDownImpl()
	g.lnI(out, "end;")
	g.raw(out, "\n")

	if tstruct.IsUnion() {
		g.lnI(out, "procedure "+clsPrefix+clsNm+".ClearUnionValues;")
		g.lnI(out, "begin")
		g.indentUpImpl()
		for _, f := range members {
			g.generateDelphiClearUnionValue(out, f, "F", false)
		}
		g.indentDownImpl()
		g.lnI(out, "end;")
		g.raw(out, "\n")
	}

	for _, f := range members {
		g.generateDelphiPropertyReaderImpl(out, clsPrefix, clsNm, f, "F", false)
		g.generateDelphiPropertyWriterImpl(out, clsPrefix, clsNm, f, "F", false, tstruct.IsUnion())
		if f.Req() != sema.Required {
			g.generateDelphiIssetReaderWriterImpl(out, clsPrefix, clsNm, f, "F", false)
		}
	}

	g.generateDelphiStructReaderImpl(out, clsPrefix, tstruct, false)
	g.generateDelphiStructWriterImpl(out, clsPrefix, tstruct, false)
	g.generateDelphiStructTostringImpl(out, clsPrefix, tstruct, false)
	g.generateDelphiStructEqualityImpl(out, clsPrefix, tstruct)
}

// generateDelphiExceptionImpl is t_delphi_generator::generate_delphi_exception_impl.
func (g *Generator) generateDelphiExceptionImpl(out *strings.Builder, clsPrefix string, tstruct *sema.Struct) {
	clsNm := g.typeName(tstruct, true, true)
	members := tstruct.Members()

	g.lnI(out, "constructor "+clsPrefix+clsNm+".Create;")
	g.lnI(out, "begin")
	g.indentUpImpl()
	g.lnI(out, "inherited Create('');")
	g.lnI(out, "FData := "+g.typeName(tstruct, true, false)+".Create;")
	g.indentDownImpl()
	g.lnI(out, "end;")
	g.raw(out, "\n")

	if len(members) > 0 {
		g.lnI(out, "constructor "+clsPrefix+clsNm+"."+"Create("+g.constructorArgumentList(tstruct, g.indentImpl())+");")
		g.lnI(out, "begin")
		g.indentUpImpl()
		g.lnI(out, "Create;")
		for _, f := range members {
			propname := g.propName(f.Name(), true, "")
			paramName := g.constructorParamName(f.Name())
			g.lnI(out, propname+" := "+paramName+";")
		}
		g.lnI(out, "UpdateMessageProperty;")
		g.indentDownImpl()
		g.lnI(out, "end;")
		g.raw(out, "\n")
	}

	g.lnI(out, "constructor "+clsPrefix+clsNm+"."+"Create(const aData: "+g.typeName(tstruct, false, false)+");")
	g.lnI(out, "begin")
	g.indentUpImpl()
	g.lnI(out, "inherited Create('');")
	g.lnI(out, "if aData <> nil")
	g.lnI(out, "then FData := aData")
	g.lnI(out, "else ASSERT(FALSE,'Invalid argument');")
	g.lnI(out, "UpdateMessageProperty;")
	g.indentDownImpl()
	g.lnI(out, "end;")
	g.raw(out, "\n")

	g.lnI(out, "destructor "+clsPrefix+clsNm+"."+"Destroy;")
	g.lnI(out, "begin")
	g.indentUpImpl()

	for range members {
		g.finalizeField()
	}

	g.lnI(out, "inherited;")
	g.indentDownImpl()
	g.lnI(out, "end;")
	g.raw(out, "\n")

	// non-refcounted IUnknown impl
	g.lnI(out, "function "+clsPrefix+clsNm+".QueryInterface(const IID: TGUID; out Obj): HRESULT;")
	g.lnI(out, "begin")
	g.indentUpImpl()
	g.lnI(out, "if GetInterface(IID, Obj)")
	g.lnI(out, "then result := S_OK")
	g.lnI(out, "else result := E_NOINTERFACE;")
	g.indentDownImpl()
	g.lnI(out, "end;")
	g.raw(out, "\n")
	g.lnI(out, "function "+clsPrefix+clsNm+"._AddRef: Integer;")
	g.lnI(out, "begin")
	g.indentUpImpl()
	g.lnI(out, "result := -1;    // not refcounted")
	g.indentDownImpl()
	g.lnI(out, "end;")
	g.raw(out, "\n")
	g.lnI(out, "function "+clsPrefix+clsNm+"._Release: Integer;")
	g.lnI(out, "begin")
	g.indentUpImpl()
	g.lnI(out, "result := -1;    // not refcounted")
	g.indentDownImpl()
	g.lnI(out, "end;")
	g.raw(out, "\n")

	for _, f := range members {
		g.generateDelphiPropertyReaderImpl(out, clsPrefix, clsNm, f, "", true)
		g.generateDelphiPropertyWriterImpl(out, clsPrefix, clsNm, f, "", true, tstruct.IsUnion())
		if f.Req() != sema.Required {
			g.generateDelphiIssetReaderWriterImpl(out, clsPrefix, clsNm, f, "", true)
		}
	}

	g.generateDelphiStructReaderImpl(out, clsPrefix, tstruct, true)
	g.generateDelphiStructWriterImpl(out, clsPrefix, tstruct, true)
	g.generateDelphiStructTostringImpl(out, clsPrefix, tstruct, true)

	// Equals override for exception wrapper class
	g.lnI(out, "function "+clsPrefix+clsNm+".Equals(Obj: TObject): System.Boolean;")
	g.lnI(out, "begin")
	g.indentUpImpl()
	g.lnI(out, "if Obj is "+clsNm)
	g.lnI(out, "then Result := FData.Equal("+clsNm+"(Obj).FData)")
	g.lnI(out, "else Result := inherited Equals(Obj);")
	g.indentDownImpl()
	g.lnI(out, "end;")
	g.raw(out, "\n")
}

// printDelphiStructTypeFactoryFunc is
// t_delphi_generator::print_delphi_struct_type_factory_func.
func (g *Generator) printDelphiStructTypeFactoryFunc(out *strings.Builder, tstruct *sema.Struct) {
	structIntfName := g.typeName(tstruct, false, false)
	g.raw(out, "Create_")
	g.raw(out, structIntfName)
	g.raw(out, "_Impl")
}

// generateDelphiStructTypeFactory is
// t_delphi_generator::generate_delphi_struct_type_factory.
func (g *Generator) generateDelphiStructTypeFactory(out *strings.Builder, tstruct *sema.Struct) {
	structIntfName := g.typeName(tstruct, false, false)
	clsNm := g.typeName(tstruct, true, false)

	g.raw(out, "function ")
	g.printDelphiStructTypeFactoryFunc(out, tstruct)
	g.raw(out, ": ")
	g.raw(out, structIntfName)
	g.raw(out, ";\n")
	g.raw(out, "begin\n")
	g.indentUp()
	g.ln(out, "Result := "+clsNm+".Create;")
	g.indentDown()
	g.raw(out, "end;\n\n")
}

// generateDelphiStructTypeFactoryRegistration is
// t_delphi_generator::generate_delphi_struct_type_factory_registration.
func (g *Generator) generateDelphiStructTypeFactoryRegistration(out *strings.Builder, tstruct *sema.Struct) {
	structIntfName := g.typeName(tstruct, false, false)

	g.wr(out, "  TypeRegistry.RegisterTypeFactory<"+structIntfName+">(")
	g.printDelphiStructTypeFactoryFunc(out, tstruct)
	g.raw(out, ");")
	g.raw(out, "\n")
}
