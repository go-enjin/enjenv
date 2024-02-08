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
	"github.com/urfave/cli/v2"

	"github.com/go-corelibs/x-text/language"
)

func (c *Command) makeBeUpdateLocalesCommand(appNamePrefix string) *cli.Command {
	return &cli.Command{
		Name:      "be-update-locales",
		Category:  c.TagName,
		Usage:     "be-extract-locales and be-merge-locales in one command",
		UsageText: appNamePrefix + " be-update-locales",
		Description: `
Equivalent of running be-extract-locales and then running be-merge-locales right
after.
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
		Action: c._updateLocalesAction,
	}
}

func (c *Command) _updateLocalesAction(ctx *cli.Context) (err error) {
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

	if err = c._extractLocalesProcess(outDir, tags, argv); err != nil {
		return
	}

	if err = c._mergeLocalesProcess(outDir, tags); err != nil {
		return
	}

	return
}
