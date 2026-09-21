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

// Package markdown is t_markdown_generator.cc: it renders a resolved
// program as the Markdown documentation the C++ compiler's --gen markdown
// writes, byte for byte. It is mostly copy/pasting/tweaking from the HTML
// generator's work, as the C++ source comment says.
package markdown

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// Options are the markdown:... generator options.
type Options struct {
	// NoEscape is "noescape": doc text is written unescaped instead of
	// html-entity escaped.
	NoEscape bool
	// Suffix is the "suffix" option's value: the file extension without
	// its leading dot, or "" for no extension. It only applies when
	// HasSuffix is set; otherwise the default extension (md) is used.
	Suffix    string
	HasSuffix bool
}

// ParseOptions parses the part after "markdown:" of a --gen argument, with
// the same splitting rules as t_generator::parse_options: options are
// comma separated and a value follows the first "=".
func ParseOptions(spec string) (Options, error) {
	var o Options
	for _, option := range strings.Split(spec, ",") {
		key, value := option, ""
		if i := strings.IndexByte(option, '='); i >= 0 {
			key, value = option[:i], option[i+1:]
		}
		switch key {
		case "":
		case "noescape":
			o.NoEscape = true
		case "suffix":
			o.Suffix = value
			o.HasSuffix = true
		default:
			return o, &emit.Error{Msg: "unknown option markdown:" + key}
		}
	}
	return o, nil
}

