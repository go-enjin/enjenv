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
	"strconv"

	"github.com/shirou/gopsutil/v3/process"
)

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
	if pid, err = GetPidFromFile(pidFile); err != nil {
		return
	}
	proc, err = GetProcessFromPid(pid)
	return
}

func GetPidFromFile(pidFile string) (pid int, err error) {
	var data []byte
	if data, err = os.ReadFile(pidFile); err != nil {
		return
	}
	pid, err = strconv.Atoi(string(data))
	return
}