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

package rs

import (
	"strconv"

	"github.com/apache/thrift/compiler/go/sema"
)

// isUnionType reports whether the true type behind t is a union struct.
func isUnionType(t sema.Type) bool {
	rt := sema.TrueType(t)
	return rt.IsStruct() && rt.(*sema.Struct).IsUnion()
}

// isBoxedByForwardTypedef walks the typedef chain of t one layer at a
// time, exactly as the C++ code's local loop does, and reports whether a
// forward typedef was found before a non-typedef type.
func isBoxedByForwardTypedef(t sema.Type) bool {
	for t.IsTypedef() {
		td := t.(*sema.Typedef)
		if td.IsForwardTypedef() {
			return true
		}
		t = td.Type()
	}
	return false
}

//-----------------------------------------------------------------------------
//
// Sync Struct Write
//
//-----------------------------------------------------------------------------

// renderStructSyncWrite is t_rs_generator::render_struct_sync_write.
func (g *generator) renderStructSyncWrite(s *sema.Struct, st structType) {
	g.line("fn write_to_out_protocol(&self, o_prot: &mut dyn TOutputProtocol) -> thrift::Result<()> {")
	g.up()

	// write struct header to output protocol
	// note: use the *original* struct name here
	g.line("let struct_ident = TStructIdentifier::new(\"" + s.Name() + "\");")
	g.line("o_prot.write_struct_begin(&struct_ident)?;")

	// write struct members to output protocol
	members := s.SortedMembers()
	for _, member := range members {
		memberReq := actualFieldReq(member, st)
		memberVar := "self." + rustFieldName(member)
		g.renderStructFieldSyncWrite(memberVar, false, member, memberReq)
	}

	// write struct footer to output protocol
	g.line("o_prot.write_field_stop()?;")
	g.line("o_prot.write_struct_end()")

	g.down()
	g.line("}")
}

// renderUnionSyncWrite is t_rs_generator::render_union_sync_write.
func (g *generator) renderUnionSyncWrite(unionName string, s *sema.Struct) {
	g.line("fn write_to_out_protocol(&self, o_prot: &mut dyn TOutputProtocol) -> thrift::Result<()> {")
	g.up()

	// write struct header to output protocol
	// note: use the *original* struct name here
	g.line("let struct_ident = TStructIdentifier::new(\"" + s.Name() + "\");")
	g.line("o_prot.write_struct_begin(&struct_ident)?;")

	// write the enum field to the output protocol
	members := s.SortedMembers()
	if len(members) != 0 {
		g.line("match *self {")
		g.up()
		for _, member := range members {
			ttype := member.Type()
			if ttype.IsTypedef() {
				// get the actual type of typedef
				ttype = ttype.(*sema.Typedef).Type()
			}
			matchVar := "f"
			if !(ttype.IsBaseType() && !ttype.IsString()) {
				matchVar = "ref f"
			}
			g.line(unionName + "::" + rustUnionFieldName(member) + "(" + matchVar + ") => {")
			g.up()
			g.renderStructFieldSyncWrite("f", true, member, sema.Required)
			g.down()
			g.line("},")
		}
		g.down()
		g.line("}")
	}

	// write struct footer to output protocol
	g.line("o_prot.write_field_stop()?;")
	g.line("o_prot.write_struct_end()")

	g.down()
	g.line("}")
}

// renderStructFieldSyncWrite is
// t_rs_generator::render_struct_field_sync_write.
func (g *generator) renderStructFieldSyncWrite(fieldVar string, fieldVarIsRef bool, tfield *sema.Field, req sema.Requiredness) {
	fieldType := tfield.Type()
	actualType := sema.TrueType(fieldType)

	fieldIdentString := "TFieldIdentifier::new(" + "\"" + tfield.Name() + "\"" + ", " + toRustFieldTypeEnum(fieldType) + ", " + strconv.FormatInt(int64(tfield.Key()), 10) + ")"

	if isOptional(req) {
		letVar := "fld_var"
		if !(actualType.IsBaseType() && !actualType.IsString()) {
			letVar = "ref fld_var"
		}
		g.line("if let Some(" + letVar + ") = " + fieldVar + " {")
		g.up()
		g.line("o_prot.write_field_begin(&" + fieldIdentString + ")?;")
		g.renderTypeSyncWrite("fld_var", true, fieldType)
		g.line("o_prot.write_field_end()?")
		g.down()
		g.line("}")
	} else {
		g.line("o_prot.write_field_begin(&" + fieldIdentString + ")?;")
		g.renderTypeSyncWrite(fieldVar, fieldVarIsRef, tfield.Type())
		g.line("o_prot.write_field_end()?;")
	}
}

