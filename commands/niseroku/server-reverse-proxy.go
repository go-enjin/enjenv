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
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/didip/tollbooth/v7/limiter"
	"golang.org/x/crypto/acme/autocert"

	bePath "github.com/go-enjin/be/pkg/path"

	beIo "github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/profiling"
	pkgRun "github.com/go-enjin/enjenv/pkg/run"
	"github.com/go-enjin/enjenv/pkg/service"
)

type ReverseProxy struct {
	service.Service

	config *Config

	http         *http.Server
	httpListener net.Listener

	https         *http.Server
	httpsListener net.Listener
	autocert      *autocert.Manager

	limiter *limiter.Limiter

	tracking *Tracking

	control net.Listener
}

func NewReverseProxy(config *Config) (rp *ReverseProxy) {
	rp = new(ReverseProxy)
	rp.Name = "reverse-proxy"
	rp.User = config.RunAs.User
	rp.Group = config.RunAs.Group
	rp.PidFile = config.Paths.ProxyPidFile
	rp.LogFile = config.LogFile
	rp.config = config
	rp.tracking = NewTracking()
	rp.BindFn = rp.Bind
	rp.ServeFn = rp.Serve
	rp.StopFn = rp.Stop
	rp.ReloadFn = rp.Reload
	return
}

func (rp *ReverseProxy) autocertHostPolicy(_ context.Context, host string) (err error) {
	rp.config.RLock()
	defer rp.config.RUnlock()
	if _, ok := rp.config.DomainLookup[host]; !ok {
		return fmt.Errorf("reverse-proxy: host %q not configured", host)
	}
	return
}

func (rp *ReverseProxy) Bind() (err error) {

	rp.LogInfoF("starting fix-fs process")
	if _, ee := pkgRun.EnjenvBg(rp.config.LogFile, rp.config.LogFile, "niseroku", "fix-fs"); ee != nil {
		rp.LogErrorF("error fixing filesystem: %v", ee)
	}

	rp.Lock()
	defer rp.Unlock()

	handler := rp.ProxyHttpHandler()
	http.Handle("/", handler)

	if rp.config.EnableSSL {
		rp.autocert = &autocert.Manager{
			Cache:      autocert.DirCache(rp.config.Paths.ProxySecrets),
			Prompt:     autocert.AcceptTOS,
			Email:      rp.config.AccountEmail,
			HostPolicy: rp.autocertHostPolicy,
		}
		handler = rp.autocert.HTTPHandler(nil)
	}

	httpAddr := fmt.Sprintf("%v:%d", rp.config.BindAddr, rp.config.Ports.Http)
	rp.http = &http.Server{
		Addr:    httpAddr,
		Handler: handler,
	}
	if rp.httpListener, err = net.Listen("tcp", httpAddr); err != nil {
		return
	}

	if rp.config.EnableSSL {
		httpsAddr := fmt.Sprintf("%v:%d", rp.config.BindAddr, rp.config.Ports.Https)
		rp.https = &http.Server{
			Addr:      httpsAddr,
			TLSConfig: rp.autocert.TLSConfig(),
		}
		rp.httpsListener = rp.autocert.Listener()
	}

	go func() {
		if rp.config.IncludeSlugs.OnStart {
			rp.LogInfoF("restarting all applications")
			var startMode string
			if startMode = "start"; rp.config.RestartSlugsOnStart {
				startMode = "restart"
			}
			if _, ee := pkgRun.EnjenvBg(rp.config.LogFile, "-", "niseroku", "--config", rp.config.Source, "app", startMode, "--all"); ee != nil {
				rp.LogErrorF("error calling niseroku app restart --all: %v\n", ee)
			}
		} else {
			rp.LogInfoF("not starting applications")
		}
	}()

	return
}

