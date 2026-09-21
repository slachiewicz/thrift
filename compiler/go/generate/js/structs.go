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

package js

import (
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

// generateStruct is generate_struct.
func (g *Generator) generateStruct(s *sema.Struct) {
	g.generateJsStruct(s, false)
}

// generateXception is generate_xception: a thrift exception is basically a
// struct that extends the Exception class.
func (g *Generator) generateXception(x *sema.Struct) {
	g.generateJsStruct(x, true)
}

// generateJsStruct is generate_js_struct: structs can be normal or
// exceptions.
func (g *Generator) generateJsStruct(s *sema.Struct, isException bool) {
	g.generateJsStructDefinition(&g.fTypes, s, isException, true)
}

// generateJsStructDefinition is generate_js_struct_definition: it
// generates a struct definition for a thrift data type. This is nothing
// in JS, where the objects are all just associative arrays (unless of
// course we decide to start using objects for them...).
func (g *Generator) generateJsStructDefinition(out *strings.Builder, s *sema.Struct, isException, isExported bool) {
	members := s.Members()

	if g.opts.Node {
		commonjsExport := ""
		if isExported {
			if g.opts.ESM {
				out.WriteString("export ")
			} else {
				commonjsExport = " = module.exports." + s.Name()
			}
		}
		prefix := g.jsConstType
		if g.hasJsNamespace(s.Program()) {
			prefix = g.jsNamespace(s.Program())
		}
		out.WriteString(prefix + s.Name() + commonjsExport)
		if g.opts.TS {
			ext := ""
			if isException {
				ext = " extends Thrift.TException"
			}
			g.fTypesTS.WriteString(g.tsPrintDoc(s) + g.tsIndent() + g.tsDeclare() + "class " + s.Name() + ext + " {" + "\n")
		}
	} else {
		out.WriteString(g.jsNamespace(s.Program()) + s.Name())
		if g.opts.TS {
			ext := ""
			if isException {
				ext = " extends Thrift.TException"
			}
			g.fTypesTS.WriteString(g.tsPrintDoc(s) + g.tsIndent() + g.tsDeclare() + "class " + s.Name() + ext + " {" + "\n")
		}
	}

	if g.opts.ES6 {
		if g.opts.Node && isException {
			out.WriteString(" = class extends Thrift.TException {" + "\n")
		} else {
			out.WriteString(" = class {" + "\n")
		}
		g.indentUp()
		out.WriteString(g.indent() + "constructor(args) {" + "\n")
	} else {
		out.WriteString(" = function(args) {" + "\n")
	}

	g.indentUp()

	// Call super() method on inherited Error class
	if g.opts.Node && isException {
		if g.opts.ES6 {
			out.WriteString(g.indent() + "super(\"" + g.jsNamespace(s.Program()) + s.Name() + "\");" + "\n")
		} else {
			out.WriteString(g.indent() + "Thrift.TException.call(this, \"" + g.jsNamespace(s.Program()) + s.Name() + "\");" + "\n")
		}
		out.WriteString(g.indent() + "this.name = \"" + g.jsNamespace(s.Program()) + s.Name() + "\";" + "\n")
	}

	// members with arguments
	for _, m := range members {
		dval := g.declareField(m, false, true)
		t := sema.TrueType(m.Type())
		if m.Value() != nil && !(t.IsStruct() || t.IsXception()) {
			dval = g.renderConstValue(m.Type(), m.Value())
			out.WriteString(g.indent() + "this." + m.Name() + " = " + dval + ";" + "\n")
		} else {
			out.WriteString(g.indent() + dval + ";" + "\n")
		}
		if g.opts.TS {
			tsAccess := ""
			if g.opts.Node {
				tsAccess = "public "
			}
			memberName := m.Name()
			// Special case. Exceptions derive from Error, and error has a
			// non optional message field. Ignore the optional flag in
			// this case, otherwise we will generate an incompatible field
			// in the eyes of typescript.
			optionalFlag := tsGetReq(m)
			if isException && memberName == "message" {
				optionalFlag = ""
			}
			g.fTypesTS.WriteString(g.tsIndent() + tsAccess + memberName + optionalFlag + ": " + g.tsGetType(m.Type()) + ";" + "\n")
		}
	}

	// Generate constructor from array
	if len(members) > 0 {
		for _, m := range members {
			t := sema.TrueType(m.Type())
			if m.Value() != nil && (t.IsStruct() || t.IsXception()) {
				out.WriteString(g.indent() + "this." + m.Name() + " = " + g.renderConstValue(t, m.Value()) + ";" + "\n")
			}
		}

		// Early returns for exceptions
		for _, m := range members {
			t := sema.TrueType(m.Type())
			if t.IsXception() {
				out.WriteString(g.indent() + "if (args instanceof " + g.jsTypeNamespace(t.Program()) + t.Name() + ") {" + "\n" +
					g.indent() + g.indent() + "this." + m.Name() + " = args;" + "\n" +
					g.indent() + g.indent() + "return;" + "\n" +
					g.indent() + "}" + "\n")
			}
		}

		out.WriteString(g.indent() + "if (args) {" + "\n")
		g.indentUp()
		if g.opts.TS {
			g.fTypesTS.WriteString("\n" + g.tsIndent() + "constructor(args?: { ")
		}

		for _, m := range members {
			t := sema.TrueType(m.Type())
			name := m.Name()
			out.WriteString(g.indent() + "if (args." + name + " !== undefined && args." + name + " !== null) {" + "\n")
			g.indentUp()
			out.WriteString(g.indent() + "this." + name)

			switch {
			case t.IsStruct():
				out.WriteString(" = new " + g.jsTypeNamespace(t.Program()) + t.Name() + "(args." + name + ");")
				out.WriteString("\n")
			case t.IsContainer():
				etype := getContainedType(t)
				copyFunc := "Thrift.copyList"
				if t.IsMap() {
					copyFunc = "Thrift.copyMap"
				}
				typeList := ""
				for etype.IsContainer() {
					if len(typeList) > 0 {
						typeList += ", "
					}
					if etype.IsMap() {
						typeList += "Thrift.copyMap"
					} else {
						typeList += "Thrift.copyList"
					}
					etype = getContainedType(etype)
				}
				if etype.IsStruct() {
					if len(typeList) > 0 {
						typeList += ", "
					}
					typeList += g.jsTypeNamespace(etype.Program()) + etype.Name()
				} else {
					if len(typeList) > 0 {
						typeList += ", "
					}
					typeList += "null"
				}
				out.WriteString(" = " + copyFunc + "(args." + name + ", [" + typeList + "]);")
				out.WriteString("\n")
			default:
				out.WriteString(" = args." + name + ";" + "\n")
			}

			g.indentDown()
			if m.Req() == sema.Required {
				out.WriteString(g.indent() + "} else {" + "\n")
				out.WriteString(g.indent() + "  throw new Thrift.TProtocolException(Thrift.TProtocolExceptionType.UNKNOWN, 'Required field " + name + " is unset!');" + "\n")
			}
			out.WriteString(g.indent() + "}" + "\n")
			if g.opts.TS {
				g.fTypesTS.WriteString(name + tsGetReq(m) + ": " + g.tsGetType(m.Type()) + "; ")
			}
		}
		g.indentDown()
		out.WriteString(g.indent() + "}" + "\n")
		if g.opts.TS {
			g.fTypesTS.WriteString("});" + "\n")
		}
	}

	// Done with constructor
	g.indentDown()
	if g.opts.ES6 {
		out.WriteString(g.indent() + "}" + "\n" + "\n")
	} else {
		out.WriteString(g.indent() + "};" + "\n")
	}

	if g.opts.TS {
		g.fTypesTS.WriteString(g.tsIndent() + "}" + "\n")
	}

	if !g.opts.ES6 {
		if isException {
			out.WriteString("Thrift.inherits(" + g.jsNamespace(s.Program()) + s.Name() + ", Thrift.TException);" + "\n")
			out.WriteString(g.jsNamespace(s.Program()) + s.Name() + ".prototype.name = '" + s.Name() + "';" + "\n")
		} else {
			// init prototype manually if we aren't using es6
			out.WriteString(g.jsNamespace(s.Program()) + s.Name() + ".prototype = {};" + "\n")
		}
	}

	g.generateJsStructReader(out, s)
	g.generateJsStructWriter(out, s)

	// Close out the class definition
	if g.opts.ES6 {
		g.indentDown()
		out.WriteString(g.indent() + "};" + "\n")
	}
}

// generateJsStructReader is generate_js_struct_reader: the read() method
// for a struct.
func (g *Generator) generateJsStructReader(out *strings.Builder, s *sema.Struct) {
	fields := s.Members()

	if g.opts.ES6 {
		out.WriteString(g.indent() + "[Symbol.for(\"read\")] (input) {" + "\n")
	} else {
		out.WriteString(g.indent() + g.jsNamespace(s.Program()) + s.Name() + ".prototype[Symbol.for(\"read\")] = function(input) {" + "\n")
	}

	g.indentUp()

	out.WriteString(g.indent() + "input.incrementRecursionDepth();" + "\n")
	out.WriteString(g.indent() + "try {" + "\n")
	g.indentUp()

	out.WriteString(g.indent() + "input.readStructBegin();" + "\n")

	// Loop over reading in fields
	out.WriteString(g.indent() + "while (true) {" + "\n")
	g.indentUp()

	out.WriteString(g.indent() + g.jsConstType + "ret = input.readFieldBegin();" + "\n")
	out.WriteString(g.indent() + g.jsConstType + "ftype = ret.ftype;" + "\n")
	if len(fields) > 0 {
		out.WriteString(g.indent() + g.jsConstType + "fid = ret.fid;" + "\n")
	}

	// Check for field STOP marker and break
	out.WriteString(g.indent() + "if (ftype == Thrift.Type.STOP) {" + "\n")
	g.indentUp()
	out.WriteString(g.indent() + "break;" + "\n")
	g.indentDown()
	out.WriteString(g.indent() + "}" + "\n")
	if len(fields) > 0 {
		// Switch statement on the field we are reading
		out.WriteString(g.indent() + "switch (fid) {" + "\n")
		g.indentUp()

		for _, f := range fields {
			out.WriteString(g.indent() + "case " + itoa32(f.Key()) + ":" + "\n")
			out.WriteString(g.indent() + "if (ftype == " + typeToEnum(f.Type()) + ") {" + "\n")

			g.indentUp()
			g.generateDeserializeField(out, f, "this.")
			g.indentDown()

			out.WriteString(g.indent() + "} else {" + "\n")
			out.WriteString(g.indent() + "  input.skip(ftype);" + "\n")

			out.WriteString(g.indent() + "}" + "\n" + g.indent() + "break;" + "\n")
		}
		if len(fields) == 1 {
			// pseudo case to make jslint happy
			out.WriteString(g.indent() + "case 0:" + "\n")
			out.WriteString(g.indent() + "  input.skip(ftype);" + "\n")
			out.WriteString(g.indent() + "  break;" + "\n")
		}
		// In the default case we skip the field
		out.WriteString(g.indent() + "default:" + "\n")
		out.WriteString(g.indent() + "  input.skip(ftype);" + "\n")

		g.scopeDown(out)
	} else {
		out.WriteString(g.indent() + "input.skip(ftype);" + "\n")
	}

	out.WriteString(g.indent() + "input.readFieldEnd();" + "\n")

	g.scopeDown(out)

	out.WriteString(g.indent() + "input.readStructEnd();" + "\n")

	g.indentDown()
	out.WriteString(g.indent() + "} finally {" + "\n")
	g.indentUp()
	out.WriteString(g.indent() + "input.decrementRecursionDepth();" + "\n")
	g.indentDown()
	out.WriteString(g.indent() + "}" + "\n")

	out.WriteString(g.indent() + "return;" + "\n")

	g.indentDown()

	if g.opts.ES6 {
		out.WriteString(g.indent() + "}" + "\n" + "\n")
	} else {
		out.WriteString(g.indent() + "};" + "\n" + "\n")
	}
}

// generateJsStructWriter is generate_js_struct_writer: the write() method
// for a struct.
func (g *Generator) generateJsStructWriter(out *strings.Builder, s *sema.Struct) {
	name := s.Name()
	fields := s.Members()

	if g.opts.ES6 {
		out.WriteString(g.indent() + "[Symbol.for(\"write\")] (output) {" + "\n")
	} else {
		out.WriteString(g.indent() + g.jsNamespace(s.Program()) + s.Name() + ".prototype[Symbol.for(\"write\")] = function(output) {" + "\n")
	}

	g.indentUp()

	out.WriteString(g.indent() + "output.incrementRecursionDepth();" + "\n")
	out.WriteString(g.indent() + "try {" + "\n")
	g.indentUp()

	out.WriteString(g.indent() + "output.writeStructBegin('" + name + "');" + "\n")

	for _, f := range fields {
		out.WriteString(g.indent() + "if (this." + f.Name() + " !== null && this." + f.Name() + " !== undefined) {" + "\n")
		g.indentUp()

		out.WriteString(g.indent() + "output.writeFieldBegin(" + "'" + f.Name() + "', " + typeToEnum(f.Type()) + ", " + itoa32(f.Key()) + ");" + "\n")

		// Write field contents
		g.generateSerializeField(out, f, "this.")

		out.WriteString(g.indent() + "output.writeFieldEnd();" + "\n")

		g.indentDown()
		out.WriteString(g.indent() + "}" + "\n")
	}

	out.WriteString(g.indent() + "output.writeFieldStop();" + "\n" + g.indent() + "output.writeStructEnd();" + "\n")

	g.indentDown()
	out.WriteString(g.indent() + "} finally {" + "\n")
	g.indentUp()
	out.WriteString(g.indent() + "output.decrementRecursionDepth();" + "\n")
	g.indentDown()
	out.WriteString(g.indent() + "}" + "\n")

	out.WriteString(g.indent() + "return;" + "\n")

	g.indentDown()
	if g.opts.ES6 {
		out.WriteString(g.indent() + "}" + "\n" + "\n")
	} else {
		out.WriteString(g.indent() + "};" + "\n" + "\n")
	}
}
