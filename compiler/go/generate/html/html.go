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

// Package html is t_html_generator.cc: it renders a resolved program as
// the HTML documentation the C++ compiler's --gen html writes, byte for
// byte, including the embedded Bootstrap stylesheet.
package html

import (
	"sort"
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// Options are the html:... generator options.
type Options struct {
	// Standalone is "standalone": self-contained mode, includes all CSS
	// in the HTML files instead of a separate style.css.
	Standalone bool
	// NoEscape is "noescape": do not escape html in doc text.
	NoEscape bool
}

// ParseOptions parses the part after "html:" of a --gen argument.
func ParseOptions(spec string) (Options, error) {
	var o Options
	for _, option := range strings.Split(spec, ",") {
		key := option
		if i := strings.IndexByte(option, '='); i >= 0 {
			key = option[:i]
		}
		switch key {
		case "":
		case "standalone":
			o.Standalone = true
		case "noescape":
			o.NoEscape = true
		default:
			return o, &emit.Error{Msg: "unknown option html:" + key}
		}
	}
	return o, nil
}

func init() {
	generate.Register(generate.Info{
		Name:     "html",
		LongName: "HTML",
		Options: []generate.Option{
			{Name: "standalone", Help: "Self-contained mode, includes all CSS in the HTML files.\nGenerates no style.css file, but HTML files will be larger."},
			{Name: "noescape", Help: "Do not escape html in doc text."},
		},
		Parse: func(spec string) (generate.Runner, error) {
			opts, err := ParseOptions(spec)
			if err != nil {
				return nil, err
			}
			return runner{opts}, nil
		},
	})
}

type runner struct{ opts Options }

func (r runner) Run(program *sema.Program, recurse bool) error {
	return Run(program, r.opts, recurse)
}

// Run generates the program and, when recurse is set, every program it
// includes first, each inheriting the output path.
func Run(program *sema.Program, opts Options, recurse bool) error {
	if recurse {
		program.SetRecursive(true)
		for _, inc := range program.Includes() {
			inc.SetOutPath(program.OutPath(), program.IsOutPathAbsolute())
			if err := Run(inc, opts, recurse); err != nil {
				return err
			}
		}
	}
	return generateOne(program, opts)
}

func generateOne(program *sema.Program, opts Options) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*sema.Error); ok {
				err = e
				return
			}
			if e, ok := r.(*emit.Error); ok {
				err = e
				return
			}
			panic(r)
		}
	}()
	program.Scope.ResolveAllConsts()
	sema.ValidateInput(program)
	g := newGenerator(program, opts)
	g.generateProgram()
	return nil
}

// inputType is input_type: how the doc-comment text presented to
// escape_html has been classified so far.
type inputType int

const (
	inputUnknown inputType = iota
	inputUTF8
	inputPlain
)

// docHolder is t_doc: anything print_doc can be called on.
type docHolder interface {
	HasDoc() bool
	Doc() string
}

// generator is t_html_generator. A fresh one is created for every program,
// matching the C++ compiler creating a new t_generator per t_program in
// main.cc's generate().
type generator struct {
	program       *sema.Program
	opts          Options
	outDir        string
	sb            *strings.Builder
	currentFile   string
	inputType     inputType
	allowedMarkup map[string]bool
	serviceName   string
}

func newGenerator(program *sema.Program, opts Options) *generator {
	outDir := program.OutPath() + "gen-html/"
	if program.IsOutPathAbsolute() {
		outDir = program.OutPath() + "/"
	}
	g := &generator{
		program:   program,
		opts:      opts,
		outDir:    outDir,
		inputType: inputUnknown,
	}
	g.initAllowedMarkup()
	return g
}

