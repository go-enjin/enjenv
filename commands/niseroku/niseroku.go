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

	"github.com/sevlyar/go-daemon"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/urfave/cli/v2"

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
	Name           = "niseroku"
	ConfigFileName = Name + ".toml"
)

var DefaultConfigLocations = []string{
	"/etc/niseroku/" + ConfigFileName,
	"./" + ConfigFileName,
}

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
					Name:      "fix-fs",
					Usage:     "repair file ownership and modes",
					UsageText: app.Name + " niseroku fix-fs",
					Action:    c.actionFixFs,
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

	var s *Server
	var ok bool
	var app *Application
	var slug *Slug

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

	if err = c.dropPrivileges(); err != nil {
		err = fmt.Errorf("error dropping root privileges: %v", err)
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
		if slug.Stop() {
			beIo.STDOUT("slug process stopped: %v\n", app.Name)
		} else {
			beIo.STDOUT("slug process already stopped: %v\n", app.Name)
		}
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