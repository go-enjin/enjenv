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

package common

import (
	"fmt"
	"os"
	"os/user"
	"strconv"

	bePath "github.com/go-enjin/be/pkg/path"
)

func GetUidGid(userName, groupName string) (uid, gid int, err error) {
	if userName != "" {
		var u *user.User
		if u, err = user.Lookup(userName); err != nil {
			return
		}
		if uid, err = strconv.Atoi(u.Uid); err != nil {
			return
		}
	}
	if groupName != "" {
		var g *user.Group
		if g, err = user.LookupGroup(groupName); err != nil {
			return
		}
		if gid, err = strconv.Atoi(g.Gid); err != nil {
			return
		}
	}
	return
}

func RepairOwnership(path, userName, groupName string) (err error) {
	var uid, gid int
	if uid, gid, err = GetUidGid(userName, groupName); err != nil {
		return
	}
	var allDirs, allFiles []string
	if allDirs, err = bePath.ListAllDirs(path); err != nil {
		return
	}
	if allFiles, err = bePath.ListAllFiles(path); err != nil {
		return
	}
	if err = os.Chown(path, uid, gid); err != nil {
		err = fmt.Errorf("error chown: %v (%d:%d) - %v", path, uid, gid, err)
		return
	}
	for _, tgt := range append(allDirs, allFiles...) {
		if err = os.Chown(tgt, uid, gid); err != nil {
			err = fmt.Errorf("error chown: %v (%d:%d) - %v", tgt, uid, gid, err)
		}
	}
	return
}