// initAllowedMarkup is init_allowed__markup: the doc-comment markup tags
// that survive escape_html_tags unescaped.
func (g *generator) initAllowedMarkup() {
	tags := []string{
		"br", "br/", "img",
		"b", "/b", "u", "/u", "i", "/i", "s", "/s",
		"big", "/big", "small", "/small", "sup", "/sup", "sub", "/sub",
		"pre", "/pre", "tt", "/tt", "ul", "/ul", "ol", "/ol", "li", "/li",
		"a", "/a", "p", "/p", "code", "/code", "dl", "/dl", "dt", "/dt", "dd", "/dd",
		"h1", "/h1", "h2", "/h2", "h3", "/h3", "h4", "/h4", "h5", "/h5", "h6", "/h6",
	}
	g.allowedMarkup = make(map[string]bool, len(tags))
	for _, t := range tags {
		g.allowedMarkup[t] = true
	}
}

// write appends to the file currently being built.
func (g *generator) write(s string) {
	g.sb.WriteString(s)
}

// generateProgram is generate_program: prepares for file generation by
// opening up the necessary file output stream.
func (g *generator) generateProgram() {
	emit.Mkdir(g.outDir)
	g.currentFile = g.program.Name() + ".html"
	g.sb = &strings.Builder{}
	g.write("<!DOCTYPE html>\n")
	g.write("<html lang=\"en\">\n")
	g.write("<head>\n")
	g.write("<meta http-equiv=\"Content-Type\" content=\"text/html;charset=utf-8\" />\n")
	g.generateStyleTag()
	g.write("<title>Thrift module: " + g.program.Name() + "</title></head><body>\n")
	g.write("<div class=\"container-fluid\">\n")
	g.write("<h1>Thrift module: " + g.program.Name() + "</h1>\n")

	g.printDoc(g.program)

	g.generateProgramTOC()

	if len(g.program.Consts()) != 0 {
		g.write("<hr/><h2 id=\"Constants\">Constants</h2>\n")
		g.write("<table class=\"table-bordered table-striped table-condensed\">")
		g.write("<thead><tr><th>Constant</th><th>Type</th><th>Value</th></tr></thead><tbody>\n")
		for _, c := range g.program.Consts() {
			g.generateConst(c)
		}
		g.write("</tbody></table>")
	}

	if len(g.program.Enums()) != 0 {
		g.write("<hr/><h2 id=\"Enumerations\">Enumerations</h2>\n")
		for _, e := range g.program.Enums() {
			g.generateEnum(e)
		}
	}

	if len(g.program.Typedefs()) != 0 {
		g.write("<hr/><h2 id=\"Typedefs\">Type declarations</h2>\n")
		for _, t := range g.program.Typedefs() {
			g.generateTypedef(t)
		}
	}

	if len(g.program.Objects()) != 0 {
		g.write("<hr/><h2 id=\"Structs\">Data structures</h2>\n")
		for _, o := range g.program.Objects() {
			if o.IsXception() {
				g.generateXception(o)
			} else {
				g.generateStruct(o)
			}
		}
	}

	if len(g.program.Services()) != 0 {
		g.write("<hr/><h2 id=\"Services\">Services</h2>\n")
		for _, s := range g.program.Services() {
			g.serviceName = s.Name()
			g.generateService(s)
		}
	}

	g.write("</div></body></html>\n")
	emit.WriteFile(g.outDir+g.currentFile, g.sb.String())

	g.generateIndex()
	g.generateCSS()
}

// generateProgramTOC is generate_program_toc: emits the Table of Contents
// links at the top of the module's page.
func (g *generator) generateProgramTOC() {
	g.write("<table class=\"table-bordered table-striped table-condensed\"><thead><tr><th>Module</th><th>Services</th>" +
		"<th>Data types</th><th>Constants</th></tr></thead><tbody>\n")
	g.generateProgramTOCRow(g.program)
	g.write("</tbody></table>\n")
}

// generateProgramTOCRows is generate_program_toc_rows: recurses through
// from the provided program and generates a ToC row for each discovered
// program exactly once by maintaining the list of completed rows in
// finished.
func (g *generator) generateProgramTOCRows(tprog *sema.Program, finished *[]*sema.Program) {
	for _, p := range *finished {
		if tprog.Path() == p.Path() {
			return
		}
	}
	*finished = append(*finished, tprog)
	g.generateProgramTOCRow(tprog)
	for _, inc := range tprog.Includes() {
		g.generateProgramTOCRows(inc, finished)
	}
}

