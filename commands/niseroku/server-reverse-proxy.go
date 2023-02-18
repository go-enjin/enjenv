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
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/didip/tollbooth/v7/limiter"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/acme/autocert"

	beNet "github.com/go-enjin/be/pkg/net"
	"github.com/go-enjin/be/pkg/net/serve"
	bePath "github.com/go-enjin/be/pkg/path"
	beIo "github.com/go-enjin/enjenv/pkg/io"
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

func (c *Command) actionReverseProxy(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	rp := NewReverseProxy(c.config)
	if rp.IsRunning() {
		err = fmt.Errorf("reverse-proxy already running")
		return
	}
	err = rp.Start()
	return
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

	if err = rp.config.PrepareDirectories(); err != nil {
		err = fmt.Errorf("error preparing directories: %v", err)
		return
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
			if _, ee := pkgRun.EnjenvBg(rp.config.LogFile, "-", "niseroku", "--config", rp.config.Source, "app", "restart", "--all"); ee != nil {
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
			rp.LogInfoF("stopping applications")
			for _, app := range maps.ValuesSortedByKeys(rp.config.Applications) {
				_ = app.SendStopSignal()
				rp.LogInfoF("stop signal sent to: %v", app.Name)
			}
		} else {
			rp.LogInfoF("not stopping applications")
		}
	}
	return
}

func (rp *ReverseProxy) Stop() (err error) {
	if rp.control != nil {
		if ee := rp.control.Close(); ee != nil {
			rp.LogErrorF("error closing control socket: %v\n", ee)
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
		rp.LogInfoF("shutting down http service")
		if ee := rp.http.Shutdown(nil); ee != nil {
			rp.LogErrorF("error shutting down http server: %v\n", ee)
		}
	}
	if rp.config.EnableSSL && rp.https != nil {
		rp.LogInfoF("shutting down https service")
		if ee := rp.https.Shutdown(nil); ee != nil {
			rp.LogErrorF("error shutting down https server: %v\n", ee)
		}
	}
	return
}

func (rp *ReverseProxy) Reload() (err error) {
	rp.Lock()
	defer rp.Unlock()
	rp.LogInfoF("reverse-proxy reloading\n")
	if err = rp.config.Reload(); err == nil {
		rp.reloadRateLimiter()
		beIo.LogFile = rp.config.LogFile
	}
	return
}

func (rp *ReverseProxy) ServeProxyHTTP(w http.ResponseWriter, r *http.Request) {
	var domain string
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
	origin, exists := rp.config.DomainLookup[domain]
	rp.RUnlock()
	if exists {
		if err := rp.ServeOriginHTTP(origin, w, r); err != nil {
			rp.LogErrorF("error handling origin request: %v\n", err)
		}
		return
	}
	remoteAddr, _ := beNet.GetIpFromRequest(r)
	rp.LogErrorF("host not found: %v - %v (%v)\n", r.Host, r.URL.String(), remoteAddr)
	serve.Serve404(w, r)
	return
}

func (rp *ReverseProxy) ServeOriginHTTP(app *Application, w http.ResponseWriter, r *http.Request) (err error) {
	var remoteAddr string
	if remoteAddr, err = beNet.GetIpFromRequest(r); err != nil {
		return
	}

	var status int
	defer func() {
		app.LogAccessF(status, remoteAddr, r)
	}()

	if app.Maintenance {
		status = http.StatusServiceUnavailable
		serve.Serve503(w, r)
		return
	}

	if err = app.LoadAllSlugs(); err != nil {
		return
	}

	req := r.Clone(r.Context())
	req.Host = r.Host
	req.URL.Host = r.Host
	req.URL.Scheme = app.Origin.Scheme
	req.RequestURI = ""
	req.Header.Set("X-Proxy", "niseroku")
	req.Header.Set("X-Forwarded-For", remoteAddr)

	var originRequestTimeout time.Duration
	if slug := app.GetThisSlug(); slug == nil {
		err = fmt.Errorf("origin missing this-slug: %v\n", app.Name)
		return
	} else {
		originRequestTimeout = slug.GetOriginRequestTimeout()
		running, ready := slug.IsRunningReady()
		switch {
		case running && !ready:
			rp.LogInfoF("origin running and not ready: [503] %v\n", slug.Name)
			status = http.StatusServiceUnavailable
			serve.Serve503(w, r)
			return
		case !running && !ready:
			rp.LogInfoF("origin not running and not ready: [502] %v\n", slug.Name)
			status = http.StatusBadGateway
			serve.Serve502(w, r)
			return
		case !running && ready:
			rp.LogErrorF("origin pidfile error, yet is ready: [port=%d] %v\n", slug.Port, slug.Name)
		case running && ready:
		}
	}

	client := http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost:       0,
			IdleConnTimeout:       originRequestTimeout,
			ResponseHeaderTimeout: originRequestTimeout,
			ExpectContinueTimeout: originRequestTimeout,
			TLSHandshakeTimeout:   originRequestTimeout,
			DialContext: func(ctx context.Context, network string, addr string) (conn net.Conn, err error) {
				conn, err = app.Origin.Dial(app.Origin.Port)
				return
			},
		},
	}

	var response *http.Response
	if response, err = client.Do(req); err != nil {
		rp.LogErrorF("origin request error: %v\n", err)
		status = http.StatusServiceUnavailable
		serve.Serve503(w, r)
		return
	}

	for k, v := range response.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}

	status = response.StatusCode
	w.WriteHeader(response.StatusCode)
	_, err = io.Copy(w, response.Body)
	return
}

func (rp *ReverseProxy) httpServe() (err error) {
	if err = rp.http.Serve(rp.httpListener); err == http.ErrServerClosed {
		err = nil
	} else if err != nil {
		err = fmt.Errorf("error serving http: %v", err)
	}
	return
}

func (rp *ReverseProxy) httpsServe() (err error) {
	if rp.config.EnableSSL && rp.httpsListener != nil {
		if err = rp.https.Serve(rp.httpsListener); err == http.ErrServerClosed {
			err = nil
		} else if err != nil {
			err = fmt.Errorf("error serving https: %v", err)
		}
	}
	return
}