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

package rs

import (
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

//-----------------------------------------------------------------------------
//
// Sync Client
//
//-----------------------------------------------------------------------------

// generateService is t_rs_generator::generate_service.
func (g *generator) generateService(s *sema.Service) {
	g.renderSyncClient(s)
	g.renderSyncProcessor(s)
	g.renderServiceCallStructs(s)
}

// renderServiceCallStructs is t_rs_generator::render_service_call_structs.
func (g *generator) renderServiceCallStructs(s *sema.Service) {
	// thrift args for service calls are packed
	// into a struct that's transmitted over the wire, so
	// generate structs for those too
	//
	// thrift returns are *also* packed into a struct
	// that's passed over the wire, so, generate the struct
	// for that too. Note that this result struct *also*
	// contains the exceptions as well
	for _, fn := range s.Functions() {
		g.renderServiceCallArgsStruct(fn)
		if !fn.IsOneway() {
			g.renderServiceCallResultValueStruct(fn)
		}
	}
}

// renderSyncClient is t_rs_generator::render_sync_client.
func (g *generator) renderSyncClient(s *sema.Service) {
	clientImplName := rustSyncClientImplName(s)

	// note: use *original* name
	g.renderTypeComment(s.Name() + " service client")
	g.renderSyncClientTrait(s)
	g.renderSyncClientMarkerTrait(s)
	g.renderSyncClientDefinitionAndImpl(clientImplName)
	g.renderSyncClientTThriftClientImpl(clientImplName)
	g.renderSyncClientMarkerTraitImpls(s, clientImplName)
	g.wl("")
	g.renderSyncClientProcessImpl(s)
}

// renderSyncClientTrait is t_rs_generator::render_sync_client_trait.
func (g *generator) renderSyncClientTrait(s *sema.Service) {
	extension := ""
	if extends := s.Extends(); extends != nil {
		extension = " : " + g.rustNamespaceService(extends) + rustSyncClientTraitName(extends)
	}

	g.renderRustdoc(s)
	g.wl("pub trait " + rustSyncClientTraitName(s) + extension + " {")
	g.up()

	for _, fn := range s.Functions() {
		funcName := serviceCallClientFunctionName(fn)
		funcArgs := g.rustSyncServiceCallDeclaration(fn, true)
		funcReturn := g.toRustType(fn.ReturnType())
		g.renderRustdoc(fn)
		g.line("fn " + funcName + funcArgs + " -> thrift::Result<" + funcReturn + ">;")
	}

	g.down()
	g.line("}")
	g.wl("")
}

// renderSyncClientMarkerTrait is
// t_rs_generator::render_sync_client_marker_trait.
func (g *generator) renderSyncClientMarkerTrait(s *sema.Service) {
	g.line("pub trait " + rustSyncClientMarkerTraitName(s) + " {}")
	g.wl("")
}

// renderSyncClientMarkerTraitImpls is
// t_rs_generator::render_sync_client_marker_trait_impls.
func (g *generator) renderSyncClientMarkerTraitImpls(s *sema.Service, implStructName string) {
	g.line("impl " + syncClientGenericBoundVars + " " + g.rustNamespaceService(s) + rustSyncClientMarkerTraitName(s) + " for " + implStructName + syncClientGenericBoundVars + " " + syncClientGenericBounds + " {}")

	if extends := s.Extends(); extends != nil {
		g.renderSyncClientMarkerTraitImpls(extends, implStructName)
	}
}

// renderSyncClientDefinitionAndImpl is
// t_rs_generator::render_sync_client_definition_and_impl.
func (g *generator) renderSyncClientDefinitionAndImpl(clientImplName string) {
	// render the definition for the client struct
	g.wl("pub struct " + clientImplName + syncClientGenericBoundVars + " " + syncClientGenericBounds + " {")
	g.up()
	g.line("_i_prot: IP,")
	g.line("_o_prot: OP,")
	g.line("_sequence_number: i32,")
	g.down()
	g.wl("}")
	g.wl("")

	// render the struct implementation
	// this includes the new() function as well as the helper send/recv methods for each service call
	g.wl("impl " + syncClientGenericBoundVars + " " + clientImplName + syncClientGenericBoundVars + " " + syncClientGenericBounds + " {")
	g.up()
	g.renderSyncClientLifecycleFunctions(clientImplName)
	g.down()
	g.wl("}")
	g.wl("")
}

// renderSyncClientLifecycleFunctions is
// t_rs_generator::render_sync_client_lifecycle_functions.
func (g *generator) renderSyncClientLifecycleFunctions(clientStruct string) {
	g.line("pub fn new(input_protocol: IP, output_protocol: OP) -> " + clientStruct + syncClientGenericBoundVars + " {")
	g.up()

	g.line(clientStruct + " { _i_prot: input_protocol, _o_prot: output_protocol, _sequence_number: 0 }")

	g.down()
	g.line("}")
}

// renderSyncClientTThriftClientImpl is
// t_rs_generator::render_sync_client_tthriftclient_impl.
func (g *generator) renderSyncClientTThriftClientImpl(clientImplName string) {
	g.line("impl " + syncClientGenericBoundVars + " TThriftClient for " + clientImplName + syncClientGenericBoundVars + " " + syncClientGenericBounds + " {")
	g.up()

	g.line("fn i_prot_mut(&mut self) -> &mut dyn TInputProtocol { &mut self._i_prot }")
	g.line("fn o_prot_mut(&mut self) -> &mut dyn TOutputProtocol { &mut self._o_prot }")
	g.line("fn sequence_number(&self) -> i32 { self._sequence_number }")
	g.line("fn increment_sequence_number(&mut self) -> i32 { self._sequence_number += 1; self._sequence_number }")

	g.down()
	g.line("}")
	g.wl("")
}

// renderSyncClientProcessImpl is
// t_rs_generator::render_sync_client_process_impl.
func (g *generator) renderSyncClientProcessImpl(s *sema.Service) {
	markerExtension := "" + g.syncClientMarkerTraitsForExtension(s)

	g.wl("impl <C: TThriftClient + " + rustSyncClientMarkerTraitName(s) + markerExtension + "> " + rustSyncClientTraitName(s) + " for C {")
	g.up()

	for _, fn := range s.Functions() {
		g.renderSyncSendRecvWrapper(fn)
	}

	g.down()
	g.wl("}")
	g.wl("")
}

// syncClientMarkerTraitsForExtension is
// t_rs_generator::sync_client_marker_traits_for_extension.
func (g *generator) syncClientMarkerTraitsForExtension(s *sema.Service) string {
	markerExtension := ""

	if extends := s.Extends(); extends != nil {
		markerExtension = " + " + g.rustNamespaceService(extends) + rustSyncClientMarkerTraitName(extends)
		markerExtension = markerExtension + g.syncClientMarkerTraitsForExtension(extends)
	}

	return markerExtension
}

// renderSyncSendRecvWrapper is t_rs_generator::render_sync_send_recv_wrapper.
func (g *generator) renderSyncSendRecvWrapper(fn *sema.Function) {
	funcName := serviceCallClientFunctionName(fn)
	funcDeclArgs := g.rustSyncServiceCallDeclaration(fn, true)
	funcReturn := g.toRustType(fn.ReturnType())

	g.line("fn " + funcName + funcDeclArgs + " -> thrift::Result<" + funcReturn + "> {")
	g.up()

	g.line("(")
	g.up()
	g.renderSyncSend(fn)
	g.down()
	g.line(")?;")
	if fn.IsOneway() {
		g.line("Ok(())")
	} else {
		g.renderSyncRecv(fn)
	}

	g.down()
	g.line("}")
}

// renderSyncSend is t_rs_generator::render_sync_send.
func (g *generator) renderSyncSend(fn *sema.Function) {
	g.line("{")
	g.up()

	// increment the sequence number and generate the call header
	messageType := "TMessageType::Call"
	if fn.IsOneway() {
		messageType = "TMessageType::OneWay"
	}
	g.line("self.increment_sequence_number();")
	// note: use *original* name
	g.line("let message_ident = TMessageIdentifier::new(\"" + fn.Name() + "\", " + messageType + ", self.sequence_number());")
	// pack the arguments into the containing struct that we'll write out over the wire
	// note that this struct is generated even if we have 0 args
	var structDefinition strings.Builder
	members := fn.Arglist().SortedMembers()
	for _, member := range members {
		memberName := rustFieldName(member)
		structDefinition.WriteString(memberName + ", ")
	}
	structFields := structDefinition.String()
	if len(structFields) > 0 {
		structFields = structFields[:len(structFields)-2] // strip trailing comma
	}
	g.line("let call_args = " + g.serviceCallArgsStructName(fn) + " { " + structFields + " };")
	// write everything over the wire
	g.line("self.o_prot_mut().write_message_begin(&message_ident)?;")
	g.line("call_args.write_to_out_protocol(self.o_prot_mut())?;") // written even if we have 0 args
	g.line("self.o_prot_mut().write_message_end()?;")
	g.line("self.o_prot_mut().flush()")

	g.down()
	g.line("}")
}

// renderSyncRecv is t_rs_generator::render_sync_recv.
func (g *generator) renderSyncRecv(fn *sema.Function) {
	g.line("{")
	g.up()

	g.line("let message_ident = self.i_prot_mut().read_message_begin()?;")
	g.line("verify_expected_sequence_number(self.sequence_number(), message_ident.sequence_number)?;")
	// note: use *original* name
	g.line("verify_expected_service_call(\"" + fn.Name() + "\", &message_ident.name)?;")
	// FIXME: replace with a "try" block
	g.line("if message_ident.message_type == TMessageType::Exception {")
	g.up()
	g.line("let remote_error = thrift::Error::read_application_error_from_in_protocol(self.i_prot_mut())?;")
	g.line("self.i_prot_mut().read_message_end()?;")
	g.line("return Err(thrift::Error::Application(remote_error))")
	g.down()
	g.line("}")
	g.line("verify_expected_message_type(TMessageType::Reply, message_ident.message_type)?;")
	g.line("let result = " + g.serviceCallResultStructName(fn) + "::read_from_in_protocol(self.i_prot_mut())?;")
	g.line("self.i_prot_mut().read_message_end()?;")
	g.line("result.ok_or()")

	g.down()
	g.line("}")
}

// rustSyncServiceCallDeclaration is
// t_rs_generator::rust_sync_service_call_declaration.
func (g *generator) rustSyncServiceCallDeclaration(fn *sema.Function, selfIsMutable bool) string {
	var funcArgs strings.Builder

	if selfIsMutable {
		funcArgs.WriteString("(&mut self")
	} else {
		funcArgs.WriteString("(&self")
	}

	if hasArgs(fn) {
		funcArgs.WriteString(", ") // put comma after "self"
		funcArgs.WriteString(g.structToDeclaration(fn.Arglist(), structArgs))
	}

	funcArgs.WriteString(")")
	return funcArgs.String()
}

// rustSyncServiceCallInvocation is
// t_rs_generator::rust_sync_service_call_invocation.
func (g *generator) rustSyncServiceCallInvocation(fn *sema.Function, fieldPrefix string) string {
	var funcArgs strings.Builder
	funcArgs.WriteString("(")

	if hasArgs(fn) {
		funcArgs.WriteString(g.structToInvocation(fn.Arglist(), fieldPrefix))
	}

	funcArgs.WriteString(")")
	return funcArgs.String()
}

// structToDeclaration is t_rs_generator::struct_to_declaration.
func (g *generator) structToDeclaration(s *sema.Struct, st structType) string {
	var args strings.Builder

	firstArg := true
	for _, tfield := range s.SortedMembers() {
		fieldReq := actualFieldReq(tfield, st)
		rustType := g.toRustType(tfield.Type())
		if isOptional(fieldReq) {
			rustType = "Option<" + rustType + ">"
		}

		if firstArg {
			firstArg = false
		} else {
			args.WriteString(", ")
		}

		args.WriteString(rustFieldName(tfield) + ": " + rustType)
	}

	return args.String()
}

// structToInvocation is t_rs_generator::struct_to_invocation.
func (g *generator) structToInvocation(s *sema.Struct, fieldPrefix string) string {
	var args strings.Builder

	firstArg := true
	for _, tfield := range s.SortedMembers() {
		if firstArg {
			firstArg = false
		} else {
			args.WriteString(", ")
		}

		args.WriteString(fieldPrefix + rustFieldName(tfield))
	}

	return args.String()
}

// renderServiceCallArgsStruct is
// t_rs_generator::render_service_call_args_struct.
func (g *generator) renderServiceCallArgsStruct(fn *sema.Function) {
	argsStructName := g.serviceCallArgsStructName(fn)
	g.renderStruct(argsStructName, fn.Arglist(), structArgs)
}

// renderServiceCallResultValueStruct is
// t_rs_generator::render_service_call_result_value_struct.
func (g *generator) renderServiceCallResultValueStruct(fn *sema.Function) {
	resultStructName := g.serviceCallResultStructName(fn)
	result := sema.NewStruct(g.program)
	result.SetName(resultStructName)

	if !fn.ReturnType().IsVoid() {
		returnValue := sema.NewField(fn.ReturnType(), serviceResultVariable, 0)
		returnValue.SetReq(sema.Optional)
		result.Append(returnValue)
	}

	exceptions := fn.Xceptions()
	for _, exceptionType := range exceptions.Members() {
		exceptionType.SetReq(sema.Optional)
		result.Append(exceptionType)
	}

	g.renderStruct(resultStructName, result, structResult)
}

//-----------------------------------------------------------------------------
//
// Sync Processor
//
//-----------------------------------------------------------------------------

// renderSyncProcessor is t_rs_generator::render_sync_processor.
func (g *generator) renderSyncProcessor(s *sema.Service) {
	// note: use *original* name
	g.renderTypeComment(s.Name() + " service processor")
	g.renderSyncHandlerTrait(s)
	g.renderSyncProcessorDefinitionAndImpl(s)
}

// renderSyncHandlerTrait is t_rs_generator::render_sync_handler_trait.
func (g *generator) renderSyncHandlerTrait(s *sema.Service) {
	extension := ""
	if extends := s.Extends(); extends != nil {
		extension = " : " + g.rustNamespaceService(extends) + rustSyncHandlerTraitName(extends)
	}

	g.renderRustdoc(s)
	g.wl("pub trait " + rustSyncHandlerTraitName(s) + extension + " {")
	g.up()
	for _, fn := range s.Functions() {
		funcName := serviceCallHandlerFunctionName(fn)
		funcArgs := g.rustSyncServiceCallDeclaration(fn, false)
		funcReturn := g.toRustType(fn.ReturnType())
		g.renderRustdoc(fn)
		g.line("fn " + funcName + funcArgs + " -> thrift::Result<" + funcReturn + ">;")
	}
	g.down()
	g.line("}")
	g.wl("")
}

// renderSyncProcessorDefinitionAndImpl is
// t_rs_generator::render_sync_processor_definition_and_impl.
func (g *generator) renderSyncProcessorDefinitionAndImpl(s *sema.Service) {
	serviceProcessorName := rustSyncProcessorName(s)
	handlerTraitName := rustSyncHandlerTraitName(s)

	// struct
	g.line("pub struct " + serviceProcessorName + "<H: " + handlerTraitName + "> {")
	g.up()
	g.line("pub handler: H,")
	g.down()
	g.line("}")
	g.wl("")

	// delegating impl
	g.line("impl <H: " + handlerTraitName + "> " + serviceProcessorName + "<H> {")
	g.up()
	g.line("pub fn new(handler: H) -> " + serviceProcessorName + "<H> {")
	g.up()
	g.line(serviceProcessorName + " {")
	g.up()
	g.line("handler,")
	g.down()
	g.line("}")
	g.down()
	g.line("}")
	g.renderSyncProcessDelegationFunctions(s)
	g.down()
	g.line("}")
	g.wl("")

	// actual impl
	serviceActualProcessorName := rustSyncProcessorImplName(s)
	g.line("pub struct " + serviceActualProcessorName + ";")
	g.wl("")
	g.line("impl " + serviceActualProcessorName + " {")
	g.up()

	for _, fn := range s.Functions() {
		g.renderSyncProcessFunction(fn, handlerTraitName)
	}

	g.down()
	g.line("}")
	g.wl("")

	// processor impl
	g.line("impl <H: " + handlerTraitName + "> TProcessor for " + serviceProcessorName + "<H> {")
	g.up()

	g.line("fn process(&self, i_prot: &mut dyn TInputProtocol, o_prot: &mut dyn TOutputProtocol) -> thrift::Result<()> {")
	g.up()

	g.line("let message_ident = i_prot.read_message_begin()?;")

	g.line("let res = match &*message_ident.name {") // [sigh] explicit deref coercion
	g.up()
	g.renderProcessMatchStatements(s)
	g.line("method => {")
	g.up()
	g.renderThriftError("Application", "ApplicationError", "ApplicationErrorKind::UnknownMethod", `format!("unknown method {}", method)`)
	g.down()
	g.line("},")

	g.down()
	g.line("};")
	g.line("thrift::server::handle_process_result(&message_ident, res, o_prot)")

	g.down()
	g.line("}")

	g.down()
	g.line("}")
	g.wl("")
}

// renderSyncProcessDelegationFunctions is
// t_rs_generator::render_sync_process_delegation_functions.
func (g *generator) renderSyncProcessDelegationFunctions(s *sema.Service) {
	actualProcessor := g.rustNamespaceService(s) + rustSyncProcessorImplName(s)

	for _, fn := range s.Functions() {
		functionName := "process_" + rustSnakeCase(fn.Name())
		g.line("fn " + functionName + "(&self, " +
			"incoming_sequence_number: i32, " +
			"i_prot: &mut dyn TInputProtocol, " +
			"o_prot: &mut dyn TOutputProtocol) " +
			"-> thrift::Result<()> {")
		g.up()

		g.line(actualProcessor + "::" + functionName + "(" +
			"&self.handler, " +
			"incoming_sequence_number, " +
			"i_prot, " +
			"o_prot" +
			")")

		g.down()
		g.line("}")
	}

	if extends := s.Extends(); extends != nil {
		g.renderSyncProcessDelegationFunctions(extends)
	}
}

// renderProcessMatchStatements is
// t_rs_generator::render_process_match_statements.
func (g *generator) renderProcessMatchStatements(s *sema.Service) {
	for _, fn := range s.Functions() {
		// note: use *original* name
		g.line("\"" + fn.Name() + "\"" + " => {")
		g.up()
		g.line("self.process_" + rustSnakeCase(fn.Name()) + "(message_ident.sequence_number, i_prot, o_prot)")
		g.down()
		g.line("},")
	}

	if extends := s.Extends(); extends != nil {
		g.renderProcessMatchStatements(extends)
	}
}

// renderSyncProcessFunction is t_rs_generator::render_sync_process_function.
func (g *generator) renderSyncProcessFunction(fn *sema.Function, handlerType string) {
	sequenceNumberParam := "incoming_sequence_number"
	outputProtocolParam := "o_prot"

	if fn.IsOneway() {
		sequenceNumberParam = "_"
		outputProtocolParam = "_"
	}

	g.line("pub fn process_" + rustSnakeCase(fn.Name()) +
		"<H: " + handlerType + ">" +
		"(handler: &H, " + sequenceNumberParam + ": i32, " +
		"i_prot: &mut dyn TInputProtocol, " + outputProtocolParam +
		": &mut dyn TOutputProtocol) " +
		"-> thrift::Result<()> {")

	g.up()

	// *always* read arguments from the input protocol
	argsVar := "args"
	if !hasNonVoidArgs(fn) {
		argsVar = "_"
	}
	g.line("let " + argsVar + " = " + g.serviceCallArgsStructName(fn) + "::read_from_in_protocol(i_prot)?;")

	g.line("match handler." + serviceCallHandlerFunctionName(fn) + g.rustSyncServiceCallInvocation(fn, "args.") + " {") // start match
	g.up()

	// handler succeeded
	handlerReturnVariable := "handler_return"
	if fn.IsOneway() || fn.ReturnType().IsVoid() {
		handlerReturnVariable = "_"
	}
	g.line("Ok(" + handlerReturnVariable + ") => {")
	g.up()
	g.renderSyncHandlerSucceeded(fn)
	g.down()
	g.line("},")
	// handler failed
	g.line("Err(e) => {")
	g.up()
	g.renderSyncHandlerFailed(fn)
	g.down()
	g.line("},")

	g.down()
	g.line("}") // end match

	g.down()
	g.line("}") // end function
}

// renderSyncHandlerSucceeded is t_rs_generator::render_sync_handler_succeeded.
func (g *generator) renderSyncHandlerSucceeded(fn *sema.Function) {
	if fn.IsOneway() {
		g.line("Ok(())")
	} else {
		// note: use *original* name
		g.line("let message_ident = TMessageIdentifier::new(\"" + fn.Name() + "\", TMessageType::Reply, incoming_sequence_number);")
		g.line("o_prot.write_message_begin(&message_ident)?;")
		g.line("let ret = " + g.handlerSuccessfulReturnStruct(fn) + ";")
		g.line("ret.write_to_out_protocol(o_prot)?;")
		g.line("o_prot.write_message_end()?;")
		g.line("o_prot.flush()")
	}
}

// renderSyncHandlerFailed is t_rs_generator::render_sync_handler_failed.
func (g *generator) renderSyncHandlerFailed(fn *sema.Function) {
	errVar := "e"

	g.line("match " + errVar + " {")
	g.up()

	// if there are any user-defined exceptions for this service call handle them first
	if fn.Xceptions() != nil && len(fn.Xceptions().SortedMembers()) > 0 {
		userErrVar := "usr_err"
		g.line("thrift::Error::User(" + userErrVar + ") => {")
		g.up()
		g.renderSyncHandlerFailedUserExceptionBranch(fn)
		g.down()
		g.line("},")
	}

	// application error
	appErrVar := "app_err"
	g.line("thrift::Error::Application(" + appErrVar + ") => {")
	g.up()
	g.renderSyncHandlerFailedApplicationExceptionBranch(fn, appErrVar)
	g.down()
	g.line("},")

	// default case
	g.line("_ => {")
	g.up()
	g.renderSyncHandlerFailedDefaultExceptionBranch(fn)
	g.down()
	g.line("},")

	g.down()
	g.line("}")
}

// renderSyncHandlerFailedUserExceptionBranch is
// t_rs_generator::render_sync_handler_failed_user_exception_branch.
func (g *generator) renderSyncHandlerFailedUserExceptionBranch(fn *sema.Function) {
	if fn.Xceptions() == nil || len(fn.Xceptions().SortedMembers()) == 0 {
		throwf("cannot render user exception branches if no user exceptions defined")
	}

	txceptions := fn.Xceptions().SortedMembers()
	branchesRendered := 0

	// run through all user-defined exceptions
	for _, xceptionField := range txceptions {
		ifStatement := "if usr_err"
		if branchesRendered != 0 {
			ifStatement = "} else if usr_err"
		}
		exceptionType := g.toRustType(xceptionField.Type())
		g.line(ifStatement + ".downcast_ref::<" + exceptionType + ">().is_some() {")
		g.up()

		g.line("let err = usr_err.downcast::<" + exceptionType + ">().expect(\"downcast already checked\");")

		// render the members of the return struct
		var members strings.Builder

		hasResultVariable := !(fn.IsOneway() || fn.ReturnType().IsVoid())
		if hasResultVariable {
			members.WriteString(serviceResultVariable + ": None, ")
		}

		for _, member := range txceptions {
			memberName := rustFieldName(member)
			if member == xceptionField {
				members.WriteString(memberName + ": Some(*err), ")
			} else {
				members.WriteString(memberName + ": None, ")
			}
		}

		memberString := members.String()
		memberString = memberString[:len(memberString)-2] + " " // trim trailing comma

		// now write out the return struct
		g.line("let ret_err = " + g.serviceCallResultStructName(fn) + "{ " + memberString + "};")

		g.line("let message_ident = TMessageIdentifier::new(\"" + fn.Name() + "\", TMessageType::Reply, incoming_sequence_number);")
		g.line("o_prot.write_message_begin(&message_ident)?;")
		g.line("ret_err.write_to_out_protocol(o_prot)?;")
		g.line("o_prot.write_message_end()?;")
		g.line("o_prot.flush()")

		g.down()

		branchesRendered++
	}

	// the catch all, if somehow it was a user exception that we don't support
	g.line("} else {")
	g.up()

	// FIXME: same as default block below

	g.line("let ret_err = {")
	g.up()
	g.renderThriftErrorStruct("ApplicationError", "ApplicationErrorKind::Unknown", "usr_err.to_string()")
	g.down()
	g.line("};")
	g.renderSyncHandlerSendExceptionResponse(fn, "ret_err")

	g.down()
	g.line("}")
}

// renderSyncHandlerFailedApplicationExceptionBranch is
// t_rs_generator::render_sync_handler_failed_application_exception_branch.
func (g *generator) renderSyncHandlerFailedApplicationExceptionBranch(fn *sema.Function, appErrVar string) {
	if fn.IsOneway() {
		g.line("Err(thrift::Error::Application(" + appErrVar + "))")
	} else {
		g.renderSyncHandlerSendExceptionResponse(fn, appErrVar)
	}
}

// renderSyncHandlerFailedDefaultExceptionBranch is
// t_rs_generator::render_sync_handler_failed_default_exception_branch.
func (g *generator) renderSyncHandlerFailedDefaultExceptionBranch(fn *sema.Function) {
	g.line("let ret_err = {")
	g.up()
	g.renderThriftErrorStruct("ApplicationError", "ApplicationErrorKind::Unknown", "e.to_string()")
	g.down()
	g.line("};")
	if fn.IsOneway() {
		g.line("Err(thrift::Error::Application(ret_err))")
	} else {
		g.renderSyncHandlerSendExceptionResponse(fn, "ret_err")
	}
}

// renderSyncHandlerSendExceptionResponse is
// t_rs_generator::render_sync_handler_send_exception_response.
func (g *generator) renderSyncHandlerSendExceptionResponse(fn *sema.Function, errVar string) {
	g.line("let message_ident = TMessageIdentifier::new(\"" + fn.Name() + "\", TMessageType::Exception, incoming_sequence_number);")
	g.line("o_prot.write_message_begin(&message_ident)?;")
	g.line("thrift::Error::write_application_error_to_out_protocol(&" + errVar + ", o_prot)?;")
	g.line("o_prot.write_message_end()?;")
	g.line("o_prot.flush()")
}

// handlerSuccessfulReturnStruct is
// t_rs_generator::handler_successful_return_struct.
func (g *generator) handlerSuccessfulReturnStruct(fn *sema.Function) string {
	memberCount := 0
	var returnStruct strings.Builder

	returnStruct.WriteString(g.serviceCallResultStructName(fn) + " { ")

	// actual return
	if !fn.ReturnType().IsVoid() {
		returnStruct.WriteString("result_value: Some(handler_return)")
		memberCount++
	}

	// any user-defined exceptions
	if fn.Xceptions() != nil {
		for _, xceptionField := range fn.Xceptions().SortedMembers() {
			if memberCount > 0 {
				returnStruct.WriteString(", ")
			}
			returnStruct.WriteString(rustFieldName(xceptionField) + ": None")
			memberCount++
		}
	}

	returnStruct.WriteString(" }")

	return returnStruct.String()
}
