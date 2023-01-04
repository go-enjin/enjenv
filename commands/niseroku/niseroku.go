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

package niseroku

import (
	"fmt"
	"io"
	"log"
	"os"
	"syscall"

	"github.com/go-git/go-git/v5"
	cp "github.com/otiai10/copy"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/sosedoff/gitkit"
	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/cli/env"
	"github.com/go-enjin/be/pkg/cli/run"
	bePath "github.com/go-enjin/be/pkg/path"

	beIo "github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/system"
)

const (
	Name = "niseroku"
)

func init() {
	log.SetOutput(io.Discard)
}

type Command struct {
	system.CCommand

	config *Config
}

func New() (s *Command) {
	s = new(Command)
	s.Init(s)
	return
}

func (c *Command) Init(this interface{}) {
	c.CCommand.Init(this)
	c.TagName = Name
	return
}

func (c *Command) ExtraCommands(app *cli.App) (commands []*cli.Command) {
	commands = append(
		commands,
		&cli.Command{
			Name: "niseroku",
			Description: `
niseroku is a small-scale hosting service for one or more Go-Enjin projects.

For "at-scale" things please use something else. Niseroku is intended for
supporting small-scale shared-deployment situations such as a collection of
Go-Enjin blog websites and so on using a cheap VPS.

A running niseroku instance has three main facets or aspects.

The first is a simple GIT remote repository system which receives "git push"
events and builds the repository with the assumption that the repository is a
Go-Enjin project.

The second is an enjin instance manager which starts, stops and handles
new slugs being deployed (after a "git push" event). When a new slug is
deployed, it is started up with a new randomly selected port (within a
configurable range) as a sub-process. When that sub-process finally opens
the specified random port (with 5 minute timeout), updates the app repo port
setting to point to the new port and destroys the previous slug instance. If
there is any error or the sub-process fails to open the port within the 5
minute window, the new slug is destroyed and the existing one remains running.

The last aspect is an HTTP(S) reverse-proxy that listens on ports 80 and 443,
using Let's Encrypt for SSL certificates. Inbound requests are re-requested to
a configured app repo instance and the responses returned as-if the request was
handled directly.
`,
			Flags: []cli.Flag{
				&cli.PathFlag{
					Name:    "config",
					Usage:   "specify path to niseroku.toml",
					EnvVars: []string{"ENJENV_NISEROKU_CONFIG_PATH"},
				},
			},
			Subcommands: []*cli.Command{
				{
					Name:      "start",
					Usage:     "start the niseroku instance",
					UsageText: app.Name + " niseroku start",
					Action:    c.actionStart,
				},
				{
					Name:      "stop",
					Usage:     "stop the niseroku instance",
					UsageText: app.Name + " niseroku stop",
					Action:    c.actionStop,
				},
				{
					Name:      "reload",
					Usage:     "reload all niseroku configurations",
					UsageText: app.Name + " niseroku reload",
					Action:    c.actionReload,
				},
				{
					Name:      "restart",
					Usage:     "restart the niseroku instance",
					UsageText: app.Name + " niseroku restart",
					Action:    c.actionRestart,
				},
				{
					Name:  "app",
					Usage: "manage specific enjin applications",
					Subcommands: []*cli.Command{

						{
							Name:   "git-pre-receive-hook",
							Action: c.actionGitPreReceiveHook,
							Hidden: true,
						},
						{
							Name:   "git-post-receive-hook",
							Action: c.actionGitPostReceiveHook,
							Hidden: true,
						},
					},
				},
			},
		},
	)
	return
}

const ConfigFileName = "niseroku.toml"

var DefaultConfigLocations = []string{
	"/etc/niseroku/" + ConfigFileName,
	"./" + ConfigFileName,
}

func (c *Command) findConfig(ctx *cli.Context) (config *Config, err error) {
	var path string
	if path = ctx.String("config"); path == "" {
		for _, check := range DefaultConfigLocations {
			if bePath.IsFile(check) {
				path = check
			}
		}
	}
	if path == "" {
		err = fmt.Errorf("%v not found, please use --config", ConfigFileName)
		return
	}
	if path, err = bePath.Abs(path); err != nil {
		return
	}
	config, err = InitConfig(path)
	return
}

