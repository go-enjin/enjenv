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
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/sosedoff/gitkit"
	"golang.org/x/crypto/acme/autocert"

	"github.com/go-enjin/be/pkg/maps"
	bePath "github.com/go-enjin/be/pkg/path"

	beIo "github.com/go-enjin/enjenv/pkg/io"
)

import (
	// #include <unistd.h>
	// #include <errno.h>
	"C"
)

/*

	- starts/stops SSH, HTTP and HTTPS listeners
	- handle os.Signals: INT (stop), HUP (restart) and USR1 (reload)
	- track domain and port lookups for configured applications
	- track available, reserved and active port numbers

*/

type Server struct {
	Config *Config

	LookupApp    map[string]*Application
	LookupPort   map[int]*Application
	LookupDomain map[string]*Application

	running bool

	sock net.Listener
	repo *gitkit.SSH

	http         *http.Server
	httpListener net.Listener

	https         *http.Server
	httpsListener net.Listener
	autocert      *autocert.Manager

	sigint chan os.Signal
	sighup chan os.Signal
	sigusr chan os.Signal

	Name string

	sync.RWMutex
}

func NewServer(config *Config) (s *Server, err error) {
	s = &Server{
		Name:   bePath.Base(config.Source),
		Config: config,
	}
	err = s.LoadApplications()
	return
}

func (s *Server) LogInfoF(format string, argv ...interface{}) {
	format = strings.TrimSpace(format)
	beIo.StdoutF("# [%v] %v\n", s.Name, fmt.Sprintf(format, argv...))
}

func (s *Server) LogError(err error) {
	beIo.StdoutF("# [%v] ERROR %v\n", s.Name, err)
}

func (s *Server) LogErrorF(format string, argv ...interface{}) {
	format = strings.TrimSpace(format)
	beIo.StdoutF("# [%v] ERROR %v\n", s.Name, fmt.Sprintf(format, argv...))
}

func (s *Server) LoadApplications() (err error) {
	var appConfigs []string
	if appConfigs, err = bePath.ListFiles(s.Config.Paths.EtcApps); err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			err = nil
		}
		return
	}
	foundApps := make(map[string]*Application)
	for _, appConfig := range appConfigs {
		var app *Application
		if app, err = NewApplication(appConfig, s.Config); err != nil {
			return
		}
		if _, exists := foundApps[app.Name]; exists {
			err = fmt.Errorf("app already exists: %v (%v)", app.Name, appConfig)
			return
		}
		foundApps[app.Name] = app
	}
	appLookup := make(map[string]*Application)
	portLookup := make(map[int]*Application)
	domainLookup := make(map[string]*Application)
	for _, app := range foundApps {
		if _, exists := appLookup[app.Name]; exists {
			err = fmt.Errorf("app %d duplicated by: %v", app.Name, app.Source)
			return
		} else {
			appLookup[app.Name] = app
		}
		if _, exists := portLookup[app.Origin.Port]; exists {
			err = fmt.Errorf("port %d duplicated by: %v", app.Origin.Port, app.Source)
			return
		} else {
			portLookup[app.Origin.Port] = app
		}
		for _, domain := range app.Domains {
			if _, exists := domainLookup[domain]; exists {
				err = fmt.Errorf("domain %v duplicated by: %v", domain, app.Source)
				return
			} else {
				domainLookup[domain] = app
			}
		}
	}
	s.Lock()
	s.LookupApp = appLookup
	s.LookupPort = portLookup
	s.LookupDomain = domainLookup
	s.Unlock()
	return
}

func (s *Server) Applications() (apps []*Application) {
	s.RLock()
	defer s.RUnlock()
	apps = maps.ValuesSortedByKeys(s.LookupApp)
	return
}

func (s *Server) InitPidFile() (err error) {

	if bePath.IsFile(s.Config.Paths.PidFile) {
		var proc *process.Process
		if proc, err = getProcessFromPidFile(s.Config.Paths.PidFile); err != nil {
			var stale bool
			if stale = strings.HasPrefix(err.Error(), "pid is not running"); stale {
			} else if stale = err.Error() == "process not found"; stale {
			} else if stale = err.Error() == "pid not found"; stale {
			}
			if stale {
				beIo.StdoutF("# removing stale pid file: %v\n", s.Config.Paths.PidFile)
				err = os.Remove(s.Config.Paths.PidFile)
			}
		} else if proc != nil {
			err = fmt.Errorf("already running")
			return
		}
	}

	return
}

func (s *Server) IsRunning() (running bool) {
	s.RLock()
	defer s.RUnlock()
	running = s.running
	return
}

