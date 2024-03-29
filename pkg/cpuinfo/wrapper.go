// Copyright (c) 2018  Patrick Wieschollek - https://github.com/PatWie
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
//
// Notice: this package is based on https://github.com/PatWie/cpuinfo
//         there is no licensing posted, treating as Public Domain

package cpuinfo

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	gopsutil_host "github.com/shirou/gopsutil/v3/host"
	"github.com/tklauser/go-sysconf"
)

// #include "wrapper.h"
import "C"

var (
	gSystemBootTime   uint64 = 0
	gSystemClockTicks int64  = 0
)

func BootEpoch() (epoch uint64) {
	if gSystemBootTime == 0 {
		gSystemBootTime, _ = gopsutil_host.BootTime()
	}
	return gSystemBootTime
}

func BootTime() time.Time {
	return time.Unix(int64(BootEpoch()), 0)
}

func ClockTicks() (ticksPerSecond int64) {
	if gSystemClockTicks == 0 {
		gSystemClockTicks, _ = sysconf.Sysconf(sysconf.SC_CLK_TCK)
	}
	return gSystemClockTicks
}

func CpuTick() (t int64) {
	return int64(C.read_cpu_tick())
}

func TimeFromPid(pid int) (t int64) {
	return int64(C.read_time_from_pid(C.int(pid)))
}

func NumCores() (n int) {
	return int(C.num_cores())
}

func GetPidStats(pid int) (t int64, st uint64, ppid, pgrp, nice, threads int) {
	cppid := C.int(0)
	cpgrp := C.int(0)
	ctime := C.ulong(0)
	cnice := C.int(0)
	cthreads := C.int(0)
	cstarttime := C.ulong(0)
	if ok := C.read_stat_from_pid(C.int(pid), &cppid, &cpgrp, &ctime, &cnice, &cthreads, &cstarttime); ok > 0 {
		t = int64(ctime)
		ppid = int(cppid)
		pgrp = int(cpgrp)
		nice = int(cnice)
		threads = int(cthreads)
		st = uint64(cstarttime) // / uint64(CpuTick())
	}
	return
}

func GetMemStats(pid int) (used uint64) {
	if fh, err := os.OpenFile(fmt.Sprintf("/proc/%d/smaps", pid), os.O_RDONLY, 0); err == nil {
		s := bufio.NewScanner(fh)
		s.Split(bufio.ScanLines)
		for s.Scan() {
			if text := s.Text(); strings.HasPrefix(text, "Rss:") || strings.HasPrefix(text, "SwapRss:") {
				if parts := strings.Fields(text); len(parts) >= 2 {
					v, _ := strconv.ParseUint(parts[1], 10, 64)
					used += v
				}
			}
		}
	}
	return
}
