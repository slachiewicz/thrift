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

package javame

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateService is generate_service: one file per service, comprising
// the interface, the client and the server.
func (g *Generator) generateService(sv *sema.Service) {
	g.fServiceName = g.packageDir + "/" + g.serviceName + ".java"
	g.fService.Reset()
	f := &g.fService

	f.WriteString(autogenComment() + g.javaPackage() + g.javaTypeImports() + g.javaThriftImports())

	f.WriteString("public class " + g.serviceName + " {\n\n")
	g.indentUp()

	// Generate the three main parts of the service
	g.generateServiceInterface(sv)
	g.generateServiceClient(sv)
	g.generateServiceServer(sv)
	g.generateServiceHelpers(sv)

	g.indentDown()
	f.WriteString("}\n")
	emit.WriteFile(g.fServiceName, f.String())
}

// generatePrimitiveServiceInterface is generate_primitive_service_interface.
// It is declared and defined in t_javame_generator.cc but never called
// from anywhere else in the file: generateService above only calls
// generateServiceInterface. This port keeps it, unreachable, for
// fidelity.
func (g *Generator) generatePrimitiveServiceInterface(sv *sema.Service) {
	g.fService.WriteString(g.indent() + "public interface Iface extends " + g.serviceName + "Iface { }\n\n")

	fInterfaceName := g.packageDir + "/" + g.serviceName + "Iface.java"
	var fIface strings.Builder

	extendsIface := ""
	if sv.Extends() != nil {
		extendsIface = " extends " + g.tn(sv.Extends()) + "Iface"
	}

	fIface.WriteString(autogenComment() + g.javaPackage() + g.javaTypeImports() + g.javaThriftImports())
	g.javaDoc(&fIface, sv)
	fIface.WriteString("public interface " + g.serviceName + "Iface" + extendsIface + " {\n\n")
	for _, fn := range sv.Functions() {
		g.javaDocFunction(&fIface, fn)
		fIface.WriteString("  public " + g.functionSignature(fn, "") + ";\n\n")
	}
	fIface.WriteString("}\n\n")

	emit.WriteFile(fInterfaceName, fIface.String())
}

// generateServiceInterface is generate_service_interface.
func (g *Generator) generateServiceInterface(sv *sema.Service) {
	f := &g.fService
	extends := ""
	extendsIface := ""
	if sv.Extends() != nil {
		extends = g.tn(sv.Extends())
		extendsIface = " extends " + extends + ".Iface"
	}

	g.javaDoc(f, sv)
	f.WriteString(g.indent() + "public interface Iface" + extendsIface + " {\n\n")
	g.indentUp()
	for _, fn := range sv.Functions() {
		g.javaDocFunction(f, fn)
		f.WriteString(g.indent() + "public " + g.functionSignature(fn, "") + ";\n\n")
	}
	g.indentDown()
	f.WriteString(g.indent() + "}\n\n")
}

// generateServiceHelpers is generate_service_helpers: structs for all
// the service args and return types.
func (g *Generator) generateServiceHelpers(sv *sema.Service) {
	for _, fn := range sv.Functions() {
		g.generateJavaStructDefinition(&g.fService, fn.Arglist(), false, true, false)
		g.generateFunctionHelpers(fn)
	}
}

// onewayOrCall renders TMessageType.ONEWAY or TMessageType.CALL.
func onewayOrCall(oneway bool) string {
	if oneway {
		return "TMessageType.ONEWAY"
	}
	return "TMessageType.CALL"
}

