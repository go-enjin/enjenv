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
	"github.com/go-enjin/enjenv/pkg/profiling"
)

func makeCommandReverseProxy(c *Command, app *cli.App) (cmd *cli.Command) {
	cmd = &cli.Command{
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
				Name:      "stop",
				Usage:     "stop reverse-proxy services",
				UsageText: app.Name + " niseroku reverse-proxy stop",
				Action:    c.actionReverseProxyStop,
			},
			{
				Name:      "cmd",
				Usage:     "run proxy-control commands",
				UsageText: app.Name + " niseroku reverse-proxy cmd <name> [argv...]",
				Action:    c.actionProxyControlCommand,
				Hidden:    true,
			},
		},
	}
	return
}

func (c *Command) actionReverseProxy(ctx *cli.Context) (err error) {
	profiling.Start()

	if err = c.Prepare(ctx); err != nil {
		return
	}
	rp := NewReverseProxy(c.config)
	if rp.IsRunning() {
		err = fmt.Errorf("reverse-proxy already running")
		return
	}
	err = rp.Start()
	return
}

func (c *Command) actionProxyControlCommand(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	argv := ctx.Args().Slice()
	if len(argv) < 1 {
		cli.ShowSubcommandHelpAndExit(ctx, 1)
	}
	name := argv[0]
	argv = argv[1:]
	var response string
	if response, err = c.config.CallProxyControlCommand(name, argv...); err != nil {
		return
	}
	io.STDOUT("%v", response)
	return
}