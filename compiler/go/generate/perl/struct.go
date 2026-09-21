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

package perl

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

// generateStruct makes a struct.
func (g *Generator) generateStruct(tstruct *sema.Struct) {
	g.generatePerlStruct(tstruct, false)
}

// generateXception generates a struct definition for a thrift exception.
// Basically the same as a struct but extends the Exception class.
func (g *Generator) generateXception(txception *sema.Struct) {
	g.generatePerlStruct(txception, true)
}

// generatePerlStruct: structs can be normal or exceptions.
func (g *Generator) generatePerlStruct(tstruct *sema.Struct, isException bool) {
	g.generateUseIncludes(&g.fTypes, &g.fTypesUseIncludesEmitted, tstruct, false)
	g.generatePerlStructDefinition(&g.fTypes, tstruct, isException)
}

// generatePerlStructDefinition generates a struct definition for a thrift
// data type. This is nothing in PERL where the objects are all just
// associative arrays (unless of course we decide to start using objects
// for them...).
func (g *Generator) generatePerlStructDefinition(out *strings.Builder, tstruct *sema.Struct, isException bool) {
	members := tstruct.Members()

	out.WriteString("package " + perlNamespace(tstruct.Program()) + tstruct.Name() +
		";  ## no critic (RequireFilenameMatchesPackage)\n")
	if isException {
		out.WriteString("use base qw(Thrift::TException);\n")
	}

	// Create simple acessor methods.
	out.WriteString("use base qw(Class::Accessor);\n")

	if len(members) > 0 {
		out.WriteString(perlNamespace(tstruct.Program()) + tstruct.Name() + "->mk_accessors( qw( ")
		for _, m := range members {
			t := sema.TrueType(m.Type())
			if !t.IsXception() {
				out.WriteString(m.Name() + " ")
			}
		}
		out.WriteString(") );\n")
	}

	out.WriteString("\n")

	// new()
	g.indentUp()
	out.WriteString("sub new {\n" + g.indent() + "my $classname = shift;\n" + g.indent() +
		"my $self      = {};\n" + g.indent() + "my $vals      = shift || {};\n")

	for _, m := range members {
		dval := "undef"
		t := sema.TrueType(m.Type())
		if m.Value() != nil && !(t.IsStruct() || t.IsXception()) {
			dval = g.renderConstValue(m.Type(), m.Value())
		}
		out.WriteString(g.indent() + "$self->{" + m.Name() + "} = " + dval + ";\n")
	}

	// Generate constructor from array.
	if len(members) > 0 {
		for _, m := range members {
			t := sema.TrueType(m.Type())
			if m.Value() != nil && (t.IsStruct() || t.IsXception()) {
				out.WriteString(g.indent() + "$self->{" + m.Name() + "} = " + g.renderConstValue(t, m.Value()) + ";\n")
			}
		}

		out.WriteString(g.indent() + "if (UNIVERSAL::isa($vals,'HASH')) {\n")
		g.indentUp()
		for _, m := range members {
			out.WriteString(g.indent() + "if (defined $vals->{" + m.Name() + "}) {\n" +
				g.indent() + "  $self->{" + m.Name() + "} = $vals->{" + m.Name() + "};\n" +
				g.indent() + "}\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}

	out.WriteString(g.indent() + "return bless ($self, $classname);\n")
	g.indentDown()
	out.WriteString("}\n\n")

	out.WriteString("sub getName {\n" + g.indent() + "  return '" + tstruct.Name() + "';\n" +
		g.indent() + "}\n\n")

	g.generatePerlStructReader(out, tstruct)
	g.generatePerlStructWriter(out, tstruct)
}

// generatePerlStructReader generates the read() method for a struct.
func (g *Generator) generatePerlStructReader(out *strings.Builder, tstruct *sema.Struct) {
	fields := tstruct.Members()

	out.WriteString("sub read {\n")

	g.indentUp()

	out.WriteString(g.indent() + "my ($self, $input) = @_;\n" + g.indent() + "my $xfer  = 0;\n" +
		g.indent() + "my $fname;\n" + g.indent() + "my $ftype = 0;\n" + g.indent() +
		"my $fid   = 0;\n")

	out.WriteString(g.indent() + "$input->incrementRecursionDepth();\n")
	out.WriteString(g.indent() + "eval {\n")
	g.indentUp()
	out.WriteString(g.indent() + "$xfer += $input->readStructBegin(\\$fname);\n")

	// Loop over reading in fields.
	out.WriteString(g.indent() + "while (1)\n")

	g.scopeUp(out)

	out.WriteString(g.indent() + "$xfer += $input->readFieldBegin(\\$fname, \\$ftype, \\$fid);\n")

	// Check for field STOP marker and break.
	out.WriteString(g.indent() + "if ($ftype == Thrift::TType::STOP) {\n")
	g.indentUp()
	out.WriteString(g.indent() + "last;\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	// Switch statement on the field we are reading.
	out.WriteString(g.indent() + "SWITCH: for($fid)\n")

	g.scopeUp(out)

	// Generate deserialization code for known cases.
	for _, f := range fields {
		out.WriteString(g.indent() + "/^" + strconv.FormatInt(int64(f.Key()), 10) + "$/ && do{")
		out.WriteString(g.indent() + "if ($ftype == " + typeToEnum(f.Type()) + ") {\n")

		g.indentUp()
		g.generateDeserializeField(out, f, "self->", false)
		g.indentDown()

		out.WriteString(g.indent() + "} else {\n")

		out.WriteString(g.indent() + "  $xfer += $input->skip($ftype);\n")

		out.WriteString(g.indent() + "}\n" + g.indent() + "last; };\n")
	}
	// In the default case we skip the field.

	out.WriteString(g.indent() + "  $xfer += $input->skip($ftype);\n")

	g.scopeDown(out)

	out.WriteString(g.indent() + "$xfer += $input->readFieldEnd();\n")

	g.scopeDown(out)

	out.WriteString(g.indent() + "$xfer += $input->readStructEnd();\n")

	g.indentDown()
	out.WriteString(g.indent() + "};\n")
	out.WriteString(g.indent() + "my $err = $@;\n")
	out.WriteString(g.indent() + "$input->decrementRecursionDepth();\n")
	out.WriteString(g.indent() + "die $err if $err;\n")

	out.WriteString(g.indent() + "return $xfer;\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generatePerlStructWriter generates the write() method for a struct.
func (g *Generator) generatePerlStructWriter(out *strings.Builder, tstruct *sema.Struct) {
	name := tstruct.Name()
	fields := tstruct.SortedMembers()

	out.WriteString("sub write {\n")

	g.indentUp()
	out.WriteString(g.indent() + "my ($self, $output) = @_;\n")
	out.WriteString(g.indent() + "my $xfer   = 0;\n")

	out.WriteString(g.indent() + "$output->incrementRecursionDepth();\n")
	out.WriteString(g.indent() + "eval {\n")
	g.indentUp()
	out.WriteString(g.indent() + "$xfer += $output->writeStructBegin('" + name + "');\n")

	for _, f := range fields {
		out.WriteString(g.indent() + "if (defined $self->{" + f.Name() + "}) {\n")
		g.indentUp()

		out.WriteString(g.indent() + "$xfer += $output->writeFieldBegin(" +
			"'" + f.Name() + "', " + typeToEnum(f.Type()) + ", " + strconv.FormatInt(int64(f.Key()), 10) + ");\n")

		// Write field contents.
		g.generateSerializeField(out, f, "self->")

		out.WriteString(g.indent() + "$xfer += $output->writeFieldEnd();\n")

		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}

	out.WriteString(g.indent() + "$xfer += $output->writeFieldStop();\n" + g.indent() +
		"$xfer += $output->writeStructEnd();\n")

	g.indentDown()
	out.WriteString(g.indent() + "};\n")
	out.WriteString(g.indent() + "my $err = $@;\n")
	out.WriteString(g.indent() + "$output->decrementRecursionDepth();\n")
	out.WriteString(g.indent() + "die $err if $err;\n")

	out.WriteString(g.indent() + "return $xfer;\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateUseIncludes generates use clauses for included entities.
//
//   - out: the output stream
//   - done: a flag reference to debounce the action
//   - typ: the type being processed
//   - selfish: flag to indicate if the current namespace types should be
//     "use"d as well.
func (g *Generator) generateUseIncludes(out *strings.Builder, done *bool, typ sema.Type, selfish bool) {
	current := typ.Program()
	if current != nil && !*done {
		if selfish {
			out.WriteString("use " + perlNamespace(current) + "Types;\n")
		}
		for _, inc := range current.Includes() {
			out.WriteString("use " + perlNamespace(inc) + "Types;\n")
		}
		out.WriteString("\n")
		*done = true
	}
}