// generateProgramTOCRow is generate_program_toc_row: emits the Table of
// Contents links at the top of the module's page.
func (g *generator) generateProgramTOCRow(tprog *sema.Program) {
	fname := tprog.Name() + ".html"
	g.write("<tr>\n<td>" + tprog.Name() + "</td><td>")
	if len(tprog.Services()) != 0 {
		for _, sv := range tprog.Services() {
			name := sv.Name()
			g.write("<a href=\"" + g.makeFileLink(fname) + "#Svc_" + name + "\">" + name + "</a><br/>\n")
			g.write("<ul>\n")
			fnHTML := map[string]string{}
			for _, fn := range sv.Functions() {
				fnName := fn.Name()
				insertIfAbsent(fnHTML, fnName, "<li><a href=\""+g.makeFileLink(fname)+"#Fn_"+name+"_"+fnName+
					"\">"+fnName+"</a></li>")
			}
			for _, k := range sortedKeys(fnHTML) {
				g.write(fnHTML[k] + "\n")
			}
			g.write("</ul>\n")
		}
	}
	g.write("</td>\n<td>")
	dataTypes := map[string]string{}
	for _, en := range tprog.Enums() {
		name := en.Name()
		insertIfAbsent(dataTypes, name, "<a href=\""+g.makeFileLink(fname)+"#Enum_"+name+"\">"+name+"</a>")
	}
	for _, td := range tprog.Typedefs() {
		name := td.Symbolic()
		insertIfAbsent(dataTypes, name, "<a href=\""+g.makeFileLink(fname)+"#Typedef_"+name+"\">"+name+"</a>")
	}
	for _, o := range tprog.Objects() {
		name := o.Name()
		insertIfAbsent(dataTypes, name, "<a href=\""+g.makeFileLink(fname)+"#Struct_"+name+"\">"+name+"</a>")
	}
	for _, k := range sortedKeys(dataTypes) {
		g.write(dataTypes[k] + "<br/>\n")
	}
	g.write("</td>\n<td>")
	if len(tprog.Consts()) != 0 {
		constHTML := map[string]string{}
		for _, c := range tprog.Consts() {
			name := c.Name()
			insertIfAbsent(constHTML, name, "<code><a href=\""+g.makeFileLink(fname)+"#Const_"+name+"\">"+name+"</a></code>")
		}
		for _, k := range sortedKeys(constHTML) {
			g.write(constHTML[k] + "<br/>\n")
		}
	}
	g.write("</td>\n</tr>")
}

// generateIndex is generate_index: emits the index.html file for the
// recursive set of Thrift programs.
func (g *generator) generateIndex() {
	g.currentFile = "index.html"
	g.sb = &strings.Builder{}
	g.write("<!DOCTYPE html>\n<html lang=\"en\"><head>\n")
	g.generateStyleTag()
	g.write("<title>All Thrift declarations</title></head><body>\n")
	g.write("<div class=\"container-fluid\">\n<h1>All Thrift declarations</h1>\n")
	g.write("<table class=\"table-bordered table-striped table-condensed\"><thead><tr><th>Module</th><th>Services</th><th>Data types</th>" +
		"<th>Constants</th></tr></thead><tbody>\n")
	var programs []*sema.Program
	g.generateProgramTOCRows(g.program, &programs)
	g.write("</tbody></table>\n")
	g.write("</div></body></html>\n")
	emit.WriteFile(g.outDir+g.currentFile, g.sb.String())
}

// generateCSS is generate_css.
func (g *generator) generateCSS() {
	if !g.opts.Standalone {
		g.currentFile = "style.css"
		var sb strings.Builder
		g.generateCSSContent(&sb)
		emit.WriteFile(g.outDir+g.currentFile, sb.String())
	}
}

