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
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/go-enjin/be/pkg/maps"
	bePath "github.com/go-enjin/be/pkg/path"
	"github.com/urfave/cli/v2"

	beIo "github.com/go-enjin/enjenv/pkg/io"
)

func (c *Command) actionStatus(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}

	watching := NewWatching(c.config, 500*time.Millisecond)
	if err = watching.Start(); err != nil {
		return
	}
	time.Sleep(600 * time.Millisecond)
	snapshot := watching.Snapshot()
	watching.Stop()
	c.statusDisplayWatchingSnapshot(&snapshot)
	_ = os.Remove(c.config.Paths.ProxyDumpStats)
	if c.config.SignalDumpStatsReverseProxy() {
		for i := 0; bePath.IsFile(c.config.Paths.ProxyDumpStats) == false; i++ {
			if i == 10 {
				// dump stats not found within 1s
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		c.statusDisplayWatchingProxyLimits()
	}
	return
}

func (c *Command) statusDisplayWatchingSnapshot(snapshot *WatchSnapshot) {
	buf := bytes.NewBuffer([]byte(""))
	tw := tabwriter.NewWriter(io.Writer(buf), 8, 2, 2, ' ', tabwriter.FilterHTML)

	writeEntry := func(stat WatchProc) {
		var pid, ports, nice, cpu, mem, num string
		if stat.Pid <= 0 {
			pid, nice, cpu, mem, num = "-", "-", "-", "-", "-"
		} else {
			pid = strconv.Itoa(stat.Pid)
			nice = fmt.Sprintf("%d", stat.Nice)
			cpu = fmt.Sprintf("%.2f", stat.Cpu)
			mem = fmt.Sprintf("%.2f", stat.Mem)
			num = fmt.Sprintf("%d (%d)", stat.Num, stat.Threads)
		}
		for idx, port := range stat.Ports {
			if idx > 0 {
				ports += ","
			}
			ports += strconv.Itoa(port)
		}
		_, _ = tw.Write([]byte(fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\n", stat.Name, pid, ports, nice, cpu, mem, num)))
	}

	// SERVICES

	_, _ = tw.Write([]byte("[ SERVICE ]\t[ PID ]\t[ PORTS ]\t[ PRIORITY ]\t[ CPU ]\t[ MEM ]\t[ PROCS ]\n"))
	for _, stat := range snapshot.Services {
		writeEntry(stat)
	}

	// APPLICATIONS
	_, _ = tw.Write([]byte("\t\t\t\t\t\n"))
	_, _ = tw.Write([]byte("[ APPLICATION ]\t[ PID ]\t[ PORTS ]\t[ PRIORITY ]\t[ CPU ]\t[ MEM ]\t[ PROCS ]\n"))
	for _, stat := range snapshot.Applications {
		writeEntry(stat)
	}

	// Output
	_ = tw.Flush()
	beIo.STDOUT(buf.String())
	return
}

func (c *Command) statusDisplayWatchingProxyLimits() {
	buf := bytes.NewBuffer([]byte(""))
	tw := tabwriter.NewWriter(io.Writer(buf), 8, 2, 2, ' ', tabwriter.FilterHTML)

	data, _ := os.ReadFile(c.config.Paths.ProxyDumpStats)
	var rTotal int64
	rHosts := make(map[string]int64)
	rAddrs := make(map[string]int64)
	var dTotal int64
	dHosts := make(map[string]int64)
	dAddrs := make(map[string]int64)
	for _, line := range strings.Split(string(data), "\n") {
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

	beIo.STDOUT("\n")

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