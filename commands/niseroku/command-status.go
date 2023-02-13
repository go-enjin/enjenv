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
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/maps"

	"github.com/go-enjin/enjenv/pkg/cpuinfo"
	"github.com/go-enjin/enjenv/pkg/service/common"

	beIo "github.com/go-enjin/enjenv/pkg/io"
)

func (c *Command) actionStatus(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	if err = common.DropPrivilegesTo(c.config.RunAs.User, c.config.RunAs.Group); err != nil {
		return
	}

	var snapshot *WatchSnapshot
	var watching *Watching

	wg := &sync.WaitGroup{}
	wg.Add(1)
	counter := 0
	if watching, err = NewWatching(c.config, 50*time.Millisecond, func() {
		if counter += 1; counter > 2 {
			watching.Stop()
			snapshot = watching.Snapshot()
			wg.Done()
		}
	}); err != nil {
		return
	}
	if err = watching.Start(); err != nil {
		return
	}
	wg.Wait()

	c.statusDisplayWatchingSystem(&snapshot.Stats)
	beIo.STDOUT("\n")
	c.statusDisplayWatchingSnapshot(snapshot)

	if proxyLimits, ee := c.config.CallProxyControlCommand("proxy-limits"); ee == nil {
		beIo.STDOUT("\n")
		c.statusDisplayWatchingProxyLimits(proxyLimits)
	}
	return
}

func (c *Command) statusDisplayWatchingSystem(stats *cpuinfo.Stats) {
	buf := bytes.NewBuffer([]byte(""))
	tw := tabwriter.NewWriter(io.Writer(buf), 8, 2, 2, ' ', tabwriter.FilterHTML)

	var cpuUsage float32
	var cpuList string
	tabs := "\t\t"
	for idx, usage := range stats.CpuUsage {
		cpuUsage += usage
		if idx > 0 {
			cpuList += "\t"
		}
		cpuList += fmt.Sprintf("%0.02f", usage*100.0)
		tabs += "\t"
	}
	cpuUsage = cpuUsage / float32(len(stats.CpuUsage)) * 100.0

	_, _ = tw.Write([]byte(fmt.Sprintf(
		"|\tMEM:\t%v/%v"+tabs+"\t|\n",
		humanize.Bytes(stats.MemUsed*1024),
		humanize.Bytes(stats.MemTotal*1024),
	)))
	_, _ = tw.Write([]byte(fmt.Sprintf("|\tCPU:\t%0.02f\t[\t%v\t]\t|\n", cpuUsage, cpuList)))
	_, _ = tw.Write([]byte(fmt.Sprintf("|\tUptime:\t%v"+tabs+"\t|\n", stats.UptimeString())))

	// Output
	_ = tw.Flush()
	beIo.STDOUT(buf.String())
}

