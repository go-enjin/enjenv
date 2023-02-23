// Copyright (c) 2022  The Go-Enjin Authors
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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"github.com/go-enjin/be/pkg/maps"
	bePath "github.com/go-enjin/be/pkg/path"
	beStrings "github.com/go-enjin/be/pkg/strings"

	"github.com/go-enjin/enjenv/pkg/service/common"
)

type Slug struct {
	App     *Application
	Name    string
	Commit  string
	Archive string

	SettingsFile string

	Settings *SlugSettings
	Workers  map[string]*SlugWorker

	liveHash     int
	liveHashLock *sync.RWMutex

	sync.RWMutex
}

func NewSlugFromZip(app *Application, archive string) (slug *Slug, err error) {
	if !bePath.IsFile(archive) {
		err = fmt.Errorf("slug not found: %v", archive)
		return
	}
	slug = &Slug{
		App:          app,
		Name:         bePath.Base(archive),
		Archive:      archive,
		liveHashLock: &sync.RWMutex{},
	}
	slug.SettingsFile = filepath.Join(app.Config.Paths.TmpRun, slug.Name+".settings")
	if RxSlugArchiveName.MatchString(slug.Archive) {
		m := RxSlugArchiveName.FindAllStringSubmatch(slug.Archive, 1)
		slug.Commit = m[0][2]
	}
	slug.RefreshWorkers()
	return
}

func (s *Slug) String() string {
	s.RLock()
	defer s.RUnlock()
	var workers []string
	for _, worker := range s.Workers {
		workers = append(workers, worker.String())
	}
	return fmt.Sprintf("*%s{\"workers\":[%v]}", s.Name, strings.Join(workers, ","))
}

func (s *Slug) RefreshWorkers() {
	s.Lock()
	defer s.Unlock()

	s.Workers = make(map[string]*SlugWorker)

	if paths, err := bePath.List(s.App.Config.Paths.TmpRun); err == nil {
		for _, path := range paths {
			baseName := filepath.Base(path)
			if strings.HasPrefix(baseName, s.Name) {
				if RxSlugRunningName.MatchString(path) {
					m := RxSlugRunningName.FindAllStringSubmatch(path, 1)
					// appName, commitId, hash, extn := m[0][1], m[0][2], m[0][3], m[0][4]
					hash := m[0][3]
					if _, exists := s.Workers[hash]; !exists {
						if si, ee := NewSlugWorkerWithHash(s, hash); ee == nil {
							s.Workers[hash] = si
						} else {
							s.App.LogErrorF("error loading slug instance: %v [%v] - %v", s.Name, hash, ee)
						}
					}
				}
			}
		}
	}

	if s.Settings == nil {
		s.Settings, _ = NewSlugSettings(s.SettingsFile)
	} else {
		_ = s.Settings.Reload()
	}

	if numWorkers := len(s.Workers); numWorkers == 0 {
		// no workers
		return
	} else if numLive := len(s.Settings.Live); numLive > 0 {
		// already live
		return
	} else if numNext := len(s.Settings.Next); numNext > 0 {
		// already starting
	} else {
		// have workers yet no settings
		s.Settings.Live = maps.SortedKeys(s.Workers)
		_ = s.Settings.Save()
		return
	}

	var live []string
	for _, hash := range s.Settings.Live {
		if _, exists := s.Workers[hash]; exists {
			live = append(live, hash)
		}
	}
	s.Settings.Live = live

	var next []string
	for _, hash := range s.Settings.Live {
		if _, exists := s.Workers[hash]; exists {
			next = append(next, hash)
		}
	}
	s.Settings.Next = next

	_ = s.Settings.Save()
}

func (s *Slug) GetSlugStartupTimeout() (timeout time.Duration) {
	switch {
	case s.App.Timeouts.SlugStartup != nil:
		timeout = *s.App.Timeouts.SlugStartup
	case s.App.Config.Timeouts.SlugStartup > 0:
		timeout = s.App.Config.Timeouts.SlugStartup
	default:
		timeout = DefaultSlugStartupTimeout
	}
	return
}

func (s *Slug) GetOriginRequestTimeout() (timeout time.Duration) {
	switch {
	case s.App.Timeouts.OriginRequest != nil:
		timeout = *s.App.Timeouts.OriginRequest
	case s.App.Config.Timeouts.OriginRequest > 0:
		timeout = s.App.Config.Timeouts.OriginRequest
	default:
		timeout = DefaultOriginRequestTimeout
	}
	return
}

func (s *Slug) GetReadyIntervalTimeout() (timeout time.Duration) {
	switch {
	case s.App.Timeouts.ReadyInterval != nil:
		timeout = *s.App.Timeouts.ReadyInterval
	case s.App.Config.Timeouts.ReadyInterval > 0:
		timeout = s.App.Config.Timeouts.ReadyInterval
	default:
		timeout = DefaultReadyIntervalTimeout
	}
	return
}

func (s *Slug) GetNumWorkers() (num int) {
	s.RLock()
	defer s.RUnlock()
	num = len(s.Workers)
	return
}

func (s *Slug) GetLivePort() (livePort int) {
	s.liveHashLock.RLock()
	defer s.liveHashLock.RUnlock()
	livePort = -1
	var hash string
	if liveCount := len(s.Settings.Live); s.liveHash >= 0 && s.liveHash < liveCount {
		hash = s.Settings.Live[s.liveHash]
	} else if liveCount > 0 {
		s.liveHash = 0
		hash = s.Settings.Live[0]
	} else {
		// no slug instances
		s.App.LogErrorF("slug workers not found, no live ports to give: %v", s.Name)
		return
	}
	if worker, ok := s.Workers[hash]; ok {
		livePort = worker.Port
	} else {
		s.App.LogErrorF("slug worker by live hash not found: %v - %v", hash, s.Name)
	}
	return
}

