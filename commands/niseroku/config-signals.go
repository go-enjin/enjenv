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
	"github.com/shirou/gopsutil/v3/process"

	bePath "github.com/go-enjin/be/pkg/path"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

func (c *Config) SignalReverseProxy(sig process.Signal) {
	c.RLock()
	defer c.RUnlock()
	if bePath.IsFile(c.Paths.ProxyPidFile) {
		if proc, err := common.GetProcessFromPidFile(c.Paths.ProxyPidFile); err == nil {
			if running, err := proc.IsRunning(); err == nil && running {
				_ = proc.SendSignal(sig)
			}
		}
	}
	return
}

func (c *Config) SignalGitRepository(sig process.Signal) {
	c.RLock()
	defer c.RUnlock()
	if bePath.IsFile(c.Paths.RepoPidFile) {
		if proc, err := common.GetProcessFromPidFile(c.Paths.RepoPidFile); err == nil {
			if running, err := proc.IsRunning(); err == nil && running {
				_ = proc.SendSignal(sig)
			}
		}
	}
	return
}