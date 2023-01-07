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
)

type AppOrigin struct {
	Scheme string `toml:"scheme,omitempty"`
	Host   string `toml:"host,omitempty"`
	Port   int    `toml:"port,omitempty,omitempty"`
}

func (o AppOrigin) String() (s string) {
	s = fmt.Sprintf("%s://%s:%d", o.Scheme, o.Host, o.Port)
	return
}

func (o AppOrigin) NetIP() (ip net.IP) {
	ip = net.ParseIP(o.Host)
	return
}

func (o AppOrigin) DialAddr() (addr string) {
	addr = fmt.Sprintf("%s:%d", o.Host, o.Port)
	return
}

func (o AppOrigin) Dialer() (dialer *net.Dialer) {
	dialer = &net.Dialer{
		LocalAddr: &net.TCPAddr{
			IP:   o.NetIP(),
			Port: 0,
		},
	}
	return
}

func (o AppOrigin) Dial() (conn net.Conn, err error) {
	conn, err = o.Dialer().Dial("tcp", o.DialAddr())
	return
}