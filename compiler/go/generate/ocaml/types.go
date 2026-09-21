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

package ocaml

import (
	"strconv"

	"github.com/apache/thrift/compiler/go/sema"
)

// generateTypedef is t_ocaml_generator::generate_typedef.
func (g *Generator) generateTypedef(t *sema.Typedef) {
	line := g.indent() + "type " + decapitalize(t.Symbolic()) + " = " + g.renderOcamlType(t.Type()) + "\n" + "\n"
	g.fTypes.WriteString(line)
	g.fTypesI.WriteString(line)
}

// generateEnum is t_ocaml_generator::generate_enum.
func (g *Generator) generateEnum(e *sema.Enum) {
	name := capitalize(e.Name())
	g.fTypes.WriteString(g.indent() + "module " + name + " = " + "\n" + "struct" + "\n")
	g.fTypesI.WriteString(g.indent() + "module " + name + " : " + "\n" + "sig" + "\n")
	g.indentUp()
	g.fTypes.WriteString(g.indent() + "type t = " + "\n")
	g.fTypesI.WriteString(g.indent() + "type t = " + "\n")
	g.indentUp()
	constants := e.Constants()
	for _, c := range constants {
		cname := capitalize(c.Name())
		g.fTypes.WriteString(g.indent() + "| " + cname + "\n")
		g.fTypesI.WriteString(g.indent() + "| " + cname + "\n")
	}
	g.indentDown()

	g.fTypes.WriteString(g.indent() + "let to_i = function" + "\n")
	g.fTypesI.WriteString(g.indent() + "val to_i : t -> Int32.t" + "\n")
	g.indentUp()
	for _, c := range constants {
		cname := capitalize(c.Name())
		g.fTypes.WriteString(g.indent() + "| " + cname + " -> " + strconv.Itoa(int(c.Value())) + "l" + "\n")
	}
	g.indentDown()

	g.fTypes.WriteString(g.indent() + "let of_i = function" + "\n")
	g.fTypesI.WriteString(g.indent() + "val of_i : Int32.t -> t" + "\n")
	g.indentUp()
	for _, c := range constants {
		cname := capitalize(c.Name())
		g.fTypes.WriteString(g.indent() + "| " + strconv.Itoa(int(c.Value())) + "l -> " + cname + "\n")
	}
	g.fTypes.WriteString(g.indent() + "| _ -> raise Thrift_error" + "\n")
	g.indentDown()
	g.indentDown()
	g.fTypes.WriteString(g.indent() + "end" + "\n")
	g.fTypesI.WriteString(g.indent() + "end" + "\n")
}
