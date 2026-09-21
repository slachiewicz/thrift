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

package haxe

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateService is t_haxe_generator::generate_service. In C++ this
// comprises an entirely separate header and source file. The header file
// defines the methods and includes the data types defined in the main
// header file, and the implementation file contains implementations of the
// basic printer and default interfaces.
func (g *generator) generateService(tservice *sema.Service) {
	// Make service interface file with only "normal" calls
	fServiceName := g.packageDir + "/" + g.makeHaxeUserTypeName(g.serviceName) + "_service.hx"
	g.fService = &strings.Builder{}

	g.fService.WriteString(g.autogenComment() + g.haxePackage() + ";\n")

	g.fService.WriteString("\n" + g.haxeTypeImports() + g.haxeThriftImports() + g.haxeThriftGenImportsService(tservice))

	if tservice.Extends() != nil {
		parent := sema.Type(tservice.Extends())
		parentNamespace := makePackageName(parent.Program().Namespace("haxe"))
		if parentNamespace != "" && parentNamespace != g.packageName {
			g.fService.WriteString("import " + getCapName(g.typeName(parent)) + "_service;\n")
		}
	}

	g.fService.WriteString("\n")

	g.generateServiceInterface(tservice, false)
	emit.WriteFile(fServiceName, g.fService.String())

	// Client interface file with dual support ("normal" and "callback" style)
	fServiceName = g.packageDir + "/" + g.makeHaxeUserTypeName(g.serviceName) + ".hx"
	g.fService = &strings.Builder{}

	g.fService.WriteString(g.autogenComment() + g.haxePackage() + ";\n")

	g.fService.WriteString("\n" + g.haxeTypeImports() + g.haxeThriftImports() + g.haxeThriftGenImportsService(tservice))

	if tservice.Extends() != nil {
		parent := sema.Type(tservice.Extends())
		parentNamespace := makePackageName(parent.Program().Namespace("haxe"))
		if parentNamespace != "" && parentNamespace != g.packageName {
			g.fService.WriteString("import " + getCapName(g.typeName(parent)) + ";\n")
		}
	}

	g.fService.WriteString("\n")

	g.generateServiceInterface(tservice, true)
	emit.WriteFile(fServiceName, g.fService.String())

	// Now make the implementation/client file
	fServiceName = g.packageDir + "/" + g.makeHaxeUserTypeName(g.serviceName) + "Impl.hx"
	g.fService = &strings.Builder{}

	g.fService.WriteString(g.autogenComment() + g.haxePackage() + ";\n\n" + g.haxeTypeImports() +
		g.haxeThriftImports() + g.haxeThriftGenImportsService(tservice) + "\n")

	if tservice.Extends() != nil {
		parent := sema.Type(tservice.Extends())
		parentNamespace := makePackageName(parent.Program().Namespace("haxe"))
		if parentNamespace != "" && parentNamespace != g.packageName {
			g.fService.WriteString("import " + getCapName(g.typeName(parent)) + "Impl;\n")
		}
	}

	g.fService.WriteString("\n")

	g.generateServiceClient(tservice)
	emit.WriteFile(fServiceName, g.fService.String())

	// Now make the helper class files
	g.generateServiceHelpers(tservice)

	// Now make the processor/server file
	fServiceName = g.packageDir + "/" + g.makeHaxeUserTypeName(g.serviceName) + "Processor.hx"
	g.fService = &strings.Builder{}

	g.fService.WriteString(g.autogenComment() + g.haxePackage() + ";\n" +
		"\n" +
		g.haxeTypeImports() +
		g.haxeThriftImports() +
		g.haxeThriftGenImportsService(tservice) +
		"\n")

	if g.packageName != "" {
		g.fService.WriteString("import " + g.packageName + ".*;\n")
		g.fService.WriteString("import " + g.packageName + "." + g.makeHaxeUserTypeName(g.serviceName) + "Impl;\n")
		g.fService.WriteString("\n")
	}

	g.generateServiceServer(tservice)
	emit.WriteFile(fServiceName, g.fService.String())
}

