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
	"github.com/urfave/cli/v2"

	beIo "github.com/go-enjin/enjenv/pkg/io"
)

func (c *Command) actionProxyControlCommand(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	argv := ctx.Args().Slice()
	if len(argv) < 1 {
		cli.ShowCommandHelpAndExit(ctx, "cmd", 1)
	}
	name := argv[0]
	argv = argv[1:]
	var response string
	if response, err = c.config.CallProxyControlCommand(name, argv...); err != nil {
		return
	}
	beIo.STDOUT("%v", response)
	return
}