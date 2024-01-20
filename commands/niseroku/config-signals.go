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

	pkgIo "github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

func (c *Config) IsReverseProxyRunning() (running bool) {
	if proc, err := common.GetProcessFromPidFile(c.Paths.ProxyPidFile); err == nil {
		running, _ = proc.IsRunning()
	}
	return
}

func (c *Config) SignalReverseProxy(sig process.Signal) (sent bool) {
	err := common.SendSignalToPidFromFile(c.Paths.ProxyPidFile, sig)
	if sent = err == nil; sent {
		pkgIo.StdoutF("# signal (%v) sent to reverse-proxy\n", sig)
	}
	return
}

func (c *Config) SignalReloadReverseProxy() (sent bool) {
	sent = c.SignalReverseProxy(syscall.SIGHUP)
	return
}

func (c *Config) SignalStopReverseProxy() (sent bool) {
	sent = c.SignalReverseProxy(syscall.SIGTERM)
	return
}

func (c *Config) IsGitRepositoryRunning() (running bool) {
	if proc, err := common.GetProcessFromPidFile(c.Paths.RepoPidFile); err == nil {
		running, _ = proc.IsRunning()
	}
	return
}

func (c *Config) SignalGitRepository(sig process.Signal) (sent bool) {
	err := common.SendSignalToPidFromFile(c.Paths.RepoPidFile, sig)
	if sent = err == nil; sent {
		pkgIo.StdoutF("# signal (%v) sent to git-repository\n", sig)
	}
	return
}

func (c *Config) SignalReloadGitRepository() (sent bool) {
	sent = c.SignalGitRepository(syscall.SIGHUP)
	return
}

func (c *Config) SignalStopGitRepository() (sent bool) {
	sent = c.SignalGitRepository(syscall.SIGTERM)
	return
}