// generateServiceMethodOnsuccess is
// t_haxe_generator::generate_service_method_onsuccess: generates the code
// snippet for the onSuccess callbacks.
func (g *generator) generateServiceMethodOnsuccess(tfunction *sema.Function, asType, omitName bool) string {
	if tfunction.IsOneway() {
		return ""
	}

	name := ""
	if !omitName {
		name = "onSuccess"
		if asType {
			name += " : "
		}
	}

	if tfunction.ReturnType().IsVoid() {
		if asType {
			return name + "Void->Void = null"
		}
		return name + "() : Void"
	}

	if asType {
		return name + g.typeName(tfunction.ReturnType()) + "->Void = null"
	}
	return name + "( retval : " + g.typeName(tfunction.ReturnType()) + ")"
}

// generateServiceMethodSignature is
// t_haxe_generator::generate_service_method_signature.
func (g *generator) generateServiceMethodSignature(tfunction *sema.Function, isInterface, combined bool) {
	if combined {
		g.generateServiceMethodSignatureCombined(tfunction, isInterface)
	} else {
		g.generateServiceMethodSignatureNormal(tfunction, isInterface)
	}
}

// generateServiceMethodSignatureNormal is
// t_haxe_generator::generate_service_method_signature_normal.
func (g *generator) generateServiceMethodSignatureNormal(tfunction *sema.Function, isInterface bool) {
	if isInterface {
		g.generateDeprecationAttribute(g.fService, tfunction, true)
		g.fService.WriteString(g.indent() + g.functionSignatureNormal(tfunction) + ";\n\n")
	} else {
		g.fService.WriteString(g.indent() + "public " + g.functionSignatureNormal(tfunction) + " {\n")
	}
}

// generateServiceMethodSignatureCombined is
// t_haxe_generator::generate_service_method_signature_combined.
func (g *generator) generateServiceMethodSignatureCombined(tfunction *sema.Function, isInterface bool) {
	if !tfunction.IsOneway() {
		onSuccessImpl := g.generateServiceMethodOnsuccess(tfunction, false, false)
		g.fService.WriteString(g.indent() + "// function onError(Dynamic) : Void;\n")
		g.fService.WriteString(g.indent() + "// function " + onSuccessImpl + ";\n")
	}

	if isInterface {
		g.generateDeprecationAttribute(g.fService, tfunction, false)
		g.fService.WriteString(g.indent() + g.functionSignatureCombined(tfunction) + ";\n\n")
	} else {
		g.fService.WriteString(g.indent() + "public " + g.functionSignatureCombined(tfunction) + " {\n")
	}
}

// functionSignatureCombined is t_haxe_generator::function_signature_combined:
// renders a function signature of the form 'type name(args)'.
func (g *generator) functionSignatureCombined(tfunction *sema.Function) string {
	onErrorSuccess := "onError : Dynamic->Void = null, " + g.generateServiceMethodOnsuccess(tfunction, true, false)

	arguments := g.argumentList(tfunction.Arglist())
	if !tfunction.IsOneway() {
		if arguments != "" {
			arguments += ", "
		}
		arguments += onErrorSuccess
	}

	var resultType string
	if tfunction.IsOneway() || tfunction.ReturnType().IsVoid() {
		resultType = "Void"
	} else {
		resultType = g.typeName(tfunction.ReturnType())
	}

	return "function " + escapeHaxeKeyword(tfunction.Name()) + "(" + arguments + ") : " + resultType
}

// functionSignatureNormal is t_haxe_generator::function_signature_normal:
// renders a function signature of the form 'type name(args)'.
func (g *generator) functionSignatureNormal(tfunction *sema.Function) string {
	arguments := g.argumentList(tfunction.Arglist())

	var resultType string
	if tfunction.IsOneway() || tfunction.ReturnType().IsVoid() {
		resultType = "Void"
	} else {
		resultType = g.typeName(tfunction.ReturnType())
	}

	return "function " + escapeHaxeKeyword(tfunction.Name()) + "(" + arguments + ") : " + resultType
}

// generateServiceInterface is t_haxe_generator::generate_service_interface:
// generates a service interface definition.
func (g *generator) generateServiceInterface(tservice *sema.Service, combined bool) {
	cbkPostfix := "_service"
	if combined {
		cbkPostfix = ""
	}

	extendsIface := ""
	if tservice.Extends() != nil {
		extendsIface = " extends " + getCapName(g.typeName(tservice.Extends())) + cbkPostfix
	}

	functions := tservice.Functions()

	g.generateHaxeDoc(g.fService, tservice)
	g.generateRttiDecoration(g.fService)
	g.generateMacroDecoration(g.fService)
	g.fService.WriteString(g.indent() + "interface " + g.makeHaxeUserTypeName(g.serviceName) + cbkPostfix + extendsIface + " {\n\n")
	g.indentUp()
	for _, f := range functions {
		g.generateHaxeFunctionDoc(g.fService, f)
		g.generateServiceMethodSignature(f, true, combined)
	}
	g.indentDown()
	g.fService.WriteString(g.indent() + "}\n\n")
}

