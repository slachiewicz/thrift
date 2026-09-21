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

package cglib

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateService generates C code to represent Thrift services. Creates
// a new GObject which can be used to access the service.
func (g *Generator) generateService(tservice *sema.Service) {
	svcnameU := initialCapsToUnderscores(tservice.Name())
	svcnameUC := g.nspaceUC + toUpperCase(svcnameU)
	filename := g.nspaceLC + toLowerCase(svcnameU)

	g.fHeader.Reset()
	g.fService.Reset()

	programNameU := initialCapsToUnderscores(g.program.Name())
	programNameLC := toLowerCase(programNameU)

	// add header file boilerplate
	g.fHeader.WriteString(autogenComment())

	// add an inclusion guard
	g.fHeader.WriteString("#ifndef " + svcnameUC + "_H\n#define " + svcnameUC + "_H\n\n")

	// add standard includes
	g.fHeader.WriteString("#include <thrift/c_glib/processor/thrift_dispatch_processor.h>\n\n")
	g.fHeader.WriteString("#include \"" + g.nspaceLC + programNameLC + "_types.h\"\n")

	// if we are inheriting from another service, include its header
	if extendsService := tservice.Extends(); extendsService != nil {
		g.fHeader.WriteString("#include \"" + g.nspaceLC +
			toLowerCase(initialCapsToUnderscores(extendsService.Name())) + ".h\"\n")
	}
	g.fHeader.WriteString("\n")

	// add the boilerplate header
	g.fService.WriteString(autogenComment())

	// include the headers
	g.fService.WriteString("#include <string.h>\n#include <thrift/c_glib/thrift.h>\n" +
		"#include <thrift/c_glib/thrift_application_exception.h>\n#include \"" + filename + ".h\"\n\n")

	// generate the service-helper classes
	g.generateServiceHelpers(tservice)

	// generate the client objects
	g.generateServiceClient(tservice)

	// generate the server objects
	g.generateServiceServer(tservice)

	// end the header inclusion guard
	g.fHeader.WriteString("#endif /* " + svcnameUC + "_H */\n")

	// close the files
	emit.WriteFile(g.outDir()+filename+".c", g.fService.String())
	emit.WriteFile(g.outDir()+filename+".h", g.fHeader.String())
}

// generateServiceHelpers generates helper classes for a service,
// consisting of a ThriftStruct subclass for the arguments to and the
// result from each method.
func (g *Generator) generateServiceHelpers(tservice *sema.Service) {
	for _, f := range tservice.Functions() {
		functionName := f.Name()
		argList := f.Arglist()
		argListNameOrig := argList.Name()

		// Generate the arguments class
		argList.SetName(tservice.Name() + underscoresToInitialCaps(functionName) + "Args")
		g.generateStruct(argList)

		argList.SetName(argListNameOrig)

		// Generate the result class
		if !f.IsOneway() {
			result := sema.NewStruct(g.program)
			result.SetName(tservice.Name() + underscoresToInitialCaps(functionName) + "Result")
			success := sema.NewField(f.ReturnType(), "success", 0)
			success.SetReq(sema.Optional)
			if !f.ReturnType().IsVoid() {
				result.Append(success)
			}

			for _, field := range f.Xceptions().Members() {
				field.SetReq(sema.Optional)
				result.Append(field)
			}

			g.generateStruct(result)
		}
	}
}

// generateServiceServer generates C code that represents a Thrift service
// server.
func (g *Generator) generateServiceServer(tservice *sema.Service) {
	// Generate the service's handler class
	g.generateServiceHandler(tservice)

	// Generate the service's processor class
	g.generateServiceProcessor(tservice)
}

// ifaceParams renders the "(...)" argument list shared by the interface
// vtable field, the interface function prototype and its implementation:
// "(nspace service If *iface[, return][, args][, xceptions], GError **error)".
func (g *Generator) ifaceParams(f *sema.Function) string {
	ttype := f.ReturnType()
	arglist := f.Arglist()
	xlist := f.Xceptions()
	hasReturn := !ttype.IsVoid()
	noArgs := len(arglist.Members()) == 0
	noXceptions := len(xlist.Members()) == 0

	s := "(" + g.nspace + g.serviceName + "If *iface"
	if hasReturn {
		s += ", " + g.typeName(ttype, false, false) + "* _return"
	}
	if !noArgs {
		s += ", " + g.argumentList(arglist)
	}
	if !noXceptions {
		s += ", " + g.xceptionList(xlist)
	}
	s += ", GError **error)"
	return s
}

