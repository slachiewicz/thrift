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

package cpp

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// newSimpleFunction builds a t_function used only to render a signature
// (a "send_"/"recv_" helper, a "noargs" call, and so on), the way the C++
// generator constructs throwaway t_function objects with the three-argument
// constructor: no exceptions, not oneway.
func newSimpleFunction(program *sema.Program, returnType sema.Type, name string, arglist *sema.Struct) *sema.Function {
	return sema.NewFunction(returnType, name, arglist, sema.NewStruct(program), false, nil)
}

// functionSignature is t_cpp_generator::function_signature: renders a
// function signature of the form 'type name(args)'.
func (g *Generator) functionSignature(f *sema.Function, style, prefix string, nameParams bool) string {
	ttype := f.ReturnType()
	arglist := f.Arglist()
	hasXceptions := len(f.Xceptions().Members()) != 0

	if style == "" {
		if g.isComplexType(ttype) {
			retSuffix := "& _return"
			if !nameParams {
				retSuffix = "& /* _return */"
			}
			return "void " + prefix + f.Name() + "(" + g.typeName(ttype, false, false) + retSuffix +
				g.argumentList(arglist, nameParams, true) + ")"
		}
		return g.typeName(ttype, false, false) + " " + prefix + f.Name() + "(" + g.argumentList(arglist, nameParams, false) + ")"
	} else if strings.HasPrefix(style, "Cob") {
		var cobType, exnCob string
		if style == "CobCl" {
			cobType = "(" + g.serviceName + "CobClient"
			if g.opts.Templates {
				cobType += "T<Protocol_>"
			}
			cobType += "* client)"
		} else if style == "CobSv" {
			if ttype.IsVoid() {
				cobType = "()"
			} else {
				cobType = "(" + g.typeName(ttype, false, false) + " const& _return)"
			}
			if hasXceptions {
				exnCob = ", ::std::function<void(::apache::thrift::TDelayedException* _throw)> /* exn_cob */"
			}
		} else {
			emit.Throw("UNKNOWN STYLE")
		}
		return "void " + prefix + f.Name() + "(::std::function<void" + cobType + "> cob" + exnCob +
			g.argumentList(arglist, nameParams, true) + ")"
	}
	emit.Throw("UNKNOWN STYLE")
	return ""
}

// argumentList is t_cpp_generator::argument_list: a comma separated list
// of all field names in the struct.
func (g *Generator) argumentList(s *sema.Struct, nameParams, startComma bool) string {
	result := ""
	first := !startComma
	for _, f := range s.Members() {
		if first {
			first = false
		} else {
			result += ", "
		}
		name := f.Name()
		if !nameParams {
			name = "/* " + f.Name() + " */"
		}
		result += g.typeName(f.Type(), false, true) + " " + name
	}
	return result
}

// generateFunctionCall is t_cpp_generator::generate_function_call.
func (g *Generator) generateFunctionCall(out *strings.Builder, f *sema.Function, target, iface, argPrefix string) {
	first := true
	retType := sema.TrueType(f.ReturnType())
	out.WriteString(g.indent())
	if !f.IsOneway() && !retType.IsVoid() {
		if g.isComplexType(retType) {
			first = false
			out.WriteString(iface + "->" + f.Name() + "(" + target)
		} else {
			out.WriteString(target + " = " + iface + "->" + f.Name() + "(")
		}
	} else {
		out.WriteString(iface + "->" + f.Name() + "(")
	}
	for _, a := range f.Arglist().Members() {
		if first {
			first = false
		} else {
			out.WriteString(", ")
		}
		out.WriteString(argPrefix + a.Name())
	}
	out.WriteString(");\n")
}

// generateService is t_cpp_generator::generate_service. In C++, this
// produces an entirely separate header and source file: the header defines
// the methods and includes the data types from the main header, and the
// implementation file contains the basic printer and default interfaces.
func (g *Generator) generateService(s *sema.Service) {
	svcname := s.Name()

	var fHeader, fService, fServiceTcc strings.Builder
	g.fHeader = &fHeader
	g.fService = &fService
	g.fServiceTcc = &fServiceTcc
	g.serviceTccOpened = g.opts.Templates

	fHeader.WriteString(g.autogenComment())
	fHeader.WriteString("#ifndef " + svcname + "_H\n#define " + svcname + "_H\n\n")
	if g.opts.CobStyle {
		fHeader.WriteString("#include <thrift/transport/TBufferTransports.h>\n" +
			"#include <functional>\n" +
			"namespace apache { namespace thrift { namespace async {\n" +
			"class TAsyncChannel;\n}}}\n")
	}
	fHeader.WriteString("#include <thrift/TDispatchProcessor.h>\n")
	if g.opts.CobStyle {
		fHeader.WriteString("#include <thrift/async/TAsyncDispatchProcessor.h>\n")
	}
	fHeader.WriteString("#include <thrift/async/TConcurrentClientSyncInfo.h>\n")
	fHeader.WriteString("#include <memory>\n")
	fHeader.WriteString("#include \"" + g.getIncludePrefix(g.program) + g.programName + "_types.h\"\n")

	if extends := s.Extends(); extends != nil {
		fHeader.WriteString("#include \"" + g.getIncludePrefix(extends.Program()) + extends.Name() + ".h\"\n")
	}

	fHeader.WriteString("\n" + g.nsOpen + "\n\n")

	fHeader.WriteString("#ifdef _MSC_VER\n" +
		"  #pragma warning( push )\n" +
		"  #pragma warning (disable : 4250 ) //inheriting methods via dominance \n" +
		"#endif\n\n")

	fService.WriteString(g.autogenComment())
	fService.WriteString("#include \"" + g.getIncludePrefix(g.program) + svcname + ".h\"\n")
	if g.opts.CobStyle {
		fService.WriteString("#include \"thrift/async/TAsyncChannel.h\"\n")
	}
	if g.opts.Templates {
		fService.WriteString("#include \"" + g.getIncludePrefix(g.program) + svcname + ".tcc\"\n")

		fServiceTcc.WriteString(g.autogenComment())
		fServiceTcc.WriteString("#include \"" + g.getIncludePrefix(g.program) + svcname + ".h\"\n")

		fServiceTcc.WriteString("#ifndef " + svcname + "_TCC\n#define " + svcname + "_TCC\n\n")

		if g.opts.CobStyle {
			fServiceTcc.WriteString("#include \"thrift/async/TAsyncChannel.h\"\n")
		}
	}

	fService.WriteString("\n" + g.nsOpen + "\n\n")
	fServiceTcc.WriteString("\n" + g.nsOpen + "\n\n")

	// Generate all the components.
	g.generateServiceInterface(s, "")
	g.generateServiceInterfaceFactory(s, "")
	g.generateServiceNull(s, "")
	g.generateServiceHelpers(s)
	g.generateServiceClient(s, "")
	g.generateServiceProcessor(s, "")
	g.generateServiceMultiface(s)
	g.generateServiceClient(s, "Concurrent")

	// Generate the skeleton.
	if !g.opts.NoSkeleton {
		g.generateServiceSkeleton(s)
	}

	// Generate all the cob components.
	if g.opts.CobStyle {
		g.generateServiceInterface(s, "CobCl")
		g.generateServiceInterface(s, "CobSv")
		g.generateServiceInterfaceFactory(s, "CobSv")
		g.generateServiceNull(s, "CobSv")
		g.generateServiceClient(s, "Cob")
		g.generateServiceProcessor(s, "Cob")

		if !g.opts.NoSkeleton {
			g.generateServiceAsyncSkeleton(s)
		}
	}

	fHeader.WriteString("#ifdef _MSC_VER\n  #pragma warning( pop )\n#endif\n\n")

	// Close the namespace.
	fService.WriteString(g.nsClose + "\n\n")
	fServiceTcc.WriteString(g.nsClose + "\n\n")
	fHeader.WriteString(g.nsClose + "\n\n")

	if g.opts.Templates {
		fHeader.WriteString("#include \"" + g.getIncludePrefix(g.program) + svcname + ".tcc\"\n" +
			"#include \"" + g.getIncludePrefix(g.program) + g.programName + "_types.tcc\"\n\n")
	}

	fHeader.WriteString("#endif\n")
	fServiceTcc.WriteString("#endif\n")

	outDir := g.outDir()
	writeFile(outDir+svcname+".h", fHeader.String())
	writeFile(outDir+svcname+".cpp", fService.String())
	if g.serviceTccOpened {
		writeFile(outDir+svcname+".tcc", fServiceTcc.String())
	}

	g.fHeader, g.fService, g.fServiceTcc = nil, nil, nil
}

// generateServiceHelpers is t_cpp_generator::generate_service_helpers:
// generates types for all the arguments and results of the service's
// functions.
func (g *Generator) generateServiceHelpers(s *sema.Service) {
	out := g.fService
	if g.opts.Templates {
		out = g.fServiceTcc
	}

	for _, f := range s.Functions() {
		ts := f.Arglist()
		nameOrig := ts.Name()

		ts.SetName(s.Name() + "_" + f.Name() + "_args")
		g.generateStructDeclaration(g.fHeader, ts, false, false, true, true, false, false)
		g.generateStructDefinition(out, g.fService, ts, false, false, false)
		g.generateStructReader(out, ts, false)
		g.generateStructWriter(out, ts, false)

		ts.SetName(s.Name() + "_" + f.Name() + "_pargs")
		g.generateStructDeclaration(g.fHeader, ts, false, true, false, true, false, false)
		g.generateStructDefinition(out, g.fService, ts, false, false, true)
		g.generateStructWriter(out, ts, true)
		ts.SetName(nameOrig)

		g.generateFunctionHelpers(s, f)
	}
}

// generateServiceInterface is t_cpp_generator::generate_service_interface.
func (g *Generator) generateServiceInterface(s *sema.Service, style string) {
	serviceIfName := g.serviceName + style + "If"
	if style == "CobCl" {
		// Forward declare the client.
		clientName := g.serviceName + "CobClient"
		if g.opts.Templates {
			clientName += "T"
			serviceIfName += "T"
			g.fHeader.WriteString(g.indent() + "template <class Protocol_>\n")
		}
		g.fHeader.WriteString(g.indent() + "class " + clientName + ";\n\n")
	}

	extends := ""
	if base := s.Extends(); base != nil {
		extends = " : virtual public " + g.typeName(base, false, false) + style + "If"
		if style == "CobCl" && g.opts.Templates {
			extends += "T<Protocol_>"
		}
	}

	if style == "CobCl" && g.opts.Templates {
		g.fHeader.WriteString("template <class Protocol_>\n")
	}

	g.generateJavaDoc(g.fHeader, s.HasDoc(), s.Doc())

	g.fHeader.WriteString("class " + serviceIfName + extends + " {\n public:\n")
	g.indentUp()
	g.fHeader.WriteString(g.indent() + "virtual ~" + serviceIfName + "() {}\n")

	for _, f := range s.Functions() {
		if f.HasDoc() {
			g.fHeader.WriteString("\n")
		}
		g.generateJavaDocFunction(g.fHeader, f)
		g.fHeader.WriteString(g.indent() + "virtual " + g.functionSignature(f, style, "", true) + " = 0;\n")
	}
	g.indentDown()
	g.fHeader.WriteString("};\n\n")

	if style == "CobCl" && g.opts.Templates {
		// Generate a backwards-compatible typedef for clients that do not
		// know about the new template-style code.
		g.fHeader.WriteString("typedef " + serviceIfName + "< ::apache::thrift::protocol::TProtocol> " +
			g.serviceName + style + "If;\n\n")
	}
}

