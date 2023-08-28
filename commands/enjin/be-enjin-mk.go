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

package enjin

import (
	"fmt"
	"regexp"

	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/_templates"
)

var (
	rxEnjinMkVersion = regexp.MustCompile(`(?m)^ENJIN_MK_VERSION := (v[0-9]+\.[0-9]+(\.[0-9]+)??)\s*$`)
)

func (c *Command) makeBeEnjinMkCommand(appNamePrefix string) *cli.Command {
	if !rxEnjinMkVersion.MatchString(_templates.EnjinMk) {
		return nil
	}
	m := rxEnjinMkVersion.FindAllStringSubmatch(_templates.EnjinMk, 1)
	return &cli.Command{
		Name:      "be-enjin-mk",
		Category:  c.TagName,
		Usage:     "Print Enjin.mk (" + m[0][1] + ") to STDOUT",
		UsageText: appNamePrefix + " be-enjin-mk",
		Description: `
Prints a new feature.CFeature implementation.
`,
		Action: func(ctx *cli.Context) (err error) {
			if err = c.Prepare(ctx); err != nil {
				return
			}

			fmt.Println(_templates.EnjinMk)
			return
		},
	}
}