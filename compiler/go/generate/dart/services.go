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

package dart

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateService is t_dart_generator::generate_service.
func (g *Generator) generateService(s *sema.Service) {
	fileName := getFileName(g.serviceName)
	fServiceName := g.srcDir + "/" + fileName + ".dart"
	f := &strings.Builder{}
	g.fService = f

	f.WriteString(autogenComment() + g.dartLibrary(fileName) + "\n")
	f.WriteString(serviceImports() + g.dartThriftImports() + "\n")
	f.WriteString("\n")

	g.generateServiceInterface(s)
	g.generateServiceClient(s)
	g.generateServiceServer(s)
	g.generateServiceHelpers(s)

	emit.WriteFile(fServiceName, f.String())
}

// generateServiceInterface is t_dart_generator::generate_service_interface:
// generates a service interface definition.
func (g *Generator) generateServiceInterface(s *sema.Service) {
	extendsIface := ""
	if s.Extends() != nil {
		extendsIface = " extends " + g.getTtypeClassName(s.Extends())
	}

	g.generateDartDoc(g.fService, s)

	className := g.serviceName
	g.exportClassToLibrary(getFileName(g.serviceName), className)
	g.fService.WriteString(g.ind() + "abstract class " + className + extendsIface)
	g.scopeUp(g.fService)

	for _, f := range s.Functions() {
		g.fService.WriteString("\n")
		g.generateDartDocFunction(g.fService, f)
		g.fService.WriteString(g.ind() + g.functionSignature(f) + ";\n")
	}

	g.scopeDownPostfix(g.fService, "\n\n")
}

// generateServiceHelpers is t_dart_generator::generate_service_helpers:
// generates structs for all the service args and return types.
func (g *Generator) generateServiceHelpers(s *sema.Service) {
	for _, f := range s.Functions() {
		ts := f.Arglist()
		g.generateDartStructDefinition(g.fService, ts, false, false, "")
		g.generateFunctionHelpers(f)
	}
}

// generateServiceClient is t_dart_generator::generate_service_client:
// generates a service client definition.
func (g *Generator) generateServiceClient(s *sema.Service) {
	extends := ""
	extendsClient := ""
	if s.Extends() != nil {
		extends = g.getTtypeClassName(s.Extends())
		extendsClient = " extends " + extends + "Client"
	}

	className := g.serviceName + "Client"
	g.exportClassToLibrary(getFileName(g.serviceName), className)
	out := g.fService
	out.WriteString(g.ind() + "class " + className + extendsClient + " implements " + g.serviceName)
	g.scopeUp(out)
	out.WriteString("\n")

	out.WriteString(g.ind() + className + "(TProtocol iprot, [TProtocol? oprot = null])")

	if extends != "" {
		g.indentUp()
		out.WriteString("\n")
		out.WriteString(g.ind() + ": super(iprot, oprot);\n")
		g.indentDown()
	} else {
		g.scopeUp(out)
		out.WriteString(g.ind() + "_iprot = iprot;\n")
		out.WriteString(g.ind() + "_oprot = (oprot == null) ? iprot : oprot;\n")
		g.scopeDown(out)
	}
	out.WriteString("\n")

	if extends == "" {
		out.WriteString(g.ind() + "late TProtocol _iprot;\n\n")
		out.WriteString(g.ind() + "TProtocol get iprot => _iprot;\n\n")
		out.WriteString(g.ind() + "late TProtocol _oprot;\n\n")
		out.WriteString(g.ind() + "TProtocol get oprot => _oprot;\n\n")
		out.WriteString(g.ind() + "int _seqid = 0;\n\n")
		out.WriteString(g.ind() + "int get seqid => _seqid;\n\n")
		out.WriteString(g.ind() + "int nextSeqid() => ++_seqid;\n\n")
	}

	// Generate client method implementations
	for _, f := range s.Functions() {
		// Open function
		out.WriteString(g.ind() + g.functionSignature(f) + " async")
		g.scopeUp(out)

		// Get the struct of function call params
		argStruct := f.Arglist()

		argsname := getArgsClassName(f.Name())
		fields := argStruct.Members()

		// Serialize the request
		messageType := "TMessageType.CALL"
		if f.IsOneway() {
			messageType = "TMessageType.ONEWAY"
		}
		out.WriteString(g.ind() + "oprot.writeMessageBegin(new TMessage(\"" + f.Name() + "\", " + messageType + ", nextSeqid()));\n")
		out.WriteString(g.ind() + argsname + " args = new " + argsname + "();\n")

		for _, fld := range fields {
			argFieldName := getMemberName(fld.Name())
			out.WriteString(g.ind() + "args." + argFieldName + " = " + argFieldName + ";\n")
		}

		out.WriteString(g.ind() + "args.write(oprot);\n")
		out.WriteString(g.ind() + "oprot.writeMessageEnd();\n\n")

		out.WriteString(g.ind() + "await oprot.transport.flush();\n\n")

		if !f.IsOneway() {
			out.WriteString(g.ind() + "TMessage msg = iprot.readMessageBegin();\n")
			out.WriteString(g.ind() + "if (msg.type == TMessageType.EXCEPTION)")
			g.scopeUp(out)
			out.WriteString(g.ind() + "TApplicationError error = TApplicationError.read(iprot);\n")
			out.WriteString(g.ind() + "iprot.readMessageEnd();\n")
			out.WriteString(g.ind() + "throw error;\n")
			g.scopeDownPostfix(out, "\n\n")

			resultClass := getResultClassName(f.Name())
			out.WriteString(g.ind() + resultClass + " result = new " + resultClass + "();\n")
			out.WriteString(g.ind() + "result.read(iprot);\n")
			out.WriteString(g.ind() + "iprot.readMessageEnd();\n")

			// Careful, only return _result if not a void function
			if !f.ReturnType().IsVoid() {
				out.WriteString(g.ind() + "if (result." + generateIssetCheckName("success") + ")")
				g.scopeUp(out)
				out.WriteString(g.ind() + "return result.success;\n")
				g.scopeDownPostfix(out, "\n\n")
			}

			xs := f.Xceptions()
			for _, x := range xs.Members() {
				resultFieldName := getMemberName(x.Name())
				out.WriteString(g.ind() + "if (result." + resultFieldName + " != null)")
				g.scopeUp(out)
				out.WriteString(g.ind() + "throw result." + resultFieldName + "!;\n")
				g.scopeDown(out)
			}

			// If you get here it's an exception, unless a void function
			if f.ReturnType().IsVoid() {
				out.WriteString(g.ind() + "return;\n")
			} else {
				out.WriteString(g.ind() + "throw new TApplicationError(TApplicationErrorType.MISSING_RESULT, \"" +
					f.Name() + " failed: unknown result\");\n")
			}
		}

		g.scopeDownPostfix(out, "\n\n")
	}

	g.scopeDownPostfix(out, "\n\n")
}

