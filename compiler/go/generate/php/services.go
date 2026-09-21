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

package php

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateService generates a thrift service.
func (g *Generator) generateService(s *sema.Service) {
	if g.opts.Classmap {
		g.fServiceName = g.packageDir + g.serviceName + ".php"
		g.fService = strings.Builder{}
		g.generateServiceHeader(s, &g.fService, phpcsDisablesClassmap)
	}

	// Generate the three main parts of the service (well, two for now
	// in PHP).
	g.generateServiceInterface(s)
	if g.opts.Rest {
		g.generateServiceRest(s)
	}
	g.generateServiceClient(s)
	g.generateServiceHelpers(s)
	if g.opts.Server {
		g.generateServiceProcessor(s)
	}

	if g.opts.Classmap {
		emit.WriteFile(g.fServiceName, g.fService.String())
	}
}

// generateServiceProcessor generates a service server definition.
func (g *Generator) generateServiceProcessor(s *sema.Service) {
	var out *strings.Builder
	var fname string
	if g.opts.Classmap {
		out = &g.fService
	} else {
		out = &strings.Builder{}
		fname = g.packageDir + g.serviceName + "Processor.php"
		g.generateServiceHeader(s, out, phpcsDisablesSnakeCaseMethods)
	}

	functions := s.Functions()

	extends := ""
	extendsProcessor := ""
	if s.Extends() != nil {
		extends = s.Extends().Name()
		extendsProcessor = " extends " + g.phpNamespace(s.Extends().Program()) + extends + "Processor"
	}

	// Generate the header portion.
	out.WriteString("class " + g.serviceName + "Processor" + extendsProcessor + "\n" + "{\n")
	g.indentUp()

	if extends == "" {
		out.WriteString(g.indent() + "protected ?object $handler_ = null;\n")
	}

	out.WriteString(g.indent() + "public function __construct(object $handler)\n" +
		g.indent() + "{\n")

	g.indentUp()
	if extends == "" {
		out.WriteString(g.indent() + "$this->handler_ = $handler;\n")
	} else {
		out.WriteString(g.indent() + "parent::__construct($handler);\n")
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	// Generate the server implementation.
	out.WriteString(g.indent() + "public function process(TProtocol $input, TProtocol $output): bool\n" +
		g.indent() + "{\n")
	g.indentUp()

	out.WriteString(g.indent() + "$rseqid = 0;\n" + g.indent() + "$fname = null;\n" +
		g.indent() + "$mtype = 0;\n\n")

	if g.opts.Inlined {
		ffname := newTempField(sema.GlobalString, "fname")
		fmtype := newTempField(sema.GlobalI8, "mtype")
		fseqid := newTempField(sema.GlobalI32, "rseqid")
		g.generateDeserializeField(out, ffname, "", true)
		g.generateDeserializeField(out, fmtype, "", true)
		g.generateDeserializeField(out, fseqid, "", true)
	} else {
		out.WriteString(g.indent() + "$input->readMessageBegin($fname, $mtype, $rseqid);\n")
	}

	// HOT: check for method implementation.
	out.WriteString(g.indent() + "$methodname = 'process_' . $fname;\n" +
		g.indent() + "if (!method_exists($this, $methodname)) {\n")

	g.indentUp()
	if g.opts.Inlined {
		out.WriteString(g.indent() + "throw new \\Exception('Function ' . $fname . ' not implemented.');\n")
	} else {
		out.WriteString(g.indent() + "$input->skip(TType::STRUCT);\n" +
			g.indent() + "$input->readMessageEnd();\n" +
			g.indent() + "$x = new TApplicationException(" +
			"'Function ' . $fname . ' not implemented.', " +
			"TApplicationException::UNKNOWN_METHOD);\n" +
			g.indent() + "$output->writeMessageBegin($fname, " +
			"TMessageType::EXCEPTION, $rseqid);\n" +
			g.indent() + "$x->write($output);\n" +
			g.indent() + "$output->writeMessageEnd();\n" +
			g.indent() + "$output->getTransport()->flush();\n" +
			g.indent() + "return false;\n")
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n" +
		g.indent() + "$this->$methodname($rseqid, $input, $output);\n" +
		g.indent() + "return true;\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	// Generate the process subfunctions.
	for _, f := range functions {
		g.generateProcessFunction(out, s, f)
	}

	g.indentDown()
	out.WriteString("}\n")

	if !g.opts.Classmap {
		emit.WriteFile(fname, out.String())
	}
}

// generateProcessFunction generates a process function definition.
func (g *Generator) generateProcessFunction(out *strings.Builder, s *sema.Service, f *sema.Function) {
	// Open function.
	out.WriteString(g.indent() + "protected function process_" + f.Name() +
		"(int $seqid, TProtocol $input, TProtocol $output): void\n" +
		g.indent() + "{\n")
	g.indentUp()

	argsname := g.phpNamespace(s.Program()) + g.serviceName + "_" + f.Name() + "_args"
	resultname := g.phpNamespace(s.Program()) + g.serviceName + "_" + f.Name() + "_result"

	out.WriteString(g.indent() + "$bin_accel = ($input instanceof " +
		"TBinaryProtocolAccelerated) && function_exists('thrift_protocol_read_binary_after_message_begin');\n")
	out.WriteString(g.indent() + "if ($bin_accel) {\n")
	g.indentUp()

	out.WriteString(g.indent() + "$args = thrift_protocol_read_binary_after_message_begin(\n")

	g.indentUp()
	out.WriteString(g.indent() + "$input,\n" +
		g.indent() + "'" + argsname + "',\n" +
		g.indent() + "$input->isStrictRead()\n")

	g.indentDown()
	out.WriteString(g.indent() + ");\n")

	g.indentDown()
	out.WriteString(g.indent() + "} else {\n")

	g.indentUp()
	out.WriteString(g.indent() + "$args = new " + argsname + "();\n" +
		g.indent() + "$args->read($input);\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	if !g.opts.Inlined {
		out.WriteString(g.indent() + "$input->readMessageEnd();\n")
	}

	xs := f.Xceptions()
	xceptions := xs.Members()

	// Declare result for non oneway function.
	if !f.IsOneway() {
		out.WriteString(g.indent() + "$result = new " + resultname + "();\n")
	}

	// Wrap handler invocations so undeclared runtime failures become
	// TApplicationException responses instead of crashing the PHP test
	// server.
	out.WriteString(g.indent() + "try {\n")
	g.indentUp()

	// Generate the function call.
	fields := f.Arglist().Members()

	out.WriteString(g.indent())
	if !f.IsOneway() && !f.ReturnType().IsVoid() {
		out.WriteString("$result->success = ")
	}
	out.WriteString("$this->handler_->" + f.Name() + "(")
	first := true
	for _, fld := range fields {
		if first {
			first = false
		} else {
			out.WriteString(", ")
		}
		out.WriteString("$args->" + fld.Name())
	}
	out.WriteString(");\n")

	g.indentDown()
	for _, x := range xceptions {
		out.WriteString(g.indent() + "} catch (" +
			g.phpNamespace(sema.TrueType(x.Type()).Program()) +
			x.Type().Name() + " $" + x.Name() + ") {\n")
		if !f.IsOneway() {
			g.indentUp()
			out.WriteString(g.indent() + "$result->" + x.Name() + " = $" + x.Name() + ";\n")
			g.indentDown()
		}
	}
	out.WriteString(g.indent() + "} catch (TApplicationException $ex) {\n")
	g.indentUp()
	if !f.IsOneway() {
		out.WriteString(g.indent() + "$output->writeMessageBegin('" + f.Name() +
			"', TMessageType::EXCEPTION, $seqid);\n" +
			g.indent() + "$ex->write($output);\n" +
			g.indent() + "$output->writeMessageEnd();\n" +
			g.indent() + "$output->getTransport()->flush();\n")
	}
	out.WriteString(g.indent() + "return;\n")
	g.indentDown()
	out.WriteString(g.indent() + "} catch (\\Throwable $ex) {\n")
	g.indentUp()
	if !f.IsOneway() {
		out.WriteString(g.indent() + "$x = new TApplicationException($ex->getMessage(), " +
			"TApplicationException::INTERNAL_ERROR);\n" +
			g.indent() + "$output->writeMessageBegin('" + f.Name() +
			"', TMessageType::EXCEPTION, $seqid);\n" +
			g.indent() + "$x->write($output);\n" +
			g.indent() + "$output->writeMessageEnd();\n" +
			g.indent() + "$output->getTransport()->flush();\n")
	}
	out.WriteString(g.indent() + "return;\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	// Shortcut out here for oneway functions.
	if f.IsOneway() {
		out.WriteString(g.indent() + "return;\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		return
	}

	out.WriteString(g.indent() + "$bin_accel = ($output instanceof " +
		"TBinaryProtocolAccelerated) && function_exists('thrift_protocol_write_binary');\n")

	out.WriteString(g.indent() + "if ($bin_accel) {\n")
	g.indentUp()

	out.WriteString(g.indent() + "thrift_protocol_write_binary(\n")

	g.indentUp()
	out.WriteString(g.indent() + "$output,\n" +
		g.indent() + "'" + f.Name() + "',\n" +
		g.indent() + "TMessageType::REPLY,\n" +
		g.indent() + "$result,\n" +
		g.indent() + "$seqid,\n" +
		g.indent() + "$output->isStrictWrite()\n")

	g.indentDown()
	out.WriteString(g.indent() + ");\n")

	g.indentDown()
	out.WriteString(g.indent() + "} else {\n")
	g.indentUp()

	// Serialize the request header.
	if g.opts.Inlined {
		out.WriteString(g.indent() + "$buff = pack('N', (TBinaryProtocol::VERSION_1 | " +
			"TMessageType::REPLY)); \n" + g.indent() + "$buff .= pack('N', strlen('" +
			f.Name() + "'));\n" + g.indent() + "$buff .= '" +
			f.Name() + "';\n" + g.indent() + "$buff .= pack('N', $seqid);\n" +
			g.indent() + "$result->write($buff);\n" + g.indent() +
			"$output->write($buff);\n" + g.indent() + "$output->flush();\n")
	} else {
		out.WriteString(g.indent() + "$output->writeMessageBegin('" + f.Name() + "', " +
			"TMessageType::REPLY, $seqid);\n" + g.indent() + "$result->write($output);\n" +
			g.indent() + "$output->writeMessageEnd();\n" + g.indent() +
			"$output->getTransport()->flush();\n")
	}

	g.scopeDown(out)

	// Close function.
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

// generateServiceHelpers generates helper functions for a service.
func (g *Generator) generateServiceHelpers(s *sema.Service) {
	functions := s.Functions()

	if g.opts.Classmap {
		g.fService.WriteString("// HELPER FUNCTIONS AND STRUCTURES\n\n")
	}

	for _, f := range functions {
		ts := f.Arglist()
		name := ts.Name()
		ts.SetName(g.serviceName + "_" + name)

		var out *strings.Builder
		var fname string
		if g.opts.Classmap {
			out = &g.fService
		} else {
			out = &strings.Builder{}
			fname = g.packageDir + g.serviceName + "_" + name + ".php"
			g.generateServiceHeader(s, out, phpcsDisablesServiceHelpers)
		}

		g.generatePHPStructDefinition(out, ts, false, false)
		if !g.opts.Classmap {
			emit.WriteFile(fname, out.String())
		}

		g.generatePHPFunctionHelpers(s, f)
		ts.SetName(name)
	}
}

// generatePHPFunctionHelpers generates a struct and helpers for a
// function.
func (g *Generator) generatePHPFunctionHelpers(s *sema.Service, f *sema.Function) {
	if f.IsOneway() {
		return
	}
	result := sema.NewStruct(g.program)
	result.SetName(g.serviceName + "_" + f.Name() + "_result")
	success := sema.NewField(f.ReturnType(), "success", 0)
	if !f.ReturnType().IsVoid() {
		result.Append(success)
	}

	for _, x := range f.Xceptions().Members() {
		result.Append(x)
	}

	var out *strings.Builder
	var fname string
	if g.opts.Classmap {
		out = &g.fService
	} else {
		out = &strings.Builder{}
		fname = g.packageDir + result.Name() + ".php"
		g.generateServiceHeader(s, out, phpcsDisablesServiceHelpers)
	}
	g.generatePHPStructDefinition(out, result, false, true)
	if !g.opts.Classmap {
		emit.WriteFile(fname, out.String())
	}
}

// generateServiceInterface generates a service interface definition.
func (g *Generator) generateServiceInterface(s *sema.Service) {
	var out *strings.Builder
	var fname string
	if g.opts.Classmap {
		out = &g.fService
	} else {
		out = &strings.Builder{}
		fname = g.packageDir + g.serviceName + "If.php"
		g.generateServiceHeader(s, out, nil)
	}

	extends := ""
	extendsIf := ""
	if s.Extends() != nil {
		extends = " extends " + g.phpNamespace(s.Extends().Program()) + s.Extends().Name()
		extendsIf = " extends " + g.phpNamespace(s.Extends().Program()) + s.Extends().Name() + "If"
	}
	_ = extends
	g.phpDoc(out, s)
	out.WriteString("interface " + phpNamespaceDeclaration(s) + "If" + extendsIf + "\n" + "{\n")

	g.indentUp()
	for _, f := range s.Functions() {
		g.phpDocFunction(out, f)
		out.WriteString(g.indent() + "public function " + g.functionSignature(f, "") + ";\n")
	}
	g.indentDown()
	out.WriteString("}\n")

	// Close service interface file.
	if !g.opts.Classmap {
		emit.WriteFile(fname, out.String())
	}
}

// generateServiceRest generates a REST interface.
func (g *Generator) generateServiceRest(s *sema.Service) {
	var out *strings.Builder
	var fname string
	if g.opts.Classmap {
		out = &g.fService
	} else {
		out = &strings.Builder{}
		fname = g.packageDir + g.serviceName + "Rest.php"
		g.generateServiceHeader(s, out, nil)
	}

	extends := ""
	extendsIf := ""
	if s.Extends() != nil {
		extends = " extends " + g.phpNamespace(s.Extends().Program()) + s.Extends().Name()
		extendsIf = " extends " + g.phpNamespace(s.Extends().Program()) + s.Extends().Name() + "Rest"
	}
	out.WriteString("class " + g.serviceName + "Rest" + extendsIf + "\n" + "{\n")
	g.indentUp()

	if extends == "" {
		out.WriteString(g.indent() + "protected object $impl;\n\n")
	}

	out.WriteString(g.indent() + "public function __construct(object $impl)\n" +
		g.indent() + "{\n" +
		g.indent() + "    $this->impl = $impl;\n" +
		g.indent() + "}\n\n")

	for _, f := range s.Functions() {
		out.WriteString(g.indent() + "public function " + f.Name() +
			"(array $request)" + g.typeToReturn(f.ReturnType()) + "\n")
		out.WriteString(g.indent() + "{\n")
		g.indentUp()
		for _, a := range f.Arglist().Members() {
			atype := sema.TrueType(a.Type())
			cast := typeToCast(atype)
			req := "$request['" + a.Name() + "']"
			if atype.IsBool() {
				out.WriteString(g.indent() + "$" + a.Name() + " = " + cast + "(!empty(" + req +
					") && (" + req + " !== 'false'));\n")
			} else {
				out.WriteString(g.indent() + "$" + a.Name() + " = isset(" + req + ") ? " +
					cast + req + " : null;\n")
			}
			switch {
			case atype.IsMap() || atype.IsList():
				out.WriteString(g.indent() + "$" + a.Name() + " = json_decode($" +
					a.Name() + ", true);\n")
			case atype.IsSet():
				out.WriteString(g.indent() + "$" + a.Name() + " = array_fill_keys(json_decode($" +
					a.Name() + ", true), 1);\n")
			case atype.IsStruct() || atype.IsXception():
				out.WriteString(g.indent() + "if ($" + a.Name() + " !== null) {\n" +
					g.indent() + "    $" + a.Name() + " = new " +
					g.phpNamespace(atype.Program()) + atype.Name() + "(json_decode($" +
					a.Name() + ", true));\n" + g.indent() + "}\n")
			}
		}
		returnKW := "return "
		if f.ReturnType().IsVoid() {
			returnKW = ""
		}
		out.WriteString(g.indent() + returnKW + "$this->impl->" + f.Name() + "(" +
			g.argumentList(f.Arglist(), false) + ");\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}
	g.indentDown()
	out.WriteString("}\n")

	// Close service rest file.
	out.WriteString("\n")
	if !g.opts.Classmap {
		emit.WriteFile(fname, out.String())
	}
}

// generateServiceClient generates a service client definition.
func (g *Generator) generateServiceClient(s *sema.Service) {
	var out *strings.Builder
	var fname string
	if g.opts.Classmap {
		out = &g.fService
	} else {
		out = &strings.Builder{}
		fname = g.packageDir + g.serviceName + "Client.php"
		g.generateServiceHeader(s, out, phpcsDisablesSnakeCaseMethods)
	}

	extends := ""
	extendsClient := ""
	if s.Extends() != nil {
		extends = s.Extends().Name()
		extendsClient = " extends " + g.phpNamespace(s.Extends().Program()) + extends + "Client"
	}

	out.WriteString("class " + phpNamespaceDeclaration(s) + "Client" + extendsClient +
		" implements " + g.phpNamespace(s.Program()) + g.serviceName + "If" + "\n" +
		"{\n")
	g.indentUp()

	// Private members.
	if extends == "" {
		out.WriteString(g.indent() + "protected ?TProtocol $input = null;\n" + g.indent() +
			"protected ?TProtocol $output = null;\n\n")
		out.WriteString(g.indent() + "protected int $seqid = 0;\n\n")
	}

	// Constructor function.
	out.WriteString(g.indent() + "public function __construct(TProtocol $input, ?TProtocol $output = null)\n" +
		g.indent() + "{\n")

	g.indentUp()
	if extends != "" {
		out.WriteString(g.indent() + "parent::__construct($input, $output);\n")
	} else {
		out.WriteString(g.indent() + "$this->input = $input;\n" +
			g.indent() + "$this->output = $output ? $output : $input;\n")
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	// Generate client method implementations.
	for _, f := range s.Functions() {
		fields := f.Arglist().Members()
		funname := f.Name()

		out.WriteString("\n")

		// Open function.
		out.WriteString(g.indent() + "public function " + g.functionSignature(f, "") + "\n")
		g.scopeUp(out)
		out.WriteString(g.indent() + "$this->send_" + funname + "(")

		first := true
		for _, fld := range fields {
			if first {
				first = false
			} else {
				out.WriteString(", ")
			}
			out.WriteString("$" + fld.Name())
		}
		out.WriteString(");\n")

		if !f.IsOneway() {
			out.WriteString(g.indent())
			if !f.ReturnType().IsVoid() {
				out.WriteString("return ")
			}
			out.WriteString("$this->recv_" + funname + "();\n")
		}
		g.scopeDown(out)
		out.WriteString("\n")

		sendFunction := sema.NewFunction(sema.GlobalVoid, "send_"+funname, f.Arglist(), sema.NewStruct(nil), false, nil)
		out.WriteString(g.indent() + "public function " + g.functionSignature(sendFunction, "") + "\n")
		g.scopeUp(out)

		argsname := g.phpNamespace(s.Program()) + g.serviceName + "_" + f.Name() + "_args"

		out.WriteString(g.indent() + "$args = new " + argsname + "();\n")

		for _, fld := range fields {
			out.WriteString(g.indent() + "$args->" + fld.Name() + " = $" + fld.Name() + ";\n")
		}

		out.WriteString(g.indent() + "$bin_accel = ($this->output instanceof " +
			"TBinaryProtocolAccelerated) && function_exists('thrift_protocol_write_binary');\n")

		out.WriteString(g.indent() + "if ($bin_accel) {\n")
		g.indentUp()

		messageType := "TMessageType::CALL"
		if f.IsOneway() {
			messageType = "TMessageType::ONEWAY"
		}

		out.WriteString(g.indent() + "thrift_protocol_write_binary(\n")

		g.indentUp()
		out.WriteString(g.indent() + "$this->output,\n" +
			g.indent() + "'" + f.Name() + "',\n" +
			g.indent() + messageType + ",\n" +
			g.indent() + "$args,\n" +
			g.indent() + "$this->seqid,\n" +
			g.indent() + "$this->output->isStrictWrite()\n")

		g.indentDown()
		out.WriteString(g.indent() + ");\n")

		g.indentDown()
		out.WriteString(g.indent() + "} else {\n")
		g.indentUp()

		// Serialize the request header.
		if g.opts.Inlined {
			out.WriteString(g.indent() + "$buff = pack('N', (TBinaryProtocol::VERSION_1 | " + messageType + "));\n" +
				g.indent() + "$buff .= pack('N', strlen('" + funname + "'));\n" +
				g.indent() + "$buff .= '" + funname + "';\n" + g.indent() +
				"$buff .= pack('N', $this->seqid);\n")
		} else {
			out.WriteString(g.indent() + "$this->output->writeMessageBegin('" + f.Name() +
				"', " + messageType + ", $this->seqid);\n")
		}

		// Write to the stream.
		if g.opts.Inlined {
			out.WriteString(g.indent() + "$args->write($buff);\n" + g.indent() +
				"$this->output->write($buff);\n" + g.indent() +
				"$this->output->flush();\n")
		} else {
			out.WriteString(g.indent() + "$args->write($this->output);\n" + g.indent() +
				"$this->output->writeMessageEnd();\n" + g.indent() +
				"$this->output->getTransport()->flush();\n")
		}

		g.scopeDown(out)
		g.scopeDown(out)

		if !f.IsOneway() {
			resultname := g.phpNamespace(s.Program()) + g.serviceName + "_" + f.Name() + "_result"
			noargs := sema.NewStruct(g.program)

			recvFunction := sema.NewFunction(f.ReturnType(), "recv_"+f.Name(), noargs, sema.NewStruct(nil), false, nil)
			// Open function.
			out.WriteString("\n" + g.indent() + "public function " + g.functionSignature(recvFunction, "") + "\n")
			g.scopeUp(out)

			out.WriteString(g.indent() + "$bin_accel = ($this->input instanceof " +
				"TBinaryProtocolAccelerated)" + " && function_exists('thrift_protocol_read_binary');\n")

			out.WriteString(g.indent() + "if ($bin_accel) {\n")

			g.indentUp()
			out.WriteString(g.indent() + "$result = thrift_protocol_read_binary(\n")

			g.indentUp()
			out.WriteString(g.indent() + "$this->input,\n" +
				g.indent() + "'" + resultname + "',\n" +
				g.indent() + "$this->input->isStrictRead()\n")

			g.indentDown()
			out.WriteString(g.indent() + ");\n")

			g.indentDown()
			out.WriteString(g.indent() + "} else {\n")

			g.indentUp()
			out.WriteString(g.indent() + "$rseqid = 0;\n" +
				g.indent() + "$fname = null;\n" +
				g.indent() + "$mtype = 0;\n\n")

			if g.opts.Inlined {
				ffname := newTempField(sema.GlobalString, "fname")
				fseqid := newTempField(sema.GlobalI32, "rseqid")
				out.WriteString(g.indent() + "$ver = unpack('N', $this->input->readAll(4));\n" +
					g.indent() + "$ver = $ver[1];\n" + g.indent() + "$mtype = $ver & 0xff;\n" +
					g.indent() + "$ver = $ver & TBinaryProtocol::VERSION_MASK;\n" +
					g.indent() + "if ($ver != TBinaryProtocol::VERSION_1) {\n" +
					g.indent() + "    throw new TProtocolException('Bad version identifier: ' . $ver, " +
					"TProtocolException::BAD_VERSION);\n" +
					g.indent() + "}\n")
				g.generateDeserializeField(out, ffname, "", true)
				g.generateDeserializeField(out, fseqid, "", true)
			} else {
				out.WriteString(g.indent() + "$this->input->readMessageBegin($fname, $mtype, $rseqid);\n" +
					g.indent() + "if ($mtype == TMessageType::EXCEPTION) {\n")

				g.indentUp()
				out.WriteString(g.indent() + "$x = new TApplicationException();\n" +
					g.indent() + "$x->read($this->input);\n" +
					g.indent() + "$this->input->readMessageEnd();\n" +
					g.indent() + "throw $x;\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
			}

			out.WriteString(g.indent() + "$result = new " + resultname + "();\n" +
				g.indent() + "$result->read($this->input);\n")

			if !g.opts.Inlined {
				out.WriteString(g.indent() + "$this->input->readMessageEnd();\n")
			}

			g.scopeDown(out)

			// Careful, only return result if not a void function.
			if !f.ReturnType().IsVoid() {
				out.WriteString(g.indent() + "if ($result->success !== null) {\n")

				g.indentUp()
				out.WriteString(g.indent() + "return $result->success;\n")

				g.indentDown()
				out.WriteString(g.indent() + "}\n")
			}

			for _, x := range f.Xceptions().Members() {
				out.WriteString(g.indent() + "if ($result->" + x.Name() + " !== null) {\n")

				g.indentUp()
				out.WriteString(g.indent() + "throw $result->" + x.Name() + ";\n")

				g.indentDown()
				out.WriteString(g.indent() + "}\n")
			}

			// Careful, only return _result if not a void function.
			if f.ReturnType().IsVoid() {
				out.WriteString(g.indent() + "return;\n")
			} else {
				out.WriteString(g.indent() + "throw new \\Exception(\"" + f.Name() +
					" failed: unknown result\");\n")
			}

			// Close function.
			g.scopeDown(out)
		}
	}

	g.indentDown()
	out.WriteString("}\n")

	// Close service client file.
	if !g.opts.Classmap {
		emit.WriteFile(fname, out.String())
	}
}
