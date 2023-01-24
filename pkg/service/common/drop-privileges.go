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

import "C"
import (
	"fmt"
	"os/user"
	"strconv"
	"syscall"
)

import (
	// #include <unistd.h>
	// #include <errno.h>
	"C"
)

func DropPrivilegesTo(userName, groupName string) (err error) {
	if syscall.Getuid() == 0 {

		var u *user.User
		if u, err = user.Lookup(userName); err != nil {
			return
		}
		var g *user.Group
		if g, err = user.LookupGroup(groupName); err != nil {
			return
		}

		// io.StdoutF("# switching user:group to %v:%v\n", userName, groupName)

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