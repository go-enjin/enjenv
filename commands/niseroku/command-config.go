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

func (c *Command) actionConfig(ctx *cli.Context) (err error) {
	if err = c.CCommand.Prepare(ctx); err != nil {
		return
	}

	var argc int
	if argc = ctx.NArg(); argc == 0 {
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