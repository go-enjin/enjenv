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
	"regexp"
	"sort"
	"strings"

	"github.com/fvbommel/sortorder"
	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/cli/git"
	bePath "github.com/go-enjin/be/pkg/path"
	"github.com/go-enjin/be/pkg/slug"
	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/io"
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

func (s *Command) Init(this interface{}) {
	s.CCommand.Init(this)
	s.TagName = Name
	return
}

func (s *Command) ExtraCommands(app *cli.App) (commands []*cli.Command) {
	commands = append(
		commands,
		s.makeValidateSlugCommand(app.Name),
		s.makeFinalizeSlugCommand(app.Name),
		s.makeWriteSlugfileCommand(app.Name),
	)
	return
}

func (s *Command) makeWriteSlugfileCommand(appNamePrefix string) *cli.Command {
	return &cli.Command{
		Name:      "write-slugfile",
		Category:  s.TagName,
		Usage:     "generate a simple Slugfile from the relative paths given",
		UsageText: appNamePrefix + " write-slugfile <path> [paths...]",
		Description: `
A Slugfile is a simple text file with one relative file path per line. This file
is used during the finalize-slug process to know what files to keep and which to
purge.

This command simply verifies each path given is in fact a relative path to the
current directory and appends it to a new Slugfile.

The write-slugfile command will be hidden whenever Slugfile or Slugsums files
are present in this or any parent directory.
`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "verbose",
				Usage:   "output detailed information",
				Aliases: []string{"v"},
			},
		},
		Action: func(ctx *cli.Context) (err error) {
			if err = s.Prepare(ctx); err != nil {
				return
			}
			argv := ctx.Args().Slice()
			if len(argv) == 0 {
				cli.ShowCommandHelpAndExit(ctx, "write-slugfile", 1)
				return
			}
			sort.Sort(sortorder.Natural(argv))
			var slugfile string
			if slugfile, err = slug.WriteSlugfile(argv...); err != nil {
				return
			}
			io.StdoutF("# wrote: ./Slugfile\n")
			if ctx.Bool("verbose") {
				io.StdoutF(slugfile)
			}
			return
		},
	}
}

func (s *Command) makeFinalizeSlugCommand(appNamePrefix string) *cli.Command {
	return &cli.Command{
		Name:      "finalize-slug",
		Category:  s.TagName,
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
			if slugfile := bePath.FindFileRelativeToPwd("Slugfile"); slugfile == "" {
				err = fmt.Errorf("missing Slugfile")
				return
			}
			if !ctx.IsSet("force") {
				err = fmt.Errorf("cannot finalize-slug without --force")
				return
			}
			if basepath.EnjenvPresent() {
				err = fmt.Errorf("finalize-slug requires '%v clean'", io.BinName)
				return
			}
			if git.IsRepo() {
				err = fmt.Errorf("cannot finalize-slug when .git is present")
				return
			}

			var slugsums string
			var removed []string
			if slugsums, removed, err = slug.FinalizeSlugfile(ctx.Bool("force")); err != nil {
				return
			}

			if ctx.Bool("verbose") {
				io.StdoutF("# Slugsums:\n")
				io.StdoutF(slugsums)
				io.StdoutF("# Removed:\n")
				io.StdoutF("%v\n", strings.Join(removed, "\n"))
			} else {
				io.StdoutF("# %d Slugsums\n", len(strings.Split(slugsums, "\n")))
				io.StdoutF("# removed %d extraneous paths\n", len(removed))
			}

			io.NotifyF("slug environment finalized")
			return
		},
	}
}

func (s *Command) makeValidateSlugCommand(appNamePrefix string) *cli.Command {
	return &cli.Command{
		Name:      "validate-slug",
		Category:  s.TagName,
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
			if slugsums := bePath.FindFileRelativeToPwd("Slugsums"); slugsums == "" {
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
				io.NotifyF("Summary: %d imposters, %d extraneous, %v validated", il, el, vl)
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
			io.NotifyF("Slugsums validated successfully")
			return
		},
	}
}