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
	"bytes"
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/maps"
	bePath "github.com/go-enjin/be/pkg/path"

	beIo "github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

func (c *Command) actionStatus(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}

	buf := bytes.NewBuffer([]byte(""))
	tw := tabwriter.NewWriter(io.Writer(buf), 3, 2, 2, ' ', tabwriter.FilterHTML)
	_, _ = tw.Write([]byte("[ SERVICE ]\t[ PID ]\t[ PORT ]\t[ NICE ]\t[ CPU ]\t[ MEM ]\t[ THREADS ]\n"))
	report := func(name string, running, ready bool, pid, port int, proc *process.Process) {
		cpu, mem, nice, threads := "-", "-", "-", "-"
		if proc != nil {
			cpuPercent, _ := proc.CPUPercent()
			memPercent, _ := proc.MemoryPercent()
			numThreads, _ := proc.NumThreads()
			var numChildren, numChildThreads int
			if children, ee := proc.Children(); ee == nil {
				var childCpuTotal float64
				var childMemTotal float32
				for _, child := range children {
					if cir, ce := child.IsRunning(); ce == nil && cir {
						numChildren += 1
						childThreads, _ := child.NumThreads()
						numChildThreads += int(childThreads)
						if cp, eee := child.CPUPercent(); eee == nil {
							// if childCpuTotal < cp {
							childCpuTotal += cp
							// }
						}
						if mp, eee := child.MemoryPercent(); eee == nil {
							// if childMemTotal < mp {
							childMemTotal += mp
							// }
						}
					}
				}
				cpuPercent += childCpuTotal
				memPercent += childMemTotal
			}
			cpu = fmt.Sprintf("%.01f", cpuPercent)
			mem = fmt.Sprintf("%.01f", memPercent)
			niceVal, _ := proc.Nice()
			nice = fmt.Sprintf("%d", 20-niceVal)
			if numChildren > 0 {
				threads = fmt.Sprintf("%v+(%d/%d)", numThreads, numChildren, numChildThreads)
			} else {
				threads = fmt.Sprintf("%v", numThreads)
			}
		}
		var runningMsg, readyMsg string
		if running {
			runningMsg = strconv.Itoa(pid)
		} else {
			runningMsg = "-"
		}
		if ready {
			readyMsg = strconv.Itoa(port)
		} else {
			readyMsg = "-"
		}
		_, _ = tw.Write([]byte(fmt.Sprintf("%v\t%v\t%v\t%v\t%v\t%v\t%v\n", name, runningMsg, readyMsg, nice, cpu, mem, threads)))
	}

	proxyPid := -1
	proxyPort := -1
	proxyIsRunning := false
	proxyIsReady := false
	var proxyProc *process.Process
	if bePath.IsFile(c.config.Paths.ProxyPidFile) {
		if proc, ee := common.GetProcessFromPidFile(c.config.Paths.ProxyPidFile); ee == nil {
			if running, ee := proc.IsRunning(); ee == nil {
				if proxyIsRunning = running; running {
					proxyPid = int(proc.Pid)
					proxyProc = proc
				}
			}
		}
		if common.IsAddressPortOpen(c.config.BindAddr, c.config.Ports.Http) {
			proxyIsReady = true
			proxyPort = c.config.Ports.Http
		}
	}
	report("reverse-proxy", proxyIsRunning, proxyIsReady, proxyPid, proxyPort, proxyProc)

	repoPid := -1
	repoPort := -1
	repoIsRunning := false
	repoIsReady := false
	var repoProc *process.Process
	if bePath.IsFile(c.config.Paths.RepoPidFile) {
		if proc, ee := common.GetProcessFromPidFile(c.config.Paths.RepoPidFile); ee == nil {
			if running, eee := proc.IsRunning(); eee == nil {
				if repoIsRunning = running; running {
					repoPid = int(proc.Pid)
					repoProc = proc
				}
			}
		}
		if common.IsAddressPortOpen(c.config.BindAddr, c.config.Ports.Git) {
			repoIsReady = true
			repoPort = c.config.Ports.Git
		}
	}
	report("git-repository", repoIsRunning, repoIsReady, repoPid, repoPort, repoProc)

	_, _ = tw.Write([]byte(" \t \t \n"))
	_, _ = tw.Write([]byte("[ APPLICATION ]\t[ PID ]\t[ PORT ]\t[ NICE ]\t[ CPU ]\t[ MEM ]\t[ THREADS ]\n"))
	for _, app := range maps.ValuesSortedByKeys(c.config.Applications) {
		var pid, port int
		var running, ready bool
		var appProc *process.Process
		if thisSlug := app.GetThisSlug(); thisSlug != nil {
			if proc, ee := thisSlug.GetBinProcess(); ee == nil {
				if r, eee := proc.IsRunning(); eee == nil {
					if running = r; r {
						pid = int(proc.Pid)
						appProc = proc
					}
				}
			}
			if ready = thisSlug.IsReady(); ready {
				if port = thisSlug.Port; port <= 0 {
					port = thisSlug.App.Origin.Port
				}
			}
		}
		report(app.Name, running, ready, pid, port, appProc)
	}

	_ = tw.Flush()
	beIo.STDOUT(buf.String())

	return
}