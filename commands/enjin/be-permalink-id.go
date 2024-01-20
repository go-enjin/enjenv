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
	"github.com/gofrs/uuid"
	"github.com/urfave/cli/v2"

	"github.com/go-enjin/enjenv/pkg/io"
)

func (c *Command) makeBePermalinkIdCommand(appNamePrefix string) *cli.Command {
	return &cli.Command{
		Name:      "be-permalink-id",
		Category:  c.TagName,
		Usage:     "Generate a new page permalink id",
		UsageText: appNamePrefix + " be-permalink-id",
		Description: `
Prints a new permalink id.
`,
		Action: func(ctx *cli.Context) (err error) {
			if err = c.Prepare(ctx); err != nil {
				return
			}
			var id uuid.UUID
			if id, err = uuid.NewV4(); err != nil {
				return
			}
			io.StdoutF("%v\n", id)
			return
		},
	}
}
