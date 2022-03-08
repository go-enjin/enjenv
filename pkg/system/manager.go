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

package system

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/cli/env"
	"github.com/go-enjin/be/pkg/cli/git"
	bePath "github.com/go-enjin/be/pkg/path"
	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/io"
)

var TmpDirName = "tmp"

const (
	ShellCategory   = "general shell"
	BuildCategory   = "general build"
	SystemCategory  = "system"
	GeneralCategory = "general"
)

var _manager *SystemsManager

type SystemsManager struct {
	commands []Command
	systems  []System
}

func Manager() (m *SystemsManager) {
	if _manager == nil {
		_manager = new(SystemsManager)
		_manager.commands = make([]Command, 0)
		_manager.systems = make([]System, 0)
	}
	m = _manager
	return
}

func (m *SystemsManager) AddCommand(c Command) *SystemsManager {
	for _, known := range m.commands {
		if known.Name() == c.Name() {
			return m
		}
	}
	m.commands = append(m.commands, c)
	return m
}

func (m *SystemsManager) AddSystem(s System) *SystemsManager {
	for _, known := range m.systems {
		if known.Name() == s.Name() {
			return m
		}
	}
	m.systems = append(m.systems, s)
	return m
}

func (m *SystemsManager) Shutdown() {
	io.Shutdown()
}