// generateServiceClient is generate_service_client.
func (g *Generator) generateServiceClient(sv *sema.Service) {
	f := &g.fService
	extends := ""
	extendsClient := ""
	if sv.Extends() != nil {
		extends = g.tn(sv.Extends())
		extendsClient = " extends " + extends + ".Client"
	}

	f.WriteString(g.indent() + "public static class Client" + extendsClient + " implements TServiceClient, Iface {\n")
	g.indentUp()

	f.WriteString(g.indent() + "public Client(TProtocol prot)\n")
	g.scopeUp(f)
	f.WriteString(g.indent() + "this(prot, prot);\n")
	g.scopeDown(f)
	f.WriteString("\n")

	f.WriteString(g.indent() + "public Client(TProtocol iprot, TProtocol oprot)\n")
	g.scopeUp(f)
	if extends == "" {
		f.WriteString(g.indent() + "iprot_ = iprot;\n" + g.indent() + "oprot_ = oprot;\n")
	} else {
		f.WriteString(g.indent() + "super(iprot, oprot);\n")
	}
	g.scopeDown(f)
	f.WriteString("\n")

	if extends == "" {
		f.WriteString(g.indent() + "protected TProtocol iprot_;\n" + g.indent() + "protected TProtocol oprot_;\n\n" +
			g.indent() + "protected int seqid_;\n\n")

		f.WriteString(g.indent() + "public TProtocol getInputProtocol()\n")
		g.scopeUp(f)
		f.WriteString(g.indent() + "return this.iprot_;\n")
		g.scopeDown(f)
		f.WriteString("\n")

		f.WriteString(g.indent() + "public TProtocol getOutputProtocol()\n")
		g.scopeUp(f)
		f.WriteString(g.indent() + "return this.oprot_;\n")
		g.scopeDown(f)
		f.WriteString("\n")
	}

	// Generate client method implementations
	for _, fn := range sv.Functions() {
		funname := fn.Name()

		// Open function
		f.WriteString(g.indent() + "public " + g.functionSignature(fn, "") + "\n")
		g.scopeUp(f)
		f.WriteString(g.indent() + "send_" + funname + "(")

		fields := fn.Arglist().Members()
		first := true
		for _, fld := range fields {
			if first {
				first = false
			} else {
				f.WriteString(", ")
			}
			f.WriteString(fld.Name())
		}
		f.WriteString(");\n")

		if !fn.IsOneway() {
			f.WriteString(g.indent())
			if !fn.ReturnType().IsVoid() {
				f.WriteString("return ")
			}
			f.WriteString("recv_" + funname + "();\n")
		}
		g.scopeDown(f)
		f.WriteString("\n")

		sendFunction := sema.NewFunction(sema.GlobalVoid, "send_"+fn.Name(), fn.Arglist(), sema.NewStruct(g.program), false, nil)

		argsname := fn.Name() + "_args"

		// Open function
		f.WriteString(g.indent() + "public " + g.functionSignature(sendFunction, "") + "\n")
		g.scopeUp(f)

		// Serialize the request
		f.WriteString(g.indent() + "oprot_.writeMessageBegin(new TMessage(\"" + funname + "\", " +
			onewayOrCall(fn.IsOneway()) + ", ++seqid_));\n" +
			g.indent() + argsname + " args = new " + argsname + "();\n")

		for _, fld := range fields {
			f.WriteString(g.indent() + "args.set" + getCapName(fld.Name()) + "(" + fld.Name() + ");\n")
		}

		f.WriteString(g.indent() + "args.write(oprot_);\n" + g.indent() + "oprot_.writeMessageEnd();\n" +
			g.indent() + "oprot_.getTransport().flush();\n")

		g.scopeDown(f)
		f.WriteString("\n")

		if !fn.IsOneway() {
			resultname := fn.Name() + "_result"

			noargs := sema.NewStruct(g.program)
			recvFunction := sema.NewFunction(fn.ReturnType(), "recv_"+fn.Name(), noargs, fn.Xceptions(), false, nil)
			// Open function
			f.WriteString(g.indent() + "public " + g.functionSignature(recvFunction, "") + "\n")
			g.scopeUp(f)

			f.WriteString(g.indent() + "TMessage msg = iprot_.readMessageBegin();\n" +
				g.indent() + "if (msg.type == TMessageType.EXCEPTION) {\n" +
				g.indent() + "  TApplicationException x = TApplicationException.read(iprot_);\n" +
				g.indent() + "  iprot_.readMessageEnd();\n" +
				g.indent() + "  throw x;\n" +
				g.indent() + "}\n" +
				g.indent() + "if (msg.seqid != seqid_) {\n" +
				g.indent() + "  throw new TApplicationException(TApplicationException.BAD_SEQUENCE_ID, \"" + fn.Name() + " failed: out of sequence response\");\n" +
				g.indent() + "}\n" +
				g.indent() + resultname + " result = new " + resultname + "();\n" +
				g.indent() + "result.read(iprot_);\n" +
				g.indent() + "iprot_.readMessageEnd();\n")

			// Careful, only return _result if not a void function
			if !fn.ReturnType().IsVoid() {
				f.WriteString(g.indent() + "if (result." + g.issetCheckName("success") + ") {\n" +
					g.indent() + "  return result.success;\n" +
					g.indent() + "}\n")
			}

			for _, x := range fn.Xceptions().Members() {
				f.WriteString(g.indent() + "if (result." + x.Name() + " != null) {\n" +
					g.indent() + "  throw result." + x.Name() + ";\n" +
					g.indent() + "}\n")
			}

			// If you get here it's an exception, unless a void function
			if fn.ReturnType().IsVoid() {
				f.WriteString(g.indent() + "return;\n")
			} else {
				f.WriteString(g.indent() + "throw new TApplicationException(TApplicationException.MISSING_RESULT, \"" + fn.Name() + " failed: unknown result\");\n")
			}

			// Close function
			g.scopeDown(f)
			f.WriteString("\n")
		}
	}

	g.indentDown()
	f.WriteString(g.indent() + "}\n")
}

