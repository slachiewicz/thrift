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

package rb

import (
	"strconv"

	"github.com/apache/thrift/compiler/go/sema"
)

// generateStruct generates a ruby struct.
func (g *Generator) generateStruct(tstruct *sema.Struct) {
	if tstruct.IsUnion() {
		g.generateRbUnion(g.fTypes, tstruct, false)
	} else {
		g.generateRbStruct(g.fTypes, tstruct, false)
	}
}

// generateForwardDeclaration generates the "forward declaration" for a
// ruby struct: simply a declaration of the class with proper inheritance.
// The rest of the struct is still generated in generateStruct as has
// always been the case. These declarations allow thrift to generate valid
// ruby in cases where thrift structs rely on recursive definitions.
func (g *Generator) generateForwardDeclaration(tstruct *sema.Struct) {
	g.generateRbStructDeclaration(g.fTypes, tstruct, tstruct.IsXception())
}

func (g *Generator) generateRbStructDeclaration(out *ofstream, tstruct *sema.Struct, isException bool) {
	g.maybeSeparateTopLevel(out)
	out.indent()
	out.write("class " + typeName(tstruct))
	if tstruct.IsUnion() {
		out.write(" < ::Thrift::Union")
	}
	if isException {
		out.write(" < ::Thrift::Exception")
	}
	out.write("; end\n")
	g.markTopLevelWritten(out)
}

// generateXception generates a struct definition for a thrift exception.
// Basically the same as a struct but extends the Exception class.
func (g *Generator) generateXception(txception *sema.Struct) {
	g.generateRbStruct(g.fTypes, txception, true)
}

// generateRbStruct generates a ruby struct.
func (g *Generator) generateRbStruct(out *ofstream, tstruct *sema.Struct, isException bool) {
	g.maybeSeparateTopLevel(out)
	g.generateRdoc(out, tstruct)
	out.indent()
	out.write("class " + typeName(tstruct))
	if isException {
		out.write(" < ::Thrift::Exception")
	}
	out.write("\n")

	out.indentUp()
	out.indent()
	out.write("include ::Thrift::Struct, ::Thrift::Struct_Union\n\n")

	if isException {
		g.generateRbSimpleExceptionConstructor(out, tstruct)
	}

	g.generateFieldConstants(out, tstruct)
	g.generateFieldDefns(out, tstruct)
	g.generateRbStructRequiredValidator(out, tstruct)

	out.indent()
	out.write("::Thrift::Struct.generate_accessors self\n")

	out.indentDown()
	out.indent()
	out.write("end\n")
	g.markTopLevelWritten(out)
}

// generateRbUnion generates a ruby union.
func (g *Generator) generateRbUnion(out *ofstream, tstruct *sema.Struct, isException bool) {
	_ = isException
	g.maybeSeparateTopLevel(out)
	g.generateRdoc(out, tstruct)
	out.indent()
	out.write("class " + typeName(tstruct) + " < ::Thrift::Union\n")

	out.indentUp()
	out.indent()
	out.write("include ::Thrift::Struct_Union\n\n")

	g.generateFieldConstructors(out, tstruct)

	g.generateFieldConstants(out, tstruct)
	g.generateFieldDefns(out, tstruct)
	g.generateRbUnionValidator(out, tstruct)

	out.indent()
	out.write("::Thrift::Union.generate_accessors self\n")

	out.indentDown()
	out.indent()
	out.write("end\n")
	g.markTopLevelWritten(out)
}

func (g *Generator) generateFieldConstructors(out *ofstream, tstruct *sema.Struct) {
	out.indent()
	out.write("class << self\n")
	out.indentUp()

	fields := tstruct.Members()
	for i, f := range fields {
		if i != 0 {
			out.write("\n")
		}
		fieldName := f.Name()

		out.indent()
		out.write("def " + fieldName + "(val)\n")
		out.indent()
		out.write("  " + tstruct.Name() + ".new(:" + fieldName + ", val)\n")
		out.indent()
		out.write("end\n")
	}

	out.indentDown()
	out.indent()
	out.write("end\n")

	out.write("\n")
}