// generateServiceInterfaceFactory is t_cpp_generator::generate_service_interface_factory.
func (g *Generator) generateServiceInterfaceFactory(s *sema.Service, style string) {
	serviceIfName := g.serviceName + style + "If"

	// Figure out the name of the upper-most parent class.
	baseService := s
	for baseService.Extends() != nil {
		baseService = baseService.Extends()
	}
	baseIfName := g.typeName(baseService, false, false) + style + "If"

	factoryName := serviceIfName + "Factory"
	extends := ""
	if base := s.Extends(); base != nil {
		extends = " : virtual public " + g.typeName(base, false, false) + style + "IfFactory"
	}

	g.fHeader.WriteString("class " + factoryName + extends + " {\n public:\n")
	g.indentUp()

	overrideSuffix := ""
	if extends != "" {
		overrideSuffix = " override"
	}
	g.fHeader.WriteString(g.indent() + "typedef " + serviceIfName + " Handler;\n\n" + g.indent() +
		"virtual ~" + factoryName + "() {}\n\n" + g.indent() + "virtual " +
		serviceIfName + "* getHandler(" +
		"const ::apache::thrift::TConnectionInfo& connInfo)" + overrideSuffix + " = 0;\n" + g.indent() +
		"virtual void releaseHandler(" + baseIfName + "* /* handler */)" + overrideSuffix + " = 0;\n" + g.indent())

	g.indentDown()
	g.fHeader.WriteString("};\n\n")

	// Generate the singleton factory class.
	singletonFactoryName := serviceIfName + "SingletonFactory"
	g.fHeader.WriteString("class " + singletonFactoryName + " : virtual public " + factoryName + " {\n public:\n")
	g.indentUp()
	g.fHeader.WriteString(g.indent() + singletonFactoryName + "(const ::std::shared_ptr<" + serviceIfName +
		">& iface) : iface_(iface) {}\n" + g.indent() + "virtual ~" +
		singletonFactoryName + "() {}\n\n" + g.indent() + "virtual " +
		serviceIfName + "* getHandler(" +
		"const ::apache::thrift::TConnectionInfo&) override {\n" + g.indent() +
		"  return iface_.get();\n" + g.indent() + "}\n" + g.indent() +
		"virtual void releaseHandler(" + baseIfName + "* /* handler */) override {}\n")

	g.fHeader.WriteString("\n protected:\n" + g.indent() + "::std::shared_ptr<" + serviceIfName + "> iface_;\n")

	g.indentDown()
	g.fHeader.WriteString("};\n\n")
}

// generateServiceNull is t_cpp_generator::generate_service_null.
func (g *Generator) generateServiceNull(s *sema.Service, style string) {
	extends := ""
	if base := s.Extends(); base != nil {
		extends = " , virtual public " + g.typeName(base, false, false) + style + "Null"
	}
	g.fHeader.WriteString("class " + g.serviceName + style + "Null : virtual public " + g.serviceName +
		style + "If" + extends + " {\n public:\n")
	g.indentUp()
	g.fHeader.WriteString(g.indent() + "virtual ~" + g.serviceName + style + "Null() {}\n")
	for _, f := range s.Functions() {
		g.fHeader.WriteString(g.indent() + g.functionSignature(f, style, "", false) + " override {\n")
		g.indentUp()

		returntype := f.ReturnType()
		returnfield := sema.NewField(returntype, "_return", 0)

		switch style {
		case "":
			if returntype.IsVoid() || g.isComplexType(returntype) {
				g.fHeader.WriteString(g.indent() + "return;\n")
			} else {
				g.fHeader.WriteString(g.indent() + g.declareField(returnfield, true, false, false, false) + "\n" +
					g.indent() + "return _return;\n")
			}
		case "CobSv":
			if returntype.IsVoid() {
				g.fHeader.WriteString(g.indent() + "return cob();\n")
			} else {
				g.fHeader.WriteString(g.indent() + g.declareField(returnfield, true, false, false, false) + "\n" +
					g.indent() + "return cob(_return);\n")
			}
		default:
			emit.Throw("UNKNOWN STYLE")
		}

		g.indentDown()
		g.fHeader.WriteString(g.indent() + "}\n")
	}
	g.indentDown()
	g.fHeader.WriteString("};\n\n")
}

// generateServiceMultiface is t_cpp_generator::generate_service_multiface: a
// single server that takes a set of objects implementing the interface and
// calls them all, returning the value of the last one called.
func (g *Generator) generateServiceMultiface(s *sema.Service) {
	functions := s.Functions()

	extends := ""
	extendsMultiface := ""
	if base := s.Extends(); base != nil {
		extends = g.typeName(base, false, false)
		extendsMultiface = ", public " + extends + "Multiface"
	}

	listType := "std::vector<std::shared_ptr<" + g.serviceName + "If> >"

	g.fHeader.WriteString("class " + g.serviceName + "Multiface : " +
		"virtual public " + g.serviceName + "If" + extendsMultiface + " {\n public:\n")
	g.indentUp()
	g.fHeader.WriteString(g.indent() + g.serviceName + "Multiface(" + listType + "& ifaces) : ifaces_(ifaces) {\n")
	if extends != "" {
		g.fHeader.WriteString(g.indent() +
			"  std::vector<std::shared_ptr<" + g.serviceName + "If> >::iterator iter;\n" +
			g.indent() + "  for (iter = ifaces.begin(); iter != ifaces.end(); ++iter) {\n" +
			g.indent() + "    " + extends + "Multiface::add(*iter);\n" +
			g.indent() + "  }\n")
	}
	g.fHeader.WriteString(g.indent() + "}\n" + g.indent() + "virtual ~" + g.serviceName + "Multiface() {}\n")
	g.indentDown()

	// Protected data members.
	g.fHeader.WriteString(" protected:\n")
	g.indentUp()
	g.fHeader.WriteString(g.indent() + listType + " ifaces_;\n" + g.indent() + g.serviceName +
		"Multiface() {}\n" + g.indent() + "void add(::std::shared_ptr<" + g.serviceName + "If> iface) {\n")
	if extends != "" {
		g.fHeader.WriteString(g.indent() + "  " + extends + "Multiface::add(iface);\n")
	}
	g.fHeader.WriteString(g.indent() + "  ifaces_.push_back(iface);\n" + g.indent() + "}\n")
	g.indentDown()

	g.fHeader.WriteString(g.indent() + " public:\n")
	g.indentUp()

	for _, f := range functions {
		g.generateJavaDocFunction(g.fHeader, f)
		args := f.Arglist().Members()

		call := "ifaces_[i]->" + f.Name() + "("
		first := true
		if g.isComplexType(f.ReturnType()) {
			call += "_return"
			first = false
		}
		for _, a := range args {
			if first {
				first = false
			} else {
				call += ", "
			}
			call += a.Name()
		}
		call += ")"

		g.fHeader.WriteString(g.indent() + g.functionSignature(f, "", "", true) + " override {\n")
		g.indentUp()
		g.fHeader.WriteString(g.indent() + "size_t sz = ifaces_.size();\n" + g.indent() + "size_t i = 0;\n" +
			g.indent() + "for (; i < (sz - 1); ++i) {\n")
		g.indentUp()
		g.fHeader.WriteString(g.indent() + call + ";\n")
		g.indentDown()
		g.fHeader.WriteString(g.indent() + "}\n")

		if !f.ReturnType().IsVoid() {
			if g.isComplexType(f.ReturnType()) {
				g.fHeader.WriteString(g.indent() + call + ";\n" + g.indent() + "return;\n")
			} else {
				g.fHeader.WriteString(g.indent() + "return " + call + ";\n")
			}
		} else {
			g.fHeader.WriteString(g.indent() + call + ";\n")
		}

		g.indentDown()
		g.fHeader.WriteString(g.indent() + "}\n\n")
	}

	g.indentDown()
	g.fHeader.WriteString(g.indent() + "};\n\n")
}

// generateFunctionHelpers is t_cpp_generator::generate_function_helpers: a
// struct and its helpers for a function.
func (g *Generator) generateFunctionHelpers(s *sema.Service, f *sema.Function) {
	if f.IsOneway() {
		return
	}

	out := g.fService
	if g.opts.Templates {
		out = g.fServiceTcc
	}

	result := sema.NewStruct(g.program)
	result.SetName(s.Name() + "_" + f.Name() + "_result")
	success := sema.NewField(f.ReturnType(), "success", 0)
	if !f.ReturnType().IsVoid() {
		result.Append(success)
	}

	for _, xf := range f.Xceptions().Members() {
		result.Append(xf)
	}

	g.generateStructDeclaration(g.fHeader, result, false, false, true, true, false, false)
	g.generateStructDefinition(out, g.fService, result, false, false, false)
	g.generateStructReader(out, result, false)
	g.generateStructResultWriter(out, result, false)

	result.SetName(s.Name() + "_" + f.Name() + "_presult")
	g.generateStructDeclaration(g.fHeader, result, false, true, true, g.opts.CobStyle, false, false)
	g.generateStructDefinition(out, g.fService, result, false, false, true)
	g.generateStructReader(out, result, true)
	if g.opts.CobStyle {
		g.generateStructWriter(out, result, true)
	}
}

