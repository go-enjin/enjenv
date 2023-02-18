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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/shirou/gopsutil/v3/process"

	"github.com/go-enjin/be/pkg/cli/run"
	bePath "github.com/go-enjin/be/pkg/path"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

type SlugInstance struct {
	Slug *Slug  `toml:"-"`
	Hash string `toml:"hash"`
	Name string `toml:"name"`

	Pid  int `toml:"-"`
	Port int `toml:"port"`

	RunPath  string `toml:"run-path"`
	PidFile  string `toml:"pid-file"`
	PortFile string `toml:"port-file"`
	LogFile  string `toml:"log-file"`

	sync.RWMutex
}

func NewSlugInstance(slug *Slug) (si *SlugInstance, err error) {
	si, err = NewSlugInstanceWithHash(slug, common.UniqueHash())
	return
}

func NewSlugInstanceWithHash(slug *Slug, hash string) (si *SlugInstance, err error) {
	si = &SlugInstance{
		Slug: slug,
		Hash: hash,
		Port: -1,
		Pid:  -1,
	}
	si.Name = slug.Name + "." + si.Hash
	si.RunPath = filepath.Join(slug.App.Config.Paths.TmpRun, si.Name)
	si.PidFile = filepath.Join(slug.App.Config.Paths.TmpRun, si.Name+".pid")
	si.PortFile = filepath.Join(slug.App.Config.Paths.TmpRun, si.Name+".port")
	si.LogFile = filepath.Join(slug.App.Config.Paths.VarLogs, slug.App.Name+".log")
	_, _ = si.GetPid()
	if bePath.IsFile(si.PortFile) {
		if v, ee := common.GetIntFromFile(si.PortFile); ee == nil {
			si.Port = v
		}
	}
	return
}

func (s *SlugInstance) GetPid() (pid int, err error) {
	s.Lock()
	defer s.Unlock()
	if s.Pid <= 0 {
		pid = -1
	}
	if bePath.IsFile(s.PidFile) {
		if pid, err = common.GetIntFromFile(s.PidFile); err == nil {
			s.Pid = pid
		}
	}
	return
}

func (s *SlugInstance) String() (text string) {
	running, ready := s.IsRunningReady()
	text = fmt.Sprintf("{slug=%v,port=%v;running=%v;ready=%v;}", s.Slug.Name, s.Port, running, ready)
	return
}

func (s *SlugInstance) Unpack() (err error) {
	if bePath.IsDir(s.RunPath) {
		s.Slug.App.LogInfoF("slug already unpacked: %v\n", s.Slug.Name)
		return
	}
	if err = bePath.Mkdir(s.RunPath); err != nil {
		s.Slug.App.LogErrorF("error making run path: %v - %v\n", s.RunPath, err)
		return
	}
	s.Slug.App.LogInfoF("unzipping: %v - %v\n", s.RunPath, s.Slug.Archive)
	var unzipBin string
	if unzipBin, err = exec.LookPath("unzip"); err != nil {
		s.Slug.App.LogErrorF("error finding unzip program: %v\n", err)
		return
	}
	if err = run.ExeWith(&run.Options{Path: s.RunPath, Name: unzipBin, Argv: []string{"-qq", s.Slug.Archive}}); err != nil {
		s.Slug.App.LogErrorF("error executing unzip: %v - %v\n", unzipBin, err)
		return
	}
	return
}

func (s *SlugInstance) ReadProcfile() (web string, err error) {
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
			err = fmt.Errorf("slug missing Procfile: %v", s.Slug.Name)
		}
	} else {
		err = fmt.Errorf("slug is not unpacked yet")
	}
	return
}

func (s *SlugInstance) IsReady() (ready bool) {
	s.RLock()
	defer s.RUnlock()
	ready = s.Port > 0 && common.IsAddressPortOpen(s.Slug.App.Origin.Host, s.Port)
	return
}

func (s *SlugInstance) IsRunning() (running bool) {
	s.RLock()
	defer s.RUnlock()
	if proc, err := s.GetBinProcess(); err == nil && proc != nil {
		running = proc.Pid > 0
	}
	return
}

func (s *SlugInstance) IsRunningReady() (running, ready bool) {
	running = s.IsRunning()
	ready = s.IsReady()
	return
}

func (s *SlugInstance) GetBinProcess() (proc *process.Process, err error) {
	proc, err = common.GetProcessFromPidFile(s.PidFile)
	return
}

