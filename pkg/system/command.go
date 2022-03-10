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

package system

import (
	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/context"

	"github.com/go-enjin/enjenv/pkg/io"
)

var _ Command = (*CCommand)(nil)

type Command interface {
	Init(this interface{})
	Name() (name string)
	This() (self Command)
	Setup(app *cli.App) (err error)
	Prepare(ctx *cli.Context) (err error)
	ExtraCommands(app *cli.App) (commands []*cli.Command)
}

type CCommand struct {
	_this interface{}

	TagName string

	App *cli.App
	Ctx context.Context
}

func (c *CCommand) Init(this interface{}) {
	c._this = this
	c.Ctx = context.New()
}

func (c *CCommand) Name() (name string) {
	name = c.TagName
	return
}

func (c *CCommand) Context() (ctx context.Context) {
	ctx = c.Ctx.Copy()
	return
}

func (c *CCommand) This() (self Command) {
	if v, ok := c._this.(System); ok {
		self = v
		return
	}
	self = c
	return
}

func (c *CCommand) Setup(app *cli.App) (err error) {
	c.Ctx = context.New()
	c.App = app
	return
}

func (s *CCommand) Prepare(ctx *cli.Context) (err error) {
	_ = io.SetupCustomIndent(ctx)
	if err = io.SetupSlackIfPresent(ctx); err != nil {
		return
	}
	return
}

func (s *CCommand) ExtraCommands(app *cli.App) (commands []*cli.Command) {
	return
}