// generateServiceServer is t_dart_generator::generate_service_server:
// generates a service server definition.
func (g *Generator) generateServiceServer(s *sema.Service) {
	out := g.fService
	functions := s.Functions()

	// typedef
	out.WriteString(g.ind() + "typedef void ProcessFunction(int seqid, TProtocol iprot, TProtocol oprot);\n\n")

	// Extends stuff
	extends := ""
	extendsProcessor := ""
	extendsCovariant := ""
	if s.Extends() != nil {
		extends = g.getTtypeClassName(s.Extends())
		extendsProcessor = " extends " + extends + "Processor"
		extendsCovariant = "covariant "
	}

	// Generate the header portion
	className := g.serviceName + "Processor"
	g.exportClassToLibrary(getFileName(g.serviceName), className)
	out.WriteString(g.ind() + "class " + className + extendsProcessor + " implements TProcessor")
	g.scopeUp(out)

	out.WriteString(g.ind() + className + "(" + g.serviceName + " iface)")
	if extends != "" {
		g.indentUp()
		out.WriteString("\n")
		out.WriteString(g.ind() + ": super(iface)")
		g.indentDown()
	}
	g.scopeUp(out)

	if extends == "" {
		out.WriteString(g.ind() + "iface_ = iface;\n")
	}

	for _, f := range functions {
		out.WriteString(g.ind() + "PROCESS_MAP[\"" + f.Name() + "\"] = " + getMemberName(f.Name()) + ";\n")
	}
	g.scopeDownPostfix(out, "\n\n")

	out.WriteString(g.ind() + extendsCovariant + "late " + g.serviceName + " iface_;\n")

	if extends == "" {
		out.WriteString(g.ind() + "final Map<String, ProcessFunction> PROCESS_MAP = {};\n")
	}

	out.WriteString("\n")

	// Generate the server implementation
	out.WriteString(g.ind() + "bool process(TProtocol iprot, TProtocol oprot)")
	g.scopeUp(out)
	out.WriteString(g.ind() + "TMessage msg = iprot.readMessageBegin();\n")
	out.WriteString(g.ind() + "ProcessFunction? fn = PROCESS_MAP[msg.name];\n")
	out.WriteString(g.ind() + "if (fn == null)")
	g.scopeUp(out)
	out.WriteString(g.ind() + "TProtocolUtil.skip(iprot, TType.STRUCT);\n")
	out.WriteString(g.ind() + "iprot.readMessageEnd();\n")
	out.WriteString(g.ind() + "TApplicationError x = new TApplicationError(TApplicationErrorType.UNKNOWN_METHOD, \"Invalid method name: '\"+msg.name+\"'\");\n")
	out.WriteString(g.ind() + "oprot.writeMessageBegin(new TMessage(msg.name, TMessageType.EXCEPTION, msg.seqid));\n")
	out.WriteString(g.ind() + "x.write(oprot);\n")
	out.WriteString(g.ind() + "oprot.writeMessageEnd();\n")
	out.WriteString(g.ind() + "oprot.transport.flush();\n")
	out.WriteString(g.ind() + "return true;\n")
	g.scopeDown(out)
	out.WriteString(g.ind() + "fn(msg.seqid, iprot, oprot);\n")
	out.WriteString(g.ind() + "return true;\n")
	g.scopeDownPostfix(out, "\n\n") // process function

	// Generate the process subfunctions
	for _, f := range functions {
		g.generateProcessFunction(s, f)
	}

	g.scopeDownPostfix(out, "\n\n") // class
}

