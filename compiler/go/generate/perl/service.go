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
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateService generates a thrift service.
func (g *Generator) generateService(tservice *sema.Service) {
	g.fService = strings.Builder{}

	g.fService.WriteString(autogenComment() + perlIncludes())

	done := false
	g.generateUseIncludes(&g.fService, &done, tservice, true)

	extendsS := tservice.Extends()
	if extendsS != nil {
		g.fService.WriteString("use " + perlNamespace(extendsS.Program()) + extendsS.Name() + ";\n")
	}

	g.fService.WriteString("\n")

	// Generate the three main parts of the service (well, two for now in
	// PERL).
	g.generateServiceHelpers(tservice)
	g.generateServiceInterface(tservice)
	g.generateServiceRest(tservice)
	g.generateServiceClient(tservice)
	g.generateServiceProcessor(tservice)

	// Close service file.
	g.fService.WriteString("1;\n")
	fServiceName := g.namespaceOutDir() + g.serviceName + ".pm"
	emit.WriteFile(fServiceName, g.fService.String())
}

// generateServiceHelpers generates helper functions for a service.
func (g *Generator) generateServiceHelpers(tservice *sema.Service) {
	g.fService.WriteString("# HELPER FUNCTIONS AND STRUCTURES\n\n")

	for _, f := range tservice.Functions() {
		ts := f.Arglist()
		name := ts.Name()
		ts.SetName(g.serviceName + "_" + name)
		g.generatePerlStructDefinition(&g.fService, ts, false)
		g.generatePerlFunctionHelpers(f)
		ts.SetName(name)
	}
}

// generatePerlFunctionHelpers generates a struct and helpers for a
// function.
func (g *Generator) generatePerlFunctionHelpers(tfunction *sema.Function) {
	result := sema.NewStruct(g.program)
	result.SetName(g.serviceName + "_" + tfunction.Name() + "_result")
	success := sema.NewField(tfunction.ReturnType(), "success", 0)
	if !tfunction.ReturnType().IsVoid() {
		result.Append(success)
	}

	xs := tfunction.Xceptions()
	for _, f := range xs.Members() {
		result.Append(f)
	}

	g.generatePerlStructDefinition(&g.fService, result, false)
}

// generateServiceInterface generates a service interface definition.
func (g *Generator) generateServiceInterface(tservice *sema.Service) {
	extendsIf := ""
	extendsS := tservice.Extends()
	if extendsS != nil {
		extendsIf = "use base qw(" + perlNamespace(extendsS.Program()) + extendsS.Name() + "If);"
	}

	g.fService.WriteString("package " + perlNamespace(g.program) + g.serviceName +
		"If;  ## no critic (RequireFilenameMatchesPackage)\n\n" +
		"use strict;\n" + extendsIf + "\n\n")

	g.indentUp()
	for _, f := range tservice.Functions() {
		g.fService.WriteString("sub " + functionSignature(f, "") + "\n" +
			"  die 'implement interface';\n}\n\n")
	}
	g.indentDown()
}