// generateServiceClient is t_cpp_generator::generate_service_client: a
// service client definition.
func (g *Generator) generateServiceClient(s *sema.Service, style string) {
	ifstyle := ""
	if style == "Cob" {
		ifstyle = "CobCl"
	}

	out := g.fService
	if g.opts.Templates {
		out = g.fServiceTcc
	}
	templateHeader, templateSuffix, shortSuffix, protocolType, this := "", "", "", "", ""
	if g.opts.Templates {
		templateHeader = "template <class Protocol_>\n"
		shortSuffix = "T"
		templateSuffix = "T<Protocol_>"
		protocolType = "Protocol_"
		this = "this->"
	} else {
		protocolType = "::apache::thrift::protocol::TProtocol"
	}
	protPtr := "std::shared_ptr< " + protocolType + ">"
	clientSuffix := "Client" + templateSuffix
	ifSuffix := "If"
	if style == "Cob" {
		ifSuffix += templateSuffix
	}

	extends := ""
	extendsClient := ""
	if base := s.Extends(); base != nil {
		extends = g.typeName(base, false, false)
		extendsClient = ", public " + extends + style + clientSuffix
	}

	// Generate the header portion.
	if style == "Concurrent" {
		g.fHeader.WriteString("// The 'concurrent' client is a thread safe client that correctly handles\n" +
			"// out of order responses.  It is slower than the regular client, so should\n" +
			"// only be used when you need to share a connection among multiple threads\n")
	}
	g.fHeader.WriteString(templateHeader + "class " + g.serviceName + style + "Client" + shortSuffix +
		" : " +
		"virtual public " + g.serviceName + ifstyle + ifSuffix + extendsClient + " {" +
		"\n public:\n")

	g.indentUp()
	if style != "Cob" {
		g.fHeader.WriteString(g.indent() + g.serviceName + style + "Client" + shortSuffix + "(" + protPtr + " prot")
		if style == "Concurrent" {
			g.fHeader.WriteString(", std::shared_ptr< ::apache::thrift::async::TConcurrentClientSyncInfo> sync")
		}
		g.fHeader.WriteString(") ")

		if extends == "" {
			if style == "Concurrent" {
				g.fHeader.WriteString(": sync_(sync)\n")
			}
			g.fHeader.WriteString("{\n")
			g.fHeader.WriteString(g.indent() + "  setProtocol" + shortSuffix + "(prot);\n" + g.indent() + "}\n")
		} else {
			g.fHeader.WriteString(":\n")
			g.fHeader.WriteString(g.indent() + "  " + extends + style + clientSuffix + "(prot, prot")
			if style == "Concurrent" {
				g.fHeader.WriteString(", sync")
			}
			g.fHeader.WriteString(") {}\n")
		}

		g.fHeader.WriteString(g.indent() + g.serviceName + style + "Client" + shortSuffix + "(" + protPtr +
			" iprot, " + protPtr + " oprot")
		if style == "Concurrent" {
			g.fHeader.WriteString(", std::shared_ptr< ::apache::thrift::async::TConcurrentClientSyncInfo> sync")
		}
		g.fHeader.WriteString(") ")

		if extends == "" {
			if style == "Concurrent" {
				g.fHeader.WriteString(": sync_(sync)\n")
			}
			g.fHeader.WriteString("{\n")
			g.fHeader.WriteString(g.indent() + "  setProtocol" + shortSuffix + "(iprot,oprot);\n" + g.indent() + "}\n")
		} else {
			g.fHeader.WriteString(":" + g.indent() + "  " + extends + style + clientSuffix + "(iprot, oprot")
			if style == "Concurrent" {
				g.fHeader.WriteString(", sync")
			}
			g.fHeader.WriteString(") {}\n")
		}

		// Create the setProtocol methods.
		if extends == "" {
			g.fHeader.WriteString(" private:\n")
			// One parameter.
			g.fHeader.WriteString(g.indent() + "void setProtocol" + shortSuffix + "(" + protPtr + " prot) {\n")
			g.fHeader.WriteString(g.indent() + "setProtocol" + shortSuffix + "(prot,prot);\n")
			g.fHeader.WriteString(g.indent() + "}\n")
			// Two parameters.
			g.fHeader.WriteString(g.indent() + "void setProtocol" + shortSuffix + "(" + protPtr + " iprot, " +
				protPtr + " oprot) {\n")

			g.fHeader.WriteString(g.indent() + "  piprot_=iprot;\n" + g.indent() + "  poprot_=oprot;\n" +
				g.indent() + "  iprot_ = iprot.get();\n" + g.indent() + "  oprot_ = oprot.get();\n")

			g.fHeader.WriteString(g.indent() + "}\n")
			g.fHeader.WriteString(" public:\n")
		}

		// Generate getters for the protocols. These are not currently
		// templated, for simplicity.
		g.fHeader.WriteString(g.indent() +
			"std::shared_ptr< ::apache::thrift::protocol::TProtocol> getInputProtocol() {\n" +
			g.indent() + "  return " + this + "piprot_;\n" + g.indent() + "}\n")

		g.fHeader.WriteString(g.indent() +
			"std::shared_ptr< ::apache::thrift::protocol::TProtocol> getOutputProtocol() {\n" +
			g.indent() + "  return " + this + "poprot_;\n" + g.indent() + "}\n")

	} else { // style == "Cob"
		g.fHeader.WriteString(g.indent() + g.serviceName + style + "Client" + shortSuffix + "(" +
			"std::shared_ptr< ::apache::thrift::async::TAsyncChannel> channel, " +
			"::apache::thrift::protocol::TProtocolFactory* protocolFactory) :\n")
		if extends == "" {
			g.fHeader.WriteString(g.indent() + "  channel_(channel),\n" + g.indent() +
				"  itrans_(new ::apache::thrift::transport::TMemoryBuffer()),\n" +
				g.indent() + "  otrans_(new ::apache::thrift::transport::TMemoryBuffer()),\n")
			if g.opts.Templates {
				// TProtocolFactory classes return generic TProtocol
				// pointers; dynamic_cast to the expected Protocol_ type.
				g.fHeader.WriteString(g.indent() + "  piprot_(::std::dynamic_pointer_cast<Protocol_>(" +
					"protocolFactory->getProtocol(itrans_))),\n" + g.indent() +
					"  poprot_(::std::dynamic_pointer_cast<Protocol_>(" +
					"protocolFactory->getProtocol(otrans_))) {\n")
				g.fHeader.WriteString(g.indent() + "  if (!piprot_ || !poprot_) {\n" + g.indent() +
					"    throw ::apache::thrift::TException(\"" +
					"TProtocolFactory returned unexpected protocol type in " + g.serviceName +
					style + "Client" + shortSuffix + " constructor\");\n" + g.indent() +
					"  }\n")
			} else {
				g.fHeader.WriteString(g.indent() + "  piprot_(protocolFactory->getProtocol(itrans_)),\n" +
					g.indent() + "  poprot_(protocolFactory->getProtocol(otrans_)) {\n")
			}
			g.fHeader.WriteString(g.indent() + "  iprot_ = piprot_.get();\n" + g.indent() +
				"  oprot_ = poprot_.get();\n" + g.indent() + "}\n")
		} else {
			g.fHeader.WriteString(g.indent() + "  " + extends + style + clientSuffix +
				"(channel, protocolFactory) {}\n")
		}
	}

	if style == "Cob" {
		g.generateJavaDoc(g.fHeader, s.HasDoc(), s.Doc())

		g.fHeader.WriteString(g.indent() +
			"::std::shared_ptr< ::apache::thrift::async::TAsyncChannel> getChannel() {\n" +
			g.indent() + "  return " + this + "channel_;\n" + g.indent() + "}\n")
		if !g.opts.NoClientCompletion {
			g.fHeader.WriteString(g.indent() + "virtual void completed__(bool /* success */) {}\n")
		}
	}

	functions := s.Functions()
	for _, f := range functions {
		g.generateJavaDocFunction(g.fHeader, f)
		g.fHeader.WriteString(g.indent() + g.functionSignature(f, ifstyle, "", true) + " override;\n")
		// TODO(dreiss): Use private inheritance to avoid generating these in cob-style.
		if style == "Concurrent" && !f.IsOneway() {
			// Concurrent clients need to move the seqid from the send
			// function to the recv function. Oneway methods don't have a
			// recv function, so no seqid move is needed there.
			sendFunction := newSimpleFunction(g.program, sema.GlobalI32, "send_"+f.Name(), f.Arglist())
			g.fHeader.WriteString(g.indent() + g.functionSignature(sendFunction, "", "", true) + ";\n")
		} else {
			sendFunction := newSimpleFunction(g.program, sema.GlobalVoid, "send_"+f.Name(), f.Arglist())
			g.fHeader.WriteString(g.indent() + g.functionSignature(sendFunction, "", "", true) + ";\n")
		}
		if !f.IsOneway() {
			if style == "Concurrent" {
				seqIDArg := sema.NewField(sema.GlobalI32, "seqid", 0)
				seqIDArgStruct := sema.NewStruct(g.program)
				seqIDArgStruct.Append(seqIDArg)
				recvFunction := newSimpleFunction(g.program, f.ReturnType(), "recv_"+f.Name(), seqIDArgStruct)
				g.fHeader.WriteString(g.indent() + g.functionSignature(recvFunction, "", "", true) + ";\n")
			} else {
				noargs := sema.NewStruct(g.program)
				recvFunction := newSimpleFunction(g.program, f.ReturnType(), "recv_"+f.Name(), noargs)
				g.fHeader.WriteString(g.indent() + g.functionSignature(recvFunction, "", "", true) + ";\n")
			}
		}
	}
	g.indentDown()

	if extends == "" {
		g.fHeader.WriteString(" protected:\n")
		g.indentUp()

		if style == "Cob" {
			g.fHeader.WriteString(g.indent() +
				"::std::shared_ptr< ::apache::thrift::async::TAsyncChannel> channel_;\n" +
				g.indent() +
				"::std::shared_ptr< ::apache::thrift::transport::TMemoryBuffer> itrans_;\n" +
				g.indent() +
				"::std::shared_ptr< ::apache::thrift::transport::TMemoryBuffer> otrans_;\n")
		}
		g.fHeader.WriteString(
			g.indent() + protPtr + " piprot_;\n" +
				g.indent() + protPtr + " poprot_;\n" +
				g.indent() + protocolType + "* iprot_;\n" +
				g.indent() + protocolType + "* oprot_;\n")

		if style == "Concurrent" {
			g.fHeader.WriteString(
				g.indent() + "std::shared_ptr< ::apache::thrift::async::TConcurrentClientSyncInfo> sync_;\n")
		}
		g.indentDown()
	}

	g.fHeader.WriteString("};\n\n")

	if g.opts.Templates {
		// Output a backwards-compatible typedef using TProtocol as the
		// template parameter.
		g.fHeader.WriteString("typedef " + g.serviceName + style +
			"ClientT< ::apache::thrift::protocol::TProtocol> " + g.serviceName + style +
			"Client;\n\n")
	}

	scope := g.serviceName + style + clientSuffix + "::"

	// Generate the client method implementations.
	for _, f := range functions {
		seqIDCapture, seqIDUse, seqIDCommaUse := "", "", ""
		if style == "Concurrent" && !f.IsOneway() {
			seqIDCapture = "int32_t seqid = "
			seqIDUse = "seqid"
			seqIDCommaUse = ", seqid"
		}

		funname := f.Name()

		// Open the function.
		if g.opts.Templates {
			out.WriteString(g.indent() + templateHeader)
		}
		out.WriteString(g.indent() + g.functionSignature(f, ifstyle, scope, true) + "\n")
		g.scopeUp(out)
		out.WriteString(g.indent() + seqIDCapture + "send_" + funname + "(")

		argStruct := f.Arglist()
		fields := argStruct.Members()
		first := true
		for _, fld := range fields {
			if first {
				first = false
			} else {
				out.WriteString(", ")
			}
			out.WriteString(fld.Name())
		}
		out.WriteString(");\n")

		if style != "Cob" {
			if !f.IsOneway() {
				out.WriteString(g.indent())
				if !f.ReturnType().IsVoid() {
					if g.isComplexType(f.ReturnType()) {
						out.WriteString("recv_" + funname + "(_return" + seqIDCommaUse + ");\n")
					} else {
						out.WriteString("return recv_" + funname + "(" + seqIDUse + ");\n")
					}
				} else {
					out.WriteString("recv_" + funname + "(" + seqIDUse + ");\n")
				}
			} else {
				out.WriteString(g.indent() + this + "oprot_->getTransport()->onewayComplete();\n")
			}
		} else {
			if !f.IsOneway() {
				out.WriteString(g.indent() + this + "channel_->sendAndRecvMessage(" +
					"::std::bind(cob, this), " + this + "otrans_.get(), " + this + "itrans_.get());\n")
			} else {
				out.WriteString(g.indent() + this + "channel_->sendMessage(" +
					"::std::bind(cob, this), " + this + "otrans_.get());\n")
			}
		}
		g.scopeDown(out)
		out.WriteString("\n")

		// TODO(dreiss): Libify the client and don't generate this for
		// cob-style.
		sendFuncReturnType := sema.Type(sema.GlobalVoid)
		if style == "Concurrent" && !f.IsOneway() {
			sendFuncReturnType = sema.GlobalI32
		}
		sendFunction := newSimpleFunction(g.program, sendFuncReturnType, "send_"+f.Name(), argStruct)

		// Open the send function.
		if g.opts.Templates {
			out.WriteString(g.indent() + templateHeader)
		}
		out.WriteString(g.indent() + g.functionSignature(sendFunction, "", scope, true) + "\n")
		g.scopeUp(out)

		argsname := s.Name() + "_" + f.Name() + "_pargs"
		resultname := s.Name() + "_" + f.Name() + "_presult"

		cseqidVal := "0"
		if style == "Concurrent" {
			if !f.IsOneway() {
				cseqidVal = "this->sync_->generateSeqId()"
			}
		}
		out.WriteString(g.indent() + "int32_t cseqid = " + cseqidVal + ";\n")
		if style == "Concurrent" {
			out.WriteString(g.indent() + "::apache::thrift::async::TConcurrentSendSentry sentry(this->sync_.get());\n")
		}
		if style == "Cob" {
			out.WriteString(g.indent() + this + "otrans_->resetBuffer();\n")
		}
		msgType := "T_CALL"
		if f.IsOneway() {
			msgType = "T_ONEWAY"
		}
		out.WriteString(
			g.indent() + this + "oprot_->writeMessageBegin(\"" + f.Name() +
				"\", ::apache::thrift::protocol::" + msgType +
				", cseqid);\n\n" +
				g.indent() + argsname + " args;\n")

		for _, fld := range fields {
			out.WriteString(g.indent() + "args." + fld.Name() + " = &" + fld.Name() + ";\n")
		}

		out.WriteString(g.indent() + "args.write(" + this + "oprot_);\n\n" + g.indent() + this +
			"oprot_->writeMessageEnd();\n" + g.indent() + this +
			"oprot_->getTransport()->writeEnd();\n" + g.indent() + this +
			"oprot_->getTransport()->flush();\n")

		if style == "Concurrent" {
			out.WriteString("\n" + g.indent() + "sentry.commit();\n")

			if !f.IsOneway() {
				out.WriteString(g.indent() + "return cseqid;\n")
			}
		}
		g.scopeDown(out)
		out.WriteString("\n")

		// Generate the recv function, unless this is a oneway function.
		if !f.IsOneway() {
			noargs := sema.NewStruct(g.program)

			seqIDArg := sema.NewField(sema.GlobalI32, "seqid", 0)
			seqIDArgStruct := sema.NewStruct(g.program)
			seqIDArgStruct.Append(seqIDArg)

			recvFunctionArgs := noargs
			if style == "Concurrent" {
				recvFunctionArgs = seqIDArgStruct
			}

			recvFunction := newSimpleFunction(g.program, f.ReturnType(), "recv_"+f.Name(), recvFunctionArgs)
			// Open the recv function.
			if g.opts.Templates {
				out.WriteString(g.indent() + templateHeader)
			}
			out.WriteString(g.indent() + g.functionSignature(recvFunction, "", scope, true) + "\n")
			g.scopeUp(out)

			out.WriteString("\n" +
				g.indent() + "int32_t rseqid = 0;\n" +
				g.indent() + "std::string fname;\n" +
				g.indent() + "::apache::thrift::protocol::TMessageType mtype;\n")
			if style == "Concurrent" {
				out.WriteString("\n" +
					g.indent() + "// the read mutex gets dropped and reacquired as part of waitForWork()\n" +
					g.indent() + "// The destructor of this sentry wakes up other clients\n" +
					g.indent() + "::apache::thrift::async::TConcurrentRecvSentry sentry(this->sync_.get(), seqid);\n")
			}
			if style == "Cob" && !g.opts.NoClientCompletion {
				out.WriteString(g.indent() + "bool completed = false;\n\n" + g.indent() + "try {")
				g.indentUp()
			}
			out.WriteString("\n")
			if style == "Concurrent" {
				out.WriteString(
					g.indent() + "while(true) {\n" +
						g.indent() + "  if(!this->sync_->getPending(fname, mtype, rseqid)) {\n")
				g.indentUp()
				g.indentUp()
			}
			out.WriteString(
				g.indent() + this + "iprot_->readMessageBegin(fname, mtype, rseqid);\n")
			if style == "Concurrent" {
				g.scopeDown(out)
				out.WriteString(g.indent() + "if(seqid == rseqid) {\n")
				g.indentUp()
			}
			out.WriteString(
				g.indent() + "if (mtype == ::apache::thrift::protocol::T_EXCEPTION) {\n" +
					g.indent() + "  ::apache::thrift::TApplicationException x;\n" +
					g.indent() + "  x.read(" + this + "iprot_);\n" +
					g.indent() + "  " + this + "iprot_->readMessageEnd();\n" +
					g.indent() + "  " + this + "iprot_->getTransport()->readEnd();\n")
			if style == "Cob" && !g.opts.NoClientCompletion {
				out.WriteString(g.indent() + "  completed = true;\n" + g.indent() + "  completed__(true);\n")
			}
			if style == "Concurrent" {
				out.WriteString(g.indent() + "  sentry.commit();\n")
			}
			out.WriteString(
				g.indent() + "  throw x;\n" +
					g.indent() + "}\n" +
					g.indent() + "if (mtype != ::apache::thrift::protocol::T_REPLY) {\n" +
					g.indent() + "  " + this + "iprot_->skip(" + "::apache::thrift::protocol::T_STRUCT);\n" +
					g.indent() + "  " + this + "iprot_->readMessageEnd();\n" +
					g.indent() + "  " + this + "iprot_->getTransport()->readEnd();\n")
			if style == "Cob" && !g.opts.NoClientCompletion {
				out.WriteString(g.indent() + "  completed = true;\n" + g.indent() + "  completed__(false);\n")
			}
			out.WriteString(
				g.indent() + "}\n" +
					g.indent() + "if (fname.compare(\"" + f.Name() + "\") != 0) {\n" +
					g.indent() + "  " + this + "iprot_->skip(" + "::apache::thrift::protocol::T_STRUCT);\n" +
					g.indent() + "  " + this + "iprot_->readMessageEnd();\n" +
					g.indent() + "  " + this + "iprot_->getTransport()->readEnd();\n")
			if style == "Cob" && !g.opts.NoClientCompletion {
				out.WriteString(g.indent() + "  completed = true;\n" + g.indent() + "  completed__(false);\n")
			}
			if style == "Concurrent" {
				out.WriteString("\n" +
					g.indent() + "  // in a bad state, don't commit\n" +
					g.indent() + "  using ::apache::thrift::protocol::TProtocolException;\n" +
					g.indent() + "  throw TProtocolException(TProtocolException::INVALID_DATA);\n")
			}
			out.WriteString(g.indent() + "}\n")

			if !f.ReturnType().IsVoid() && !g.isComplexType(f.ReturnType()) {
				returnfield := sema.NewField(f.ReturnType(), "_return", 0)
				out.WriteString(g.indent() + g.declareField(returnfield, false, false, false, false) + "\n")
			}

			out.WriteString(g.indent() + resultname + " result;\n")

			if !f.ReturnType().IsVoid() {
				out.WriteString(g.indent() + "result.success = &_return;\n")
			}

			out.WriteString(g.indent() + "result.read(" + this + "iprot_);\n" + g.indent() + this +
				"iprot_->readMessageEnd();\n" + g.indent() + this +
				"iprot_->getTransport()->readEnd();\n\n")

			// Careful, only look for _result if not a void function.
			if !f.ReturnType().IsVoid() {
				if g.isComplexType(f.ReturnType()) {
					out.WriteString(
						g.indent() + "if (result.__isset.success) {\n")
					out.WriteString(
						g.indent() + "  // _return pointer has now been filled\n")
					if style == "Cob" && !g.opts.NoClientCompletion {
						out.WriteString(g.indent() + "  completed = true;\n" + g.indent() + "  completed__(true);\n")
					}
					if style == "Concurrent" {
						out.WriteString(g.indent() + "  sentry.commit();\n")
					}
					out.WriteString(
						g.indent() + "  return;\n" +
							g.indent() + "}\n")
				} else {
					out.WriteString(g.indent() + "if (result.__isset.success) {\n")
					if style == "Cob" && !g.opts.NoClientCompletion {
						out.WriteString(g.indent() + "  completed = true;\n" + g.indent() + "  completed__(true);\n")
					}
					if style == "Concurrent" {
						out.WriteString(g.indent() + "  sentry.commit();\n")
					}
					out.WriteString(g.indent() + "  return _return;\n" + g.indent() + "}\n")
				}
			}

			xceptions := f.Xceptions().Members()
			for _, x := range xceptions {
				out.WriteString(g.indent() + "if (result.__isset." + x.Name() + ") {\n")
				if style == "Cob" && !g.opts.NoClientCompletion {
					out.WriteString(g.indent() + "  completed = true;\n" + g.indent() + "  completed__(true);\n")
				}
				if style == "Concurrent" {
					out.WriteString(g.indent() + "  sentry.commit();\n")
				}
				out.WriteString(g.indent() + "  throw result." + x.Name() + ";\n" + g.indent() + "}\n")
			}

			// Only reached for a void function.
			if f.ReturnType().IsVoid() {
				if style == "Cob" && !g.opts.NoClientCompletion {
					out.WriteString(g.indent() + "completed = true;\n" + g.indent() + "completed__(true);\n")
				}
				if style == "Concurrent" {
					out.WriteString(g.indent() + "sentry.commit();\n")
				}
				out.WriteString(g.indent() + "return;\n")
			} else {
				if style == "Cob" && !g.opts.NoClientCompletion {
					out.WriteString(g.indent() + "completed = true;\n" + g.indent() + "completed__(true);\n")
				}
				if style == "Concurrent" {
					out.WriteString(g.indent() + "// in a bad state, don't commit\n")
				}
				out.WriteString(g.indent() + "throw " +
					"::apache::thrift::TApplicationException(::apache::thrift::" +
					"TApplicationException::MISSING_RESULT, \"" + f.Name() +
					" failed: unknown result\");\n")
			}
			if style == "Concurrent" {
				g.indentDown()
				g.indentDown()
				out.WriteString(g.indent() + "  }\n" +
					g.indent() + "  // seqid != rseqid\n" +
					g.indent() + "  this->sync_->updatePending(fname, mtype, rseqid);\n" +
					"\n" +
					g.indent() +
					"  // this will temporarily unlock the readMutex, and let other clients get work done\n" +
					g.indent() + "  this->sync_->waitForWork(seqid);\n" +
					g.indent() + "} // end while(true)\n")
			}
			if style == "Cob" && !g.opts.NoClientCompletion {
				g.indentDown()
				out.WriteString(g.indent() + "} catch (...) {\n" + g.indent() + "  if (!completed) {\n" +
					g.indent() + "    completed__(false);\n" + g.indent() + "  }\n" +
					g.indent() + "  throw;\n" + g.indent() + "}\n")
			}
			// Close the function.
			g.scopeDown(out)
			out.WriteString("\n")
		}
	}
}

