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

package java

import (
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateService is generate_service: one file per service.
func (g *Generator) generateService(sv *sema.Service) {
	g.fServiceName = g.packageDir + "/" + makeValidJavaFilename(g.serviceName) + ".java"
	g.fService.Reset()
	f := &g.fService

	f.WriteString(autogenComment() + g.javaPackage())

	if !g.opts.SuppressGeneratedAnnotation {
		g.generateJavaxGeneratedAnnotation(f)
	}
	f.WriteString(javaSuppressions())
	f.WriteString("public class " + makeValidJavaIdentifier(g.serviceName) + " {\n\n")
	g.indentUp()

	g.generateServiceInterface(sv)
	g.generateServiceAsyncInterface(sv)
	if g.opts.GenerateFutureIface {
		g.generateServiceFutureInterface(sv)
	}
	g.generateServiceClient(sv)
	g.generateServiceAsyncClient(sv)
	if g.opts.GenerateFutureIface {
		g.generateServiceFutureClient(sv)
	}
	g.generateServiceServer(sv)
	g.generateServiceAsyncServer(sv)
	g.generateServiceHelpers(sv)

	g.indentDown()
	f.WriteString("}\n")
	emit.WriteFile(g.fServiceName, f.String())
}

func (g *Generator) generateServiceInterface(sv *sema.Service) {
	f := &g.fService
	extendsIface := ""
	if sv.Extends() != nil {
		extendsIface = " extends " + g.tn(sv.Extends()) + ".Iface"
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

func (g *Generator) generateServiceAsyncInterface(sv *sema.Service) {
	f := &g.fService
	extendsIface := ""
	if sv.Extends() != nil {
		extendsIface = " extends " + g.tn(sv.Extends()) + ".AsyncIface"
	}

	f.WriteString(g.indent() + "public interface AsyncIface" + extendsIface + " {\n\n")
	g.indentUp()
	for _, fn := range sv.Functions() {
		f.WriteString(g.indent() + "public " + g.functionSignatureAsync(fn, true, "") + " throws org.apache.thrift.TException;\n\n")
	}
	g.indentDown()
	f.WriteString(g.indent() + "}\n\n")
}

func (g *Generator) generateServiceFutureInterface(sv *sema.Service) {
	f := &g.fService
	extendsIface := ""
	if sv.Extends() != nil {
		extendsIface = " extends " + g.tn(sv.Extends()) + " .FutureIface"
	}

	f.WriteString(g.indent() + "public interface FutureIface" + extendsIface + " {\n\n")
	g.indentUp()
	for _, fn := range sv.Functions() {
		f.WriteString(g.indent() + "public " + g.functionSignatureFuture(fn, "") + " throws org.apache.thrift.TException;\n\n")
	}
	g.scopeDown(f)
	f.WriteString("\n\n")
}

func (g *Generator) generateServiceHelpers(sv *sema.Service) {
	for _, fn := range sv.Functions() {
		g.generateJavaStructDefinition(&g.fService, fn.Arglist(), false, true, false)
		g.generateFunctionHelpers(fn)
	}
}

// generateServiceClient is generate_service_client.
func (g *Generator) generateServiceClient(sv *sema.Service) {
	f := &g.fService
	extendsClient := "org.apache.thrift.TServiceClient"
	if sv.Extends() != nil {
		extendsClient = g.tn(sv.Extends()) + ".Client"
	}

	f.WriteString(g.indent() + "public static class Client extends " + extendsClient + " implements Iface {\n")
	g.indentUp()

	f.WriteString(g.indent() + "public static class Factory implements org.apache.thrift.TServiceClientFactory<Client> {\n")
	g.indentUp()
	f.WriteString(g.indent() + "public Factory() {}\n")
	f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	f.WriteString(g.indent() + "public Client getClient(org.apache.thrift.protocol.TProtocol prot) {\n")
	g.indentUp()
	f.WriteString(g.indent() + "return new Client(prot);\n")
	g.indentDown()
	f.WriteString(g.indent() + "}\n")
	f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	f.WriteString(g.indent() + "public Client getClient(org.apache.thrift.protocol.TProtocol iprot, org.apache.thrift.protocol.TProtocol oprot) {\n")
	g.indentUp()
	f.WriteString(g.indent() + "return new Client(iprot, oprot);\n")
	g.indentDown()
	f.WriteString(g.indent() + "}\n")
	g.indentDown()
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + "public Client(org.apache.thrift.protocol.TProtocol prot)\n")
	g.scopeUp(f)
	f.WriteString(g.indent() + "super(prot, prot);\n")
	g.scopeDown(f)
	f.WriteString("\n")

	f.WriteString(g.indent() + "public Client(org.apache.thrift.protocol.TProtocol iprot, org.apache.thrift.protocol.TProtocol oprot) {\n")
	f.WriteString(g.indent() + "  super(iprot, oprot);\n")
	f.WriteString(g.indent() + "}\n\n")

	for _, fn := range sv.Functions() {
		funname := fn.Name()
		sep := "_"
		javaname := funname
		if g.opts.FullcamelStyle {
			sep = ""
			javaname = asCamelCase(funname, true)
		}

		f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
		f.WriteString(g.indent() + "public " + g.functionSignature(fn, "") + "\n")
		g.scopeUp(f)
		f.WriteString(g.indent() + "send" + sep + javaname + "(")

		argStruct := fn.Arglist()
		fields := argStruct.Members()
		first := true
		for _, fld := range fields {
			if first {
				first = false
			} else {
				f.WriteString(", ")
			}
			f.WriteString(makeValidJavaIdentifier(fld.Name()))
		}
		f.WriteString(");\n")

		if !fn.IsOneway() {
			f.WriteString(g.indent())
			if !fn.ReturnType().IsVoid() {
				f.WriteString("return ")
			}
			f.WriteString("recv" + sep + javaname + "();\n")
		}
		g.scopeDown(f)
		f.WriteString("\n")

		sendFunction := sema.NewFunction(sema.GlobalVoid, "send"+sep+javaname, fn.Arglist(), sema.NewStruct(g.program), false, nil)

		argsname := fn.Name() + "_args"

		f.WriteString(g.indent() + "public " + g.functionSignature(sendFunction, "") + "\n")
		g.scopeUp(f)

		f.WriteString(g.indent() + argsname + " args = new " + argsname + "();\n")

		for _, fld := range fields {
			f.WriteString(g.indent() + "args.set" + g.capName(fld.Name()) + "(" + makeValidJavaIdentifier(fld.Name()) + ");\n")
		}

		sendBaseName := "sendBase"
		if fn.IsOneway() {
			sendBaseName = "sendBaseOneway"
		}
		f.WriteString(g.indent() + sendBaseName + "(\"" + funname + "\", args);\n")

		g.scopeDown(f)
		f.WriteString("\n")

		if !fn.IsOneway() {
			resultname := fn.Name() + "_result"

			noargs := sema.NewStruct(g.program)
			recvFunction := sema.NewFunction(fn.ReturnType(), "recv"+sep+javaname, noargs, fn.Xceptions(), false, nil)
			f.WriteString(g.indent() + "public " + g.functionSignature(recvFunction, "") + "\n")
			g.scopeUp(f)

			f.WriteString(g.indent() + resultname + " result = new " + resultname + "();\n" +
				g.indent() + "receiveBase(result, \"" + funname + "\");\n")

			if !fn.ReturnType().IsVoid() {
				f.WriteString(g.indent() + "if (result." + g.issetCheckName("success") + ") {\n" +
					g.indent() + "  return result.success;\n" +
					g.indent() + "}\n")
			}

			for _, x := range fn.Xceptions().Members() {
				xid := makeValidJavaIdentifier(x.Name())
				f.WriteString(g.indent() + "if (result." + xid + " != null) {\n" +
					g.indent() + "  throw result." + xid + ";\n" +
					g.indent() + "}\n")
			}

			if fn.ReturnType().IsVoid() {
				f.WriteString(g.indent() + "return;\n")
			} else {
				f.WriteString(g.indent() + "throw new org.apache.thrift.TApplicationException(org.apache.thrift.TApplicationException.MISSING_RESULT, \"" +
					fn.Name() + " failed: unknown result\");\n")
			}

			g.scopeDown(f)
			f.WriteString("\n")
		}
	}

	g.indentDown()
	f.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateServiceFutureClient(sv *sema.Service) {
	f := &g.fService
	extendsClient := ""
	if sv.Extends() != nil {
		extendsClient = "extends " + g.tn(sv.Extends()) + ".FutureClient "
	}

	const adapterClass = "org.apache.thrift.async.AsyncMethodFutureAdapter"
	f.WriteString(g.indent() + "public static class FutureClient " + extendsClient + "implements FutureIface {\n")
	g.indentUp()
	f.WriteString(g.indent() + "public FutureClient(AsyncIface delegate) {\n")
	g.indentUp()
	f.WriteString(g.indent() + "this.delegate = delegate;\n")
	g.scopeDown(f)
	f.WriteString(g.indent() + "private final AsyncIface delegate;\n")
	for _, fn := range sv.Functions() {
		funname := fn.Name()
		retTypeName := g.typeName(fn.ReturnType(), true, false, false, false)

		f.WriteString(g.indent() + "@Override\n")
		f.WriteString(g.indent() + "public " + g.functionSignatureFuture(fn, "") + " throws org.apache.thrift.TException {\n")
		g.indentUp()
		adapter := g.tmp("asyncMethodFutureAdapter")
		f.WriteString(g.indent() + adapterClass + "<" + retTypeName + "> " + adapter + " = " + adapterClass + ".<" + retTypeName + ">create();\n")
		emptyArgs := len(fn.Arglist().Members()) == 0
		comma := ", "
		if emptyArgs {
			comma = ""
		}
		f.WriteString(g.indent() + "delegate." + g.rpcMethodName(funname) + "(" + g.argumentList(fn.Arglist(), false) + comma + adapter + ");\n")
		f.WriteString(g.indent() + "return " + adapter + ".getFuture();\n")
		g.scopeDown(f)
		f.WriteString("\n")
	}
	g.scopeDown(f)
	f.WriteString("\n")
}

// generateServiceAsyncClient is generate_service_async_client.
func (g *Generator) generateServiceAsyncClient(sv *sema.Service) {
	f := &g.fService
	extendsClient := "org.apache.thrift.async.TAsyncClient"
	if sv.Extends() != nil {
		extendsClient = g.tn(sv.Extends()) + ".AsyncClient"
	}

	f.WriteString(g.indent() + "public static class AsyncClient extends " + extendsClient + " implements AsyncIface {\n")
	g.indentUp()

	f.WriteString(g.indent() + "public static class Factory implements org.apache.thrift.async.TAsyncClientFactory<AsyncClient> {\n")
	f.WriteString(g.indent() + "  private org.apache.thrift.async.TAsyncClientManager clientManager;\n")
	f.WriteString(g.indent() + "  private org.apache.thrift.protocol.TProtocolFactory protocolFactory;\n")
	f.WriteString(g.indent() + "  public Factory(org.apache.thrift.async.TAsyncClientManager clientManager, org.apache.thrift.protocol.TProtocolFactory protocolFactory) {\n")
	f.WriteString(g.indent() + "    this.clientManager = clientManager;\n")
	f.WriteString(g.indent() + "    this.protocolFactory = protocolFactory;\n")
	f.WriteString(g.indent() + "  }\n")
	f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	f.WriteString(g.indent() + "  public AsyncClient getAsyncClient(org.apache.thrift.transport.TNonblockingTransport transport) {\n")
	f.WriteString(g.indent() + "    return new AsyncClient(protocolFactory, clientManager, transport);\n")
	f.WriteString(g.indent() + "  }\n")
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + "public AsyncClient(org.apache.thrift.protocol.TProtocolFactory protocolFactory, org.apache.thrift.async.TAsyncClientManager clientManager, org.apache.thrift.transport.TNonblockingTransport transport) {\n")
	f.WriteString(g.indent() + "  super(protocolFactory, clientManager, transport);\n")
	f.WriteString(g.indent() + "}\n\n")

	for _, fn := range sv.Functions() {
		funname := fn.Name()
		sep := "_"
		javaname := funname
		if g.opts.FullcamelStyle {
			sep = ""
			javaname = asCamelCase(javaname, true)
		}
		retType := fn.ReturnType()
		argStruct := fn.Arglist()
		funclassname := funname + "_call"
		fields := argStruct.Members()
		xceptions := fn.Xceptions().Members()
		argsName := fn.Name() + "_args"

		f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
		f.WriteString(g.indent() + "public " + g.functionSignatureAsync(fn, false, "") + " throws org.apache.thrift.TException {\n")
		f.WriteString(g.indent() + "  checkReady();\n")
		f.WriteString(g.indent() + "  " + funclassname + " method_call = new " + funclassname + "(" +
			g.asyncArgumentList(fn, argStruct, false) + ", this, ___protocolFactory, ___transport);\n")
		f.WriteString(g.indent() + "  this.___currentMethod = method_call;\n")
		f.WriteString(g.indent() + "  ___manager.call(method_call);\n")
		f.WriteString(g.indent() + "}\n")

		f.WriteString("\n")

		f.WriteString(g.indent() + "public static class " + funclassname + " extends org.apache.thrift.async.TAsyncMethodCall<" +
			g.typeName(fn.ReturnType(), true, false, false, false) + "> {\n")
		g.indentUp()

		for _, fld := range fields {
			f.WriteString(g.indent() + "private " + g.tn(fld.Type()) + " " + makeValidJavaIdentifier(fld.Name()) + ";\n")
		}

		oneway := "false"
		if fn.IsOneway() {
			oneway = "true"
		}
		f.WriteString(g.indent() + "public " + funclassname + "(" + g.asyncArgumentList(fn, argStruct, true) +
			", org.apache.thrift.async.TAsyncClient client, org.apache.thrift.protocol.TProtocolFactory protocolFactory, org.apache.thrift.transport.TNonblockingTransport transport) throws org.apache.thrift.TException {\n")
		f.WriteString(g.indent() + "  super(client, protocolFactory, transport, resultHandler, " + oneway + ");\n")

		for _, fld := range fields {
			id := makeValidJavaIdentifier(fld.Name())
			f.WriteString(g.indent() + "  this." + id + " = " + id + ";\n")
		}

		f.WriteString(g.indent() + "}\n\n")
		f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
		f.WriteString(g.indent() + "public void write_args(org.apache.thrift.protocol.TProtocol prot) throws org.apache.thrift.TException {\n")
		g.indentUp()

		msgType := "TMessageType.CALL"
		if fn.IsOneway() {
			msgType = "TMessageType.ONEWAY"
		}
		f.WriteString(g.indent() + "prot.writeMessageBegin(new org.apache.thrift.protocol.TMessage(\"" + funname +
			"\", org.apache.thrift.protocol." + msgType + ", 0));\n" +
			g.indent() + argsName + " args = new " + argsName + "();\n")

		for _, fld := range fields {
			f.WriteString(g.indent() + "args.set" + g.capName(fld.Name()) + "(" + makeValidJavaIdentifier(fld.Name()) + ");\n")
		}

		f.WriteString(g.indent() + "args.write(prot);\n" +
			g.indent() + "prot.writeMessageEnd();\n")

		g.indentDown()
		f.WriteString(g.indent() + "}\n\n")

		f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
		f.WriteString(g.indent() + "public " + g.typeName(retType, true, false, false, false) + " getResult() throws ")
		for _, x := range xceptions {
			f.WriteString(g.typeName(x.Type(), false, false, false, false) + ", ")
		}
		f.WriteString("org.apache.thrift.TException {\n")

		g.indentUp()
		f.WriteString(g.indent() + "if (getState() != org.apache.thrift.async.TAsyncMethodCall.State.RESPONSE_READ) {\n" +
			g.indent() + "  throw new java.lang.IllegalStateException(\"Method call not finished!\");\n" +
			g.indent() + "}\n" +
			g.indent() + "org.apache.thrift.transport.TMemoryInputTransport memoryTransport = new org.apache.thrift.transport.TMemoryInputTransport(getFrameBuffer().array());\n" +
			g.indent() + "org.apache.thrift.protocol.TProtocol prot = client.getProtocolFactory().getProtocol(memoryTransport);\n")
		f.WriteString(g.indent())
		if retType.IsVoid() {
			if !fn.IsOneway() {
				f.WriteString("(new Client(prot)).recv" + sep + javaname + "();\n")
				f.WriteString(g.indent())
			}
			f.WriteString("return null;\n")
		} else {
			f.WriteString("return (new Client(prot)).recv" + sep + javaname + "();\n")
		}

		g.indentDown()
		f.WriteString(g.indent() + "}\n")

		g.indentDown()
		f.WriteString(g.indent() + "}\n\n")
	}

	g.scopeDown(f)
	f.WriteString("\n")
}

// generateServiceServer is generate_service_server.
func (g *Generator) generateServiceServer(sv *sema.Service) {
	f := &g.fService
	functions := sv.Functions()

	extendsProcessor := ""
	if sv.Extends() == nil {
		extendsProcessor = "org.apache.thrift.TBaseProcessor<I>"
	} else {
		extendsProcessor = g.tn(sv.Extends()) + ".Processor<I>"
	}

	f.WriteString(g.indent() + "public static class Processor<I extends Iface> extends " + extendsProcessor + " implements org.apache.thrift.TProcessor {\n")
	g.indentUp()

	f.WriteString(g.indent() + "private static final org.slf4j.Logger _LOGGER = org.slf4j.LoggerFactory.getLogger(Processor.class.getName());\n")

	f.WriteString(g.indent() + "public Processor(I iface) {\n")
	f.WriteString(g.indent() + "  super(iface, getProcessMap(new java.util.HashMap<java.lang.String, org.apache.thrift.ProcessFunction<I, ? extends org.apache.thrift.TBase, ? extends org.apache.thrift.TBase>>()));\n")
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + "protected Processor(I iface, java.util.Map<java.lang.String, org.apache.thrift.ProcessFunction<I, ? extends org.apache.thrift.TBase, ? extends org.apache.thrift.TBase>> processMap) {\n")
	f.WriteString(g.indent() + "  super(iface, getProcessMap(processMap));\n")
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + "private static <I extends Iface> java.util.Map<java.lang.String, org.apache.thrift.ProcessFunction<I, ? extends org.apache.thrift.TBase, ? extends org.apache.thrift.TBase>> getProcessMap(java.util.Map<java.lang.String, org.apache.thrift.ProcessFunction<I, ? extends  org.apache.thrift.TBase, ? extends org.apache.thrift.TBase>> processMap) {\n")
	g.indentUp()
	for _, fn := range functions {
		f.WriteString(g.indent() + "processMap.put(\"" + fn.Name() + "\", new " + makeValidJavaIdentifier(fn.Name()) + "());\n")
	}
	f.WriteString(g.indent() + "return processMap;\n")
	g.indentDown()
	f.WriteString(g.indent() + "}\n\n")

	for _, fn := range functions {
		g.generateProcessFunction(sv, fn)
	}

	g.indentDown()
	f.WriteString(g.indent() + "}\n\n")
}

