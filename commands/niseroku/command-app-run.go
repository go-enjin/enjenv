// Copyright (c) 2023  The Go-Enjin Authors
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
	"os"
	"syscall"

	"github.com/sevlyar/go-daemon"
	"github.com/urfave/cli/v2"

	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

func (c *Command) actionAppRun(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	io.LogFile = ""
	if argc := ctx.NArg(); argc > 1 {
		err = fmt.Errorf("too many arguments")
		return
	} else if argc < 1 {
		cli.ShowCommandHelpAndExit(ctx, "run", 1)
	}
	appName := ctx.Args().Get(0)

	var ok bool
	var app *Application

	if app, ok = c.config.Applications[appName]; !ok {
		err = fmt.Errorf("app not found: %v", appName)
		return
	}

	if app.IsDeploying() {
		err = fmt.Errorf("application deployment in progress: %v", appName)
		return
	}

	if !ctx.Bool("slug-process") {
		binPath := basepath.EnjenvBinPath
		argv := []string{binPath, "niseroku", "app", "run", "--slug-process", appName}

		dCtx := &daemon.Context{
			Args:  argv,
			Env:   os.Environ(),
			Umask: 0222,
		}

		var dProc *os.Process
		if dProc, err = dCtx.Reborn(); err != nil {
			io.StderrF("error daemonizing slug process: %v - %v\n", appName, err)
			return
		} else if dProc != nil {
			io.StdoutF("slug process running: %v\n", appName)
			return
		}
		defer func() {
			if ee := dCtx.Release(); ee != nil {
				io.StderrF("error releasing daemon context: %v - %v\n", appName, err)
			}
		}()
	}

	if syscall.Getuid() == 0 {
		thisPid := os.Getpid()
		niceVal := c.config.SlugNice
		ee := common.SetPgrpPriority(thisPid, niceVal)
		if err = common.DropPrivilegesTo(c.config.RunAs.User, c.config.RunAs.Group); err != nil {
			err = fmt.Errorf("error dropping root privileges: %v", err)
			return
		} else if ee != nil {
			app.LogErrorF("error setting pid-group(%d) priority(%d): %v - %v\n", thisPid, niceVal, app.Name, ee)
		} else {
			app.LogInfoF("pid-group (%d) priority set: %d\n", thisPid, niceVal)
		}
	}

	if err = app.Deploy(); err != nil {
		app.LogErrorF("error deploying application: %v - %v\n", app.Name, err)
		return
	}

	return
}