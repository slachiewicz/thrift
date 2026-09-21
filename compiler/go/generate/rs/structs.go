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
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

//-----------------------------------------------------------------------------
//
// Structs, Unions and Exceptions
//
//-----------------------------------------------------------------------------

// generateXception is t_rs_generator::generate_xception.
func (g *generator) generateXception(s *sema.Struct) {
	g.renderStruct(rustStructName(s), s, structException)
}

// generateStruct is t_rs_generator::generate_struct.
func (g *generator) generateStruct(s *sema.Struct) {
	if s.IsUnion() {
		g.renderUnion(s)
	} else if s.IsStruct() {
		g.renderStruct(rustStructName(s), s, structRegular)
	} else {
		throwf("cannot generate struct for exception")
	}
}

// renderStruct is t_rs_generator::render_struct.
func (g *generator) renderStruct(structName string, s *sema.Struct, st structType) {
	g.renderTypeComment(structName)
	g.renderStructDefinition(structName, s, st)
	g.renderStructImpl(structName, s, st)
	if st == structException {
		g.renderExceptionStructErrorTraitImpls(structName, s)
	}
}

// renderStructDefinition is t_rs_generator::render_struct_definition.
func (g *generator) renderStructDefinition(structName string, s *sema.Struct, st structType) {
	g.renderRustdoc(s)

	members := s.SortedMembers()
	needDefault := st == structRegular || st == structException
	for _, member := range members {
		if !needDefault {
			break
		}
		if !isOptional(member.Req()) {
			needDefault = false
		}
	}

	derive := "#[derive(Clone, Debug"
	if needDefault {
		derive += ", Default"
	}
	derive += ", Eq, Hash, Ord, PartialEq, PartialOrd)]"
	g.wl(derive)
	g.wl(visibilityQualifier(st) + "struct " + structName + " {")

	// render the members
	if len(members) != 0 {
		g.up()

		for _, member := range members {
			memberReq := actualFieldReq(member, st)

			rustType := g.toRustType(member.Type())
			if isOptional(memberReq) {
				rustType = "Option<" + rustType + ">"
			}

			g.renderRustdoc(member)
			g.line(visibilityQualifier(st) + rustFieldName(member) + ": " + rustType + ",")
		}

		g.down()
	}

	g.wl("}")
	g.wl("")
}

// renderExceptionStructErrorTraitImpls is
// t_rs_generator::render_exception_struct_error_trait_impls.
func (g *generator) renderExceptionStructErrorTraitImpls(structName string, s *sema.Struct) {
	// error::Error trait
	g.wl("impl Error for " + structName + " {}")
	g.wl("")

	// convert::From trait
	g.wl("impl From<" + structName + "> for thrift::Error {")
	g.up()
	g.line("fn from(e: " + structName + ") -> Self {")
	g.up()
	g.line("thrift::Error::User(Box::new(e))")
	g.down()
	g.line("}")
	g.down()
	g.wl("}")
	g.wl("")

	// fmt::Display trait
	g.wl("impl Display for " + structName + " {")
	g.up()
	g.line("fn fmt(&self, f: &mut Formatter) -> fmt::Result {")
	g.up()
	// use *original* name
	g.line("write!(f, \"remote service threw " + s.Name() + "\")")
	g.down()
	g.line("}")
	g.down()
	g.wl("}")
	g.wl("")
}

// renderStructImpl is t_rs_generator::render_struct_impl.
func (g *generator) renderStructImpl(structName string, s *sema.Struct, st structType) {
	g.wl("impl " + structName + " {")
	g.up()

	if st == structRegular || st == structException {
		g.renderStructConstructor(structName, s, st)
	}

	if st == structResult {
		g.renderResultStructToResultMethod(s)
	}

	if st == structRegular || st == structException {
		g.down()
		g.wl("}")
		g.wl("")

		g.wl("impl TSerializable for " + structName + " {")
		g.up()
	}

	g.renderStructSyncRead(structName, s, st)
	g.renderStructSyncWrite(s, st)

	g.down()
	g.wl("}")
	g.wl("")
}

// renderStructConstructor is t_rs_generator::render_struct_constructor.
func (g *generator) renderStructConstructor(structName string, s *sema.Struct, st structType) {
	members := s.SortedMembers()

	// build the convenience type parameters that allows us to pass unwrapped values to a constructor
	// and have them automatically converted into Option<value>
	firstArg := true

	var genericTypeParameters strings.Builder
	var genericTypeQualifiers strings.Builder
	for _, member := range members {
		memberReq := actualFieldReq(member, st)

		if isOptional(memberReq) {
			if firstArg {
				firstArg = false
			} else {
				genericTypeParameters.WriteString(", ")
				genericTypeQualifiers.WriteString(", ")
			}
			genericTypeParameters.WriteString("F" + rustSafeFieldID(member.Key()))
			genericTypeQualifiers.WriteString("F" + rustSafeFieldID(member.Key()) + ": Into<Option<" + g.toRustType(member.Type()) + ">>")
		}
	}

	typeParameterString := genericTypeParameters.String()
	if len(typeParameterString) != 0 {
		typeParameterString = "<" + typeParameterString + ">"
	}

	typeQualifierString := genericTypeQualifiers.String()
	if len(typeQualifierString) != 0 {
		typeQualifierString = "where " + typeQualifierString + " "
	}

	// now build the actual constructor arg list
	// when we're building this list we have to use the type parameters in place of the actual type
	// names if necessary
	var args strings.Builder
	firstArg = true
	for _, member := range members {
		memberReq := actualFieldReq(member, st)
		memberName := rustFieldName(member)

		if firstArg {
			firstArg = false
		} else {
			args.WriteString(", ")
		}

		if isOptional(memberReq) {
			args.WriteString(memberName + ": " + "F" + rustSafeFieldID(member.Key()))
		} else {
			args.WriteString(memberName + ": " + g.toRustType(member.Type()))
		}
	}

	argString := args.String()

	visibility := visibilityQualifier(st)
	g.line(visibility + "fn new" + typeParameterString + "(" + argString + ") -> " + structName + " " + typeQualifierString + "{")
	g.up()

	if len(members) == 0 {
		g.line(structName + " {}")
	} else {
		g.line(structName + " {")
		g.up()

		for _, member := range members {
			memberReq := actualFieldReq(member, st)
			memberName := rustFieldName(member)

			if isOptional(memberReq) {
				g.line(memberName + ": " + memberName + ".into(),")
			} else {
				g.line(memberName + ",")
			}
		}

		g.down()
		g.line("}")
	}

	g.down()
	g.line("}")
}

