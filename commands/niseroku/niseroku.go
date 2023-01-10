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
	"os/user"
	"strconv"
	"syscall"

	"github.com/go-git/go-git/v5"
	cp "github.com/otiai10/copy"
	"github.com/sevlyar/go-daemon"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/sosedoff/gitkit"
	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/cli/env"
	"github.com/go-enjin/be/pkg/cli/run"
	bePath "github.com/go-enjin/be/pkg/path"

	"github.com/go-enjin/enjenv/pkg/basepath"
	beIo "github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/system"
)

import (
	// #include <unistd.h>
	// #include <errno.h>
	"C"
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

						{
							Name:   "start",
							Usage:  "start an app slug",
							Action: c.actionAppStart,
							Flags: []cli.Flag{
								&cli.BoolFlag{
									Name:   "slug-process",
									Hidden: true,
								},
							},
						},
						{
							Name:   "stop",
							Usage:  "stop an app slug",
							Action: c.actionAppStop,
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
	if !bePath.IsFile(c.config.Paths.PidFile) {
		beIo.STDERR("pid file not found: %v\n", c.config.Paths.PidFile)
		return
	}
	var resp string
	if resp, err = c.CallControlCommand("shutdown"); err != nil {
		return
	}
	beIo.STDOUT("%v", resp)
	return
}

func (c *Command) actionReload(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}

	// if pid-file exists, read pid from file
	// if pid is a running process: send SIGUSR1

	if !bePath.IsFile(c.config.Paths.PidFile) {
		err = fmt.Errorf("pid file not found, nothing to signal")
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
	err = fmt.Errorf("niseroku restart not implemented")
	return
}

func (c *Command) actionGitPreReceiveHook(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}

	beIo.STDOUT("# initializing slug building process\n")

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
				beIo.StdoutF("# validated git repository for ssh-id: %v\n", a.Name)
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

	tmpName := bePath.Base(tmpPath)
	buildDir := config.Paths.TmpBuild + "/" + tmpName
	cacheDir := config.Paths.VarCache + "/" + app.Name
	slugZip := config.Paths.VarSlugs + "/" + app.Name + "--" + info.NewRev + ".zip"
	buildPackClonePath := config.Paths.TmpClone + "/" + app.Name
	envDir := config.Paths.VarSettings + "/" + app.Name

	beIo.STDOUT("# preparing ENV_DIR...\n")
	if bePath.IsDir(envDir) {
		if err = os.RemoveAll(envDir); err != nil {
			err = fmt.Errorf("error removing enjin env path: %v - %v", envDir, err)
			return
		}
	}
	if err = bePath.Mkdir(envDir); err != nil {
		err = fmt.Errorf("error making enjin deployment path: %v - %v", envDir, err)
		return
	}
	if err = app.ApplySettings(envDir); err != nil {
		err = fmt.Errorf("error applying enjin settings: %v - %v", envDir, err)
		return
	}

	beIo.STDOUT("# preparing CACHE_DIR...\n")
	if !bePath.IsDir(cacheDir) {
		if err = bePath.Mkdir(cacheDir); err != nil {
			err = fmt.Errorf("error making enjin deployment path: %v - %v", cacheDir, err)
			return
		}
	}

	beIo.STDOUT("# preparing BUILD_DIR...\n")
	if err = cp.Copy(tmpPath, buildDir); err != nil {
		err = fmt.Errorf("error copying to enjin build path: %v - %v", buildDir, err)
		return
	}
	defer func() {
		// cleanup build dir, if success, zip is all that is needed
		beIo.STDOUT("# cleaning BUILD_DIR...\n")
		_ = os.RemoveAll(buildDir)
	}()

	beIo.STDOUT("# preparing enjenv buildpack...\n")
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
		beIo.STDOUT("# cleaning enjenv buildpack...\n")
		_ = os.RemoveAll(envDir)
		_ = os.RemoveAll(buildPackClonePath)
	}()

	var status int
	beIo.STDOUT("# buildpack: detecting... ")
	if status, err = run.Exe(buildPackClonePath+"/bin/detect", buildDir); err != nil {
		err = fmt.Errorf("error detecting buildpack: %v", err)
		beIo.STDOUT("\n")
		return
	} else if status != 0 {
		beIo.STDOUT("\n")
		return
	}

	beIo.STDOUT("# buildpack: compiling...\n")
	if status, err = run.Exe(buildPackClonePath+"/bin/compile", buildDir, cacheDir, envDir); err != nil {
		return
	} else if status != 0 {
		return
	}

	beIo.STDOUT("# starting slug compression\n")
	pwd := bePath.Pwd()
	if err = os.Chdir(buildDir); err != nil {
		return
	}
	if status, err = run.Exe("zip", "--quiet", "--recurse-paths", slugZip, "."); err != nil {
		return
	} else if status != 0 {
		return
	}
	beIo.STDOUT("# finished slug compression\n")

	if err = os.Chdir(pwd); err != nil {
		return
	}

	if app.ThisSlug != slugZip {
		app.NextSlug = slugZip
		beIo.STDOUT("# updating niseroku application config for next slug\n")
		if err = app.Save(); err != nil {
			return
		}
	}

	beIo.STDOUT("# build completed, signaling for slug deployment\n")
	if err = sendSignalToPidFromFile(c.config.Paths.PidFile, syscall.SIGUSR1); err != nil {
		beIo.StderrF("# error signaling for slug deployment: %v\n", err)
		return
	}
	return
}

