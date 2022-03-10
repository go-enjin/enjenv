package heroku

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	bePath "github.com/go-enjin/be/pkg/path"
	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/system"
)

func (c *Command) makeBuildpackCommand(appNamePrefix string) *cli.Command {
	return &cli.Command{
		Name:      "buildpack",
		Category:  c.TagName,
		Usage:     "actual heroku slug buildpack process",
		UsageText: appNamePrefix + " buildpack [options]",
		Description: `
buildpack will perform the following steps, stopping on any error:

	- run enjenv init --golang "--golang" --nodejs "--nodejs"
	- run make "--release-targets"
	- run enjenv clean --force
	- run enjenv finalize-slug --verbose --force

Note that this is a destructive process and exclusively intended for use within
the Go-Enjin Heroku buildpack: github.com/go-enjin/enjenv-heroku-buildpack
`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "force",
				Usage: "actually perform the buildpack process",
			},
			&cli.StringSliceFlag{
				Name:    "target",
				Usage:   "specify one or more build targets to use",
				Value:   cli.NewStringSlice("release"),
				EnvVars: []string{system.EnvPrefix + "_DEPLOY_SLUG_TARGETS"},
				Aliases: []string{"t"},
			},
			&cli.StringFlag{
				Name:    "golang",
				Usage:   "pass through to 'golang init --golang'",
				EnvVars: []string{system.EnvPrefix + "_DEPLOY_SLUG_GOLANG"},
			},
			&cli.StringFlag{
				Name:    "nodejs",
				Usage:   "pass through to 'nodejs init --nodejs'",
				EnvVars: []string{system.EnvPrefix + "_DEPLOY_SLUG_NODEJS"},
			},
			// &cli.BoolFlag{
			// 	Name:    "verbose",
			// 	Usage:   "output detailed information",
			// 	Aliases: []string{"v"},
			// 	EnvVars: []string{system.EnvPrefix+"_DEPLOY_SLUG_VERBOSE"},
			// },
		},
		Action: c.ActionBuildpack,
	}
}

func (c *Command) ActionBuildpack(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}

	if !ctx.IsSet("force") || !ctx.Bool("force") {
		err = fmt.Errorf("buildpack requires --force argument")
		return
	}

	pwd := bePath.Pwd()
	_ = os.Setenv(system.EnvPrefix+"_NOTIFY_PREFIX", "buildpack "+bePath.Base(pwd))
	io.NotifyF("buildpack", "starting deployment")

	// 	- run enjenv init --golang "--golang" --nodejs "--nodejs"
	io.StdoutF("# initializing enjenv\n")

	if basepath.EnjenvPresent() {
		io.StdoutF("# enjenv path present, masking for local\n")
		_ = os.Setenv("ENVJENV_PATH", "")
	}

	var initArgs []string
	initArgs = append(initArgs, "init")
	if golang := ctx.String("golang"); golang != "" {
		initArgs = append(initArgs, "--golang", golang)
	}
	if nodejs := ctx.String("nodejs"); nodejs != "" {
		initArgs = append(initArgs, "--nodejs", nodejs)
	}
	initArgs = append(initArgs, ".")

	if _, err = c.enjenvExe(initArgs...); err != nil {
		err = fmt.Errorf("enjenv init error: %v", err)
		return
	}

	//	- run make "--release-targets"
	targets := ctx.StringSlice("target")
	io.NotifyF("buildpack", "making release targets: %v\n", targets)
	if _, err = c.makeExe(targets...); err != nil {
		err = fmt.Errorf("make error: %v", err)
		return
	}
	io.NotifyF("buildpack", "release targets completed: %v\n", targets)

	//	- run enjenv clean --force
	io.StdoutF("# enjenv cleaning up\n")
	if _, err = c.enjenvExe("clean", "--force"); err != nil {
		err = fmt.Errorf("enjenv clean error: %v", err)
		return
	}

	//	- run enjenv finalize-slug --verbose --force
	io.NotifyF("buildpack", "enjenv finalize-slug")
	if _, err = c.enjenvExe("finalize-slug", "--verbose", "--force"); err != nil {
		err = fmt.Errorf("enjenv finalize-slug error: %v", err)
		return
	}

	io.NotifyF("buildpack", "buildpack deployment completed")
	return
}