// renderResultStructToResultMethod is
// t_rs_generator::render_result_struct_to_result_method.
func (g *generator) renderResultStructToResultMethod(s *sema.Struct) {
	// we don't use the rust struct name in this method, just the service call name
	serviceCallName := s.Name()

	// check that we actually have a result
	index := strings.Index(serviceCallName, resultStructSuffix)
	if index < 0 {
		throwf("result struct %s missing result suffix", serviceCallName)
	} else {
		serviceCallName = serviceCallName[:index] + serviceCallName[index+6:]
	}

	members := s.SortedMembers()

	// find out what the call's expected return type was
	rustReturnType := "()"
	for _, member := range members {
		if member.Name() == serviceResultVariable { // don't have to check safe name here
			rustReturnType = g.toRustType(member.Type())
			break
		}
	}

	// NOTE: ideally I would generate the branches and render them separately
	// I tried this however, and the resulting code was harder to understand
	// maintaining a rendered branch count (while a little ugly) got me the
	// rendering I wanted with code that was reasonably understandable

	g.line("fn ok_or(self) -> thrift::Result<" + rustReturnType + "> {")
	g.up()

	renderedBranchCount := 0

	// render the exception branches
	for _, tfield := range members {
		if tfield.Name() != serviceResultVariable { // don't have to check safe name here
			fieldName := "self." + rustFieldName(tfield)
			branchStatement := "if"
			if renderedBranchCount != 0 {
				branchStatement = "} else if"
			}

			g.line(branchStatement + " " + fieldName + ".is_some() {")
			g.up()
			g.line("Err(thrift::Error::User(Box::new(" + fieldName + ".unwrap())))")
			g.down()

			renderedBranchCount++
		}
	}

	// render the return value branches
	if rustReturnType == "()" {
		if renderedBranchCount == 0 {
			// we have the unit return and this service call has no user-defined
			// exceptions. this means that we've a trivial return (happens with oneways)
			g.line("Ok(())")
		} else {
			// we have the unit return, but there are user-defined exceptions
			// if we've gotten this far then we have the default return (i.e. call successful)
			g.line("} else {")
			g.up()
			g.line("Ok(())")
			g.down()
			g.line("}")
		}
	} else {
		branchStatement := "if"
		if renderedBranchCount != 0 {
			branchStatement = "} else if"
		}
		g.line(branchStatement + " self." + serviceResultVariable + ".is_some() {")
		g.up()
		g.line("Ok(self." + serviceResultVariable + ".unwrap())")
		g.down()
		g.line("} else {")
		g.up()
		// if we haven't found a valid return value *or* a user exception
		// then we're in trouble; return a default error
		g.renderThriftError("Application", "ApplicationError", "ApplicationErrorKind::MissingResult", "\"no result received for "+serviceCallName+"\"")
		g.down()
		g.line("}")
	}

	g.down()
	g.line("}")
}

// renderUnion is t_rs_generator::render_union.
func (g *generator) renderUnion(s *sema.Struct) {
	unionName := rustStructName(s)
	g.renderTypeComment(unionName)
	g.renderUnionDefinition(unionName, s)
	g.renderUnionImpl(unionName, s)
}

// renderUnionDefinition is t_rs_generator::render_union_definition.
func (g *generator) renderUnionDefinition(unionName string, s *sema.Struct) {
	members := s.SortedMembers()
	if len(members) == 0 {
		// may be valid thrift, but it's invalid rust
		throwf("cannot generate rust enum with 0 members")
	}

	g.wl("#[derive(Clone, Debug, Eq, Hash, Ord, PartialEq, PartialOrd)]")
	g.wl("pub enum " + unionName + " {")
	g.up()

	for _, tfield := range members {
		g.line(rustUnionFieldName(tfield) + "(" + g.toRustType(tfield.Type()) + "),")
	}

	g.down()
	g.wl("}")
	g.wl("")
}

// renderUnionImpl is t_rs_generator::render_union_impl.
func (g *generator) renderUnionImpl(unionName string, s *sema.Struct) {
	g.wl("impl TSerializable for " + unionName + " {")
	g.up()

	g.renderUnionSyncRead(unionName, s)
	g.renderUnionSyncWrite(unionName, s)

	g.down()
	g.wl("}")
	g.wl("")
}