func (rp *ReverseProxy) Serve() (err error) {

	if bePath.Exists(rp.config.Paths.ProxyRpcSock) {
		if err = os.Remove(rp.config.Paths.ProxyRpcSock); err != nil {
			err = fmt.Errorf("error removing enjin-proxy sock: %v\n", err)
		}
	}
	if rp.control, err = net.Listen("unix", rp.config.Paths.ProxyRpcSock); err != nil {
		return
	}
	// _ = common.RepairOwnership(rp.config.Paths.ProxyRpcSock, rp.config.RunAs.User, rp.config.RunAs.Group)
	_ = os.Chmod(rp.config.Paths.ProxyRpcSock, 0770)

	go rp.HandleSIGHUP()

	// SIGINT+TERM handler
	idleConnectionsClosed := make(chan struct{})
	go func() {
		rp.HandleSIGINT()
		close(idleConnectionsClosed)
	}()

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		rp.LogInfoF("starting http service: %d\n", rp.config.Ports.Http)
		if err = rp.httpServe(); err != nil {
			rp.LogErrorF("error running http service: %v\n", err)
		}
		wg.Done()
	}()

	if rp.config.EnableSSL {
		wg.Add(1)
		go func() {
			rp.LogInfoF("starting https service: %d\n", rp.config.Ports.Https)
			if err = rp.httpsServe(); err != nil {
				rp.LogErrorF("error running https service: %v\n", err)
			}
			wg.Done()
		}()
	}

	wg.Add(1)
	go func() {
		rp.LogInfoF("starting control service: %v\n", rp.config.Paths.ProxyRpcSock)
		if ee := rp.controlSocketServe(); ee != nil {
			rp.LogErrorF("error running control service: %v\n", ee)
		}
		wg.Done()
	}()

	rp.LogInfoF("all services running")
	if wg.Wait(); err == nil {
		rp.LogInfoF("awaiting idle connections")
		<-idleConnectionsClosed
		rp.LogInfoF("all idle connections closed")
		if rp.config.IncludeSlugs.OnStop {
			_ = rp.config.Reload()
			rp.LogInfoF("stopping applications")
			o, e, _ := pkgRun.EnjenvCmd("niseroku", "--config", rp.config.Source, "app", "stop", "--all")
			if o != "" {
				rp.LogInfoF("%v", o)
			}
			if e != "" {
				rp.LogErrorF("%v", e)
			}
		} else {
			rp.LogInfoF("not stopping applications on shutdown")
		}
	}
	return
}

func (rp *ReverseProxy) Stop() (err error) {
	rp.Lock()
	defer rp.Unlock()
	if rp.control != nil {
		if ee := rp.control.Close(); ee != nil {
			rp.LogErrorF("error closing control socket: %v\n", ee)
		} else {
			rp.LogInfoF("rpc service shutdown")
		}
		if ee := recover(); ee != nil {
			rp.LogErrorF("panic caught control: %v", ee)
		}
		if bePath.IsFile(rp.config.Paths.ProxyRpcSock) {
			if ee := os.Remove(rp.config.Paths.ProxyRpcSock); ee != nil {
				rp.LogErrorF("error removing control socket: %v\n", ee)
			}
		}
	}
	if rp.http != nil {
		if ee := rp.http.Shutdown(context.Background()); ee != nil {
			rp.LogErrorF("error shutting down http server: %v\n", ee)
		} else {
			rp.LogInfoF("http service shutdown")
		}
		if ee := recover(); ee != nil {
			rp.LogErrorF("panic caught http: %v", ee)
		}
	}
	if rp.config.EnableSSL && rp.https != nil {
		if ee := rp.https.Shutdown(context.Background()); ee != nil {
			rp.LogErrorF("error shutting down https server: %v\n", ee)
		} else {
			rp.LogInfoF("https service shutdown")
		}
		if ee := recover(); ee != nil {
			rp.LogErrorF("panic caught https: %v", ee)
		}
	}
	profiling.Stop()
	return
}

func (rp *ReverseProxy) Reload() (err error) {
	rp.Lock()
	defer rp.Unlock()
	rp.LogInfoF("reverse-proxy reloading\n")
	if err = rp.config.Reload(); err == nil {
		rp.reloadRateLimiter()
		if beIo.LogFile != rp.config.LogFile {
			beIo.LogFile = rp.config.LogFile
		}
		for _, app := range rp.config.Applications {
			if thisSlug := app.GetThisSlug(); thisSlug != nil {
				thisSlug.RefreshWorkers()
			} else {
				rp.LogInfoF("this slug not found: %v", app.Name)
			}
			if nextSlug := app.GetNextSlug(); nextSlug != nil {
				nextSlug.RefreshWorkers()
			}
		}
	}
	return
}

func (rp *ReverseProxy) GetAppDomain(r *http.Request) (domain string, app *Application, ok bool) {
	if strings.Contains(r.Host, ":") {
		if h, p, err := net.SplitHostPort(r.Host); err == nil {
			switch {
			case rp.config.EnableSSL && strconv.Itoa(rp.config.Ports.Https) == p:
				domain = h
			case strconv.Itoa(rp.config.Ports.Http) == p:
				domain = h
			}
		} else {
			rp.LogErrorF("error parsing request.Host: \"%v\" - %v\n", r.Host, err)
		}
	} else {
		domain = r.Host
	}
	rp.RLock()
	defer rp.RUnlock()
	app, ok = rp.config.DomainLookup[domain]
	return
}

func (rp *ReverseProxy) httpServe() (err error) {
	if err = rp.http.Serve(rp.httpListener); errors.Is(err, http.ErrServerClosed) {
		err = nil
	} else if err != nil {
		err = fmt.Errorf("error serving http: %v", err)
	}
	return
}

func (rp *ReverseProxy) httpsServe() (err error) {
	if rp.config.EnableSSL && rp.httpsListener != nil {
		if err = rp.https.Serve(rp.httpsListener); errors.Is(err, http.ErrServerClosed) {
			err = nil
		} else if err != nil {
			err = fmt.Errorf("error serving https: %v", err)
		}
	}
	return
}