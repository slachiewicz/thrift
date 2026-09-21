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

// generateEqualContainer is t_delphi_generator::generate_equal_container.
//
// Emit comparison code for a struct, binary or container value into
// `code`, with any new locals accumulated into `vars` (2-space prefix,
// level-independent).
//
// In exit mode (exitOnFail = true) a mismatch leaves the generated Equal
// function directly via Exit(False) and an empty string is returned. In
// trial mode a mismatch only records the outcome: a fresh Boolean
// variable, whose name is returned, is set to False and control falls
// through, so the caller can keep probing other candidates. The set/map
// consume-scans below rely on that to test rhs candidates without
// aborting the whole comparison.
//
// Unless alreadyLocal is set, both operands are captured into fresh
// locals first, so property getters, GetItem calls and hash lookups are
// evaluated once instead of once per mention.
func (g *Generator) generateEqualContainer(code, vars *strings.Builder, ttype sema.Type, lhsIn, rhsIn string, exitOnFail, alreadyLocal bool) string {
	ttype = sema.TrueType(ttype)

	lhs, rhs := lhsIn, rhsIn
	if !alreadyLocal {
		lhs = g.tmp("_l")
		rhs = g.tmp("_r")
		vars.WriteString("  " + lhs + " : " + g.typeName(ttype, false, false) + ";\n")
		vars.WriteString("  " + rhs + " : " + g.typeName(ttype, false, false) + ";\n")
		g.lnI(code, lhs+" := "+lhsIn+";")
		g.lnI(code, rhs+" := "+rhsIn+";")
	}

	eqVar := ""
	if !exitOnFail {
		eqVar = g.tmp("_eq")
		vars.WriteString("  " + eqVar + " : System.Boolean;\n")
		g.lnI(code, eqVar+" := True;")
	}

	failStmt := "Exit(False);"
	if !exitOnFail {
		failStmt = "begin " + eqVar + " := False; break; end;"
	}

	containerProlog := func() {
		if exitOnFail {
			g.lnI(code, "if ("+lhs+" = nil) <> ("+rhs+" = nil) then Exit(False);")
			g.lnI(code, "if "+lhs+" <> nil then begin")
			g.indentUpImpl()
			g.lnI(code, "if "+lhs+".Count <> "+rhs+".Count then Exit(False);")
		} else {
			g.lnI(code, "if ("+lhs+" = nil) <> ("+rhs+" = nil) then "+eqVar+" := False")
			g.lnI(code, "else if "+lhs+" <> nil then begin")
			g.indentUpImpl()
			g.lnI(code, "if "+lhs+".Count <> "+rhs+".Count then "+eqVar+" := False")
			g.lnI(code, "else begin")
			g.indentUpImpl()
		}
	}
	containerEpilog := func() {
		if !exitOnFail {
			g.indentDownImpl()
			g.lnI(code, "end;") // end else (counts equal)
		}
		g.indentDownImpl()
		g.lnI(code, "end;") // end if lhs <> nil
	}

	switch {
	case ttype.IsStruct() || ttype.IsXception():
		if exitOnFail {
			g.lnI(code, "if ("+lhs+" = nil) <> ("+rhs+" = nil) then Exit(False);")
			g.lnI(code, "if ("+lhs+" <> nil) and not "+lhs+".Equal("+rhs+") then Exit(False);")
		} else {
			g.lnI(code, "if ("+lhs+" = nil) <> ("+rhs+" = nil) then "+eqVar+" := False")
			g.lnI(code, "else if "+lhs+" <> nil then "+eqVar+" := "+lhs+".Equal("+rhs+");")
		}

	case ttype.IsBinary():
		switch {
		case g.opts.ComTypes:
			// IThriftBytes -- nullable interface
			if exitOnFail {
				g.lnI(code, "if ("+lhs+" = nil) <> ("+rhs+" = nil) then Exit(False);")
				g.lnI(code, "if "+lhs+" <> nil then begin")
				g.indentUpImpl()
				g.lnI(code, "if "+lhs+".Count <> "+rhs+".Count then Exit(False);")
				g.lnI(code, "if ("+lhs+".Count > 0) and not SysUtils.CompareMem("+lhs+".QueryRawDataPtr, "+rhs+".QueryRawDataPtr, "+lhs+".Count) then Exit(False);")
				g.indentDownImpl()
				g.lnI(code, "end;")
			} else {
				g.lnI(code, "if ("+lhs+" = nil) <> ("+rhs+" = nil) then "+eqVar+" := False")
				g.lnI(code, "else if "+lhs+" <> nil then begin")
				g.indentUpImpl()
				g.lnI(code, "if "+lhs+".Count <> "+rhs+".Count then "+eqVar+" := False")
				g.lnI(code, "else if "+lhs+".Count > 0 then")
				g.lnI(code, "  "+eqVar+" := SysUtils.CompareMem("+lhs+".QueryRawDataPtr, "+rhs+".QueryRawDataPtr, "+lhs+".Count);")
				g.indentDownImpl()
				g.lnI(code, "end;")
			}
		case g.opts.RTTI:
			// TThriftBytes -- packed record with .data and .Length
			if exitOnFail {
				g.lnI(code, "if "+lhs+".Length <> "+rhs+".Length then Exit(False);")
				g.lnI(code, "if ("+lhs+".Length > 0) and not SysUtils.CompareMem(@"+lhs+".data[0], @"+rhs+".data[0], "+lhs+".Length) then Exit(False);")
			} else {
				g.lnI(code, "if "+lhs+".Length <> "+rhs+".Length then "+eqVar+" := False")
				g.lnI(code, "else if "+lhs+".Length > 0 then")
				g.lnI(code, "  "+eqVar+" := SysUtils.CompareMem(@"+lhs+".data[0], @"+rhs+".data[0], "+lhs+".Length);")
			}
		default:
			// SysUtils.TBytes -- dynamic array
			if exitOnFail {
				g.lnI(code, "if System.Length("+lhs+") <> System.Length("+rhs+") then Exit(False);")
				g.lnI(code, "if (System.Length("+lhs+") > 0) and not SysUtils.CompareMem("+"PByte("+lhs+"), PByte("+rhs+"), System.Length("+lhs+")) then Exit(False);")
			} else {
				g.lnI(code, "if System.Length("+lhs+") <> System.Length("+rhs+") then "+eqVar+" := False")
				g.lnI(code, "else if System.Length("+lhs+") > 0 then")
				g.lnI(code, "  "+eqVar+" := SysUtils.CompareMem("+"PByte("+lhs+"), PByte("+rhs+"), System.Length("+lhs+"));")
			}
		}

	case ttype.IsList():
		tlist := ttype.(*sema.List)
		elemType := sema.TrueType(tlist.ElemType())
		iVar := g.tmp("_i")
		vars.WriteString("  " + iVar + " : System.Integer;\n")
		elemNeedsHelper := typeCanBeNull(elemType) || elemType.IsBinary() || elemType.IsContainer()

		containerProlog()
		g.lnI(code, "for "+iVar+" := 0 to "+lhs+".Count - 1 do begin")
		g.indentUpImpl()

		if !elemNeedsHelper {
			if elemType.IsBaseType() && elemType.(*sema.BaseType).Base() == sema.TypeUUID {
				g.lnI(code, "if not SysUtils.IsEqualGUID("+lhs+"["+iVar+"], "+rhs+"["+iVar+"]) then "+failStmt)
			} else {
				g.lnI(code, "if "+lhs+"["+iVar+"] <> "+rhs+"["+iVar+"] then "+failStmt)
			}
		} else {
			if exitOnFail {
				g.generateEqualContainer(code, vars, elemType, lhs+"["+iVar+"]", rhs+"["+iVar+"]", true, false)
			} else {
				inner := g.generateEqualContainer(code, vars, elemType, lhs+"["+iVar+"]", rhs+"["+iVar+"]", false, false)
				g.lnI(code, "if not "+inner+" then "+failStmt)
			}
		}

		g.indentDownImpl()
		g.lnI(code, "end;") // end for
		containerEpilog()

	case ttype.IsSet():
		tset := ttype.(*sema.Set)
		elemType := sema.TrueType(tset.ElemType())
		elemNeedsScan := typeCanBeNull(elemType) || elemType.IsBinary() || elemType.IsContainer()

		containerProlog()

		eVar := g.tmp("_e")
		vars.WriteString("  " + eVar + " : " + g.typeName(elemType, false, false) + ";\n")

		if !elemNeedsScan {
			g.lnI(code, "for "+eVar+" in "+lhs+" do")
			g.raw(code, g.indentImpl()+"  if not "+rhs+".Contains("+eVar+") then "+failStmt+"\n")
		} else {
			arrVar := g.tmp("_arr")
			usedVar := g.tmp("_used")
			idxVar := g.tmp("_i")
			foundVar := g.tmp("_found")
			e2Var := g.tmp("_e")
			vars.WriteString("  " + arrVar + " : array of " + g.typeName(elemType, false, false) + ";\n")
			vars.WriteString("  " + usedVar + " : array of System.Boolean;\n")
			vars.WriteString("  " + idxVar + " : System.Integer;\n")
			vars.WriteString("  " + foundVar + " : System.Boolean;\n")
			vars.WriteString("  " + e2Var + " : " + g.typeName(elemType, false, false) + ";\n")

			g.lnI(code, "System.SetLength("+arrVar+", "+rhs+".Count);")
			g.lnI(code, "System.SetLength("+usedVar+", "+rhs+".Count);")
			g.lnI(code, idxVar+" := 0;")
			g.lnI(code, "for "+e2Var+" in "+rhs+" do begin")
			g.indentUpImpl()
			g.lnI(code, arrVar+"["+idxVar+"] := "+e2Var+";")
			g.lnI(code, "System.Inc("+idxVar+");")
			g.indentDownImpl()
			g.lnI(code, "end;")

			g.lnI(code, "for "+eVar+" in "+lhs+" do begin")
			g.indentUpImpl()
			g.lnI(code, foundVar+" := False;")
			g.lnI(code, "for "+idxVar+" := 0 to System.Length("+arrVar+") - 1 do begin")
			g.indentUpImpl()
			g.lnI(code, "if not "+usedVar+"["+idxVar+"] then begin")
			g.indentUpImpl()
			// the candidate probe must not abort the function -> always trial mode
			innerEq := g.generateEqualContainer(code, vars, elemType, eVar, arrVar+"["+idxVar+"]", false, true)
			g.lnI(code, "if "+innerEq+" then begin")
			g.indentUpImpl()
			g.lnI(code, usedVar+"["+idxVar+"] := True;")
			g.lnI(code, foundVar+" := True;")
			g.lnI(code, "break;")
			g.indentDownImpl()
			g.lnI(code, "end;") // end if matched
			g.indentDownImpl()
			g.lnI(code, "end;") // end if not yet used
			g.indentDownImpl()
			g.lnI(code, "end;") // end scan for
			g.lnI(code, "if not "+foundVar+" then "+failStmt)
			g.indentDownImpl()
			g.lnI(code, "end;") // end outer for
		}

		containerEpilog()

	case ttype.IsMap():
		tmap := ttype.(*sema.Map)
		ktype := sema.TrueType(tmap.KeyType())
		vtype := sema.TrueType(tmap.ValType())
		keyNeedsScan := typeCanBeNull(ktype) || ktype.IsBinary() || ktype.IsContainer()
		valNeedsHelper := typeCanBeNull(vtype) || vtype.IsBinary() || vtype.IsContainer()
		valIsUUID := vtype.IsBaseType() && vtype.(*sema.BaseType).Base() == sema.TypeUUID

		containerProlog()

		// NB: iterate the dictionary's .Keys (and look values up by key)
		// rather than using "for pair in dict". For-in over the TPair
		// enumerator returned through the IThriftDictionary interface
		// miscompiles on some Delphi versions (bad result pointer in
		// TPairEnumerator.GetCurrent -> access violation); .Keys
		// enumeration is safe and is the same pattern the generated
		// Read/Write code uses.
		if !keyNeedsScan {
			kVar := g.tmp("_key")
			vVar := g.tmp("_v")
			vars.WriteString("  " + kVar + " : " + g.typeName(ktype, false, false) + ";\n")
			vars.WriteString("  " + vVar + " : " + g.typeName(vtype, false, false) + ";\n")

			g.lnI(code, "for "+kVar+" in "+lhs+".Keys do begin")
			g.indentUpImpl()
			g.lnI(code, "if not "+rhs+".TryGetValue("+kVar+", "+vVar+") then "+failStmt)

			vl := lhs + "[" + kVar + "]"
			if !valNeedsHelper {
				if valIsUUID {
					g.lnI(code, "if not SysUtils.IsEqualGUID("+vl+", "+vVar+") then "+failStmt)
				} else {
					g.lnI(code, "if "+vl+" <> "+vVar+" then "+failStmt)
				}
			} else {
				if exitOnFail {
					g.generateEqualContainer(code, vars, vtype, vl, vVar, true, true)
				} else {
					valInner := g.generateEqualContainer(code, vars, vtype, vl, vVar, false, true)
					g.lnI(code, "if not "+valInner+" then "+failStmt)
				}
			}

			g.indentDownImpl()
			g.lnI(code, "end;") // end for keys

		} else {
			klVar := g.tmp("_key")
			krVar := g.tmp("_key")
			karrVar := g.tmp("_karr")
			varrVar := g.tmp("_varr")
			usedVar := g.tmp("_used")
			idxVar := g.tmp("_i")
			foundVar := g.tmp("_found")
			vars.WriteString("  " + klVar + " : " + g.typeName(ktype, false, false) + ";\n")
			vars.WriteString("  " + krVar + " : " + g.typeName(ktype, false, false) + ";\n")
			vars.WriteString("  " + karrVar + " : array of " + g.typeName(ktype, false, false) + ";\n")
			vars.WriteString("  " + varrVar + " : array of " + g.typeName(vtype, false, false) + ";\n")
			vars.WriteString("  " + usedVar + " : array of System.Boolean;\n")
			vars.WriteString("  " + idxVar + " : System.Integer;\n")
			vars.WriteString("  " + foundVar + " : System.Boolean;\n")

			g.lnI(code, "System.SetLength("+karrVar+", "+rhs+".Count);")
			g.lnI(code, "System.SetLength("+varrVar+", "+rhs+".Count);")
			g.lnI(code, "System.SetLength("+usedVar+", "+rhs+".Count);")
			g.lnI(code, idxVar+" := 0;")
			g.lnI(code, "for "+krVar+" in "+rhs+".Keys do begin")
			g.indentUpImpl()
			g.lnI(code, karrVar+"["+idxVar+"] := "+krVar+";")
			g.lnI(code, varrVar+"["+idxVar+"] := "+rhs+"["+krVar+"];")
			g.lnI(code, "System.Inc("+idxVar+");")
			g.indentDownImpl()
			g.lnI(code, "end;")

			g.lnI(code, "for "+klVar+" in "+lhs+".Keys do begin")
			g.indentUpImpl()
			g.lnI(code, foundVar+" := False;")
			g.lnI(code, "for "+idxVar+" := 0 to System.Length("+karrVar+") - 1 do begin")
			g.indentUpImpl()
			g.lnI(code, "if not "+usedVar+"["+idxVar+"] then begin")
			g.indentUpImpl()
			// the candidate probe must not abort the function -> always trial mode
			keyInner := g.generateEqualContainer(code, vars, ktype, klVar, karrVar+"["+idxVar+"]", false, true)
			g.lnI(code, "if "+keyInner+" then begin")
			g.indentUpImpl()
			// Keys match: compare values, and only consume the rhs key/mark
			// found if the value matches too (a key-only match with a
			// differing value must keep scanning).
			vl := lhs + "[" + klVar + "]"
			vr := varrVar + "[" + idxVar + "]"
			if !valNeedsHelper {
				if valIsUUID {
					g.lnI(code, "if SysUtils.IsEqualGUID("+vl+", "+vr+") then begin")
				} else {
					g.lnI(code, "if "+vl+" = "+vr+" then begin")
				}
				g.indentUpImpl()
				g.lnI(code, usedVar+"["+idxVar+"] := True;")
				g.lnI(code, foundVar+" := True;")
				g.lnI(code, "break;")
				g.indentDownImpl()
				g.lnI(code, "end;") // end if value matches
			} else {
				valInner := g.generateEqualContainer(code, vars, vtype, vl, vr, false, true)
				g.lnI(code, "if "+valInner+" then begin")
				g.indentUpImpl()
				g.lnI(code, usedVar+"["+idxVar+"] := True;")
				g.lnI(code, foundVar+" := True;")
				g.lnI(code, "break;")
				g.indentDownImpl()
				g.lnI(code, "end;") // end if val_inner
			}
			g.indentDownImpl()
			g.lnI(code, "end;") // end if key_inner
			g.indentDownImpl()
			g.lnI(code, "end;") // end if not yet used
			g.indentDownImpl()
			g.lnI(code, "end;") // end scan for
			g.lnI(code, "if not "+foundVar+" then "+failStmt)
			g.indentDownImpl()
			g.lnI(code, "end;") // end for lhs keys
		}

		containerEpilog()
	}

	return eqVar
}

