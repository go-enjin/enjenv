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
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	beNet "github.com/go-enjin/be/pkg/net"
)

func (s *Server) httpServe() (err error) {
	if err = s.http.Serve(s.httpListener); err == http.ErrServerClosed {
		err = nil
	} else if err != nil {
		err = fmt.Errorf("error serving http: %v", err)
	}
	return
}

func (s *Server) httpsServe() (err error) {
	if s.Config.EnableSSL {
		if err = s.https.Serve(s.httpsListener); err == http.ErrServerClosed {
			err = nil
		} else if err != nil {
			err = fmt.Errorf("error serving https: %v", err)
		}
	}
	return
}

func (s *Server) Handler(w http.ResponseWriter, r *http.Request) {
	var domain string
	if strings.Contains(r.Host, ":") {
		if h, p, err := net.SplitHostPort(r.Host); err == nil {
			switch {
			case s.Config.EnableSSL && strconv.Itoa(s.Config.Ports.Https) == p:
				domain = h
			case strconv.Itoa(s.Config.Ports.Http) == p:
				domain = h
			}
		} else {
			s.LogErrorF("error parsing request.Host: \"%v\" - %v\n", r.Host, err)
		}
	} else {
		domain = r.Host
	}
	s.RLock()
	origin, exists := s.LookupDomain[domain]
	s.RUnlock()
	if exists {
		if err := s.Handle(origin, w, r); err != nil {
			s.LogErrorF("error handling origin request: %v\n", err)
		}
		return
	}
	remoteAddr, _ := beNet.GetIpFromRequest(r)
	s.LogErrorF("host not found: %v - %v\n", r.Host, r.URL.String(), remoteAddr)
	s.Serve404(w, r)
	return
}

func (s *Server) Handle(app *Application, w http.ResponseWriter, r *http.Request) (err error) {
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
		s.Serve503(w, r)
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
		err = fmt.Errorf("origin missing this slug: %v\n", app.Name)
		return
	} else {
		originRequestTimeout = slug.GetOriginRequestTimeout()
	}

	client := http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost: 0,
			IdleConnTimeout: originRequestTimeout,
			DialContext: func(ctx context.Context, network string, addr string) (conn net.Conn, err error) {
				conn, err = app.Origin.Dial()
				return
			},
		},
	}

	var response *http.Response
	if response, err = client.Do(req); err != nil {
		s.LogErrorF("origin request error: %v\n", err)
		status = http.StatusServiceUnavailable
		s.Serve503(w, r)
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