func init() {
	generate.Register(generate.Info{
		Name:     "markdown",
		LongName: "Markdown",
		Options: []generate.Option{
			{Name: "suffix", Value: "<ext>", Help: "Override default file extension (default: md). Use\nsuffix= (empty) for no extension."},
			{Name: "noescape", Help: "Do not escape with html-entities in doc text."},
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

// inputEncoding is the input_type enum.
type inputEncoding int

const (
	inputUnknown inputEncoding = iota
	inputUTF8
	inputPlain
)

// docNode is t_doc*: anything with a doc comment.
type docNode interface {
	HasDoc() bool
	Doc() string
}

// generator is t_markdown_generator. A fresh instance generates exactly
// one program, mirroring the C++ compiler creating a new generator
// instance per program under -r.
type generator struct {
	sb            strings.Builder
	program       *sema.Program
	currentFile   string
	outDir        string
	inputType     inputEncoding
	allowedMarkup map[string]bool
	unsafe        bool
	extension     string
}

func newGenerator(program *sema.Program, opts Options) *generator {
	g := &generator{
		program:   program,
		unsafe:    opts.NoEscape,
		extension: ".md",
		inputType: inputUnknown,
	}
	if opts.HasSuffix {
		if opts.Suffix == "" {
			g.extension = ""
		} else {
			g.extension = "." + opts.Suffix
		}
	}
	g.initAllowedMarkup()
	return g
}

// strToID is str_to_id: string to markdown-id link reference.
func strToID(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '.' {
			sb.WriteByte(asciiToLower(c))
		}
	}
	return sb.String()
}

func asciiToLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c - 'A' + 'a'
	}
	return c
}

func isSpaceByte(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}

func isAlphaByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// formatDouble mimics `ostream << double` with the default precision of
// six significant digits.
func formatDouble(f float64) string {
	return strconv.FormatFloat(f, 'g', 6, 64)
}

// markdownEscapeString is get_escaped_string(tvalue): escape_string using
// this generator's own escape_ table (html entities), which overrides the
// default backslash-escape table t_generator otherwise uses.
func markdownEscapeString(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
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
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
}

// generateProgramTOC emits the Table of Contents links at the top of the
// module's page.
func (g *generator) generateProgramTOC() {
	g.sb.WriteString("| Module | Services & Functions | Data types | Constants |\n")
	g.sb.WriteString("| --- | --- | --- | --- |\n")
	g.generateProgramTOCRow(g.program)
	g.sb.WriteString("\n")
}

// generateProgramTOCRows recurses through from the provided program and
// generates a ToC row for each discovered program exactly once by
// maintaining the list of completed rows in finished.
func (g *generator) generateProgramTOCRows(tprog *sema.Program, finished *[]*sema.Program) {
	for _, p := range *finished {
		if tprog.Path() == p.Path() {
			return
		}
	}
	*finished = append(*finished, tprog)
	g.generateProgramTOCRow(tprog)
	for _, include := range tprog.Includes() {
		g.generateProgramTOCRows(include, finished)
	}
}

// tocRow is "| Module | Services | Data types | Constants |".
type tocRow [4]string

// generateProgramTOCRow emits the Table of Contents links at the top of
// the module's page.
func (g *generator) generateProgramTOCRow(tprog *sema.Program) {
	var filling []tocRow

	fname := g.makeFileName(tprog.Name())

	filling = append(filling, tocRow{})
	filling[0][0] = tprog.Name()

	if services := tprog.Services(); len(services) > 0 {
		for i, sv := range services {
			if i != 0 {
				filling = append(filling, tocRow{})
			}
			idx := len(filling) - 1
			name := sv.Name()
			filling[idx][1] = "[" + name + "](" + g.makeFileLink(fname) + "#service-" + strToID(name) + ")"

			for _, fn := range sv.Functions() {
				filling = append(filling, tocRow{})
				idx = len(filling) - 1
				fnName := fn.Name()
				filling[idx][1] = "    [ &bull; " + fnName + "](" + g.makeFileLink(fname) + "#function-" + strToID(name+fnName) + ")"
			}
		}
	}

	// Data Types Column
	itFill := 0
	setCol := func(col int, value string) {
		var idx int
		if itFill == len(filling) {
			filling = append(filling, tocRow{})
			idx = len(filling) - 1
			itFill = len(filling)
		} else {
			idx = itFill
			itFill++
		}
		filling[idx][col] = value
	}

	if enums := tprog.Enums(); len(enums) > 0 {
		for _, en := range enums {
			name := en.Name()
			setCol(2, "["+name+"]("+g.makeFileLink(fname)+"#enumeration-"+strToID(name)+")")
		}
	}
	if typedefs := tprog.Typedefs(); len(typedefs) > 0 {
		for _, td := range typedefs {
			name := td.Symbolic()
			setCol(2, "["+name+"]("+g.makeFileLink(fname)+"#typedef-"+strToID(name)+")")
		}
	}
	if objects := tprog.Objects(); len(objects) > 0 {
		for _, o := range objects {
			name := o.Name()
			link := "[" + name + "](" + g.makeFileLink(fname)
			if o.IsXception() {
				link += "#exception-"
			} else if o.IsStruct() && o.IsUnion() {
				link += "#union-"
			} else {
				link += "#struct-"
			}
			link += strToID(name) + ")"
			setCol(2, link)
		}
	}

	// Constants Column
	itFill = 0

	if consts := tprog.Consts(); len(consts) > 0 {
		for _, c := range consts {
			name := c.Name()
			setCol(3, "["+name+"]("+g.makeFileLink(fname)+"#constant-"+strToID(name)+")")
		}
	}

	for _, row := range filling {
		for _, c := range row {
			g.sb.WriteString("|" + c)
		}
		g.sb.WriteString("|\n")
	}
}

// generateProgram prepares for file generation by opening up the
// necessary file output stream.
func (g *generator) generateProgram() {
	outDir := g.program.OutPath() + "gen-markdown/"
	if g.program.IsOutPathAbsolute() {
		outDir = g.program.OutPath() + "/"
	}
	emit.Mkdir(outDir)
	g.outDir = outDir

	pname := g.program.Name()
	g.currentFile = g.makeFileName(pname)
	fname := outDir + g.currentFile

	g.sb.Reset()
	g.sb.WriteString("# Thrift module: " + pname + "\n\n")

	g.printDoc(g.program)
	g.sb.WriteString("\n\n")

	g.generateProgramTOC()

	if consts := g.program.Consts(); len(consts) > 0 {
		g.sb.WriteString("***\n## Constants\n\n")
		g.sb.WriteString("|Constant|Type|Value||\n")
		g.sb.WriteString("|---|---|---|---|\n")
		for _, c := range consts {
			g.generateConst(c)
		}
		g.sb.WriteString("\n")
	}

	if enums := g.program.Enums(); len(enums) > 0 {
		g.sb.WriteString("***\n## Enumerations\n\n")
		for _, e := range enums {
			g.generateEnum(e)
		}
	}

	if typedefs := g.program.Typedefs(); len(typedefs) > 0 {
		g.sb.WriteString("***\n## Type declarations\n\n")
		for _, t := range typedefs {
			g.generateTypedef(t)
		}
	}

	if objects := g.program.Objects(); len(objects) > 0 {
		g.sb.WriteString("***\n## Data structures\n\n")
		for _, o := range objects {
			if o.IsXception() {
				g.generateXception(o)
			} else {
				g.generateStruct(o)
			}
		}
	}

	if services := g.program.Services(); len(services) > 0 {
		g.sb.WriteString("***\n## Services\n\n")
		for _, s := range services {
			g.generateService(s, s.Name())
		}
	}

	g.sb.WriteString("\n")
	emit.WriteFile(fname, g.sb.String())

	g.generateIndex()
}

// generateIndex emits the index(.ext) file for the recursive set of
// Thrift programs.
func (g *generator) generateIndex() {
	g.currentFile = g.makeFileName("index")
	indexFname := g.outDir + g.currentFile

	g.sb.Reset()
	g.sb.WriteString("# Thrift declarations\n")
	g.sb.WriteString("| Module | Services & Functions | Data types | Constants |\n")
	g.sb.WriteString("| --- | --- | --- | --- |\n")
	var programs []*sema.Program
	g.generateProgramTOCRows(g.program, &programs)
	g.sb.WriteString("\n")
	emit.WriteFile(indexFname, g.sb.String())
}

// makeFileLink returns the target file for a link. The returned string is
// empty whenever filename refers to the current file.
func (g *generator) makeFileLink(filename string) string {
	if g.currentFile != filename {
		return filename
	}
	return ""
}

// makeFileName returns the file name with the configured extension.
func (g *generator) makeFileName(filename string) string {
	if g.extension == "" {
		return filename
	}
	return filename + g.extension
}

// printDoc emits the documentable object's doc comment, if any, in HTML
// format unless noescape was given.
func (g *generator) printDoc(tdoc docNode) {
	if tdoc.HasDoc() {
		if g.unsafe {
			g.sb.WriteString(tdoc.Doc())
		} else {
			g.sb.WriteString(g.escapeHTML(tdoc.Doc()))
		}
	}
}

// escapeMDTableCell is escape_md_table_cell.
func escapeMDTableCell(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '|':
			sb.WriteString(`\|`)
		case '\r', '\n':
			sb.WriteByte(' ')
		default:
			sb.WriteByte(c)
		}
	}
	result := sb.String()
	for len(result) > 0 && result[len(result)-1] == ' ' {
		result = result[:len(result)-1]
	}
	return result
}

