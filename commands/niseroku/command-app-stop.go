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

	"github.com/go-corelibs/maps"

	"github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

func makeCommandAppStop(c *Command, app *cli.App) (cmd *cli.Command) {
	cmd = &cli.Command{
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
	}
	return
}

func (c *Command) actionAppStop(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	io.LogFile = ""

	var appNames []string
	if all := ctx.Bool("all"); all {
		appNames = maps.SortedKeys(c.config.Applications)
	} else if !all && ctx.NArg() >= 1 {
		appNames = ctx.Args().Slice()
	} else {
		cli.ShowSubcommandHelpAndExit(ctx, 1)
	}

	if err = common.DropPrivilegesTo(c.config.RunAs.User, c.config.RunAs.Group); err != nil {
		err = fmt.Errorf("error dropping root privileges: %v", err)
		return
	}

	var ok bool
	for _, name := range appNames {
		var app *Application
		if app, ok = c.config.Applications[name]; !ok {
			io.STDERR("application not found: %v\n", name)
			continue
		}

		if app.ThisSlug == "" && app.NextSlug == "" {
			io.STDOUT("application slugs not found: %v\n", name)
			continue
		}

		stopped := 0
		for _, slug := range []*Slug{app.GetThisSlug(), app.GetNextSlug()} {
			if slug != nil {
				stopped += slug.StopAll()
			}
		}

		app.Cleanup()

		if stopped > 0 {
			io.STDOUT("application stopped: %v (workers stopped: %d)\n", app.Name, stopped)
		} else {
			io.STDOUT("application not running: %v\n", app.Name)
		}
	}

	return
}