// renderTypeSyncWrite is t_rs_generator::render_type_sync_write.
func (g *generator) renderTypeSyncWrite(typeVar string, typeVarIsRef bool, t sema.Type) {
	switch {
	case t.IsBaseType():
		b := t.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeVoid:
			throwf("cannot write field of type TYPE_VOID to output protocol")
		case sema.TypeString:
			ref := "&"
			if typeVarIsRef {
				ref = ""
			}
			if b.IsBinary() {
				g.line("o_prot.write_bytes(" + ref + typeVar + ")?;")
			} else {
				g.line("o_prot.write_string(" + ref + typeVar + ")?;")
			}
			return
		case sema.TypeUUID:
			g.line("o_prot.write_uuid(&" + typeVar + ")?;")
			return
		case sema.TypeBool:
			g.line("o_prot.write_bool(" + typeVar + ")?;")
			return
		case sema.TypeI8:
			g.line("o_prot.write_i8(" + typeVar + ")?;")
			return
		case sema.TypeI16:
			g.line("o_prot.write_i16(" + typeVar + ")?;")
			return
		case sema.TypeI32:
			g.line("o_prot.write_i32(" + typeVar + ")?;")
			return
		case sema.TypeI64:
			g.line("o_prot.write_i64(" + typeVar + ")?;")
			return
		case sema.TypeDouble:
			g.line("o_prot.write_double(" + typeVar + ".into())?;")
			return
		default:
			throwf("compiler error: unhandled type")
		}
	case t.IsTypedef():
		td := t.(*sema.Typedef)
		g.renderTypeSyncWrite(typeVar, typeVarIsRef, td.Type())
		return
	case t.IsEnum(), t.IsStruct(), t.IsXception():
		g.line(typeVar + ".write_to_out_protocol(o_prot)?;")
		return
	case t.IsMap():
		g.renderMapSyncWrite(typeVar, typeVarIsRef, t.(*sema.Map))
		return
	case t.IsSet():
		g.renderSetSyncWrite(typeVar, typeVarIsRef, t.(*sema.Set))
		return
	case t.IsList():
		g.renderListSyncWrite(typeVar, typeVarIsRef, t.(*sema.List))
		return
	}

	throwf("cannot write unsupported type %s", t.Name())
}

// renderListSyncWrite is t_rs_generator::render_list_sync_write.
func (g *generator) renderListSyncWrite(listVar string, listVarIsRef bool, tlist *sema.List) {
	elemType := tlist.ElemType()

	g.line("o_prot.write_list_begin(&TListIdentifier::new(" + toRustFieldTypeEnum(elemType) + ", " + listVar + ".len() as i32))?;")

	ref := "&"
	if listVarIsRef {
		ref = ""
	}
	g.line("for e in " + ref + listVar + " {")
	g.up()
	g.renderTypeSyncWrite(g.stringContainerWriteVariable(elemType, "e"), true, elemType)
	g.down()
	g.line("}")
	g.line("o_prot.write_list_end()?;")
}

// renderSetSyncWrite is t_rs_generator::render_set_sync_write.
func (g *generator) renderSetSyncWrite(setVar string, setVarIsRef bool, tset *sema.Set) {
	elemType := tset.ElemType()

	g.line("o_prot.write_set_begin(&TSetIdentifier::new(" + toRustFieldTypeEnum(elemType) + ", " + setVar + ".len() as i32))?;")

	ref := "&"
	if setVarIsRef {
		ref = ""
	}
	g.line("for e in " + ref + setVar + " {")
	g.up()
	g.renderTypeSyncWrite(g.stringContainerWriteVariable(elemType, "e"), true, elemType)
	g.down()
	g.line("}")
	g.line("o_prot.write_set_end()?;")
}