// printDocWithAtParams prints function documentation, rendering @param
// and @return tags as a table. Tags are only recognised at the start of a
// line (after optional whitespace), so @-signs embedded in prose (e.g.
// email addresses) are left untouched. Text preceding the first tag line
// is emitted as normal prose.
func (g *generator) printDocWithAtParams(tdoc docNode) {
	if !tdoc.HasDoc() {
		return
	}
	raw := tdoc.Doc()

	// Split raw doc into lines (tolerate both \n and \r\n)
	var lines []string
	start := 0
	for i := 0; i <= len(raw); i++ {
		if i == len(raw) || raw[i] == '\n' {
			end := i
			if end > start && raw[end-1] == '\r' {
				end--
			}
			lines = append(lines, raw[start:end])
			start = i + 1
		}
	}

	// Returns true when a line is a recognised tag line (@param /
	// @return). On success, tagName and rest receive the tag keyword and
	// the remainder.
	parseTagLine := func(line string) (tagName, rest string, ok bool) {
		p := 0
		for p < len(line) && isSpaceByte(line[p]) {
			p++
		}
		if p >= len(line) || line[p] != '@' {
			return "", "", false
		}
		p++
		tn := p
		for p < len(line) && isAlphaByte(line[p]) {
			p++
		}
		tagName = line[tn:p]
		if tagName != "param" && tagName != "return" {
			return "", "", false
		}
		for p < len(line) && isSpaceByte(line[p]) {
			p++
		}
		rest = line[p:]
		return tagName, rest, true
	}

	// Bail out early if there are no tag lines at all
	hasTags := false
	for _, l := range lines {
		if _, _, ok := parseTagLine(l); ok {
			hasTags = true
			break
		}
	}
	if !hasTags {
		g.printDoc(tdoc)
		return
	}

	// Accumulate general prose and tagged entries
	var general strings.Builder
	type rawEntry struct{ tag, content string }
	var entries []rawEntry
	inGeneral := true

	for _, line := range lines {
		if tagName, rest, ok := parseTagLine(line); ok {
			inGeneral = false
			entries = append(entries, rawEntry{tagName, rest})
		} else if inGeneral {
			general.WriteString(line + "\n")
		} else if len(entries) > 0 {
			// Continuation line: trim and append to last entry's content
			ts := 0
			for ts < len(line) && isSpaceByte(line[ts]) {
				ts++
			}
			trimmed := line[ts:]
			for len(trimmed) > 0 && isSpaceByte(trimmed[len(trimmed)-1]) {
				trimmed = trimmed[:len(trimmed)-1]
			}
			if trimmed != "" {
				entries[len(entries)-1].content += " " + trimmed
			}
		}
	}

	// Trim trailing whitespace from general prose
	generalStr := general.String()
	for len(generalStr) > 0 && isSpaceByte(generalStr[len(generalStr)-1]) {
		generalStr = generalStr[:len(generalStr)-1]
	}

	// Split each entry's content at the first ' - ' / ' -<end>' separator
	type parsedEntry struct{ tag, signature, description string }
	var parsed []parsedEntry
	for _, e := range entries {
		content := e.content
		dashPos := -1
		for i := 0; i < len(content); i++ {
			if content[i] == '-' {
				beforeOK := i == 0 || isSpaceByte(content[i-1])
				afterOK := i+1 >= len(content) || isSpaceByte(content[i+1])
				if beforeOK && afterOK {
					dashPos = i
					break
				}
			}
		}
		var sig, desc string
		if dashPos >= 0 {
			sig = content[:dashPos]
			for len(sig) > 0 && isSpaceByte(sig[len(sig)-1]) {
				sig = sig[:len(sig)-1]
			}
			ds := dashPos + 1
			for ds < len(content) && isSpaceByte(content[ds]) {
				ds++
			}
			desc = content[ds:]
			for len(desc) > 0 && isSpaceByte(desc[len(desc)-1]) {
				desc = desc[:len(desc)-1]
			}
		} else {
			sig = content
			for len(sig) > 0 && isSpaceByte(sig[len(sig)-1]) {
				sig = sig[:len(sig)-1]
			}
		}
		parsed = append(parsed, parsedEntry{e.tag, sig, desc})
	}

	// Emit prose
	if generalStr != "" {
		if g.unsafe {
			g.sb.WriteString(generalStr)
		} else {
			g.sb.WriteString(g.escapeHTML(generalStr))
		}
		g.sb.WriteString("\n\n")
	}

	// Emit @param / @return as a Markdown table
	if len(parsed) > 0 {
		g.sb.WriteString("| Name | Description |\n")
		g.sb.WriteString("| --- | --- |\n")
		for _, p := range parsed {
			cellSig := escapeMDTableCell(p.signature)
			descSrc := p.description
			if !g.unsafe {
				descSrc = g.escapeHTML(p.description)
			}
			cellDesc := escapeMDTableCell(descSrc)
			if p.tag == "param" {
				g.sb.WriteString("| `" + cellSig + "` | " + cellDesc + " |\n")
			} else {
				retCol := "**Returns**"
				if cellSig != "" {
					retCol += " `" + cellSig + "`"
				}
				g.sb.WriteString("| " + retCol + " | " + cellDesc + " |\n")
			}
		}
		g.sb.WriteString("\n")
	}
}

