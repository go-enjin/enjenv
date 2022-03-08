package heroku

import (
	"github.com/urfave/cli/v2"

	"github.com/go-enjin/enjenv/pkg/io"
)

func (s *Command) makeDeployCommand(appNamePrefix string) *cli.Command {
	return &cli.Command{
		Name:      "deploy",
		Category:  s.TagName,
		Usage:     "deploy a git repository in a heroku fashion to a local path",
		UsageText: appNamePrefix + " deploy <src> <dst> [options]",
		Description: `
deploy will perform the following steps, stopping on the first error:

	- rename the destination path (if it exists)
	- clone the git repo into the destination path
	- configure the shell environment variables for further sub-commands
	- check if a Slugfile is present, error otherwise
	- initialize a local enjin environment, includes nodejs if package.json detected
	- if a Makefile is detected:
		- check for one target: release
		- run 'make release'
	- if no Makefile is present:
		- run 'enjenv golang build --verbose'
		- run 'enjenv finalize-slug --verbose --force'

See: the run-deployed command for next steps.
`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "verbose",
				Usage:   "output detailed information",
				Aliases: []string{"v"},
			},
			&cli.StringFlag{
				Name:  "target",
				Usage: "specify the target name to use",
				Value: "release",
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
			io.StderrF("not implemented")
			return
		},
	}
}