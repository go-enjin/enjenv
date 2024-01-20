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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/lang/catalog"
	"github.com/go-enjin/golang-org-x-text/language"

	clpath "github.com/go-corelibs/path"

	"github.com/go-enjin/enjenv/pkg/io"
)

func (c *Command) makeBeMergeLocalesCommand(appNamePrefix string) *cli.Command {
	return &cli.Command{
		Name:      "be-merge-locales",
		Category:  c.TagName,
		Usage:     "merge out.gotext.json with translations from message.gotext.json",
		UsageText: appNamePrefix + " be-merge-locales",
		Description: `
For each of the locales present, merge the out.gotext.json file with
translations from messages.gotext.json and report any translations missing in
messages.gotext.json.
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
		Action: c._mergeLocalesAction,
	}
}

func (c *Command) _mergeLocalesAction(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}

	var outDir string
	var tags []language.Tag
	if outDir, tags, err = parseLangOutArgv(ctx); err != nil {
		return
	}

	err = c._mergeLocalesProcess(outDir, tags)
	return
}

func (c *Command) _mergeLocalesProcess(outDir string, tags []language.Tag) (err error) {
	for _, tag := range tags {
		outDirTag := outDir + "/" + tag.String()
		if clpath.IsDir(outDirTag) {
			if ee := c._mergeLocales(outDirTag); ee == nil {
				io.StdoutF("# updated: %v\n", outDirTag)
			} else if ee.Error() == outDirTag+"/messages.gotext.json not found" {
				if ee = c._initLocales(outDirTag); ee == nil {
					io.StdoutF("# initialized: %v\n", outDirTag)
				} else {
					io.StdoutF("# skipped (init): %v (%v)\n", outDirTag, ee)
				}
			} else {
				io.StdoutF("# skipped: %v (%v)\n", outDirTag, ee)
			}
		}
	}
	return
}

func (c *Command) _mergeLocales(dir string) (err error) {
	messagesGotextPath := dir + "/messages.gotext.json"
	outGotextPath := dir + "/out.gotext.json"

	if !clpath.IsFile(messagesGotextPath) {
		err = fmt.Errorf("%v not found", messagesGotextPath)
		return
	}

	if !clpath.IsFile(outGotextPath) {
		err = fmt.Errorf("%v not found", outGotextPath)
		return
	}

	var messagesGotextJson catalog.GoText
	var messagesGotext map[string]*catalog.Message
	if messagesGotextJson, messagesGotext, _, err = c._readGoText(messagesGotextPath); err != nil {
		return
	}

	var outGotextJson catalog.GoText
	var outGotext map[string]*catalog.Message
	if outGotextJson, outGotext, _, err = c._readGoText(outGotextPath); err != nil {
		return
	}

	var modified []*catalog.Message
	unique := make(map[string]*catalog.Message)

	// populate all found messages
	for _, data := range outGotextJson.Messages {
		if v, ok := messagesGotext[data.Key]; ok {
			if v.Translation.Select == nil && v.Translation.String == "" {
				io.StderrF("# %v locale string translation is empty: \"%v\"\n", outGotextJson.Language, data.Key)
			} else if v.Translation.Select != nil && len(v.Translation.Select.Cases) == 0 {
				io.StderrF("# %v locale select translation has no cases: \"%v\"\n", outGotextJson.Language, data.Key)
			} else {
				data.Translation = v.Translation
			}
		} else {
			io.StderrF("# %v locale translation missing from messages: \"%v\"\n", outGotextJson.Language, data.Key)
		}
		if existing, present := unique[data.Key]; present {
			//io.StderrF("# %v locale translation duplicate skipped: \"%v\"\n", outGotextJson.Language, data.Key)
			existing.TranslatorComment += data.TranslatorComment
		} else {
			unique[data.Key] = data
			modified = append(modified, data)
		}
	}

	// populate custom messages
	for _, data := range messagesGotextJson.Messages {
		if _, ok := outGotext[data.Key]; !ok {
			io.StderrF("# %v locale includes custom message translation: \"%v\": \"%v\"\n", messagesGotextJson.Language, data.Key, data.Translation)
			modified = append(modified, data)
		}
	}

	for idx := range modified {
		modified[idx].TranslatorComment = catalog.CoalesceTranslatorComment(modified[idx].TranslatorComment)
	}

	outGotextJson.Messages = modified

	if err = c._writeGoText(outGotextPath, outGotextJson); err != nil {
		return
	}
	return
}

func (c *Command) _initLocales(dir string) (err error) {
	messagesGotextPath := dir + "/messages.gotext.json"
	outGotextPath := dir + "/out.gotext.json"

	if clpath.IsFile(messagesGotextPath) {
		err = fmt.Errorf("%v exists already", messagesGotextPath)
		return
	}

	if !clpath.IsFile(outGotextPath) {
		err = fmt.Errorf("%v not found", outGotextPath)
		return
	}

	var messageOrder []string
	var outGotextJson catalog.GoText
	var outGotext map[string]*catalog.Message
	if outGotextJson, outGotext, messageOrder, err = c._readGoText(outGotextPath); err != nil {
		return
	}

	messagesGotextJson := catalog.GoText{}
	messagesGotextJson.Language = outGotextJson.Language
	outGotextJson.Messages = nil

	for _, key := range messageOrder {
		message := outGotext[key].Copy()
		if message.Translation == nil {
			message.Translation = &catalog.Translation{
				String: message.Key,
			}
		} else if message.Translation.String == "" && message.Translation.Select == nil {
			message.Translation.String = message.Key
		}
		outGotextJson.Messages = append(outGotextJson.Messages, outGotext[key]) // original
		messagesGotextJson.Messages = append(messagesGotextJson.Messages, message)
	}

	if err = c._writeGoText(outGotextPath, outGotextJson); err != nil {
		return
	}
	if err = c._writeGoText(messagesGotextPath, messagesGotextJson); err != nil {
		return
	}

	return
}

func (c *Command) _readGoText(path string) (gotext catalog.GoText, messages map[string]*catalog.Message, order []string, err error) {
	gotext = catalog.GoText{}
	var data []byte
	if data, err = clpath.ReadFile(path); err != nil {
		err = fmt.Errorf("error reading file: %v - %v", path, err)
		return
	}
	if err = json.Unmarshal(data, &gotext); err != nil {
		err = fmt.Errorf("error parsing json: %v - %v", path, err)
		return
	}
	messages = make(map[string]*catalog.Message)
	for _, message := range gotext.Messages {
		if previous, present := messages[message.Key]; present {
			if message.TranslatorComment != "" && !strings.Contains(previous.TranslatorComment, message.TranslatorComment) {
				messages[message.Key].TranslatorComment += message.TranslatorComment
			}
		} else {
			messages[message.Key] = message
			order = append(order, message.Key)
		}
	}
	return
}

func (c *Command) _writeGoText(destination string, gotext catalog.GoText) (err error) {
	var data []byte
	if data, err = json.MarshalIndent(gotext, "", "    "); err != nil {
		err = fmt.Errorf("error encoding json: %v - %v", destination, err)
		return
	}
	if err = os.WriteFile(destination, data, 0664); err != nil {
		err = fmt.Errorf("error writing file: %v - %v", destination, err)
		return
	}

	return
}
