// Copyright (c) 2022  The Go-Enjin Authors
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

package enjin

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-enjin/golang-org-x-text/language"
	"github.com/goccy/go-json"
	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/hash/sha"
	"github.com/go-enjin/be/pkg/maps"
	bePath "github.com/go-enjin/be/pkg/path"
	"github.com/go-enjin/be/pkg/slices"
	"github.com/go-enjin/enjenv/pkg/io"
)

func (c *Command) makeBeExtractLocalesCommand(appNamePrefix string) *cli.Command {
	return &cli.Command{
		Name:      "be-extract-locales",
		Category:  c.TagName,
		Usage:     "extract translation strings from enjin template files",
		UsageText: appNamePrefix + " be-extract-locales [options] <path> [paths...]",
		Description: `
Searches all paths recursively for any uses of the "_" template function and
produces a basic out.gotext.json file.
`,
		Flags: []cli.Flag{
			&cli.PathFlag{
				Name:  "out",
				Value: "locales",
				Usage: "locales directory path to use",
			},
			&cli.StringFlag{
				Name:  "lang",
				Value: language.English.String(),
				Usage: "command separated list of languages to process",
			},
		},
		Action: c._extractLocalesAction,
	}
}

var (
	rxpQuotedText = `".+?"`
	rxpContextKey = `\.[a-zA-Z0-9][_.a-zA-Z0-9]+`
	rxpVariable   = `\$[a-zA-Z][_.a-zA-Z0-9]+`
	rxpPipeline   = `\(.+?\)`

	rxpExtractFn   = `_\s+(` + rxpQuotedText + `|` + rxpVariable + `|` + rxpContextKey + `|` + rxpPipeline + `)\s*`
	rxpExtractArgs = `_\s+(` + rxpQuotedText + `|` + rxpVariable + `|` + rxpContextKey + `|` + rxpPipeline + `)\s*([^}]*)`
	rxpExtractNote = `/\*\s+?([^*}]+)\s+?\*/`

	rxExtractFnNope         = regexp.MustCompile(`\(\s*([^_][^"\s]+)\s*(.*)\s*\)`)
	rxExtractFnCallArgsNote = regexp.MustCompile(`\(\s*` + rxpExtractArgs + `\s*` + rxpExtractNote + `\s*\)`)
	rxExtractFnCallNote     = regexp.MustCompile(`\(\s*` + rxpExtractFn + `\s*` + rxpExtractNote + `\s*\)`)
	rxExtractFnCallArgs     = regexp.MustCompile(`\(\s*` + rxpExtractArgs + `\s*\)`)
	rxExtractFnCall         = regexp.MustCompile(`\(\s*` + rxpExtractFn + `\s*\)`)

	rxExtractFnPipeArgsNote = regexp.MustCompile(`\{\{-??\s*` + rxpExtractArgs + `\s*` + rxpExtractNote + `\s*-??}}`)
	rxExtractFnPipeNote     = regexp.MustCompile(`\{\{-??\s*` + rxpExtractFn + `\s*` + rxpExtractNote + `\s*-??}}`)
	rxExtractFnPipeArgs     = regexp.MustCompile(`\{\{-??\s*` + rxpExtractArgs + `\s*-??}}`)
	rxExtractFnPipe         = regexp.MustCompile(`\{\{-??\s*` + rxpExtractFn + `\s*-??}}`)
)

func (c *Command) _extractLocalesParseMessages(path, content string, rx *regexp.Regexp) (modified string, extracted, variables map[string][]string) {
	name := filepath.Base(path)
	extracted = make(map[string][]string)
	variables = make(map[string][]string)
	if rx.MatchString(content) {
		m := rx.FindAllStringSubmatch(content, -1)
		for _, mm := range m {
			var hint string
			switch len(mm) {
			case 2: // no hints (matched and key)
			case 3: // args are hints (matched, key, hint)
				hint = mm[2]
			case 4: // hints are hints (matched, key, vars, hint)
				hint = mm[3]
			}
			if hint != "" {
				hint = strings.TrimSpace(hint) + " [from: " + name + "]"
			} else {
				hint = "[from: " + name + "]"
			}
			if key := mm[1]; key != "" && key[0] == '"' {
				key = strings.ReplaceAll(key, "\"", "")
				extracted[key] = append(extracted[key], hint)
			} else {
				variables[key] = append(variables[key], hint)
			}
		}
		modified = rx.ReplaceAllString(content, "($1)")
	}
	return
}

