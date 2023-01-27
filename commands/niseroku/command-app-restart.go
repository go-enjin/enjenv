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
	"time"

	"github.com/go-enjin/be/pkg/maps"
	"github.com/urfave/cli/v2"

	"github.com/go-enjin/enjenv/pkg/io"
	pkgRun "github.com/go-enjin/enjenv/pkg/run"
)

func (c *Command) actionAppRestart(ctx *cli.Context) (err error) {
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
		cli.ShowCommandHelpAndExit(ctx, "restart", 1)
	}
	forceOverride := ctx.Bool("force")

	if err = c.dropPrivileges(); err != nil {
		err = fmt.Errorf("error dropping root privileges: %v", err)
		return
	}

	restartApp := func(app *Application) (err error) {
		app.NextSlug = app.ThisSlug
		if ee := app.Save(); ee != nil {
			err = fmt.Errorf("error saving %v application config: %v\n", app.Name, ee)
		} else if _, _, eee := pkgRun.EnjenvCmd("niseroku", "app", "start", app.Name); eee != nil {
			err = fmt.Errorf("error starting %v application: %v\n", app.Name, eee)
		}
		return
	}

	for _, name := range appNames {
		if app, ok := c.config.Applications[name]; !ok {
			io.STDERR("%v application not found\n", name)
		} else if app.Maintenance && !forceOverride {
			io.STDOUT("%v application in maintenance mode (use --force to override)\n", name)
		} else if ee := restartApp(app); ee != nil {
			io.STDERR("%v application restart error: %v\n", name, ee)
		} else {
			io.STDOUT("%v application restarting\n", name)
		}
		time.Sleep(100 * time.Millisecond) // slight delay before next app is restarted
	}

	return
}