func (c *Command) statusDisplayWatchingSnapshot(snapshot *WatchSnapshot) {
	buf := bytes.NewBuffer([]byte(""))
	tw := tabwriter.NewWriter(io.Writer(buf), 8, 2, 2, ' ', tabwriter.FilterHTML)

	writeEntry := func(stat WatchProc) {
		var pid, ports, nice, cpu, mem, num, threads string
		if stat.Pid <= 0 {
			pid, nice, cpu, mem, num, threads = "-", "-", "-", "-", "-", "-"
		} else {
			pid = strconv.Itoa(stat.Pid)
			nice = fmt.Sprintf("%d", stat.Nice)
			cpu = fmt.Sprintf("%.2f", stat.Cpu)
			mem = fmt.Sprintf("%.2f", stat.Mem)
			num = fmt.Sprintf("%d", stat.Num)
			threads = fmt.Sprintf("%d", stat.Threads)
		}
		for idx, port := range stat.Ports {
			if idx > 0 {
				ports += ","
			}
			ports += strconv.Itoa(port)
		}
		_, _ = tw.Write([]byte(fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", stat.Name, pid, ports, nice, cpu, mem, num, threads)))
	}

	// SERVICES

	_, _ = tw.Write([]byte("[ SERVICE ]\t[ PID ]\t[ PORT ]\t[ PRIORITY ]\t[ CPU ]\t[ MEM ]\t[ PROC ]\t[ THREAD ]\n"))
	for _, stat := range snapshot.Services {
		writeEntry(stat)
	}

	// APPLICATIONS
	_, _ = tw.Write([]byte("\t\t\t\t\t\n"))
	_, _ = tw.Write([]byte("[ APPLICATION ]\t[ PID ]\t[ PORT ]\t[ PRIORITY ]\t[ CPU ]\t[ MEM ]\t[ PROC ]\t[ THREAD ]\n"))
	for _, stat := range snapshot.Applications {
		writeEntry(stat)
	}

	// Output
	_ = tw.Flush()
	beIo.STDOUT(buf.String())
	return
}

func parseProxyLimits(proxyLimits string) (rTotal, dTotal int64, rHosts, rAddrs, dHosts, dAddrs map[string]int64) {
	rHosts = make(map[string]int64)
	rAddrs = make(map[string]int64)
	dHosts = make(map[string]int64)
	dAddrs = make(map[string]int64)

	for _, line := range strings.Split(proxyLimits, "\n") {
		line = strings.TrimSpace(line)
		if parts := strings.Split(line, "="); len(parts) == 2 {
			nameParts := strings.Split(parts[0], ",")
			switch len(nameParts) {
			case 2:
				if nameParts[0] == "host" {
					rHosts[nameParts[1]], _ = strconv.ParseInt(parts[1], 10, 64)
				} else if nameParts[0] == "addr" {
					rAddrs[nameParts[1]], _ = strconv.ParseInt(parts[1], 10, 64)
				}
			case 3:
				if nameParts[0] == "delay" {
					if nameParts[1] == "host" {
						dHosts[nameParts[2]], _ = strconv.ParseInt(parts[1], 10, 64)
					} else if nameParts[1] == "addr" {
						dAddrs[nameParts[2]], _ = strconv.ParseInt(parts[1], 10, 64)
					}
				}
			default:
				if parts[0] == "__total__" {
					rTotal, _ = strconv.ParseInt(parts[1], 10, 64)
				} else if parts[0] == "__delay__" {
					dTotal, _ = strconv.ParseInt(parts[1], 10, 64)
				}
			}
		}
	}

	return
}

func (c *Command) statusDisplayWatchingProxyLimits(proxyLimits string) {
	buf := bytes.NewBuffer([]byte(""))
	tw := tabwriter.NewWriter(io.Writer(buf), 8, 2, 2, ' ', tabwriter.FilterHTML)

	rTotal, dTotal, rHosts, rAddrs, dHosts, dAddrs := parseProxyLimits(proxyLimits)

	_, _ = tw.Write([]byte("[ PROXY LIMITS ]\t[ CURRENT ]\t[ DELAYED ]\n"))
	_, _ = tw.Write([]byte(fmt.Sprintf("(total)\t%d\t%d\n", rTotal, dTotal)))
	if len(rHosts) > 0 {
		_, _ = tw.Write([]byte("\t\t\n"))
		_, _ = tw.Write([]byte("[ HOST LIMITS ]\t[ CURRENT ]\t[ DELAYED ]\n"))
		for _, key := range maps.SortedKeys(rHosts) {
			dHostValue, _ := dHosts[key]
			_, _ = tw.Write([]byte(fmt.Sprintf("%s\t%d\t%d\n", key, rHosts[key], dHostValue)))
		}
	}
	if len(rAddrs) > 0 {
		_, _ = tw.Write([]byte("\t\t\n"))
		_, _ = tw.Write([]byte("[ ADDR LIMITS ]\t[ CURRENT ]\t[ DELAYED ]\n"))
		for _, key := range maps.SortedKeys(rAddrs) {
			dAddrValue, _ := dAddrs[key]
			_, _ = tw.Write([]byte(fmt.Sprintf("%s\t%d\t%d\n", key, rAddrs[key], dAddrValue)))
		}
	}

	_ = tw.Flush()
	beIo.STDOUT(buf.String())
}