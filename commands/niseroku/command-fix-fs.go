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

import "C"
import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/urfave/cli/v2"

	bePath "github.com/go-enjin/be/pkg/path"

	beIo "github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

func makeCommandFixFs(c *Command, app *cli.App) (cmd *cli.Command) {
	cmd = &cli.Command{
		Name:      "fix-fs",
		Usage:     "repair file ownership and modes",
		UsageText: app.Name + " niseroku fix-fs",
		Action:    c.actionFixFs,
	}
	return
}

func (c *Command) actionFixFs(ctx *cli.Context) (err error) {
	defer func() {
		if err != nil {
			if beIo.LogFile != "" {
				beIo.STDERR("[fix-fs] error: %v\n", err)
			} else {
				beIo.StderrF("[fix-fs] error: %v\n", err)
			}
		}
	}()

	if err = c.Prepare(ctx); err != nil {
		return
	}

	if syscall.Getuid() != 0 {
		beIo.STDERR("# niseroku fix-fs requires super user privileges, nothing to do\n")
		return
	}

	fixFsPidFile := filepath.Join(c.config.Paths.Tmp, "fix-fs.pid")

	if bePath.IsFile(fixFsPidFile) {
		var stale bool
		if proc, ee := common.GetProcessFromPidFile(fixFsPidFile); ee != nil {
			stale = true
		} else if running, eee := proc.IsRunning(); eee != nil {
			stale = true
		} else if !running {
			stale = true
		} else if eee == nil && running {
			beIo.STDOUT("# [fix-fs] found another fix-fs process already running, nothing to do\n")
			err = nil
			return
		}
		if stale {
			beIo.StdoutF("# [fix-fs] removing stale fix-fs.pid file\n")
			_ = os.Remove(fixFsPidFile)
		}
	}

	beIo.STDOUT("# [fix-fs] filesystem repair starting\n")
	if err = os.WriteFile(fixFsPidFile, []byte(strconv.Itoa(os.Getpid())), 0660); err != nil {
		err = fmt.Errorf("[fix-fs] error writing pid file: %v - %v", fixFsPidFile, err)
		return
	}
	defer func() {
		beIo.StdoutF("# [fix-fs] cleaning up fix-fs.pid file\n")
		_ = os.Remove(fixFsPidFile)
	}()

	var uid, gid int
	if uid, gid, err = common.GetUidGid(c.config.RunAs.User, c.config.RunAs.Group); err != nil {
		err = fmt.Errorf("[fix-fs] error getting UID and/or GID for: %v:%v", c.config.RunAs.User, c.config.RunAs.Group)
		return
	}

	normalPaths := []string{
		c.config.Paths.Etc,
		c.config.Paths.EtcApps,
		c.config.Paths.EtcUsers,
		c.config.Paths.Tmp,
		c.config.Paths.TmpRun,
		c.config.Paths.TmpClone,
		c.config.Paths.TmpBuild,
		c.config.Paths.Var,
		c.config.Paths.VarLogs,
		c.config.Paths.VarSlugs,
		c.config.Paths.VarCache,
		c.config.Paths.VarRepos,
		c.config.Paths.VarAptRoot,
	}

	if err = common.PerformMkdirChownChmod(uid, gid, 0660, 0770, normalPaths...); err != nil {
		err = fmt.Errorf("[fix-fs] error performing normal paths mkdir+chown+chmod: %v", err)
		return
	}

	secretsPaths := []string{
		c.config.Paths.AptSecrets,
		c.config.Paths.VarSettings,
		c.config.Paths.RepoSecrets,
		c.config.Paths.ProxySecrets,
	}

	if err = common.PerformMkdirChownChmod(uid, gid, 0600, 0700, secretsPaths...); err != nil {
		err = fmt.Errorf("error performing secrets paths mkdir+chown+chmod: %v", err)
		return
	}

	if c.config.LogFile != "" {
		if !bePath.IsFile(c.config.LogFile) {
			if err = os.WriteFile(c.config.LogFile, []byte(""), 0660); err != nil {
				beIo.StderrF("[fix-fs] error preparing log file: %v - %v\n", c.config.LogFile, err)
			}
		} else {
			if err = os.Chmod(c.config.LogFile, 0660); err != nil {
				beIo.StderrF("[fix-fs] error changing mode of: %v [%v] - %v", c.config.LogFile, fs.FileMode(0660), err)
			}
		}
		if err = os.Chown(c.config.LogFile, uid, gid); err != nil {
			beIo.StderrF("[fix-fs] error changing ownership of: %v - %v\n", c.config.LogFile, err)
		}
	}

	if bePath.IsFile(c.config.Paths.ProxyRpcSock) {
		if err = os.Chmod(c.config.Paths.ProxyRpcSock, 0660); err != nil {
			beIo.StderrF("[fix-fs] error changing mode of: %v [%v] - %v", c.config.Paths.ProxyRpcSock, fs.FileMode(0660), err)
		}
		if err = os.Chown(c.config.Paths.ProxyRpcSock, uid, gid); err != nil {
			beIo.StderrF("[fix-fs] error changing ownership of: %v - %v\n", c.config.Paths.ProxyRpcSock, err)
		}
	}

	if bePath.IsFile(c.config.Paths.ProxyPidFile) {
		if err = os.Chmod(c.config.Paths.ProxyPidFile, 0660); err != nil {
			beIo.StderrF("[fix-fs] error changing mode of: %v [%v] - %v", c.config.Paths.ProxyPidFile, fs.FileMode(0660), err)
		}
		if err = os.Chown(c.config.Paths.ProxyPidFile, uid, gid); err != nil {
			beIo.StderrF("[fix-fs] error changing ownership of: %v - %v\n", c.config.Paths.ProxyPidFile, err)
		}
	}

	if bePath.IsFile(c.config.Paths.RepoPidFile) {
		if err = os.Chmod(c.config.Paths.RepoPidFile, 0660); err != nil {
			beIo.StderrF("[fix-fs] error changing mode of: %v [%v] - %v", c.config.Paths.RepoPidFile, fs.FileMode(0660), err)
		}
		if err = os.Chown(c.config.Paths.RepoPidFile, uid, gid); err != nil {
			beIo.StderrF("[fix-fs] error changing ownership of: %v - %v\n", c.config.Paths.RepoPidFile, err)
		}
	}

	beIo.STDOUT("# [fix-fs] filesystem repair completed\n")
	return
}