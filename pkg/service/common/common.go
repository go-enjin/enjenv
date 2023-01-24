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
	"encoding/csv"
	"fmt"
	"net"
	"os/user"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	beIo "github.com/go-enjin/enjenv/pkg/io"
	pkgRun "github.com/go-enjin/enjenv/pkg/run"
)

var (
	RxSshPubKey = regexp.MustCompile(`^(\S+)\s+(\S+)((?:\s*).*)$`)
)

func ParseSshKey(input string) (prefix, data, comment, id string, ok bool) {
	if ok = RxSshPubKey.MatchString(input); ok {
		m := RxSshPubKey.FindAllStringSubmatch(input, 1)
		prefix, data, comment = m[0][1], m[0][2], m[0][3]
		id = prefix + " " + data
	}
	return
}

func ParseControlArgv(input string) (argv []string, err error) {
	r := csv.NewReader(strings.NewReader(input))
	r.Comma = ' '
	argv, err = r.Read()
	return
}

func SendSignalToPidFromFile(pidFile string, sig process.Signal) (err error) {
	var proc *process.Process
	if proc, err = GetProcessFromPidFile(pidFile); err == nil {
		err = proc.SendSignal(sig)
	}
	return
}

func GetProcessFromPid(pid int) (proc *process.Process, err error) {
	var running bool
	if proc, err = process.NewProcess(int32(pid)); err != nil {
		err = fmt.Errorf("process not found")
		return
	} else if running, err = proc.IsRunning(); err != nil {
		return
	} else if !running {
		proc = nil
		err = fmt.Errorf("pid not found")
		return
	}
	return
}

func GetProcessFromPidFile(pidFile string) (proc *process.Process, err error) {
	var pid int
	if pid, err = pkgRun.GetPidFromFile(pidFile); err != nil {
		return
	}
	proc, err = GetProcessFromPid(pid)
	return
}

func IsAddressPortOpen(host string, port int) (ok bool) {
	ok = IsAddressPortOpenWithTimeout(host, port, 100*time.Millisecond)
	return
}

func IsAddressPortOpenWithTimeout(host string, port int, timeout time.Duration) (ok bool) {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", address, timeout)
	ok = err == nil && conn != nil
	return
}

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

		beIo.StdoutF("switching user:group to %v:%v\n", userName, groupName)

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