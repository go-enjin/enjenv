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
	"sort"
	"sync"
	"time"

	clpath "github.com/go-corelibs/path"

	"github.com/go-enjin/enjenv/pkg/cpuinfo"
	beIo "github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

type WatchProc struct {
	Name    string
	Hash    string
	Pid     int
	Cpu     float32
	Mem     uint64
	Nice    int
	Ports   []int
	Num     int
	Threads int
	Created uint64
}

type WatchSnapshot struct {
	Stats        cpuinfo.Stats
	Services     []WatchProc
	Applications []WatchProc
}

type Watching struct {
	config *Config

	refresh time.Duration
	fn      func()

	cpulist []cpuinfo.Process
	cpuinfo *cpuinfo.CpuInfo

	snapshot *WatchSnapshot

	stop chan bool

	sync.RWMutex
}

func NewWatching(config *Config, refresh time.Duration, fn func()) (w *Watching, err error) {
	w = new(Watching)
	w.config = config
	w.stop = make(chan bool, 1)
	w.refresh = refresh
	w.fn = fn
	if w.cpuinfo, err = cpuinfo.New(); err != nil {
		return
	}
	w.snapshot = &WatchSnapshot{}
	w.updateCpuInfos()
	w.updateSnapshot()
	time.Sleep(50 * time.Millisecond)
	return
}

func (w *Watching) LogInfoF(format string, argv ...interface{}) {
	beIo.StdoutF("[watcher] "+format, argv...)
}

func (w *Watching) LogErrorF(format string, argv ...interface{}) {
	beIo.StderrF("[watcher] "+format, argv...)
}

func (w *Watching) Snapshot() (s *WatchSnapshot) {
	w.RLock()
	defer w.RUnlock()
	var err error
	var stats cpuinfo.Stats
	if stats, err = w.cpuinfo.GetStats(); err != nil {
		w.LogErrorF("error getting cpu stats: %v", err)
	}
	s = &WatchSnapshot{
		Stats:        stats,
		Services:     append([]WatchProc{}, w.snapshot.Services...),
		Applications: append([]WatchProc{}, w.snapshot.Applications...),
	}
	return
}

func (w *Watching) Start() (err error) {
	go w.watcher()
	return
}

func (w *Watching) Stop() {
	w.stop <- true
}

func (w *Watching) watcher() {
	for {
		w.updateCpuInfos()
		w.updateSnapshot()
		if w.fn != nil {
			w.fn()
		}
		select {
		case <-w.stop:
			// w.LogInfoF("stop signal received")
			return
		case <-time.After(w.refresh):
		}
	}
}

func (w *Watching) updateCpuInfos() {
	w.Lock()
	defer w.Unlock()
	if err := w.cpuinfo.Update(); err != nil {
		w.LogErrorF("error updating cpuinfo: %v", err)
	}
	if list, err := w.cpuinfo.GetProcesses(false); err != nil {
		w.LogErrorF("error getting cpuinfo processes: %v\n", err)
	} else {
		w.cpulist = list
	}
}

func (w *Watching) updateSnapshot() {
	w.Lock()
	defer w.Unlock()

	w.snapshot.Applications = []WatchProc{}
	w.snapshot.Services = []WatchProc{
		{
			Name:    "reverse-proxy",
			Hash:    "",
			Pid:     -1,
			Cpu:     0,
			Mem:     0,
			Nice:    0,
			Num:     0,
			Threads: 0,
			Created: 0,
		},
		{
			Name:    "git-repository",
			Hash:    "",
			Pid:     -1,
			Cpu:     0,
			Mem:     0,
			Nice:    0,
			Num:     0,
			Threads: 0,
			Created: 0,
		},
	}

	// reverse-proxy
	proxyPorts := []int{w.config.Ports.Http}
	if w.config.EnableSSL {
		proxyPorts = append(proxyPorts, w.config.Ports.Https)
	}
	w.updateSnapshotEntry(&w.snapshot.Services[0], w.config.Paths.ProxyPidFile, proxyPorts)

	// git-repository
	w.updateSnapshotEntry(&w.snapshot.Services[1], w.config.Paths.RepoPidFile, []int{w.config.Ports.Git})

	// applications
	for _, app := range w.config.Applications {
		w.snapshot.Applications = append(
			w.snapshot.Applications,
			w.updateSnapshotApplication(app)...,
		)
	}
	sort.Sort(WatchingByUsage(w.snapshot.Applications))
}