// processorGenerator is ProcessorGenerator: a helper class factoring out
// the differences between the normal and cob-style service processors.
type processorGenerator struct {
	g       *Generator
	service *sema.Service
	fHeader *strings.Builder
	fOut    *strings.Builder

	serviceName      string
	style            string
	pstyle           string
	className        string
	ifName           string
	factoryClassName string
	finishCob        string
	finishCobDecl    string
	retType          string
	callContext      string
	cobArg           string
	callContextArg   string
	callContextDecl  string
	templateHeader   string
	templateSuffix   string
	typenameStr      string
	extends          string
}

func newProcessorGenerator(g *Generator, s *sema.Service, style string) *processorGenerator {
	p := &processorGenerator{
		g:           g,
		service:     s,
		fHeader:     g.fHeader,
		fOut:        g.fService,
		serviceName: g.serviceName,
		style:       style,
	}
	if g.opts.Templates {
		p.fOut = g.fServiceTcc
	}
	if style == "Cob" {
		p.pstyle = "Async"
		p.className = p.serviceName + p.pstyle + "Processor"
		p.ifName = p.serviceName + "CobSvIf"

		p.finishCob = "::std::function<void(bool ok)> cob, "
		p.finishCobDecl = "::std::function<void(bool ok)>, "
		p.cobArg = "cob, "
		p.retType = "void "
	} else {
		p.className = p.serviceName + "Processor"
		p.ifName = p.serviceName + "If"

		p.retType = "bool "
		// TODO(edhall): callContext should eventually be added to
		// TAsyncProcessor.
		p.callContext = ", void* callContext"
		p.callContextArg = ", callContext"
		p.callContextDecl = ", void*"
	}

	p.factoryClassName = p.className + "Factory"

	if g.opts.Templates {
		p.templateHeader = "template <class Protocol_>\n"
		p.templateSuffix = "<Protocol_>"
		p.typenameStr = "typename "
		p.className += "T"
		p.factoryClassName += "T"
	}

	if base := s.Extends(); base != nil {
		p.extends = g.typeName(base, false, false) + p.pstyle + "Processor"
		if g.opts.Templates {
			// TODO(simpkins): this assumes all parent services were also
			// generated with templates enabled.
			p.extends += "T<Protocol_>"
		}
	}

	return p
}

