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

	"github.com/go-enjin/enjenv/pkg/service/common"
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

func (s *Slug) String() (text string) {
	running, ready := s.IsRunningReady()
	text = fmt.Sprintf("{slug=%v,port=%v;running=%v;ready=%v;}", s.Name, s.Port, running, ready)
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
	// TODO: ReadProcfile needs to return a map[string]string containing all Procfile entries
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
	port := s.Port
	if port <= 0 {
		port = s.App.Origin.Port
	}
	ready = common.IsAddressPortOpen(s.App.Origin.Host, port)
	return
}

func (s *Slug) IsRunning() (running bool) {
	s.RLock()
	defer s.RUnlock()
	if proc, err := s.GetBinProcess(); err == nil && proc != nil {
		running = proc.Pid > 0
	}
	return
}

func (s *Slug) IsRunningReady() (running, ready bool) {
	running = s.IsRunning()
	ready = s.IsReady()
	return
}

func (s *Slug) GetBinProcess() (proc *process.Process, err error) {
	proc, err = common.GetProcessFromPidFile(s.PidFile)
	// if proc, err = GetProcessFromPidFile(s.PidFile); err == nil && proc != nil {
	// 	if s.App.BinName != "" {
	// 		var ee error
	// 		var procName string
	// 		if procName, ee = proc.Name(); ee == nil {
	// 			if procName = filepath.Base(procName); procName == s.App.BinName {
	// 				// top process is the app binary
	// 				return
	// 			}
	// 		}
	// 		var children []*process.Process
	// 		if children, ee = proc.Children(); ee == nil {
	// 			for _, child := range children {
	// 				if childName, childErr := child.Name(); childErr == nil {
	// 					if childName = filepath.Base(childName); childName == s.App.BinName {
	// 						proc = child
	// 						return
	// 					}
	// 				}
	// 			}
	// 		}
	// 	}
	// }
	return
}

func (s *Slug) PrepareStart(port int) (webCmd string, webArgv, environ []string, err error) {

	if running, ready := s.IsRunningReady(); ready {
		s.Port = port
		err = fmt.Errorf("slug already running and ready: %v (PORT=%d)", s.Name, s.Port)
		return
	} else if running {
		err = fmt.Errorf("slug already running and not ready: %v", s.Name)
		return
	}

	if common.IsAddressPortOpen(s.App.Origin.Host, port) {
		err = fmt.Errorf("port already open by another process")
		s.App.LogErrorF("%v: %d\n", err, port)
		return
	}
	s.Port = port

	var web string
	if web, err = s.ReadProcfile(); err != nil {
		err = fmt.Errorf("error reading Procfile: %v", err)
		return
	}

	s.App.LogInfoF("preparing slug: PORT=%d %v (%v)\n", port, web, s.Name)

	environ = append(s.App.OsEnviron(), fmt.Sprintf("PORT=%d", port))
	var parsedArgs []string
	if parsedArgs, err = common.ParseControlArgv(web); err != nil {
		err = fmt.Errorf("error parsing Procfile web entry argv: %v \"%v\"", s.Name, web)
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
	if found, _ := exec.LookPath(webCmd); found != "" {
		webCmd = found
	}

	// s.App.LogInfoF("%v using log file: %v\n", s.App.Name, logfile)
	// s.App.LogInfoF("%v using pid file: %v\n", s.App.Name, s.PidFile)
	// s.App.LogInfoF("%v environment: %v\n", s.App.Name, environ)
	return
}

func (s *Slug) StartForeground(port int) (err error) {
	if err = s.Unpack(); err != nil {
		err = fmt.Errorf("error unpacking this slug: %v - %v", s.Name, err)
		return
	}
	var webCmd string
	var webArgv, environ []string
	if webCmd, webArgv, environ, err = s.PrepareStart(port); err != nil {
		if strings.Contains(err.Error(), "slug already running") || strings.Contains(err.Error(), "maintenance mode") {
			s.App.LogInfoF("%v", err)
			err = nil
			return
		}
		s.App.LogErrorF("error preparing slug: %v (port=%d) - %v\n", s.Name, port, err)
		return
	}

	s.App.LogInfoF("starting slug web process: %v - %v %v\n", s.App.Name, webCmd, webArgv)
	if err = run.ExeWith(&run.Options{Path: s.RunPath, Name: webCmd, Argv: webArgv, Stdout: s.LogFile, Stderr: s.LogFile, Environ: environ, PidFile: s.PidFile}); err != nil {
		if strings.Contains(err.Error(), "signal: terminated") {
			err = nil
		} else {
			err = fmt.Errorf("error executing slug: %v %v - %v", webCmd, webArgv, err)
		}
	}
	return
}

func (s *Slug) Start(port int) (err error) {
	if err = s.Unpack(); err != nil {
		err = fmt.Errorf("error unpacking this slug: %v - %v", s.Name, err)
		return
	}
	var webCmd string
	var webArgv, environ []string
	if webCmd, webArgv, environ, err = s.PrepareStart(port); err != nil {
		if strings.Contains(err.Error(), "slug already running") || strings.Contains(err.Error(), "maintenance mode") {
			s.App.LogInfoF("%v", err)
			err = nil
			return
		}
		s.App.LogErrorF("error preparing slug: %v (port=%d) - %v\n", s.Name, port, err)
		return
	}

	s.App.LogInfoF("backgrounding slug web process: %v - %v %v\n", s.App.Name, webCmd, webArgv)
	if pid, ee := run.BackgroundWith(&run.Options{Path: s.RunPath, Name: webCmd, Argv: webArgv, Stdout: s.LogFile, Stderr: s.LogFile, Environ: environ}); ee != nil {
		err = fmt.Errorf("error backgrounding slug: %v %v - %v", webCmd, webArgv, ee)
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

func (s *Slug) Stop() (stopped bool) {
	if bePath.IsFile(s.PidFile) {
		if proc, err := s.GetBinProcess(); err == nil && proc != nil {
			if err = proc.SendSignal(syscall.SIGTERM); err != nil {
				s.App.LogErrorF("error sending SIGTERM to process: %d\n", proc.Pid)
			} else if stopped = err == nil; stopped {
				s.App.LogInfoF("sent SIGTERM to process %d: %v\n", proc.Pid, s.Name)
			}
		} else if err != nil {
			s.App.LogErrorF("error getting process from pid file: %v - %v\n", s.PidFile, err)
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
	return
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