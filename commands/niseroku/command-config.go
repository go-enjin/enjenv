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
	"strings"

	"github.com/urfave/cli/v2"

	beIo "github.com/go-enjin/enjenv/pkg/io"
)

func makeCommandConfig(c *Command, app *cli.App) (cmd *cli.Command) {
	cmd = &cli.Command{
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
	}
	return
}

func (c *Command) actionConfig(ctx *cli.Context) (err error) {

	if path := ctx.Path("init-default"); path != "" {
		err = WriteDefaultConfig(path)
		return
	}

	if err = c.CCommand.Prepare(ctx); err != nil {
		return
	}

	resetComments := ctx.Bool("reset-comments")

	var argc int
	if argc = ctx.NArg(); argc == 0 && !resetComments {
		err = c.actionConfigDisplayFull(ctx)
		return
	}

	argv := ctx.Args().Slice()
	if argc == 1 {
		err = c.actionConfigDisplay(ctx, argv[0])
		return
	} else if argc == 2 {
		err = c.actionConfigSet(ctx, argv[0], argv[1])
		return
	}

	if ctx.Bool("reset-comments") {
		if c.config, err = c.findConfig(ctx); err != nil {
			return
		} else if err = c.config.Save(false); err == nil {
			beIo.STDOUT("OK\n")
		}
		return
	}

	cli.ShowCommandHelpAndExit(ctx, "config", 1)
	return
}

func (c *Command) actionConfigDisplayFull(ctx *cli.Context) (err error) {
	if c.config, err = c.findConfig(ctx); err != nil {
		return
	}
	for _, key := range c.config.tomlMetaData.Keys() {
		tk := key.String()
		tt := c.config.tomlMetaData.Type(strings.Split(tk, ".")...)
		if tv := c.config.GetTomlValue(tk); tv != nil {
			switch tt {
			case "String":
				beIo.STDOUT("%v = \"%v\"\n", tk, tv)
			default:
				beIo.STDOUT("%v = %v\n", tk, tv)
			}
		}
	}
	return
}

func (c *Command) actionConfigDisplay(ctx *cli.Context, given string) (err error) {
	if c.config, err = c.findConfig(ctx); err != nil {
		return
	}
	for _, key := range c.config.tomlMetaData.Keys() {
		if tk := key.String(); tk == given {
			if tv := c.config.GetTomlValue(tk); tv != nil {
				beIo.STDOUT("%v\n", tv)
			}
			return
		}
	}
	err = fmt.Errorf("key not found")
	return
}

func (c *Command) actionConfigSet(ctx *cli.Context, givenKey, givenValue string) (err error) {
	if c.config, err = c.findConfig(ctx); err != nil {
		return
	}
	for _, key := range c.config.tomlMetaData.Keys() {
		if tk := key.String(); tk == givenKey {
			if err = c.config.SetTomlValue(tk, givenValue); err == nil {
				if err = c.config.Save(!ctx.Bool("reset-comments")); err == nil {
					beIo.STDOUT("OK\n")
				}
			}
			return
		}
	}
	err = fmt.Errorf("key not found")
	return
}