func (p *processorGenerator) run() {
	p.generateClassDefinition()

	// Generate the dispatchCall() function.
	p.generateDispatchCall(false)
	if p.g.opts.Templates {
		p.generateDispatchCall(true)
	}

	// Generate all of the process subfunctions.
	p.generateProcessFunctions()

	p.generateFactory()
}

func (p *processorGenerator) generateClassDefinition() {
	g := p.g
	functions := p.service.Functions()

	parentClass := ""
	if p.service.Extends() != nil {
		parentClass = p.extends
	} else {
		if p.style == "Cob" {
			parentClass = "::apache::thrift::async::TAsyncDispatchProcessor"
		} else {
			parentClass = "::apache::thrift::TDispatchProcessor"
		}
		if g.opts.Templates {
			parentClass += "T<Protocol_>"
		}
	}

	p.fHeader.WriteString(p.templateHeader + "class " + p.className + " : public " + parentClass + " {\n")

	// Protected data members.
	p.fHeader.WriteString(" protected:\n")
	g.indentUp()
	p.fHeader.WriteString(g.indent() + "::std::shared_ptr<" + p.ifName + "> iface_;\n")
	p.fHeader.WriteString(g.indent() + "virtual " + p.retType + "dispatchCall(" + p.finishCob +
		"::apache::thrift::protocol::TProtocol* iprot, " +
		"::apache::thrift::protocol::TProtocol* oprot, " +
		"const std::string& fname, int32_t seqid" + p.callContext +
		") override;\n")
	if g.opts.Templates {
		p.fHeader.WriteString(g.indent() + "virtual " + p.retType + "dispatchCallTemplated(" + p.finishCob +
			"Protocol_* iprot, Protocol_* oprot, " +
			"const std::string& fname, int32_t seqid" + p.callContext + ");\n")
	}
	g.indentDown()

	// Process function declarations.
	p.fHeader.WriteString(" private:\n")
	g.indentUp()

	// Declare processMap_.
	p.fHeader.WriteString(g.indent() + "typedef  void (" + p.className + "::*" +
		"ProcessFunction)(" + p.finishCobDecl + "int32_t, " +
		"::apache::thrift::protocol::TProtocol*, " +
		"::apache::thrift::protocol::TProtocol*" + p.callContextDecl + ");\n")
	if g.opts.Templates {
		p.fHeader.WriteString(g.indent() + "typedef void (" + p.className + "::*" +
			"SpecializedProcessFunction)(" + p.finishCobDecl + "int32_t, " +
			"Protocol_*, Protocol_*" + p.callContextDecl + ");\n" + g.indent() +
			"struct ProcessFunctions {\n" + g.indent() + "  ProcessFunction generic;\n" +
			g.indent() + "  SpecializedProcessFunction specialized;\n" + g.indent() +
			"  ProcessFunctions(ProcessFunction g, " +
			"SpecializedProcessFunction s) :\n" + g.indent() + "    generic(g),\n" +
			g.indent() + "    specialized(s) {}\n" + g.indent() +
			"  ProcessFunctions() : generic(nullptr), specialized(nullptr) " +
			"{}\n" + g.indent() + "};\n" + g.indent() +
			"typedef std::map<std::string, ProcessFunctions> " +
			"ProcessMap;\n")
	} else {
		p.fHeader.WriteString(g.indent() + "typedef std::map<std::string, ProcessFunction> " +
			"ProcessMap;\n")
	}
	p.fHeader.WriteString(g.indent() + "ProcessMap processMap_;\n")

	for _, f := range functions {
		p.fHeader.WriteString(g.indent() + "void process_" + f.Name() + "(" + p.finishCob +
			"int32_t seqid, ::apache::thrift::protocol::TProtocol* iprot, " +
			"::apache::thrift::protocol::TProtocol* oprot" + p.callContext + ");\n")
		if g.opts.Templates {
			p.fHeader.WriteString(g.indent() + "void process_" + f.Name() + "(" + p.finishCob +
				"int32_t seqid, Protocol_* iprot, Protocol_* oprot" + p.callContext +
				");\n")
		}
		if p.style == "Cob" {
			retArg := ""
			if !f.ReturnType().IsVoid() {
				retArg = ", const " + g.typeName(f.ReturnType(), false, false) + "& _return"
			}
			p.fHeader.WriteString(g.indent() + "void return_" + f.Name() +
				"(::std::function<void(bool ok)> cob, int32_t seqid, " +
				"::apache::thrift::protocol::TProtocol* oprot, " +
				"void* ctx" + retArg + ");\n")
			if g.opts.Templates {
				p.fHeader.WriteString(g.indent() + "void return_" + f.Name() +
					"(::std::function<void(bool ok)> cob, int32_t seqid, " +
					"Protocol_* oprot, void* ctx" + retArg + ");\n")
			}
			// XXX Don't declare throw if it doesn't exist.
			p.fHeader.WriteString(g.indent() + "void throw_" + f.Name() +
				"(::std::function<void(bool ok)> cob, int32_t seqid, " +
				"::apache::thrift::protocol::TProtocol* oprot, void* ctx, " +
				"::apache::thrift::TDelayedException* _throw);\n")
			if g.opts.Templates {
				p.fHeader.WriteString(g.indent() + "void throw_" + f.Name() +
					"(::std::function<void(bool ok)> cob, int32_t seqid, " +
					"Protocol_* oprot, void* ctx, " +
					"::apache::thrift::TDelayedException* _throw);\n")
			}
		}
	}

	p.fHeader.WriteString(" public:\n" + g.indent() + p.className + "(::std::shared_ptr<" + p.ifName +
		"> iface) :\n")
	if p.extends != "" {
		p.fHeader.WriteString(g.indent() + "  " + p.extends + "(iface),\n")
	}
	p.fHeader.WriteString(g.indent() + "  iface_(iface) {\n")
	g.indentUp()

	for _, f := range functions {
		p.fHeader.WriteString(g.indent() + "processMap_[\"" + f.Name() + "\"] = ")
		if g.opts.Templates {
			p.fHeader.WriteString("ProcessFunctions(\n")
			if g.opts.TemplatesOnly {
				p.fHeader.WriteString(g.indent() + "  nullptr,\n")
			} else {
				p.fHeader.WriteString(g.indent() + "  &" + p.className + "::process_" + f.Name() + ",\n")
			}
			p.fHeader.WriteString(g.indent() + "  &" + p.className + "::process_" + f.Name() + ")")
		} else {
			p.fHeader.WriteString("&" + p.className + "::process_" + f.Name())
		}
		p.fHeader.WriteString(";\n")
	}

	g.indentDown()
	p.fHeader.WriteString(g.indent() + "}\n\n" + g.indent() + "virtual ~" + p.className + "() {}\n")
	g.indentDown()
	p.fHeader.WriteString("};\n\n")

	if g.opts.Templates {
		// Generate a backwards-compatible typedef, for callers who don't
		// know about the new template-style code. TProtocol cannot be used
		// as the template parameter, since ProcessorT provides overloaded
		// versions of most methods, one accepting TProtocol pointers and
		// one accepting Protocol_ pointers; instantiating with
		// Protocol_ == TProtocol results in a compile error. TDummyProtocol
		// exists solely to be used as the template parameter here.
		p.fHeader.WriteString("typedef " + p.className + "< ::apache::thrift::protocol::TDummyProtocol > " +
			p.serviceName + p.pstyle + "Processor;\n\n")
	}
}

