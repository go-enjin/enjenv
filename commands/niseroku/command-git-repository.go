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

	"github.com/go-enjin/enjenv/pkg/profiling"
)

func (c *Command) actionGitRepository(ctx *cli.Context) (err error) {
	profiling.Start()

	if err = c.Prepare(ctx); err != nil {
		return
	}
	gr := NewGitRepository(c.config)
	if gr.IsRunning() {
		err = fmt.Errorf("git-repository already running")
		return
	}
	err = gr.Start()
	return
}