// renderMapSyncWrite is t_rs_generator::render_map_sync_write.
func (g *generator) renderMapSyncWrite(mapVar string, mapVarIsRef bool, tmap *sema.Map) {
	keyType, valType := tmap.KeyType(), tmap.ValType()

	g.line("o_prot.write_map_begin(&TMapIdentifier::new(" + toRustFieldTypeEnum(keyType) + ", " + toRustFieldTypeEnum(valType) + ", " + mapVar + ".len() as i32))?;")

	ref := "&"
	if mapVarIsRef {
		ref = ""
	}
	g.line("for (k, v) in " + ref + mapVar + " {")
	g.up()
	g.renderTypeSyncWrite(g.stringContainerWriteVariable(keyType, "k"), true, keyType)
	g.renderTypeSyncWrite(g.stringContainerWriteVariable(valType, "v"), true, valType)
	g.down()
	g.line("}")
	g.line("o_prot.write_map_end()?;")
}

// stringContainerWriteVariable is
// t_rs_generator::string_container_write_variable.
func (g *generator) stringContainerWriteVariable(t sema.Type, baseVar string) string {
	typeNeedsDeref := needsDerefOnContainerWrite(t)
	typeIsDouble := isDouble(t)

	if typeIsDouble && typeNeedsDeref {
		return "(*" + baseVar + ")"
	}
	if typeNeedsDeref {
		return "*" + baseVar
	}
	return baseVar
}

// needsDerefOnContainerWrite is
// t_rs_generator::needs_deref_on_container_write.
func needsDerefOnContainerWrite(t sema.Type) bool {
	t = sema.TrueType(t)
	return t.IsBaseType() && !t.IsString()
}

//-----------------------------------------------------------------------------
//
// Sync Struct Read
//
//-----------------------------------------------------------------------------

