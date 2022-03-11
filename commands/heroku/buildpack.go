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
	"os"

	"github.com/urfave/cli/v2"

	bePath "github.com/go-enjin/be/pkg/path"

	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/io"
)

func (c *Command) makeBuildpackCommand(appNamePrefix string) *cli.Command {
	return &cli.Command{
		Name:      "buildpack",
		Category:  c.TagName,
		Usage:     "actual heroku slug buildpack process",
		UsageText: appNamePrefix + " buildpack [options]",
		Description: `
buildpack will perform the following steps, stopping on any error:

	- run enjenv golang init --golang "--golang"
	- run enjenv nodejs init --nodejs "--nodejs" // if --nodejs given
	- run make "--release-targets"
	- run enjenv clean --force // if .enjenv present in PWD
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
				EnvVars: []string{"ENJENV_BUILDPACK_TARGETS"},
				Aliases: []string{"t"},
			},
			&cli.StringFlag{
				Name:    "golang",
				Usage:   "pass through to 'golang init --golang'",
				EnvVars: []string{"ENJENV_BUILDPACK_GOLANG"},
			},
			&cli.StringFlag{
				Name:    "nodejs",
				Usage:   "pass through to 'nodejs init --nodejs'",
				EnvVars: []string{"ENJENV_BUILDPACK_NODEJS"},
			},
		},
		Action: c.ActionBuildpack,
	}
}

func (c *Command) ActionBuildpack(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}

	if !ctx.IsSet("force") || !ctx.Bool("force") {
		err = io.ErrorF("buildpack requires --force argument")
		return
	}

	pwd := bePath.Pwd()
	_ = os.Setenv("_ENJENV_NOTIFY_PREFIX", "buildpack "+bePath.Base(pwd))
	io.NotifyF("buildpack", "starting deployment")

	// 	- run enjenv init --golang "--golang" --nodejs "--nodejs"
	io.StdoutF("# initializing enjenv: %v\n", basepath.EnjenvPath)

	var initSystemNames []string
	initSystemNames = append(initSystemNames, "golang")
	if ctx.IsSet("nodejs") {
		initSystemNames = append(initSystemNames, "nodejs")
	}
	for _, sysName := range initSystemNames {
		var initArgs []string
		initArgs = append(initArgs, sysName, "init")
		if arg := ctx.String(sysName); arg != "" {
			initArgs = append(initArgs, "--"+sysName, arg)
		}
		initArgs = append(initArgs, "--force")
		if _, err = c.enjenvExe(initArgs...); err != nil {
			err = io.ErrorF("enjenv %v init error: %v", sysName, err)
			return
		}
	}

	//	- run make "--release-targets"
	targets := ctx.StringSlice("target")
	io.NotifyF("buildpack", "making release targets: %v\n", targets)
	if _, err = c.makeExe(targets...); err != nil {
		err = io.ErrorF("make error: %v", err)
		return
	}
	io.NotifyF("buildpack", "release targets completed: %v\n", targets)

	//	- run enjenv clean --force
	if basepath.EnjenvIsInPwd() {
		io.StdoutF("# cleaning up local .enjenv for finalization\n")
		if _, err = c.enjenvExe("clean", "--force"); err != nil {
			err = io.ErrorF("enjenv clean error: %v", err)
			return
		}
	}

	//	- run enjenv finalize-slug --verbose --force
	io.NotifyF("buildpack", "enjenv finalize-slug")
	if _, err = c.enjenvExe("finalize-slug", "--verbose", "--force"); err != nil {
		err = io.ErrorF("enjenv finalize-slug error: %v", err)
		return
	}

	io.NotifyF("buildpack", "buildpack deployment completed")
	return
}