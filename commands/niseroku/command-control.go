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
	"net"
	"strings"
)

func (c *Command) CallControlCommand(name string, argv ...string) (response string, err error) {
	var conn net.Conn
	if conn, err = net.Dial("unix", c.config.Paths.Control); err != nil {
		return
	}
	defer func() {
		_ = conn.Close()
	}()
	command := name + " " + strings.Join(argv, " ")
	if _, err = conn.Write([]byte(command)); err != nil {
		return
	}
	var data []byte
	buf := make([]byte, 4096)
	if n, ee := conn.Read(buf); ee == nil {
		data = append(data, buf[:n]...)
	}
	response = string(data)
	return
}