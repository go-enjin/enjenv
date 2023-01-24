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
)

func (c *Command) actionAppStop(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	io.LogFile = ""
	if argc := ctx.NArg(); argc > 1 {
		err = fmt.Errorf("too many arguments")
		return
	} else if argc < 1 {
		cli.ShowCommandHelpAndExit(ctx, "start", 1)
	}
	appName := ctx.Args().Get(0)

	var ok bool
	var app *Application
	var slug *Slug

	if err = c.dropPrivileges(); err != nil {
		err = fmt.Errorf("error dropping root privileges: %v", err)
		return
	}

	if app, ok = c.config.Applications[appName]; !ok {
		err = fmt.Errorf("app not found: %v", appName)
		return
	}

	if slug = app.GetThisSlug(); slug != nil {
		if slug.Stop() {
			io.STDOUT("slug process stopped: %v\n", app.Name)
		} else {
			io.STDOUT("slug process already stopped: %v\n", app.Name)
		}
	} else {
		err = fmt.Errorf("%v - slug not found", app.Name)
	}

	return
}