func (c *Command) Prepare(ctx *cli.Context) (err error) {
	if err = c.CCommand.Prepare(ctx); err != nil {
		return
	}
	if c.config, err = c.findConfig(ctx); err != nil {
		return
	}
	if c.config.LogFile != "" {
		beIo.LogFile = c.config.LogFile
	}
	return
}

func (c *Command) actionStart(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}

	beIo.StdoutF("# starting new service\n")

	var s *Server
	if s, err = NewServer(c.config); err != nil {
		err = fmt.Errorf("new service error: %v", err)
	} else if err = s.InitPidFile(); err != nil {
		err = fmt.Errorf("init pid file error: %v", err)
	} else if err = s.Start(); err != nil {
		err = fmt.Errorf("error starting service: %v", err)
	}
	return
}

func (c *Command) actionStop(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}

	// if pid-file exists, read pid from file
	// if pid is a running process: send SIGINT

	if !bePath.IsFile(c.config.Paths.PidFile) {
		err = fmt.Errorf("pid file not found, nothing to stop")
		return
	}

	var proc *process.Process
	if proc, err = getProcessFromPidFile(c.config.Paths.PidFile); err == nil && proc != nil {
		if err = proc.SendSignal(syscall.SIGINT); err != nil {
			err = fmt.Errorf("error sending SIGINT to process: %d - %v", proc.Pid, err)
		}
	} else if err = os.Remove(c.config.Paths.PidFile); err != nil {
		err = fmt.Errorf("error removing stale pid file: %v", err)
		return
	}

	return
}

func (c *Command) actionReload(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}

	// if pid-file exists, read pid from file
	// if pid is a running process: send SIGUSR1

	if !bePath.IsFile(c.config.Paths.PidFile) {
		err = fmt.Errorf("pid file not found, nothing to stop")
		return
	}

	var proc *process.Process
	if proc, err = getProcessFromPidFile(c.config.Paths.PidFile); err == nil && proc != nil {
		if err = proc.SendSignal(syscall.SIGUSR1); err != nil {
			err = fmt.Errorf("error sending SIGUSR1 to process: %d - %v", proc.Pid, err)
		} else {
			beIo.StdoutF("sent SIGUSR1 to: %d\n", proc.Pid)
		}
	}

	return
}

func (c *Command) actionRestart(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}

	// if pid-file exists, read pid from file
	// if pid is a running process: send SIGHUP

	err = fmt.Errorf("restart command unimplemented\n")

	return
}

func (c *Command) actionGitPreReceiveHook(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	// beIo.StdoutF("git pre-receive hook\n")

	// authenticate git repo endpoint (and branch) with ssh-key
	// early error during a git-push

	receiver := gitkit.Receiver{
		MasterOnly: false,              // if set to true, only pushes to master branch will be allowed
		TmpDir:     c.config.Paths.Tmp, // directory for temporary git checkouts
		HandlerFunc: func(info *gitkit.HookInfo, tmpPath string) (err error) {
			var app *Application
			if app, err = c.enjinRepoGitHandlerSetup(c.config, info); err != nil {
				return
			}
			err = c.enjinRepoPreReceiveHandler(app, c.config, info, tmpPath) // your handler function
			return
		},
	}

	// Git hook data is provided via STDIN
	if err = receiver.Handle(os.Stdin); err != nil {
		return
	}

	return
}

func (c *Command) actionGitPostReceiveHook(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}

	// beIo.StdoutF("git post-receive hook\n")

	// validate git repo endpoint (and branch) with ssh-key
	// detect, build and archive a new app slug
	// notify slug runner to "try" the new slug (destroyed on error)

	receiver := gitkit.Receiver{
		MasterOnly: false,              // if set to true, only pushes to master branch will be allowed
		TmpDir:     c.config.Paths.Tmp, // directory for temporary git checkouts
		HandlerFunc: func(info *gitkit.HookInfo, tmpPath string) (err error) {
			var app *Application
			if app, err = c.enjinRepoGitHandlerSetup(c.config, info); err != nil {
				return
			}
			err = c.enjinRepoPostReceiveHandler(app, c.config, info, tmpPath) // your handler function
			return
		},
	}

	// Git hook data is provided via STDIN
	if err = receiver.Handle(os.Stdin); err != nil {
		return
	}

	return
}