// generateFunctionHelpers is t_dart_generator::generate_function_helpers:
// generates a struct and helpers for a function.
func (g *Generator) generateFunctionHelpers(f *sema.Function) {
	if f.IsOneway() {
		return
	}

	result := sema.NewStruct(g.program)
	result.SetName(getResultClassName(f.Name()))
	success := sema.NewField(f.ReturnType(), "success", 0)
	if !f.ReturnType().IsVoid() {
		result.Append(success)
	}

	xs := f.Xceptions()
	for _, fld := range xs.Members() {
		result.Append(fld)
	}

	g.generateDartStructDefinition(g.fService, result, false, true, "")
}

// generateProcessFunction is t_dart_generator::generate_process_function:
// generates a process function definition.
func (g *Generator) generateProcessFunction(s *sema.Service, f *sema.Function) {
	_ = s
	out := g.fService

	awaitResult := !f.IsOneway() && !f.ReturnType().IsVoid()

	out.WriteString(g.ind() + getMemberName(f.Name()) + "(int seqid, TProtocol iprot, TProtocol oprot)")
	if awaitResult {
		out.WriteString(" async")
	}
	g.scopeUp(out)

	argsname := getArgsClassName(f.Name())
	resultname := getResultClassName(f.Name())

	out.WriteString(g.ind() + argsname + " args = new " + argsname + "();\n")
	out.WriteString(g.ind() + "args.read(iprot);\n")
	out.WriteString(g.ind() + "iprot.readMessageEnd();\n")

	xs := f.Xceptions()
	xceptions := xs.Members()

	if !f.IsOneway() {
		out.WriteString(g.ind() + resultname + " result = new " + resultname + "();\n")
	}

	if !f.IsOneway() && len(xceptions) > 0 {
		out.WriteString(g.ind() + "try")
		g.scopeUp(out)
	}

	// Generate the function call
	argStruct := f.Arglist()
	fields := argStruct.Members()

	out.WriteString(g.ind())
	if awaitResult {
		out.WriteString("result.success = await ")
	}
	out.WriteString("iface_." + getMemberName(f.Name()) + "(")
	first := true
	for _, fld := range fields {
		if first {
			first = false
		} else {
			out.WriteString(", ")
		}
		out.WriteString("args." + getMemberName(fld.Name()))
	}
	out.WriteString(");\n")

	if !f.IsOneway() && len(xceptions) > 0 {
		for _, x := range xceptions {
			resultFieldName := getMemberName(x.Name())
			g.scopeDownPostfix(out, "")
			out.WriteString(" on " + g.typeName(x.Type()) + " catch(" + resultFieldName + ")")
			g.scopeUp(out)
			if !f.IsOneway() {
				out.WriteString(g.ind() + "result." + resultFieldName + " = " + resultFieldName + ";\n")
			}
		}
		g.scopeDownPostfix(out, " ")
		out.WriteString("catch (th)")
		g.scopeUp(out)
		out.WriteString(g.ind() + "// Internal error\n")
		out.WriteString(g.ind() + "TApplicationError x = new TApplicationError(TApplicationErrorType.INTERNAL_ERROR, \"Internal error processing " +
			f.Name() + "\");\n")
		out.WriteString(g.ind() + "oprot.writeMessageBegin(new TMessage(\"" + f.Name() + "\", TMessageType.EXCEPTION, seqid));\n")
		out.WriteString(g.ind() + "x.write(oprot);\n")
		out.WriteString(g.ind() + "oprot.writeMessageEnd();\n")
		out.WriteString(g.ind() + "oprot.transport.flush();\n")
		out.WriteString(g.ind() + "return;\n")
		g.scopeDown(out)
	}

	if f.IsOneway() {
		out.WriteString(g.ind() + "return;\n")
	} else {
		out.WriteString(g.ind() + "oprot.writeMessageBegin(new TMessage(\"" + f.Name() + "\", TMessageType.REPLY, seqid));\n")
		out.WriteString(g.ind() + "result.write(oprot);\n")
		out.WriteString(g.ind() + "oprot.writeMessageEnd();\n")
		out.WriteString(g.ind() + "oprot.transport.flush();\n")
	}

	g.scopeDownPostfix(out, "\n\n")
}