// generateCSSContent is generate_css_content.
func (g *generator) generateCSSContent(target *strings.Builder) {
	target.WriteString(bootstrapCSS + "\n")
	target.WriteString("/* Auto-generated CSS for generated Thrift docs */\n")
	target.WriteString("h3, h4 { margin-bottom: 6px; }\n")
	target.WriteString("div.definition { border: 1px solid #CCC; margin-bottom: 10px; padding: 10px; }\n")
	target.WriteString("div.extends { margin: -0.5em 0 1em 5em }\n")
	target.WriteString("td { vertical-align: top; }\n")
	target.WriteString("table { empty-cells: show; }\n")
	target.WriteString("code { line-height: 20px; }\n")
	target.WriteString(".table-bordered th, .table-bordered td { border-bottom: 1px solid #DDDDDD; }\n")
}

// generateStyleTag is generate_style_tag: depending on "standalone",
// either a CSS file link (default), or the entire CSS is embedded inline.
func (g *generator) generateStyleTag() {
	if !g.opts.Standalone {
		g.write("<link href=\"style.css\" rel=\"stylesheet\" type=\"text/css\"/>\n")
	} else {
		g.write("<style type=\"text/css\"/><!--\n")
		var sb strings.Builder
		g.generateCSSContent(&sb)
		g.write(sb.String())
		g.write("--></style>\n")
	}
}

// makeFileLink is make_file_link: returns the target file for a <a href>
// link. The returned string is empty whenever filename refers to the
// current file.
func (g *generator) makeFileLink(filename string) string {
	if g.currentFile != filename {
		return filename
	}
	return ""
}

// printDoc is print_doc: if the provided documentable object has
// documentation attached, this will emit it to the output stream in HTML
// format.
func (g *generator) printDoc(tdoc docHolder) {
	if tdoc.HasDoc() {
		if g.opts.NoEscape {
			g.write(tdoc.Doc() + "<br/>")
		} else {
			g.write("<pre>" + g.escapeHTML(tdoc.Doc()) + "</pre><br/>")
		}
	}
}

// isUTF8Sequence is is_utf8_sequence.
func isUTF8Sequence(str string, firstpos int) bool {
	c := str[firstpos]
	count := 0
	switch {
	case c&0xE0 == 0xC0:
		count = 1
	case c&0xF0 == 0xE0:
		count = 2
	case c&0xF8 == 0xF0:
		count = 3
	case c&0xFC == 0xF8:
		count = 4
	case c&0xFE == 0xFC:
		count = 5
	default:
		return false // no UTF-8
	}

	pos := firstpos + 1
	for pos < len(str) && count > 0 {
		c = str[pos]
		if c&0xC0 != 0x80 {
			return false // no UTF-8
		}
		count--
		pos++
	}

	return count == 0
}

// detectInputEncoding is detect_input_encoding.
func (g *generator) detectInputEncoding(str string, firstpos int) {
	if isUTF8Sequence(str, firstpos) {
		g.inputType = inputUTF8
		return
	}
	// fallback
	g.inputType = inputPlain
}

// escapeHTMLTags is escape_html_tags.
func (g *generator) escapeHTMLTags(str string) string {
	var result strings.Builder

	var c byte = '?'
	firstpos := 0
	for firstpos < len(str) {
		// look for non-ASCII char
		lastpos := firstpos
		for lastpos < len(str) {
			c = str[lastpos]
			if c == '<' || c == '>' {
				break
			}
			lastpos++
		}

		// copy what we got so far
		if lastpos > firstpos {
			result.WriteString(str[firstpos:lastpos])
			firstpos = lastpos
		}

		// reached the end?
		if firstpos >= len(str) {
			break
		}

		// tag end without corresponding begin
		firstpos++
		if c == '>' {
			result.WriteString("&gt;")
			continue
		}

		// extract the tag
		var tagstream strings.Builder
		for firstpos < len(str) {
			c = str[firstpos]
			firstpos++
			if c == '<' {
				tagstream.WriteString("&lt;") // nested begin?
			} else if c == '>' {
				break
			} else {
				tagstream.WriteByte(c) // not very efficient, but tags should be quite short
			}
		}

		// we allow for several markup in docstrings, all else will become escaped
		tagContent := tagstream.String()
		tagKey := tagContent
		if firstWhite := strings.IndexAny(tagKey, " \t\f\v\n\r"); firstWhite >= 0 {
			tagKey = tagKey[:firstWhite]
		}
		tagKey = strings.ToLower(tagKey)
		if g.allowedMarkup[tagKey] {
			result.WriteString("<" + tagContent + ">")
		} else {
			result.WriteString("&lt;" + tagContent + "&gt;")
		}
	}

	return result.String()
}