// generateServiceAsyncServer is generate_service_async_server.
func (g *Generator) generateServiceAsyncServer(sv *sema.Service) {
	f := &g.fService
	functions := sv.Functions()

	extendsProcessor := ""
	if sv.Extends() == nil {
		extendsProcessor = "org.apache.thrift.TBaseAsyncProcessor<I>"
	} else {
		extendsProcessor = g.tn(sv.Extends()) + ".AsyncProcessor<I>"
	}

	f.WriteString(g.indent() + "public static class AsyncProcessor<I extends AsyncIface> extends " + extendsProcessor + " {\n")
	g.indentUp()

	f.WriteString(g.indent() + "private static final org.slf4j.Logger _LOGGER = org.slf4j.LoggerFactory.getLogger(AsyncProcessor.class.getName());\n")

	f.WriteString(g.indent() + "public AsyncProcessor(I iface) {\n")
	f.WriteString(g.indent() + "  super(iface, getProcessMap(new java.util.HashMap<java.lang.String, org.apache.thrift.AsyncProcessFunction<I, ? extends org.apache.thrift.TBase, ?, ? extends org.apache.thrift.TBase>>()));\n")
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + "protected AsyncProcessor(I iface, java.util.Map<java.lang.String,  org.apache.thrift.AsyncProcessFunction<I, ? extends  org.apache.thrift.TBase, ?, ? extends org.apache.thrift.TBase>> processMap) {\n")
	f.WriteString(g.indent() + "  super(iface, getProcessMap(processMap));\n")
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + "private static <I extends AsyncIface> java.util.Map<java.lang.String,  org.apache.thrift.AsyncProcessFunction<I, ? extends  org.apache.thrift.TBase, ?, ? extends org.apache.thrift.TBase>> getProcessMap(java.util.Map<java.lang.String,  org.apache.thrift.AsyncProcessFunction<I, ? extends  org.apache.thrift.TBase, ?, ? extends org.apache.thrift.TBase>> processMap) {\n")
	g.indentUp()
	for _, fn := range functions {
		f.WriteString(g.indent() + "processMap.put(\"" + fn.Name() + "\", new " + makeValidJavaIdentifier(fn.Name()) + "());\n")
	}
	f.WriteString(g.indent() + "return processMap;\n")
	g.indentDown()
	f.WriteString(g.indent() + "}\n\n")

	for _, fn := range functions {
		g.generateProcessAsyncFunction(sv, fn)
	}

	g.indentDown()
	f.WriteString(g.indent() + "}\n\n")
}

