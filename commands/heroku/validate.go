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

	"github.com/urfave/cli/v2"

	"github.com/go-corelibs/path"
	"github.com/go-enjin/be/pkg/slug"

	"github.com/go-enjin/enjenv/pkg/io"
)

func (c *Command) makeValidateSlugCommand(appNamePrefix string) *cli.Command {
	return &cli.Command{
		Name:      "validate-slug",
		Category:  c.TagName,
		Usage:     "use the present Slugsums to validate the current slug environment",
		UsageText: appNamePrefix + " validate-slug",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "verbose",
				Usage:   "output more detailed information",
				Aliases: []string{"v"},
			},
			&cli.BoolFlag{
				Name:    "strict",
				Usage:   "extraneous files are considered an error",
				Aliases: []string{"s"},
			},
		},
		Action: func(ctx *cli.Context) (err error) {
			if slugsums := path.FindFileRelativeToPwd("Slugsums"); slugsums == "" {
				err = fmt.Errorf("missing Slugsums")
				return
			}
			var imposters, extraneous, validated []string
			if imposters, extraneous, validated, err = slug.ValidateSlugsums(); err != nil {
				return
			}
			il := len(imposters)
			el := len(extraneous)
			vl := len(validated)
			failed := il != 0 || (ctx.Bool("strict") && el != 0)
			if ctx.Bool("verbose") {
				for _, file := range imposters {
					io.StderrF("# imposter: %v\n", file)
				}
				for _, file := range extraneous {
					io.StderrF("# extraneous: %v\n", file)
				}
				for _, file := range validated {
					io.StderrF("# validated: %v\n", file)
				}
				io.NotifyF("finalize-slug", "Summary: %d imposters, %d extraneous, %v validated", il, el, vl)
			}
			if failed {
				if il > 0 && el > 0 {
					err = fmt.Errorf("%d imposters and %d extraneaous files found", len(imposters), len(extraneous))
				} else if il > 0 {
					err = fmt.Errorf("%d imposters found", len(imposters))
				} else if el > 0 {
					err = fmt.Errorf("%d extraneaous files found", len(extraneous))
				}
				return
			}
			io.NotifyF("finalize-slug", "Slugsums validated successfully")
			return
		},
	}
}
