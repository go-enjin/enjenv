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
	"github.com/shirou/gopsutil/v3/process"
)

func SendSignalToPidFromFile(pidFile string, sig process.Signal) (err error) {
	var proc *process.Process
	if proc, err = GetProcessFromPidFile(pidFile); err == nil {
		err = proc.SendSignal(sig)
	}
	return
}

type SignalErrors []error

func NewSignalErrors() (e *SignalErrors) {
	e = &SignalErrors{}
	return
}

func (e *SignalErrors) Error() (message string) {
	for idx, err := range *e {
		if idx > 0 {
			message += "\n"
		}
		message += err.Error()
	}
	return
}

func (e *SignalErrors) Errors() []error {
	return *e
}

func (e *SignalErrors) Append(errs ...error) {
	*e = append(*e, errs...)
}

func (e *SignalErrors) Len() int {
	return len(*e)
}

func SendSignalToPidTreeFromFile(pidFile string, sig process.Signal) (err error) {
	var proc *process.Process
	if proc, err = GetProcessFromPidFile(pidFile); err == nil {
		err = SendSignalToPidTree(int(proc.Pid), sig)
	}
	return
}

func SendSignalToPidTree(pid int, sig process.Signal) (signalErrors *SignalErrors) {
	se := NewSignalErrors()
	if proc, ee := GetProcessFromPid(pid); ee != nil {
		se.Append(ee)
	} else {
		if children, eee := proc.Children(); eee == nil {
			for _, child := range children {
				if cse := SendSignalToPidTree(int(child.Pid), sig); cse != nil && cse.Len() > 0 {
					se.Append(cse.Errors()...)
				}
			}
		}
		if ee := proc.SendSignal(sig); ee != nil {
			se.Append(ee)
		}
	}
	if se.Len() > 0 {
		signalErrors = se
	}
	return
}
