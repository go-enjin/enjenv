// Copyright (c) 2023  The Go-Enjin Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package niseroku

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

var (
	rxTomlSection      = regexp.MustCompile(`^(\s*)(\[[^#]+?\])\s*$`)
	rxTomlStatement    = regexp.MustCompile(`^(\s*)([^#]+?)\s*=\s*([^#]+?)\s*$`)
	rxTomlCommentLine  = regexp.MustCompile(`^\s*#(.+?)\s*$`)
	rxTomlCommentTrail = regexp.MustCompile(`^\s*([^#]+?)\s*#(.+?)\s*$`)
	rxEmptyString      = regexp.MustCompile(`^\s*$`)
)

type TomlComments []*TomlComment

func (t TomlComments) String() (v string) {
	v += "[\n"
	last := len(t) - 1
	for idx, tc := range t {
		v += "\t" + tc.String()
		if idx < last {
			v += ","
		}
		v += "\n"
	}
	v += "]\n"
	return
}

type TomlComment struct {
	Lines     []string
	Inline    string
	Statement string
}

func (t *TomlComment) String() (v string) {
	v += "{ "
	v += `s:"` + t.Statement + `", `
	v += `l:"` + strings.Join(t.Lines, "\\n") + `", `
	v += `t:"` + t.Inline + `"`
	v += " }"
	return
}

func ParseComments(content string) (tomlComments TomlComments, err error) {
	input := strings.NewReader(content)
	scanner := bufio.NewScanner(input)

	var thisEntry *TomlComment
	for scanner.Scan() {
		line := scanner.Text()
		if rxEmptyString.MatchString(line) {
			continue
		}

		if rxTomlCommentLine.MatchString(line) {
			m := rxTomlCommentLine.FindAllStringSubmatch(line, 1)
			if thisEntry == nil {
				thisEntry = new(TomlComment)
			}
			thisEntry.Lines = append(thisEntry.Lines, m[0][1])
			continue
		}

		if rxTomlCommentTrail.MatchString(line) {
			m := rxTomlCommentTrail.FindAllStringSubmatch(line, 1)
			if thisEntry == nil {
				thisEntry = new(TomlComment)
			}
			if rxTomlStatement.MatchString(m[0][1]) {
				mm := rxTomlStatement.FindAllStringSubmatch(m[0][1], 1)
				thisEntry.Statement = mm[0][2]
			} else if rxTomlSection.MatchString(m[0][1]) {
				mm := rxTomlSection.FindAllStringSubmatch(m[0][1], 1)
				thisEntry.Statement = mm[0][2]
			} else {
				err = fmt.Errorf("unable to parse line with trailing comment: %v", line)
				return
			}
			thisEntry.Inline = m[0][2]
			tomlComments = append(tomlComments, thisEntry)
			thisEntry = nil
			continue
		}

		if thisEntry != nil {
			if thisEntry.Statement == "" {
				if rxTomlStatement.MatchString(line) {
					m := rxTomlStatement.FindAllStringSubmatch(line, 1)
					thisEntry.Statement = m[0][2]
					tomlComments = append(tomlComments, thisEntry)
					thisEntry = nil
					continue
				}

				if rxTomlSection.MatchString(line) {
					m := rxTomlSection.FindAllStringSubmatch(line, 1)
					thisEntry.Statement = m[0][2]
					tomlComments = append(tomlComments, thisEntry)
					thisEntry = nil
					continue
				}
			}
		}
	}

	err = scanner.Err()
	return
}

func ApplyComments(content string, comments TomlComments) (modified string, err error) {
	input := strings.NewReader(content)
	scanner := bufio.NewScanner(input)

	isFirstLine := true
	prevIsEmpty := false
	for scanner.Scan() {
		line := scanner.Text()
		if rxEmptyString.MatchString(line) {
			modified += "\n"
			prevIsEmpty = true
			continue
		}

		var valid bool
		var pad, stmnt, actual string
		if valid = rxTomlStatement.MatchString(line); valid {
			m := rxTomlStatement.FindAllStringSubmatch(line, 1)
			pad = m[0][1]
			stmnt = m[0][2]
			actual = pad + stmnt + " = " + m[0][3]
		} else if valid = rxTomlSection.MatchString(line); valid {
			m := rxTomlSection.FindAllStringSubmatch(line, 1)
			pad = m[0][1]
			stmnt = m[0][2]
			actual = pad + stmnt
		} else {
			err = fmt.Errorf("invalid toml statement: %v\n", line)
			return
		}

		found := false
		for _, comment := range comments {
			if found = comment.Statement == stmnt; found {
				if len(comment.Lines) > 0 {
					if pad == "" && !isFirstLine && !prevIsEmpty {
						modified += "\n"
					}
					for _, cl := range comment.Lines {
						modified += pad + "#" + cl + "\n"
					}
				}
				modified += actual
				if comment.Inline != "" {
					modified += " #" + comment.Inline
				}
				modified += "\n"
				break
			}
		}
		if !found {
			modified += line + "\n"
		}

		prevIsEmpty = false
		isFirstLine = false
	}

	err = scanner.Err()
	return
}