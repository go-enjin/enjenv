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
)

type Slug struct {
	App     *Application
	Name    string
	Commit  string
	Archive string

	Instances []*SlugInstance

	sync.RWMutex
}

func NewSlugFromZip(app *Application, archive string) (slug *Slug, err error) {
	if !bePath.IsFile(archive) {
		err = fmt.Errorf("slug not found: %v", archive)
		return
	}
	slug = &Slug{
		App:     app,
		Name:    bePath.Base(archive),
		Archive: archive,
	}
	if RxSlugArchiveName.MatchString(slug.Archive) {
		m := RxSlugArchiveName.FindAllStringSubmatch(slug.Archive, 1)
		slug.Commit = m[0][2]
	}
	slug.ReloadSlugInstances()
	return
}

func (s *Slug) ReloadSlugInstances() {
	s.Lock()
	defer s.Unlock()

	s.Instances = make([]*SlugInstance, 0)

	hashes := make(map[string]bool)
	if paths, err := bePath.List(s.App.Config.Paths.TmpRun); err == nil {
		for _, path := range paths {
			baseName := filepath.Base(path)
			if strings.HasPrefix(baseName, s.Name) {
				if RxSlugRunningName.MatchString(path) {
					m := RxSlugRunningName.FindAllStringSubmatch(path, 1)
					// appName, commitId, hash, extn := m[0][1], m[0][2], m[0][3], m[0][4]
					hash := m[0][3]
					hashes[hash] = true
				}
			}
		}
	}

	for _, hash := range maps.SortedKeys(hashes) {
		// s.App.LogInfoF("loading slug instance with hash: %v.%v", s.Name, hash)
		if si, err := NewSlugInstanceWithHash(s, hash); err == nil {
			s.Instances = append(s.Instances, si)
		}
	}
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

func (s *Slug) NumInstances() (num int) {
	s.RLock()
	defer s.RUnlock()
	num = len(s.Instances)
	return
}

func (s *Slug) GetPort(idx int) (port int) {
	s.RLock()
	defer s.RUnlock()
	if num := len(s.Instances); num > 0 && idx < num {
		port = s.Instances[idx].Port
	}
	return
}

func (s *Slug) IsRunning(idx int) (running bool) {
	s.RLock()
	defer s.RUnlock()
	if num := len(s.Instances); num > 0 && idx < num {
		running = s.Instances[idx].IsRunning()
	}
	return
}

func (s *Slug) IsReady(idx int) (ready bool) {
	s.RLock()
	defer s.RUnlock()
	if num := len(s.Instances); num > 0 && idx < num {
		ready = s.Instances[idx].IsReady()
	}
	return
}

func (s *Slug) IsRunningReady(idx int) (running, ready bool) {
	s.RLock()
	defer s.RUnlock()
	if num := len(s.Instances); num > 0 && idx < num {
		running, ready = s.Instances[idx].IsRunningReady()
	}
	return
}

func (s *Slug) Stop(idx int) (stopped bool) {
	s.Lock()
	defer s.Unlock()
	if num := len(s.Instances); num > 0 && idx < num {
		stopped = s.Instances[idx].Stop()
		if last := len(s.Instances) - 1; idx < last {
			s.Instances = append(s.Instances[:idx], s.Instances[idx+1:]...)
		} else {
			s.Instances = s.Instances[:idx]
		}
	}
	return
}

func (s *Slug) StopAll() (stopped int) {
	s.Lock()
	defer s.Unlock()
	for _, si := range s.Instances {
		if si.Stop() {
			stopped += 1
		}
	}
	s.Instances = make([]*SlugInstance, 0)
	return
}

func (s *Slug) SendSignal(idx int, signal process.Signal) {
	s.Lock()
	defer s.Unlock()
	if num := len(s.Instances); num > 0 && idx < num {
		if proc, err := s.Instances[idx].GetBinProcess(); err == nil && proc != nil {
			_ = proc.SendSignal(signal)
		}
	}
}

func (s *Slug) SendSignalToAll(signal process.Signal) {
	s.Lock()
	defer s.Unlock()
	for _, si := range s.Instances {
		if proc, err := si.GetBinProcess(); err == nil && proc != nil {
			_ = proc.SendSignal(signal)
		}
	}
}

func (s *Slug) Destroy() (err error) {
	s.Lock()
	defer s.Unlock()
	for _, si := range s.Instances {
		si.Stop()
	}
	if bePath.IsFile(s.Archive) {
		if err = os.Remove(s.Archive); err != nil {
			return
		}
	}
	return
}

func (s *Slug) StartForeground(port int) (err error) {
	var si *SlugInstance
	if si, err = NewSlugInstance(s); err != nil {
		return
	}
	s.Lock()
	s.Instances = append(s.Instances, si)
	s.Unlock()
	err = si.StartForeground(port)
	return
}

func (s *Slug) Start(port int) (err error) {
	var si *SlugInstance
	if si, err = NewSlugInstance(s); err != nil {
		return
	}
	s.Lock()
	s.Instances = append(s.Instances, si)
	s.Unlock()
	err = si.Start(port)
	return
}