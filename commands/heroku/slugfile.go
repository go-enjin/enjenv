package heroku

import (
	"sort"

	"github.com/fvbommel/sortorder"
	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/slug"
	"github.com/go-enjin/enjenv/pkg/io"
)

func (c *Command) makeWriteSlugfileCommand(appNamePrefix string) *cli.Command {
	return &cli.Command{
		Name:      "write-slugfile",
		Category:  c.TagName,
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
			if err = c.Prepare(ctx); err != nil {
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