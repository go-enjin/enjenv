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

package service

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"

	"github.com/shirou/gopsutil/v3/process"

	bePath "github.com/go-enjin/be/pkg/path"

	"github.com/go-enjin/enjenv/pkg/service/common"
)

type Service struct {
	Name string

	User  string
	Group string

	PidFile string
	LogFile string

	SigINT  chan os.Signal
	SigHUP  chan os.Signal
	SigUSR1 chan os.Signal

	BindFn    func() (err error)
	ServeFn   func() (err error)
	StopFn    func() (err error)
	ReloadFn  func() (err error)
	RestartFn func() (err error)

	this interface{}

	sync.RWMutex
}

func (s *Service) IsRunning() (running bool) {
	s.RLock()
	defer s.RUnlock()
	if proc, err := common.GetProcessFromPidFile(s.PidFile); err != nil {
		return
	} else if running, err = proc.IsRunning(); err != nil {
		running = false
	}
	return
}

func (s *Service) Start() (err error) {

	if bePath.IsFile(s.PidFile) {
		var proc *process.Process
		if proc, err = common.GetProcessFromPidFile(s.PidFile); err != nil {
			var stale bool
			if stale = strings.HasPrefix(err.Error(), "pid is not running"); stale {
			} else if stale = err.Error() == "process not found"; stale {
			} else if stale = err.Error() == "pid not found"; stale {
			}
			if stale {
				s.LogInfoF("removing stale pid file: %v", s.PidFile)
				err = os.Remove(s.PidFile)
			}
		} else if proc != nil {
			err = fmt.Errorf("already running")
			return
		}
	}

	if err = s.BindFn(); err != nil {
		return
	}

	if err = common.DropPrivilegesTo(s.User, s.Group); err != nil {
		return
	}

	if err = os.WriteFile(s.PidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0664); err != nil {
		err = fmt.Errorf("error writing pid file: %v", err)
		return
	}

	err = s.ServeFn()
	return
}

func (s *Service) SendSignal(sig syscall.Signal) (err error) {
	if bePath.IsFile(s.PidFile) {
		err = common.SendSignalToPidFromFile(s.PidFile, sig)
	}
	return
}

func (s *Service) Cleanup() (err error) {
	if bePath.IsFile(s.PidFile) {
		err = os.Remove(s.PidFile)
	}
	return
}