// generateServiceHelpers is t_haxe_generator::generate_service_helpers:
// generates structs for all the service args and return types. The C++
// version writes two blank lines to f_service_ first, but every byte
// written there is discarded: f_service_ is the already-closed Impl file
// stream and the next real write is a fresh .open() for the Processor
// file, which clears the buffer before anything else touches it, so
// those two newlines never reach any output file and are not reproduced.
func (g *generator) generateServiceHelpers(tservice *sema.Service) {
	functions := tservice.Functions()
	for _, f := range functions {
		ts := f.Arglist()
		g.generateHaxeStruct(ts, false, false)
		g.generateFunctionHelpers(f)
	}
}

// generateServiceClient is t_haxe_generator::generate_service_client:
// generates a service client definition.
func (g *generator) generateServiceClient(tservice *sema.Service) {
	out := g.fService

	extends := ""
	extendsClient := ""
	if tservice.Extends() != nil {
		extends = getCapName(g.typeName(tservice.Extends()))
		extendsClient = " extends " + extends + "Impl"
	}

	g.generateRttiDecoration(out)
	// build macro is inherited from interface
	out.WriteString(g.indent() + "class " + g.makeHaxeUserTypeName(g.serviceName) + "Impl" + extendsClient +
		" implements " + g.makeHaxeUserTypeName(g.serviceName) + " {\n\n")
	g.indentUp()

	out.WriteString(g.indent() + "public function new( iprot : TProtocol, oprot : TProtocol = null)\n")
	g.scopeUp(out)
	if extends == "" {
		out.WriteString(g.indent() + "iprot_ = iprot;\n")
		out.WriteString(g.indent() + "if (oprot == null) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "oprot_ = iprot;\n")
		g.indentDown()
		out.WriteString(g.indent() + "} else {\n")
		g.indentUp()
		out.WriteString(g.indent() + "oprot_ = oprot;\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	} else {
		out.WriteString(g.indent() + "super(iprot, oprot);\n")
	}
	g.scopeDown(out)
	out.WriteString("\n")

	if extends == "" {
		out.WriteString(g.indent() + "private var iprot_ : TProtocol;\n" + g.indent() +
			"private var oprot_ : TProtocol;\n" + g.indent() +
			"private var seqid_ : Int;\n\n")

		out.WriteString(g.indent() + "public function getInputProtocol() : TProtocol\n")
		g.scopeUp(out)
		out.WriteString(g.indent() + "return this.iprot_;\n")
		g.scopeDown(out)
		out.WriteString("\n")

		out.WriteString(g.indent() + "public function getOutputProtocol() : TProtocol\n")
		g.scopeUp(out)
		out.WriteString(g.indent() + "return this.oprot_;\n")
		g.scopeDown(out)
		out.WriteString("\n")
	}

	// Generate client method implementations
	functions := tservice.Functions()
	for _, fn := range functions {
		// Open function
		g.generateServiceMethodSignature(fn, false, true)

		g.indentUp()

		argStruct := fn.Arglist()
		argsname := getCapName(fn.Name() + "_args")
		fields := argStruct.Members()

		// Serialize the request
		args := g.tmp("args")
		calltype := "CALL"
		if fn.IsOneway() {
			calltype = "ONEWAY"
		}
		out.WriteString(g.indent() + "oprot_.writeMessageBegin(new TMessage(\"" + fn.Name() +
			"\", TMessageType." + calltype + ", seqid_));\n" + g.indent() +
			"var " + args + " : " + argsname + " = new " + argsname + "();\n")

		for _, fld := range fields {
			out.WriteString(g.indent() + args + "." + escapeHaxeKeyword(fld.Name()) + " = " +
				escapeHaxeKeyword(fld.Name()) + ";\n")
		}

		out.WriteString(g.indent() + args + ".write(oprot_);\n" + g.indent() +
			"oprot_.writeMessageEnd();\n")

		retval := g.tmp("retval")
		if !(fn.IsOneway() || fn.ReturnType().IsVoid()) {
			out.WriteString(g.indent() + "var " + retval + " : " + g.typeName(fn.ReturnType()) +
				" = " + g.renderDefaultValueForType(fn.ReturnType(), true) + ";\n")
		}

		if fn.IsOneway() {
			out.WriteString(g.indent() + "oprot_.getTransport().flush();\n")
		} else {
			out.WriteString(g.indent() + "oprot_.getTransport().flush(function(error:Dynamic) : Void {\n")
			g.indentUp()
			out.WriteString(g.indent() + "try {\n")
			g.indentUp()
			appex := g.tmp("appex")
			out.WriteString(g.indent() + "var " + appex + " : TApplicationException;\n")
			resultname := getCapName(fn.Name() + "_result")
			out.WriteString(g.indent() + "if (error != null) {\n")
			g.indentUp()
			out.WriteString(g.indent() + "if (onError == null)\n")
			g.indentUp()
			out.WriteString(g.indent() + "throw error;\n")
			g.indentDown()
			out.WriteString(g.indent() + "onError(error);\n")
			out.WriteString(g.indent() + "return;\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n\n")
			msg := g.tmp("msg")
			out.WriteString(g.indent() + "var " + msg + " : TMessage = iprot_.readMessageBegin();\n")
			out.WriteString(g.indent() + "if (" + msg + ".type == TMessageType.EXCEPTION) {\n")
			g.indentUp()
			out.WriteString(g.indent() + appex + " = TApplicationException.read(iprot_);\n")
			out.WriteString(g.indent() + "iprot_.readMessageEnd();\n")
			out.WriteString(g.indent() + "if (onError == null)\n")
			g.indentUp()
			out.WriteString(g.indent() + "throw " + appex + ";\n")
			g.indentDown()
			out.WriteString(g.indent() + "onError(" + appex + ");\n")
			out.WriteString(g.indent() + "return;\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n\n")
			result := g.tmp("result")
			out.WriteString(g.indent() + "var " + result + " : " + resultname + " = new " + resultname + "();\n")
			out.WriteString(g.indent() + "" + result + ".read(iprot_);\n")
			out.WriteString(g.indent() + "iprot_.readMessageEnd();\n")

			// Careful, only return _result if not a void function
			if !fn.ReturnType().IsVoid() {
				out.WriteString(g.indent() + "if (" + result + "." + generateIssetCheckName("success") + ") {\n")
				g.indentUp()
				out.WriteString(g.indent() + "if (onSuccess != null)\n")
				g.indentUp()
				out.WriteString(g.indent() + "onSuccess(" + result + ".success);\n")
				g.indentDown()
				out.WriteString(g.indent() + retval + " = " + result + ".success;\n")
				out.WriteString(g.indent() + "return;\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n\n")
			}

			xs := fn.Xceptions()
			xceptions := xs.Members()
			for _, x := range xceptions {
				xname := escapeHaxeKeyword(x.Name())
				out.WriteString(g.indent() + "if (" + result + "." + xname + " != null) {\n")
				g.indentUp()
				out.WriteString(g.indent() + "if (onError == null)\n")
				g.indentUp()
				out.WriteString(g.indent() + "throw " + result + "." + xname + ";\n")
				g.indentDown()
				out.WriteString(g.indent() + "onError(" + result + "." + xname + ");\n")
				out.WriteString(g.indent() + "return;\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n\n")
			}

			// If you get here it's an exception, unless a void function
			if fn.ReturnType().IsVoid() {
				out.WriteString(g.indent() + "if (onSuccess != null)\n")
				g.indentUp()
				out.WriteString(g.indent() + "onSuccess();\n")
				g.indentDown()
				out.WriteString(g.indent() + "return;\n")
			} else {
				out.WriteString(g.indent() + appex + " = new TApplicationException(" +
					"TApplicationException.MISSING_RESULT," +
					"\"" + fn.Name() + " failed: unknown result\");\n")
				out.WriteString(g.indent() + "if (onError == null)\n")
				g.indentUp()
				out.WriteString(g.indent() + "throw " + appex + ";\n")
				g.indentDown()
				out.WriteString(g.indent() + "onError(" + appex + ");\n")
				out.WriteString(g.indent() + "return;\n")
			}

			g.indentDown()
			out.WriteString(g.indent() + "\n")
			out.WriteString(g.indent() + "} catch( e : TException) {\n")
			g.indentUp()
			out.WriteString(g.indent() + "if (onError == null)\n")
			g.indentUp()
			out.WriteString(g.indent() + "throw e;\n")
			g.indentDown()
			out.WriteString(g.indent() + "onError(e);\n")
			out.WriteString(g.indent() + "return;\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n")

			g.indentDown()
			out.WriteString(g.indent() + "});\n\n")
		}

		if !(fn.IsOneway() || fn.ReturnType().IsVoid()) {
			out.WriteString(g.indent() + "return " + retval + ";\n")
		}

		// Close function
		g.scopeDown(out)
		out.WriteString("\n")
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

// generateServiceServer is t_haxe_generator::generate_service_server:
// generates a service server definition.
func (g *generator) generateServiceServer(tservice *sema.Service) {
	out := g.fService
	functions := tservice.Functions()

	// Extends stuff
	extends := ""
	extendsProcessor := ""
	if tservice.Extends() != nil {
		extends = getCapName(g.typeName(tservice.Extends()))
		extendsProcessor = " extends " + extends + "Processor"
	}

	// Generate the header portion
	g.generateRttiDecoration(out)
	g.generateMacroDecoration(out)
	out.WriteString(g.indent() + "class " + g.makeHaxeUserTypeName(g.serviceName) + "Processor" + extendsProcessor +
		" implements TProcessor {\n\n")
	g.indentUp()

	out.WriteString(g.indent() + "private var " + g.makeHaxeUserTypeName(g.serviceName) +
		"_iface_ : " + g.makeHaxeUserTypeName(g.serviceName) + "_service;\n")

	if extends == "" {
		out.WriteString(g.indent() +
			"private var PROCESS_MAP = new StringMap< Int->TProtocol->TProtocol->Void >();\n")
	}

	out.WriteString("\n")

	out.WriteString(g.indent() + "public function new( iface : " + g.makeHaxeUserTypeName(g.serviceName) + "_service)\n")
	g.scopeUp(out)
	if extends != "" {
		out.WriteString(g.indent() + "super(iface);\n")
	}
	out.WriteString(g.indent() + g.makeHaxeUserTypeName(g.serviceName) + "_iface_ = iface;\n")

	for _, f := range functions {
		out.WriteString(g.indent() + "PROCESS_MAP.set(\"" + f.Name() + "\", " + f.Name() + "());\n")
	}

	g.scopeDown(out)
	out.WriteString("\n")

	// Generate the server implementation
	override := ""
	if tservice.Extends() != nil {
		override = "override "
	}
	out.WriteString(g.indent() + override +
		"public function process( iprot : TProtocol, oprot : TProtocol) : Bool\n")
	g.scopeUp(out)

	out.WriteString(g.indent() + "var msg : TMessage = iprot.readMessageBegin();\n")

	// TODO(mcslee): validate message, was the seqid etc. legit?

	out.WriteString(g.indent() + "var fn  = PROCESS_MAP.get(msg.name);\n" +
		g.indent() + "if (fn == null) {\n" +
		g.indent() + "  TProtocolUtil.skip(iprot, TType.STRUCT);\n" +
		g.indent() + "  iprot.readMessageEnd();\n" +
		g.indent() + "  var appex = new TApplicationException(TApplicationException.UNKNOWN_METHOD, " +
		"\"Invalid method name: '\"+msg.name+\"'\");\n" +
		g.indent() + "  oprot.writeMessageBegin(new TMessage(msg.name, TMessageType.EXCEPTION, msg.seqid));\n" +
		g.indent() + "  appex.write(oprot);\n" + g.indent() + "  oprot.writeMessageEnd();\n" +
		g.indent() + "  oprot.getTransport().flush();\n" +
		g.indent() + "  return true;\n" + g.indent() + "}\n" +
		g.indent() + "fn( msg.seqid, iprot, oprot);\n")

	out.WriteString(g.indent() + "return true;\n")

	g.scopeDown(out)
	out.WriteString("\n")

	// Generate the process subfunctions
	for _, f := range functions {
		g.generateProcessFunction(tservice, f)
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateFunctionHelpers is t_haxe_generator::generate_function_helpers:
// generates a struct and helpers for a function.
func (g *generator) generateFunctionHelpers(tfunction *sema.Function) {
	if tfunction.IsOneway() {
		return
	}

	resultName := getCapName(tfunction.Name() + "_result")
	result := sema.NewStruct(g.program)
	result.SetName(resultName)
	if !tfunction.ReturnType().IsVoid() {
		success := sema.NewField(tfunction.ReturnType(), "success", 0)
		result.Append(success)
	}

	xs := tfunction.Xceptions()
	for _, f := range xs.Members() {
		result.Append(f)
	}

	g.generateHaxeStruct(result, false, true)
}

// generateProcessFunction is t_haxe_generator::generate_process_function:
// generates a process function definition.
func (g *generator) generateProcessFunction(tservice *sema.Service, tfunction *sema.Function) {
	out := g.fService

	// Open class
	out.WriteString(g.indent() + "private function " + tfunction.Name() +
		"() : Int->TProtocol->TProtocol->Void {\n")
	g.indentUp()

	// Open function
	out.WriteString(g.indent() + "return function( seqid : Int, iprot : TProtocol, oprot : TProtocol) : Void\n")
	g.scopeUp(out)

	argsname := getCapName(tfunction.Name() + "_args")
	resultname := getCapName(tfunction.Name() + "_result")

	out.WriteString(g.indent() + "var args : " + argsname + " = new " + argsname + "();\n" +
		g.indent() + "args.read(iprot);\n" + g.indent() + "iprot.readMessageEnd();\n")

	xs := tfunction.Xceptions()
	xceptions := xs.Members()

	// Declare result for non oneway function
	if !tfunction.IsOneway() {
		out.WriteString(g.indent() + "var result : " + resultname + " = new " + resultname + "();\n")
	}

	// Try block for any function to catch (defined or undefined) exceptions
	out.WriteString(g.indent() + "try {\n")
	g.indentUp()

	// normal function():result style

	// Generate the function call
	argStruct := tfunction.Arglist()
	fields := argStruct.Members()

	out.WriteString(g.indent())
	if !(tfunction.IsOneway() || tfunction.ReturnType().IsVoid()) {
		out.WriteString("result.success = ")
	}
	out.WriteString(g.makeHaxeUserTypeName(g.serviceName) + "_iface_." + escapeHaxeKeyword(tfunction.Name()) + "(")
	first := true
	for _, f := range fields {
		if first {
			first = false
		} else {
			out.WriteString(", ")
		}
		out.WriteString("args." + escapeHaxeKeyword(f.Name()))
	}
	out.WriteString(");\n")

	g.indentDown()
	out.WriteString(g.indent() + "}")
	if !tfunction.IsOneway() {
		// catch exceptions defined in the IDL
		for _, x := range xceptions {
			xname := escapeHaxeKeyword(x.Name())
			out.WriteString(" catch (" + xname + ":" + getCapName(g.typeName(x.Type())) + ") {\n")
			g.indentUp()
			out.WriteString(g.indent() + "result." + xname + " = " + xname + ";\n")
			g.indentDown()
			out.WriteString(g.indent() + "}")
		}
	}

	// always catch all exceptions to prevent from service denial
	appex := g.tmp("appex")
	_ = appex // the C++ generator computes this tmp name but never uses it; only the counter tick matters for parity
	out.WriteString(" catch (th : Dynamic) {\n")
	g.indentUp()
	out.WriteString(g.indent() + "trace(\"Internal error processing " + tfunction.Name() + "\", th);\n")
	if !tfunction.IsOneway() {
		out.WriteString(g.indent() + "var appex = new TApplicationException(TApplicationException.INTERNAL_ERROR, " +
			"\"Internal error processing " + tfunction.Name() + "\");\n")
		out.WriteString(g.indent() + "oprot.writeMessageBegin(new TMessage(\"" + tfunction.Name() +
			"\", TMessageType.EXCEPTION, seqid));\n")
		out.WriteString(g.indent() + "appex.write(oprot);\n")
		out.WriteString(g.indent() + "oprot.writeMessageEnd();\n")
		out.WriteString(g.indent() + "oprot.getTransport().flush();\n")
	}
	out.WriteString(g.indent() + "return;\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	// Shortcut out here for oneway functions
	if tfunction.IsOneway() {
		out.WriteString(g.indent() + "return;\n")
		g.scopeDown(out)

		// Close class
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
		return
	}

	out.WriteString(g.indent() + "oprot.writeMessageBegin(new TMessage(\"" + tfunction.Name() +
		"\", TMessageType.REPLY, seqid));\n" + g.indent() + "result.write(oprot);\n" +
		g.indent() + "oprot.writeMessageEnd();\n" + g.indent() +
		"oprot.getTransport().flush();\n")

	// Close function
	g.scopeDown(out)
	out.WriteString("\n")

	// Close class
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}
