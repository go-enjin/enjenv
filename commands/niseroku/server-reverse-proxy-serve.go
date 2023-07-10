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
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-enjin/be/pkg/net/serve"
)

func (rp *ReverseProxy) ServeOriginHTTP(app *Application, forwardFor string, w http.ResponseWriter, r *http.Request) (status int, err error) {

	if app.Maintenance {
		status = http.StatusServiceUnavailable
		serve.Serve503(w, r)
		return
	}

	req := r.Clone(r.Context())
	req.Host = r.Host
	req.URL.Host = r.Host
	req.URL.Scheme = app.Origin.Scheme
	req.RequestURI = ""
	req.Header.Set("X-Proxy", "niseroku")
	req.Header.Set("X-Forwarded-For", forwardFor)

	var slug *Slug
	var slugPort int
	var originRequestTimeout time.Duration
	if slug = app.GetThisSlug(); slug == nil {
		err = fmt.Errorf("origin missing this-slug: %v", app.Name)
		return
	} else {

		running, ready := slug.IsRunningReady()
		switch {
		case !running && !ready:
			for i := 0; i < 100; i++ {
				time.Sleep(100 * time.Millisecond)
				if slug = app.GetThisSlug(); slug != nil {
					if running, ready = slug.IsRunningReady(); running && ready {
						break
					}
				}
			}
			if !running {
				status = http.StatusBadGateway
				rp.LogInfoF("origin not running and not ready: [502] %v\n", slug.Name)
				serve.Serve502(w, r)
				return
			} else if !ready {
				status = http.StatusServiceUnavailable
				rp.LogInfoF("origin running and not ready: [503] %v\n", slug.Name)
				serve.Serve503(w, r)
				return
			}
		case !running && ready:
			rp.LogErrorF("origin pidfile error, yet is ready: [port=%d] %v\n", slugPort, slug.Name)
		case running && ready:
		}

		slugPort = slug.ConsumeLivePort()
		originRequestTimeout = slug.GetOriginRequestTimeout()
	}

	var response *http.Response
	if response, err = rp.proxyClientRequest(req, app, slugPort, originRequestTimeout); err != nil {
		if strings.Contains(err.Error(), "connection reset by peer") {
			time.Sleep(100 * time.Millisecond)
			response, err = rp.proxyClientRequest(req, app, slugPort, originRequestTimeout)
		}
	}
	if err != nil {
		return
	}

	for k, v := range response.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}

	status = response.StatusCode
	w.WriteHeader(status)
	// prevent 204 responses from having any body
	if serve.StatusHasBody(status) {
		var body []byte
		if body, err = io.ReadAll(response.Body); err != nil {
			rp.LogErrorF("error reading response.Body: %v -- %v", err, req)
			status = http.StatusInternalServerError
			serve.Serve500(w, r)
			err = nil
			return
		}
		_, err = w.Write(body)
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return
}