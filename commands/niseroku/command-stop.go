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

func (c *Command) actionStop(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	if c.config.SignalStopReverseProxy() {
		beIo.STDOUT("stop signal sent to reverse-proxy\n")
	}
	if c.config.SignalStopGitRepository() {
		beIo.STDOUT("stop signal sent to git-repository\n")
	}
	return
}

func (c *Command) actionReverseProxyStop(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	if c.config.SignalStopReverseProxy() {
		beIo.STDOUT("stop signal sent to reverse-proxy\n")
	}
	return
}

func (c *Command) actionGitRepositoryStop(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	if c.config.SignalStopGitRepository() {
		beIo.STDOUT("stop signal sent to git-repository\n")
	}
	return
}