func (p *processorGenerator) generateDispatchCall(templateProtocol bool) {
	g := p.g
	protocol := "::apache::thrift::protocol::TProtocol"
	functionSuffix := ""
	if templateProtocol {
		protocol = "Protocol_"
		// The generic version is called dispatchCall(), and the
		// specialized version dispatchCallTemplated(); calling them both
		// dispatchCall() causes a compiler warning if a templated service
		// extends one that isn't, since the subclass then only implements
		// the generic overload and hides the templated one.
		functionSuffix = "Templated"
	}

	p.fOut.WriteString(p.templateHeader + p.retType + p.className + p.templateSuffix + "::dispatchCall" +
		functionSuffix + "(" + p.finishCob + protocol + "* iprot, " + protocol +
		"* oprot, " +
		"const std::string& fname, int32_t seqid" + p.callContext + ") {\n")
	g.indentUp()

	// HOT: member function pointer map.
	p.fOut.WriteString(g.indent() + p.typenameStr + "ProcessMap::iterator pfn;\n" + g.indent() +
		"pfn = processMap_.find(fname);\n" + g.indent() +
		"if (pfn == processMap_.end()) {\n")
	if p.extends == "" {
		retLine := "  return true;"
		if p.style == "Cob" {
			retLine = "  return cob(true);"
		}
		p.fOut.WriteString(g.indent() + "  iprot->skip(::apache::thrift::protocol::T_STRUCT);\n" + g.indent() +
			"  iprot->readMessageEnd();\n" + g.indent() +
			"  iprot->getTransport()->readEnd();\n" + g.indent() +
			"  ::apache::thrift::TApplicationException " +
			"x(::apache::thrift::TApplicationException::UNKNOWN_METHOD, \"Invalid method name: " +
			"'\"+fname+\"'\");\n" + g.indent() +
			"  oprot->writeMessageBegin(fname, ::apache::thrift::protocol::T_EXCEPTION, seqid);\n" + g.indent() +
			"  x.write(oprot);\n" + g.indent() +
			"  oprot->writeMessageEnd();\n" + g.indent() +
			"  oprot->getTransport()->writeEnd();\n" + g.indent() +
			"  oprot->getTransport()->flush();\n" + g.indent() +
			retLine + "\n")
	} else {
		cobPrefix := ""
		if p.style == "Cob" {
			cobPrefix = "cob, "
		}
		p.fOut.WriteString(g.indent() + "  return " + p.extends + "::dispatchCall(" +
			cobPrefix + "iprot, oprot, fname, seqid" + p.callContextArg +
			");\n")
	}
	p.fOut.WriteString(g.indent() + "}\n")
	if templateProtocol {
		p.fOut.WriteString(g.indent() + "(this->*(pfn->second.specialized))")
	} else {
		if g.opts.TemplatesOnly {
			// TODO: this is a null pointer, so nothing good comes from
			// calling it; throw an exception instead.
			p.fOut.WriteString(g.indent() + "(this->*(pfn->second.generic))")
		} else if g.opts.Templates {
			p.fOut.WriteString(g.indent() + "(this->*(pfn->second.generic))")
		} else {
			p.fOut.WriteString(g.indent() + "(this->*(pfn->second))")
		}
	}
	p.fOut.WriteString("(" + p.cobArg + "seqid, iprot, oprot" + p.callContextArg + ");\n")

	// TODO(dreiss): return pfn ret?
	if p.style == "Cob" {
		p.fOut.WriteString(g.indent() + "return;\n")
	} else {
		p.fOut.WriteString(g.indent() + "return true;\n")
	}

	g.indentDown()
	p.fOut.WriteString("}\n\n")
}

func (p *processorGenerator) generateProcessFunctions() {
	for _, f := range p.service.Functions() {
		if p.g.opts.Templates {
			p.g.generateProcessFunction(p.service, f, p.style, false)
			p.g.generateProcessFunction(p.service, f, p.style, true)
		} else {
			p.g.generateProcessFunction(p.service, f, p.style, false)
		}
	}
}

func (p *processorGenerator) generateFactory() {
	g := p.g
	ifFactoryName := p.ifName + "Factory"

	procNs, procKind := "", "TProcessor"
	factoryBase := "TProcessorFactory"
	if p.style == "Cob" {
		procNs = "async::"
		procKind = "TAsyncProcessor"
		factoryBase = "async::TAsyncProcessorFactory"
	}

	// Generate the factory class definition.
	p.fHeader.WriteString(p.templateHeader + "class " + p.factoryClassName + " : public ::apache::thrift::" +
		factoryBase + " {\n public:\n")
	g.indentUp()

	p.fHeader.WriteString(g.indent() + p.factoryClassName + "(const ::std::shared_ptr< " + ifFactoryName +
		" >& handlerFactory) noexcept :\n" + g.indent() +
		"    handlerFactory_(handlerFactory) {}\n\n" + g.indent() +
		"::std::shared_ptr< ::apache::thrift::" +
		procNs + procKind + " > " +
		"getProcessor(const ::apache::thrift::TConnectionInfo& connInfo) override;\n")

	p.fHeader.WriteString("\n protected:\n" + g.indent() + "::std::shared_ptr< " +
		ifFactoryName + " > handlerFactory_;\n")

	g.indentDown()
	p.fHeader.WriteString("};\n\n")

	// If generating templates, output a typedef for the plain factory
	// name.
	if g.opts.Templates {
		p.fHeader.WriteString("typedef " + p.factoryClassName +
			"< ::apache::thrift::protocol::TDummyProtocol > " + p.serviceName + p.pstyle +
			"ProcessorFactory;\n\n")
	}

	// Generate the getProcessor() method.
	p.fOut.WriteString(p.templateHeader + g.indent() + "::std::shared_ptr< ::apache::thrift::" +
		procNs + procKind + " > " +
		p.factoryClassName + p.templateSuffix + "::getProcessor(" +
		"const ::apache::thrift::TConnectionInfo& connInfo) {\n")
	g.indentUp()

	p.fOut.WriteString(g.indent() + "::apache::thrift::ReleaseHandler< " + ifFactoryName +
		" > cleanup(handlerFactory_);\n" + g.indent() + "::std::shared_ptr< " +
		p.ifName + " > handler(" +
		"handlerFactory_->getHandler(connInfo), cleanup);\n" + g.indent() +
		"::std::shared_ptr< ::apache::thrift::" +
		procNs + procKind + " > " +
		"processor(new " + p.className + p.templateSuffix + "(handler));\n" + g.indent() +
		"return processor;\n")

	g.indentDown()
	p.fOut.WriteString(g.indent() + "}\n\n")
}

// generateServiceProcessor is t_cpp_generator::generate_service_processor.
func (g *Generator) generateServiceProcessor(s *sema.Service, style string) {
	newProcessorGenerator(g, s, style).run()
}