// escapeHTML is escape_html.
func (g *generator) escapeHTML(str string) string {
	// the generated HTML header says it is UTF-8 encoded
	// if UTF-8 input has been detected before, we don't need to change anything
	if g.inputType == inputUTF8 {
		return g.escapeHTMLTags(str)
	}

	// convert unsafe chars to their &#<num>; equivalent
	var result strings.Builder
	var c byte = '?'
	firstpos := 0
	for firstpos < len(str) {
		// look for non-ASCII char
		lastpos := firstpos
		var ic uint
		for lastpos < len(str) {
			c = str[lastpos]
			ic = uint(c)
			if ic < 32 || ic > 127 {
				break
			}
			lastpos++
		}

		// copy what we got so far
		if lastpos > firstpos {
			result.WriteString(str[firstpos:lastpos])
			firstpos = lastpos
		}

		// reached the end?
		if firstpos >= len(str) {
			break
		}

		// some control code?
		if ic <= 31 {
			switch c {
			case '\r', '\n', '\t':
				result.WriteByte(c)
			default: // silently consume all other ctrl chars
			}
			firstpos++
			continue
		}

		// reached the end?
		if firstpos >= len(str) {
			break
		}

		// try to detect input encoding
		if g.inputType == inputUnknown {
			g.detectInputEncoding(str, firstpos)
			if g.inputType == inputUTF8 {
				result.WriteString(str[firstpos:])
				break
			}
		}

		// convert the character to something useful based on the detected encoding
		switch g.inputType {
		case inputPlain:
			result.WriteString("&#" + strconv.FormatUint(uint64(ic), 10) + ";")
			firstpos++
		default:
			emit.Throw("Unexpected or unrecognized input encoding")
		}
	}

	return g.escapeHTMLTags(result.String())
}

// printType is print_type: prints out the provided type in HTML.
func (g *generator) printType(ttype sema.Type) int {
	var length int
	g.write("<code>")
	switch {
	case ttype.IsContainer():
		switch {
		case ttype.IsList():
			g.write("list&lt;")
			length = 6 + g.printType(ttype.(*sema.List).ElemType())
			g.write("&gt;")
		case ttype.IsSet():
			g.write("set&lt;")
			length = 5 + g.printType(ttype.(*sema.Set).ElemType())
			g.write("&gt;")
		case ttype.IsMap():
			g.write("map&lt;")
			m := ttype.(*sema.Map)
			length = 5 + g.printType(m.KeyType())
			g.write(", ")
			length += g.printType(m.ValType())
			g.write("&gt;")
		}
	case ttype.IsBaseType():
		if ttype.IsBinary() {
			g.write("binary")
		} else {
			g.write(ttype.Name())
		}
		length = len(ttype.Name())
	default:
		progName := ttype.Program().Name()
		typeName := ttype.Name()
		g.write("<a href=\"" + g.makeFileLink(progName+".html") + "#")
		switch {
		case ttype.IsTypedef():
			g.write("Typedef_")
		case ttype.IsStruct() || ttype.IsXception():
			g.write("Struct_")
		case ttype.IsEnum():
			g.write("Enum_")
		case ttype.IsService():
			g.write("Svc_")
		}
		g.write(typeName + "\">")
		length = len(typeName)
		if ttype.Program() != g.program {
			g.write(progName + ".")
			length += len(progName) + 1
		}
		g.write(typeName + "</a>")
	}
	g.write("</code>")
	return length
}