func (s *Server) handleSIGUSR1() {
	s.Lock()
	s.sigusr = make(chan os.Signal, 1)
	s.Unlock()
	signal.Notify(s.sigusr, syscall.SIGUSR1)
	for {
		<-s.sigusr
		var runningSlugs []*Slug
		for _, app := range s.Applications() {
			var slug *Slug
			if slug = app.GetThisSlug(); slug != nil {
				if slug.IsRunning() {
					runningSlugs = append(runningSlugs, slug)
				}
			}
		}
		beIo.StdoutF("SIGUSR1: reload all configurations\n")
		if err := s.LoadApplications(); err != nil {
			beIo.StderrF("error reloading applications: %v\n", err)
		}
		beIo.StdoutF("SIGUSR1: migrating all app slugs\n")
		if err := s.migrateAppSlugs(runningSlugs); err != nil {
			beIo.StderrF("critical error migrating all app slugs: %v\n", err)
			s.Stop()
		} else {
			beIo.StdoutF("SIGUSR1: reloading complete\n")
		}
	}
}

func (s *Server) handleSIGHUP() {
	s.Lock()
	s.sighup = make(chan os.Signal, 1)
	s.Unlock()
	signal.Notify(s.sighup, syscall.SIGHUP)
	for {
		<-s.sighup
		beIo.StdoutF("SIGHUP: restart not implemented\n")
	}
}

func (s *Server) handleSIGINT() {
	s.Lock()
	s.sigint = make(chan os.Signal, 1)
	s.Unlock()
	signal.Notify(s.sigint, os.Interrupt, syscall.SIGTERM)

	<-s.sigint

	s.Lock()
	s.running = false
	s.Unlock()

	if s.sock != nil {
		if ee := s.sock.Close(); ee != nil {
			beIo.StderrF("error closing sock: %v\n", ee)
		}
	}
	if bePath.Exists(s.Config.Paths.Control) {
		if ee := os.Remove(s.Config.Paths.Control); ee != nil {
			beIo.StderrF("error removing control file: %v\n", ee)
		}
	}

	if s.repo != nil {
		if ee := s.repo.Stop(); ee != nil {
			beIo.StderrF("error stopping git: %v\n", ee)
		}
	}

	if s.http != nil {
		if ee := s.http.Shutdown(context.Background()); ee != nil {
			beIo.StderrF("error shutting down http: %v\n", ee)
		}
	}
	if s.https != nil {
		if ee := s.https.Shutdown(context.Background()); ee != nil {
			beIo.StderrF("error shutting down https: %v\n", ee)
		}
	}
	if bePath.IsFile(s.Config.Paths.PidFile) {
		if ee := os.Remove(s.Config.Paths.PidFile); ee != nil {
			beIo.StderrF("error removing pid file: %v", ee)
		}
	}

	for _, app := range s.Applications() {
		for _, slug := range app.Slugs {
			if slug.IsRunning() {
				slug.Stop()
			}
		}
	}
}