func (c *Command) _extractLocales(path string) (extracted map[string][]string) {
	extracted = make(map[string][]string)

	if bePath.IsDir(path) {
		// recurse
		if files, err := bePath.ListAllFiles(path); err == nil {
			for _, file := range files {
				for k, v := range c._extractLocales(file) {
					extracted[k] = append(extracted[k], v...)
				}
			}
		}
		return
	}

	var err error
	var contents []byte
	if contents, err = bePath.ReadFile(path); err != nil {
		io.StderrF("error reading file: %v - %v\n", path, err)
		return
	}

	variableKeys := make(map[string][]string)
	modified := contents
	modified = []byte(rxExtractFnNope.ReplaceAllString(string(modified), "($1)"))

	update := func(label, mod string, foundMsgs, foundVars map[string][]string) {
		modified = []byte(mod)
		for k, v := range foundMsgs {
			for _, vv := range v {
				if !slices.Present(vv, extracted[k]...) {
					extracted[k] = append(extracted[k], vv)
				}
			}
			// io.StderrF("[%v] message: %v - %v\n", label, k, strings.Join(v, ", "))
		}
		for k, v := range foundVars {
			for _, vv := range v {
				if !slices.Present(vv, variableKeys[k]...) {
					variableKeys[k] = append(variableKeys[k], vv)
				}
			}
			// io.StderrF("[%v] variable: %v - %v\n", label, k, strings.Join(v, ", "))
		}
	}

	if mod, foundMsgs, foundVars := c._extractLocalesParseMessages(path, string(modified), rxExtractFnCallArgsNote); mod != "" {
		update("call-args-note", mod, foundMsgs, foundVars)
	}

	if mod, foundMsgs, foundVars := c._extractLocalesParseMessages(path, string(modified), rxExtractFnCallNote); mod != "" {
		update("call-note", mod, foundMsgs, foundVars)
	}

	if mod, foundMsgs, foundVars := c._extractLocalesParseMessages(path, string(modified), rxExtractFnCallArgs); mod != "" {
		update("call-args", mod, foundMsgs, foundVars)
	}

	if mod, foundMsgs, foundVars := c._extractLocalesParseMessages(path, string(modified), rxExtractFnCall); mod != "" {
		update("call", mod, foundMsgs, foundVars)
	}

	if mod, foundMsgs, foundVars := c._extractLocalesParseMessages(path, string(modified), rxExtractFnPipeArgsNote); mod != "" {
		update("pipe-args-note", mod, foundMsgs, foundVars)
	}

	if mod, foundMsgs, foundVars := c._extractLocalesParseMessages(path, string(modified), rxExtractFnPipeNote); mod != "" {
		update("pipe-note", mod, foundMsgs, foundVars)
	}

	if mod, foundMsgs, foundVars := c._extractLocalesParseMessages(path, string(modified), rxExtractFnPipeArgs); mod != "" {
		update("pipe-args", mod, foundMsgs, foundVars)
	}

	if mod, foundMsgs, foundVars := c._extractLocalesParseMessages(path, string(modified), rxExtractFnPipe); mod != "" {
		update("pipe", mod, foundMsgs, foundVars)
	}

	for _, vk := range maps.SortedKeys(variableKeys) {
		if vk != "" {
			io.StderrF("# skipping variable translation: %v - %v\n", vk, strings.Join(variableKeys[vk], ", "))
		}
	}

	return
}

func (c *Command) _extractLocalesAction(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	argv := ctx.Args().Slice()
	argc := len(argv)
	switch argc {
	case 0:
		cli.ShowCommandHelpAndExit(ctx, "be-extract-locales", 1)
	}

	var outDir string
	var tags []language.Tag
	if outDir, tags, err = parseLangOutArgv(ctx); err != nil {
		return
	}

	err = c._extractLocalesProcess(outDir, tags, argv)
	return
}

func (c *Command) _extractLocalesProcess(outDir string, tags []language.Tag, argv []string) (err error) {

	if !bePath.Exists(outDir) {
		if err = bePath.Mkdir(outDir); err != nil {
			err = fmt.Errorf("error making directory: %v - %v\n", outDir, err)
			return
		}
	}

	for _, tag := range tags {
		outDirTag := outDir + "/" + tag.String()
		if !bePath.IsDir(outDirTag) {
			if err = bePath.Mkdir(outDirTag); err != nil {
				err = fmt.Errorf("error making directory: %v - %v\n", outDirTag, err)
				return
			}
		}
	}

	found := make(map[string][]string)

	for _, arg := range argv {
		for key, hints := range c._extractLocales(arg) {
			for _, hint := range hints {
				if !slices.Present(hint, found[key]...) {
					found[key] = append(found[key], hint)
				}
			}
		}
	}

	sorted := maps.SortedKeys(found)

	for idx, tag := range tags {
		outPath := outDir + "/" + tag.String() + "/out.gotext.json"
		outData := GotextData{
			Language: tag.String(),
		}
		for _, key := range sorted {
			var hints string
			if len(found[key]) > 0 {
				if examples := strings.Join(found[key], ", "); examples != "" {
					hints = examples
				} else {
					hints = "Copied from source."
				}
			} else {
				hints = "Copied from source."
			}
			datum := Message{
				Id:                key,
				Key:               key,
				Message:           key,
				TranslatorComment: hints,
			}
			if idx > 0 {
				datum.Translation = key + " (" + tag.String() + ")"
			} else {
				datum.Translation = key
			}
			outData.Messages = append(outData.Messages, datum)
		}
		if output, eee := json.MarshalIndent(outData, "", "\t"); eee == nil {
			if eee = os.WriteFile(outPath, []byte(output), 0664); eee != nil {
				err = fmt.Errorf("error writing file: %v - %v", outPath, eee)
				return
			}
			if sum, ee := sha.FileHash64(outPath); ee != nil {
				err = fmt.Errorf("error gettin shasum: %v - %v", outPath, ee)
				return
			} else {
				io.StdoutF("%v %v\n", sum, outPath)
			}
		} else {
			err = fmt.Errorf("error encoding json: %v - %v", outPath, eee)
		}
	}

	return
}