func (s *SlugInstance) PrepareStart(port int) (webCmd string, webArgv, environ []string, err error) {
	if err = s.Unpack(); err != nil {
		err = fmt.Errorf("error unpacking this slug: %v - %v", s.Slug.Name, err)
		return
	}

	if ready := s.IsReady(); ready && port == s.Port {
		err = fmt.Errorf("slug already running on given port: %v (PORT=%d)", s.Slug.Name, s.Port)
		return
	}

	if common.IsAddressPortOpen(s.Slug.App.Origin.Host, port) {
		err = fmt.Errorf("port already open by another process")
		s.Slug.App.LogErrorF("%v: %d\n", err, port)
		return
	}
	s.Port = port
	if err = os.WriteFile(s.PortFile, []byte(strconv.Itoa(port)), 0660); err != nil {
		err = fmt.Errorf("error writing port-file: %v - %v", s.PortFile, err)
		return
	}

	var web string
	if web, err = s.ReadProcfile(); err != nil {
		err = fmt.Errorf("error reading Procfile: %v", err)
		return
	}

	s.Slug.App.LogInfoF("preparing slug instance: PORT=%d %v (%v)\n", port, web, s.Slug.Name)

	environ = append(s.Slug.App.OsEnviron(), fmt.Sprintf("PORT=%d", port))
	var parsedArgs []string
	if parsedArgs, err = common.ParseControlArgv(web); err != nil {
		err = fmt.Errorf("error parsing Procfile web entry argv: %v \"%v\"", s.Slug.Name, web)
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

func (s *SlugInstance) StartForeground(port int) (err error) {
	var webCmd string
	var webArgv, environ []string
	if webCmd, webArgv, environ, err = s.PrepareStart(port); err != nil {
		if strings.Contains(err.Error(), "slug already running") || strings.Contains(err.Error(), "maintenance mode") {
			s.Slug.App.LogInfoF("%v", err)
			err = nil
			return
		}
		s.Slug.App.LogErrorF("error preparing slug: %v (port=%d) - %v\n", s.Slug.Name, port, err)
		return
	}

	s.Slug.App.LogInfoF("starting slug web process: %v - %v %v\n", s.Slug.App.Name, webCmd, webArgv)
	if err = run.ExeWith(&run.Options{Path: s.RunPath, Name: webCmd, Argv: webArgv, Stdout: s.LogFile, Stderr: s.LogFile, Environ: environ, PidFile: s.PidFile}); err != nil {
		if strings.Contains(err.Error(), "signal: terminated") {
			err = nil
		} else {
			err = fmt.Errorf("error executing slug: %v %v - %v", webCmd, webArgv, err)
		}
	}
	return
}

func (s *SlugInstance) Start(port int) (err error) {
	var webCmd string
	var webArgv, environ []string
	if webCmd, webArgv, environ, err = s.PrepareStart(port); err != nil {
		if strings.Contains(err.Error(), "slug already running") || strings.Contains(err.Error(), "maintenance mode") {
			s.Slug.App.LogInfoF("%v", err)
			err = nil
			return
		}
		s.Slug.App.LogErrorF("error preparing slug: %v (port=%d) - %v\n", s.Slug.Name, port, err)
		return
	}

	s.Slug.App.LogInfoF("backgrounding slug web process: %v - %v %v\n", s.Slug.App.Name, webCmd, webArgv)
	if pid, ee := run.BackgroundWith(&run.Options{Path: s.RunPath, Name: webCmd, Argv: webArgv, Stdout: s.LogFile, Stderr: s.LogFile, Environ: environ}); ee != nil {
		err = fmt.Errorf("error backgrounding slug: %v %v - %v", webCmd, webArgv, ee)
		return
	} else {
		if err = os.WriteFile(s.PidFile, []byte(strconv.Itoa(pid)), 0660); err != nil {
			err = fmt.Errorf("error writing pidfile: %v - %v", s.PidFile, err)
			return
		} else {
			s.Slug.App.LogInfoF("started process %d: %v on port %d\n", pid, s.Slug.Name, s.Port)
		}
	}

	return
}

func (s *SlugInstance) Stop() (stopped bool) {
	if bePath.IsDir(s.RunPath) {
		if err := os.RemoveAll(s.RunPath); err != nil {
			s.Slug.App.LogErrorF("error removing slug run path: %v - %v\n", s.RunPath, err)
		} else {
			s.Slug.App.LogInfoF("removed slug run path: %v\n", s.RunPath)
		}
	}
	if bePath.IsFile(s.PortFile) {
		if err := os.Remove(s.PortFile); err != nil {
			s.Slug.App.LogErrorF("error removing slug port file: %v - %v\n", s.PortFile, err)
		} else {
			s.Slug.App.LogInfoF("removed slug port file: %v\n", s.PortFile)
		}
	}
	if bePath.IsFile(s.PidFile) {
		if proc, err := s.GetBinProcess(); err == nil && proc != nil {
			if err = proc.SendSignal(syscall.SIGTERM); err != nil {
				s.Slug.App.LogErrorF("error sending SIGTERM to process: %d\n", proc.Pid)
			} else if stopped = err == nil; stopped {
				s.Slug.App.LogInfoF("sent SIGTERM to process %d: %v\n", proc.Pid, s.Slug.Name)
			}
		} else if err != nil {
			s.Slug.App.LogErrorF("error getting process from pid file: %v - %v\n", s.PidFile, err)
		}
		if err := os.Remove(s.PidFile); err != nil {
			s.Slug.App.LogErrorF("error removing slug pid file: %v - %v\n", s.PidFile, err)
		} else {
			s.Slug.App.LogInfoF("removed slug pid file: %v\n", s.PidFile)
		}
	}
	return
}

func (s *SlugInstance) Destroy() (err error) {
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
	return
}