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
	"net/http"
	"time"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth/v7/limiter"
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
}

func (rp *ReverseProxy) ProxyHttpHandler() (h http.Handler) {
	rp.initRateLimiter()
	lastLimited := time.Now()
	numLimited := 0
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tbe := tollbooth.LimitByRequest(rp.limiter, w, r); tbe != nil {
			rp.limiter.ExecOnLimitReached(w, r)
			if rp.limiter.GetOverrideDefaultResponseWriter() {
				return
			}
			w.Header().Add("Content-Type", rp.limiter.GetMessageContentType())
			w.WriteHeader(tbe.StatusCode)
			_, _ = w.Write([]byte(tbe.Message))
			numLimited += 1
			if now := time.Now(); now.Sub(lastLimited) > time.Second || numLimited >= 100 {
				rp.LogInfoF("rate limited %d request(s): %v - %v", numLimited, r.Host, r.URL)
				lastLimited = now
				numLimited = 0
			}
			return
		}
		// request is not limited
		rp.ServeProxyHTTP(w, r)
	})
}