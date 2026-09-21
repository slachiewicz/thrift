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

// generateDelphiStructReaderImpl is
// t_delphi_generator::generate_delphi_struct_reader_impl.
func (g *Generator) generateDelphiStructReaderImpl(out *strings.Builder, clsPrefix string, tstruct *sema.Struct, isException bool) {
	var localVars, codeBlock strings.Builder

	fields := tstruct.Members()

	g.lnI(&codeBlock, "begin")
	g.indentUpImpl()

	g.lnI(&localVars, "tracker : IProtocolRecursionTracker;")
	g.lnI(&codeBlock, "tracker := iprot.NextRecursionLevel;")

	for _, f := range fields {
		if f.Req() == sema.Required {
			g.lnI(&localVars, g.propNameF(f, isException, "_req_isset_")+" : System.Boolean;")
			g.lnI(&codeBlock, g.propNameF(f, isException, "_req_isset_")+" := FALSE;")
		}
	}

	g.lnI(&codeBlock, "struc := iprot.ReadStructBegin;")

	g.lnI(&codeBlock, "try")
	g.indentUpImpl()

	g.lnI(&codeBlock, "while (true) do begin")
	g.indentUpImpl()

	g.lnI(&codeBlock, "field_ := iprot.ReadFieldBegin();")

	g.lnI(&codeBlock, "if (field_.Type_ = TType.Stop) then Break;")

	first := true

	for i, f := range fields {
		if first {
			codeBlock.WriteString("\n")
			g.lnI(&codeBlock, "case field_.ID of")
			g.indentUpImpl()
		}

		first = false
		if i != 0 {
			codeBlock.WriteString("\n")
		}

		g.lnI(&codeBlock, itoa32(f.Key())+": begin")
		g.indentUpImpl()
		g.lnI(&codeBlock, "if (field_.Type_ = "+g.typeToEnum(f.Type())+") then begin")
		g.indentUpImpl()

		g.generateDeserializeField(&codeBlock, isException, f, "Self.", &localVars)

		if f.Req() == sema.Required {
			g.lnI(&codeBlock, g.propNameF(f, isException, "_req_isset_")+" := TRUE;")
		}

		g.indentDownImpl()

		g.lnI(&codeBlock, "end else begin")
		g.indentUpImpl()
		g.lnI(&codeBlock, "TProtocolUtil.Skip(iprot, field_.Type_);")
		g.indentDownImpl()
		g.lnI(&codeBlock, "end;")
		g.indentDownImpl()
		g.wrI(&codeBlock, "end;")
	}

	if !first {
		codeBlock.WriteString("\n")
		g.indentDownImpl()
		g.lnI(&codeBlock, "else")
		g.indentUpImpl()
	}

	g.lnI(&codeBlock, "TProtocolUtil.Skip(iprot, field_.Type_);")

	if !first {
		g.indentDownImpl()
		g.lnI(&codeBlock, "end;")
	}

	g.lnI(&codeBlock, "iprot.ReadFieldEnd;")

	g.indentDownImpl()

	g.lnI(&codeBlock, "end;")
	g.indentDownImpl()

	g.lnI(&codeBlock, "finally")
	g.indentUpImpl()
	g.lnI(&codeBlock, "iprot.ReadStructEnd;")
	g.indentDownImpl()
	g.lnI(&codeBlock, "end;")

	first = true
	for _, f := range fields {
		if f.Req() == sema.Required {
			if first {
				codeBlock.WriteString("\n")
				first = false
			}
			g.lnI(&codeBlock, "if not "+g.propNameF(f, isException, "_req_isset_"))
			g.lnI(&codeBlock, "then raise TProtocolExceptionInvalidData.Create("+"'required field "+g.propNameF(f, isException, "")+" not set');")
		}
	}

	if isException {
		codeBlock.WriteString("\n")
		g.lnI(&codeBlock, "UpdateMessageProperty;")
	}
	g.indentDownImpl()
	g.lnI(&codeBlock, "end;")
	codeBlock.WriteString("\n")

	clsNm := g.typeName(tstruct, true, isException)

	g.lnI(out, "procedure "+clsPrefix+clsNm+".Read( const iprot: IProtocol);")
	g.lnI(out, "var")
	g.indentUpImpl()
	g.lnI(out, "field_ : TThriftField;")
	g.lnI(out, "struc : TThriftStruct;")
	g.indentDownImpl()
	out.WriteString(localVars.String() + "\n")
	out.WriteString(codeBlock.String())
}

