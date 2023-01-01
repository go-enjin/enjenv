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
	"time"

	beNet "github.com/go-enjin/be/pkg/net"
	beIo "github.com/go-enjin/enjenv/pkg/io"
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
	domain := r.Host
	s.RLock()
	if origin, exists := s.LookupDomain[domain]; exists {
		s.RUnlock()
		if err := s.Handle(origin, w, r); err != nil {
			beIo.StderrF("error handling origin request: %v\n", err)
		}
	} else {
		s.RUnlock()
		beIo.StderrF("domain not found: %v\n", domain)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("404 - Not Found\n"))
	}
	return
}

func (s *Server) Handle(origin *Application, w http.ResponseWriter, r *http.Request) (err error) {
	var remoteAddr string
	if remoteAddr, err = beNet.GetIpFromRequest(r); err != nil {
		return
	}

	req := r.Clone(r.Context())
	req.Host = r.Host
	req.URL.Host = r.Host
	req.URL.Scheme = origin.Scheme
	req.RequestURI = ""
	req.Header.Set("X-Proxy", "niseroku")
	req.Header.Set("X-Forwarded-For", remoteAddr)

	var originRequestTimeout time.Duration
	if slug := origin.GetThisSlug(); slug == nil {
		err = fmt.Errorf("origin missing this slug: %v\n", origin.Name)
		return
	} else {
		originRequestTimeout = slug.GetOriginRequestTimeout()
	}

	client := http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost: 0,
			IdleConnTimeout: originRequestTimeout,
			DialContext: func(ctx context.Context, network string, addr string) (conn net.Conn, err error) {
				dialer := &net.Dialer{
					LocalAddr: &net.TCPAddr{
						IP:   net.ParseIP(origin.Host),
						Port: 0,
					},
				}
				conn, err = dialer.Dial("tcp", fmt.Sprintf("%s:%d", origin.Host, origin.Port))
				return
			},
		},
	}

	var status int
	defer func() {
		beIo.StdoutF(
			"[%v] %v - %v - (%d) - %v %v\n",
			time.Now().Format("20060102-150405"),
			remoteAddr,
			r.Host,
			status,
			r.Method,
			r.URL.Path,
		)
	}()

	var response *http.Response
	if response, err = client.Do(req); err != nil {
		status = http.StatusServiceUnavailable
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprintf(w, "503 - Service Unavailable")
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