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
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/_templates"

	"github.com/go-enjin/enjenv/pkg/io"
)

func (c *Command) makeBePkgListCommand(appNamePrefix string) *cli.Command {
	return &cli.Command{
		Name:      "be-pkg-list",
		Category:  c.TagName,
		Usage:     "print all github.com/go-enjin/be packages",
		UsageText: appNamePrefix + " be-pkg-list [-l]",
		Description: `
Print a space-separated list of all Go-Enjin package names, useful for
generating translation locales using the github.com/go-enjin/golang-org-x-text
version of gotext (which supports go modules).
`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "list",
				Usage:   "output one line per pkg name",
				Aliases: []string{"l"},
			},
		},
		Action: func(ctx *cli.Context) (err error) {
			if err = c.Prepare(ctx); err != nil {
				return
			}
			list := _templates.GoEnjinPackageList()
			if !ctx.Bool("list") {
				io.StdoutF("%v\n", strings.Join(list, " "))
			} else {
				for _, pkg := range list {
					io.StdoutF("%v\n", pkg)
				}
			}
			return
		},
	}
}