// generateServiceClient generates C code that represents a Thrift
// service client.
func (g *Generator) generateServiceClient(tservice *sema.Service) {
	serviceNameLC := toLowerCase(initialCapsToUnderscores(g.serviceName))
	serviceNameUC := toUpperCase(serviceNameLC)

	parentClassName := "GObject"
	parentTypeName := "G_TYPE_OBJECT"

	extendsService := tservice.Extends()
	if extendsService != nil {
		parentServiceName := extendsService.Name()
		parentServiceNameLC := toLowerCase(initialCapsToUnderscores(parentServiceName))
		parentServiceNameUC := toUpperCase(parentServiceNameLC)

		parentClassName = g.nspace + parentServiceName + "Client"
		parentTypeName = g.nspaceUC + "TYPE_" + parentServiceNameUC + "_CLIENT"
	}

	baseService := tservice
	for baseService.Extends() != nil {
		baseService = baseService.Extends()
	}

	baseServiceNameLC := toLowerCase(initialCapsToUnderscores(baseService.Name()))
	baseServiceNameUC := toUpperCase(baseServiceNameLC)

	// Generate the client interface dummy object in the header.
	g.fHeader.WriteString("/* " + g.serviceName + " service interface */\ntypedef struct _" +
		g.nspace + g.serviceName + "If " + g.nspace + g.serviceName + "If;  /* dummy object */\n\n")

	// Generate the client interface object in the header.
	g.fHeader.WriteString("struct _" + g.nspace + g.serviceName + "IfInterface\n{\n" +
		"  GTypeInterface parent;\n\n")

	/* write out the functions for this interface */
	g.indentUp()
	functions := tservice.Functions()
	for _, f := range functions {
		funname := initialCapsToUnderscores(f.Name())
		params := g.ifaceParams(f)
		g.fHeader.WriteString(g.indent() + "gboolean (*" + funname + ") " + params + ";\n")
	}
	g.indentDown()

	g.fHeader.WriteString("};\ntypedef struct _" + g.nspace + g.serviceName + "IfInterface " +
		g.nspace + g.serviceName + "IfInterface;\n\n")

	// generate all the interface boilerplate
	g.fHeader.WriteString("GType " + g.nspaceLC + serviceNameLC + "_if_get_type (void);\n" +
		"#define " + g.nspaceUC + "TYPE_" + serviceNameUC + "_IF " +
		"(" + g.nspaceLC + serviceNameLC + "_if_get_type())\n#define " +
		g.nspaceUC + serviceNameUC + "_IF(obj) " +
		"(G_TYPE_CHECK_INSTANCE_CAST ((obj), " + g.nspaceUC + "TYPE_" +
		serviceNameUC + "_IF, " + g.nspace + g.serviceName + "If))\n" +
		"#define " + g.nspaceUC + "IS_" + serviceNameUC + "_IF(obj) " +
		"(G_TYPE_CHECK_INSTANCE_TYPE ((obj), " + g.nspaceUC + "TYPE_" +
		serviceNameUC + "_IF))\n#define " + g.nspaceUC +
		serviceNameUC + "_IF_GET_INTERFACE(inst) (G_TYPE_INSTANCE_GET_INTERFACE ((inst), " +
		g.nspaceUC + "TYPE_" + serviceNameUC + "_IF, " + g.nspace +
		g.serviceName + "IfInterface))\n\n")

	// write out all the interface function prototypes
	for _, f := range functions {
		funname := initialCapsToUnderscores(f.Name())
		params := g.ifaceParams(f)
		g.fHeader.WriteString("gboolean " + g.nspaceLC + serviceNameLC + "_if_" + funname + " " + params + ";\n")
	}
	g.fHeader.WriteString("\n")

	// Generate the client object instance definition in the header.
	g.fHeader.WriteString("/* " + g.serviceName + " service client */\nstruct _" + g.nspace +
		g.serviceName + "Client\n{\n  " + parentClassName + " parent;\n")
	if extendsService == nil {
		// Define "input_protocol" and "output_protocol" properties only
		// for base services; child service-client classes will inherit
		// these
		g.fHeader.WriteString("\n  ThriftProtocol *input_protocol;\n  ThriftProtocol *output_protocol;\n")
	}
	g.fHeader.WriteString("};\ntypedef struct _" + g.nspace + g.serviceName + "Client " +
		g.nspace + g.serviceName + "Client;\n\n")

	// Generate the class definition in the header.
	g.fHeader.WriteString("struct _" + g.nspace + g.serviceName + "ClientClass\n{\n  " +
		parentClassName + "Class parent;\n};\ntypedef struct _" + g.nspace + g.serviceName +
		"ClientClass " + g.nspace + g.serviceName + "ClientClass;\n\n")

	// Create all the GObject boilerplate
	g.fHeader.WriteString("GType " + g.nspaceLC + serviceNameLC + "_client_get_type (void);\n" +
		"#define " + g.nspaceUC + "TYPE_" + serviceNameUC + "_CLIENT " +
		"(" + g.nspaceLC + serviceNameLC + "_client_get_type())\n" +
		"#define " + g.nspaceUC + serviceNameUC + "_CLIENT(obj) " +
		"(G_TYPE_CHECK_INSTANCE_CAST ((obj), " + g.nspaceUC + "TYPE_" +
		serviceNameUC + "_CLIENT, " + g.nspace + g.serviceName + "Client))\n" +
		"#define " + g.nspaceUC + serviceNameUC + "_CLIENT_CLASS(c) " +
		"(G_TYPE_CHECK_CLASS_CAST ((c), " + g.nspaceUC + "TYPE_" + serviceNameUC +
		"_CLIENT, " + g.nspace + g.serviceName + "ClientClass))\n#define " +
		g.nspaceUC + serviceNameUC + "_IS_CLIENT(obj) " +
		"(G_TYPE_CHECK_INSTANCE_TYPE ((obj), " + g.nspaceUC + "TYPE_" +
		serviceNameUC + "_CLIENT))\n#define " + g.nspaceUC + serviceNameUC +
		"_IS_CLIENT_CLASS(c) " +
		"(G_TYPE_CHECK_CLASS_TYPE ((c), " + g.nspaceUC + "TYPE_" + serviceNameUC +
		"_CLIENT))\n#define " + g.nspaceUC + serviceNameUC +
		"_CLIENT_GET_CLASS(obj) " +
		"(G_TYPE_INSTANCE_GET_CLASS ((obj), " + g.nspaceUC + "TYPE_" +
		serviceNameUC + "_CLIENT, " + g.nspace + g.serviceName + "ClientClass))" +
		"\n\n")

	/* write out the function prototypes */
	for _, f := range functions {
		funname := toLowerCase(initialCapsToUnderscores(f.Name()))

		serviceFunction := newFuncSig(f.ReturnType(), serviceNameLC+"_client_"+funname, f.Arglist(), f.Xceptions())
		g.fHeader.WriteString(g.indent() + g.functionSignature(serviceFunction) + ";\n")

		sendFunction := newFuncSig3(sema.GlobalVoid, serviceNameLC+"_client_send_"+funname, f.Arglist())
		g.fHeader.WriteString(g.indent() + g.functionSignature(sendFunction) + ";\n")

		// implement recv if not a oneway service
		if !f.IsOneway() {
			noargs := sema.NewStruct(g.program)
			recvFunction := newFuncSig(f.ReturnType(), serviceNameLC+"_client_recv_"+funname, noargs, f.Xceptions())
			g.fHeader.WriteString(g.indent() + g.functionSignature(recvFunction) + ";\n")
		}
	}

	/* write out the get/set function prototypes */
	g.fHeader.WriteString("void " + serviceNameLC + "_client_set_property (GObject *object, guint " +
		"property_id, const GValue *value, GParamSpec *pspec);\n")
	g.fHeader.WriteString("void " + serviceNameLC + "_client_get_property (GObject *object, guint " +
		"property_id, GValue *value, GParamSpec *pspec);\n")

	g.fHeader.WriteString("\n")
	// end of header code

	// Generate interface method implementations
	for _, f := range functions {
		funname := initialCapsToUnderscores(f.Name())
		ttype := f.ReturnType()
		arglist := f.Arglist()
		xlist := f.Xceptions()
		hasReturn := !ttype.IsVoid()
		params := g.ifaceParams(f)

		paramsWithoutType := "iface, "
		if hasReturn {
			paramsWithoutType += "_return, "
		}

		for _, field := range arglist.Members() {
			paramsWithoutType += field.Name() + ", "
		}
		for _, x := range xlist.Members() {
			paramsWithoutType += x.Name() + ", "
		}

		g.fService.WriteString("gboolean\n" + g.nspaceLC + serviceNameLC + "_if_" + funname +
			" " + params + "\n{\n  return " + g.nspaceUC + serviceNameUC + "_IF_GET_INTERFACE (iface)->" +
			funname + " (" + paramsWithoutType + "error);\n}\n\n")
	}

	// Generate interface boilerplate
	g.fService.WriteString("GType\n" + g.nspaceLC + serviceNameLC + "_if_get_type (void)\n{\n" +
		"  static GType type = 0;\n  if (type == 0)\n  {\n" +
		"    static const GTypeInfo type_info =\n    {\n" +
		"      sizeof (" + g.nspace + g.serviceName + "IfInterface),\n" +
		"      NULL,  /* base_init */\n" +
		"      NULL,  /* base_finalize */\n" +
		"      NULL,  /* class_init */\n" +
		"      NULL,  /* class_finalize */\n" +
		"      NULL,  /* class_data */\n" +
		"      0,     /* instance_size */\n" +
		"      0,     /* n_preallocs */\n" +
		"      NULL,  /* instance_init */\n" +
		"      NULL   /* value_table */\n    };\n" +
		"    type = g_type_register_static (G_TYPE_INTERFACE,\n" +
		"                                   \"" + g.nspace + g.serviceName + "If\",\n" +
		"                                   &type_info, 0);\n  }\n" +
		"  return type;\n}\n\n")

	// Generate client boilerplate
	g.fService.WriteString("static void \n" + g.nspaceLC + serviceNameLC +
		"_if_interface_init (" + g.nspace + g.serviceName + "IfInterface *iface);\n\n" +
		"G_DEFINE_TYPE_WITH_CODE (" + g.nspace + g.serviceName + "Client, " + g.nspaceLC + serviceNameLC + "_client,\n" +
		"                         " + parentTypeName + ", \n" +
		"                         G_IMPLEMENT_INTERFACE (" + g.nspaceUC + "TYPE_" + serviceNameUC + "_IF,\n" +
		"                                                " + g.nspaceLC + serviceNameLC + "_if_interface_init))\n\n")

	// Generate property-related code only for base services---child
	// service-client classes have only properties inherited from their
	// parent class
	if extendsService == nil {
		// Generate client properties
		g.fService.WriteString("enum _" + g.nspace + g.serviceName + "ClientProperties\n{\n" +
			"  PROP_0,\n  PROP_" + g.nspaceUC + serviceNameUC + "_CLIENT_INPUT_PROTOCOL,\n  PROP_" +
			g.nspaceUC + serviceNameUC + "_CLIENT_OUTPUT_PROTOCOL\n};\n\n")

		// generate property setter
		g.fService.WriteString("void\n" + g.nspaceLC + serviceNameLC + "_client_set_property (" +
			"GObject *object, guint property_id, const GValue *value, GParamSpec *pspec)\n{\n  " +
			g.nspace + g.serviceName + "Client *client = " + g.nspaceUC + serviceNameUC +
			"_CLIENT (object);\n\n  THRIFT_UNUSED_VAR (pspec);\n\n  switch (property_id)\n  {\n" +
			"    case PROP_" + g.nspaceUC + serviceNameUC + "_CLIENT_INPUT_PROTOCOL:\n" +
			"      client->input_protocol = g_value_get_object (value);\n      break;\n" +
			"    case PROP_" + g.nspaceUC + serviceNameUC + "_CLIENT_OUTPUT_PROTOCOL:\n" +
			"      client->output_protocol = g_value_get_object (value);\n      break;\n  }\n}\n\n")

		// generate property getter
		g.fService.WriteString("void\n" + g.nspaceLC + serviceNameLC + "_client_get_property (" +
			"GObject *object, guint property_id, GValue *value, GParamSpec *pspec)\n{\n  " +
			g.nspace + g.serviceName + "Client *client = " + g.nspaceUC + serviceNameUC +
			"_CLIENT (object);\n\n  THRIFT_UNUSED_VAR (pspec);\n\n  switch (property_id)\n  {\n" +
			"    case PROP_" + g.nspaceUC + serviceNameUC + "_CLIENT_INPUT_PROTOCOL:\n" +
			"      g_value_set_object (value, client->input_protocol);\n      break;\n" +
			"    case PROP_" + g.nspaceUC + serviceNameUC + "_CLIENT_OUTPUT_PROTOCOL:\n" +
			"      g_value_set_object (value, client->output_protocol);\n      break;\n  }\n}\n\n")
	}

	// Generate client method implementations
	for _, f := range functions {
		name := f.Name()
		funname := initialCapsToUnderscores(name)

		argStruct := f.Arglist()

		// Function for sending
		sendFunction := newFuncSig3(sema.GlobalVoid, serviceNameLC+"_client_send_"+funname, f.Arglist())

		// Open the send function
		g.fService.WriteString(g.indent() + g.functionSignature(sendFunction) + "\n")
		g.scopeUp(&g.fService)

		reqType := "T_CALL"
		if f.IsOneway() {
			reqType = "T_ONEWAY"
		}

		// Serialize the request
		g.fService.WriteString(g.indent() + "gint32 cseqid = 0;\n" + g.indent() +
			"ThriftProtocol * protocol = " + g.nspaceUC + baseServiceNameUC +
			"_CLIENT (iface)->output_protocol;\n\n" + g.indent() +
			"if (thrift_protocol_write_message_begin (protocol, \"" + name + "\", " +
			reqType + ", cseqid, error) < 0)\n" + g.indent() + "  return FALSE;\n\n")

		g.generateStructWriter(&g.fService, argStruct, "", "", false)

		g.fService.WriteString(g.indent() + "if (thrift_protocol_write_message_end (protocol, error) < 0)\n" +
			g.indent() + "  return FALSE;\n" + g.indent() +
			"if (!thrift_transport_flush (protocol->transport, error))\n" + g.indent() +
			"  return FALSE;\n" + g.indent() +
			"if (!thrift_transport_write_end (protocol->transport, error))\n" +
			g.indent() + "  return FALSE;\n\n" + g.indent() + "return TRUE;\n")

		g.scopeDown(&g.fService)
		g.fService.WriteString("\n")

		// Generate recv function only if not an async function
		if !f.IsOneway() {
			noargs := sema.NewStruct(g.program)
			recvFunction := newFuncSig(f.ReturnType(), serviceNameLC+"_client_recv_"+funname, noargs, f.Xceptions())
			// Open function
			g.fService.WriteString(g.indent() + g.functionSignature(recvFunction) + "\n")
			g.scopeUp(&g.fService)

			g.fService.WriteString(g.indent() + "gint32 rseqid;\n" +
				g.indent() + "gchar * fname = NULL;\n" +
				g.indent() + "ThriftMessageType mtype;\n" +
				g.indent() + "ThriftProtocol * protocol = " +
				g.nspaceUC + baseServiceNameUC + "_CLIENT (iface)->input_protocol;\n" +
				g.indent() + "ThriftApplicationException *xception;\n\n" +
				g.indent() + "if (thrift_protocol_read_message_begin (protocol, &fname, &mtype, &rseqid, error) < 0) {\n")
			g.indentUp()
			g.fService.WriteString(g.indent() + "if (fname) g_free (fname);\n" + g.indent() + "return FALSE;\n")
			g.indentDown()
			g.fService.WriteString(g.indent() + "}\n\n" + g.indent() + "if (mtype == T_EXCEPTION) {\n")
			g.indentUp()
			g.fService.WriteString(g.indent() + "if (fname) g_free (fname);\n" +
				g.indent() + "xception = g_object_new (THRIFT_TYPE_APPLICATION_EXCEPTION, NULL);\n" +
				g.indent() + "thrift_struct_read (THRIFT_STRUCT (xception), protocol, NULL);\n" +
				g.indent() + "thrift_protocol_read_message_end (protocol, NULL);\n" +
				g.indent() + "thrift_transport_read_end (protocol->transport, NULL);\n" +
				g.indent() + "g_set_error (error, THRIFT_APPLICATION_EXCEPTION_ERROR,xception->type, \"application error: %s\", xception->message);\n" +
				g.indent() + "g_object_unref (xception);\n" +
				g.indent() + "return FALSE;\n")
			g.indentDown()
			g.fService.WriteString(g.indent() + "} else if (mtype != T_REPLY) {\n")
			g.indentUp()
			g.fService.WriteString(g.indent() + "if (fname) g_free (fname);\n" +
				g.indent() + "thrift_protocol_skip (protocol, T_STRUCT, NULL);\n" +
				g.indent() + "thrift_protocol_read_message_end (protocol, NULL);\n" +
				g.indent() + "thrift_transport_read_end (protocol->transport, NULL);\n" +
				g.indent() + "g_set_error (error, THRIFT_APPLICATION_EXCEPTION_ERROR, THRIFT_APPLICATION_EXCEPTION_ERROR_INVALID_MESSAGE_TYPE, \"invalid message type %d, expected T_REPLY\", mtype);\n" +
				g.indent() + "return FALSE;\n")
			g.indentDown()
			g.fService.WriteString(g.indent() + "} else if (strncmp (fname, \"" + name + "\", " +
				strconv.Itoa(len(name)) + ") != 0) {\n")
			g.indentUp()
			g.fService.WriteString(g.indent() + "thrift_protocol_skip (protocol, T_STRUCT, NULL);\n" +
				g.indent() + "thrift_protocol_read_message_end (protocol,error);\n" +
				g.indent() + "thrift_transport_read_end (protocol->transport, error);\n" +
				g.indent() + "g_set_error (error, THRIFT_APPLICATION_EXCEPTION_ERROR, THRIFT_APPLICATION_EXCEPTION_ERROR_WRONG_METHOD_NAME, \"wrong method name %s, expected " + name + "\", fname);\n" +
				g.indent() + "if (fname) g_free (fname);\n" +
				g.indent() + "return FALSE;\n")
			g.indentDown()
			g.fService.WriteString(g.indent() + "}\n" + g.indent() + "if (fname) g_free (fname);\n\n")

			xceptions := f.Xceptions().Members()

			result := sema.NewStruct(g.program)
			result.SetName(tservice.Name() + "_" + f.Name() + "_result")
			success := sema.NewField(f.ReturnType(), "*_return", 0)
			if !f.ReturnType().IsVoid() {
				result.Append(success)
			}
			for _, x := range xceptions {
				xception := sema.NewField(x.Type(), "*"+x.Name(), x.Key())
				result.Append(xception)
			}
			g.generateStructReader(&g.fService, result, "", "", false)

			g.fService.WriteString(g.indent() + "if (thrift_protocol_read_message_end (protocol, error) < 0)\n" +
				g.indent() + "  return FALSE;\n\n" + g.indent() +
				"if (!thrift_transport_read_end (protocol->transport, error))\n" +
				g.indent() + "  return FALSE;\n\n")

			// copy over any throw exceptions and return failure
			for _, x := range xceptions {
				xtNameUC := toUpperCase(initialCapsToUnderscores(x.Type().Name()))
				g.fService.WriteString(g.indent() + "if (*" + x.Name() + " != NULL)\n" +
					g.indent() + "{\n" + g.indent() + "    g_set_error (error, " + g.nspaceUC +
					xtNameUC + "_ERROR, " + g.nspaceUC + xtNameUC + "_ERROR_CODE, \"" +
					x.Type().Name() + "\");\n" + g.indent() + "    return FALSE;\n" + g.indent() + "}\n")
			}
			// Close function
			g.fService.WriteString(g.indent() + "return TRUE;\n")
			g.scopeDown(&g.fService)
			g.fService.WriteString("\n")
		}

		// Open function
		serviceFunction := newFuncSig(f.ReturnType(), serviceNameLC+"_client_"+funname, f.Arglist(), f.Xceptions())
		g.fService.WriteString(g.indent() + g.functionSignature(serviceFunction) + "\n")
		g.scopeUp(&g.fService)

		// wrap each function
		g.fService.WriteString(g.indent() + "if (!" + g.nspaceLC + serviceNameLC + "_client_send_" + funname + " (iface")

		// Declare the function arguments
		for _, fld := range argStruct.Members() {
			g.fService.WriteString(", " + fld.Name())
		}
		g.fService.WriteString(", error))\n" + g.indent() + "  return FALSE;\n")

		// if not oneway, implement recv
		if !f.IsOneway() {
			ret := ""
			if !f.ReturnType().IsVoid() {
				ret = "_return, "
			}

			for _, x := range f.Xceptions().Members() {
				ret += x.Name() + ", "
			}

			g.fService.WriteString(g.indent() + "if (!" + g.nspaceLC + serviceNameLC + "_client_recv_" +
				funname + " (iface, " + ret + "error))\n" + g.indent() + "  return FALSE;\n")
		}

		// return TRUE which means all functions were called OK
		g.fService.WriteString(g.indent() + "return TRUE;\n")
		g.scopeDown(&g.fService)
		g.fService.WriteString("\n")
	}

	// create the interface initializer
	g.fService.WriteString("static void\n" + g.nspaceLC + serviceNameLC + "_if_interface_init (" +
		g.nspace + g.serviceName + "IfInterface *iface)\n")
	g.scopeUp(&g.fService)
	if len(functions) > 0 {
		for _, f := range functions {
			funname := initialCapsToUnderscores(f.Name())
			g.fService.WriteString(g.indent() + "iface->" + funname + " = " + g.nspaceLC +
				serviceNameLC + "_client_" + funname + ";\n")
		}
	} else {
		g.fService.WriteString(g.indent() + "THRIFT_UNUSED_VAR (iface);\n")
	}
	g.scopeDown(&g.fService)
	g.fService.WriteString("\n")

	// create the client instance initializer
	g.fService.WriteString("static void\n" + g.nspaceLC + serviceNameLC + "_client_init (" +
		g.nspace + g.serviceName + "Client *client)\n")
	g.scopeUp(&g.fService)
	if extendsService == nil {
		g.fService.WriteString(g.indent() + "client->input_protocol = NULL;\n" +
			g.indent() + "client->output_protocol = NULL;\n")
	} else {
		g.fService.WriteString(g.indent() + "THRIFT_UNUSED_VAR (client);\n")
	}
	g.scopeDown(&g.fService)
	g.fService.WriteString("\n")

	// create the client class initializer
	g.fService.WriteString("static void\n" + g.nspaceLC + serviceNameLC + "_client_class_init (" +
		g.nspace + g.serviceName + "ClientClass *cls)\n{\n")
	if extendsService == nil {
		g.fService.WriteString("  GObjectClass *gobject_class = G_OBJECT_CLASS (cls);\n" +
			"  GParamSpec *param_spec;\n\n" +
			"  gobject_class->set_property = " + g.nspaceLC + serviceNameLC + "_client_set_property;\n" +
			"  gobject_class->get_property = " + g.nspaceLC + serviceNameLC + "_client_get_property;\n\n" +
			"  param_spec = g_param_spec_object (\"input_protocol\",\n" +
			"                                    \"input protocol (construct)\",\n" +
			"                                    \"Set the client input protocol\",\n" +
			"                                    THRIFT_TYPE_PROTOCOL,\n" +
			"                                    G_PARAM_READWRITE);\n" +
			"  g_object_class_install_property (gobject_class,\n" +
			"                                   PROP_" + g.nspaceUC + serviceNameUC + "_CLIENT_INPUT_PROTOCOL, param_spec);\n\n" +
			"  param_spec = g_param_spec_object (\"output_protocol\",\n" +
			"                                    \"output protocol (construct)\",\n" +
			"                                    \"Set the client output protocol\",\n" +
			"                                    THRIFT_TYPE_PROTOCOL,\n" +
			"                                    G_PARAM_READWRITE);\n" +
			"  g_object_class_install_property (gobject_class,\n" +
			"                                   PROP_" + g.nspaceUC + serviceNameUC + "_CLIENT_OUTPUT_PROTOCOL, param_spec);\n")
	} else {
		g.fService.WriteString("  THRIFT_UNUSED_VAR (cls);\n")
	}
	g.fService.WriteString("}\n\n")
}