// generateDelphiStructEqualityImpl is
// t_delphi_generator::generate_delphi_struct_equality_impl.
func (g *Generator) generateDelphiStructEqualityImpl(out *strings.Builder, clsPrefix string, tstruct *sema.Struct) {
	var localVars, codeBlock strings.Builder

	fields := tstruct.Members()

	clsNm := g.typeName(tstruct, true, false)
	intfNm := g.typeName(tstruct, false, false)

	g.lnI(&codeBlock, "begin")
	g.indentUpImpl()

	g.lnI(&codeBlock, "if not Supports(other, "+intfNm+", _eq_other) then Exit(False);")

	for _, field := range fields {
		ftype := sema.TrueType(field.Type())
		isOptional := field.Req() != sema.Required
		nullAllowed := typeCanBeNull(ftype)
		isBin := ftype.IsBaseType() && ftype.IsBinary()
		needsHelper := nullAllowed || isBin || ftype.IsContainer()

		selfProp := "Self." + g.propNameF(field, false, "")
		otherProp := "_eq_other." + g.propNameF(field, false, "")
		issetSelf := g.propNameF(field, false, "__isset_")
		issetOther := "_eq_other." + g.propNameF(field, false, "__isset_")

		if isOptional {
			g.lnI(&codeBlock, "if "+issetSelf+" <> "+issetOther+" then Exit(False);")
			g.lnI(&codeBlock, "if "+issetSelf+" then begin")
			g.indentUpImpl()
		}

		if !needsHelper {
			if ftype.IsBaseType() && ftype.(*sema.BaseType).Base() == sema.TypeUUID {
				g.lnI(&codeBlock, "if not SysUtils.IsEqualGUID("+selfProp+", "+otherProp+") then Exit(False);")
			} else {
				g.lnI(&codeBlock, "if "+selfProp+" <> "+otherProp+" then Exit(False);")
			}
		} else {
			g.generateEqualContainer(&codeBlock, &localVars, ftype, selfProp, otherProp, true, false)
		}

		if isOptional {
			g.indentDownImpl()
			g.lnI(&codeBlock, "end;")
		}
	}

	g.lnI(&codeBlock, "Result := True;")
	g.indentDownImpl()
	g.lnI(&codeBlock, "end;")
	codeBlock.WriteString("\n")

	// Equal(other: IInterface): Boolean
	g.lnI(out, "function "+clsPrefix+clsNm+".Equal(const other: IInterface): System.Boolean;")
	g.lnI(out, "var")
	g.indentUpImpl()
	g.lnI(out, "_eq_other : "+intfNm+";")
	g.indentDownImpl()
	if localVars.Len() > 0 {
		out.WriteString(localVars.String())
	}
	out.WriteString(codeBlock.String())

	// Equals(Obj: TObject): Boolean override
	intfVar := g.tmp("_intf")
	g.lnI(out, "function "+clsPrefix+clsNm+".Equals(Obj: TObject): System.Boolean;")
	g.lnI(out, "var")
	g.indentUpImpl()
	g.lnI(out, intfVar+" : IInterface;")
	g.indentDownImpl()
	g.lnI(out, "begin")
	g.indentUpImpl()
	g.lnI(out, "if Supports(Obj, IInterface, "+intfVar+")")
	g.lnI(out, "then Result := Equal("+intfVar+")")
	g.lnI(out, "else Result := inherited Equals(Obj);")
	g.indentDownImpl()
	g.lnI(out, "end;")
	out.WriteString("\n")
}