// generateServiceServer is generate_service_server.
func (g *Generator) generateServiceServer(sv *sema.Service) {
	f := &g.fService

	extends := ""
	extendsProcessor := ""
	if sv.Extends() != nil {
		extends = g.tn(sv.Extends())
		extendsProcessor = " extends " + extends + ".Processor"
	}

	// Generate the header portion
	f.WriteString(g.indent() + "public static class Processor" + extendsProcessor + " implements TProcessor {\n")
	g.indentUp()

	f.WriteString(g.indent() + "public Processor(Iface iface)\n")
	g.scopeUp(f)
	if extends != "" {
		f.WriteString(g.indent() + "super(iface);\n")
	}
	f.WriteString(g.indent() + "iface_ = iface;\n")

	for _, fn := range sv.Functions() {
		f.WriteString(g.indent() + "processMap_.put(\"" + fn.Name() + "\", new " + fn.Name() + "());\n")
	}

	g.scopeDown(f)
	f.WriteString("\n")

	if extends == "" {
		f.WriteString(g.indent() + "protected static interface ProcessFunction {\n" + g.indent() +
			"  public void process(int seqid, TProtocol iprot, TProtocol oprot) throws TException;\n" +
			g.indent() + "}\n\n")
	}

	f.WriteString(g.indent() + "private Iface iface_;\n")

	if extends == "" {
		f.WriteString(g.indent() + "protected final Hashtable processMap_ = new Hashtable();\n")
	}

	f.WriteString("\n")

	// Generate the server implementation
	f.WriteString(g.indent() + "public boolean process(TProtocol iprot, TProtocol oprot) throws TException\n")
	g.scopeUp(f)

	f.WriteString(g.indent() + "TMessage msg = iprot.readMessageBegin();\n")

	// TODO(mcslee): validate message, was the seqid etc. legit?

	f.WriteString(g.indent() + "ProcessFunction fn = (ProcessFunction)processMap_.get(msg.name);\n" +
		g.indent() + "if (fn == null) {\n" +
		g.indent() + "  TProtocolUtil.skip(iprot, TType.STRUCT);\n" +
		g.indent() + "  iprot.readMessageEnd();\n" +
		g.indent() + "  TApplicationException x = new TApplicationException(TApplicationException.UNKNOWN_METHOD, \"Invalid method name: '\"+msg.name+\"'\");\n" +
		g.indent() + "  oprot.writeMessageBegin(new TMessage(msg.name, TMessageType.EXCEPTION, msg.seqid));\n" +
		g.indent() + "  x.write(oprot);\n" +
		g.indent() + "  oprot.writeMessageEnd();\n" +
		g.indent() + "  oprot.getTransport().flush();\n" +
		g.indent() + "  return true;\n" +
		g.indent() + "}\n" +
		g.indent() + "fn.process(msg.seqid, iprot, oprot);\n")

	f.WriteString(g.indent() + "return true;\n")

	g.scopeDown(f)
	f.WriteString("\n")

	// Generate the process subfunctions
	for _, fn := range sv.Functions() {
		g.generateProcessFunction(sv, fn)
	}

	g.indentDown()
	f.WriteString(g.indent() + "}\n\n")
}

// generateFunctionHelpers is generate_function_helpers: a struct and
// helpers for a function.
func (g *Generator) generateFunctionHelpers(fn *sema.Function) {
	if fn.IsOneway() {
		return
	}

	result := sema.NewStruct(g.program)
	result.SetName(fn.Name() + "_result")
	success := sema.NewField(fn.ReturnType(), "success", 0)
	if !fn.ReturnType().IsVoid() {
		result.Append(success)
	}

	for _, f := range fn.Xceptions().Members() {
		result.Append(f)
	}

	g.generateJavaStructDefinition(&g.fService, result, false, true, true)
}