func (c *Command) enjinRepoGitHandlerSetup(config *Config, info *gitkit.HookInfo) (app *Application, err error) {
	if envSshId := env.Get("GITKIT_KEY", ""); envSshId != "" {
		// beIo.StdoutF("key: %v\n", envSshId)
		// beIo.StdoutF("pwd: %v\n", bePath.Pwd())
		// beIo.StdoutF("cfg: %+#v\n", config)
		var s *Server
		if s, err = NewServer(config); err != nil {
			return
		}
		s.RLock()
		defer s.RUnlock()
		for _, a := range s.Applications() {
			var present bool
			if present, err = a.HasSshKey(envSshId); err != nil {
				return
			} else if present {
				if info.RepoPath != a.RepoPath {
					err = fmt.Errorf("invalid git repository for ssh-id")
					return
				}
				beIo.StdoutF("# valid git repository for ssh-id: %v\n", a.Name)
				app = a
				break
			}
		}
		if app != nil && info.RefName != "main" {
			err = fmt.Errorf("invalid branch received")
			return
		}
	} else {
		err = fmt.Errorf("missing ssh-key id")
		return
	}
	return
}

func (c *Command) enjinRepoPreReceiveHandler(app *Application, config *Config, info *gitkit.HookInfo, tmpPath string) (err error) {
	// nop, all safety checks done in git handler setup
	return
}

func (c *Command) enjinRepoPostReceiveHandler(app *Application, config *Config, info *gitkit.HookInfo, tmpPath string) (err error) {
	// beIo.StdoutF("# post-receive handler:\n%+#v\n", info)

	tmpName := bePath.Base(tmpPath)
	buildDir := config.Paths.TmpBuild + "/" + tmpName
	cacheDir := config.Paths.VarCache + "/" + app.Name
	slugZip := config.Paths.VarSlugs + "/" + app.Name + "--" + info.NewRev + ".zip"
	buildPackClonePath := config.Paths.TmpClone + "/" + app.Name
	envDir := config.Paths.VarSettings + "/" + app.Name

	if err = cp.Copy(tmpPath, buildDir); err != nil {
		return
	}
	defer func() {
		// cleanup build dir, if success, zip is all that is needed
		_ = os.RemoveAll(buildDir)
	}()

	for _, dir := range []string{envDir, cacheDir /*, buildPackClonePath*/} {
		if !bePath.IsDir(dir) {
			if err = bePath.Mkdir(dir); err != nil {
				err = fmt.Errorf("error making enjin deployment path: %v - %v", dir, err)
				return
			}
		}
	}

	if err = app.ApplySettings(envDir); err != nil {
		return
	}

	if bePath.IsDir(config.BuildPack) {
		if err = cp.Copy(config.BuildPack, buildPackClonePath); err != nil {
			return
		}
	} else if _, err = git.PlainClone(buildPackClonePath, false, &git.CloneOptions{
		URL: "https://github.com/go-enjin/enjenv-heroku-buildpack.git",
	}); err != nil {
		return
	}
	defer func() {
		_ = os.RemoveAll(envDir)
		_ = os.RemoveAll(buildPackClonePath)
	}()

	var status int
	if status, err = run.Exe(buildPackClonePath+"/bin/detect", buildDir); err != nil {
		return
	} else if status != 0 {
		return
	}

	if status, err = run.Exe(buildPackClonePath+"/bin/compile", buildDir, cacheDir, envDir); err != nil {
		return
	} else if status != 0 {
		return
	}

	pwd := bePath.Pwd()
	if err = os.Chdir(buildDir); err != nil {
		return
	}

	if status, err = run.Exe("zip", "-r", slugZip, "."); err != nil {
		return
	} else if status != 0 {
		return
	}

	if err = os.Chdir(pwd); err != nil {
		return
	}

	app.NextSlug = slugZip
	if err = app.Save(); err != nil {
		return
	}

	// beIo.StdoutF("# BUILD_DIR: %v\n", buildDir)
	// beIo.StdoutF("# CACHE_DIR: %v\n", cacheDir)
	// beIo.StdoutF("# ENV_DIR: %v\n", envDir)
	// beIo.StdoutF("# app.NextSlug: %v\n", app.NextSlug)

	beIo.StdoutF("# build completed, signaling for slug deployment")
	if err = sendSignalToPidFromFile(c.config.Paths.PidFile, syscall.SIGUSR1); err != nil {
		beIo.StderrF("# error signaling for slug deployment: %v\n", err)
		return
	}
	return
}