// generateFunctionHelpers is generate_function_helpers: the result struct.
func (g *Generator) generateFunctionHelpers(fn *sema.Function) {
	if fn.IsOneway() {
		return
	}

	result := sema.NewStruct(g.program)
	result.SetName(fn.Name() + "_result")
	if !fn.ReturnType().IsVoid() {
		result.Append(sema.NewField(fn.ReturnType(), "success", 0))
	}

	for _, x := range fn.Xceptions().Members() {
		result.Append(x)
	}

	g.generateJavaStructDefinition(&g.fService, result, false, true, true)
}

// generateProcessAsyncFunction is generate_process_async_function.
func (g *Generator) generateProcessAsyncFunction(sv *sema.Service, fn *sema.Function) {
	_ = sv
	f := &g.fService
	argsname := fn.Name() + "_args"

	resultname := fn.Name() + "_result"
	if fn.IsOneway() {
		resultname = "org.apache.thrift.TBase"
	}

	resulttype := g.typeName(fn.ReturnType(), true, false, false, false)
	id := makeValidJavaIdentifier(fn.Name())

	f.WriteString(g.indent() + "public static class " + id +
		"<I extends AsyncIface> extends org.apache.thrift.AsyncProcessFunction<I, " + argsname + ", " + resulttype + ", " + resultname + "> {\n")
	g.indentUp()

	f.WriteString(g.indent() + "public " + id + "() {\n")
	f.WriteString(g.indent() + "  super(\"" + fn.Name() + "\");\n")
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	f.WriteString(g.indent() + "public " + resultname + " getEmptyResultInstance() {\n")
	if fn.IsOneway() {
		f.WriteString(g.indent() + "  return null;\n")
	} else {
		f.WriteString(g.indent() + "  return new " + resultname + "();\n")
	}
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	f.WriteString(g.indent() + "public " + argsname + " getEmptyArgsInstance() {\n")
	f.WriteString(g.indent() + "  return new " + argsname + "();\n")
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	f.WriteString(g.indent() + "public org.apache.thrift.async.AsyncMethodCallback<" + resulttype +
		"> getResultHandler(final org.apache.thrift.server.AbstractNonblockingServer.AsyncFrameBuffer fb, final int seqid) {\n")
	g.indentUp()
	f.WriteString(g.indent() + "final org.apache.thrift.AsyncProcessFunction fcall = this;\n")
	f.WriteString(g.indent() + "return new org.apache.thrift.async.AsyncMethodCallback<" + resulttype + ">() { \n")
	g.indentUp()
	f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	f.WriteString(g.indent() + "public void onComplete(" + resulttype + " o) {\n")

	g.indentUp()
	if !fn.IsOneway() {
		f.WriteString(g.indent() + resultname + " result = new " + resultname + "();\n")

		if !fn.ReturnType().IsVoid() {
			f.WriteString(g.indent() + "result.success = o;\n")
			if !typeCanBeNull(fn.ReturnType()) {
				f.WriteString(g.indent() + "result.set" + g.capName("success") + g.capName("isSet") + "(true);\n")
			}
		}

		f.WriteString(g.indent() + "try {\n")
		f.WriteString(g.indent() + "  fcall.sendResponse(fb, result, org.apache.thrift.protocol.TMessageType.REPLY,seqid);\n")
		f.WriteString(g.indent() + "} catch (org.apache.thrift.transport.TTransportException e) {\n")
		g.indentUp()
		f.WriteString(g.indent() + "_LOGGER.error(\"TTransportException writing to internal frame buffer\", e);\n" +
			g.indent() + "fb.close();\n")
		g.indentDown()
		f.WriteString(g.indent() + "} catch (java.lang.Exception e) {\n")
		g.indentUp()
		f.WriteString(g.indent() + "_LOGGER.error(\"Exception writing to internal frame buffer\", e);\n" +
			g.indent() + "onError(e);\n")
		g.indentDown()
		f.WriteString(g.indent() + "}\n")
	}
	g.indentDown()
	f.WriteString(g.indent() + "}\n")

	f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	f.WriteString(g.indent() + "public void onError(java.lang.Exception e) {\n")
	g.indentUp()

	if fn.IsOneway() {
		f.WriteString(g.indent() + "if (e instanceof org.apache.thrift.transport.TTransportException) {\n")
		g.indentUp()

		f.WriteString(g.indent() + "_LOGGER.error(\"TTransportException inside handler\", e);\n" +
			g.indent() + "fb.close();\n")

		g.indentDown()
		f.WriteString(g.indent() + "} else {\n")
		g.indentUp()

		f.WriteString(g.indent() + "_LOGGER.error(\"Exception inside oneway handler\", e);\n")

		g.indentDown()
		f.WriteString(g.indent() + "}\n")
	} else {
		f.WriteString(g.indent() + "byte msgType = org.apache.thrift.protocol.TMessageType.REPLY;\n")
		f.WriteString(g.indent() + "org.apache.thrift.TSerializable msg;\n")
		f.WriteString(g.indent() + resultname + " result = new " + resultname + "();\n")

		xceptions := fn.Xceptions().Members()
		if len(xceptions) > 0 {
			for i, x := range xceptions {
				if i == 0 {
					f.WriteString(g.indent())
				}
				typ := g.typeName(x.Type(), false, false, false, false)
				name := x.Name()
				f.WriteString("if (e instanceof " + typ + ") {\n")
				g.indentUp()
				f.WriteString(g.indent() + "result." + makeValidJavaIdentifier(name) + " = (" + typ + ") e;\n" +
					g.indent() + "result.set" + g.capName(name) + g.capName("isSet") + "(true);\n" +
					g.indent() + "msg = result;\n")
				g.indentDown()
				f.WriteString(g.indent() + "} else ")
			}
		} else {
			f.WriteString(g.indent())
		}
		f.WriteString("if (e instanceof org.apache.thrift.transport.TTransportException) {\n")
		g.indentUp()
		f.WriteString(g.indent() + "_LOGGER.error(\"TTransportException inside handler\", e);\n" +
			g.indent() + "fb.close();\n" +
			g.indent() + "return;\n")
		g.indentDown()
		f.WriteString(g.indent() + "} else if (e instanceof org.apache.thrift.TApplicationException) {\n")
		g.indentUp()
		f.WriteString(g.indent() + "_LOGGER.error(\"TApplicationException inside handler\", e);\n" +
			g.indent() + "msgType = org.apache.thrift.protocol.TMessageType.EXCEPTION;\n" +
			g.indent() + "msg = (org.apache.thrift.TApplicationException)e;\n")
		g.indentDown()
		f.WriteString(g.indent() + "} else {\n")
		g.indentUp()
		f.WriteString(g.indent() + "_LOGGER.error(\"Exception inside handler\", e);\n" +
			g.indent() + "msgType = org.apache.thrift.protocol.TMessageType.EXCEPTION;\n" +
			g.indent() + "msg = new org.apache.thrift.TApplicationException(org.apache.thrift.TApplicationException.INTERNAL_ERROR, e.getMessage());\n")
		g.indentDown()
		f.WriteString(g.indent() + "}\n" +
			g.indent() + "try {\n" +
			g.indent() + "  fcall.sendResponse(fb,msg,msgType,seqid);\n" +
			g.indent() + "} catch (java.lang.Exception ex) {\n" +
			g.indent() + "  _LOGGER.error(\"Exception writing to internal frame buffer\", ex);\n" +
			g.indent() + "  fb.close();\n" +
			g.indent() + "}\n")
	}
	g.indentDown()
	f.WriteString(g.indent() + "}\n")
	g.indentDown()
	f.WriteString(g.indent() + "};\n")
	g.indentDown()
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	f.WriteString(g.indent() + "public boolean isOneway() {\n")
	oneway := "false"
	if fn.IsOneway() {
		oneway = "true"
	}
	f.WriteString(g.indent() + "  return " + oneway + ";\n")
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	f.WriteString(g.indent() + "public void start(I iface, " + argsname + " args, org.apache.thrift.async.AsyncMethodCallback<" + resulttype + "> resultHandler) throws org.apache.thrift.TException {\n")
	g.indentUp()

	fields := fn.Arglist().Members()
	f.WriteString(g.indent())

	f.WriteString("iface." + g.rpcMethodName(fn.Name()) + "(")
	first := true
	for _, fld := range fields {
		if first {
			first = false
		} else {
			f.WriteString(", ")
		}
		f.WriteString("args." + makeValidJavaIdentifier(fld.Name()))
	}
	if !first {
		f.WriteString(",")
	}
	f.WriteString("resultHandler")
	f.WriteString(");\n")

	g.indentDown()
	f.WriteString(g.indent() + "}")

	f.WriteString("\n")

	g.indentDown()
	f.WriteString(g.indent() + "}\n\n")
}

