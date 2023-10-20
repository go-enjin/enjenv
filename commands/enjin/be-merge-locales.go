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

	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/lang/catalog"
	"github.com/go-enjin/golang-org-x-text/language"

	bePath "github.com/go-enjin/be/pkg/path"
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
		if bePath.IsDir(outDirTag) {
			if ee := c._mergeLocales(outDirTag); ee == nil {
				io.StdoutF("# updated: %v\n", outDirTag)
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

	if !bePath.IsFile(messagesGotextPath) {
		err = fmt.Errorf("%v not found", messagesGotextPath)
		return
	}

	if !bePath.IsFile(outGotextPath) {
		err = fmt.Errorf("%v not found", outGotextPath)
		return
	}

	var messagesGotextJson catalog.GoText
	var messagesGotextContents []byte
	if messagesGotextContents, err = bePath.ReadFile(messagesGotextPath); err != nil {
		err = fmt.Errorf("error reading file: %v - %v", messagesGotextPath, err)
		return
	}
	if err = json.Unmarshal(messagesGotextContents, &messagesGotextJson); err != nil {
		err = fmt.Errorf("error parsing json: %v - %v", messagesGotextPath, err)
		return
	}
	messagesGotext := make(map[string]*catalog.Message)
	for _, data := range messagesGotextJson.Messages {
		messagesGotext[data.Key] = data
	}

	var outGotextJson catalog.GoText
	var outGotextContents []byte
	if outGotextContents, err = bePath.ReadFile(outGotextPath); err != nil {
		err = fmt.Errorf("error reading file: %v - %v", outGotextPath, err)
		return
	}
	if err = json.Unmarshal(outGotextContents, &outGotextJson); err != nil {
		err = fmt.Errorf("error parsing json: %v - %v", outGotextPath, err)
		return
	}
	outGotext := make(map[string]*catalog.Message)
	for _, data := range outGotextJson.Messages {
		outGotext[data.Key] = data
	}

	var modified []*catalog.Message

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
		modified = append(modified, data)
	}

	for _, data := range messagesGotextJson.Messages {
		if _, ok := outGotext[data.Key]; !ok {
			io.StderrF("# %v locale includes custom message translation: \"%v\": \"%v\"\n", messagesGotextJson.Language, data.Key, data.Translation)
			modified = append(modified, data)
		}
	}

	outGotextJson.Messages = modified

	var outJson []byte
	if outJson, err = json.MarshalIndent(outGotextJson, "", "\t"); err != nil {
		err = fmt.Errorf("error encoding json: %v - %v", outGotextPath, err)
		return
	}
	if err = os.WriteFile(outGotextPath, outJson, 0664); err != nil {
		err = fmt.Errorf("error writing file: %v - %v", outGotextPath, err)
		return
	}
	return
}