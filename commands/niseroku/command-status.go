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

func makeCommandStatus(c *Command, app *cli.App) (cmd *cli.Command) {
	cmd = &cli.Command{
		Name:      "status",
		Usage:     "display the status of all niseroku services",
		UsageText: app.Name + " niseroku status",
		Action:    c.actionStatus,
		Subcommands: []*cli.Command{
			makeCommandStatusWatch(c, app),
		},
	}
	return
}

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
			mem = humanize.Bytes(stat.Mem * 1024)
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

type ParsedProxyLimitsData struct {
	Apps  map[string]int64
	Addrs map[string]int64
	Hosts map[string]int64
	Ports map[string]int64
}

func NewProxyLimitsData() ParsedProxyLimitsData {
	return ParsedProxyLimitsData{
		Apps:  make(map[string]int64),
		Addrs: make(map[string]int64),
		Hosts: make(map[string]int64),
		Ports: make(map[string]int64),
	}
}

type ParsedProxyLimits struct {
	TotalRequest int64
	TotalDelayed int64

	Delayed ParsedProxyLimitsData
	Request ParsedProxyLimitsData
}

func parseProxyLimits(proxyLimits string) (ppl *ParsedProxyLimits) {
	ppl = new(ParsedProxyLimits)
	ppl.Request = NewProxyLimitsData()
	ppl.Delayed = NewProxyLimitsData()

	for _, line := range strings.Split(proxyLimits, "\n") {
		line = strings.TrimSpace(line)
		if parts := strings.Split(line, "="); len(parts) == 2 {
			value := strings.TrimSpace(parts[1])
			names := strings.Split(parts[0], "|")
			switch len(names) {

			case 2:
				switch names[0] {
				case "app":
					ppl.Request.Apps[names[1]], _ = strconv.ParseInt(value, 10, 64)
				case "host":
					ppl.Request.Hosts[names[1]], _ = strconv.ParseInt(value, 10, 64)
				case "addr":
					ppl.Request.Addrs[names[1]], _ = strconv.ParseInt(value, 10, 64)
				case "port":
					ppl.Request.Ports[names[1]], _ = strconv.ParseInt(value, 10, 64)
				}

			case 3:
				if names[0] == "delay" {
					switch names[1] {
					case "app":
						ppl.Delayed.Apps[names[2]], _ = strconv.ParseInt(value, 10, 64)
					case "host":
						ppl.Delayed.Hosts[names[2]], _ = strconv.ParseInt(value, 10, 64)
					case "addr":
						ppl.Delayed.Addrs[names[2]], _ = strconv.ParseInt(value, 10, 64)
					case "port":
						ppl.Delayed.Ports[names[2]], _ = strconv.ParseInt(value, 10, 64)
					}
				}

			default:
				switch names[0] {
				case "__total__":
					ppl.TotalRequest, _ = strconv.ParseInt(value, 10, 64)
				case "__delay__":
					ppl.TotalDelayed, _ = strconv.ParseInt(value, 10, 64)
				}
			}
		}
	}

	return
}

func (c *Command) statusDisplayWatchingProxyLimits(proxyLimits string) {
	buf := bytes.NewBuffer([]byte(""))
	tw := tabwriter.NewWriter(io.Writer(buf), 8, 2, 2, ' ', tabwriter.FilterHTML)

	ppl := parseProxyLimits(proxyLimits)

	_, _ = tw.Write([]byte("[ PROXY LIMITS ]\t[ CURRENT ]\t[ DELAYED ]\n"))
	_, _ = tw.Write([]byte(fmt.Sprintf("(total)\t%d\t%d\n", ppl.TotalRequest, ppl.TotalDelayed)))
	if len(ppl.Request.Hosts) > 0 {
		_, _ = tw.Write([]byte("\t\t\n"))
		_, _ = tw.Write([]byte("[ HOST LIMITS ]\t[ CURRENT ]\t[ DELAYED ]\n"))
		for _, key := range maps.SortedKeys(ppl.Request.Hosts) {
			dHostValue, _ := ppl.Request.Hosts[key]
			_, _ = tw.Write([]byte(fmt.Sprintf("%s\t%d\t%d\n", key, ppl.Request.Hosts[key], dHostValue)))
		}
	}
	if len(ppl.Request.Addrs) > 0 {
		_, _ = tw.Write([]byte("\t\t\n"))
		_, _ = tw.Write([]byte("[ ADDR LIMITS ]\t[ CURRENT ]\t[ DELAYED ]\n"))
		for _, key := range maps.SortedKeys(ppl.Request.Addrs) {
			dAddrValue, _ := ppl.Delayed.Addrs[key]
			_, _ = tw.Write([]byte(fmt.Sprintf("%s\t%d\t%d\n", key, ppl.Request.Addrs[key], dAddrValue)))
		}
	}

	_ = tw.Flush()
	beIo.STDOUT(buf.String())
}