// isUTF8Sequence: the leading char determines the length of the
// sequence.
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

	// following chars
	pos := firstpos + 1
	for pos < len(str) && count > 0 {
		c = str[pos]
		if c&0xC0 != 0x80 {
			return false // no UTF-8
		}
		count--
		pos++
	}

	// true if the sequence is complete
	return count == 0
}

func (g *generator) detectInputEncoding(str string, firstpos int) {
	if isUTF8Sequence(str, firstpos) {
		g.inputType = inputUTF8
		return
	}
	// fallback
	g.inputType = inputPlain
}

// initAllowedMarkup is init_allowed__markup.
func (g *generator) initAllowedMarkup() {
	g.allowedMarkup = map[string]bool{
		// standalone tags
		"br": true, "br/": true, "img": true,
		// paired tags
		"b": true, "/b": true,
		"u": true, "/u": true,
		"i": true, "/i": true,
		"s": true, "/s": true,
		"big": true, "/big": true,
		"small": true, "/small": true,
		"sup": true, "/sup": true,
		"sub": true, "/sub": true,
		"pre": true, "/pre": true,
		"tt": true, "/tt": true,
		"ul": true, "/ul": true,
		"ol": true, "/ol": true,
		"li": true, "/li": true,
		"a": true, "/a": true,
		"p": true, "/p": true,
		"code": true, "/code": true,
		"dl": true, "/dl": true,
		"dt": true, "/dt": true,
		"dd": true, "/dd": true,
		"h1": true, "/h1": true,
		"h2": true, "/h2": true,
		"h3": true, "/h3": true,
		"h4": true, "/h4": true,
		"h5": true, "/h5": true,
		"h6": true, "/h6": true,
	}
}

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
		if idx := strings.IndexAny(tagKey, " \t\f\v\n\r"); idx >= 0 {
			tagKey = tagKey[:idx]
		}
		tagKey = asciiLower(tagKey)
		if g.allowedMarkup[tagKey] {
			result.WriteString("<" + tagContent + ">")
		} else {
			result.WriteString("&lt;" + tagContent + "&gt;")
		}
	}

	return result.String()
}