// generateDelphiStructWriterImpl is
// t_delphi_generator::generate_delphi_struct_writer_impl.
func (g *Generator) generateDelphiStructWriterImpl(out *strings.Builder, clsPrefix string, tstruct *sema.Struct, isException bool) {
	var localVars, codeBlock strings.Builder

	name := tstruct.Name()
	fields := tstruct.SortedMembers()

	g.lnI(&codeBlock, "begin")
	g.indentUpImpl()

	g.lnI(&localVars, "tracker : IProtocolRecursionTracker;")
	g.lnI(&codeBlock, "tracker := oprot.NextRecursionLevel;")

	g.lnI(&codeBlock, "Thrift.Protocol.Init( struc, '"+name+"');")
	g.lnI(&codeBlock, "oprot.WriteStructBegin(struc);")

	if len(fields) > 0 {
		g.lnI(&codeBlock, "Thrift.Protocol.Init( field_);")
	}

	for _, f := range fields {
		fieldname := g.propNameF(f, isException, "")
		issetName := g.propNameF(f, isException, "__isset_")
		nullAllowed := typeCanBeNull(f.Type())
		isRequired := f.Req() == sema.Required
		hasIsset := !isRequired
		if isRequired && nullAllowed {
			nullAllowed = false
			g.lnI(&codeBlock, "if (Self."+fieldname+" = nil)")
			g.lnI(&codeBlock, "then raise TProtocolExceptionInvalidData.Create("+"'required field "+fieldname+" not set');")
		}
		if nullAllowed {
			g.wrI(&codeBlock, "if (Self."+fieldname+" <> nil)")
			if hasIsset {
				codeBlock.WriteString(" and " + issetName)
			}
			codeBlock.WriteString(" then begin\n")
			g.indentUpImpl()
		} else if hasIsset {
			g.lnI(&codeBlock, "if ("+issetName+") then begin")
			g.indentUpImpl()
		}
		g.lnI(&codeBlock, "field_.Name := '"+f.Name()+"';")
		g.lnI(&codeBlock, "field_.Type_  := "+g.typeToEnum(f.Type())+";")
		g.lnI(&codeBlock, "field_.ID := "+itoa32(f.Key())+";")
		g.lnI(&codeBlock, "oprot.WriteFieldBegin(field_);")
		g.generateSerializeField(&codeBlock, isException, f, "Self.", &localVars)
		g.lnI(&codeBlock, "oprot.WriteFieldEnd();")
		if nullAllowed || hasIsset {
			g.indentDownImpl()
			g.lnI(&codeBlock, "end;")
		}
	}

	g.lnI(&codeBlock, "oprot.WriteFieldStop();")
	g.lnI(&codeBlock, "oprot.WriteStructEnd();")

	g.indentDownImpl()
	g.lnI(&codeBlock, "end;")
	codeBlock.WriteString("\n")

	clsNm := g.typeName(tstruct, true, isException)

	g.lnI(out, "procedure "+clsPrefix+clsNm+".Write( const oprot: IProtocol);")
	g.lnI(out, "var")
	g.indentUpImpl()
	g.lnI(out, "struc : TThriftStruct;")
	if len(fields) > 0 {
		g.lnI(out, "field_ : TThriftField;")
	}
	out.WriteString(localVars.String())
	g.indentDownImpl()
	out.WriteString(codeBlock.String())
}

