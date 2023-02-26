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
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/cli/git"
	"github.com/go-enjin/be/pkg/path"
	"github.com/go-enjin/be/pkg/slug"

	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/io"
)

func (c *Command) makeFinalizeSlugCommand(appNamePrefix string) *cli.Command {
	return &cli.Command{
		Name:      "finalize-slug",
		Category:  c.TagName,
		Usage:     "use the present Slugfile to prepare and finalize a heroku slug environment",
		UsageText: appNamePrefix + " finalize-slug [options]",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "force",
				Usage: "confirm the destructive process",
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Usage:   "output detailed information",
				Aliases: []string{"v"},
			},
		},
		Action: func(ctx *cli.Context) (err error) {
			if slugfile := path.FindFileRelativeToPwd("Slugfile"); slugfile == "" {
				err = fmt.Errorf("missing Slugfile")
				return
			}
			if !ctx.IsSet("force") {
				err = fmt.Errorf("cannot finalize-slug without --force")
				return
			}
			if basepath.EnjenvIsInPwd() {
				err = fmt.Errorf("cannot finalize-slug: found .enjenv in slug, use '%v clean'", io.BinName)
				return
			}
			if git.IsRepo() {
				err = fmt.Errorf("cannot finalize-slug when .git is present")
				return
			}

			io.NotifyF("finalize-slug", "finalizing slug environment")

			var slugsums string
			var removed []string
			if slugsums, removed, err = slug.FinalizeSlugfile(ctx.Bool("force")); err != nil {
				return
			}

			count := len(strings.Split(slugsums, "\n"))

			if lr := len(removed); lr > 0 {
				io.StdoutF("# removed %d extraneous paths\n", lr)
			}

			io.NotifyF("finalize-slug", "Wrote %d Slugsums:\n%v", count, slugsums)
			io.NotifyF("finalize-slug", "slug environment finalized")
			return
		},
	}
}