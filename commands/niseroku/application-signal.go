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
	"syscall"

	"github.com/shirou/gopsutil/v3/process"
)

func (a *Application) SendSignal(sig process.Signal) (err error) {
	if thisSlug := a.GetThisSlug(); thisSlug != nil {
		var proc *process.Process
		if proc, err = thisSlug.GetBinProcess(); err == nil {
			err = proc.SendSignal(sig)
		}
	}
	return
}

func (a *Application) SendStopSignal() (err error) {
	err = a.SendSignal(syscall.SIGTERM)
	return
}