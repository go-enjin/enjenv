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
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/procfs"

	"github.com/go-enjin/be/pkg/maps"
)

type CpuInfo struct {
	cores    int
	tickPrev int64
	tickThis int64

	data     map[int]*CProcess
	cpusPrev map[int64]procfs.CPUStat
	cpusThis map[int64]procfs.CPUStat

	procfs procfs.FS

	sync.RWMutex
}

func New() (t *CpuInfo, err error) {
	var pfs procfs.FS
	if pfs, err = procfs.NewDefaultFS(); err != nil {
		return
	}
	tick := CpuTick()
	var ps procfs.Stat
	if ps, err = pfs.Stat(); err != nil {
		return
	}
	t = &CpuInfo{
		tickPrev: tick,
		tickThis: tick,
		procfs:   pfs,
		cores:    NumCores(),
		data:     make(map[int]*CProcess),
		cpusPrev: ps.CPU,
		cpusThis: ps.CPU,
	}
	return
}

func (t *CpuInfo) Update() (err error) {
	t.markDirty()
	t.tickPrev = t.tickThis
	t.tickThis = CpuTick()
	t.cpusPrev = t.cpusThis
	err = t.updateTableData()
	return
}

func (t *CpuInfo) getFactor() (factor float32) {
	factor = float32(t.tickThis-t.tickPrev) / float32(t.cores) / 100.0
	return
}

func (t *CpuInfo) GetStats() (stats Stats, err error) {
	var ps procfs.Stat
	if ps, err = t.procfs.Stat(); err != nil {
		return
	}

	var mi procfs.Meminfo
	if mi, err = t.procfs.Meminfo(); err != nil {
		return
	}

	t.cpusThis = ps.CPU

	bootTime := time.Unix(int64(ps.BootTime), 0)
	uptime := time.Now().Sub(bootTime)

	var usages []float32
	for _, idx := range maps.SortedNumbers(ps.CPU) {
		prev := t.cpusPrev[idx]
		this := ps.CPU[idx]

		prevIdle := prev.Idle + prev.Iowait
		thisIdle := this.Idle + this.Iowait

		prevNonIdle := prev.User + prev.Nice + prev.System + prev.IRQ + prev.SoftIRQ + prev.Steal
		thisNonIdle := this.User + this.Nice + this.System + this.IRQ + this.SoftIRQ + this.Steal

		prevTotal := prevIdle + prevNonIdle
		thisTotal := thisIdle + thisNonIdle
		// fmt.Println(PrevIdle, Idle, prevNonIdle, thisNonIdle, prevTotal, thisTotal)

		//  differentiate: actual value minus the previous one
		totald := thisTotal - prevTotal
		idled := thisIdle - prevIdle

		usage := (float32(totald) - float32(idled)) / float32(totald)
		usages = append(usages, usage)
	}

	stats = Stats{
		MemTotal: *mi.MemTotal,
		MemFree:  *mi.MemAvailable,
		MemUsed:  *mi.MemTotal - *mi.MemAvailable,
		CpuUsage: usages,
		Uptime:   uptime,
	}
	return
}

func (t *CpuInfo) GetProcesses(sortByUsage bool) (list []Process, err error) {

	factor := t.getFactor()

	for _, v := range t.data {
		if v.Dirty == false && v.Active == true {
			p := Process{
				Pid:     v.Pid,
				Ppid:    v.Ppid,
				Pgrp:    v.Pgrp,
				Nice:    v.Nice,
				Threads: v.Threads,
				Usage:   0.0,
			}
			if v.TimeThis-v.TimePrev > 0 {
				p.Usage = 1.0 / factor * float32(v.TimeThis-v.TimePrev)
			}
			list = append(list, p)
		}
	}

	if sortByUsage {
		sort.Sort(ByUsage(list))
	} else {
		sort.Sort(ByPid(list))
	}
	return
}

func (t *CpuInfo) markDirty() {
	t.Lock()
	defer t.Unlock()
	for _, proc := range t.data {
		proc.Dirty = true
		proc.Active = false
	}
}

func (t *CpuInfo) updateTableData() (err error) {
	t.Lock()
	defer t.Unlock()

	var allProcFiles []os.DirEntry
	if allProcFiles, err = os.ReadDir("/proc"); err != nil {
		return
	}

	for _, procFile := range allProcFiles {
		name := procFile.Name()
		if name[0] < '0' || name[0] > '9' {
			// not a /proc/<PID> file
			continue
		}

		var pid int // get pid
		if v, ee := strconv.Atoi(name); ee != nil {
			continue
		} else {
			pid = v
		}

		var ok bool
		var p *CProcess // get process instance
		if p, ok = t.data[pid]; ok {
			// update existing
			p.Dirty = false
			p.Active = true
			p.TimePrev = p.TimeThis
		} else {
			// construct new
			p = &CProcess{
				Pid:      pid,
				Ppid:     0,
				Pgrp:     0,
				Nice:     0,
				Threads:  0,
				TimePrev: 0,
				TimeThis: 0,
				Dirty:    true,
				Active:   true,
				Usage:    0.0,
			}
		}

		p.TimeThis, p.Ppid, p.Pgrp, p.Nice, p.Threads = GetPidStats(pid)
		t.data[pid] = p
	}

	// remove inactive processes
	for key, v := range t.data {
		if v.Active == false {
			delete(t.data, key)
		}
	}

	return
}