func (g *Generator) generateRbSimpleExceptionConstructor(out *ofstream, tstruct *sema.Struct) {
	members := tstruct.Members()

	if len(members) == 1 {
		m := members[0]

		if m.Type().IsString() {
			name := m.Name()

			out.indent()
			out.write("def initialize(message = nil)\n")
			out.indentUp()
			out.indent()
			out.write("super()\n")
			out.indent()
			out.write("self." + name + " = message\n")
			out.indentDown()
			out.indent()
			out.write("end\n\n")

			if name != "message" {
				out.indent()
				out.write("def message; " + name + " end\n\n")
			}
		}
	}
}

func (g *Generator) generateFieldConstants(out *ofstream, tstruct *sema.Struct) {
	fields := tstruct.Members()

	for _, f := range fields {
		fieldName := f.Name()
		out.indent()
		out.write(fieldIDConstantName(fieldName) + " = " + strconv.FormatInt(int64(f.Key()), 10) + "\n")
	}
	if len(fields) != 0 {
		out.write("\n")
	}
}

func (g *Generator) generateFieldDefns(out *ofstream, tstruct *sema.Struct) {
	fields := tstruct.Members()

	out.indent()
	out.write("FIELDS = {\n")
	out.indentUp()
	for _, f := range fields {
		// Generate the field docstrings within the FIELDS constant. No
		// real better place...
		g.generateRdoc(out, f)

		out.indent()
		out.write(fieldIDConstantName(f.Name()) + " => ")

		generateFieldData(out, f.Type(), f.Name(), f.Value(), f.Req() == sema.Optional)
		out.write(",\n")
	}
	out.indentDown()
	out.indent()
	out.write("}\n\n")

	out.indent()
	out.write("def struct_fields; FIELDS; end\n\n")
}

func (g *Generator) generateRbStructRequiredValidator(out *ofstream, tstruct *sema.Struct) {
	out.indent()
	out.write("def validate\n")
	out.indentUp()

	fields := tstruct.Members()

	for _, f := range fields {
		if f.Req() == sema.Required {
			out.indent()
			out.write("raise ::Thrift::ProtocolException.new(::Thrift::ProtocolException::INVALID_DATA, " +
				"\"Required field " + f.Name() + " is unset!\")")
			if f.Type().IsBool() {
				out.write(" if @" + f.Name() + ".nil?")
			} else {
				out.write(" unless @" + f.Name())
			}
			out.write("\n")
		}
	}

	// If field is an enum, check that its value is valid.
	for _, f := range fields {
		if f.Type().IsEnum() {
			out.indent()
			out.write("unless @" + f.Name() + ".nil? || " + fullTypeName(f.Type()) +
				"::VALID_VALUES.include?(@" + f.Name() + ")\n")
			out.indentUp()
			out.indent()
			out.write("raise ::Thrift::ProtocolException.new(::Thrift::ProtocolException::INVALID_DATA, " +
				"\"Invalid value of field " + f.Name() + "!\")\n")
			out.indentDown()
			out.indent()
			out.write("end\n")
		}
	}

	out.indentDown()
	out.indent()
	out.write("end\n\n")
}

func (g *Generator) generateRbUnionValidator(out *ofstream, tstruct *sema.Struct) {
	out.indent()
	out.write("def validate\n")
	out.indentUp()

	fields := tstruct.Members()

	out.indent()
	out.write("raise ::Thrift::ProtocolException.new(::Thrift::ProtocolException::INVALID_DATA, " +
		"\"Union fields are not set.\") if get_set_field.nil? || get_value.nil?\n")

	// If field is an enum, check that its value is valid.
	for _, f := range fields {
		if f.Type().IsEnum() {
			out.indent()
			out.write("if get_set_field == :" + f.Name() + "\n")
			out.indent()
			out.write("  raise ::Thrift::ProtocolException.new(::Thrift::ProtocolException::INVALID_DATA, " +
				"\"Invalid value of field " + f.Name() + "!\") unless " + fullTypeName(f.Type()) +
				"::VALID_VALUES.include?(get_value)\n")
			out.indent()
			out.write("end\n")
		}
	}

	out.indentDown()
	out.indent()
	out.write("end\n\n")
}
