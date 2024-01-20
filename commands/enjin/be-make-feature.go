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
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"text/template"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/_templates"
	"github.com/go-enjin/be/features/pages/funcmaps"
	beContext "github.com/go-enjin/be/pkg/context"
)

func (c *Command) makeMakeFeatureCommand(appNamePrefix string) *cli.Command {
	return &cli.Command{
		Name:      "be-make-feature",
		Category:  c.TagName,
		Usage:     "Generate boilerplate feature package source code",
		UsageText: appNamePrefix + " make-feature [options] <pkg> <tag>",
		Description: `
Output a new feature.CFeature implementation.
`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "site-feature",
				Aliases: []string{"s"},
				Usage:   "produce a feature.SiteFeature",
			},
			&cli.BoolFlag{
				Name:    "with-header",
				Aliases: []string{"w"},
				Usage:   "include source license header comment",
			},
			&cli.StringFlag{
				Name:    "license",
				Aliases: []string{"l"},
				Usage:   "specify source header license (go-enjin or non-free)",
				Value:   "go-enjin",
			},
			&cli.StringFlag{
				Name:    "copyright",
				Aliases: []string{"c"},
				Usage:   "specify copyright notice to use",
				Value:   "The Go-Enjin Authors",
			},
		},
		Action: func(ctx *cli.Context) (err error) {
			if err = c.Prepare(ctx); err != nil {
				return
			}

			rpl := beContext.Context{
				"CurrentYear": time.Now().Year(),
			}

			argv := ctx.Args().Slice()
			argc := len(argv)

			switch {
			case argc == 0:
				cli.ShowSubcommandHelpAndExit(ctx, 1)
				return
			case argc == 1:
				rpl["PackageName"] = argv[0]
				rpl["FeatureTag"] = argv[0]
			case argc == 2:
				rpl["PackageName"] = argv[0]
				rpl["FeatureTag"] = argv[1]
			default:
				err = fmt.Errorf("too many arguments")
				return
			}

			fm := funcmaps.New().Defaults().Make().MakeFuncMap(rpl).AsTEXT()

			if ctx.Bool("with-header") {
				tt := template.New("header.go").Funcs(fm)
				switch ctx.String("license") {
				case "go-enjin":
					if tt, err = tt.Parse(_templates.LicenseGoEnjinGoTmpl); err != nil {
						return
					}
				case "non-free":
					if tt, err = tt.Parse(_templates.LicenseNonFreeGoTmpl); err != nil {
						return
					}
				default:
					err = fmt.Errorf(`--license must be one of: "go-enjin" or "non-free"`)
					return
				}

				if notice := ctx.String("copyright"); notice == "" {
					err = fmt.Errorf(`--with-header requires --copyright`)
					return
				} else {
					rpl["CopyrightNotice"] = notice
				}

				var buf bytes.Buffer
				if err = tt.Execute(&buf, rpl); err != nil {
					return
				}
				rpl["Copyright"] = buf.String()
			}

			var tmplContent string
			if ctx.Bool("site-feature") {
				tmplContent = _templates.SiteFeatureGoTmpl
			} else {
				tmplContent = _templates.FeatureGoTmpl
			}

			var tt *template.Template
			if tt, err = template.New("feature.go").Funcs(fm).Parse(tmplContent); err != nil {
				return
			}

			err = tt.Execute(os.Stdout, rpl)
			return
		},
	}
}