func (s *Server) Start() (err error) {
	if s.IsRunning() {
		err = fmt.Errorf("server already running")
		return
	}

	if err = s.Config.PrepareDirectories(); err != nil {
		err = fmt.Errorf("error preparing directories: %v", err)
		return
	}

	if err = s.bindGitListener(); err != nil {
		err = fmt.Errorf("error binding git listener: %v", err)
		return
	}

	// bind all port listeners (without serving them)
	if s.Config.EnableSSL {
		if err = s.bindBothHttpListeners(); err != nil {
			err = fmt.Errorf("error binding http and https listeners: %v", err)
			return
		}
	} else if err = s.bindOnlyHttpListener(); err != nil {
		err = fmt.Errorf("error binding http listener: %v", err)
		return
	}

	if err = os.WriteFile(s.Config.Paths.PidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0660); err != nil {
		err = fmt.Errorf("error writing pid file: %v", err)
		return
	}

	// drop privileges, default www-data user
	if err = s.dropPrivileges(); err != nil {
		err = fmt.Errorf("error dropping privileges: %v", err)
		return
	}

	for _, app := range s.Applications() {
		if err = app.SetupRepo(); err != nil {
			err = fmt.Errorf("error setting up app repo: %v - %v", app.Name, err)
			return
		}
	}

	if err = s.updateGitHookScripts(); err != nil {
		err = fmt.Errorf("error updating git hook scripts: %v", err)
		return
	}

	if bePath.Exists(s.Config.Paths.Control) {
		if err = os.Remove(s.Config.Paths.Control); err != nil {
			err = fmt.Errorf("error removing enjin-proxy sock: %v\n", err)
		}
	}
	if s.sock, err = net.Listen("unix", s.Config.Paths.Control); err != nil {
		return
	}

	// SIGUSR1 handler
	go s.handleSIGUSR1()

	// SIGHUP handler
	go s.handleSIGHUP()

	// SIGINT+TERM handler
	idleConnectionsClosed := make(chan struct{})
	go func() {
		s.handleSIGINT()
		close(idleConnectionsClosed)
	}()

	wg := &sync.WaitGroup{}

	go func() {
		wg.Add(1)
		beIo.StdoutF("starting control service: %v\n", s.Config.Paths.Control)
		if ee := s.sockServe(); ee != nil {
			beIo.StderrF("%v\n", ee)
		}
		wg.Done()
	}()

	go func() {
		wg.Add(1)
		beIo.StdoutF("starting git service: %v\n", s.Config.Ports.Git)
		if ee := s.gitServe(); ee != nil {
			beIo.StderrF("%v\n", ee)
		}
		wg.Done()
	}()

	go func() {
		wg.Add(1)
		beIo.StdoutF("starting http service: %d\n", s.Config.Ports.Http)
		if err = s.httpServe(); err != nil {
			beIo.StderrF("%v\n", err)
		}
		wg.Done()
	}()

	if s.Config.EnableSSL {
		go func() {
			wg.Add(1)
			beIo.StdoutF("starting https service: %d\n", s.Config.Ports.Https)
			if err = s.httpsServe(); err != nil {
				beIo.StderrF("%v\n", err)
			}
			wg.Done()
		}()
	}

	s.Lock()
	s.running = true
	s.Unlock()

	if err = s.startAppSlugs(); err != nil {
		beIo.StderrF("error starting app slugs: %v\n", err)
		err = nil
		s.Stop()
	}

	if wg.Wait(); err == nil {
		<-idleConnectionsClosed
	}
	return
}

func (s *Server) Stop() {
	if s.IsRunning() {
		s.sigint <- syscall.SIGINT
	}
	return
}

func (s *Server) dropPrivileges() (err error) {
	if syscall.Getuid() == 0 {
		beIo.StdoutF("dropping root privileges to %v:%v\n", s.Config.RunAs.User, s.Config.RunAs.Group)
		var u *user.User
		if u, err = user.Lookup(s.Config.RunAs.User); err != nil {
			return
		}
		var g *user.Group
		if g, err = user.LookupGroup(s.Config.RunAs.Group); err != nil {
			return
		}

		var uid, gid int
		if uid, err = strconv.Atoi(u.Uid); err != nil {
			return
		}
		if gid, err = strconv.Atoi(g.Gid); err != nil {
			return
		}

		for _, p := range []string{s.Config.Paths.Etc, s.Config.Paths.Tmp, s.Config.Paths.Var} {
			if err = os.Chown(p, uid, gid); err != nil {
				beIo.StderrF("error changing ownership of: %v - %v\n", p, err)
				continue
			}
			var allDirs []string
			if allDirs, err = bePath.ListAllDirs(p); err != nil {
				beIo.StderrF("error listing all dirs: %v - %v\n", p, err)
				continue
			}
			var allFiles []string
			if allFiles, err = bePath.ListAllDirs(p); err != nil {
				beIo.StderrF("error listing all files: %v - %v\n", p, err)
				continue
			}
			for _, dir := range append(allDirs, allFiles...) {
				if err = os.Chown(dir, uid, gid); err != nil {
					beIo.StderrF("error changing ownership of: %v - %v\n", dir, err)
				}
			}
		}

		if s.Config.LogFile != "" {
			if !bePath.IsFile(s.Config.LogFile) {
				if err = os.WriteFile(s.Config.LogFile, []byte(""), 0660); err != nil {
					beIo.StderrF("error preparing log file: %v - %v\n", s.Config.LogFile, err)
				}
			}
			if err = os.Chown(s.Config.LogFile, uid, gid); err != nil {
				beIo.StderrF("error changing ownership of: %v - %v\n", s.Config.LogFile, err)
			}
		}

		if cerr, errno := C.setgid(C.__gid_t(gid)); cerr != 0 {
			err = fmt.Errorf("set GID error: %v", errno)
			return
		} else if cerr, errno = C.setuid(C.__uid_t(uid)); cerr != 0 {
			err = fmt.Errorf("set UID error: %v", errno)
			return
		}
	}
	return
}