// renderStructSyncRead is t_rs_generator::render_struct_sync_read.
func (g *generator) renderStructSyncRead(structName string, s *sema.Struct, st structType) {
	g.line("fn read_from_in_protocol(i_prot: &mut dyn TInputProtocol) -> thrift::Result<" + structName + "> {")

	g.up()

	g.line("i_prot.read_struct_begin()?;")

	// create temporary variables: one for each field in the struct
	members := s.SortedMembers()
	for _, member := range members {
		memberReq := actualFieldReq(member, st)

		g.indentRaw("let mut " + structFieldReadTempVariable(member) + ": Option<" + g.toRustType(member.Type()) + "> = ")
		if memberReq == sema.OptInReqOut {
			g.raw(optInReqOutValue(member.Type()) + ";")
		} else {
			g.raw("None;")
		}
		g.raw("\n")
	}

	// now loop through the fields we've received
	g.line("loop {") // start loop
	g.up()

	// break out if you've found the Stop field
	g.line("let field_ident = i_prot.read_field_begin()?;")
	g.line("if field_ident.field_type == TType::Stop {")
	g.up()
	g.line("break;")
	g.down()
	g.line("}")

	// now read all the fields found
	// avoid clippy::match_single_binding
	if len(members) == 0 {
		g.line("i_prot.skip(field_ident.field_type)?;")
	} else {
		g.line("let field_id = field_id(&field_ident)?;")
		g.line("match field_id {") // start match
		g.up()

		for _, tfield := range members {
			resolved := sema.TrueType(tfield.Type())
			isUnionField := resolved.IsStruct() && resolved.(*sema.Struct).IsUnion()
			g.line(rustSafeFieldID(tfield.Key()) + " => {")
			g.up()
			if isUnionField {
				// Use the resolved (non-Box) type since Box<T>::method() isn't valid syntax.
				resolvedType := g.toRustType(resolved)
				isBoxed := isBoxedByForwardTypedef(tfield.Type())
				readCall := resolvedType + "::read_from_in_protocol(i_prot)"
				valExpr := "val"
				if isBoxed {
					valExpr = "Box::new(val)"
				}
				suppressUnknown := (st == structRegular || st == structException) && isOptional(actualFieldReq(tfield, st))
				if suppressUnknown {
					g.line("match " + readCall + " {")
					g.up()
					g.line("Ok(val) => { " + structFieldReadTempVariable(tfield) + " = Some(" + valExpr + "); },")
					g.line("Err(thrift::Error::Protocol(ref e)) if e.kind == ProtocolErrorKind::UnknownUnionVariant => {")
					g.line("},")
					g.line("Err(e) => return Err(e),")
					g.down()
					g.line("}")
				} else {
					g.line("let val = " + readCall + "?;")
					g.line(structFieldReadTempVariable(tfield) + " = Some(" + valExpr + ");")
				}
			} else {
				g.renderTypeSyncRead("val", tfield.Type(), false)
				g.line(structFieldReadTempVariable(tfield) + " = Some(val);")
			}
			g.down()
			g.line("},")
		}

		// default case (skip fields)
		g.line("_ => {")
		g.up()
		g.line("i_prot.skip(field_ident.field_type)?;")
		g.down()
		g.line("},")

		g.down()
		g.line("};") // finish match
	}

	g.line("i_prot.read_field_end()?;")
	g.down()
	g.line("}")                          // finish loop
	g.line("i_prot.read_struct_end()?;") // read message footer from the wire

	// verify that all required fields exist
	for _, tfield := range members {
		req := actualFieldReq(tfield, st)
		if !isOptional(req) {
			g.line("verify_required_field_exists(" + "\"" + structName + "." + rustFieldName(tfield) + "\"" + ", " + "&" + structFieldReadTempVariable(tfield) + ")?;")
		}
	}

	// construct the struct
	if len(members) == 0 {
		g.line("let ret = " + structName + " {};")
	} else {
		g.line("let ret = " + structName + " {")
		g.up()

		for _, tfield := range members {
			req := actualFieldReq(tfield, st)
			fieldName := rustFieldName(tfield)
			fieldKey := structFieldReadTempVariable(tfield)
			if isOptional(req) {
				g.line(fieldName + ": " + fieldKey + ",")
			} else {
				g.line(fieldName + ": " + fieldKey + ".expect(\"auto-generated code should have checked for presence of required fields\")" + ",")
			}
		}

		g.down()
		g.line("};")
	}

	// return the constructed value
	g.line("Ok(ret)")

	g.down()
	g.line("}")
}

