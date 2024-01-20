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
	"os"
	"os/signal"
	"syscall"
)

func (s *Service) HandleSIGINT() {
	s.Lock()
	s.SigINT = make(chan os.Signal, 1)
	s.Unlock()
	signal.Notify(s.SigINT, syscall.SIGINT, syscall.SIGTERM)
	s.LogInfoF("listening for INT/TERM signals\n")
	switch <-s.SigINT {
	case syscall.SIGINT, syscall.SIGTERM:
		s.LogInfoF("signal received: INT/TERM (shutdown)")
		if err := s.StopFn(); err != nil {
			s.LogErrorF("error stopping during SIGINT/SIGTERM: %v\n", err)
		}
		if err := s.Cleanup(); err != nil {
			s.LogErrorF("error cleaning during SIGINT/SIGTERM: %v\n", err)
		}
	}
}

func (s *Service) HandleSIGHUP() {
	s.Lock()
	s.SigHUP = make(chan os.Signal, 1)
	s.Unlock()
	signal.Notify(s.SigHUP, syscall.SIGHUP)
	s.LogInfoF("listening for HUP signals\n")
	for {
		switch <-s.SigHUP {
		case syscall.SIGHUP:
			s.LogInfoF("signal received: HUP (reload)")
			if err := s.ReloadFn(); err != nil {
				s.LogErrorF("error during SIGHUP: %v\n", err)
			}
		}
	}
}
