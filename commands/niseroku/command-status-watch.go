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

	"github.com/go-enjin/enjenv/pkg/profiling"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

var (
	DefaultStatusWatchUpdateFrequency = time.Second
)

func makeCommandStatusWatch(c *Command, app *cli.App) (cmd *cli.Command) {
	cmd = &cli.Command{
		Name:      "watch",
		Usage:     "continually display the status of all niseroku services",
		UsageText: app.Name + " niseroku status watch",
		Action:    c.actionStatusWatch,
		Flags: []cli.Flag{
			&cli.DurationFlag{
				Name:    "update-frequency",
				Usage:   "time.Duration between update cycles",
				Aliases: []string{"n"},
			},
			&cli.PathFlag{
				Name:   "tty-path",
				Usage:  "specify the TTY path for go-curses",
				Value:  "/dev/tty",
				Hidden: true,
			},
		},
	}
	return
}

func (c *Command) actionStatusWatch(ctx *cli.Context) (err error) {
	profiling.Start()
	defer profiling.Stop()

	if err = c.Prepare(ctx); err != nil {
		return
	}
	if err = common.DropPrivilegesTo(c.config.RunAs.User, c.config.RunAs.Group); err != nil {
		return
	}
	var freq time.Duration
	if ctx.IsSet("update-frequency") {
		freq = ctx.Duration("update-frequency")
	} else {
		freq = DefaultStatusWatchUpdateFrequency
	}
	var sw *StatusWatch
	if sw, err = NewStatusWatch(c, freq, ctx.Path("tty-path")); err != nil {
		return
	}
	err = sw.Run(ctx)
	return
}