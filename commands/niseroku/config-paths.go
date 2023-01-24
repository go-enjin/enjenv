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

	bePath "github.com/go-enjin/be/pkg/path"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

func (c *Config) PrepareDirectories() (err error) {
	c.Lock()
	defer c.Unlock()

	var uid, gid int
	var chown bool
	if syscall.Geteuid() == 0 {
		if uid, gid, err = common.GetUidGid(c.RunAs.User, c.RunAs.Group); err != nil {
			return
		}
		chown = uid > 0 && gid > 0
	}
	for _, p := range []string{
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
		c.Paths.RepoSecrets,
		c.Paths.ProxySecrets,
	} {
		if err = bePath.Mkdir(p); err != nil {
			err = fmt.Errorf("error preparing directory: %v - %v", p, err)
			return
		}
		if chown {
			if err = os.Chown(p, uid, gid); err != nil {
				return
			}
		}
	}
	if chown && bePath.IsFile(c.LogFile) {
		err = os.Chown(c.LogFile, uid, gid)
	}
	return
}