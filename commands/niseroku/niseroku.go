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
	"os/user"
	"strconv"
	"syscall"

	"github.com/urfave/cli/v2"

	bePath "github.com/go-enjin/be/pkg/path"

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
					Name:      "reverse-proxy",
					Usage:     "niseroku reverse-proxy service",
					UsageText: app.Name + " niseroku reverse-proxy",
					Action:    c.actionReverseProxy,
				},
				{
					Name:      "git-repository",
					Usage:     "niseroku git-repository service",
					UsageText: app.Name + " niseroku git-repository",
					Action:    c.actionGitRepository,
				},
				{
					Name:      "status",
					Usage:     "display the status of all niseroku services",
					UsageText: app.Name + " niseroku status",
					Action:    c.actionStatus,
				},
				{
					Name:      "deploy-slug",
					Usage:     "deploy a built slug",
					UsageText: "niseroku deploy-slug <slug.zip> [slugs.zip...]",
					Action:    c.actionDeploySlug,
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
							Name:      "start",
							Usage:     "start an app slug",
							UsageText: app.Name + " niseroku app start <name>",
							Action:    c.actionAppStart,
							Flags: []cli.Flag{
								&cli.BoolFlag{
									Name:   "slug-process",
									Hidden: true,
								},
							},
						},
						{
							Name:      "stop",
							Usage:     "stop an app slug",
							UsageText: app.Name + " niseroku app stop <name>",
							Action:    c.actionAppStop,
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
	config, err = LoadConfig(path)
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

func (c *Command) dropPrivileges() (err error) {
	if syscall.Getuid() == 0 {

		var u *user.User
		if u, err = user.Lookup(c.config.RunAs.User); err != nil {
			return
		}
		var g *user.Group
		if g, err = user.LookupGroup(c.config.RunAs.Group); err != nil {
			return
		}

		// beIo.StdoutF("switching user:group to %v:%v\n", c.config.RunAs.User, c.config.RunAs.Group)

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