// generateProcessFunction is t_cpp_generator::generate_process_function: a
// process function definition, the dispatcher for tfunction.
func (g *Generator) generateProcessFunction(s *sema.Service, f *sema.Function, style string, specialized bool) {
	fields := f.Arglist().Members()
	xceptions := f.Xceptions().Members()
	serviceFuncName := "\"" + s.Name() + "." + f.Name() + "\""

	out := g.fService
	if g.opts.Templates {
		out = g.fServiceTcc
	}

	protType := "::apache::thrift::protocol::TProtocol"
	if specialized {
		protType = "Protocol_"
	}
	classSuffix := ""
	if g.opts.Templates {
		classSuffix = "T<Protocol_>"
	}

	// I tried to do this as one function. I really did. But it was too hard.
	if style != "Cob" {
		if g.opts.Templates {
			out.WriteString(g.indent() + "template <class Protocol_>\n")
		}
		out.WriteString("void " + s.Name() + "Processor" + classSuffix + "::" +
			"process_" + f.Name() + "(" +
			"int32_t" + " seqid, " + protType + "* iprot, " +
			protType + "*" + " oprot, " + "void* callContext)" +
			"\n")
		g.scopeUp(out)

		argsname := s.Name() + "_" + f.Name() + "_args"
		resultname := s.Name() + "_" + f.Name() + "_result"

		if f.IsOneway() {
			out.WriteString(g.indent() + "(void) seqid;\n")
		}

		out.WriteString(g.indent() + "void* ctx = nullptr;\n" + g.indent() +
			"if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
			"  ctx = this->eventHandler_->getContext(" + serviceFuncName + ", callContext);\n" + g.indent() + "}\n" + g.indent() +
			"::apache::thrift::TProcessorContextFreer freer(" +
			"this->eventHandler_.get(), ctx, " + serviceFuncName + ");\n\n" +
			g.indent() + "if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
			"  this->eventHandler_->preRead(ctx, " + serviceFuncName + ");\n" + g.indent() +
			"}\n\n" + g.indent() + argsname + " args;\n" + g.indent() +
			"args.read(iprot);\n" + g.indent() + "iprot->readMessageEnd();\n" + g.indent() +
			"uint32_t bytes = iprot->getTransport()->readEnd();\n\n" + g.indent() +
			"if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
			"  this->eventHandler_->postRead(ctx, " + serviceFuncName + ", bytes);\n" + g.indent() + "}\n\n")

		if !f.IsOneway() {
			out.WriteString(g.indent() + resultname + " result;\n")
		}

		out.WriteString(g.indent() + "try {\n")
		g.indentUp()

		first := true
		out.WriteString(g.indent())
		if !f.IsOneway() && !f.ReturnType().IsVoid() {
			if g.isComplexType(f.ReturnType()) {
				first = false
				out.WriteString("iface_->" + f.Name() + "(result.success")
			} else {
				out.WriteString("result.success = iface_->" + f.Name() + "(")
			}
		} else {
			out.WriteString("iface_->" + f.Name() + "(")
		}
		for _, fld := range fields {
			if first {
				first = false
			} else {
				out.WriteString(", ")
			}
			out.WriteString("args." + fld.Name())
		}
		out.WriteString(");\n")

		if !f.IsOneway() && !f.ReturnType().IsVoid() {
			out.WriteString(g.indent() + "result.__isset.success = true;\n")
		}

		g.indentDown()
		out.WriteString(g.indent() + "}")

		if !f.IsOneway() {
			for _, x := range xceptions {
				out.WriteString(" catch (" + g.typeName(x.Type(), false, false) + " &" + x.Name() + ") {\n")
				g.indentUp()
				out.WriteString(g.indent() + "result." + x.Name() +
					" = std::move(" + x.Name() + ");\n" +
					g.indent() + "result.__isset." + x.Name() + " = true;\n")
				g.indentDown()
				out.WriteString(g.indent() + "}")
			}
		}

		if !f.IsOneway() {
			out.WriteString(" catch (const std::exception& e) {\n")
		} else {
			out.WriteString(" catch (const std::exception&) {\n")
		}

		g.indentUp()
		out.WriteString(g.indent() + "if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
			"  this->eventHandler_->handlerError(ctx, " + serviceFuncName + ");\n" + g.indent() + "}\n")

		if !f.IsOneway() {
			out.WriteString("\n" + g.indent() + "::apache::thrift::TApplicationException x(e.what());\n" + g.indent() +
				"oprot->writeMessageBegin(\"" + f.Name() +
				"\", ::apache::thrift::protocol::T_EXCEPTION, seqid);\n" + g.indent() +
				"x.write(oprot);\n" + g.indent() + "oprot->writeMessageEnd();\n" + g.indent() +
				"oprot->getTransport()->writeEnd();\n" + g.indent() +
				"oprot->getTransport()->flush();\n")
		} else {
			out.WriteString("\n" + g.indent() + "oprot->getTransport()->onewayComplete();\n")
		}
		out.WriteString(g.indent() + "return;\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")

		// Shortcut out for oneway functions.
		if f.IsOneway() {
			out.WriteString(g.indent() + "if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
				"  this->eventHandler_->asyncComplete(ctx, " + serviceFuncName + ");\n" + g.indent() +
				"}\n\n" + g.indent() +
				"oprot->getTransport()->onewayComplete();\n" + g.indent() + "return;\n")
			g.indentDown()
			out.WriteString("}\n\n")
			return
		}

		// Serialize the result into a struct.
		out.WriteString(g.indent() + "if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
			"  this->eventHandler_->preWrite(ctx, " + serviceFuncName + ");\n" + g.indent() +
			"}\n\n" + g.indent() + "oprot->writeMessageBegin(\"" + f.Name() +
			"\", ::apache::thrift::protocol::T_REPLY, seqid);\n" + g.indent() +
			"result.write(oprot);\n" + g.indent() + "oprot->writeMessageEnd();\n" + g.indent() +
			"bytes = oprot->getTransport()->writeEnd();\n" + g.indent() +
			"oprot->getTransport()->flush();\n\n" + g.indent() +
			"if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
			"  this->eventHandler_->postWrite(ctx, " + serviceFuncName + ", bytes);\n" + g.indent() + "}\n")

		g.scopeDown(out)
		out.WriteString("\n")
		return
	}

	// Cob style.
	// TODO(edhall): update for callContext when TEventServer is ready.
	if g.opts.Templates {
		out.WriteString(g.indent() + "template <class Protocol_>\n")
	}
	out.WriteString("void " + s.Name() + "AsyncProcessor" + classSuffix + "::process_" +
		f.Name() + "(::std::function<void(bool ok)> cob, int32_t seqid, " +
		protType + "* iprot, " + protType + "* oprot)\n")
	g.scopeUp(out)

	// TODO(simpkins): could try to consolidate this with the non-cob code
	// above.
	if g.opts.Templates && !specialized {
		// If these are instances of Protocol_, instead of any old
		// TProtocol, use the specialized process function instead.
		out.WriteString(g.indent() + "Protocol_* _iprot = dynamic_cast<Protocol_*>(iprot);\n" + g.indent() +
			"Protocol_* _oprot = dynamic_cast<Protocol_*>(oprot);\n" + g.indent() +
			"if (_iprot && _oprot) {\n" + g.indent() + "  return process_" +
			f.Name() + "(cob, seqid, _iprot, _oprot);\n" + g.indent() + "}" +
			"\n" + g.indent() + "T_GENERIC_PROTOCOL(this, iprot, _iprot);\n" + g.indent() +
			"T_GENERIC_PROTOCOL(this, oprot, _oprot);\n\n")
	}

	if f.IsOneway() {
		out.WriteString(g.indent() + "(void) seqid;\n" + g.indent() + "(void) oprot;\n")
	}

	out.WriteString(g.indent() + s.Name() + "_" + f.Name() + "_args args;\n" +
		g.indent() + "void* ctx = nullptr;\n" + g.indent() +
		"if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
		"  ctx = this->eventHandler_->getContext(" + serviceFuncName + ", nullptr);\n" + g.indent() + "}\n" + g.indent() +
		"::apache::thrift::TProcessorContextFreer freer(" +
		"this->eventHandler_.get(), ctx, " + serviceFuncName + ");\n\n" +
		g.indent() + "try {\n")
	g.indentUp()
	out.WriteString(g.indent() + "if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
		"  this->eventHandler_->preRead(ctx, " + serviceFuncName + ");\n" + g.indent() +
		"}\n" + g.indent() + "args.read(iprot);\n" + g.indent() +
		"iprot->readMessageEnd();\n" + g.indent() +
		"uint32_t bytes = iprot->getTransport()->readEnd();\n" + g.indent() +
		"if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
		"  this->eventHandler_->postRead(ctx, " + serviceFuncName + ", bytes);\n" + g.indent() + "}\n")
	g.scopeDown(out)

	// TODO(dreiss): Handle TExceptions? Expose to the server?
	out.WriteString(g.indent() + "catch (const std::exception&) {\n" + g.indent() +
		"  if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
		"    this->eventHandler_->handlerError(ctx, " + serviceFuncName + ");\n" + g.indent() +
		"  }\n" + g.indent() + "  return cob(false);\n" + g.indent() + "}\n")

	if f.IsOneway() {
		out.WriteString(g.indent() + "if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
			"  this->eventHandler_->asyncComplete(ctx, " + serviceFuncName + ");\n" + g.indent() + "}\n")
	}
	// TODO(dreiss): Figure out a strategy for exceptions in async handlers.
	out.WriteString(g.indent() + "freer.unregister();\n")
	if f.IsOneway() {
		// No return; just hand off the cob.
		// TODO(dreiss): Call the cob immediately?
		out.WriteString(g.indent() + "iface_->" + f.Name() + "(" +
			"::std::bind(cob, true)\n")
		g.indentUp()
		g.indentUp()
	} else {
		retArg, retPlaceholder := "", ""
		if !f.ReturnType().IsVoid() {
			retArg = ", const " + g.typeName(f.ReturnType(), false, false) + "& _return"
			retPlaceholder = ", ::std::placeholders::_1"
		}

		// When gen_templates_ is true, return_ and throw_ are overloaded;
		// pointers to them let the compiler resolve the correct overload.
		out.WriteString(g.indent() + "void (" + s.Name() + "AsyncProcessor" + classSuffix +
			"::*return_fn)(::std::function<void(bool ok)> " +
			"cob, int32_t seqid, " + protType + "* oprot, void* ctx" + retArg +
			") =\n")
		out.WriteString(g.indent() + "  &" + s.Name() + "AsyncProcessor" + classSuffix +
			"::return_" + f.Name() + ";\n")
		if len(xceptions) != 0 {
			out.WriteString(g.indent() + "void (" + s.Name() + "AsyncProcessor" + classSuffix +
				"::*throw_fn)(::std::function<void(bool ok)> " +
				"cob, int32_t seqid, " + protType + "* oprot, void* ctx, " +
				"::apache::thrift::TDelayedException* _throw) =\n")
			out.WriteString(g.indent() + "  &" + s.Name() + "AsyncProcessor" + classSuffix +
				"::throw_" + f.Name() + ";\n")
		}

		out.WriteString(g.indent() + "iface_->" + f.Name() + "(\n")
		g.indentUp()
		g.indentUp()
		out.WriteString(g.indent() + "::std::bind(return_fn, this, cob, seqid, oprot, ctx" + retPlaceholder +
			")")
		if len(xceptions) != 0 {
			out.WriteString(",\n" + g.indent() + "::std::bind(throw_fn, this, cob, seqid, oprot, " +
				"ctx, ::std::placeholders::_1)")
		}
	}

	// XXX Whitespace cleanup.
	for _, fld := range fields {
		out.WriteString(",\n" + g.indent() + "args." + fld.Name())
	}
	out.WriteString(");\n")
	g.indentDown()
	g.indentDown()
	g.scopeDown(out)
	out.WriteString("\n")

	// Normal return.
	if !f.IsOneway() {
		retArgDecl, retArgName := "", ""
		if !f.ReturnType().IsVoid() {
			retArgDecl = ", const " + g.typeName(f.ReturnType(), false, false) + "& _return"
			retArgName = ", _return"
		}
		if g.opts.Templates {
			out.WriteString(g.indent() + "template <class Protocol_>\n")
		}
		out.WriteString("void " + s.Name() + "AsyncProcessor" + classSuffix + "::return_" +
			f.Name() + "(::std::function<void(bool ok)> cob, int32_t seqid, " +
			protType + "* oprot, void* ctx" + retArgDecl + ")\n")
		g.scopeUp(out)

		if g.opts.Templates && !specialized {
			// If oprot is a Protocol_ instance, use the specialized return
			// function instead.
			out.WriteString(g.indent() + "Protocol_* _oprot = dynamic_cast<Protocol_*>(oprot);\n" + g.indent() +
				"if (_oprot) {\n" + g.indent() + "  return return_" +
				f.Name() + "(cob, seqid, _oprot, ctx" + retArgName + ");\n" + g.indent() + "}\n" + g.indent() +
				"T_GENERIC_PROTOCOL(this, oprot, _oprot);\n\n")
		}

		out.WriteString(g.indent() + s.Name() + "_" + f.Name() + "_presult result;\n")
		if !f.ReturnType().IsVoid() {
			// The const_cast here is unfortunate, but hard to avoid, and
			// this struct is only used to write, which is const-safe.
			out.WriteString(g.indent() + "result.success = const_cast<" + g.typeName(f.ReturnType(), false, false) +
				"*>(&_return);\n" + g.indent() + "result.__isset.success = true;\n")
		}
		// Serialize the result into a struct.
		out.WriteString("\n" + g.indent() + "if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
			"  ctx = this->eventHandler_->getContext(" + serviceFuncName + ", nullptr);\n" + g.indent() +
			"}\n" + g.indent() +
			"::apache::thrift::TProcessorContextFreer freer(" +
			"this->eventHandler_.get(), ctx, " + serviceFuncName + ");\n\n" +
			g.indent() + "if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
			"  this->eventHandler_->preWrite(ctx, " + serviceFuncName + ");\n" +
			g.indent() + "}\n\n" + g.indent() + "oprot->writeMessageBegin(\"" +
			f.Name() + "\", ::apache::thrift::protocol::T_REPLY, seqid);\n" +
			g.indent() + "result.write(oprot);\n" + g.indent() + "oprot->writeMessageEnd();\n" +
			g.indent() + "uint32_t bytes = oprot->getTransport()->writeEnd();\n" +
			g.indent() + "oprot->getTransport()->flush();\n" + g.indent() +
			"if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
			"  this->eventHandler_->postWrite(ctx, " + serviceFuncName + ", bytes);\n" + g.indent() + "}\n" + g.indent() +
			"return cob(true);\n")
		g.scopeDown(out)
		out.WriteString("\n")
	}

	// Exception return.
	if !f.IsOneway() && len(xceptions) != 0 {
		if g.opts.Templates {
			out.WriteString(g.indent() + "template <class Protocol_>\n")
		}
		out.WriteString("void " + s.Name() + "AsyncProcessor" + classSuffix + "::throw_" +
			f.Name() + "(::std::function<void(bool ok)> cob, int32_t seqid, " +
			protType + "* oprot, void* ctx, " +
			"::apache::thrift::TDelayedException* _throw)\n")
		g.scopeUp(out)

		if g.opts.Templates && !specialized {
			// If oprot is a Protocol_ instance, use the specialized throw
			// function instead.
			out.WriteString(g.indent() + "Protocol_* _oprot = dynamic_cast<Protocol_*>(oprot);\n" + g.indent() +
				"if (_oprot) {\n" + g.indent() + "  return throw_" +
				f.Name() + "(cob, seqid, _oprot, ctx, _throw);\n" + g.indent() +
				"}\n" + g.indent() + "T_GENERIC_PROTOCOL(this, oprot, _oprot);\n\n")
		}

		// Get the event handler context.
		out.WriteString("\n" + g.indent() + "if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
			"  ctx = this->eventHandler_->getContext(" + serviceFuncName + ", nullptr);\n" + g.indent() +
			"}\n" + g.indent() +
			"::apache::thrift::TProcessorContextFreer freer(" +
			"this->eventHandler_.get(), ctx, " + serviceFuncName + ");\n\n")

		// Throw the TDelayedException and catch the result.
		out.WriteString(g.indent() + s.Name() + "_" + f.Name() + "_result result;\n\n" + g.indent() + "try {\n")
		g.indentUp()
		out.WriteString(g.indent() + "_throw->throw_it();\n" + g.indent() + "return cob(false);\n")
		g.indentDown()
		out.WriteString(g.indent() + "}")
		for _, x := range xceptions {
			out.WriteString("  catch (" + g.typeName(x.Type(), false, false) + " &" + x.Name() + ") {\n")
			g.indentUp()
			out.WriteString(g.indent() + "result." + x.Name() + " = " + x.Name() +
				";\n" + g.indent() + "result.__isset." + x.Name() + " = true;\n")
			g.scopeDown(out)
		}

		// Handle the case where an undeclared exception is thrown.
		out.WriteString(" catch (std::exception& e) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
			"  this->eventHandler_->handlerError(ctx, " + serviceFuncName + ");\n" + g.indent() +
			"}\n\n" + g.indent() +
			"::apache::thrift::TApplicationException x(e.what());\n" + g.indent() +
			"oprot->writeMessageBegin(\"" + f.Name() +
			"\", ::apache::thrift::protocol::T_EXCEPTION, seqid);\n" + g.indent() +
			"x.write(oprot);\n" + g.indent() + "oprot->writeMessageEnd();\n" + g.indent() +
			"oprot->getTransport()->writeEnd();\n" + g.indent() +
			"oprot->getTransport()->flush();\n" +
			// The cob argument is true here, since a response was
			// successfully written, even though it is an exception.
			g.indent() + "return cob(true);\n")
		g.scopeDown(out)

		// Serialize the result into a struct.
		out.WriteString(g.indent() + "if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
			"  this->eventHandler_->preWrite(ctx, " + serviceFuncName + ");\n" + g.indent() +
			"}\n\n" + g.indent() + "oprot->writeMessageBegin(\"" +
			f.Name() + "\", ::apache::thrift::protocol::T_REPLY, seqid);\n" +
			g.indent() + "result.write(oprot);\n" + g.indent() + "oprot->writeMessageEnd();\n" +
			g.indent() + "uint32_t bytes = oprot->getTransport()->writeEnd();\n" +
			g.indent() + "oprot->getTransport()->flush();\n" + g.indent() +
			"if (this->eventHandler_.get() != nullptr) {\n" + g.indent() +
			"  this->eventHandler_->postWrite(ctx, " + serviceFuncName + ", bytes);\n" + g.indent() + "}\n" + g.indent() +
			"return cob(true);\n")
		g.scopeDown(out)
		out.WriteString("\n")
	}
}

