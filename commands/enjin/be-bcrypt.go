// Copyright (c) 2022  The Go-Enjin Authors
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

package enjin

import (
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/go-enjin/enjenv/pkg/io"
)

func (c *Command) makeBeHtpasswdCommand(appNamePrefix string) *cli.Command {
	return &cli.Command{
		Name:      "be-bcrypt",
		Category:  c.TagName,
		Usage:     "generate a bcrypt hash",
		UsageText: appNamePrefix + " be-bcrypt <passwd>",
		Description: `
Generate a bcrypt password hash from password given, used with the Go-Enjin
basic-auth feature's environment variable users setting.
`,
		Action: func(ctx *cli.Context) (err error) {
			if err = c.Prepare(ctx); err != nil {
				return
			}
			if ctx.NArg() != 1 {
				cli.ShowSubcommandHelpAndExit(ctx, 1)
			}
			arg := ctx.Args().Slice()[0]
			var hashed []byte
			if hashed, err = bcrypt.GenerateFromPassword([]byte(arg), bcrypt.DefaultCost); err != nil {
				return
			}
			io.StdoutF("%v\n", string(hashed))
			return
		},
	}
}