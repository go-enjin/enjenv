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
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"github.com/go-enjin/be/pkg/cli/run"
	bePath "github.com/go-enjin/be/pkg/path"
)

type Slug struct {
	App     *Application
	Name    string
	Archive string
	RunPath string
	PidFile string
	LogFile string

	Port int `toml:"-"`

	sync.RWMutex
}

func NewSlugFromZip(app *Application, archive string) (slug *Slug, err error) {
	if !bePath.IsFile(archive) {
		err = fmt.Errorf("slug not found: %v", archive)
		return
	}
	slugName := bePath.Base(archive)
	runPath := app.Config.Paths.TmpRun + "/" + slugName
	pidFile := runPath + ".pid"
	logFile := app.Config.Paths.VarLogs + "/" + app.Name + ".log"

	slug = &Slug{
		App:     app,
		Name:    slugName,
		Archive: archive,
		RunPath: runPath,
		PidFile: pidFile,
		LogFile: logFile,
	}
	return
}

func (s *Slug) Compare(other *Slug) (sameSlug, samePort bool) {
	sameSlug = s.Name == other.Name &&
		s.App.Name == other.App.Name &&
		s.Archive == other.Archive &&
		s.PidFile == other.PidFile &&
		s.RunPath == other.RunPath
	samePort = s.Port == other.Port
	return
}

func (s *Slug) Unpack() (err error) {
	if bePath.IsDir(s.RunPath) {
		s.App.LogInfoF("slug already unpacked: %v\n", s.Name)
		return
	}
	if err = bePath.Mkdir(s.RunPath); err != nil {
		s.App.LogErrorF("error making run path: %v - %v\n", s.RunPath, err)
		return
	}
	s.App.LogInfoF("unzipping: %v - %v\n", s.RunPath, s.Archive)
	var unzipBin string
	if unzipBin, err = exec.LookPath("unzip"); err != nil {
		s.App.LogErrorF("error finding unzip program: %v\n", err)
		return
	}
	if err = run.ExeWith(&run.Options{Path: s.RunPath, Name: unzipBin, Argv: []string{"-qq", s.Archive}}); err != nil {
		s.App.LogErrorF("error executing unzip: %v - %v\n", unzipBin, err)
		return
	}
	return
}

var RxSlugProcfileWebEntry = regexp.MustCompile(`(?ms)^web:\s*(.+?)\s*$`)

func (s *Slug) ReadProcfile() (web string, err error) {
	if bePath.IsDir(s.RunPath) {
		procfile := s.RunPath + "/Procfile"
		if bePath.IsFile(procfile) {
			var data []byte
			if data, err = os.ReadFile(procfile); err != nil {
				return
			}
			procdata := string(data)
			if RxSlugProcfileWebEntry.MatchString(procdata) {
				m := RxSlugProcfileWebEntry.FindAllStringSubmatch(procdata, 1)
				web = m[0][1]
			} else {
				err = fmt.Errorf("slug procfile missing web entry:\n%v", procdata)
			}
		} else {
			err = fmt.Errorf("slug missing Procfile: %v", s.Name)
		}
	} else {
		err = fmt.Errorf("slug is not unpacked yet")
	}
	return
}

func (s *Slug) IsReady() (ready bool) {
	if s.IsRunning() && s.Port > 0 {
		ready = isAddressPortOpen(s.App.Origin.Host, s.Port)
	}
	return
}

func (s *Slug) IsRunning() (running bool) {
	s.RLock()
	defer s.RUnlock()
	if proc, err := getProcessFromPidFile(s.PidFile); err == nil && proc != nil {
		running = proc.Pid > 0
	}
	return
}

func (s *Slug) IsRunningReady() (running, ready bool) {
	s.RLock()
	defer s.RUnlock()
	if proc, ee := getProcessFromPidFile(s.PidFile); ee == nil && proc != nil {
		running = proc.Pid > 0
		ready = isAddressPortOpen(s.App.Origin.Host, s.Port)
	}
	return
}