func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		b[i] = asciiToLower(c)
	}
	return string(b)
}

func (g *generator) escapeHTML(str string) string {
	// the generated HTML header says it is UTF-8 encoded
	// if UTF-8 input has been detected before, we don't need to change anything
	// if (input_type_ == INPUT_UTF8) {
	//  return escape_html_tags(str);
	// }

	// convert unsafe chars to their &#<num>; equivalent
	var result strings.Builder
	var c byte = '?'
	ic := 0
	firstpos := 0
	for firstpos < len(str) {
		// look for non-ASCII char
		lastpos := firstpos
		for lastpos < len(str) {
			c = str[lastpos]
			ic = int(c)
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
				result.WriteString(" ")
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
			result.WriteString("&#" + strconv.Itoa(ic) + ";")
			firstpos++
		default:
			emit.Throw("Unexpected or unrecognized input encoding")
		}
	}

	return g.escapeHTMLTags(result.String())
}

// printType prints out the provided type in Markdown.
func (g *generator) printType(ttype sema.Type) int {
	length := 0
	switch {
	case ttype.IsContainer():
		switch {
		case ttype.IsList():
			g.sb.WriteString("list&lt;")
			length = 6 + g.printType(ttype.(*sema.List).ElemType())
			g.sb.WriteString("&gt;")
		case ttype.IsSet():
			g.sb.WriteString("set&lt;")
			length = 5 + g.printType(ttype.(*sema.Set).ElemType())
			g.sb.WriteString("&gt;")
		case ttype.IsMap():
			m := ttype.(*sema.Map)
			g.sb.WriteString("map&lt;")
			length = 5 + g.printType(m.KeyType())
			g.sb.WriteString(", ")
			length += g.printType(m.ValType())
			g.sb.WriteString("&gt;")
		}
	case ttype.IsBaseType():
		name := ttype.Name()
		display := name
		if ttype.IsBinary() {
			display = "binary"
		}
		g.sb.WriteString("```" + display + "```")
		length = len(name)
	default:
		progName := ttype.Program().Name()
		typeName := ttype.Name()
		g.sb.WriteString("[```" + typeName + "```](" + g.makeFileLink(g.makeFileName(progName)) + "#")
		switch {
		case ttype.IsTypedef():
			g.sb.WriteString("typedef-")
		case ttype.IsXception():
			g.sb.WriteString("exception-")
		case ttype.IsStruct():
			if ttype.(*sema.Struct).IsUnion() {
				g.sb.WriteString("union-")
			} else {
				g.sb.WriteString("struct-")
			}
		case ttype.IsEnum():
			g.sb.WriteString("enumeration-")
		case ttype.IsService():
			g.sb.WriteString("service-")
		}
		length = len(typeName)
		if ttype.Program() != g.program {
			g.sb.WriteString(strToID(progName))
			length += len(progName) + 1
		}
		g.sb.WriteString(strToID(typeName) + ")")
	}
	return length
}

