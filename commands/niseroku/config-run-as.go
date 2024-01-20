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
	"os/user"
	"strconv"
)

var (
	gRunAsUidCache = make(map[string]int)
	gRunAsGidCache = make(map[string]int)
)

func (c *Config) RunAsGetUidGid() (uid, gid int, err error) {
	if c.RunAs.User == "" {
		err = fmt.Errorf("run-as.user setting not found")
		return
	} else if c.RunAs.Group == "" {
		err = fmt.Errorf("run-as.group setting not found")
		return
	}
	var ok bool
	if uid, ok = gRunAsUidCache[c.RunAs.User]; !ok {
		var u *user.User
		if u, err = user.Lookup(c.RunAs.User); err != nil {
			return
		} else if uid, err = strconv.Atoi(u.Uid); err != nil {
			return
		}
		gRunAsUidCache[c.RunAs.User] = uid
	}
	if gid, ok = gRunAsGidCache[c.RunAs.Group]; !ok {
		var g *user.Group
		if g, err = user.LookupGroup(c.RunAs.Group); err != nil {
			return
		} else if gid, err = strconv.Atoi(g.Gid); err != nil {
			return
		}
		gRunAsGidCache[c.RunAs.Group] = gid
	}
	return
}

func (c *Config) RunAsChown(paths ...string) (err error) {
	var uid, gid int
	if uid, gid, err = c.RunAsGetUidGid(); err != nil {
		return
	}
	for _, path := range paths {
		if err = os.Chown(path, uid, gid); err != nil {
			return
		}
	}
	return
}