// renderUnionSyncRead is t_rs_generator::render_union_sync_read.
func (g *generator) renderUnionSyncRead(unionName string, s *sema.Struct) {
	g.line("fn read_from_in_protocol(i_prot: &mut dyn TInputProtocol) -> thrift::Result<" + unionName + "> {")
	g.up()

	// create temporary variables to hold the
	// completed union as well as a count of fields read
	g.line("let mut ret: Option<" + unionName + "> = None;")
	g.line("let mut received_field_count = 0;")
	g.line("let mut total_field_count = 0;")

	// read the struct preamble
	g.line("i_prot.read_struct_begin()?;")

	// now loop through the fields we've received
	g.line("loop {") // start loop
	g.up()

	// break out if you've found the Stop field
	g.line("let field_ident = i_prot.read_field_begin()?;")
	g.line("if field_ident.field_type == TType::Stop {")
	g.up()
	g.line("break;")
	g.down()
	g.line("}")

	// now read all the fields found
	g.line("let field_id = field_id(&field_ident)?;")
	g.line("match field_id {") // start match
	g.up()

	members := s.SortedMembers()
	for _, member := range members {
		memberResolved := sema.TrueType(member.Type())
		memberIsUnion := memberResolved.IsStruct() && memberResolved.(*sema.Struct).IsUnion()
		g.line(rustSafeFieldID(member.Key()) + " => {")
		g.up()
		if memberIsUnion {
			// Use the resolved (non-Box) type since Box<T>::read_from_in_protocol()
			// isn't valid syntax; a recursive union variant is stored as Box<T>, so the
			// value read via the resolved type is re-boxed here. Mirrors the struct
			// reader (see renderStructSyncRead).
			memberType := g.toRustType(memberResolved)
			isBoxed := isBoxedByForwardTypedef(member.Type())
			valExpr := "val"
			if isBoxed {
				valExpr = "Box::new(val)"
			}
			memberRead := memberType + "::read_from_in_protocol(i_prot)"
			g.line("match " + memberRead + " {")
			g.up()
			g.line("Ok(val) => {")
			g.up()
			g.line("if ret.is_none() {")
			g.up()
			g.line("ret = Some(" + unionName + "::" + rustUnionFieldName(member) + "(" + valExpr + "));")
			g.down()
			g.line("}")
			g.line("received_field_count += 1;")
			g.down()
			g.line("},")
			g.line("Err(thrift::Error::Protocol(ref e)) if e.kind == ProtocolErrorKind::UnknownUnionVariant => {},")
			g.line("Err(e) => return Err(e),")
			g.down()
			g.line("}")
		} else {
			g.renderTypeSyncRead("val", member.Type(), false)
			g.line("if ret.is_none() {")
			g.up()
			g.line("ret = Some(" + unionName + "::" + rustUnionFieldName(member) + "(val));")
			g.down()
			g.line("}")
			g.line("received_field_count += 1;")
		}
		g.down()
		g.line("},")
	}

	// default case (skip unknown fields without affecting the count)
	g.line("_ => {")
	g.up()
	g.line("i_prot.skip(field_ident.field_type)?;")
	g.down()
	g.line("},")

	g.down()
	g.line("};") // finish match
	g.line("total_field_count += 1;")
	g.line("i_prot.read_field_end()?;")
	g.down()
	g.line("}")                          // finish loop
	g.line("i_prot.read_struct_end()?;") // finish reading message from wire

	// return the value or an error
	g.line("if total_field_count == 0 {")
	g.up()
	g.renderThriftError("Protocol", "ProtocolError", "ProtocolErrorKind::EmptyUnion", "\"received empty union from remote "+unionName+"\"")
	g.down()
	g.line("} else if received_field_count > 1 {")
	g.up()
	g.renderThriftError("Protocol", "ProtocolError", "ProtocolErrorKind::InvalidData", "\"received multiple fields for union from remote "+unionName+"\"")
	g.down()
	g.line("} else if received_field_count == 0 {")
	g.up()
	g.renderThriftError("Protocol", "ProtocolError", "ProtocolErrorKind::UnknownUnionVariant", "\"received union with unknown variant from remote "+unionName+"\"")
	g.down()
	g.line("} else if let Some(ret) = ret {")
	g.up()
	g.line("Ok(ret)")
	g.down()
	g.line("} else {")
	g.up()
	g.line("Err(")
	g.up()
	g.line("thrift::Error::Protocol(")
	g.line("  ProtocolError::new(ProtocolErrorKind::InvalidData, \"return value should have been constructed\")")
	g.line(")")
	g.down()
	g.line(")")
	g.down()
	g.line("}")

	g.down()
	g.line("}")
}