// printConstValue prints out a Markdown representation of the provided
// constant value.
func (g *generator) printConstValue(typ sema.Type, tvalue *sema.ConstValue) {
	// if tvalue is an identifier, the constant content is already shown elsewhere
	if tvalue.Kind() == sema.CVIdentifier {
		fname := g.makeFileName(g.program.Name())
		name := g.escapeHTML(tvalue.Identifier())
		g.sb.WriteString("[```" + name + "```](" + g.makeFileLink(fname) + "#constant-" + strToID(name) + ")")
		return
	}

	truetype := sema.TrueType(typ)

	first := true
	switch {
	case truetype.IsBaseType():
		tbase := truetype.(*sema.BaseType).Base()
		g.sb.WriteString("```")
		switch tbase {
		case sema.TypeString:
			g.sb.WriteString(markdownEscapeString(tvalue.String()))
		case sema.TypeBool:
			if tvalue.Integer() != 0 {
				g.sb.WriteString("true")
			} else {
				g.sb.WriteString("false")
			}
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			g.sb.WriteString(strconv.FormatInt(tvalue.Integer(), 10))
		case sema.TypeDouble:
			if tvalue.Kind() == sema.CVInteger {
				g.sb.WriteString(strconv.FormatInt(tvalue.Integer(), 10))
			} else {
				g.sb.WriteString(formatDouble(tvalue.Double()))
			}
		default:
			g.sb.WriteString("UNKNOWN BASE TYPE")
		}
		g.sb.WriteString("```")
	case truetype.IsEnum():
		g.sb.WriteString(g.escapeHTML(truetype.Name()) + "." + g.escapeHTML(tvalue.IdentifierName()))
	case truetype.IsStruct() || truetype.IsXception():
		g.sb.WriteString("{ ")
		fields := truetype.(*sema.Struct).Members()
		for _, entry := range tvalue.Map() {
			var fieldType sema.Type
			for _, f := range fields {
				if f.Name() == entry.Key.String() {
					fieldType = f.Type()
				}
			}
			if fieldType == nil {
				emit.Throw("type error: %s has no field %s", truetype.Name(), entry.Key.String())
			}
			if !first {
				g.sb.WriteString(", ")
			}
			first = false
			g.sb.WriteString(g.escapeHTML(entry.Key.String()) + " = ")
			g.printConstValue(fieldType, entry.Value)
		}
		g.sb.WriteString(" }")
	case truetype.IsMap():
		g.sb.WriteString("{ ")
		m := truetype.(*sema.Map)
		for _, entry := range tvalue.Map() {
			if !first {
				g.sb.WriteString(", ")
			}
			first = false
			g.printConstValue(m.KeyType(), entry.Key)
			g.sb.WriteString(" = ")
			g.printConstValue(m.ValType(), entry.Value)
		}
		g.sb.WriteString(" }")
	case truetype.IsList():
		g.sb.WriteString("{ ")
		l := truetype.(*sema.List)
		for _, e := range tvalue.List() {
			if !first {
				g.sb.WriteString(", ")
			}
			first = false
			g.printConstValue(l.ElemType(), e)
		}
		g.sb.WriteString(" }")
	case truetype.IsSet():
		g.sb.WriteString("{ ")
		s := truetype.(*sema.Set)
		for _, e := range tvalue.List() {
			if !first {
				g.sb.WriteString(", ")
			}
			first = false
			g.printConstValue(s.ElemType(), e)
		}
		g.sb.WriteString(" }")
	default:
		g.sb.WriteString("UNKNOWN TYPE")
	}
}