// generateServiceHandler generates C code that represents a Thrift
// service handler.
func (g *Generator) generateServiceHandler(tservice *sema.Service) {
	functions := tservice.Functions()

	serviceNameLC := toLowerCase(initialCapsToUnderscores(g.serviceName))
	serviceNameUC := toUpperCase(serviceNameLC)

	serviceHandlerName := g.serviceName + "Handler"

	className := g.nspace + serviceHandlerName
	classNameLC := g.nspaceLC + initialCapsToUnderscores(serviceHandlerName)
	classNameUC := toUpperCase(classNameLC)

	var parentClassName, parentTypeName string

	extendsService := tservice.Extends()

	if extendsService != nil {
		parentServiceName := extendsService.Name()
		parentServiceNameLC := toLowerCase(initialCapsToUnderscores(parentServiceName))
		parentServiceNameUC := toUpperCase(parentServiceNameLC)

		parentClassName = g.nspace + parentServiceName + "Handler"
		parentTypeName = g.nspaceUC + "TYPE_" + parentServiceNameUC + "_HANDLER"
	} else {
		parentClassName = "GObject"
		parentTypeName = "G_TYPE_OBJECT"
	}

	// Generate the handler instance definition
	g.fHeader.WriteString("/* " + g.serviceName + " handler (abstract base class) */\nstruct _" +
		className + "\n{\n")
	g.indentUp()
	g.fHeader.WriteString(g.indent() + parentClassName + " parent;\n")
	g.indentDown()
	g.fHeader.WriteString("};\ntypedef struct _" + className + " " + className + ";\n\n")

	// Generate the handler class definition, including its class members (methods)
	g.fHeader.WriteString("struct _" + className + "Class\n{\n")
	g.indentUp()
	g.fHeader.WriteString(g.indent() + parentClassName + "Class parent;\n\n")

	for _, f := range functions {
		methodName := initialCapsToUnderscores(f.Name())
		params := g.ifaceParams(f)
		g.fHeader.WriteString(g.indent() + "gboolean (*" + methodName + ") " + params + ";\n")
	}
	g.indentDown()

	g.fHeader.WriteString("};\ntypedef struct _" + className + "Class " + className + "Class;\n\n")

	// Generate the remaining header boilerplate
	g.fHeader.WriteString("GType " + classNameLC + "_get_type (void);\n#define " +
		g.nspaceUC + "TYPE_" + serviceNameUC + "_HANDLER (" + classNameLC + "_get_type())\n#define " +
		classNameUC + "(obj) (G_TYPE_CHECK_INSTANCE_CAST ((obj), " + g.nspaceUC + "TYPE_" +
		serviceNameUC + "_HANDLER, " + className + "))\n#define " +
		g.nspaceUC + "IS_" + serviceNameUC + "_HANDLER(obj) (G_TYPE_CHECK_INSTANCE_TYPE ((obj), " +
		g.nspaceUC + "TYPE_" + serviceNameUC + "_HANDLER))\n#define " + classNameUC +
		"_CLASS(c) (G_TYPE_CHECK_CLASS_CAST ((c), " + g.nspaceUC + "TYPE_" + serviceNameUC +
		"_HANDLER, " + className + "Class))\n#define " + g.nspaceUC + "IS_" + serviceNameUC +
		"_HANDLER_CLASS(c) (G_TYPE_CHECK_CLASS_TYPE ((c), " + g.nspaceUC + "TYPE_" + serviceNameUC +
		"_HANDLER))\n#define " + g.nspaceUC + serviceNameUC + "_HANDLER_GET_CLASS(obj) " +
		"(G_TYPE_INSTANCE_GET_CLASS ((obj), " + g.nspaceUC + "TYPE_" + serviceNameUC +
		"_HANDLER, " + className + "Class))\n\n")

	// Generate the handler class' method definitions
	for _, f := range functions {
		methodName := initialCapsToUnderscores(f.Name())
		params := g.ifaceParams(f)
		g.fHeader.WriteString("gboolean " + classNameLC + "_" + methodName + " " + params + ";\n")
	}
	g.fHeader.WriteString("\n")

	// Generate the handler's implementation in the implementation file

	// Generate the implementation boilerplate
	g.fService.WriteString("static void\n" + classNameLC + "_" + serviceNameLC +
		"_if_interface_init (" + g.nspace + g.serviceName + "IfInterface *iface);\n\n")

	argsIndent := strings.Repeat(" ", 25)
	g.fService.WriteString("G_DEFINE_TYPE_WITH_CODE (" + className + ", \n" + argsIndent +
		classNameLC + ",\n" + argsIndent + parentTypeName + ",\n" + argsIndent +
		"G_IMPLEMENT_INTERFACE (" + g.nspaceUC + "TYPE_" + serviceNameUC + "_IF,\n")
	argsIndent += strings.Repeat(" ", 23)
	g.fService.WriteString(argsIndent + classNameLC + "_" + serviceNameLC + "_if_interface_init))\n\n")

	// Generate the handler method implementations
	for _, f := range functions {
		methodName := initialCapsToUnderscores(f.Name())
		returnType := f.ReturnType()
		argList := f.Arglist()
		xList := f.Xceptions()

		args := argList.Members()
		xceptions := xList.Members()

		implementingFunction := newFuncSig(returnType, serviceNameLC+"_handler_"+methodName, argList, xList)

		g.fService.WriteString(g.indent() + g.functionSignature(implementingFunction) + "\n")
		g.scopeUp(&g.fService)
		g.fService.WriteString(g.indent() + "g_return_val_if_fail (" + g.nspaceUC + "IS_" +
			serviceNameUC + "_HANDLER (iface), FALSE);\n\n" + g.indent() +
			"return " + classNameUC + "_GET_CLASS (iface)->" + methodName + " (iface, ")

		if !returnType.IsVoid() {
			g.fService.WriteString("_return, ")
		}
		for _, field := range args {
			g.fService.WriteString(field.Name() + ", ")
		}
		for _, field := range xceptions {
			g.fService.WriteString(field.Name() + ", ")
		}
		g.fService.WriteString("error);\n")
		g.scopeDown(&g.fService)
		g.fService.WriteString("\n")
	}

	// Generate the handler interface initializer
	g.fService.WriteString("static void\n" + classNameLC + "_" + serviceNameLC +
		"_if_interface_init (" + g.nspace + g.serviceName + "IfInterface *iface)\n")
	g.scopeUp(&g.fService)
	if len(functions) > 0 {
		for _, f := range functions {
			methodName := initialCapsToUnderscores(f.Name())
			g.fService.WriteString(g.indent() + "iface->" + methodName + " = " + classNameLC + "_" +
				methodName + ";\n")
		}
	} else {
		g.fService.WriteString("THRIFT_UNUSED_VAR (iface);\n")
	}
	g.scopeDown(&g.fService)
	g.fService.WriteString("\n")

	// Generate the handler instance initializer
	g.fService.WriteString("static void\n" + classNameLC + "_init (" + className + " *self)\n")
	g.scopeUp(&g.fService)
	g.fService.WriteString(g.indent() + "THRIFT_UNUSED_VAR (self);\n")
	g.scopeDown(&g.fService)
	g.fService.WriteString("\n")

	// Generate the handler class initializer
	g.fService.WriteString("static void\n" + classNameLC + "_class_init (" + className + "Class *cls)\n")
	g.scopeUp(&g.fService)
	if len(functions) > 0 {
		for _, f := range functions {
			methodName := initialCapsToUnderscores(f.Name())
			// All methods are pure virtual and must be implemented by subclasses
			g.fService.WriteString(g.indent() + "cls->" + methodName + " = NULL;\n")
		}
	} else {
		g.fService.WriteString(g.indent() + "THRIFT_UNUSED_VAR (cls);\n")
	}
	g.scopeDown(&g.fService)
	g.fService.WriteString("\n")
}