// renderTypeSyncRead is t_rs_generator::render_type_sync_read: construct
// the rust representation of all supported types from the wire.
func (g *generator) renderTypeSyncRead(typeVar string, t sema.Type, isBoxed bool) {
	switch {
	case t.IsBaseType():
		b := t.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeVoid:
			throwf("cannot read field of type TYPE_VOID from input protocol")
		case sema.TypeString:
			if b.IsBinary() {
				g.line("let " + typeVar + " = i_prot.read_bytes()?;")
			} else {
				g.line("let " + typeVar + " = i_prot.read_string()?;")
			}
			return
		case sema.TypeUUID:
			g.line("let " + typeVar + " = i_prot.read_uuid()?;")
			return
		case sema.TypeBool:
			g.line("let " + typeVar + " = i_prot.read_bool()?;")
			return
		case sema.TypeI8:
			g.line("let " + typeVar + " = i_prot.read_i8()?;")
			return
		case sema.TypeI16:
			g.line("let " + typeVar + " = i_prot.read_i16()?;")
			return
		case sema.TypeI32:
			g.line("let " + typeVar + " = i_prot.read_i32()?;")
			return
		case sema.TypeI64:
			g.line("let " + typeVar + " = i_prot.read_i64()?;")
			return
		case sema.TypeDouble:
			g.line("let " + typeVar + " = OrderedFloat::from(i_prot.read_double()?);")
			return
		default:
			throwf("compiler error: unhandled type")
		}
	case t.IsTypedef():
		// FIXME: not a fan of separate `is_boxed` parameter
		// This is problematic because it's an optional parameter, and only comes
		// into play once. The core issue is that I lose an important piece of type
		// information (whether the type is a fwd ref) by unwrapping the typedef'd
		// type and making the recursive call using it. I can't modify or wrap the
		// generated string after the fact because it's written directly into the file,
		// so I have to pass this parameter along. Going with this approach because it
		// seems like the lowest-cost option to easily support recursive types.
		td := t.(*sema.Typedef)
		g.renderTypeSyncRead(typeVar, td.Type(), td.IsForwardTypedef())
		return
	case t.IsEnum(), t.IsStruct(), t.IsXception():
		readCall := g.toRustType(t) + "::read_from_in_protocol(i_prot)?"
		if isBoxed {
			readCall = "Box::new(" + readCall + ")"
		}
		g.line("let " + typeVar + " = " + readCall + ";")
		return
	case t.IsMap():
		g.renderMapSyncRead(t.(*sema.Map), typeVar)
		return
	case t.IsSet():
		g.renderSetSyncRead(t.(*sema.Set), typeVar)
		return
	case t.IsList():
		g.renderListSyncRead(t.(*sema.List), typeVar)
		return
	}

	throwf("cannot read unsupported type %s", t.Name())
}

// renderListSyncRead is t_rs_generator::render_list_sync_read: construct
// the rust representation of a list from the wire.
func (g *generator) renderListSyncRead(tlist *sema.List, listVar string) {
	elemType := tlist.ElemType()

	g.line("let list_ident = i_prot.read_list_begin()?;")
	g.line("let mut " + listVar + ": " + g.toRustType(sema.Type(tlist)) + " = Vec::with_capacity(thrift::protocol::prealloc_size(list_ident.size));")
	g.line("for _ in 0..list_ident.size {")

	g.up()

	listElemVar := g.tmp("list_elem_")
	resolvedElem := sema.TrueType(elemType)
	elemIsUnion := resolvedElem.IsStruct() && resolvedElem.(*sema.Struct).IsUnion()
	if elemIsUnion {
		resolvedType := g.toRustType(resolvedElem)
		readCall := resolvedType + "::read_from_in_protocol(i_prot)"
		g.line("match " + readCall + " {")
		g.up()
		g.line("Ok(elem) => { " + listVar + ".push(Box::new(elem)); },")
		g.line("Err(thrift::Error::Protocol(ref e)) if e.kind == ProtocolErrorKind::UnknownUnionVariant => { continue; },")
		g.line("Err(e) => return Err(e),")
		g.down()
		g.line("}")
	} else {
		g.renderTypeSyncRead(listElemVar, elemType, false)
		g.line(listVar + ".push(" + listElemVar + ");")
	}

	g.down()

	g.line("}")
	g.line("i_prot.read_list_end()?;")
}