// printFnArgsDoc prints out documentation for arguments/exceptions of a
// function, if any documentation has been supplied.
func (g *generator) printFnArgsDoc(tfunction *sema.Function) {
	hasDocs := false
	args := tfunction.Arglist().Members()
	for _, a := range args {
		if a.HasDoc() && a.Doc() != "" {
			hasDocs = true
			break
		}
	}
	if hasDocs {
		g.sb.WriteString("\n* parameters:\n")
		for n, a := range args {
			g.sb.WriteString(strconv.Itoa(n+1) + ". " + a.Name())
			g.sb.WriteString(" - " + g.escapeHTML(a.Doc()))
			g.sb.WriteString("\n")
		}
		g.sb.WriteString("\n")
	}
	if !hasDocs {
		g.sb.WriteString("\n")
	}

	hasDocs = false
	excepts := tfunction.Xceptions().Members()
	for _, x := range excepts {
		if x.HasDoc() && x.Doc() != "" {
			hasDocs = true
			break
		}
	}
	if hasDocs {
		g.sb.WriteString("* exceptions:\n")
		for _, x := range excepts {
			g.sb.WriteString("  * " + x.Type().Name())
			g.sb.WriteString(" - ")
			g.sb.WriteString(g.escapeHTML(x.Doc()))
			g.sb.WriteString("\n")
		}
		g.sb.WriteString("\n")
	}
}

// generateTypedef generates a typedef.
func (g *generator) generateTypedef(ttypedef *sema.Typedef) {
	name := ttypedef.Name()
	g.sb.WriteString("### Typedef: " + name + "\n")
	g.printDoc(ttypedef)
	g.sb.WriteString("\n\n")
	g.sb.WriteString("_Base type_: **")
	g.printType(ttypedef.Type())
	g.sb.WriteString("**\n\n")
	g.sb.WriteString("\n")
}

// generateEnum generates code for an enumerated type.
func (g *generator) generateEnum(tenum *sema.Enum) {
	name := tenum.Name()
	g.sb.WriteString("### Enumeration: " + name + "\n")
	g.printDoc(tenum)
	g.sb.WriteString("\n\n|Name|Value|Description|\n|---|---|---|\n")
	for _, v := range tenum.Constants() {
		g.sb.WriteString("|```")
		g.sb.WriteString(v.Name())
		g.sb.WriteString("```|```")
		g.sb.WriteString(strconv.Itoa(int(v.Value())))
		g.sb.WriteString("```|")
		g.printDoc(v)
		g.sb.WriteString("|\n")
	}
	g.sb.WriteString("\n")
}

