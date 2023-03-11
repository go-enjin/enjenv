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
	"path/filepath"
	"syscall"

	bePath "github.com/go-enjin/be/pkg/path"

	"github.com/go-enjin/enjenv/pkg/service/common"
)

func (c *Config) PrepareDirectories() (err error) {
	c.Lock()
	defer c.Unlock()

	var uid, gid int = -1, -1
	if syscall.Geteuid() == 0 {
		if uid, gid, err = common.GetUidGid(c.RunAs.User, c.RunAs.Group); err != nil {
			return
		}
	}

	if err = common.PerformMkdirChownChmod(
		uid, gid,
		0660, 0770,
		c.Paths.Etc,
		c.Paths.EtcApps,
		c.Paths.EtcUsers,
		c.Paths.Tmp,
		c.Paths.TmpRun,
		c.Paths.TmpClone,
		c.Paths.TmpBuild,
		c.Paths.Var,
		c.Paths.VarLogs,
		c.Paths.VarSlugs,
		c.Paths.VarSettings,
		c.Paths.VarCache,
		c.Paths.VarRepos,
		c.Paths.VarAptRoot,
	); err != nil {
		return
	}

	if bePath.IsFile(c.LogFile) {
		if err = common.PerformChownChmod(uid, gid, 0660, 0770, c.LogFile); err != nil {
			return
		}
	}

	if err = common.PerformMkdirChownChmod(
		uid, gid,
		0600, 0700,
		c.Paths.AptSecrets,
		c.Paths.RepoSecrets,
		c.Paths.ProxySecrets,
	); err != nil {
		return
	}

	for _, app := range c.Applications {
		if app.AptEnjin != nil && app.AptEnjin.Enable {
			for _, section := range []string{"apt-archives", "apt-repository"} {
				for flavour, _ := range app.AptEnjin.Flavours {
					target := filepath.Join(c.Paths.VarAptRoot, app.Name, section, flavour)
					if err = common.PerformMkdirChownChmod(uid, gid, 0660, 0770, target); err != nil {
						return
					}
				}
			}
			target := filepath.Join(c.Paths.AptSecrets, app.Name)
			if err = common.PerformMkdirChownChmod(uid, gid, 0600, 0700, target); err != nil {
				return
			}
		}
	}
	return
}