// generateProcessFunction is generate_process_function: a process
// function definition (dispatcher).
func (g *Generator) generateProcessFunction(sv *sema.Service, fn *sema.Function) {
	_ = sv
	f := &g.fService

	// Open class
	f.WriteString(g.indent() + "private class " + fn.Name() + " implements ProcessFunction {\n")
	g.indentUp()

	// Open function
	f.WriteString(g.indent() + "public void process(int seqid, TProtocol iprot, TProtocol oprot) throws TException\n")
	g.scopeUp(f)

	argsname := fn.Name() + "_args"
	resultname := fn.Name() + "_result"

	f.WriteString(g.indent() + argsname + " args = new " + argsname + "();\n" + g.indent() + "try {\n")
	g.indentUp()
	f.WriteString(g.indent() + "args.read(iprot);\n")
	g.indentDown()
	f.WriteString(g.indent() + "} catch (TProtocolException e) {\n")
	g.indentUp()
	f.WriteString(g.indent() + "iprot.readMessageEnd();\n" +
		g.indent() + "TApplicationException x = new TApplicationException(TApplicationException.PROTOCOL_ERROR, e.getMessage());\n" +
		g.indent() + "oprot.writeMessageBegin(new TMessage(\"" + fn.Name() + "\", TMessageType.EXCEPTION, seqid));\n" +
		g.indent() + "x.write(oprot);\n" +
		g.indent() + "oprot.writeMessageEnd();\n" +
		g.indent() + "oprot.getTransport().flush();\n" +
		g.indent() + "return;\n")
	g.indentDown()
	f.WriteString(g.indent() + "}\n")
	f.WriteString(g.indent() + "iprot.readMessageEnd();\n")

	xceptions := fn.Xceptions().Members()

	// Declare result for non oneway function
	if !fn.IsOneway() {
		f.WriteString(g.indent() + resultname + " result = new " + resultname + "();\n")
	}

	// Try block for a function with exceptions
	if len(xceptions) > 0 {
		f.WriteString(g.indent() + "try {\n")
		g.indentUp()
	}

	// Generate the function call
	fields := fn.Arglist().Members()

	f.WriteString(g.indent())
	if !fn.IsOneway() && !fn.ReturnType().IsVoid() {
		f.WriteString("result.success = ")
	}
	f.WriteString("iface_." + fn.Name() + "(")
	first := true
	for _, fld := range fields {
		if first {
			first = false
		} else {
			f.WriteString(", ")
		}
		f.WriteString("args." + fld.Name())
	}
	f.WriteString(");\n")

	// Set isset on success field
	if !fn.IsOneway() && !fn.ReturnType().IsVoid() && !typeCanBeNull(fn.ReturnType()) {
		f.WriteString(g.indent() + "result.set" + getCapName("success") + getCapName("isSet") + "(true);\n")
	}

	if !fn.IsOneway() && len(xceptions) > 0 {
		g.indentDown()
		f.WriteString(g.indent() + "}")
		for _, x := range xceptions {
			f.WriteString(" catch (" + g.typeName(x.Type(), false) + " " + x.Name() + ") {\n")
			if !fn.IsOneway() {
				g.indentUp()
				f.WriteString(g.indent() + "result." + x.Name() + " = " + x.Name() + ";\n")
				g.indentDown()
				f.WriteString(g.indent() + "}")
			} else {
				f.WriteString("}")
			}
		}
		f.WriteString(" catch (Throwable th) {\n")
		g.indentUp()
		f.WriteString(g.indent() + "TApplicationException x = new TApplicationException(TApplicationException.INTERNAL_ERROR, \"Internal error processing " + fn.Name() + "\");\n" +
			g.indent() + "oprot.writeMessageBegin(new TMessage(\"" + fn.Name() + "\", TMessageType.EXCEPTION, seqid));\n" +
			g.indent() + "x.write(oprot);\n" +
			g.indent() + "oprot.writeMessageEnd();\n" +
			g.indent() + "oprot.getTransport().flush();\n" +
			g.indent() + "return;\n")
		g.indentDown()
		f.WriteString(g.indent() + "}\n")
	}

	// Shortcut out here for oneway functions
	if fn.IsOneway() {
		f.WriteString(g.indent() + "return;\n")
		g.scopeDown(f)

		// Close class
		g.indentDown()
		f.WriteString(g.indent() + "}\n\n")
		return
	}

	f.WriteString(g.indent() + "oprot.writeMessageBegin(new TMessage(\"" + fn.Name() + "\", TMessageType.REPLY, seqid));\n" +
		g.indent() + "result.write(oprot);\n" +
		g.indent() + "oprot.writeMessageEnd();\n" +
		g.indent() + "oprot.getTransport().flush();\n")

	// Close function
	g.scopeDown(f)
	f.WriteString("\n")

	// Close class
	g.indentDown()
	f.WriteString(g.indent() + "}\n\n")
}