func (s *Slug) ConsumeLivePort() (consumedPort int) {
	consumedPort = s.GetLivePort()
	s.liveHashLock.Lock()
	defer s.liveHashLock.Unlock()
	s.liveHash += 1
	if liveCount := len(s.Settings.Live); s.liveHash < 0 || s.liveHash > liveCount {
		s.liveHash = 0
	}
	return
}

func (s *Slug) GetInstanceByPid(pid int) (si *SlugWorker) {
	s.RLock()
	defer s.RUnlock()
	for _, worker := range s.Workers {
		if worker.Pid > 0 {
			if worker.Pid == pid {
				si = worker
				return
			}
		}
		if workerPid, _ := worker.GetPid(); workerPid == pid {
			si = worker
			return
		}
	}
	return
}

func (s *Slug) IsRunning(hash string) (running bool) {
	s.RLock()
	defer s.RUnlock()
	if worker, ok := s.Workers[hash]; ok {
		running = worker.IsRunning()
	}
	return
}

func (s *Slug) IsReady(hash string) (ready bool) {
	s.RLock()
	defer s.RUnlock()
	if worker, ok := s.Workers[hash]; ok {
		ready = worker.IsReady()
	}
	return
}

func (s *Slug) IsRunningReady() (running, ready bool) {
	s.RLock()
	defer s.RUnlock()
	for _, worker := range s.Workers {
		ru, re := worker.IsRunningReady()
		if ru && !running {
			running = true
		}
		if re && !ready {
			if ready = true; running {
				return
			}
		}
	}
	return
}

func (s *Slug) StopWorker(hash string) (stopped bool) {
	s.Lock()
	defer s.Unlock()
	if worker, ok := s.Workers[hash]; ok {
		stopped = worker.SendStopSignal()
		delete(s.Workers, hash)
		if idx := beStrings.StringIndexInSlice(hash, s.Settings.Live); idx >= 0 {
			s.Settings.Live = beStrings.RemoveIndexFromStrings(idx, s.Settings.Live)
		}
		_ = s.Settings.Save()
	}
	return
}

func (s *Slug) StopAll() (stopped int) {
	s.Lock()
	defer s.Unlock()
	for _, si := range s.Workers {
		if si.Stop() {
			stopped += 1
		}
	}
	s.Workers = make(map[string]*SlugWorker)
	_ = os.Remove(s.SettingsFile)
	return
}

func (s *Slug) SendSignalToAll(sig process.Signal) {
	s.Lock()
	defer s.Unlock()
	for _, worker := range s.Workers {
		worker.SendSignal(sig)
	}
}

func (s *Slug) Destroy() (err error) {
	s.StopAll()
	s.Lock()
	defer s.Unlock()
	if bePath.IsFile(s.Archive) {
		err = os.Remove(s.Archive)
	}
	return
}

func (s *Slug) StartForegroundWorkers(workersReady chan bool) (err error) {
	if len(s.Settings.Next) > 0 {
		err = fmt.Errorf("already starting next workers: %v", s.Name)
		return
	}

	slugStartupTimeout := s.GetSlugStartupTimeout()
	readyIntervalTimeout := s.GetReadyIntervalTimeout()

	var numReady int
	wg := &sync.WaitGroup{}

	for i := 0; i < s.App.GetWebWorkers(); i++ {

		start := time.Now()

		reservedPort := s.App.Config.GetUnusedPort()
		_ = s.App.Config.ReservePort(reservedPort, s.App)

		var si *SlugWorker
		if si, err = NewSlugWorker(s); err != nil {
			return
		}

		wg.Add(1)
		go func() {
			if ee := si.StartForeground(reservedPort); ee != nil {
				s.App.LogErrorF("slug instance start error: %v [%v] - %v", s.Name, si.Hash, ee)
			}
			wg.Done()
		}()

		go func() {
			s.App.LogInfoF("polling slug startup: %v - %v\n", s.Name, slugStartupTimeout)
			for now := time.Now(); now.Sub(start) < slugStartupTimeout; now = time.Now() {
				if common.IsAddressPortOpenWithTimeout(s.App.Origin.Host, reservedPort, readyIntervalTimeout) {
					if numReady += 1; numReady >= s.App.GetWebWorkers() {
						s.liveHashLock.Lock()
						_ = s.App.Config.PromotePortReservation(reservedPort)
						for _, hash := range s.Settings.Live {
							if worker, ok := s.Workers[hash]; ok {
								if worker.Port > 0 {
									s.App.Config.RemoveFromPortLookup(worker.Port)
								}
								worker.Stop()
							}
						}
						s.Settings.Live = s.Settings.Next
						s.Settings.Next = make([]string, 0)
						if ee := s.Settings.Save(); ee != nil {
							s.App.LogErrorF("error saving settings on all workers ready: %v - %v", s.Name, ee)
						}
						s.liveHashLock.Unlock()
						if workersReady != nil {
							workersReady <- true
						}
					}
					s.App.LogInfoF("slug %d of %d ready: %v [%v] on port %d (%v)\n", numReady, s.App.GetWebWorkers(), s.Name, si.Hash, reservedPort, time.Now().Sub(start))
					return
				}
			}

			s.App.LogInfoF("slug startup timeout reached: %v [%v] on port %d\n", s.Name, si.Hash, reservedPort)
			s.StopWorker(si.Hash)
			err = fmt.Errorf("slug startup timeout reached")
			if workersReady != nil {
				workersReady <- true
			}
		}()

		s.Settings.Next = append(s.Settings.Next, si.Hash)
		if err = s.Settings.Save(); err != nil {
			return
		}
	}

	wg.Wait()
	return
}