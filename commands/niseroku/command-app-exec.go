// Copyright (c) 2023  The Go-Enjin Authors
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

	"github.com/urfave/cli/v2"

	"github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

func makeCommandAppExec(c *Command, app *cli.App) (cmd *cli.Command) {
	cmd = &cli.Command{
		Name:      "exec",
		Usage:     "run a command within a slug",
		UsageText: app.Name + " niseroku app exec <app-name> [cmd [argv...]]",
		Action:    c.actionAppExec,
	}
	return
}

func (c *Command) actionAppExec(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	io.LogFile = ""
	if argc := ctx.NArg(); argc < 1 {
		cli.ShowCommandHelpAndExit(ctx, "exec", 1)
	}

	// drop privileges
	if err = common.DropPrivilegesTo(c.config.RunAs.User, c.config.RunAs.Group); err != nil {
		err = fmt.Errorf("error dropping root privileges: %v", err)
		return
	}

	cliArgv := ctx.Args().Slice()
	appName := cliArgv[0]

	var name string
	var argv []string

	if argc := len(cliArgv); argc > 1 {
		name = cliArgv[1]
		if argc > 2 {
			argv = cliArgv[2:]
		}
	}

	var ok bool
	var app *Application

	if app, ok = c.config.Applications[appName]; !ok {
		err = fmt.Errorf("app not found: %v", appName)
		return
	}

	if app.IsDeploying() {
		err = fmt.Errorf("application deployment in progress: %v", appName)
		return
	}

	slug := app.GetThisSlug()
	if slug == nil {
		err = fmt.Errorf("app has no slugs: %v", appName)
		return
	}

	if name == "" {
		err = slug.StartShell()
		return
	}
	err = slug.StartCommand(name, argv...)
	return
}