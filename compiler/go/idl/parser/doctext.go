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

package parser

import "strings"

// docStatus mirrors PROGDOCTEXT_STATUS in the C++ compiler. It decides
// whether the first doc comment of a file becomes the program's doc.
type docStatus int

const (
	docInvalid docStatus = iota
	docStillCandidate
	docAlreadyProcessed
	docAbsolutelySure
	docNoProgramDoctext
)

// docState is the doc-comment bookkeeping of one parse. It reproduces the
// globals g_doctext, g_doctext_lineno, g_program_doctext_candidate,
// g_program_doctext_lineno and g_program_doctext_status.
type docState struct {
	pending     string
	hasPending  bool
	lastDocLine int

	candidate     string
	hasCandidate  bool
	candidateLine int
	status        docStatus
}

// onDoc is the lexer action for a doc comment.
func (d *docState) onDoc(raw string, line int) {
	cleaned, ok := cleanUpDoctext(raw)
	d.pending, d.hasPending = cleaned, ok
	d.lastDocLine = line
	if ok && !d.hasCandidate && d.status == docInvalid {
		d.candidate, d.hasCandidate = cleaned, true
		d.candidateLine = line
		d.status = docStillCandidate
	}
}

// capture is the CaptureDocText grammar action.
func (d *docState) capture() (string, bool) {
	doc, ok := d.pending, d.hasPending
	d.pending, d.hasPending = "", false
	return doc, ok
}

// destroy is the DestroyDocText grammar action and clear_doctext.
func (d *docState) destroy() {
	d.pending, d.hasPending = "", false
}

// declareValid is declare_valid_program_doctext, run for every header.
func (d *docState) declareValid() {
	if d.hasCandidate && d.status == docStillCandidate {
		d.status = docAbsolutelySure
	} else {
		d.status = docNoProgramDoctext
	}
}

// setDoc is the side effect of t_doc::set_doc: a definition that consumes
// the doc comment on the candidate line takes it away from the program.
func (d *docState) setDoc() {
	if d.candidateLine == d.lastDocLine && d.status == docStillCandidate {
		d.status = docAlreadyProcessed
	}
}

// programDoc is the end-of-Program action.
func (d *docState) programDoc() (string, bool) {
	if d.hasCandidate && d.status != docAlreadyProcessed {
		return d.candidate, true
	}
	return "", false
}

func firstNotOf(s string, set string, from int) int {
	for i := from; i < len(s); i++ {
		if strings.IndexByte(set, s[i]) < 0 {
			return i
		}
	}
	return -1
}

func lastNotOf(s string, set string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if strings.IndexByte(set, s[i]) < 0 {
			return i
		}
	}
	return -1
}

// cleanUpDoctext is a port of clean_up_doctext in main.cc. The second
// result is false when the C++ function would return NULL, meaning the
// comment carried no text.
func cleanUpDoctext(doctext string) (string, bool) {
	docstring := strings.ReplaceAll(doctext, "\r", "")

	// Separate into lines. The last segment is dropped when it is only
	// whitespace.
	var lines []string
	last := 0
	for {
		pos := strings.IndexByte(docstring[last:], '\n')
		if pos < 0 {
			if firstNotOf(docstring, " \t", last) >= 0 {
				lines = append(lines, docstring[last:])
			}
			break
		}
		pos += last
		lines = append(lines, docstring[last:pos])
		last = pos + 1
	}

	if len(lines) == 0 {
		return "", false
	}

	// Clear leading whitespace from the first line.
	if pos := firstNotOf(lines[0], " \t", 0); pos >= 0 {
		lines[0] = lines[0][pos:]
	} else {
		lines[0] = ""
	}

	// If every nonblank line after the first has the same number of
	// spaces/tabs, then a star, remove them.
	havePrefix := true
	foundPrefix := false
	prefixLen := 0
	for i := 1; i < len(lines); i++ {
		if lines[i] == "" {
			continue
		}
		pos := firstNotOf(lines[i], " \t", 0)
		if !foundPrefix {
			if pos >= 0 {
				if lines[i][pos] == '*' {
					foundPrefix = true
					prefixLen = pos
				} else {
					havePrefix = false
					break
				}
			} else {
				lines[i] = ""
			}
		} else if pos >= 0 && lines[i][pos] == '*' && pos == prefixLen {
			// Business as usual.
		} else if pos < 0 {
			lines[i] = ""
		} else {
			havePrefix = false
			break
		}
	}

	if havePrefix {
		// Get the star too.
		prefixLen++
		for i := 1; i < len(lines); i++ {
			if len(lines[i]) > prefixLen {
				lines[i] = lines[i][prefixLen:]
			} else {
				lines[i] = ""
			}
		}
	}

	// Now delete the minimum amount of leading whitespace from each line.
	minLen := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "" {
			continue
		}
		pos := firstNotOf(lines[i], " \t", 0)
		if pos >= 0 && (minLen < 0 || pos < minLen) {
			minLen = pos
		}
	}
	if minLen >= 0 {
		for i := 1; i < len(lines); i++ {
			if len(lines[i]) > minLen {
				lines[i] = lines[i][minLen:]
			} else {
				lines[i] = ""
			}
		}
	}

	// Remove trailing whitespace from every line. A line that is nothing
	// but whitespace is left alone, as in the original.
	for i := range lines {
		pos := lastNotOf(lines[i], " \t")
		if pos >= 0 && pos != len(lines[i])-1 {
			lines[i] = lines[i][:pos+1]
		}
	}

	// If the first line is empty, remove it.
	if lines[0] == "" {
		lines = lines[1:]
	}

	var sb strings.Builder
	for _, l := range lines {
		sb.WriteString(l)
		sb.WriteByte('\n')
	}
	return sb.String(), true
}