func (s *Slug) Start(port int) (err error) {
	if s.App.Maintenance {
		s.App.LogInfoF("slug app maintenance mode: %v on port %d\n", s.Name, s.Port)
		return
	}
	if running, ready := s.IsRunningReady(); ready {
		s.Port = port
		s.App.LogInfoF("slug already running and ready: %v on port %d\n", s.Name, s.Port)
		return
	} else if running {
		err = fmt.Errorf("slug already running and not ready: %v\n", s.Name)
		return
	}

	if isAddressPortOpen(s.App.Origin.Host, port) {
		err = fmt.Errorf("port already open by another process")
		return
	}
	s.Port = port

	var web string
	if web, err = s.ReadProcfile(); err != nil {
		return
	}
	s.App.LogInfoF("starting slug: PORT=%d %v (%v)\n", port, web, s.Name)

	environ := append(s.App.OsEnviron(), fmt.Sprintf("PORT=%d", port))
	logfile := s.App.Config.Paths.VarLogs + "/" + s.App.Name + ".log"
	var webCmd string
	var webArgv []string
	var parsedArgs []string
	if parsedArgs, err = parseArgv(web); err != nil {
		err = fmt.Errorf("error parsing Procfile web entry argv: \"%v\"", web)
		return
	}
	switch len(parsedArgs) {
	case 0:
		err = fmt.Errorf("error parsing Procfile web arguments: \"%v\"", web)
	case 1:
		webCmd = parsedArgs[0]
	default:
		webCmd = parsedArgs[0]
		webArgv = parsedArgs[1:]
	}

	// s.App.LogInfoF("%v using log file: %v\n", s.App.Name, logfile)
	// s.App.LogInfoF("%v using pid file: %v\n", s.App.Name, s.PidFile)
	// s.App.LogInfoF("%v environment: %v\n", s.App.Name, environ)

	if pid, ee := run.Daemonize(s.RunPath, webCmd, webArgv, logfile, logfile, environ); ee != nil {
		err = fmt.Errorf("error daemonizing slug: %v", ee)
		return
	} else {
		if err = os.WriteFile(s.PidFile, []byte(strconv.Itoa(pid)), 0660); err != nil {
			err = fmt.Errorf("error writing pidfile: %v - %v", s.PidFile, err)
			return
		} else {
			s.App.LogInfoF("started process %d: %v on port %d\n", pid, s.Name, s.Port)
		}
	}

	return
}

func (s *Slug) Stop() {
	if proc, err := getProcessFromPidFile(s.PidFile); err == nil && proc != nil {
		if err = proc.SendSignal(syscall.SIGTERM); err != nil {
			s.App.LogErrorF("error sending SIGTERM to process: %d\n", proc.Pid)
		} else {
			s.App.LogInfoF("sent SIGTERM to process %d: %v\n", proc.Pid, s.Name)
		}
	}
	if bePath.IsDir(s.RunPath) {
		if err := os.RemoveAll(s.RunPath); err != nil {
			s.App.LogErrorF("error removing slug run path: %v - %v\n", s.RunPath, err)
		} else {
			s.App.LogInfoF("removed slug run path: %v\n", s.RunPath)
		}
	}
	if bePath.IsFile(s.PidFile) {
		if err := os.Remove(s.PidFile); err != nil {
			s.App.LogErrorF("error removing slug pid file: %v - %v\n", s.PidFile, err)
		} else {
			s.App.LogInfoF("removed slug pid file: %v\n", s.PidFile)
		}
	}
}

func (s *Slug) Destroy() (err error) {
	s.Stop()
	if bePath.IsDir(s.RunPath) {
		if err = os.RemoveAll(s.RunPath); err != nil {
			return
		}
	}
	if bePath.IsFile(s.PidFile) {
		if err = os.Remove(s.PidFile); err != nil {
			return
		}
	}
	if bePath.IsFile(s.Archive) {
		if err = os.Remove(s.Archive); err != nil {
			return
		}
	}
	return
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