// generateDelphiStructTostringImpl is
// t_delphi_generator::generate_delphi_struct_tostring_impl.
func (g *Generator) generateDelphiStructTostringImpl(out *strings.Builder, clsPrefix string, tstruct *sema.Struct, isException bool) {
	fields := tstruct.Members()

	clsNm := g.typeName(tstruct, true, isException)

	tmpSb := g.tmp("_sb")
	tmpFirst := g.tmp("_first")
	useFirstFlag := false

	g.lnI(out, "function "+clsPrefix+clsNm+".ToString: string;")
	g.lnI(out, "var")
	g.indentUpImpl()
	g.lnI(out, tmpSb+" : TThriftStringBuilder;")
	for _, f := range fields {
		isOptional := f.Req() != sema.Required
		if isOptional {
			g.lnI(out, tmpFirst+" : System.Boolean;")
			useFirstFlag = true
		}
		break
	}
	g.indentDownImpl()
	g.lnI(out, "begin")
	g.indentUpImpl()

	g.lnI(out, tmpSb+" := TThriftStringBuilder.Create('(');")
	g.lnI(out, "try")
	g.indentUpImpl()

	if useFirstFlag {
		g.lnI(out, tmpFirst+" := TRUE;")
	}

	hadRequired := false

	for _, f := range fields {
		nullAllowed := typeCanBeNull(f.Type())
		isOptional := f.Req() != sema.Required
		if nullAllowed {
			g.wrI(out, "if (Self."+g.propNameF(f, isException, "")+" <> nil)")
			if isOptional {
				out.WriteString(" and " + g.propNameF(f, isException, "__isset_"))
			}
			out.WriteString(" then begin\n")
			g.indentUpImpl()
		} else if isOptional {
			g.lnI(out, "if ("+g.propNameF(f, isException, "__isset_")+") then begin")
			g.indentUpImpl()
		}

		if useFirstFlag && !hadRequired {
			g.lnI(out, "if not "+tmpFirst+" then "+tmpSb+".Append(',');")
			if isOptional {
				g.lnI(out, tmpFirst+" := FALSE;")
			}
			g.lnI(out, tmpSb+".Append('"+g.propNameF(f, isException, "")+": ');")
		} else {
			g.lnI(out, tmpSb+".Append(', "+g.propNameF(f, isException, "")+": ');")
		}

		ttype := sema.TrueType(f.Type())

		if ttype.IsXception() || ttype.IsStruct() {
			g.lnI(out, "if (Self."+g.propNameF(f, isException, "")+" = nil) then "+tmpSb+
				".Append('<null>') else "+tmpSb+".Append( Self."+g.propNameF(f, isException, "")+".ToString());")
		} else if ttype.IsEnum() {
			g.lnI(out, tmpSb+".Append(EnumUtils<"+g.typeName(ttype, false, true)+
				">.ToString( System.Ord( Self."+g.propNameF(f, isException, "")+")));")
		} else if ttype.IsUUID() {
			g.lnI(out, tmpSb+".Append( GUIDToString(Self."+g.propNameF(f, isException, "")+"));")
		} else {
			g.lnI(out, tmpSb+".Append( Self."+g.propNameF(f, isException, "")+");")
		}

		if nullAllowed || isOptional {
			g.indentDownImpl()
			g.lnI(out, "end;")
		}

		if !isOptional {
			hadRequired = true
		}
	}

	g.lnI(out, tmpSb+".Append(')');")
	g.lnI(out, "Result := "+tmpSb+".ToString;")
	if useFirstFlag {
		g.lnI(out, "if "+tmpFirst+" then {prevent warning};")
	}

	g.indentDownImpl()
	g.lnI(out, "finally")
	g.indentUpImpl()
	g.lnI(out, tmpSb+".Free;")
	g.indentDownImpl()
	g.lnI(out, "end;")

	g.indentDownImpl()
	g.lnI(out, "end;")
	out.WriteString("\n")
}
