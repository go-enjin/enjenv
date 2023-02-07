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
)

type Table struct {
	cores    int
	tickPrev int64
	tickThis int64

	data map[int]*CProcess

	sync.RWMutex
}

func New() (t *Table) {
	tick := CpuTick()
	t = &Table{
		tickPrev: tick,
		tickThis: tick,
		cores:    NumCores(),
		data:     make(map[int]*CProcess),
	}
	return
}

func (t *Table) Update(sortByUsage bool) (list []Process, err error) {
	t.markDirty()

	t.tickThis = CpuTick()
	if err = t.updateTableData(); err != nil {
		return
	}

	factor := float32(t.tickThis-t.tickPrev) / float32(t.cores) / 100.0

	for _, v := range t.data {
		if v.Dirty == false && v.Active == true {
			p := Process{v.Pid, v.Ppid, v.Nice, v.Threads, 0}
			if v.TimeCur-v.TimePrev > 0 {
				p.Usage = 1. / factor * float32(v.TimeCur-v.TimePrev)
			}
			list = append(list, p)
		}
	}

	t.tickPrev = t.tickThis
	if sortByUsage {
		sort.Sort(ByUsage(list))
	} else {
		sort.Sort(ByPid(list))
	}
	return
}

func (t *Table) markDirty() {
	t.Lock()
	defer t.Unlock()
	for _, proc := range t.data {
		proc.Dirty = true
		proc.Active = false
	}
}

func (t *Table) updateTableData() (err error) {
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
			p = &CProcess{pid, 0, 0, 0, 0, 0, true, true, 0.}
		}

		p.TimeThis, p.Ppid, p.Nice, p.Threads = GetPidStats(pid)
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