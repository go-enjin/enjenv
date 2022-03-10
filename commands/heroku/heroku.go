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

package heroku

import (
	"fmt"
	"os"
	"regexp"

	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/cli/run"
	bePath "github.com/go-enjin/be/pkg/path"
	"github.com/go-enjin/enjenv/pkg/system"
)

var (
	rxSlugfileLine = regexp.MustCompile(`^\s*([^/].+?)\s*$`)
	rxSlugsumsLine = regexp.MustCompile(`(?ms)^\s*([0-9a-f]{64})\s*([^/].+?)\s*$`)
)

const (
	Name = "heroku-cli"
)

type Command struct {
	system.CCommand
}

func New() (s *Command) {
	s = new(Command)
	s.Init(s)
	return
}

func (c *Command) Init(this interface{}) {
	c.CCommand.Init(this)
	c.TagName = Name
	return
}

func (c *Command) ExtraCommands(app *cli.App) (commands []*cli.Command) {
	commands = append(
		commands,
		c.makeValidateSlugCommand(app.Name),
		c.makeFinalizeSlugCommand(app.Name),
		c.makeWriteSlugfileCommand(app.Name),
		c.makeDeploySlugCommand(app.Name),
		c.makeBuildpackCommand(app.Name),
	)
	return
}

func (c *Command) makeExe(argv ...string) (status int, err error) {
	status, err = run.Exe("make", argv...)
	return
}

func (c *Command) enjenvExe(argv ...string) (status int, err error) {
	self := os.Args[0]
	if len(self) > 3 {
		if self[0:2] == "./" || self[0:3] == "../" || self[0] == '/' {
			if self, err = bePath.Abs(self); err != nil {
				err = fmt.Errorf("error getting absolute path to: %v", os.Args[0])
				return
			}
		}
	}
	status, err = run.Exe(os.Args[0], argv...)
	return
}