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

package nodejs

import (
	"github.com/urfave/cli/v2"

	bePath "github.com/go-enjin/be/pkg/path"

	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/system"
)

func (s *System) IncludeCommands(app *cli.App) (commands []*cli.Command) {
	appNamePrefix := app.Name + " " + Name
	commands = []*cli.Command{}
	if _, err := s.GetInstalledVersion(); err == nil {
		if !s.herokuPresent() {
			commands = append(
				commands,
				&cli.Command{
					Name:      "setup-heroku",
					Category:  system.SystemCategory,
					Usage:     "install heroku",
					UsageText: appNamePrefix + " setup-heroku",
					Action: func(ctx *cli.Context) (err error) {
						if s.herokuPresent() {
							io.StdoutF("# found heroku, nothing to do\n")
							return
						}
						err = s.installHeroku()
						return
					},
				},
			)
		}
	} else {
		io.StderrF("error getting installed nodejs version: %v\n", err)
	}
	return
}

func (s *System) herokuPresent() (ok bool) {
	heroku := basepath.MakeEnjenvPath(s.Root, "bin", "heroku")
	ok = bePath.IsFile(heroku)
	return
}

func (s *System) installHeroku() (err error) {
	_, err = s.NpmBin("install", "-g", "heroku")
	return
}

func (s *System) ExtraCommands(app *cli.App) (commands []*cli.Command) {
	commands = []*cli.Command{
		{
			HideHelpCommand: true,
			Name:            "node",
			Usage:           "wrapper for local bin/node",
			UsageText:       app.Name + " node -- [node arguments]",
			Action: func(ctx *cli.Context) (err error) {
				if err = s.Prepare(ctx); err != nil {
					return
				}
				argv := ctx.Args().Slice()
				if len(argv) > 0 {
					err = s.NodeBin(argv...)
					return
				}
				err = s.NodeBin("--help")
				return
			},
		},
		{
			HideHelpCommand: true,
			Name:            "npm",
			Usage:           "wrapper for local bin/npm",
			UsageText:       app.Name + " npm -- [npm arguments]",
			Action: func(ctx *cli.Context) (err error) {
				if err = s.Prepare(ctx); err != nil {
					return
				}
				argv := ctx.Args().Slice()
				if len(argv) > 0 {
					name := argv[0]
					args := argv[1:]
					_, err = s.NpmBin(name, args...)
					return
				}
				_, err = s.NpmBin("--help")
				return
			},
		},
		{
			HideHelpCommand: true,
			Name:            "npx",
			Usage:           "wrapper for local bin/npx",
			UsageText:       app.Name + " npx -- [npx arguments]",
			Action: func(ctx *cli.Context) (err error) {
				if err = s.Prepare(ctx); err != nil {
					return
				}
				argv := ctx.Args().Slice()
				if len(argv) > 0 {
					name := argv[0]
					args := argv[1:]
					_, err = s.NpxBin(name, args...)
					return
				}
				_, err = s.NpxBin("--help")
				return
			},
		},
		{
			HideHelpCommand: true,
			Name:            "yarn",
			Usage:           "wrapper for local yarn",
			UsageText:       app.Name + " yarn -- [yarn arguments]",
			Action: func(ctx *cli.Context) (err error) {
				if err = s.Prepare(ctx); err != nil {
					return
				}
				argv := ctx.Args().Slice()
				if len(argv) > 0 {
					name := argv[0]
					args := argv[1:]
					_, err = s.YarnBin(name, args...)
					return
				}
				_, err = s.YarnBin("--help")
				return
			},
		},
	}

	if s.herokuPresent() {
		commands = append(commands, &cli.Command{
			HideHelpCommand: true,
			Name:            "heroku",
			Usage:           "wrapper for local heroku",
			UsageText:       app.Name + " heroku -- [heroku arguments]",
			Action: func(ctx *cli.Context) (err error) {
				if err = s.Prepare(ctx); err != nil {
					return
				}
				argv := ctx.Args().Slice()
				if len(argv) > 0 {
					name := argv[0]
					args := argv[1:]
					_, err = s.HerokuBin(name, args...)
					return
				}
				_, err = s.HerokuBin("--help")
				return
			},
		})
	}

	if scripts := s.MakeScriptCommands(app); scripts != nil {
		for _, script := range scripts {
			commands = append(commands, script)
		}
	}
	return
}