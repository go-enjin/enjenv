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
	"math"
	"net/http"
	"time"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth/v7/limiter"
	"github.com/go-enjin/be/pkg/net"
	"github.com/kataras/requestid"
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
		rateLimits := rp.config.ProxyLimit
		remoteAddr := "err"
		if addr, err := net.GetIpFromRequest(r); err == nil {
			remoteAddr = addr
		}
		reqId := requestid.Get(r)
		reqUrl, _, reqHost, _ := DecomposeUrl(r)

		go rp.tracking.Increment("__total__", "host,"+reqHost, "addr,"+remoteAddr)
		defer func() {
			time.Sleep(10 * time.Millisecond)
			rp.tracking.Decrement("__total__", "host,"+reqHost, "addr,"+remoteAddr)
		}()

		if tbe := tollbooth.LimitByKeys(rp.limiter, []string{reqHost, remoteAddr}); tbe != nil {
			var delayCount int
			itrDelay := time.Duration(rateLimits.MaxDelay.Nanoseconds() / int64(rateLimits.DelayScale))
			totalDelay := time.Duration(0)
			go rp.tracking.Increment("__delay__", "delay,host,"+reqHost, "delay,addr,"+remoteAddr)
			defer func() {
				time.Sleep(10 * time.Millisecond)
				rp.tracking.Decrement("__delay__", "delay,host,"+reqHost, "delay,addr,"+remoteAddr)
			}()
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
		rp.ServeProxyHTTP(w, r)
	}))
}