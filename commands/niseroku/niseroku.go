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

package niseroku

import (
	"fmt"
	"io"
	"log"

	"github.com/urfave/cli/v2"

	bePath "github.com/go-enjin/be/pkg/path"

	beIo "github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/system"
)

const (
	Name           = "niseroku"
	ConfigFileName = Name + ".toml"
)

var DefaultConfigLocations = []string{
	"/etc/niseroku/" + ConfigFileName,
	"./" + ConfigFileName,
}

func init() {
	log.SetOutput(io.Discard)
}

type Command struct {
	system.CCommand

	config *Config
}

func New() (s *Command) {
	s = new(Command)
	s.Init(s)
	return
}

func (c *Command) Init(this interface{}) {
	c.CCommand.Init(this)
	c.TagName = Name
	return
}

func (c *Command) ExtraCommands(app *cli.App) (commands []*cli.Command) {
	commands = append(
		commands,
		&cli.Command{
			Name: "niseroku",
			Description: `
niseroku is a small-scale hosting service for one or more Go-Enjin projects.

For "at-scale" things please use something else. Niseroku is intended for
supporting small-scale shared-deployment situations such as a collection of
Go-Enjin blog websites and so on using a cheap VPS.

A running niseroku instance has three main facets or aspects.

The first is a simple GIT remote repository system which receives "git push"
events and builds the repository with the assumption that the repository is a
Go-Enjin project.

The second is an enjin instance manager which starts, stops and handles
new slugs being deployed (after a "git push" event). When a new slug is
deployed, it is started up with a new randomly selected port (within a
configurable range) as a sub-process. When that sub-process finally opens
the specified random port (with 5 minute timeout), updates the app repo port
setting to point to the new port and destroys the previous slug instance. If
there is any error or the sub-process fails to open the port within the 5
minute window, the new slug is destroyed and the existing one remains running.

The last aspect is an HTTP(S) reverse-proxy that listens on ports 80 and 443,
using Let's Encrypt for SSL certificates. Inbound requests are re-requested to
a configured app repo instance and the responses returned as-if the request was
handled directly.
`,
			Flags: []cli.Flag{
				&cli.PathFlag{
					Name:    "config",
					Usage:   "specify path to niseroku.toml",
					EnvVars: []string{"ENJENV_NISEROKU_CONFIG_PATH"},
				},
			},
			Subcommands: []*cli.Command{
				{
					Name:      "reverse-proxy",
					Usage:     "niseroku reverse-proxy service",
					UsageText: app.Name + " niseroku reverse-proxy",
					Action:    c.actionReverseProxy,
					Subcommands: []*cli.Command{
						{
							Name:      "reload",
							Usage:     "reload reverse-proxy services",
							UsageText: app.Name + " niseroku reverse-proxy reload",
							Action:    c.actionReverseProxyReload,
						},
						{
							Name:      "cmd",
							Usage:     "run proxy-control commands",
							UsageText: app.Name + " niseroku reverse-proxy cmd <name> [argv...]",
							Action:    c.actionProxyControlCommand,
							Hidden:    true,
						},
					},
				},
				{
					Name:      "git-repository",
					Usage:     "niseroku git-repository service",
					UsageText: app.Name + " niseroku git-repository",
					Action:    c.actionGitRepository,
					Subcommands: []*cli.Command{
						{
							Name:      "reload",
							Usage:     "reload git-repository services",
							UsageText: app.Name + " niseroku git-repository reload",
							Action:    c.actionGitRepositoryReload,
						},
					},
				},
				{
					Name:      "reload",
					Usage:     "reload all niseroku services",
					UsageText: app.Name + " niseroku reload",
					Action:    c.actionReload,
				},
				{
					Name:      "status",
					Usage:     "display the status of all niseroku services",
					UsageText: app.Name + " niseroku status",
					Action:    c.actionStatus,
					Subcommands: []*cli.Command{
						{
							Name:      "watch",
							Usage:     "continually display the status of all niseroku services",
							UsageText: app.Name + " niseroku status watch",
							Action:    c.actionStatusWatch,
						},
					},
				},
				{
					Name:      "config",
					Usage:     "get, set and test configuration settings",
					UsageText: app.Name + " niseroku config [key] [value]",
					Description: `
With no arguments specified, displays all the settings where each key is output
in a toml-specific format. For example: "ports.git = 2403".

With just the key argument, displays the value of that key.

When both key and value are given, applies the value to the configuration
setting. Prints "OK" if no value parsing or config file saving errors occurred.
`,
					Action: c.actionConfig,
					Subcommands: []*cli.Command{
						{
							Name:        "test",
							Usage:       "test the current config file for syntax and other errors",
							UsageText:   app.Name + " niseroku config test",
							Description: `Prints "OK" if no parsing errors occurred.`,
							Action:      c.actionConfigTest,
						},
					},
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:  "reset-comments",
							Usage: "restore default comments on config save",
						},
						&cli.PathFlag{
							Name:  "init-default",
							Usage: "write a default niseroku.toml file",
						},
					},
				},
				{
					Name:      "deploy-slug",
					Usage:     "deploy a built slug",
					UsageText: "niseroku deploy-slug <slug.zip> [slugs.zip...]",
					Action:    c.actionDeploySlug,
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:  "verbose",
							Usage: "use STDOUT and STDERR for command logging",
						},
					},
				},
				{
					Name:      "fix-fs",
					Usage:     "repair file ownership and modes",
					UsageText: app.Name + " niseroku fix-fs",
					Action:    c.actionFixFs,
				},
				{
					Name:  "app",
					Usage: "manage specific enjin applications",
					Subcommands: []*cli.Command{

						{
							Name:   "git-pre-receive-hook",
							Action: c.actionGitPreReceiveHook,
							Hidden: true,
						},

						{
							Name:   "git-post-receive-hook",
							Action: c.actionGitPostReceiveHook,
							Hidden: true,
						},

						{
							Name:      "run",
							Usage:     "run an app slug process",
							UsageText: app.Name + " niseroku app run <name>",
							Action:    c.actionAppRun,
							Flags: []cli.Flag{
								&cli.BoolFlag{
									Name:   "slug-process",
									Hidden: true,
								},
							},
						},

						{
							Name:      "start",
							Usage:     "start one or more applications",
							UsageText: app.Name + " niseroku app start <name> [name...]",
							Action:    c.actionAppStart,
							Flags: []cli.Flag{
								&cli.BoolFlag{
									Name:  "all",
									Usage: "start all applications",
								},
								&cli.BoolFlag{
									Name:  "force",
									Usage: "start regardless of maintenance mode",
								},
							},
						},

						{
							Name:      "stop",
							Usage:     "stop one or more running applications",
							UsageText: app.Name + " niseroku app stop <name> [name...]",
							Action:    c.actionAppStop,
							Flags: []cli.Flag{
								&cli.BoolFlag{
									Name:  "all",
									Usage: "stop all applications",
								},
							},
						},

						{
							Name:      "restart",
							Usage:     "restart one or more applications",
							UsageText: app.Name + " niseroku app restart <name> [name...]",
							Action:    c.actionAppRestart,
							Flags: []cli.Flag{
								&cli.BoolFlag{
									Name:  "all",
									Usage: "restart all applications",
								},
							},
						},

						{
							Name:      "rename",
							Usage:     "rename an application",
							UsageText: app.Name + " niseroku app rename <old> <new>",
							Action:    c.actionAppRename,
						},
					},
				},
			},
		},
	)
	return
}

func (c *Command) findConfig(ctx *cli.Context) (config *Config, err error) {
	var path string
	if path = ctx.String("config"); path == "" {
		for _, check := range DefaultConfigLocations {
			if bePath.IsFile(check) {
				path = check
			}
		}
	}
	if path == "" {
		err = fmt.Errorf("%v not found, please use --config", ConfigFileName)
		return
	}
	if path, err = bePath.Abs(path); err != nil {
		return
	}
	config, err = LoadConfig(path)
	return
}

func (c *Command) Prepare(ctx *cli.Context) (err error) {
	if err = c.CCommand.Prepare(ctx); err != nil {
		return
	}
	if c.config, err = c.findConfig(ctx); err != nil {
		return
	}
	if c.config.LogFile != "" {
		beIo.LogFile = c.config.LogFile
	}
	return
}