// generateServiceSkeleton is t_cpp_generator::generate_service_skeleton: a
// skeleton file illustrating how to build a server.
func (g *Generator) generateServiceSkeleton(s *sema.Service) {
	svcname := s.Name()

	fSkeletonName := g.outDir() + svcname + "_server.skeleton.cpp"

	ns := namespacePrefix(s.Program().Namespace("cpp"))

	var fSkeleton strings.Builder
	fSkeleton.WriteString("// This autogenerated skeleton file illustrates how to build a server.\n" +
		"// You should copy it to another filename to avoid overwriting it.\n\n" +
		"#include \"" + g.getIncludePrefix(g.program) + svcname + ".h\"\n" +
		"#include <thrift/protocol/TBinaryProtocol.h>\n" +
		"#include <thrift/server/TSimpleServer.h>\n" +
		"#include <thrift/transport/TServerSocket.h>\n" +
		"#include <thrift/transport/TBufferTransports.h>\n\n" +
		"using namespace ::apache::thrift;\n" +
		"using namespace ::apache::thrift::protocol;\n" +
		"using namespace ::apache::thrift::transport;\n" +
		"using namespace ::apache::thrift::server;\n\n")

	// "using namespace ;" and "using namespace ::;" would not compile.
	if ns != "" && ns != " ::" {
		fSkeleton.WriteString("using namespace " + ns[:len(ns)-2] + ";\n\n")
	}

	fSkeleton.WriteString("class " + svcname + "Handler : virtual public " + svcname + "If {\n public:\n")
	g.indentUp()
	fSkeleton.WriteString(g.indent() + svcname + "Handler() {\n" + g.indent() +
		"  // Your initialization goes here\n" + g.indent() + "}\n\n")

	for _, f := range s.Functions() {
		g.generateJavaDocFunction(&fSkeleton, f)
		fSkeleton.WriteString(g.indent() + g.functionSignature(f, "", "", true) + " {\n" + g.indent() +
			"  // Your implementation goes here\n" + g.indent() + "  printf(\"" +
			f.Name() + "\\n\");\n" + g.indent() + "}\n\n")
	}

	g.indentDown()
	fSkeleton.WriteString("};\n\n")

	fSkeleton.WriteString(g.indent() + "int main(int argc, char **argv) {\n")
	g.indentUp()
	fSkeleton.WriteString(
		g.indent() + "int port = 9090;\n" + g.indent() + "::std::shared_ptr<" + svcname +
			"Handler> handler(new " + svcname + "Handler());\n" + g.indent() +
			"::std::shared_ptr<TProcessor> processor(new " + svcname + "Processor(handler));\n" +
			g.indent() + "::std::shared_ptr<TServerTransport> serverTransport(new TServerSocket(port));\n" +
			g.indent() +
			"::std::shared_ptr<TTransportFactory> transportFactory(new TBufferedTransportFactory());\n" +
			g.indent() + "::std::shared_ptr<TProtocolFactory> protocolFactory(new TBinaryProtocolFactory());\n" +
			"\n" + g.indent() +
			"TSimpleServer server(processor, serverTransport, transportFactory, protocolFactory);\n" +
			g.indent() + "server.serve();\n" + g.indent() + "return 0;\n")
	g.indentDown()
	fSkeleton.WriteString("}\n\n")

	writeFile(fSkeletonName, fSkeleton.String())
}

// generateServiceAsyncSkeleton is t_cpp_generator::generate_service_async_skeleton.
func (g *Generator) generateServiceAsyncSkeleton(s *sema.Service) {
	svcname := s.Name()

	fSkeletonName := g.outDir() + svcname + "_async_server.skeleton.cpp"

	ns := namespacePrefix(s.Program().Namespace("cpp"))

	var fSkeleton strings.Builder
	fSkeleton.WriteString("// This autogenerated skeleton file illustrates one way to adapt a synchronous\n" +
		"// interface into an asynchronous interface. You should copy it to another\n" +
		"// filename to avoid overwriting it and rewrite as asynchronous any functions\n" +
		"// that would otherwise introduce unwanted latency.\n\n" +
		"#include \"" + g.getIncludePrefix(g.program) + svcname + ".h\"\n" +
		"#include <thrift/protocol/TBinaryProtocol.h>\n" +
		"#include <thrift/async/TAsyncProtocolProcessor.h>\n" +
		"#include <thrift/async/TEvhttpServer.h>\n" +
		"#include <event.h>\n" +
		"#include <evhttp.h>\n\n" +
		"using namespace ::apache::thrift;\n" +
		"using namespace ::apache::thrift::protocol;\n" +
		"using namespace ::apache::thrift::transport;\n" +
		"using namespace ::apache::thrift::async;\n\n")

	if ns != "" && ns != " ::" {
		fSkeleton.WriteString("using namespace " + ns[:len(ns)-2] + ";\n\n")
	}

	fSkeleton.WriteString("class " + svcname + "Handler : virtual public " + svcname + "If {\n public:\n")
	g.indentUp()
	fSkeleton.WriteString(g.indent() + svcname + "Handler() {\n" + g.indent() +
		"  // Your initialization goes here\n" + g.indent() + "}\n\n")

	functions := s.Functions()
	for _, f := range functions {
		g.generateJavaDocFunction(&fSkeleton, f)
		fSkeleton.WriteString(g.indent() + g.functionSignature(f, "", "", true) + " {\n" + g.indent() +
			"  // Your implementation goes here\n" + g.indent() + "  printf(\"" +
			f.Name() + "\\n\");\n" + g.indent() + "}\n\n")
	}

	g.indentDown()
	fSkeleton.WriteString("};\n\n")

	fSkeleton.WriteString("class " + svcname + "AsyncHandler : " +
		"public " + svcname + "CobSvIf {\n public:\n")
	g.indentUp()
	fSkeleton.WriteString(g.indent() + svcname + "AsyncHandler() {\n" + g.indent() +
		"  syncHandler_ = std::unique_ptr<" + svcname + "Handler>(new " + svcname +
		"Handler);\n" + g.indent() + "  // Your initialization goes here\n" +
		g.indent() + "}\n")
	fSkeleton.WriteString(g.indent() + "virtual ~" + g.serviceName + "AsyncHandler();\n")

	for _, f := range functions {
		fSkeleton.WriteString("\n" + g.indent() + g.functionSignature(f, "CobSv", "", true) + " {" +
			"\n")
		g.indentUp()

		returntype := f.ReturnType()
		returnfield := sema.NewField(returntype, "_return", 0)

		target := "_return"
		if returntype.IsVoid() {
			target = ""
		}
		if !returntype.IsVoid() {
			fSkeleton.WriteString(g.indent() + g.declareField(returnfield, true, false, false, false) + "\n")
		}
		g.generateFunctionCall(&fSkeleton, f, target, "syncHandler_", "")
		fSkeleton.WriteString(g.indent() + "return cob(" + target + ");\n")

		g.scopeDown(&fSkeleton)
	}
	fSkeleton.WriteString("\n protected:\n" + g.indent() + "std::unique_ptr<" + svcname +
		"Handler> syncHandler_;\n")
	g.indentDown()
	fSkeleton.WriteString("};\n\n")

	fSkeleton.WriteString(g.indent() + "int main(int argc, char **argv) {\n")
	g.indentUp()
	fSkeleton.WriteString(
		g.indent() + "int port = 9090;\n" + g.indent() + "::std::shared_ptr<" + svcname +
			"AsyncHandler> handler(new " + svcname + "AsyncHandler());\n" + g.indent() +
			"::std::shared_ptr<" + svcname + "AsyncProcessor> processor(new " + svcname + "AsyncProcessor(handler));\n" +
			g.indent() + "::std::shared_ptr<TProtocolFactory> protocolFactory(new TBinaryProtocolFactory());" +
			"\n" +
			g.indent() + "::std::shared_ptr<TAsyncProtocolProcessor> protocolProcessor(new TAsyncProtocolProcessor(processor, protocolFactory));" +
			"\n\n" + g.indent() +
			"TEvhttpServer server(protocolProcessor, port);" +
			"\n" + g.indent() + "server.serve();\n" + g.indent() + "return 0;\n")
	g.indentDown()
	fSkeleton.WriteString("}\n\n")

	writeFile(fSkeletonName, fSkeleton.String())
}
