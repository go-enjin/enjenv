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

	"github.com/go-enjin/be/pkg/hash/sha"

	clpath "github.com/go-corelibs/path"

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

func (c *Command) _extractLocalesAction(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	argv := ctx.Args().Slice()
	argc := len(argv)
	switch argc {
	case 0:
		cli.ShowSubcommandHelpAndExit(ctx, 1)
	}

	var outDir string
	var tags []language.Tag
	if outDir, tags, err = parseLangOutArgv(ctx); err != nil {
		return
	}

	err = c._extractLocalesProcess(outDir, tags, argv)
	return
}

func (c *Command) _extractLocalesRecurse(path string) (msgs []*catalog.Message, err error) {
	if clpath.IsDir(path) {
		// recurse
		if files, e := clpath.ListAllFiles(path, true); e == nil {
			for _, file := range files {
				if found, ee := c._extractLocalesRecurse(file); ee == nil {
					msgs = append(msgs, found...)
				}
			}
		}
		return
	}

	var contents string
	if data, e := clpath.ReadFile(path); e != nil {
		err = e
		return
	} else {
		contents = string(data)
	}

	msgs, err = catalog.ParseTemplateMessages(contents)
	for idx := range msgs {
		if msgs[idx].TranslatorComment != "" {
			msgs[idx].TranslatorComment += "\n"
		}
		msgs[idx].TranslatorComment += "[from: " + path + "]"
	}
	for idx := range msgs {
		msgs[idx].TranslatorComment = catalog.CoalesceTranslatorComment(msgs[idx].TranslatorComment)
	}
	return
}

func (c *Command) _extractLocalesProcess(outDir string, tags []language.Tag, argv []string) (err error) {

	if !clpath.Exists(outDir) {
		if err = clpath.MkdirAll(outDir); err != nil {
			err = fmt.Errorf("error making directory: %v - %v\n", outDir, err)
			return
		}
	}

	for _, tag := range tags {
		outDirTag := outDir + "/" + tag.String()
		if !clpath.IsDir(outDirTag) {
			if err = clpath.MkdirAll(outDirTag); err != nil {
				err = fmt.Errorf("error making directory: %v - %v\n", outDirTag, err)
				return
			}
		}
	}

	var messages []*catalog.Message

	for _, arg := range argv {
		if msgs, ee := c._extractLocalesRecurse(arg); ee == nil {
			messages = append(messages, msgs...)
		}
	}

	for _, tag := range tags {
		outPath := outDir + "/" + tag.String() + "/out.gotext.json"
		outData := catalog.GoText{
			Language: tag.String(),
			Messages: messages,
		}
		if output, eee := json.MarshalIndent(outData, "", "    "); eee == nil {
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
