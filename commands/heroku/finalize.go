package heroku

import (
	"fmt"

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
			if basepath.EnjenvPresent() {
				err = fmt.Errorf("finalize-slug requires '%v clean'", io.BinName)
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

			lr := len(removed)
			if lr > 0 {
				io.StdoutF("# removed %d extraneous paths\n", lr)
			}
			// if ctx.Bool("verbose") {
			// 	io.StdoutF("# %d Slugsums:\n", len(strings.Split(slugsums, "\n")))
			// 	io.StdoutF(slugsums)
			// } else {
			// 	io.StdoutF("# %d Slugsums\n", len(strings.Split(slugsums, "\n")))
			// }
			io.NotifyF("finalize-slug", "Wrote %d Slugsums:\n%v", len(slugsums), slugsums)

			io.NotifyF("finalize-slug", "slug environment finalized")
			return
		},
	}
}