func (m *SystemsManager) Setup(app *cli.App) (err error) {
	app.HideHelpCommand = true
	app.UsageText = "\n\t" + BinName + " command [command options]\n\n"
	app.UsageText += "\t# with no arguments, prints the current enjenv path\n"
	app.UsageText += "\t$ " + BinName + "\n"
	app.UsageText += "\t/path/to/.enjenv\n"

	app.Flags = append(app.Flags, &cli.StringFlag{
		Name:    "slack",
		Usage:   "send notifications the given slack channel as well as os.Stdout",
		EnvVars: []string{EnvPrefix + "_SLACK"},
	})

	var names []string
	isInstalled := make(map[string]bool)
	atLeastOneInstalled := false
	for _, s := range m.systems {
		name := s.Name()
		names = append(names, name)
		if err = s.Setup(app); err != nil {
			return
		}
		if _, err := s.GetInstalledVersion(); err == nil {
			isInstalled[name] = true
			atLeastOneInstalled = true
		}
	}

	var commands []*cli.Command
	for _, s := range m.systems {
		name := s.Name()
		c := &cli.Command{
			HideHelpCommand: true,
			Name:            name,
			Category:        GeneralCategory + " environments",
			Usage:           "work with a local " + name + " environment",
			Action: func(ctx *cli.Context) (err error) {
				cli.ShowAppHelpAndExit(ctx, 1)
				return
			},
			Subcommands: []*cli.Command{
				&cli.Command{
					HideHelpCommand: true,
					Name:            "init",
					Category:        SystemCategory,
					Usage:           "create a local " + name + " environment",
					UsageText:       app.Name + " " + name + " init [options]",
					Action:          s.InitAction,
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:  "force",
							Usage: "overwrite any existing installation",
						},
						&cli.StringFlag{
							Name:  name,
							Usage: "version of " + name + " to download, or path to local .tar.gz",
							Value: s.GetDefaultVersion(),
						},
					},
				},
			},
		}
		if isInstalled[name] {
			c.Subcommands = append(
				c.Subcommands,
				&cli.Command{
					HideHelpCommand: true,
					Name:            "version",
					Category:        GeneralCategory,
					Usage:           "reports the installed " + name + " version",
					UsageText:       app.Name + " " + name + " version",
					Action:          s.VersionAction,
				},
				&cli.Command{
					HideHelpCommand: true,
					Name:            "clean",
					Category:        GeneralCategory,
					Usage:           "delete the local " + name + " environment",
					UsageText:       app.Name + " " + name + " clean [options]",
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:  "force",
							Usage: "actually delete things",
						},
					},
					Action: s.CleanAction,
				},
				&cli.Command{
					HideHelpCommand: true,
					Name:            "export",
					Category:        ShellCategory,
					Usage:           "output shell export statements for all installed systems",
					UsageText:       app.Name + " " + name + " export",
					Action:          s.ExportAction,
				},
				&cli.Command{
					HideHelpCommand: true,
					Name:            "unexport",
					Category:        ShellCategory,
					Usage:           "output shell unset statements for all installed systems (inverse of export)",
					UsageText:       app.Name + " " + name + " unexport",
					Action:          s.UnExportAction,
				},
			)

			if included := s.IncludeCommands(app); len(included) > 0 {
				c.Subcommands = append(c.Subcommands, included...)
			}
		}
		commands = append(commands, c)
	}

	for _, s := range m.systems {
		name := s.Name()
		if isInstalled[name] {
			if extras := s.ExtraCommands(app); len(extras) > 0 {
				// commands = append(commands, extras...)
				for _, extra := range extras {
					if extra.Category == "" {
						extra.Category = name + " " + SystemCategory
					}
					commands = append(commands, extra)
				}
			}
		}
	}

	for _, c := range m.commands {
		if extras := c.ExtraCommands(app); len(extras) > 0 {
			for _, extra := range extras {
				commands = append(commands, extra)
			}
		}
	}

	initCommand := &cli.Command{
		HideHelpCommand: true,
		Name:            "init",
		Category:        GeneralCategory,
		Usage:           "create local environments for all systems",
		UsageText: fmt.Sprintf(`
	%v init [options] [path]

	# using ENJENV_PATH (defaults to ./%v)
	%v init --golang 1.17.7

	# force current directory (creates ./%v)
	%v init --golang 1.17.7 .
`,
			app.Name,
			basepath.EnjenvDirName,
			app.Name,
			basepath.EnjenvDirName,
			app.Name,
		),
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "force",
				Usage: "overwrite any existing installation",
			},
		},
		Action: func(ctx *cli.Context) (err error) {
			if err = setupSlackIfPresent(ctx); err != nil {
				return
			}
			if len(m.systems) == 0 {
				err = fmt.Errorf("no systems built-in, nothing to do")
				return
			}
			io.NotifyF("all systems init started")
			for _, s := range m.systems {
				name := s.Self().Name()
				if ctx.IsSet(name) && ctx.String(name) != "" {
					if err = s.InitSystem(ctx); err != nil {
						return
					}
				}
			}
			io.NotifyF("all systems init complete")
			return
		},
	}

	for _, s := range m.systems {
		name := s.Self().Name()
		initCommand.Flags = append(
			initCommand.Flags,
			&cli.StringFlag{
				Name:  name,
				Usage: "version of " + name + " to download, or path to local .tar.gz",
				Value: s.Self().GetDefaultVersion(),
			},
		)
	}
	commands = append(commands, initCommand)

	if bePath.IsDir(basepath.EnjenvPath) {
		commands = append(
			commands,
			&cli.Command{
				Name:      "write-scripts",
				Category:  ShellCategory,
				Usage:     "create bash_completion, activate and deactivate shell scripts",
				UsageText: app.Name + " write-scripts",
				Action: func(ctx *cli.Context) (err error) {
					// activate
					content := fmt.Sprintf("export ENJENV_PATH=\"%v\"\n", basepath.EnjenvPath)
					for _, s := range m.systems {
						if err = s.Self().Prepare(ctx); err != nil {
							return
						}
						if v, e := s.Self().ExportString(ctx); e == nil {
							content += v
						}
						s.ExportPathVariable(false)
					}
					content += fmt.Sprintf("export TMPDIR=\"%v\"\n", basepath.MakeEnjenvPath(TmpDirName))
					content += fmt.Sprintf("export PATH=\"%v\"\n", strings.Join(env.GetPaths(), ":"))
					script := basepath.MakeEnjenvPath("activate")
					if err = os.WriteFile(script, []byte(content), 0660); err != nil {
						return
					}
					io.StdoutF("# wrote: %v\n", script)

					// deactivate
					content = fmt.Sprintf("unset ENJENV_PATH;\n")
					for _, s := range m.systems {
						if v, e := s.Self().UnExportString(ctx); e == nil {
							content += v
						}
						s.UnExportPathVariable(false)
					}
					content += fmt.Sprintf("export TMPDIR;\n")
					content += fmt.Sprintf("export PATH=\"%v\"\n", strings.Join(env.GetPaths(), ":"))
					script = basepath.MakeEnjenvPath("deactivate")
					if err = os.WriteFile(script, []byte(content), 0660); err != nil {
						return
					}
					io.StdoutF("# wrote: %v\n", script)

					// bash_completion
					script = basepath.MakeEnjenvPath("bash_completion")
					if err = os.WriteFile(script, []byte(BashCompletionScript), 0660); err != nil {
						return
					}
					io.StdoutF("# wrote: %v\n", script)
					return
				},
			},
			&cli.Command{
				HideHelpCommand: true,
				Name:            "clean",
				Category:        GeneralCategory,
				Usage:           "delete the local environments",
				UsageText:       app.Name + " clean [options]",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "force",
						Usage: "actually delete things",
					},
				},
				Action: func(ctx *cli.Context) (err error) {
					if !ctx.Bool("force") {
						err = fmt.Errorf("not cleaning local environments: (missing --force)")
						return
					}
					for _, s := range m.systems {
						if _, e := s.GetInstalledVersion(); e == nil {
							if err = s.Clean(ctx); err != nil {
								return
							}
						}
					}
					path := basepath.EnjenvPath
					if bePath.IsDir(path) {
						bePath.ChmodAll(path)
						err = os.RemoveAll(path)
						io.StdoutF("# cleaned: %v\n", path)
					}
					return
				},
			},
		)
	}

	if atLeastOneInstalled {
		commands = append(
			commands,
			&cli.Command{
				HideHelpCommand: true,
				Name:            "export",
				Category:        ShellCategory,
				Usage:           "output shell export statements",
				Action: func(ctx *cli.Context) (err error) {
					io.StdoutF("export ENJENV_PATH=\"%v\";\n", basepath.EnjenvPath)
					for _, s := range m.systems {
						if err = s.Prepare(ctx); err != nil {
							return
						}
						if err = s.Export(ctx); err != nil {
							return
						}
					}
					for _, s := range m.systems {
						s.ExportPathVariable(false)
					}
					io.StdoutF("export TMPDIR=\"%v\"\n", basepath.MakeEnjenvPath(TmpDirName))
					io.StdoutF("export PATH=\"%v\"\n", strings.Join(env.GetPaths(), ":"))
					return
				},
			},
			&cli.Command{
				HideHelpCommand: true,
				Name:            "unexport",
				Category:        ShellCategory,
				Usage:           "output shell unset statements (inverse of export)",
				Action: func(ctx *cli.Context) (err error) {
					io.StdoutF("unset ENJENV_PATH;\n")
					for _, s := range m.systems {
						if err = s.Prepare(ctx); err != nil {
							return
						}
						if err = s.UnExport(ctx); err != nil {
							return
						}
					}
					for _, s := range m.systems {
						s.UnExportPathVariable(false)
					}
					io.StdoutF("unset TMPDIR;\n")
					io.StdoutF("export PATH=\"%v\"\n", strings.Join(env.GetPaths(), ":"))
					return
				},
			},
		)
	}

	commands = append(
		commands,
		&cli.Command{
			Name:      "git-tag",
			Category:  GeneralCategory + " utilities",
			Usage:     "derive a tagged version string (\"untagged\" if git fails)",
			UsageText: app.Name + " git-tag",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "untagged",
					Usage:   "specify the value to use for untagged cases",
					Value:   "untagged",
					Aliases: []string{"u"},
				},
			},
			Action: func(ctx *cli.Context) (err error) {
				if tag, ok := git.Describe(); ok {
					io.StdoutF("%v\n", tag)
				} else {
					io.StdoutF("%v\n", ctx.String("untagged"))
				}
				return
			},
		},
		&cli.Command{
			Name:      "rel-ver",
			Category:  GeneralCategory + " utilities",
			Usage:     "derive a Go-Enjin release version from 'git describe' output",
			UsageText: app.Name + " rel-ver",
			Description: `
rel-ver prints the currently determined release version value used in Go-Enjin projects.

Go-Enjin release versions are generated as follows:

	if "git status" results in any error:
		version is "release"

	if there are no changes in the git repo:
		version is "git rev-parse --short=10 HEAD"
		version is also prefixed with "c-"
		example: "c-a1b2c3d4e5"

	there are known changes in the git repo:
		version is the first 10 characters of a sha256sum of "git diff" output
		version is also prefixed with "d-"
		example: "d-a1b2c3d4e5"

The additional flags available for customizing this value are for use in generic
cases outside the Go-Enjin project.
`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "r-value",
					Usage: "specify the release-value to use",
					Value: "release",
				},
				&cli.StringFlag{
					Name:  "c-prefix",
					Usage: "specify the commit-value to prefix with",
					Value: "c",
				},
				&cli.StringFlag{
					Name:  "d-prefix",
					Usage: "specify the diff-value to prefix with",
					Value: "d",
				},
				&cli.BoolFlag{
					Name:  "no-prefix",
					Usage: "disable value prefixing completely",
				},
				&cli.BoolFlag{
					Name:  "no-c-prefix",
					Usage: "disable commit-value prefixing",
				},
				&cli.BoolFlag{
					Name:  "no-d-prefix",
					Usage: "disable diff-value prefixing",
				},
			},
			Action: func(ctx *cli.Context) (err error) {
				var r, c, d string
				r = ctx.String("r-value")
				if !ctx.Bool("no-prefix") {
					if !ctx.Bool("no-c-prefix") {
						c = ctx.String("c-prefix")
					}
					if !ctx.Bool("no-d-prefix") {
						d = ctx.String("d-prefix")
					}
				}
				io.StdoutF("%v\n", git.MakeCustomVersion(r, c, d))
				return
			},
		},
	)

	app.Commands = append(app.Commands, commands...)

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))
	return
}