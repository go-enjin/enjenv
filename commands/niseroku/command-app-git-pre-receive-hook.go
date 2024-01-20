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
	"os"

	"github.com/sosedoff/gitkit"
	"github.com/urfave/cli/v2"

	pkgIo "github.com/go-enjin/enjenv/pkg/io"
)

func makeCommandAppGitPreReceiveHook(c *Command, app *cli.App) (cmd *cli.Command) {
	cmd = &cli.Command{
		Name:   "git-pre-receive-hook",
		Action: c.actionAppGitPreReceiveHook,
		Hidden: true,
	}
	return
}

func (c *Command) actionAppGitPreReceiveHook(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}

	pkgIo.STDOUT("# preparing slug building process\n")

	receiver := gitkit.Receiver{
		MasterOnly: false,
		TmpDir:     c.config.Paths.Tmp,
		HandlerFunc: func(info *gitkit.HookInfo, tmpPath string) (err error) {
			if _, err = c.enjinRepoGitHandlerSetup(c.config, info); err != nil {
				return
			}
			return
		},
	}

	err = receiver.Handle(os.Stdin)
	return
}
