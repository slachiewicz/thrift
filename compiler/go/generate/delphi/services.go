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

package delphi

import (
	"github.com/apache/thrift/compiler/go/sema"
)

// functionSignature is t_delphi_generator::function_signature.
func (g *Generator) functionSignature(tfunction *sema.Function, forAsync bool, fullCls string, isXception bool) string {
	ttype := tfunction.ReturnType()
	prefix := ""
	if fullCls != "" {
		prefix = fullCls + "."
	}

	signature := ""

	if forAsync {
		if isVoid(ttype) {
			signature = "function " + prefix + g.normalizeName(tfunction.Name(), true, isXception, false) + "Async(" +
				g.argumentList(tfunction.Arglist()) + "): IFuture<Integer>;"
		} else {
			signature = "function " + prefix + g.normalizeName(tfunction.Name(), true, isXception, false) + "Async(" +
				g.argumentList(tfunction.Arglist()) + "): IFuture<" + g.typeName(ttype, false, true) + ">;"
		}
	} else {
		if isVoid(ttype) {
			signature = "procedure " + prefix + g.normalizeName(tfunction.Name(), true, isXception, false) + "(" +
				g.argumentList(tfunction.Arglist()) + ");"
		} else {
			signature = "function " + prefix + g.normalizeName(tfunction.Name(), true, isXception, false) + "(" +
				g.argumentList(tfunction.Arglist()) + "): " + g.typeName(ttype, false, true) + ";"
		}
	}

	if fullCls == "" {
		signature += renderDeprecationAttribute(tfunction.Annotations(), " ", ";")
	}

	return signature
}

// sendSignature and recvSignature build the signature of the synthetic
// send_<f>/recv_<f> helper functions the way function_signature would if
// called on the send_function/recv_function objects the C++ generator
// constructs inline (a void-returning function taking f's own argument
// list, and a function returning f's return type taking no arguments).
func (g *Generator) sendSignature(f *sema.Function, fullCls string) string {
	prefix := ""
	if fullCls != "" {
		prefix = fullCls + "."
	}
	return "procedure " + prefix + g.normalizeName("send_"+f.Name(), true, false, false) + "(" + g.argumentList(f.Arglist()) + ");"
}

func (g *Generator) recvSignature(f *sema.Function, fullCls string) string {
	prefix := ""
	if fullCls != "" {
		prefix = fullCls + "."
	}
	ttype := f.ReturnType()
	if isVoid(ttype) {
		return "procedure " + prefix + g.normalizeName("recv_"+f.Name(), true, false, false) + "();"
	}
	return "function " + prefix + g.normalizeName("recv_"+f.Name(), true, false, false) + "(): " + g.typeName(ttype, false, true) + ";"
}

// generateService is t_delphi_generator::generate_service.
func (g *Generator) generateService(tservice *sema.Service) {
	g.indentUp()
	g.generateDelphiDoc(&g.sService, tservice)
	g.ln(&g.sService, g.normalizeClsnm(g.serviceName, "T")+" = class")
	g.ln(&g.sService, "public")
	g.indentUp()
	g.ln(&g.sService, "type")
	g.generateServiceInterface(tservice)
	g.generateServiceClient(tservice)
	g.generateServiceServer(tservice)
	g.generateServiceHelpers(tservice)
	g.indentDown()
	g.indentDown()
	g.ln(&g.sService, "end;")
	g.ln(&g.sService, "")
	g.indentDown()
}

// generateServiceInterface is t_delphi_generator::generate_service_interface
// (the one-argument overload).
func (g *Generator) generateServiceInterface(tservice *sema.Service) {
	g.generateServiceInterfaceFor(tservice, false)
	if g.opts.Async {
		g.generateServiceInterfaceFor(tservice, true)
	}
}

// generateServiceInterfaceFor is t_delphi_generator::generate_service_interface
// (the two-argument overload).
func (g *Generator) generateServiceInterfaceFor(tservice *sema.Service, forAsync bool) {
	extends := ""
	extendsIface := ""
	ifaceName := "Iface"
	if forAsync {
		ifaceName = "IAsync"
	}

	g.indentUp()

	g.generateDelphiDoc(&g.sService, tservice)
	if tservice.Extends() != nil {
		extends = g.typeName(tservice.Extends(), true, true)
		extendsIface = extends + "." + ifaceName
		g.generateDelphiDoc(&g.sService, tservice)
		g.ln(&g.sService, ifaceName+" = interface("+extendsIface+")")
	} else {
		g.ln(&g.sService, ifaceName+" = interface")
	}

	g.indentUp()
	if g.opts.GuidV4 {
		g.generateGuid(&g.sService)
	} else {
		g.generateGuidV8Service(&g.sService, tservice)
	}
	for _, f := range tservice.Functions() {
		g.generateDelphiDocFunction(&g.sService, f)
		g.ln(&g.sService, g.functionSignature(f, forAsync, "", false))
	}
	g.indentDown()
	g.ln(&g.sService, "end;"+renderDeprecationAttribute(tservice.Annotations(), " {", "}"))
	g.raw(&g.sService, "\n")

	g.indentDown()
}

