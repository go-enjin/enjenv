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

	"github.com/go-enjin/be/pkg/maps"

	"github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

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
		cli.ShowCommandHelpAndExit(ctx, "stop", 1)
	}

	if err = common.DropPrivilegesTo(c.config.RunAs.User, c.config.RunAs.Group); err != nil {
		err = fmt.Errorf("error dropping root privileges: %v", err)
		return
	}

	for _, name := range appNames {
		if app, ok := c.config.Applications[name]; !ok {
			io.STDERR("%v application not found\n", name)
		} else {
			found := false
			stopped := 0
			if slug := app.GetThisSlug(); slug != nil {
				found = true
				stopped += slug.StopAll()
			}
			if slug := app.GetNextSlug(); slug != nil {
				found = true
				stopped += slug.StopAll()
			}
			if !found {
				io.STDERR("%v this slug not found\n", name)
			} else if stopped > 0 {
				io.STDOUT("%v stopped %d instances\n", app.Name, stopped)
			} else {
				io.STDOUT("%v not running\n", app.Name)
			}
		}
	}

	return
}