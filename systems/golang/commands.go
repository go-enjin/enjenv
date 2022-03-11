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

package golang

import (
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"

	"github.com/go-enjin/be/pkg/cli/run"

	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/system"
)

func (s *System) IncludeCommands(app *cli.App) (commands []*cli.Command) {
	appNamePrefix := app.Name + " " + Name
	commands = []*cli.Command{
		&cli.Command{
			HideHelpCommand: true,
			Name:            "build",
			Category:        system.SystemCategory,
			Usage:           "specialized go build for working with Go-Enjin projects",
			UsageText:       appNamePrefix + " build [options] -- [go build arguments]",
			Action:          s.ActionGoBuild,
			Flags:           CliBuildFlags,
			Before: altsrc.InitInputSourceWithContext(
				CliBuildFlags,
				altsrc.NewTomlSourceFromFlagFunc("config"),
			),
		},
	}
	if _, err := s.GetInstalledVersion(); err == nil {
		if !s.nancyPresent() {
			commands = append(
				commands,
				&cli.Command{
					Name:      "setup-nancy",
					Category:  system.SystemCategory,
					Usage:     "build nancy from a git clone",
					UsageText: appNamePrefix + " setup-nancy",
					Action: func(ctx *cli.Context) (err error) {
						if s.nancyPresent() {
							io.StdoutF("# found nancy, nothing to do\n")
							return
						}
						err = s.installNancy()
						return
					},
				},
			)
		}
	}
	return
}

func (s *System) ExtraCommands(app *cli.App) (commands []*cli.Command) {
	commands = []*cli.Command{
		&cli.Command{
			HideHelpCommand: true,
			Name:            "go",
			Usage:           "wrapper for local bin/go",
			UsageText:       app.Name + " go -- [go arguments]",
			Action: func(ctx *cli.Context) (err error) {
				if err = s.Prepare(ctx); err != nil {
					return
				}
				argv := ctx.Args().Slice()
				if len(argv) > 0 {
					name := argv[0]
					args := argv[1:]
					_, err = s.GoBin(name, args...)
					return
				}
				_, err = s.GoBin("help")
				return
			},
		},
		&cli.Command{
			HideHelpCommand: true,
			Name:            "go-local",
			Usage:           "go mod edit -replace wrapper",
			UsageText: "\n" +
				"\t# set an arbitrary package name to be replaced with given path\n" +
				"\t" + app.Name + " go-local <any/package/name> <path/to/checkout>\n\n" +
				"\t# set github.com/go-enjin/be to be replaced with given path\n" +
				"\t" + app.Name + " go-local <path/to/go-enjin/be>\n\n" +
				"\t# set github.com/go-enjin/be to be replaced with the $BE_LOCAL_PATH environment variable\n" +
				"\t" + app.Name + " go-local",
			Action: s.ActionGoModLocal,
		},
		&cli.Command{
			HideHelpCommand: true,
			Name:            "go-unlocal",
			Usage:           "go mod edit -dropreplace wrapper",
			UsageText: "\n" +
				"\t# drop the replacement for an arbitrary package name\n" +
				"\t" + app.Name + " go-unlocal <package>\n\n" +
				"\t# drop the replacement for github.com/go-enjin/be\n" +
				"\t" + app.Name + " go-unlocal",
			Action: s.ActionGoModUnLocal,
		},
	}
	if _, err := s.GetInstalledVersion(); err == nil {
		if s.nancyPresent() {
			commands = append(
				commands,
				&cli.Command{
					Name:      "nancy",
					Usage:     "wrapper around the installed nancy binary",
					UsageText: app.Name + " nancy -- [nancy arguments]",
					Action: func(ctx *cli.Context) (err error) {
						argv := ctx.Args().Slice()
						_, err = s.NancyBin(argv...)
						return
					},
				},
				&cli.Command{
					Name:      "go-audit",
					Usage:     "wrapper for 'go list mod -json -deps | nancy sleuth [nancy arguments]'",
					UsageText: app.Name + " go-audit -- [nancy arguments]",
					Action: func(ctx *cli.Context) (err error) {
						goBin := basepath.MakeEnjenvPath(s.Root, "bin", "go")
						nancyBin := basepath.MakeEnjenvPath(s.Root, "bin", "nancy")
						argv := append([]string{"sleuth"}, ctx.Args().Slice()...)
						_, err = run.ExePipe(
							run.NewPipe(goBin, "list", "-json", "-deps"),
							run.NewPipe(nancyBin, argv...),
						)
						return
					},
				},
			)
		}
	}
	return
}