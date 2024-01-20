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
	"net"
	"net/http"
	"strconv"
)

func CheckAB[V interface{}](a, b V, check bool) (v V) {
	if check {
		v = a
	} else {
		v = b
	}
	return
}

func DecomposeUrl(r *http.Request) (full, scheme, host string, port int) {
	port = 80
	if scheme = "http"; r.TLS != nil {
		scheme = "https"
		port = 443
	}
	if h, p, e := net.SplitHostPort(r.Host); e == nil {
		host = h
		if v, ee := strconv.Atoi(p); ee == nil {
			port = v
		}
	} else {
		host = r.Host
	}
	portStr := ""
	if port != 80 && port != 443 {
		portStr = ":" + strconv.Itoa(port)
	}
	full = fmt.Sprintf("%v://%v%v%v", scheme, host, portStr, r.URL)
	return
}
