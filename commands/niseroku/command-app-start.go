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
	"time"

	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/maps"

	"github.com/go-enjin/enjenv/pkg/io"
)

func (c *Command) actionAppStart(ctx *cli.Context) (err error) {
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
		cli.ShowCommandHelpAndExit(ctx, "start", 1)
	}

	forceOverride := ctx.Bool("force")

	for _, name := range appNames {
		if app, ok := c.config.Applications[name]; !ok {
			io.STDERR("application not found: %v\n", name)
		} else if app.Maintenance && !forceOverride {
			io.STDOUT("application in maintenance mode: %v (use --force to override)\n", name)
		} else if app.ThisSlug == "" && app.NextSlug == "" {
			io.STDERR("application slugs not found: %v\n", name)
		} else if app.IsDeploying() {
			io.STDERR("application deployment in progress: %v\n", name)
		} else if app.IsRunningReady() && !forceOverride {
			io.STDERR("application already running ready: %v\n", name)
		} else if ee := app.Invoke(); ee != nil {
			io.STDERR("application invoke error: %v - %v\n", name, ee)
		} else {
			io.STDOUT("application started: %v\n", name)
		}
		time.Sleep(100 * time.Millisecond) // slight delay before next app is started
	}

	return
}