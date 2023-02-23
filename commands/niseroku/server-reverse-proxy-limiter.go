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
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth/v7/limiter"
	"github.com/kataras/requestid"

	beNet "github.com/go-enjin/be/pkg/net"
	"github.com/go-enjin/be/pkg/net/serve"
)

var (
	DefaultProxyLimitsStatLifetime = time.Second
)

func (rp *ReverseProxy) initRateLimiter() {
	if rp.limiter != nil {
		return
	}
	rp.limiter = tollbooth.NewLimiter(
		rp.config.ProxyLimit.Max,
		&limiter.ExpirableOptions{
			DefaultExpirationTTL: rp.config.ProxyLimit.TTL,
		},
	)
	if rp.config.ProxyLimit.Burst > 0 {
		rp.limiter.SetBurst(rp.config.ProxyLimit.Burst)
	}
	rp.limiter.SetStatusCode(http.StatusTooManyRequests)
	rp.limiter.SetMessage("429 - Too Many Requests")
	rp.limiter.SetMessageContentType("text/plain; charset=utf-8")
}

func (rp *ReverseProxy) reloadRateLimiter() {
	if rp.limiter == nil {
		rp.initRateLimiter()
		return
	}
	rp.limiter.SetMax(rp.config.ProxyLimit.Max)
	if rp.config.ProxyLimit.Burst > 0 {
		rp.limiter.SetBurst(rp.config.ProxyLimit.Burst)
	} else {
		rp.limiter.SetBurst(int(math.Max(1, rp.config.ProxyLimit.Max)))
	}
}

func (rp *ReverseProxy) ProxyHttpHandler() (h http.Handler) {
	rp.initRateLimiter()
	return requestid.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		var err error
		var exists bool
		var thisSlug *Slug
		var portKey string
		var app *Application

		remoteAddr := "<nil>"
		if addr, ee := beNet.GetIpFromRequest(r); ee == nil {
			remoteAddr = addr
		}

		if _, app, exists = rp.GetAppDomain(r); exists {
			if thisSlug = app.GetThisSlug(); thisSlug != nil {
				_ = thisSlug.Settings.Reload()
				running, ready := thisSlug.IsRunningReady()
				if !running || !ready {
					for i := 0; i < 20; i++ {
						time.Sleep(500 * time.Millisecond)
						_, app, _ = rp.GetAppDomain(r)
						if thisSlug = app.GetThisSlug(); thisSlug != nil {
							rp.LogInfoF("limiter polling [%d] slug running+ready: %v", i, thisSlug.Name)
							if running, ready = thisSlug.IsRunningReady(); running && ready {
								break
							}
						}
					}
				}
				if !running || !ready {
					err = fmt.Errorf("slug not running or not ready")
				} else if port := thisSlug.GetLivePort(); port > 0 {
					portKey = "port," + strconv.Itoa(port)
					err = nil
				} else {
					err = fmt.Errorf("slug has no live ports")
				}
			} else {
				err = fmt.Errorf("app slug not found")
			}
		} else {
			err = fmt.Errorf("app domain not found")
		}
		if err != nil {
			rp.LogErrorF("proxy error: %v %v (%v) - %v\n", r.Host, r.URL.String(), remoteAddr, err)
			serve.Serve404(w, r)
			return
		}

		var status int
		if app != nil {
			start := time.Now()
			defer func() {
				app.LogAccessF(status, remoteAddr, r, start)
			}()
		}

		rateLimits := rp.config.ProxyLimit

		reqId := requestid.Get(r)
		reqUrl, _, reqHost, _ := DecomposeUrl(r)

		rp.tracking.Increment("__total__")
		defer rp.deferDecTracking("__total__")

		if tbe := tollbooth.LimitByKeys(rp.limiter, []string{reqHost, remoteAddr}); tbe != nil {
			var delayCount int
			itrDelay := time.Duration(rateLimits.MaxDelay.Nanoseconds() / int64(rateLimits.DelayScale))
			totalDelay := time.Duration(0)
			delayTrackingKeys := []string{"__delay__", "delay,host," + reqHost, "delay,addr," + remoteAddr}
			if portKey != "" {
				delayTrackingKeys = append(delayTrackingKeys, "delay,"+portKey)
			}
			rp.tracking.Increment(delayTrackingKeys...)
			defer rp.deferDecTracking(delayTrackingKeys...)
			for delayCount = 1; delayCount <= rateLimits.DelayScale; delayCount++ {
				time.Sleep(itrDelay)
				totalDelay = time.Duration(itrDelay.Nanoseconds() * int64(delayCount))
				if !rp.limiter.LimitReached(reqHost) && !rp.limiter.LimitReached(remoteAddr) {
					if delayCount > 1 && rateLimits.LogAllowed {
						rp.LogInfoF("[rate] allowed - %v - %v - %v - %v", reqId, remoteAddr, reqUrl, totalDelay)
					}
					break
				}
				if rateLimits.LogDelayed {
					rp.LogInfoF("[rate] delayed - %v - %v - %v - %v", reqId, remoteAddr, reqUrl, totalDelay)
				}
			}
			if delayCount > rateLimits.DelayScale {
				rp.limiter.ExecOnLimitReached(w, r)
				if rp.limiter.GetOverrideDefaultResponseWriter() {
					return
				}
				w.Header().Add("Content-Type", rp.limiter.GetMessageContentType())
				w.WriteHeader(tbe.StatusCode)
				_, _ = w.Write([]byte(tbe.Message))
				if rateLimits.LogLimited {
					rp.LogInfoF("[rate] limited - %v - %v - %v - %v", reqId, remoteAddr, reqUrl, totalDelay)
				}
				return
			}
		}

		// request is allowed
		trackingKeys := []string{"host," + reqHost, "addr," + remoteAddr}
		if portKey != "" {
			trackingKeys = append(trackingKeys, portKey)
		}
		rp.tracking.Increment(trackingKeys...)
		defer rp.deferDecTracking(trackingKeys...)

		if status, err = rp.ServeOriginHTTP(app, remoteAddr, w, r); err != nil {
			if strings.Contains(err.Error(), "context canceled") {
				status = http.StatusTeapot
				err = nil
				return
			}
			if strings.Contains(err.Error(), "connection refused") {
				status = http.StatusBadGateway
				serve.Serve502(w, r)
				return
			}
			rp.LogErrorF("origin request error: %v - %v\n", app.Name, err)
			status = http.StatusInternalServerError
			serve.Serve500(w, r)
		}
	}))
}

func (rp *ReverseProxy) deferDecTracking(keys ...string) {
	go func() {
		time.Sleep(DefaultProxyLimitsStatLifetime)
		rp.tracking.Decrement(keys...)
	}()
}