func (w *Watching) updateSnapshotApplication(app *Application) (stats []WatchProc) {

	updateSlug := func(slug *Slug, next string) {
		if slug.GetNumWorkers() == 0 {
			stat := WatchProc{
				Name:    app.Name,
				Hash:    next,
				Pid:     -1,
				Cpu:     0,
				Mem:     0,
				Nice:    0,
				Num:     0,
				Threads: 0,
			}
			stats = append(stats, stat)
		} else {
			for _, si := range slug.Workers {
				stat := WatchProc{
					Name:    app.Name,
					Hash:    si.Hash,
					Pid:     -1,
					Cpu:     0,
					Mem:     0,
					Nice:    0,
					Num:     0,
					Threads: 0,
				}
				w.updateSnapshotEntry(&stat, si.PidFile, []int{si.Port})
				stats = append(stats, stat)
			}
		}
	}

	var thisSlug string
	if slug := app.GetThisSlug(); slug != nil {
		thisSlug = slug.Archive
		updateSlug(slug, "0000000000")
	}
	if slug := app.GetNextSlug(); slug != nil {
		if thisSlug != "" && thisSlug != slug.Archive {
			updateSlug(slug, "0000000000")
		}
	}
	return
}

func (w *Watching) updateSnapshotEntry(entry *WatchProc, pidfile string, ports []int) {
	if clpath.IsFile(pidfile) {
		var portsReady []int
		var isRunning, isReady bool
		var pid int = -1
		var num, threads int
		var nice int
		var usage float32 = -1.0
		var mem uint64 = 0
		var created uint64 = 0

		if v, ee := common.GetIntFromFile(pidfile); ee == nil {
			pid = v
			for _, proc := range w.cpulist {
				if isRunning = proc.Pid == pid; isRunning {
					usage, num, threads = w.getProcUsage(pid)
					nice = proc.Nice
					mem = w.getProcMemUsed(pid)
					created = proc.Created
					break
				}
			}
		}

		for _, port := range ports {
			if isReady = common.IsAddressPortOpen(w.config.BindAddr, port); isReady {
				portsReady = append(portsReady, port)
			}
		}

		entry.Pid = pid
		entry.Cpu = usage
		entry.Mem = mem
		entry.Nice = 20 - nice
		entry.Ports = portsReady
		entry.Num = num
		entry.Threads = threads
		entry.Created = created
	}
}

func (w *Watching) getProcMemUsed(pid int) (used uint64) {
	for _, proc := range w.getRelatedProcList(pid) {
		if proc.Pid == pid {
			used += proc.MemUsed
		} else {
			used += w.getProcMemUsed(proc.Pid)
		}
	}
	return
}

func (w *Watching) getProcUsage(pid int) (usage float32, num, threads int) {
	for _, proc := range w.getRelatedProcList(pid) {
		if proc.Pid == pid {
			// is this
			usage += proc.Usage
			num += 1
			threads += proc.Threads
		} else {
			// is related
			u, n, t := w.getProcUsage(proc.Pid)
			usage += u
			num += n
			threads += t
		}
	}
	return
}

func (w *Watching) getRelatedProcList(pid int) (list []cpuinfo.Process) {
	var parent *cpuinfo.Process
	for _, proc := range w.cpulist {
		if proc.Pid == pid {
			parent = &proc
			break
		}
	}
	if parent == nil {
		return
	}
	// parent is always first
	list = append(list, *parent)
	for _, proc := range w.cpulist {
		if proc.Ppid == pid {
			list = append(list, proc)
		}
	}
	return
}