// generateServiceHelpers is t_delphi_generator::generate_service_helpers.
func (g *Generator) generateServiceHelpers(tservice *sema.Service) {
	for _, f := range tservice.Functions() {
		ts := f.Arglist()
		g.generateDelphiStructDefinition(&g.sService, ts)
		g.generateDelphiStructImpl(&g.sServiceImpl, g.normalizeClsnm(g.serviceName, "T")+".", ts, false, false)
		g.generateFunctionHelpers(f)
	}
}

// generateFunctionHelpers is t_delphi_generator::generate_function_helpers.
func (g *Generator) generateFunctionHelpers(tfunction *sema.Function) {
	if tfunction.IsOneway() {
		return
	}

	result := sema.NewStruct(g.program)
	result.SetName(tfunction.Name() + "_result")
	if !tfunction.ReturnType().IsVoid() {
		result.Append(sema.NewField(tfunction.ReturnType(), "Success", 0))
	}

	for _, f := range tfunction.Xceptions().Members() {
		result.Append(f)
	}

	g.generateDelphiStructDefinition(&g.sService, result)
	g.generateDelphiStructImpl(&g.sServiceImpl, g.normalizeClsnm(g.serviceName, "T")+".", result, true, false)
}

// generateServiceClient is t_delphi_generator::generate_service_client.
func (g *Generator) generateServiceClient(tservice *sema.Service) {
	g.indentUp()
	extends := ""
	extendsClient := "TInterfacedObject"
	implements := "Iface"
	if g.opts.Async {
		implements = "Iface, IAsync"
	}

	g.generateDelphiDoc(&g.sService, tservice)
	if tservice.Extends() != nil {
		extends = g.typeName(tservice.Extends(), true, true)
		extendsClient = extends + ".TClient"
	}
	g.ln(&g.sService, "TClient = class( "+extendsClient+", "+implements+")")

	g.ln(&g.sService, "public")
	g.indentUp()

	g.ln(&g.sService, "constructor Create( prot: IProtocol); overload;")

	g.lnI(&g.sServiceImpl, "constructor "+g.normalizeClsnm(g.serviceName, "T")+".TClient.Create( prot: IProtocol);")
	g.lnI(&g.sServiceImpl, "begin")
	g.indentUpImpl()
	g.lnI(&g.sServiceImpl, "Create( prot, prot );")
	g.indentDownImpl()
	g.lnI(&g.sServiceImpl, "end;")
	g.raw(&g.sServiceImpl, "\n")

	g.ln(&g.sService, "constructor Create( const iprot: IProtocol; const oprot: IProtocol); overload;")

	g.lnI(&g.sServiceImpl, "constructor "+g.normalizeClsnm(g.serviceName, "T")+".TClient.Create( const iprot: IProtocol; const oprot: IProtocol);")
	g.lnI(&g.sServiceImpl, "begin")
	g.indentUpImpl()
	g.lnI(&g.sServiceImpl, "inherited Create;")
	g.lnI(&g.sServiceImpl, "iprot_ := iprot;")
	g.lnI(&g.sServiceImpl, "oprot_ := oprot;")
	g.indentDownImpl()
	g.lnI(&g.sServiceImpl, "end;")
	g.raw(&g.sServiceImpl, "\n")

	g.indentDown()

	if extends == "" {
		g.ln(&g.sService, "protected")
		g.indentUp()
		g.ln(&g.sService, "iprot_: IProtocol;")
		g.ln(&g.sService, "oprot_: IProtocol;")
		g.ln(&g.sService, "seqid_: System.Integer;")
		g.indentDown()

		g.ln(&g.sService, "public")
		g.indentUp()
		g.ln(&g.sService, "property InputProtocol: IProtocol read iprot_;")
		g.ln(&g.sService, "property OutputProtocol: IProtocol read oprot_;")
		g.indentDown()
	}

	functions := tservice.Functions()

	g.ln(&g.sService, "protected")
	g.indentUp()

	g.ln(&g.sService, "// Iface")
	for _, f := range functions {
		g.generateDelphiDocFunction(&g.sService, f)
		g.ln(&g.sService, g.functionSignature(f, false, "", false))
	}

	if g.opts.Async {
		g.ln(&g.sService, "")
		g.ln(&g.sService, "// IAsync")
		for _, f := range functions {
			g.generateDelphiDocFunction(&g.sService, f)
			g.ln(&g.sService, g.functionSignature(f, true, "", false))
		}
	}

	g.indentDown()

	g.ln(&g.sService, "public")
	g.indentUp()

	fullCls := g.normalizeClsnm(g.serviceName, "T") + ".TClient"

	for _, f := range functions {
		funname := f.Name()
		argStruct := f.Arglist()
		fields := argStruct.Members()

		// one for sync only, two for async+sync
		mode := 0
		if g.opts.Async {
			mode = 1
		}
		for mode >= 0 {
			forAsync := mode != 0
			mode--

			if forAsync {
				g.lnI(&g.sServiceImpl, g.functionSignature(f, true, fullCls, false))
			} else {
				g.lnI(&g.sServiceImpl, g.functionSignature(f, false, fullCls, false))
			}
			g.lnI(&g.sServiceImpl, "begin")
			g.indentUpImpl()

			ttype := f.ReturnType()
			if forAsync {
				if isVoid(ttype) {
					// Delphi forces us to specify a type with IFuture<T>, so we use Integer=0 for void methods
					g.lnI(&g.sServiceImpl, "result := TTask.Future<System.Integer>(function: System.Integer")
				} else {
					rettype := g.typeName(ttype, false, true)
					g.lnI(&g.sServiceImpl, "result := TTask.Future<"+rettype+">(function: "+rettype)
				}
				g.lnI(&g.sServiceImpl, "begin")
				g.indentUpImpl()
			}

			g.wrI(&g.sServiceImpl, "send_"+funname+"(")

			first := true
			for _, fld := range fields {
				if first {
					first = false
				} else {
					g.raw(&g.sServiceImpl, ", ")
				}
				g.raw(&g.sServiceImpl, g.normalizeNameSimple(fld.Name()))
			}
			g.raw(&g.sServiceImpl, ");\n")

			if !f.IsOneway() {
				g.raw(&g.sServiceImpl, g.indentImpl())
				if !f.ReturnType().IsVoid() {
					g.raw(&g.sServiceImpl, "Result := ")
				}
				g.raw(&g.sServiceImpl, "recv_"+funname+"();\n")
			}

			if forAsync {
				if isVoid(ttype) {
					g.lnI(&g.sServiceImpl, "Result := 0;") // no IFuture<void> in Delphi
				}
				g.indentDownImpl()
				g.lnI(&g.sServiceImpl, "end);")
			}

			g.indentDownImpl()
			g.lnI(&g.sServiceImpl, "end;")
			g.raw(&g.sServiceImpl, "\n")
		}

		argsname := f.Name() + "_args"
		argsClsnm := g.normalizeClsnm(argsname, "T")
		argsIntfnm := g.normalizeClsnm(argsname, "I")

		argsvar := g.tmp("_args")
		msgvar := g.tmp("_msg")

		g.ln(&g.sService, g.sendSignature(f, ""))
		g.lnI(&g.sServiceImpl, g.sendSignature(f, fullCls))
		g.lnI(&g.sServiceImpl, "var")
		g.indentUpImpl()
		g.lnI(&g.sServiceImpl, argsvar+" : "+argsIntfnm+";")
		g.lnI(&g.sServiceImpl, msgvar+" : Thrift.Protocol.TThriftMessage;")
		g.indentDownImpl()
		g.lnI(&g.sServiceImpl, "begin")
		g.indentUpImpl()

		g.lnI(&g.sServiceImpl, "seqid_ := seqid_ + 1;")
		msgType := "TMessageType.Call"
		if f.IsOneway() {
			msgType = "TMessageType.Oneway"
		}
		g.lnI(&g.sServiceImpl, "Thrift.Protocol.Init( "+msgvar+", '"+funname+"', "+msgType+", seqid_);")

		g.lnI(&g.sServiceImpl, "oprot_.WriteMessageBegin( "+msgvar+" );")
		g.lnI(&g.sServiceImpl, argsvar+" := "+argsClsnm+"Impl.Create();")

		for _, fld := range fields {
			g.lnI(&g.sServiceImpl, argsvar+"."+g.propNameF(fld, false, "")+" := "+g.normalizeNameSimple(fld.Name())+";")
		}
		g.lnI(&g.sServiceImpl, argsvar+".Write(oprot_);")
		for _, fld := range fields {
			g.lnI(&g.sServiceImpl, argsvar+"."+g.propNameF(fld, false, "")+" := "+g.emptyValue(fld.Type())+";")
		}

		g.lnI(&g.sServiceImpl, "oprot_.WriteMessageEnd();")
		g.lnI(&g.sServiceImpl, "oprot_.Transport.Flush();")

		g.indentDownImpl()
		g.lnI(&g.sServiceImpl, "end;")
		g.raw(&g.sServiceImpl, "\n")

		if !f.IsOneway() {
			orgResultname := f.Name() + "_result"
			resultClsnm := g.normalizeClsnm(orgResultname, "T")
			resultIntfnm := g.normalizeClsnm(orgResultname, "I")

			xceptions := f.Xceptions().Members()

			exceptvar := g.tmp("_ex")
			appexvar := g.tmp("_ax")
			retvar := g.tmp("_ret")

			g.ln(&g.sService, g.recvSignature(f, ""))
			g.lnI(&g.sServiceImpl, g.recvSignature(f, fullCls))
			g.lnI(&g.sServiceImpl, "var")
			g.indentUpImpl()
			g.lnI(&g.sServiceImpl, msgvar+" : Thrift.Protocol.TThriftMessage;")
			if len(xceptions) > 0 {
				g.lnI(&g.sServiceImpl, exceptvar+" : Exception;")
			}
			g.lnI(&g.sServiceImpl, appexvar+" : TApplicationException;")
			g.lnI(&g.sServiceImpl, retvar+" : "+resultIntfnm+";")

			g.indentDownImpl()
			g.lnI(&g.sServiceImpl, "begin")
			g.indentUpImpl()
			g.lnI(&g.sServiceImpl, msgvar+" := iprot_.ReadMessageBegin();")
			g.lnI(&g.sServiceImpl, "if ("+msgvar+".Type_ = TMessageType.Exception) then begin")
			g.indentUpImpl()
			g.lnI(&g.sServiceImpl, appexvar+" := TApplicationException.Read(iprot_);")
			g.lnI(&g.sServiceImpl, "iprot_.ReadMessageEnd();")
			g.lnI(&g.sServiceImpl, "raise "+appexvar+";")
			g.indentDownImpl()
			g.lnI(&g.sServiceImpl, "end;")

			g.lnI(&g.sServiceImpl, retvar+" := "+resultClsnm+"Impl.Create();")
			g.lnI(&g.sServiceImpl, retvar+".Read(iprot_);")
			g.lnI(&g.sServiceImpl, "iprot_.ReadMessageEnd();")

			if !f.ReturnType().IsVoid() {
				g.lnI(&g.sServiceImpl, "if ("+retvar+".__isset_success) then begin")
				g.indentUpImpl()
				g.lnI(&g.sServiceImpl, "Result := "+retvar+".Success;")
				typ := f.ReturnType()
				if typ.IsStruct() || typ.IsXception() || typ.IsMap() || typ.IsList() || typ.IsSet() {
					g.lnI(&g.sServiceImpl, retvar+".Success := nil;")
				}
				g.lnI(&g.sServiceImpl, "Exit;")
				g.indentDownImpl()
				g.lnI(&g.sServiceImpl, "end;")
			}

			for _, x := range xceptions {
				g.lnI(&g.sServiceImpl, "if ("+retvar+"."+g.propNameF(x, false, "__isset_")+") then begin")
				g.indentUpImpl()
				g.lnI(&g.sServiceImpl, exceptvar+" := "+g.typeName(x.Type(), true, true)+".Create("+retvar+"."+g.propNameF(x, false, "")+");")
				g.lnI(&g.sServiceImpl, "raise "+exceptvar+";")
				g.indentDownImpl()
				g.lnI(&g.sServiceImpl, "end;")
			}

			if !f.ReturnType().IsVoid() {
				g.lnI(&g.sServiceImpl, "raise TApplicationExceptionMissingResult.Create('"+f.Name()+" failed: unknown result');")
			}

			g.indentDownImpl()
			g.lnI(&g.sServiceImpl, "end;")
			g.raw(&g.sServiceImpl, "\n")
		}
	}

	g.indentDown()
	g.ln(&g.sService, "end;"+renderDeprecationAttribute(tservice.Annotations(), " {", "}"))
	g.raw(&g.sService, "\n")
}