func (c *Command) actionAppStart(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	beIo.LogFile = ""
	if ctx.NArg() != 1 {
		err = fmt.Errorf("too many arguments")
		return
	}
	appName := ctx.Args().Get(0)

	if !ctx.Bool("slug-process") {
		binPath := basepath.EnjenvBinPath
		argv := []string{binPath, "niseroku", "app", "start", "--slug-process", appName}

		dCtx := &daemon.Context{
			Args:  argv,
			Env:   os.Environ(),
			Umask: 0222,
		}

		var dProc *os.Process
		if dProc, err = dCtx.Reborn(); err != nil {
			beIo.StderrF("error daemonizing slug process: %v - %v\n", appName, err)
			return
		} else if dProc != nil {
			beIo.StdoutF("slug process started: %v\n", appName)
			return
		}
		defer func() {
			if ee := dCtx.Release(); ee != nil {
				beIo.StderrF("error releasing daemon context: %v - %v\n", appName, err)
			}
		}()
	}

	var s *Server
	var ok bool
	var app *Application
	var slug *Slug

	if err = c.dropPrivileges(); err != nil {
		err = fmt.Errorf("error dropping root privileges: %v", err)
		return
	}

	if s, err = NewServer(c.config); err != nil {
		return
	}

	if app, ok = s.LookupApp[appName]; !ok {
		err = fmt.Errorf("app not found: %v", appName)
		return
	}

	if slug, err = s.PrepareAppSlug(app); err != nil {
		return
	}

	if running, ready := slug.IsRunningReady(); running && ready {
		err = fmt.Errorf("slug is already running and ready")
		return
	} else if running {
		err = fmt.Errorf("slug is already running though not ready")
		return
	}

	var webCmd string
	var webArgv, environ []string
	if webCmd, webArgv, environ, err = slug.PrepareStart(app.Origin.Port); err != nil {
		return
	}

	if ee := run.ExeWith(&run.Options{Path: slug.RunPath, Name: webCmd, Argv: webArgv, Stdout: slug.LogFile, Stderr: slug.LogFile, PidFile: slug.PidFile, Environ: environ}); ee != nil {
		err = ee
		return
	}

	return
}

func (c *Command) actionAppStop(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	beIo.LogFile = ""
	if ctx.NArg() != 1 {
		err = fmt.Errorf("too many arguments")
		return
	}
	appName := ctx.Args().Get(0)

	var s *Server
	var ok bool
	var app *Application
	var slug *Slug

	if err = c.dropPrivileges(); err != nil {
		err = fmt.Errorf("error dropping root privileges: %v", err)
		return
	}

	if s, err = NewServer(c.config); err != nil {
		return
	}
	if app, ok = s.LookupApp[appName]; !ok {
		err = fmt.Errorf("app not found: %v", appName)
		return
	}

	if err = app.LoadAllSlugs(); err != nil {
		err = fmt.Errorf("error loading all app slugs: %v - %v", app.Name, err)
		return
	}

	if slug = app.GetThisSlug(); slug != nil {
		slug.Stop()
	} else {
		err = fmt.Errorf("error getting this slug for app: %v", app.Name)
	}

	return
}

func (c *Command) dropPrivileges() (err error) {
	if syscall.Getuid() == 0 {
		// beIo.StdoutF("dropping root privileges to %v:%v\n", c.config.RunAs.User, c.config.RunAs.Group)
		var u *user.User
		if u, err = user.Lookup(c.config.RunAs.User); err != nil {
			return
		}
		var g *user.Group
		if g, err = user.LookupGroup(c.config.RunAs.Group); err != nil {
			return
		}

		var uid, gid int
		if uid, err = strconv.Atoi(u.Uid); err != nil {
			return
		}
		if gid, err = strconv.Atoi(g.Gid); err != nil {
			return
		}

		if cerr, errno := C.setgid(C.__gid_t(gid)); cerr != 0 {
			err = fmt.Errorf("set GID error: %v", errno)
			return
		} else if cerr, errno = C.setuid(C.__uid_t(uid)); cerr != 0 {
			err = fmt.Errorf("set UID error: %v", errno)
			return
		}
	}
	return
}