// generateProcessFunction is generate_process_function.
func (g *Generator) generateProcessFunction(sv *sema.Service, fn *sema.Function) {
	_ = sv
	f := &g.fService
	argsname := fn.Name() + "_args"
	resultname := fn.Name() + "_result"
	if fn.IsOneway() {
		resultname = "org.apache.thrift.TBase"
	}
	id := makeValidJavaIdentifier(fn.Name())

	f.WriteString(g.indent() + "public static class " + id +
		"<I extends Iface> extends org.apache.thrift.ProcessFunction<I, " + argsname + ", " + resultname + "> {\n")
	g.indentUp()

	f.WriteString(g.indent() + "public " + id + "() {\n")
	f.WriteString(g.indent() + "  super(\"" + fn.Name() + "\");\n")
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	f.WriteString(g.indent() + "public " + argsname + " getEmptyArgsInstance() {\n")
	f.WriteString(g.indent() + "  return new " + argsname + "();\n")
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	f.WriteString(g.indent() + "public boolean isOneway() {\n")
	oneway := "false"
	if fn.IsOneway() {
		oneway = "true"
	}
	f.WriteString(g.indent() + "  return " + oneway + ";\n")
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	f.WriteString(g.indent() + "protected boolean rethrowUnhandledExceptions() {\n")
	rethrow := "false"
	if g.opts.RethrowUnhandledExceptions {
		rethrow = "true"
	}
	f.WriteString(g.indent() + "  return " + rethrow + ";\n")
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	f.WriteString(g.indent() + "public " + resultname + " getEmptyResultInstance() {\n")
	if fn.IsOneway() {
		f.WriteString(g.indent() + "  return null;\n")
	} else {
		f.WriteString(g.indent() + "  return new " + resultname + "();\n")
	}
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	f.WriteString(g.indent() + "public " + resultname + " getResult(I iface, " + argsname + " args) throws org.apache.thrift.TException {\n")
	g.indentUp()
	if !fn.IsOneway() {
		f.WriteString(g.indent() + resultname + " result = getEmptyResultInstance();\n")
	}

	xceptions := fn.Xceptions().Members()

	if len(xceptions) > 0 {
		f.WriteString(g.indent() + "try {\n")
		g.indentUp()
	}

	fields := fn.Arglist().Members()
	f.WriteString(g.indent())

	if !fn.IsOneway() && !fn.ReturnType().IsVoid() {
		f.WriteString("result.success = ")
	}
	f.WriteString("iface." + g.rpcMethodName(fn.Name()) + "(")
	first := true
	for _, fld := range fields {
		if first {
			first = false
		} else {
			f.WriteString(", ")
		}
		f.WriteString("args." + makeValidJavaIdentifier(fld.Name()))
	}
	f.WriteString(");\n")

	if !fn.IsOneway() && !fn.ReturnType().IsVoid() && !typeCanBeNull(fn.ReturnType()) {
		f.WriteString(g.indent() + "result.set" + g.capName("success") + g.capName("isSet") + "(true);\n")
	}

	if !fn.IsOneway() && len(xceptions) > 0 {
		g.indentDown()
		f.WriteString(g.indent() + "}")
		for _, x := range xceptions {
			xid := makeValidJavaIdentifier(x.Name())
			f.WriteString(" catch (" + g.typeName(x.Type(), false, false, false, false) + " " + xid + ") {\n")
			if !fn.IsOneway() {
				g.indentUp()
				f.WriteString(g.indent() + "result." + xid + " = " + xid + ";\n")
				g.indentDown()
				f.WriteString(g.indent() + "}")
			} else {
				f.WriteString("}")
			}
		}
		f.WriteString("\n")
	}

	if fn.IsOneway() {
		f.WriteString(g.indent() + "return null;\n")
	} else {
		f.WriteString(g.indent() + "return result;\n")
	}
	g.indentDown()
	f.WriteString(g.indent() + "}")

	f.WriteString("\n")

	g.indentDown()
	f.WriteString(g.indent() + "}\n\n")
}