// generateServiceProcessor generates C code that represents a Thrift
// service processor.
func (g *Generator) generateServiceProcessor(tservice *sema.Service) {
	functions := tservice.Functions()

	serviceNameLC := toLowerCase(initialCapsToUnderscores(g.serviceName))
	serviceNameUC := toUpperCase(serviceNameLC)

	serviceProcessorName := g.serviceName + "Processor"

	className := g.nspace + serviceProcessorName
	classNameLC := g.nspaceLC + initialCapsToUnderscores(serviceProcessorName)
	classNameUC := toUpperCase(classNameLC)

	var parentClassName, parentTypeName string
	var ai string

	handlerClassNameLC := initialCapsToUnderscores(g.nspace + g.serviceName + "Handler")

	processFunctionTypeName := className + "ProcessFunction"
	processFunctionDefTypeName := classNameLC + "_process_function_def"

	extendsService := tservice.Extends()

	if extendsService != nil {
		parentServiceName := extendsService.Name()
		parentServiceNameLC := toLowerCase(initialCapsToUnderscores(parentServiceName))
		parentServiceNameUC := toUpperCase(parentServiceNameLC)

		parentClassName = g.nspace + parentServiceName + "Processor"
		parentTypeName = g.nspaceUC + "TYPE_" + parentServiceNameUC + "_PROCESSOR"
	} else {
		parentClassName = "ThriftDispatchProcessor"
		parentTypeName = "THRIFT_TYPE_DISPATCH_PROCESSOR"
	}

	// Generate the processor instance definition
	g.fHeader.WriteString("/* " + g.serviceName + " processor */\nstruct _" + className + "\n{\n")
	g.indentUp()
	g.fHeader.WriteString(g.indent() + parentClassName + " parent;\n\n" + g.indent() +
		"/* protected */\n" + g.indent() + g.nspace + g.serviceName + "Handler *handler;\n" +
		g.indent() + "GHashTable *process_map;\n")
	g.indentDown()
	g.fHeader.WriteString("};\ntypedef struct _" + className + " " + className + ";\n\n")

	// Generate the processor class definition
	g.fHeader.WriteString("struct _" + className + "Class\n{\n")
	g.indentUp()
	g.fHeader.WriteString(g.indent() + parentClassName + "Class parent;\n\n" + g.indent() +
		"/* protected */\n" + g.indent() +
		"gboolean (*dispatch_call) (ThriftDispatchProcessor *processor,\n")
	argsIndent := g.indent() + strings.Repeat(" ", 27)
	g.fHeader.WriteString(argsIndent + "ThriftProtocol *in,\n" + argsIndent + "ThriftProtocol *out,\n" +
		argsIndent + "gchar *fname,\n" + argsIndent + "gint32 seqid,\n" + argsIndent + "GError **error);\n")
	g.indentDown()
	g.fHeader.WriteString("};\ntypedef struct _" + className + "Class " + className + "Class;\n\n")

	// Generate the remaining header boilerplate
	g.fHeader.WriteString("GType " + classNameLC + "_get_type (void);\n#define " +
		g.nspaceUC + "TYPE_" + serviceNameUC + "_PROCESSOR (" + classNameLC + "_get_type())\n#define " +
		classNameUC + "(obj) (G_TYPE_CHECK_INSTANCE_CAST ((obj), " + g.nspaceUC + "TYPE_" +
		serviceNameUC + "_PROCESSOR, " + className + "))\n#define " +
		g.nspaceUC + "IS_" + serviceNameUC + "_PROCESSOR(obj) (G_TYPE_CHECK_INSTANCE_TYPE ((obj), " +
		g.nspaceUC + "TYPE_" + serviceNameUC + "_PROCESSOR))\n#define " + classNameUC +
		"_CLASS(c) (G_TYPE_CHECK_CLASS_CAST ((c), " + g.nspaceUC + "TYPE_" + serviceNameUC +
		"_PROCESSOR, " + className + "Class))\n#define " + g.nspaceUC + "IS_" + serviceNameUC +
		"_PROCESSOR_CLASS(c) (G_TYPE_CHECK_CLASS_TYPE ((c), " + g.nspaceUC + "TYPE_" + serviceNameUC +
		"_PROCESSOR))\n#define " + g.nspaceUC + serviceNameUC + "_PROCESSOR_GET_CLASS(obj) " +
		"(G_TYPE_INSTANCE_GET_CLASS ((obj), " + g.nspaceUC + "TYPE_" + serviceNameUC +
		"_PROCESSOR, " + className + "Class))\n\n")

	// Generate the processor's properties enum
	g.fService.WriteString("enum _" + className + "Properties\n{\n")
	g.indentUp()
	g.fService.WriteString(g.indent() + "PROP_" + classNameUC + "_0,\n" + g.indent() + "PROP_" +
		classNameUC + "_HANDLER\n")
	g.indentDown()
	g.fService.WriteString("};\n\n")

	// Generate the implementation boilerplate
	ai15 := strings.Repeat(" ", 15)
	g.fService.WriteString("G_DEFINE_TYPE (" + className + ",\n" + ai15 + classNameLC + ",\n" +
		ai15 + parentTypeName + ")\n\n")

	// Generate the processor's processing-function type
	aiPF := strings.Repeat(" ", len(processFunctionTypeName)+23)
	g.fService.WriteString("typedef gboolean (* " + processFunctionTypeName + ") (" + className + " *, \n" +
		aiPF + "gint32,\n" + aiPF + "ThriftProtocol *,\n" + aiPF + "ThriftProtocol *,\n" +
		aiPF + "GError **);\n\n")

	// Generate the processor's processing-function-definition type
	g.fService.WriteString("typedef struct\n{\n")
	g.indentUp()
	g.fService.WriteString(g.indent() + "gchar *name;\n" + g.indent() + processFunctionTypeName + " function;\n")
	g.indentDown()
	g.fService.WriteString("} " + processFunctionDefTypeName + ";\n\n")

	// Generate forward declarations of the processor's processing functions so we
	// can refer to them in the processing-function-definition struct below and
	// keep all of the processor's declarations in one place
	for _, f := range functions {
		functionName := classNameLC + "_process_" + initialCapsToUnderscores(f.Name())
		ai := strings.Repeat(" ", len(functionName)+2)
		g.fService.WriteString("static gboolean\n" + functionName + " (" + className + " *,\n" +
			ai + "gint32,\n" + ai + "ThriftProtocol *,\n" + ai + "ThriftProtocol *,\n" +
			ai + "GError **);\n")
	}
	g.fService.WriteString("\n")

	// Generate the processor's processing-function definitions, if the service
	// defines any methods
	if len(functions) > 0 {
		g.fService.WriteString(g.indent() + "static " + processFunctionDefTypeName + "\n" +
			g.indent() + classNameLC + "_process_function_defs[" + strconv.Itoa(len(functions)) + "] = {\n")
		g.indentUp()
		for i, f := range functions {
			serviceFunctionName := f.Name()
			processFunctionName := classNameLC + "_process_" + initialCapsToUnderscores(serviceFunctionName)

			g.fService.WriteString(g.indent() + "{\n")
			g.indentUp()
			g.fService.WriteString(g.indent() + "\"" + serviceFunctionName + "\",\n" +
				g.indent() + processFunctionName + "\n")
			g.indentDown()
			comma := ","
			if i == len(functions)-1 {
				comma = ""
			}
			g.fService.WriteString(g.indent() + "}" + comma + "\n")
		}
		g.indentDown()
		g.fService.WriteString(g.indent() + "};\n\n")
	}

	// Generate the processor's processing functions
	for _, f := range functions {
		serviceFunctionName := f.Name()
		serviceFunctionNameIC := underscoresToInitialCaps(serviceFunctionName)
		serviceFunctionNameLC := initialCapsToUnderscores(serviceFunctionName)
		serviceFunctionNameUC := toUpperCase(serviceFunctionNameLC)

		returnType := f.ReturnType()
		hasReturnValue := !returnType.IsVoid()

		argList := f.Arglist()
		args := argList.Members()

		xceptions := f.Xceptions().Members()

		argsClassName := g.nspace + g.serviceName + serviceFunctionNameIC + "Args"
		argsClassType := g.nspaceUC + "TYPE_" + serviceNameUC + "_" + serviceFunctionNameUC + "_ARGS"

		resultClassName := g.nspace + g.serviceName + serviceFunctionNameIC + "Result"
		resultClassType := g.nspaceUC + "TYPE_" + serviceNameUC + "_" + serviceFunctionNameUC + "_RESULT"

		handlerFunctionName := handlerClassNameLC + "_" + serviceFunctionNameLC

		functionName := classNameLC + "_process_" + initialCapsToUnderscores(serviceFunctionName)

		ai := strings.Repeat(" ", len(functionName)+2)
		g.fService.WriteString("static gboolean\n" + functionName + " (" + className + " *self,\n" +
			ai + "gint32 sequence_id,\n" + ai + "ThriftProtocol *input_protocol,\n" +
			ai + "ThriftProtocol *output_protocol,\n" + ai + "GError **error)\n")
		g.scopeUp(&g.fService)
		g.fService.WriteString(g.indent() + "gboolean result = TRUE;\n" +
			g.indent() + "ThriftTransport * transport;\n" +
			g.indent() + "ThriftApplicationException *xception;\n" +
			g.indent() + argsClassName + " * args =\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + "g_object_new (" + argsClassType + ", NULL);\n\n")
		g.indentDown()
		if f.IsOneway() {
			g.fService.WriteString(g.indent() + "THRIFT_UNUSED_VAR (sequence_id);\n" +
				g.indent() + "THRIFT_UNUSED_VAR (output_protocol);\n\n")
		}
		g.fService.WriteString(g.indent() + "g_object_get (input_protocol, \"transport\", &transport, NULL);\n\n")

		// Read the method's arguments from the caller
		g.fService.WriteString(g.indent() + "if ((thrift_struct_read (THRIFT_STRUCT (args), input_protocol, error) != -1) &&\n" +
			g.indent() + "    (thrift_protocol_read_message_end (input_protocol, error) != -1) &&\n" +
			g.indent() + "    (thrift_transport_read_end (transport, error) != FALSE))\n")
		g.scopeUp(&g.fService)

		for _, arg := range args {
			g.fService.WriteString(g.indent() + g.propertyTypeName(arg.Type(), false, false) + " " + arg.Name() + ";\n")
		}
		for _, x := range xceptions {
			g.fService.WriteString(g.indent() + g.typeName(x.Type(), false, false) + " " +
				initialCapsToUnderscores(x.Name()) + " = NULL;\n")
		}
		if hasReturnValue {
			g.fService.WriteString(g.indent() + g.propertyTypeName(returnType, false, false) + " return_value;\n")
		}
		if !f.IsOneway() {
			g.fService.WriteString(g.indent() + resultClassName + " * result_struct;\n")
		}
		g.fService.WriteString("\n")

		if len(args) > 0 {
			g.fService.WriteString(g.indent() + "g_object_get (args,\n")
			ai := g.indent() + strings.Repeat(" ", 14)
			for _, arg := range args {
				argName := arg.Name()
				g.fService.WriteString(ai + "\"" + argName + "\", &" + argName + ",\n")
			}
			g.fService.WriteString(ai + "NULL);\n\n")
		}

		if !f.IsOneway() {
			g.fService.WriteString(g.indent() + "g_object_unref (transport);\n" + g.indent() +
				"g_object_get (output_protocol, \"transport\", &transport, NULL);\n\n" + g.indent() +
				"result_struct = g_object_new (" + resultClassType + ", NULL);\n")
			if hasReturnValue {
				g.fService.WriteString(g.indent() + "g_object_get (result_struct, \"success\", &return_value, NULL);\n")
			}
			g.fService.WriteString("\n")
		}

		// Pass the arguments to the corresponding method in the handler
		g.fService.WriteString(g.indent() + "if (" + handlerFunctionName + " (" + g.nspaceUC +
			serviceNameUC + "_IF (self->handler),\n")
		ai = g.indent() + strings.Repeat(" ", len(handlerFunctionName)+6)
		if hasReturnValue {
			returnTypeName := g.typeName(returnType, false, false)

			g.fService.WriteString(ai)

			// Cast return_value if it was declared as a type other than the return
			// value's actual type---this is true for integer values 32 bits or fewer
			// in width, for which GLib requires a plain gint type be used when
			// storing or retrieving as an object property
			if returnTypeName != g.propertyTypeName(returnType, false, false) {
				if returnTypeName[len(returnTypeName)-1] != '*' {
					returnTypeName += " "
				}
				returnTypeName += "*"

				g.fService.WriteString("(" + returnTypeName + ")")
			}

			g.fService.WriteString("&return_value,\n")
		}
		for _, arg := range args {
			g.fService.WriteString(ai + arg.Name() + ",\n")
		}
		for _, x := range xceptions {
			g.fService.WriteString(ai + "&" + initialCapsToUnderscores(x.Name()) + ",\n")
		}
		g.fService.WriteString(ai + "error) == TRUE)\n")
		g.scopeUp(&g.fService)

		// The handler reported success; return the result, if any, to the caller
		if !f.IsOneway() {
			if hasReturnValue {
				g.fService.WriteString(g.indent() + "g_object_set (result_struct, \"success\", ")
				if g.typeName(returnType, false, false) != g.propertyTypeName(returnType, false, false) {
					// Roundtrip cast to fix the position of sign bit.
					g.fService.WriteString("(" + g.propertyTypeName(returnType, false, false) + ")" +
						"(" + g.typeName(returnType, false, false) + ")")
				}
				g.fService.WriteString("return_value, NULL);\n")
				g.fService.WriteString("\n")
			}
			g.fService.WriteString(g.indent() + "result =\n")
			g.indentUp()
			g.fService.WriteString(g.indent() + "((thrift_protocol_write_message_begin (output_protocol,\n")
			ai = g.indent() + strings.Repeat(" ", 39)
			g.fService.WriteString(ai + "\"" + serviceFunctionName + "\",\n" + ai + "T_REPLY,\n" +
				ai + "sequence_id,\n" + ai + "error) != -1) &&\n" + g.indent() +
				" (thrift_struct_write (THRIFT_STRUCT (result_struct),\n")
			ai = g.indent() + strings.Repeat(" ", 23)
			g.fService.WriteString(ai + "output_protocol,\n" + ai + "error) != -1));\n")
			g.indentDown()
		}
		g.scopeDown(&g.fService)
		g.fService.WriteString(g.indent() + "else\n")
		g.scopeUp(&g.fService)

		// The handler reported failure; check to see if an application-defined
		// exception was raised and if so, return it to the caller
		g.fService.WriteString(g.indent())
		if len(xceptions) > 0 {
			for _, x := range xceptions {
				g.fService.WriteString("if (" + initialCapsToUnderscores(x.Name()) + " != NULL)\n")
				g.scopeUp(&g.fService)
				g.fService.WriteString(g.indent() + "g_object_set (result_struct,\n")
				ai = g.indent() + strings.Repeat(" ", 14)
				g.fService.WriteString(ai + "\"" + x.Name() + "\", " + x.Name() + ",\n" + ai + "NULL);\n\n")
				g.fService.WriteString(g.indent() + "g_object_unref (" + x.Name() + ");\n")
				g.fService.WriteString(g.indent() + "result =\n")
				g.indentUp()
				g.fService.WriteString(g.indent() + "((thrift_protocol_write_message_begin (output_protocol,\n")
				ai = g.indent() + strings.Repeat(" ", 39)
				g.fService.WriteString(ai + "\"" + serviceFunctionName + "\",\n" + ai + "T_REPLY,\n" +
					ai + "sequence_id,\n" + ai + "error) != -1) &&\n" + g.indent() +
					" (thrift_struct_write (THRIFT_STRUCT (result_struct),\n")
				ai = g.indent() + strings.Repeat(" ", 23)
				g.fService.WriteString(ai + "output_protocol,\n" + ai + "error) != -1));\n")
				g.indentDown()
				g.scopeDown(&g.fService)
				g.fService.WriteString(g.indent() + "else\n")
			}

			g.scopeUp(&g.fService)
			g.fService.WriteString(g.indent())
		}

		// If the handler reported failure but raised no application-defined
		// exception, return a Thrift application exception with the information
		// returned via GLib's own error-reporting mechanism
		g.fService.WriteString("if (*error == NULL)\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + "g_warning (\"" + g.serviceName + "." + f.Name() +
			" implementation returned FALSE \"\n" + g.indent() + strings.Repeat(" ", 11) +
			"\"but did not set an error\");\n\n")
		g.indentDown()
		g.fService.WriteString(g.indent() + "xception =\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + "g_object_new (THRIFT_TYPE_APPLICATION_EXCEPTION,\n")
		ai = g.indent() + strings.Repeat(" ", 14)
		g.fService.WriteString(ai + "\"type\",    *error != NULL ? (*error)->code :\n" +
			ai + strings.Repeat(" ", 11) + "THRIFT_APPLICATION_EXCEPTION_ERROR_UNKNOWN,\n" +
			ai + "\"message\", *error != NULL ? (*error)->message : NULL,\n" +
			ai + "NULL);\n")
		g.indentDown()
		g.fService.WriteString(g.indent() + "g_clear_error (error);\n\n" + g.indent() + "result =\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + "((thrift_protocol_write_message_begin (output_protocol,\n")
		ai = g.indent() + strings.Repeat(" ", 39)
		g.fService.WriteString(ai + "\"" + serviceFunctionName + "\",\n" + ai + "T_EXCEPTION,\n" +
			ai + "sequence_id,\n" + ai + "error) != -1) &&\n" + g.indent() +
			" (thrift_struct_write (THRIFT_STRUCT (xception),\n")
		ai = g.indent() + strings.Repeat(" ", 23)
		g.fService.WriteString(ai + "output_protocol,\n" + ai + "error) != -1));\n")
		g.indentDown()
		g.fService.WriteString("\n" + g.indent() + "g_object_unref (xception);\n")

		if len(xceptions) > 0 {
			g.scopeDown(&g.fService)
		}
		g.scopeDown(&g.fService)
		g.fService.WriteString("\n")

		// Dellocate or unref retrieved argument values as necessary
		for _, arg := range args {
			argName := arg.Name()
			argType := sema.TrueType(arg.Type())

			if argType.IsBaseType() {
				bt := argType.(*sema.BaseType)

				if bt.Base() == sema.TypeString {
					g.fService.WriteString(g.indent() + "if (" + argName + " != NULL)\n")
					g.indentUp()
					if bt.IsBinary() {
						g.fService.WriteString(g.indent() + "g_byte_array_unref (" + argName + ");\n")
					} else {
						g.fService.WriteString(g.indent() + "g_free (" + argName + ");\n")
					}
					g.indentDown()
				}
			} else if argType.IsContainer() {
				g.fService.WriteString(g.indent() + "if (" + argName + " != NULL)\n")
				g.indentUp()

				if argType.IsList() {
					elemType := argType.(*sema.List).ElemType()

					g.fService.WriteString(g.indent())
					if g.isNumeric(elemType) {
						g.fService.WriteString("g_array_unref")
					} else {
						g.fService.WriteString("g_ptr_array_unref")
					}
					g.fService.WriteString(" (" + argName + ");\n")
				} else if argType.IsMap() || argType.IsSet() {
					g.fService.WriteString(g.indent() + "g_hash_table_unref (" + argName + ");\n")
				}

				g.indentDown()
			} else if argType.IsStruct() {
				g.fService.WriteString(g.indent() + "if (" + argName + " != NULL)\n")
				g.indentUp()
				g.fService.WriteString(g.indent() + "g_object_unref (" + argName + ");\n")
				g.indentDown()
			}
		}

		if !f.IsOneway() {
			if hasReturnValue {
				// Deallocate (or unref) return_value
				rt := sema.TrueType(returnType)
				if rt.IsBaseType() {
					bt := rt.(*sema.BaseType)
					if bt.Base() == sema.TypeString {
						g.fService.WriteString(g.indent() + "if (return_value != NULL)\n")
						g.indentUp()
						if bt.IsBinary() {
							g.fService.WriteString(g.indent() + "g_byte_array_unref (return_value);\n")
						} else {
							g.fService.WriteString(g.indent() + "g_free (return_value);\n")
						}
						g.indentDown()
					}
				} else if rt.IsContainer() {
					g.fService.WriteString(g.indent() + "if (return_value != NULL)\n")
					g.indentUp()

					if rt.IsList() {
						elemType := rt.(*sema.List).ElemType()

						g.fService.WriteString(g.indent())
						if g.isNumeric(elemType) {
							g.fService.WriteString("g_array_unref")
						} else {
							g.fService.WriteString("g_ptr_array_unref")
						}
						g.fService.WriteString(" (return_value);\n")
					} else if rt.IsMap() || rt.IsSet() {
						g.fService.WriteString(g.indent() + "g_hash_table_unref (return_value);\n")
					}

					g.indentDown()
				} else if rt.IsStruct() {
					g.fService.WriteString(g.indent() + "if (return_value != NULL)\n")
					g.indentUp()
					g.fService.WriteString(g.indent() + "g_object_unref (return_value);\n")
					g.indentDown()
				}
			}
			g.fService.WriteString(g.indent() + "g_object_unref (result_struct);\n\n" + g.indent() +
				"if (result == TRUE)\n")
			g.indentUp()
			g.fService.WriteString(g.indent() + "result =\n")
			g.indentUp()
			g.fService.WriteString(g.indent() + "((thrift_protocol_write_message_end (output_protocol, error) != -1) &&\n" +
				g.indent() + " (thrift_transport_write_end (transport, error) != FALSE) &&\n" +
				g.indent() + " (thrift_transport_flush (transport, error) != FALSE));\n")
			g.indentDown()
			g.indentDown()
		}
		g.scopeDown(&g.fService)
		g.fService.WriteString(g.indent() + "else\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + "result = FALSE;\n")
		g.indentDown()

		g.fService.WriteString("\n" + g.indent() + "g_object_unref (transport);\n" + g.indent() +
			"g_object_unref (args);\n\n" + g.indent() + "return result;\n")
		g.scopeDown(&g.fService)

		g.fService.WriteString("\n")
	}

	// Generate the processor's dispatch_call implementation
	functionName := classNameLC + "_dispatch_call"
	ai = g.indent() + strings.Repeat(" ", len(functionName)+2)
	g.fService.WriteString("static gboolean\n" + functionName + " (ThriftDispatchProcessor *dispatch_processor,\n" +
		ai + "ThriftProtocol *input_protocol,\n" + ai + "ThriftProtocol *output_protocol,\n" +
		ai + "gchar *method_name,\n" + ai + "gint32 sequence_id,\n" + ai + "GError **error)\n")
	g.scopeUp(&g.fService)
	g.fService.WriteString(g.indent() + classNameLC + "_process_function_def *process_function_def;\n")
	g.fService.WriteString(g.indent() + "gboolean dispatch_result = FALSE;\n\n" + g.indent() +
		className + " *self = " + classNameUC + " (dispatch_processor);\n")
	g.fService.WriteString(g.indent() + parentClassName + "Class *parent_class =\n")
	g.indentUp()
	g.fService.WriteString(g.indent() + "g_type_class_peek_parent (g_type_class_peek (" + classNameLC + "_get_type ()));\n")
	g.indentDown()
	g.fService.WriteString("\n" + g.indent() + "process_function_def = " +
		"g_hash_table_lookup (self->process_map, method_name);\n" +
		g.indent() + "if (process_function_def != NULL)\n")
	g.scopeUp(&g.fService)
	ai = g.indent() + strings.Repeat(" ", 53)
	g.fService.WriteString(g.indent() + "g_free (method_name);\n" +
		g.indent() + "dispatch_result = (*process_function_def->function) (self,\n" +
		ai + "sequence_id,\n" + ai + "input_protocol,\n" + ai + "output_protocol,\n" + ai + "error);\n")
	g.scopeDown(&g.fService)
	g.fService.WriteString(g.indent() + "else\n")
	g.scopeUp(&g.fService)

	// Method name not recognized; chain up to our parent processor---note the
	// top-most implementation of this method, in ThriftDispatchProcessor itself,
	// will return an application exception to the caller if no class in the
	// hierarchy recognizes the method name
	g.fService.WriteString(g.indent() + "dispatch_result = parent_class->dispatch_call (dispatch_processor,\n")
	ai = g.indent() + strings.Repeat(" ", 47)
	g.fService.WriteString(ai + "input_protocol,\n" + ai + "output_protocol,\n" + ai + "method_name,\n" +
		ai + "sequence_id,\n" + ai + "error);\n")
	g.scopeDown(&g.fService)
	g.fService.WriteString("\n" + g.indent() + "return dispatch_result;\n")
	g.scopeDown(&g.fService)
	g.fService.WriteString("\n")

	// Generate the processor's property setter
	functionName = classNameLC + "_set_property"
	ai = strings.Repeat(" ", len(functionName)+2)
	g.fService.WriteString("static void\n" + functionName + " (GObject *object,\n" +
		ai + "guint property_id,\n" + ai + "const GValue *value,\n" + ai + "GParamSpec *pspec)\n")
	g.scopeUp(&g.fService)
	g.fService.WriteString(g.indent() + className + " *self = " + classNameUC + " (object);\n\n" +
		g.indent() + "switch (property_id)\n")
	g.scopeUp(&g.fService)
	g.fService.WriteString(g.indent() + "case PROP_" + classNameUC + "_HANDLER:\n")
	g.indentUp()
	g.fService.WriteString(g.indent() + "if (self->handler != NULL)\n")
	g.indentUp()
	g.fService.WriteString(g.indent() + "g_object_unref (self->handler);\n")
	g.indentDown()
	g.fService.WriteString(g.indent() + "self->handler = g_value_get_object (value);\n" + g.indent() +
		"g_object_ref (self->handler);\n")
	if extendsService != nil {
		// Chain up to set the handler in every superclass as well
		g.fService.WriteString("\n" + g.indent() + "G_OBJECT_CLASS (" + classNameLC + "_parent_class)->\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + "set_property (object, property_id, value, pspec);\n")
		g.indentDown()
	}
	g.fService.WriteString(g.indent() + "break;\n")
	g.indentDown()
	g.fService.WriteString(g.indent() + "default:\n")
	g.indentUp()
	g.fService.WriteString(g.indent() + "G_OBJECT_WARN_INVALID_PROPERTY_ID (object, property_id, pspec);\n" +
		g.indent() + "break;\n")
	g.indentDown()
	g.scopeDown(&g.fService)
	g.scopeDown(&g.fService)
	g.fService.WriteString("\n")

	// Generate processor's property getter
	functionName = classNameLC + "_get_property"
	ai = strings.Repeat(" ", len(functionName)+2)
	g.fService.WriteString("static void\n" + functionName + " (GObject *object,\n" +
		ai + "guint property_id,\n" + ai + "GValue *value,\n" + ai + "GParamSpec *pspec)\n")
	g.scopeUp(&g.fService)
	g.fService.WriteString(g.indent() + className + " *self = " + classNameUC + " (object);\n\n" +
		g.indent() + "switch (property_id)\n")
	g.scopeUp(&g.fService)
	g.fService.WriteString(g.indent() + "case PROP_" + classNameUC + "_HANDLER:\n")
	g.indentUp()
	g.fService.WriteString(g.indent() + "g_value_set_object (value, self->handler);\n" + g.indent() + "break;\n")
	g.indentDown()
	g.fService.WriteString(g.indent() + "default:\n")
	g.indentUp()
	g.fService.WriteString(g.indent() + "G_OBJECT_WARN_INVALID_PROPERTY_ID (object, property_id, pspec);\n" +
		g.indent() + "break;\n")
	g.indentDown()
	g.scopeDown(&g.fService)
	g.scopeDown(&g.fService)
	g.fService.WriteString("\n")

	// Generator the processor's dispose function
	g.fService.WriteString("static void\n" + classNameLC + "_dispose (GObject *gobject)\n")
	g.scopeUp(&g.fService)
	g.fService.WriteString(g.indent() + className + " *self = " + classNameUC + " (gobject);\n\n" +
		g.indent() + "if (self->handler != NULL)\n")
	g.scopeUp(&g.fService)
	g.fService.WriteString(g.indent() + "g_object_unref (self->handler);\n" + g.indent() + "self->handler = NULL;\n")
	g.scopeDown(&g.fService)
	g.fService.WriteString("\n" + g.indent() + "G_OBJECT_CLASS (" + classNameLC + "_parent_class)->dispose (gobject);\n")
	g.scopeDown(&g.fService)
	g.fService.WriteString("\n")

	// Generate processor finalize function
	g.fService.WriteString("static void\n" + classNameLC + "_finalize (GObject *gobject)\n")
	g.scopeUp(&g.fService)
	g.fService.WriteString(g.indent() + g.nspace + g.serviceName + "Processor *self = " + g.nspaceUC +
		serviceNameUC + "_PROCESSOR (gobject);\n\n" + g.indent() +
		"thrift_safe_hash_table_destroy (self->process_map);\n\n" + g.indent() +
		"G_OBJECT_CLASS (" + classNameLC + "_parent_class)->finalize (gobject);\n")
	g.scopeDown(&g.fService)
	g.fService.WriteString("\n")

	// Generate processor instance initializer
	g.fService.WriteString("static void\n" + classNameLC + "_init (" + className + " *self)\n")
	g.scopeUp(&g.fService)
	if len(functions) > 0 {
		g.fService.WriteString(g.indent() + "guint index;\n\n")
	}
	g.fService.WriteString(g.indent() + "self->handler = NULL;\n" + g.indent() +
		"self->process_map = g_hash_table_new (g_str_hash, g_str_equal);\n")
	if len(functions) > 0 {
		ai := strings.Repeat(" ", 21)
		g.fService.WriteString("\n" + g.indent() + "for (index = 0; index < " + strconv.Itoa(len(functions)) + "; index += 1)\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + "g_hash_table_insert (self->process_map,\n" +
			g.indent() + ai + classNameLC + "_process_function_defs[index].name,\n" +
			g.indent() + ai + "&" + classNameLC + "_process_function_defs[index]);\n")
		g.indentDown()
	}
	g.scopeDown(&g.fService)
	g.fService.WriteString("\n")

	// Generate processor class initializer
	g.fService.WriteString("static void\n" + classNameLC + "_class_init (" + className + "Class *cls)\n")
	g.scopeUp(&g.fService)
	g.fService.WriteString(g.indent() + "GObjectClass *gobject_class = G_OBJECT_CLASS (cls);\n" +
		g.indent() + "ThriftDispatchProcessorClass *dispatch_processor_class =\n")
	g.indentUp()
	g.fService.WriteString(g.indent() + "THRIFT_DISPATCH_PROCESSOR_CLASS (cls);\n")
	g.indentDown()
	g.fService.WriteString(g.indent() + "GParamSpec *param_spec;\n\n" + g.indent() +
		"gobject_class->dispose = " + classNameLC + "_dispose;\n" + g.indent() +
		"gobject_class->finalize = " + classNameLC + "_finalize;\n" + g.indent() +
		"gobject_class->set_property = " + classNameLC + "_set_property;\n" + g.indent() +
		"gobject_class->get_property = " + classNameLC + "_get_property;\n\n" + g.indent() +
		"dispatch_processor_class->dispatch_call = " + classNameLC + "_dispatch_call;\n" + g.indent() +
		"cls->dispatch_call = " + classNameLC + "_dispatch_call;\n\n" + g.indent() +
		"param_spec = g_param_spec_object (\"handler\",\n")
	ai = g.indent() + strings.Repeat(" ", 34)
	g.fService.WriteString(ai + "\"Service handler implementation\",\n" + ai +
		"\"The service handler implementation \"\n" + ai +
		"\"to which method calls are dispatched.\",\n" + ai +
		g.nspaceUC + "TYPE_" + serviceNameUC + "_HANDLER,\n" + ai + "G_PARAM_READWRITE);\n")
	g.fService.WriteString(g.indent() + "g_object_class_install_property (gobject_class,\n")
	ai = g.indent() + strings.Repeat(" ", 33)
	g.fService.WriteString(ai + "PROP_" + classNameUC + "_HANDLER,\n" + ai + "param_spec);\n")
	g.scopeDown(&g.fService)
}
