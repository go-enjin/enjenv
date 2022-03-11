package heroku

import (
	"fmt"
	"os"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/cli/git"
	bePath "github.com/go-enjin/be/pkg/path"

	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/io"
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
		cli.ShowCommandHelpAndExit(ctx, "deploy-slug", 1)
		return
	}
	src := argv[0]
	dst := argv[1]

	_ = os.Setenv("_ENJENV_NOTIFY_PREFIX", "deploy-slug "+bePath.Base(src))
	io.NotifyF("deploy-slug", "starting local deployment")
	io.StdoutF("# src: %v, dst: %v\n", src, dst)

	if bePath.IsFile(dst) {
		err = fmt.Errorf("<dst> is a file, something must be wrong")
		return
	}

	if bePath.IsDir(dst) {
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
					bePath.Dir(dst),
					bePath.Base(dst),
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
	pwd := bePath.Pwd()
	if err = os.Chdir(dst); err != nil {
		err = fmt.Errorf("chdir %v error: %v", dst, err)
		return
	}

	if _, err = git.Exe("clone", src, "."); err != nil {
		err = fmt.Errorf("git clone %v . error: %v", src, err)
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
		if _, err = c.enjenvExe(initArgs...); err != nil {
			err = fmt.Errorf("enjenv %v init error: %v", sysName, err)
			return
		}
	}

	//	- run make "--release-targets"
	targets := ctx.StringSlice("target")
	io.NotifyF("deploy-slug", "making release targets: %v\n", targets)
	if _, err = c.makeExe(targets...); err != nil {
		err = fmt.Errorf("make error: %v", err)
		return
	}
	io.NotifyF("deploy-slug", "release targets completed: %v\n", targets)

	//	- run enjenv clean --force
	io.StdoutF("# enjenv cleaning up\n")
	if _, err = c.enjenvExe("clean", "--force"); err != nil {
		err = fmt.Errorf("enjenv clean error: %v", err)
		return
	}

	if err = os.RemoveAll(".git"); err != nil {
		err = fmt.Errorf("removing .git error: %v", err)
		return
	}

	//	- run enjenv finalize-slug --verbose --force
	io.NotifyF("deploy-slug", "enjenv finalize-slug")
	if _, err = c.enjenvExe("finalize-slug", "--verbose", "--force"); err != nil {
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