// generateServiceRest generates a REST interface.
func (g *Generator) generateServiceRest(tservice *sema.Service) {
	extends := ""
	extendsIf := ""
	extendsS := tservice.Extends()
	if extendsS != nil {
		extends = extendsS.Name()
		extendsIf = "use base qw(" + perlNamespace(extendsS.Program()) + extendsS.Name() + "Rest);"
	}
	g.fService.WriteString("package " + perlNamespace(g.program) + g.serviceName +
		"Rest;  ## no critic (RequireFilenameMatchesPackage)\n\n" +
		"use strict;\n" + extendsIf + "\n\n")

	if extends == "" {
		g.fService.WriteString("sub new {\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + "my ($classname, $impl) = @_;\n" + g.indent() +
			"my $self     ={ impl => $impl };\n\n" + g.indent() +
			"return bless($self,$classname);\n")
		g.indentDown()
		g.fService.WriteString("}\n\n")
	}

	for _, f := range tservice.Functions() {
		g.fService.WriteString("sub " + f.Name() + "{\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + "my ($self, $request) = @_;\n\n")

		for _, a := range f.Arglist().Members() {
			req := "$request->{'" + a.Name() + "'}"
			g.fService.WriteString(g.indent() + "my $" + a.Name() + " = (" + req + ") ? " + req + " : undef;\n")
		}
		g.fService.WriteString(g.indent() + "return $self->{impl}->" + f.Name() + "(" +
			argumentList(f.Arglist()) + ");\n")
		g.indentDown()
		g.fService.WriteString(g.indent() + "}\n\n")
	}
}

// generateServiceClient generates a service client definition.
func (g *Generator) generateServiceClient(tservice *sema.Service) {
	extends := ""
	extendsClient := ""
	extendsS := tservice.Extends()
	if extendsS != nil {
		extends = perlNamespace(extendsS.Program()) + extendsS.Name()
		extendsClient = "use base qw(" + extends + "Client);"
	}

	g.fService.WriteString("package " + perlNamespace(g.program) + g.serviceName +
		"Client;  ## no critic (RequireFilenameMatchesPackage)\n\n" +
		extendsClient + "\n" + "use base qw(" + perlNamespace(g.program) + g.serviceName + "If);\n")

	// Constructor function.
	g.fService.WriteString("sub new {\n")

	g.indentUp()

	g.fService.WriteString(g.indent() + "my ($classname, $input, $output) = @_;\n" + g.indent() +
		"my $self      = {};\n")

	if extends != "" {
		g.fService.WriteString(g.indent() + "$self = $classname->SUPER::new($input, $output);\n")
	} else {
		g.fService.WriteString(g.indent() + "$self->{input}  = $input;\n" + g.indent() +
			"$self->{output} = defined $output ? $output : $input;\n" + g.indent() +
			"$self->{seqid}  = 0;\n")
	}

	g.fService.WriteString(g.indent() + "return bless($self,$classname);\n")

	g.indentDown()

	g.fService.WriteString("}\n\n")

	// Generate client method implementations.
	for _, f := range tservice.Functions() {
		argStruct := f.Arglist()
		fields := argStruct.Members()
		funname := f.Name()

		// Open function.
		g.fService.WriteString("sub " + functionSignature(f, "") + "\n")

		g.indentUp()

		g.fService.WriteString(g.indent() + g.indent() + "$self->send_" + funname + "(")

		first := true
		for _, fld := range fields {
			if first {
				first = false
			} else {
				g.fService.WriteString(", ")
			}
			g.fService.WriteString("$" + fld.Name())
		}
		g.fService.WriteString(");\n")

		if !f.IsOneway() {
			g.fService.WriteString(g.indent())
			if !f.ReturnType().IsVoid() {
				g.fService.WriteString("return ")
			}
			g.fService.WriteString("$self->recv_" + funname + "();\n")
		}

		g.indentDown()

		g.fService.WriteString("}\n\n")

		g.fService.WriteString("sub send_" + functionSignature(f, "") + "\n")

		g.indentUp()

		argsname := perlNamespace(tservice.Program()) + g.serviceName + "_" + f.Name() + "_args"

		// Serialize the request header.
		messageType := "Thrift::TMessageType::CALL"
		if f.IsOneway() {
			messageType = "Thrift::TMessageType::ONEWAY"
		}
		g.fService.WriteString(g.indent() + "$self->{output}->writeMessageBegin('" + f.Name() +
			"', " + messageType + ", $self->{seqid});\n")

		g.fService.WriteString(g.indent() + "my $args = " + argsname + "->new();\n")

		for _, fld := range fields {
			g.fService.WriteString(g.indent() + "$args->{" + fld.Name() + "} = $" + fld.Name() + ";\n")
		}

		// Write to the stream.
		g.fService.WriteString(g.indent() + "$args->write($self->{output});\n" + g.indent() +
			"$self->{output}->writeMessageEnd();\n" + g.indent() +
			"$self->{output}->getTransport()->flush();\n")

		g.indentDown()

		g.fService.WriteString("}\n")

		if !f.IsOneway() {
			resultname := perlNamespace(tservice.Program()) + g.serviceName + "_" + f.Name() + "_result"
			// t_struct noargs(program_); t_function recv_function(...):
			// the receive-side function has an empty argument list, so
			// its function_signature is always just its bare name.
			recvSig := "recv_" + f.Name() + "{\n  my $self = shift;\n"

			// Open function.
			g.fService.WriteString("\n" + "sub " + recvSig + "\n")

			g.indentUp()

			g.fService.WriteString(g.indent() + "my $rseqid = 0;\n" + g.indent() + "my $fname;\n" +
				g.indent() + "my $mtype = 0;\n\n")

			g.fService.WriteString(g.indent() + "$self->{input}->readMessageBegin(\\$fname, \\$mtype, \\$rseqid);\n" +
				g.indent() + "if ($mtype == Thrift::TMessageType::EXCEPTION) {\n" +
				g.indent() + "  my $x = Thrift::TApplicationException->new();\n" +
				g.indent() + "  $x->read($self->{input});\n" +
				g.indent() + "  $self->{input}->readMessageEnd();\n" +
				g.indent() + "  die $x;\n" +
				g.indent() + "}\n")

			g.fService.WriteString(g.indent() + "my $result = " + resultname + "->new();\n" +
				g.indent() + "$result->read($self->{input});\n")

			g.fService.WriteString(g.indent() + "$self->{input}->readMessageEnd();\n\n")

			// Careful, only return result if not a void function.
			if !f.ReturnType().IsVoid() {
				g.fService.WriteString(g.indent() + "if (defined $result->{success} ) {\n" +
					g.indent() + "  return $result->{success};\n" + g.indent() + "}\n")
			}

			for _, x := range f.Xceptions().Members() {
				g.fService.WriteString(g.indent() + "if (defined $result->{" + x.Name() + "}) {\n" +
					g.indent() + "  die $result->{" + x.Name() + "};\n" + g.indent() + "}\n")
			}

			// Careful, only return _result if not a void function.
			if f.ReturnType().IsVoid() {
				g.fService.WriteString(g.indent() + "return;\n")
			} else {
				g.fService.WriteString(g.indent() + "die \"" + f.Name() + " failed: unknown result\";\n")
			}

			// Close function.
			g.indentDown()
			g.fService.WriteString("}\n")
		}
	}
}

// generateServiceProcessor generates a service server definition.
//
// The leading indentUp below is never matched by a corresponding
// indentDown in this function (see t_perl_generator.cc: indent_up() is
// called before the header is written, but the only indent_down() calls
// pair with the indent_up() calls inside the "sub new" and "sub process"
// bodies). The extra indent level therefore leaks into whatever this
// generator writes next -- reproduced here rather than fixed, since byte
// parity with the C++ output requires it.
func (g *Generator) generateServiceProcessor(tservice *sema.Service) {
	functions := tservice.Functions()

	extends := ""
	extendsProcessor := ""
	extendsS := tservice.Extends()
	if extendsS != nil {
		extends = perlNamespace(extendsS.Program()) + extendsS.Name()
		extendsProcessor = "use base qw(" + extends + "Processor);"
	}

	g.indentUp()

	// Generate the header portion.
	g.fService.WriteString("package " + perlNamespace(g.program) + g.serviceName +
		"Processor;  ## no critic (RequireFilenameMatchesPackage)\n\n" +
		"use strict;\n" + extendsProcessor + "\n\n")

	if extends == "" {
		g.fService.WriteString("sub new {\n")

		g.indentUp()

		g.fService.WriteString(g.indent() + "my ($classname, $handler) = @_;\n" + g.indent() +
			"my $self      = {};\n")

		g.fService.WriteString(g.indent() + "$self->{handler} = $handler;\n")

		g.fService.WriteString(g.indent() + "return bless ($self, $classname);\n")

		g.indentDown()

		g.fService.WriteString("}\n\n")
	}

	// Generate the server implementation.
	g.fService.WriteString("sub process {\n")
	g.indentUp()

	g.fService.WriteString(g.indent() + "my ($self, $input, $output) = @_;\n")

	g.fService.WriteString(g.indent() + "my $rseqid = 0;\n" + g.indent() + "my $fname  = undef;\n" +
		g.indent() + "my $mtype  = 0;\n\n")

	g.fService.WriteString(g.indent() + "$input->readMessageBegin(\\$fname, \\$mtype, \\$rseqid);\n")

	// HOT: check for method implementation.
	g.fService.WriteString(g.indent() + "my $methodname = 'process_'.$fname;\n" + g.indent() +
		"if (!$self->can($methodname)) {\n")
	g.indentUp()

	g.fService.WriteString(g.indent() + "$input->skip(Thrift::TType::STRUCT);\n" + g.indent() +
		"$input->readMessageEnd();\n" + g.indent() +
		"my $x = Thrift::TApplicationException->new('Function '.$fname.' not implemented.', " +
		"Thrift::TApplicationException::UNKNOWN_METHOD);\n" + g.indent() +
		"$output->writeMessageBegin($fname, Thrift::TMessageType::EXCEPTION, $rseqid);\n" +
		g.indent() + "$x->write($output);\n" + g.indent() + "$output->writeMessageEnd();\n" +
		g.indent() + "$output->getTransport()->flush();\n" + g.indent() + "return;\n")

	g.indentDown()
	g.fService.WriteString(g.indent() + "}\n" + g.indent() +
		"$self->$methodname($rseqid, $input, $output);\n" + g.indent() + "return 1;\n")

	g.indentDown()

	g.fService.WriteString("}\n\n")

	// Generate the process subfunctions.
	for _, f := range functions {
		g.generateProcessFunction(tservice, f)
	}
}

// generateProcessFunction generates a process function definition.
func (g *Generator) generateProcessFunction(tservice *sema.Service, tfunction *sema.Function) {
	// Open function.
	g.fService.WriteString("sub process_" + tfunction.Name() + " {\n")

	g.indentUp()

	g.fService.WriteString(g.indent() + "my ($self, $seqid, $input, $output) = @_;\n")

	argsname := perlNamespace(tservice.Program()) + g.serviceName + "_" + tfunction.Name() + "_args"
	resultname := perlNamespace(tservice.Program()) + g.serviceName + "_" + tfunction.Name() + "_result"

	g.fService.WriteString(g.indent() + "my $args = " + argsname + "->new();\n" + g.indent() +
		"$args->read($input);\n")

	g.fService.WriteString(g.indent() + "$input->readMessageEnd();\n")

	xceptions := tfunction.Xceptions().Members()

	// Declare result for non oneway function.
	if !tfunction.IsOneway() {
		g.fService.WriteString(g.indent() + "my $result = " + resultname + "->new();\n")
	}

	// Try block for a function with exceptions.
	if len(xceptions) > 0 {
		g.fService.WriteString(g.indent() + "eval {\n")
		g.indentUp()
	}

	// Generate the function call.
	fields := tfunction.Arglist().Members()

	g.fService.WriteString(g.indent())
	if !tfunction.IsOneway() && !tfunction.ReturnType().IsVoid() {
		g.fService.WriteString("$result->{success} = ")
	}
	g.fService.WriteString("$self->{handler}->" + tfunction.Name() + "(")
	first := true
	for _, f := range fields {
		if first {
			first = false
		} else {
			g.fService.WriteString(", ")
		}
		g.fService.WriteString("$args->" + f.Name())
	}
	g.fService.WriteString(");\n")

	if !tfunction.IsOneway() && len(xceptions) > 0 {
		g.indentDown()
		for _, x := range xceptions {
			g.fService.WriteString(g.indent() + "}; if( UNIVERSAL::isa($@,'" +
				perlNamespace(x.Type().Program()) + x.Type().Name() + "') ){ \n")

			g.indentUp()
			g.fService.WriteString(g.indent() + "$result->{" + x.Name() + "} = $@;\n")
			g.fService.WriteString(g.indent() + "$@ = undef;\n")
			g.indentDown()
			g.fService.WriteString(g.indent())
		}
		g.fService.WriteString("}\n")

		// Catch-all for unexpected exceptions (THRIFT-3191).
		g.fService.WriteString(g.indent() + "if ($@) {\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + `$@ =~ s/^\s+|\s+$//g;` + "\n" +
			g.indent() + `my $err = Thrift::TApplicationException->new("Unexpected Exception: " . $@, Thrift::TApplicationException::INTERNAL_ERROR);` + "\n" +
			g.indent() + "$output->writeMessageBegin('" + tfunction.Name() + "', Thrift::TMessageType::EXCEPTION, $seqid);\n" +
			g.indent() + "$err->write($output);\n" +
			g.indent() + "$output->writeMessageEnd();\n" +
			g.indent() + "$output->getTransport()->flush();\n" +
			g.indent() + "$@ = undef;\n" +
			g.indent() + "return;\n")
		g.indentDown()
		g.fService.WriteString(g.indent() + "}\n")
	}

	// Shortcut out here for oneway functions.
	if tfunction.IsOneway() {
		g.fService.WriteString(g.indent() + "return;\n")
		g.indentDown()
		g.fService.WriteString("}\n")
		return
	}

	// Serialize the reply.
	g.fService.WriteString(g.indent() + "$output->writeMessageBegin('" + tfunction.Name() +
		"', Thrift::TMessageType::REPLY, $seqid);\n" +
		g.indent() + "$result->write($output);\n" +
		g.indent() + "$output->writeMessageEnd();\n" +
		g.indent() + "$output->getTransport()->flush();\n")

	// Close function.
	g.indentDown()
	g.fService.WriteString("}\n\n")
}
