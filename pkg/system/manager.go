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

	"github.com/urfave/cli/v2"

	"github.com/go-corelibs/env"
	clpath "github.com/go-corelibs/path"
	"github.com/go-enjin/be/pkg/cli/git"

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
	// nothing to do here anymore, leaving stub temporarily
}

func (m *SystemsManager) Setup(app *cli.App) (err error) {
	app.HideHelpCommand = true
	app.UsageText = "\n\t" + BinName + " command [command options]\n\n"
	app.UsageText += "\t# with no arguments, prints the current enjenv path\n"
	app.UsageText += "\t$ " + BinName + "\n"
	app.UsageText += "\t/path/to/.enjenv\n"

	app.Flags = append(
		app.Flags,
		&cli.StringFlag{
			Name:    "slack",
			Usage:   "send notifications the given slack channel as well as os.Stdout",
			EnvVars: []string{"ENJENV_SLACK"},
		},
		&cli.StringFlag{
			Name:    "custom-indent",
			Usage:   "include custom indentation with all output",
			EnvVars: []string{"ENJENV_CUSTOM_INDENT"},
		},
	)

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
				cli.ShowSubcommandHelpAndExit(ctx, 1)
				return
			},
			Subcommands: []*cli.Command{
				{
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
			if err = io.SetupSlackIfPresent(ctx); err != nil {
				return
			}
			if len(m.systems) == 0 {
				err = fmt.Errorf("no systems built-in, nothing to do")
				return
			}
			io.NotifyF("init", "all systems init started")
			for _, s := range m.systems {
				name := s.Self().Name()
				if ctx.IsSet(name) && ctx.String(name) != "" {
					if err = s.InitSystem(ctx); err != nil {
						return
					}
				}
			}
			io.NotifyF("init", "all systems init complete")
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

	if clpath.IsDir(basepath.EnjenvPath) {
		commands = append(
			commands,
			&cli.Command{
				Name:      "write-scripts",
				Category:  ShellCategory,
				Usage:     "create bash_completion, activate and deactivate shell scripts",
				UsageText: app.Name + " write-scripts",
				Action: func(ctx *cli.Context) (err error) {
					writeFile := func(name string, data string) (err error) {
						if err = os.WriteFile(name, []byte(data), 0660); err != nil {
							err = fmt.Errorf("error writing %v: %v", name, err)
							return
						}
						io.StdoutF("# wrote: %v\n", name)
						return
					}

					// functions
					if err = writeFile(basepath.MakeEnjenvPath("functions"), ShellFunctionsSource); err != nil {
						return
					}

					// bash_completion
					if err = writeFile(basepath.MakeEnjenvPath("bash_completion"), BashCompletionSource); err != nil {
						return
					}

					// activate (re-use script)
					content := "#: source enjenv common shell functions\n"
					content += `source "$(dirname "${BASH_SOURCE[0]}")/functions"`
					content += "\n\n"
					content += "#: ENJENV_PATH overrides the default ./.enjenv path location\n"
					content += fmt.Sprintf(`export ENJENV_PATH="%v"`+"\n", basepath.EnjenvPath)
					content += "#: TMPDIR is required (if not already present)\n"
					content += fmt.Sprintf(`[ -z "${TMPDIR}" ] && export TMPDIR="%v"`+"\n", basepath.MakeEnjenvPath(TmpDirName))
					systems := make([]System, 0)
					for _, s := range m.systems {
						if e := s.Self().Prepare(ctx); e != nil {
							_ = io.ErrorF("error preparing %v: %v\n", s.Self().Name(), e)
							continue
						}
						systems = append(systems, s)
						if v, e := s.Self().ExportString(ctx); e == nil {
							content += fmt.Sprintf("\n#: begin enjenv %v variables\n\n", s.Name())
							content += v
							content += fmt.Sprintf("\n\n#: end enjenv %v variables\n\n", s.Name())
						}
						if v := s.GetExportPaths(); len(v) > 0 {
							for _, p := range v {
								content += fmt.Sprintf(`_enjenv_add_path "%s"`+"\n", p)
							}
						}
					}
					if err = writeFile(basepath.MakeEnjenvPath("activate"), content); err != nil {
						return
					}

					// deactivate (re-use content, script)
					content = "#: source enjenv common shell functions\n"
					content += `source "$(dirname "${BASH_SOURCE[0]}")/functions"`
					content += "\n\n"
					content = fmt.Sprintf(`[ -n "${ENJENV_PATH}" ] && unset ENJENV_PATH;` + "\n")
					content += fmt.Sprintf(`[ "${TMPDIR}" == "%s" ] && unset TMPDIR`+"\n", basepath.MakeEnjenvPath(TmpDirName))
					for _, s := range systems {
						if v, e := s.Self().UnExportString(ctx); e == nil {
							content += v
						}
						if v := s.GetExportPaths(); len(v) > 0 {
							for _, p := range v {
								content += fmt.Sprintf(`_enjenv_rem_path "%s"`+"\n", p)
							}
						}
					}
					content += "_enjenv_unset_functions\n"
					if err = writeFile(basepath.MakeEnjenvPath("deactivate"), content); err != nil {
						return
					}

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
					if clpath.IsDir(path) {
						clpath.ChmodAll(path)
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
					io.StdoutF("export PATHS=\"%v\"\n", env.String("PATHS", ""))
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
					io.StdoutF("export PATHS=\"%v\"\n", env.String("PATHS", ""))
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
		&cli.Command{
			Name:      "features",
			Category:  GeneralCategory + " utilities",
			Usage:     "query enjenv for which capabilities are present in the current environment",
			UsageText: app.Name + " features [sub-commands]",
			Subcommands: []*cli.Command{
				{
					Name:      "list",
					Usage:     "list all available features, one per line",
					UsageText: app.Name + " features list",
					Action: func(ctx *cli.Context) (err error) {
						for _, name := range ListFeatures(app.Commands) {
							io.StdoutF("%v\n", name)
						}
						return
					},
				},
				{
					Name:      "has",
					Usage:     "check if the given names are all present, errors on first missing",
					UsageText: app.Name + " features has <name> [names...]",
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:  "not",
							Usage: "invert the output, reports true if not present",
						},
					},
					Action: func(ctx *cli.Context) (err error) {
						not := ctx.Bool("not")
						for _, arg := range ctx.Args().Slice() {
							if not {
								if HasFeature(arg, app.Commands) {
									err = fmt.Errorf("%v is present", arg)
									return
								}
								continue
							}
							if !HasFeature(arg, app.Commands) {
								err = fmt.Errorf("%v not present", arg)
								return
							}
						}
						return
					},
				},
			},
		},
	)

	app.Commands = append(app.Commands, commands...)

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))
	return
}
