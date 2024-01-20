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
	"os"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/cli/git"
	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/run"

	clpath "github.com/go-corelibs/path"
)

func (c *Command) makeDeploySlugCommand(appNamePrefix string) *cli.Command {
	return &cli.Command{
		Name:      "deploy-slug",
		Category:  c.TagName,
		Usage:     "deploy a git repository in a heroku slug fashion to a local path",
		UsageText: appNamePrefix + " deploy-slug <src> <dst> [options]",
		Description: `
deploy-slug will perform the following steps, stopping on any error:

	- if <dst> exists:
		- if --backup:
			- rename <dst> to <backup-path>
		- if not --no-backup:
			- rename <dst> to <dst>--backup--YYYYMMDD-HHMMSS-ZONE
	- else:
		- mkdir <dst> and cd <dst>
	- run git clone <src> .
	- run enjenv golang init --golang "--golang"
	- run enjenv nodejs init --nodejs "--nodejs" // if --nodejs given
	- run make "--release-targets"
	- run enjenv clean --force
	- run rm -rf .git
	- run enjenv finalize-slug --verbose --force
`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "backup",
				Usage:   "specify the backup directory path",
				EnvVars: []string{"ENJENV_DEPLOY_SLUG_BACKUP"},
			},
			&cli.BoolFlag{
				Name:    "no-backup",
				Usage:   "if <dst> exists, remove it without backing up",
				EnvVars: []string{"ENJENV_DEPLOY_SLUG_NO_BACKUP"},
			},
			&cli.StringSliceFlag{
				Name:    "target",
				Usage:   "specify one or more build targets to use",
				Value:   cli.NewStringSlice("release"),
				EnvVars: []string{"ENJENV_DEPLOY_SLUG_TARGETS"},
				Aliases: []string{"t"},
			},
			&cli.StringFlag{
				Name:    "golang",
				Usage:   "pass through to 'golang init --golang'",
				EnvVars: []string{"ENJENV_DEPLOY_SLUG_GOLANG"},
			},
			&cli.StringFlag{
				Name:    "nodejs",
				Usage:   "pass through to 'nodejs init --nodejs'",
				EnvVars: []string{"ENJENV_DEPLOY_SLUG_NODEJS"},
			},
			// &cli.BoolFlag{
			// 	Name:    "verbose",
			// 	Usage:   "output detailed information",
			// 	Aliases: []string{"v"},
			// 	EnvVars: []string{"ENJENV_DEPLOY_SLUG_VERBOSE"},
			// },
		},
		Action: c.ActionDeploySlug,
	}
}

func (c *Command) ActionDeploySlug(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	argv := ctx.Args().Slice()
	if len(argv) < 2 {
		cli.ShowSubcommandHelpAndExit(ctx, 1)
		return
	}
	src := argv[0]
	dst := argv[1]

	_ = os.Setenv("_ENJENV_NOTIFY_PREFIX", "deploy-slug "+clpath.Base(src))
	io.NotifyF("deploy-slug", "starting local deployment")
	io.StdoutF("# src: %v, dst: %v\n", src, dst)

	if clpath.IsFile(dst) {
		err = fmt.Errorf("<dst> is a file, something must be wrong")
		return
	}

	if clpath.IsDir(dst) {
		switch {
		case ctx.Bool("no-backup"):
			io.StdoutF("# removing %v, --no-backup\n", dst)
			if err = os.RemoveAll(dst); err != nil {
				return
			}
		default:
			var backupPath string
			if backupPath = ctx.String("backup"); backupPath == "" {
				now := time.Now()
				backupPath = fmt.Sprintf(
					"%v/%v--backup--%v",
					clpath.Dir(dst),
					clpath.Base(dst),
					now.Format("20060102-150405-0700"),
				)
			}
			io.StdoutF("# renaming %v => %v\n", dst, backupPath)
			if err = os.Rename(dst, backupPath); err != nil {
				return
			}
		}
	}

	io.StdoutF("# preparing destination\n")
	if err = os.Mkdir(dst, 0770); err != nil {
		err = fmt.Errorf("mkdir error: %v", err)
		return
	}
	pwd := clpath.Pwd()
	if err = os.Chdir(dst); err != nil {
		err = fmt.Errorf("chdir %v error: %v", dst, err)
		return
	}

	var status int
	if status, err = git.Exe("clone", src, "."); err != nil {
		err = fmt.Errorf("git clone %v error: %v", src, err)
		return
	} else if status != 0 {
		err = fmt.Errorf("git clone %v exited with status: %d", src, status)
		return
	}

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
		if err = run.EnjenvExe(initArgs...); err != nil {
			err = io.ErrorF("enjenv %v init error: %v", sysName, err)
			return
		}
	}

	//	- run make "--release-targets"
	targets := ctx.StringSlice("target")
	io.NotifyF("deploy-slug", "making release targets: %v\n", targets)
	if err = run.MakeExe(targets...); err != nil {
		err = fmt.Errorf("make error: %v", err)
		return
	}
	io.NotifyF("deploy-slug", "release targets completed: %v\n", targets)

	//	- run enjenv clean --force
	if basepath.EnjenvIsInPwd() {
		io.StdoutF("# enjenv cleaning up\n")
		if err = run.EnjenvExe("clean", "--force"); err != nil {
			err = fmt.Errorf("enjenv clean error: %v", err)
			return
		}
	}

	if err = os.RemoveAll(".git"); err != nil {
		err = fmt.Errorf("removing .git error: %v", err)
		return
	}

	//	- run enjenv finalize-slug --verbose --force
	io.NotifyF("deploy-slug", "enjenv finalize-slug")
	if err = run.EnjenvExe("finalize-slug", "--verbose", "--force"); err != nil {
		err = fmt.Errorf("enjenv finalize-slug error: %v", err)
		return
	}

	if err = os.Chdir(pwd); err != nil {
		err = fmt.Errorf("chdir %v error: %v", pwd, err)
		return
	}
	io.NotifyF("deploy-slug", "local deployment completed")
	return
}