// getEscapedString is get_escaped_string: escape_string(constval->get_string())
// with the html generator's own escape map (& < > " ').
func getEscapedString(tvalue *sema.ConstValue) string {
	in := tvalue.String()
	var sb strings.Builder
	for i := 0; i < len(in); i++ {
		switch c := in[i]; c {
		case '&':
			sb.WriteString("&amp;")
		case '<':
			sb.WriteString("&lt;")
		case '>':
			sb.WriteString("&gt;")
		case '"':
			sb.WriteString("&quot;")
		case '\'':
			sb.WriteString("&apos;")
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

// formatDouble mimics `ostream << double` with the default precision of
// six significant digits, as golang.formatDouble does.
func formatDouble(f float64) string {
	return strconv.FormatFloat(f, 'g', 6, 64)
}

// printConstValue is print_const_value: prints out an HTML representation
// of the provided constant value.
func (g *generator) printConstValue(typ sema.Type, tvalue *sema.ConstValue) {
	// if tvalue is an identifier, the constant content is already shown elsewhere
	if tvalue.Kind() == sema.CVIdentifier {
		fname := g.program.Name() + ".html"
		name := g.escapeHTML(tvalue.Identifier())
		g.write("<code><a href=\"" + g.makeFileLink(fname) + "#Const_" + name + "\">" + name + "</a></code>")
		return
	}

	truetype := sema.TrueType(typ)

	first := true
	switch {
	case truetype.IsBaseType():
		base := truetype.(*sema.BaseType)
		switch base.Base() {
		case sema.TypeString:
			g.write("\"" + g.escapeHTML(getEscapedString(tvalue)) + "\"")
		case sema.TypeBool:
			if tvalue.Integer() != 0 {
				g.write("true")
			} else {
				g.write("false")
			}
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			g.write(strconv.FormatInt(tvalue.Integer(), 10))
		case sema.TypeDouble:
			if tvalue.Kind() == sema.CVInteger {
				g.write(strconv.FormatInt(tvalue.Integer(), 10))
			} else {
				g.write(formatDouble(tvalue.Double()))
			}
		default:
			g.write("UNKNOWN BASE TYPE")
		}
	case truetype.IsEnum():
		g.write(g.escapeHTML(truetype.Name()) + "." + g.escapeHTML(tvalue.IdentifierName()))
	case truetype.IsStruct() || truetype.IsXception():
		g.write("{ ")
		fields := truetype.(*sema.Struct).Members()
		for _, e := range tvalue.Map() {
			var fieldType sema.Type
			for _, f := range fields {
				if f.Name() == e.Key.String() {
					fieldType = f.Type()
				}
			}
			if fieldType == nil {
				emit.Throw("type error: %s has no field %s", truetype.Name(), e.Key.String())
			}
			if !first {
				g.write(", ")
			}
			first = false
			g.write(g.escapeHTML(e.Key.String()) + " = ")
			g.printConstValue(fieldType, e.Value)
		}
		g.write(" }")
	case truetype.IsMap():
		g.write("{ ")
		m := truetype.(*sema.Map)
		for _, e := range tvalue.Map() {
			if !first {
				g.write(", ")
			}
			first = false
			g.printConstValue(m.KeyType(), e.Key)
			g.write(" = ")
			g.printConstValue(m.ValType(), e.Value)
		}
		g.write(" }")
	case truetype.IsList():
		g.write("{ ")
		l := truetype.(*sema.List)
		for _, e := range tvalue.List() {
			if !first {
				g.write(", ")
			}
			first = false
			g.printConstValue(l.ElemType(), e)
		}
		g.write(" }")
	case truetype.IsSet():
		g.write("{ ")
		s := truetype.(*sema.Set)
		for _, e := range tvalue.List() {
			if !first {
				g.write(", ")
			}
			first = false
			g.printConstValue(s.ElemType(), e)
		}
		g.write(" }")
	default:
		g.write("UNKNOWN TYPE")
	}
}

// printFnArgsDoc is print_fn_args_doc: prints out documentation for
// arguments/exceptions of a function, if any documentation has been
// supplied.
func (g *generator) printFnArgsDoc(tfunction *sema.Function) {
	args := tfunction.Arglist().Members()
	if len(args) != 0 {
		hasDocs := false
		for _, a := range args {
			if a.HasDoc() && a.Doc() != "" {
				hasDocs = true
			}
		}
		if hasDocs {
			g.write("<br/><h4 id=\"Parameters_" + g.serviceName + "_" + tfunction.Name() + "\">Parameters</h4>\n")
			g.write("<table class=\"table-bordered table-striped table-condensed\">")
			g.write("<thead><tr><th>Name</th><th>Description</th></tr></thead><tbody>")
			for _, a := range args {
				g.write("<tr><td>" + a.Name())
				g.write("</td><td>")
				g.write(g.escapeHTML(a.Doc()))
				g.write("</td></tr>\n")
			}
			g.write("</tbody></table>")
		}
	}

	excepts := tfunction.Xceptions().Members()
	if len(excepts) != 0 {
		hasDocs := false
		for _, x := range excepts {
			if x.HasDoc() && x.Doc() != "" {
				hasDocs = true
			}
		}
		if hasDocs {
			g.write("<br/><h4 id=\"Exceptions_" + g.serviceName + "_" + tfunction.Name() + "\">Exceptions</h4>\n")
			g.write("<table class=\"table-bordered table-striped table-condensed\">")
			g.write("<thead><tr><th>Type</th><th>Description</th></tr></thead><tbody>")
			for _, x := range excepts {
				g.write("<tr><td>" + x.Type().Name())
				g.write("</td><td>")
				g.write(g.escapeHTML(x.Doc()))
				g.write("</td></tr>\n")
			}
			g.write("</tbody></table>")
		}
	}
}

// generateTypedef is generate_typedef.
func (g *generator) generateTypedef(ttypedef *sema.Typedef) {
	name := ttypedef.Name()
	g.write("<div class=\"definition\">")
	g.write("<h3 id=\"Typedef_" + name + "\">Typedef: " + name + "</h3>\n")
	g.write("<p><strong>Base type:</strong>&nbsp;")
	g.printType(ttypedef.Type())
	g.write("</p>\n")
	g.printDoc(ttypedef)
	g.write("</div>\n")
}

// generateEnum is generate_enum: generates code for an enumerated type.
func (g *generator) generateEnum(tenum *sema.Enum) {
	name := tenum.Name()
	g.write("<div class=\"definition\">")
	g.write("<h3 id=\"Enum_" + name + "\">Enumeration: " + name + "</h3>\n")
	g.printDoc(tenum)
	g.write("<br/><table class=\"table-bordered table-striped table-condensed\">\n")
	for _, v := range tenum.Constants() {
		g.write("<tr><td><code>")
		g.write(v.Name())
		g.write("</code></td><td><code>")
		g.write(strconv.FormatInt(int64(v.Value()), 10))
		g.write("</code></td><td>\n")
		g.printDoc(v)
		g.write("</td></tr>\n")
	}
	g.write("</table></div>\n")
}

// generateConst is generate_const: generates a constant value.
func (g *generator) generateConst(tconst *sema.Const) {
	name := tconst.Name()
	g.write("<tr id=\"Const_" + name + "\"><td><code>" + name + "</code></td><td>")
	g.printType(tconst.Type())
	g.write("</td><td><code>")
	g.printConstValue(tconst.Type(), tconst.Value())
	g.write("</code></td></tr>")
	if tconst.HasDoc() {
		g.write("<tr><td colspan=\"3\"><blockquote>")
		g.printDoc(tconst)
		g.write("</blockquote></td></tr>")
	}
}

// generateStruct is generate_struct: generates a struct definition for a
// thrift data type.
func (g *generator) generateStruct(tstruct *sema.Struct) {
	name := tstruct.Name()
	g.write("<div class=\"definition\">")
	g.write("<h3 id=\"Struct_" + name + "\">")
	switch {
	case tstruct.IsXception():
		g.write("Exception: ")
	case tstruct.IsUnion():
		g.write("Union: ")
	default:
		g.write("Struct: ")
	}
	g.write(name + "</h3>\n")
	g.write("<table class=\"table-bordered table-striped table-condensed\">")
	g.write("<thead><tr><th>Key</th><th>Field</th><th>Type</th><th>Description</th><th>Requiredness</th><th>Default value</th></tr></thead><tbody>\n")
	for _, m := range tstruct.Members() {
		g.write("<tr><td>" + strconv.FormatInt(int64(m.Key()), 10) + "</td><td>")
		g.write(m.Name())
		g.write("</td><td>")
		g.printType(m.Type())
		g.write("</td><td>")
		g.write(g.escapeHTML(m.Doc()))
		g.write("</td><td>")
		switch m.Req() {
		case sema.Optional:
			g.write("optional")
		case sema.Required:
			g.write("required")
		default:
			g.write("default")
		}
		g.write("</td><td>")
		defaultVal := m.Value()
		if defaultVal != nil {
			g.write("<code>")
			g.printConstValue(m.Type(), defaultVal)
			g.write("</code>")
		}
		g.write("</td></tr>\n")
	}
	g.write("</tbody></table><br/>")
	g.printDoc(tstruct)
	g.write("</div>")
}

// generateXception is generate_xception: exceptions are special structs.
func (g *generator) generateXception(txception *sema.Struct) {
	g.generateStruct(txception)
}

// generateService is generate_service: generates the HTML block for a
// Thrift service.
func (g *generator) generateService(tservice *sema.Service) {
	g.write("<h3 id=\"Svc_" + g.serviceName + "\">Service: " + g.serviceName + "</h3>\n")

	if tservice.Extends() != nil {
		g.write("<div class=\"extends\"><em>extends</em> ")
		g.printType(tservice.Extends())
		g.write("</div>\n")
	}
	g.printDoc(tservice)
	for _, fn := range tservice.Functions() {
		fnName := fn.Name()
		g.write("<div class=\"definition\">")
		g.write("<h4 id=\"Fn_" + g.serviceName + "_" + fnName + "\">Function: " + g.serviceName + "." + fnName + "</h4>\n")
		g.write("<pre>")
		offset := g.printType(fn.ReturnType())
		first := true
		g.write(" " + fnName + "(")
		offset += len(fnName) + 2
		args := fn.Arglist().Members()
		for _, a := range args {
			if !first {
				g.write(",\n")
				g.write(strings.Repeat(" ", offset))
			}
			first = false
			g.printType(a.Type())
			g.write(" " + a.Name())
			if a.Value() != nil {
				g.write(" = ")
				g.printConstValue(a.Type(), a.Value())
			}
		}
		g.write(")\n")
		first = true
		excepts := fn.Xceptions().Members()
		if len(excepts) != 0 {
			g.write("    throws ")
			for _, x := range excepts {
				if !first {
					g.write(", ")
				}
				first = false
				g.printType(x.Type())
			}
			g.write("\n")
		}
		g.write("</pre>")
		g.printDoc(fn)
		g.printFnArgsDoc(fn)
		g.write("</div>")
	}
}

// insertIfAbsent is std::map::insert's pair form: it stores the value only
// when the key is not already present, keeping the first entry as the
// C++ compiler's tables do.
func insertIfAbsent(m map[string]string, key, value string) {
	if _, ok := m[key]; !ok {
		m[key] = value
	}
}

// sortedKeys returns the keys of a string-keyed map in sorted order, the
// iteration order of the std::map<string, string> the C++ compiler keeps
// for these tables.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
