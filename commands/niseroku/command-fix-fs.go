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
	"os/user"
	"strconv"
	"syscall"

	bePath "github.com/go-enjin/be/pkg/path"
	"github.com/urfave/cli/v2"

	beIo "github.com/go-enjin/enjenv/pkg/io"
)

func (c *Command) actionFixFs(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}

	if syscall.Getuid() != 0 {
		err = fmt.Errorf("niseroku fix-fs requires super user privileges")
		return
	}

	var s *Server
	if s, err = NewServer(c.config); err != nil {
		return
	}

	var u *user.User
	if u, err = user.Lookup(s.Config.RunAs.User); err != nil {
		return
	}
	var g *user.Group
	if g, err = user.LookupGroup(s.Config.RunAs.Group); err != nil {
		return
	}

	var uid, gid int
	if uid, err = strconv.Atoi(u.Uid); err != nil {
		return
	}
	if gid, err = strconv.Atoi(g.Gid); err != nil {
		return
	}

	for _, p := range []string{s.Config.Paths.Etc, s.Config.Paths.Tmp, s.Config.Paths.Var} {
		if err = os.Chown(p, uid, gid); err != nil {
			beIo.StderrF("error changing ownership of: %v - %v\n", p, err)
			continue
		}
		var allDirs []string
		if allDirs, err = bePath.ListAllDirs(p); err != nil {
			beIo.StderrF("error listing all dirs: %v - %v\n", p, err)
			continue
		}
		var allFiles []string
		if allFiles, err = bePath.ListAllFiles(p); err != nil {
			beIo.StderrF("error listing all files: %v - %v\n", p, err)
			continue
		}
		for _, dir := range append(allDirs, allFiles...) {
			if err = os.Chown(dir, uid, gid); err != nil {
				beIo.StderrF("error changing ownership of: %v - %v\n", dir, err)
			}
		}
	}

	if s.Config.LogFile != "" {
		if !bePath.IsFile(s.Config.LogFile) {
			if err = os.WriteFile(s.Config.LogFile, []byte(""), 0660); err != nil {
				beIo.StderrF("error preparing log file: %v - %v\n", s.Config.LogFile, err)
			}
		}
		if err = os.Chown(s.Config.LogFile, uid, gid); err != nil {
			beIo.StderrF("error changing ownership of: %v - %v\n", s.Config.LogFile, err)
		}
	}

	if err = os.Chown(s.Config.Paths.PidFile, uid, gid); err != nil {
		beIo.StderrF("error changing ownership of: %v - %v\n", s.Config.Paths.PidFile, err)
	}

	beIo.STDOUT("# filesystem repair completed\n")
	return
}