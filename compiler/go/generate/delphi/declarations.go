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
	"github.com/apache/thrift/compiler/go/sema"
)

// generateForwardDeclaration is t_delphi_generator::generate_forward_declaration.
func (g *Generator) generateForwardDeclaration(tstruct *sema.Struct) {
	g.hasForward = true
	g.indentUp()
	g.ln(&g.sForwardDecr, g.typeName(tstruct, false, true)+" = interface;")
	if tstruct.IsXception() {
		g.ln(&g.sForwardDecr, g.typeName(tstruct, true, true)+" = class;")
	}
	g.indentDown()
	g.addDefinedType(tstruct)
}

// generateTypedef is t_delphi_generator::generate_typedef.
func (g *Generator) generateTypedef(ttypedef *sema.Typedef) {
	typ := ttypedef.Type()

	if !g.isFullyDefinedType(typ) {
		g.typedefsPending = append(g.typedefsPending, ttypedef)
		return
	}

	g.indentUp()
	g.generateDelphiDoc(&g.sStruct, ttypedef)
	g.wr(&g.sStruct, g.typeName(ttypedef, false, false)+" = ")
	g.raw(&g.sStruct, g.typeName(ttypedef.Type(), false, false)+";\n\n")
	g.indentDown()

	g.addDefinedType(ttypedef)
}

// generateEnum is t_delphi_generator::generate_enum.
func (g *Generator) generateEnum(tenum *sema.Enum) {
	g.hasEnum = true
	g.indentUp()
	g.generateDelphiDoc(&g.sEnum, tenum)
	g.wr(&g.sEnum, g.typeName(tenum, true, true)+" = "+"(\n")
	g.indentUp()
	constants := tenum.Constants()
	if len(constants) == 0 {
		g.wr(&g.sEnum, "dummy = 0  // empty enums are not allowed")
	} else {
		for i, c := range constants {
			if i != 0 {
				g.raw(&g.sEnum, ",\n")
			}
			g.generateDelphiDoc(&g.sEnum, c)
			g.wr(&g.sEnum, g.normalizeNameSimple(c.Name())+" = "+itoa32(c.Value()))
			g.raw(&g.sEnum, renderDeprecationAttribute(c.Annotations(), " {", "}"))
		}
	}
	g.raw(&g.sEnum, "\n")
	g.indentDown()
	g.wr(&g.sEnum, ")"+renderDeprecationAttribute(tenum.Annotations(), " ", "")+";\n\n")
	g.indentDown()

	g.addDefinedType(tenum)
}

// generateStruct is t_delphi_generator::generate_struct.
func (g *Generator) generateStruct(tstruct *sema.Struct) {
	g.generateDelphiStruct(tstruct)
}

// generateXception is t_delphi_generator::generate_xception.
func (g *Generator) generateXception(tstruct *sema.Struct) {
	g.generateDelphiException(tstruct)
}

// generateDelphiStruct is t_delphi_generator::generate_delphi_struct.
func (g *Generator) generateDelphiStruct(tstruct *sema.Struct) {
	g.indentUp()
	g.generateDelphiStructDefinition(&g.sStruct, tstruct)
	g.indentDown()

	g.addDefinedType(tstruct)

	g.generateDelphiStructImpl(&g.sStructImpl, "", tstruct, false, false)
	if g.opts.RegisterTypes {
		g.generateDelphiStructTypeFactory(&g.sTypeFactoryFuncs, tstruct)
		g.generateDelphiStructTypeFactoryRegistration(&g.sTypeFactoryRegistration, tstruct)
	}
}

// generateDelphiException is t_delphi_generator::generate_delphi_exception.
func (g *Generator) generateDelphiException(tstruct *sema.Struct) {
	// generate exception data class first
	g.generateDelphiStruct(tstruct)

	g.indentUp()
	g.generateDelphiExceptionDefinition(&g.sStruct, tstruct)
	g.indentDown()

	g.addDefinedType(tstruct)

	g.generateDelphiExceptionImpl(&g.sStructImpl, "", tstruct)
}