// renderSetSyncRead is t_rs_generator::render_set_sync_read: construct
// the rust representation of a set from the wire.
func (g *generator) renderSetSyncRead(tset *sema.Set, setVar string) {
	elemType := tset.ElemType()

	g.line("let set_ident = i_prot.read_set_begin()?;")
	g.line("let mut " + setVar + ": " + g.toRustType(sema.Type(tset)) + " = BTreeSet::new();")
	g.line("for _ in 0..set_ident.size {")

	g.up()

	setElemVar := g.tmp("set_elem_")
	resolvedElem := sema.TrueType(elemType)
	elemIsUnion := resolvedElem.IsStruct() && resolvedElem.(*sema.Struct).IsUnion()
	if elemIsUnion {
		resolvedType := g.toRustType(resolvedElem)
		readCall := resolvedType + "::read_from_in_protocol(i_prot)"
		g.line("match " + readCall + " {")
		g.up()
		g.line("Ok(elem) => { " + setVar + ".insert(Box::new(elem)); },")
		g.line("Err(thrift::Error::Protocol(ref e)) if e.kind == ProtocolErrorKind::UnknownUnionVariant => { continue; },")
		g.line("Err(e) => return Err(e),")
		g.down()
		g.line("}")
	} else {
		g.renderTypeSyncRead(setElemVar, elemType, false)
		g.line(setVar + ".insert(" + setElemVar + ");")
	}

	g.down()

	g.line("}")
	g.line("i_prot.read_set_end()?;")
}

// renderMapSyncRead is t_rs_generator::render_map_sync_read: construct
// the rust representation of a map from the wire.
func (g *generator) renderMapSyncRead(tmap *sema.Map, mapVar string) {
	keyType, valType := tmap.KeyType(), tmap.ValType()

	g.line("let map_ident = i_prot.read_map_begin()?;")
	g.line("let mut " + mapVar + ": " + g.toRustType(sema.Type(tmap)) + " = BTreeMap::new();")
	g.line("for _ in 0..map_ident.size {")

	g.up()

	resolvedKey := sema.TrueType(keyType)
	resolvedVal := sema.TrueType(valType)
	keyIsUnion := resolvedKey.IsStruct() && resolvedKey.(*sema.Struct).IsUnion()
	valIsUnion := resolvedVal.IsStruct() && resolvedVal.(*sema.Struct).IsUnion()
	if keyIsUnion || valIsUnion {
		// Read key
		keyElemVar := g.tmp("map_key_")
		if keyIsUnion {
			keyRead := g.toRustType(resolvedKey) + "::read_from_in_protocol(i_prot)"
			g.line("let " + keyElemVar + " = match " + keyRead + " {")
			g.up()
			g.line("Ok(val) => val,")
			g.line("Err(thrift::Error::Protocol(ref e)) if e.kind == ProtocolErrorKind::UnknownUnionVariant => {")
			g.up()
			// Skip the value and continue to next entry
			g.renderTypeSyncRead(g.tmp("discard_"), valType, false)
			g.line("continue;")
			g.down()
			g.line("},")
			g.line("Err(e) => return Err(e),")
			g.down()
			g.line("};")
		} else {
			g.renderTypeSyncRead(keyElemVar, keyType, false)
		}
		// Read value
		valElemVar := g.tmp("map_val_")
		if valIsUnion {
			valRead := g.toRustType(resolvedVal) + "::read_from_in_protocol(i_prot)"
			g.line("match " + valRead + " {")
			g.up()
			g.line("Ok(val) => { " + mapVar + ".insert(" + keyElemVar + ", val); },")
			g.line("Err(thrift::Error::Protocol(ref e)) if e.kind == ProtocolErrorKind::UnknownUnionVariant => { continue; },")
			g.line("Err(e) => return Err(e),")
			g.down()
			g.line("}")
		} else {
			g.renderTypeSyncRead(valElemVar, valType, false)
			g.line(mapVar + ".insert(" + keyElemVar + ", " + valElemVar + ");")
		}
	} else {
		keyElemVar := g.tmp("map_key_")
		g.renderTypeSyncRead(keyElemVar, keyType, false)
		valElemVar := g.tmp("map_val_")
		g.renderTypeSyncRead(valElemVar, valType, false)
		g.line(mapVar + ".insert(" + keyElemVar + ", " + valElemVar + ");")
	}

	g.down()

	g.line("}")
	g.line("i_prot.read_map_end()?;")
}

// structFieldReadTempVariable is
// t_rs_generator::struct_field_read_temp_variable.
func structFieldReadTempVariable(tfield *sema.Field) string {
	return "f_" + rustSafeFieldID(tfield.Key())
}