// generateConst generates a constant value.
func (g *generator) generateConst(tconst *sema.Const) {
	// |Constant|Type|Value|HAS_DOC|
	name := tconst.Name()
	g.sb.WriteString("| ```" + name + "``` | ")
	g.printType(tconst.Type())
	g.sb.WriteString("| ```")
	g.printConstValue(tconst.Type(), tconst.Value())
	g.sb.WriteString("``` |")
	g.printDoc(tconst)
	g.sb.WriteString("|\n")
}

// generateStruct generates a struct definition for a thrift data type.
func (g *generator) generateStruct(tstruct *sema.Struct) {
	name := tstruct.Name()
	g.sb.WriteString("### ")
	switch {
	case tstruct.IsXception():
		g.sb.WriteString("Exception: ")
	case tstruct.IsUnion():
		g.sb.WriteString("Union: ")
	default:
		g.sb.WriteString("Struct: ")
	}
	g.sb.WriteString(name + "\n")
	g.printDoc(tstruct)
	g.sb.WriteString("\n\n")
	members := tstruct.Members()
	g.sb.WriteString("| Key | Field | Type | Description | Requiredness " +
		"| Default value |\n")
	g.sb.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	for _, f := range members {
		g.sb.WriteString("|" + strconv.Itoa(int(f.Key())))
		g.sb.WriteString("|" + f.Name())
		g.sb.WriteString("|")
		g.printType(f.Type())
		g.sb.WriteString("|" + g.escapeHTML(f.Doc()) + "|")
		switch f.Req() {
		case sema.Optional:
			g.sb.WriteString("optional")
		case sema.Required:
			g.sb.WriteString("required")
		default:
			g.sb.WriteString("default")
		}
		g.sb.WriteString("|")
		if defaultVal := f.Value(); defaultVal != nil {
			g.sb.WriteString("```")
			g.printConstValue(f.Type(), defaultVal)
			g.sb.WriteString("```")
		}
		g.sb.WriteString("|\n")
	}
	g.sb.WriteString("\n")
}

// generateXception: exceptions are special structs.
func (g *generator) generateXception(txception *sema.Struct) {
	g.generateStruct(txception)
}

// generateService generates the Markdown block for a Thrift service.
func (g *generator) generateService(tservice *sema.Service, serviceName string) {
	g.sb.WriteString("### Service: " + serviceName + "\n")

	if extends := tservice.Extends(); extends != nil {
		g.sb.WriteString("**extends ** _")
		g.printType(extends)
		g.sb.WriteString("_\n")
	}

	g.printDoc(tservice)
	g.sb.WriteString("\n")

	for _, fn := range tservice.Functions() {
		fnName := fn.Name()
		g.sb.WriteString("#### Function: " + serviceName + "." + fnName + "\n")
		g.printDocWithAtParams(fn)
		g.sb.WriteString("\n\n")
		g.printType(fn.ReturnType())
		first := true
		g.sb.WriteString("\n _" + fnName + "_(")
		args := fn.Arglist().Members()
		for _, a := range args {
			if !first {
				g.sb.WriteString(",\n")
			}
			first = false
			g.printType(a.Type())
			g.sb.WriteString(" " + a.Name())
			if a.Value() != nil {
				g.sb.WriteString(" = ")
				g.printConstValue(a.Type(), a.Value())
			}
		}
		g.sb.WriteString(")\n")
		first = true
		excepts := fn.Xceptions().Members()
		if len(excepts) > 0 {
			g.sb.WriteString("> throws ")
			for _, x := range excepts {
				if !first {
					g.sb.WriteString(", ")
				}
				first = false
				g.printType(x.Type())
			}
			g.sb.WriteString("\n")
		}
		g.printFnArgsDoc(fn)
		g.sb.WriteString("\n")
	}
}
