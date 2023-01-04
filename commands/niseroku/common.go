// Copyright (c) 2022  The Go-Enjin Authors
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
	"encoding/csv"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

var (
	RxSshPubKey = regexp.MustCompile(`^(\S+)\s+(\S+)((?:\s*).*)$`)
)

func parseSshKey(input string) (prefix, data, comment, id string, ok bool) {
	if ok = RxSshPubKey.MatchString(input); ok {
		m := RxSshPubKey.FindAllStringSubmatch(input, 1)
		prefix, data, comment = m[0][1], m[0][2], m[0][3]
		id = prefix + " " + data
	}
	return
}

func parseArgv(input string) (argv []string, err error) {
	r := csv.NewReader(strings.NewReader(input))
	r.Comma = ' '
	argv, err = r.Read()
	return
}

func sendSignalToPidFromFile(pidFile string, sig process.Signal) (err error) {
	var proc *process.Process
	if proc, err = getProcessFromPidFile(pidFile); err == nil {
		err = proc.SendSignal(sig)
	}
	return
}

func getIntFromFile(pidFile string) (pid int, err error) {
	var data []byte
	if data, err = os.ReadFile(pidFile); err != nil {
		return
	}
	pid, err = strconv.Atoi(string(data))
	return
}

func getProcessFromPid(pid int) (proc *process.Process, err error) {
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

func getProcessFromPidFile(pidFile string) (proc *process.Process, err error) {
	var pid int
	if pid, err = getIntFromFile(pidFile); err != nil {
		return
	}
	proc, err = getProcessFromPid(pid)
	return
}

func isAddressPortOpen(host string, port int) (ok bool) {
	ok = isAddressPortOpenWithTimeout(host, port, 100*time.Millisecond)
	return
}

func isAddressPortOpenWithTimeout(host string, port int, timeout time.Duration) (ok bool) {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", address, timeout)
	ok = err == nil && conn != nil
	return
}