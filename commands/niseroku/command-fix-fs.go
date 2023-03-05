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
	"os"
	"syscall"

	"github.com/urfave/cli/v2"

	bePath "github.com/go-enjin/be/pkg/path"

	beIo "github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

func (c *Command) actionFixFs(ctx *cli.Context) (err error) {
	defer func() {
		if err != nil {
			if beIo.LogFile != "" {
				beIo.STDERR("error: %v\n", err)
			} else {
				beIo.StderrF("error: %v\n", err)
			}
		}
	}()

	if err = c.Prepare(ctx); err != nil {
		return
	}

	if syscall.Getuid() != 0 {
		err = fmt.Errorf("niseroku fix-fs requires super user privileges")
		return
	}

	var uid, gid int
	if uid, gid, err = common.GetUidGid(c.config.RunAs.User, c.config.RunAs.Group); err != nil {
		return
	}

	if err = common.PerformMkdirChownChmod(uid, gid, 0660, 0770, c.config.Paths.Etc, c.config.Paths.Tmp, c.config.Paths.Var); err != nil {
		return
	}

	if err = common.PerformMkdirChownChmod(uid, gid, 0660, 0770, c.config.Paths.AptSecrets, c.config.Paths.ProxySecrets, c.config.Paths.RepoSecrets); err != nil {
		return
	}

	if c.config.LogFile != "" {
		if !bePath.IsFile(c.config.LogFile) {
			if err = os.WriteFile(c.config.LogFile, []byte(""), 0660); err != nil {
				beIo.StderrF("error preparing log file: %v - %v\n", c.config.LogFile, err)
			}
		}
		if err = os.Chown(c.config.LogFile, uid, gid); err != nil {
			beIo.StderrF("error changing ownership of: %v - %v\n", c.config.LogFile, err)
		}
	}

	if bePath.IsFile(c.config.Paths.ProxyPidFile) {
		if err = os.Chown(c.config.Paths.ProxyPidFile, uid, gid); err != nil {
			beIo.StderrF("error changing ownership of: %v - %v\n", c.config.Paths.ProxyPidFile, err)
		}
	}

	if bePath.IsFile(c.config.Paths.RepoPidFile) {
		if err = os.Chown(c.config.Paths.RepoPidFile, uid, gid); err != nil {
			beIo.StderrF("error changing ownership of: %v - %v\n", c.config.Paths.RepoPidFile, err)
		}
	}

	beIo.STDOUT("# filesystem repair completed\n")
	return
}