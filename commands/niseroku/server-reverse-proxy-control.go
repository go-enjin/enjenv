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
	"encoding/csv"
	"fmt"
	"net"
	"strings"
)

func (rp *ReverseProxy) controlSocketListen() (err error) {
	for {
		var conn net.Conn
		if conn, err = rp.control.Accept(); err != nil {
			break
		}
		go rp.HandleSock(conn)
	}
	return
}

func (rp *ReverseProxy) controlSocketServe() (err error) {
	if err = rp.controlSocketListen(); err == net.ErrClosed {
		err = nil
	} else if err != nil {
		if strings.Contains(err.Error(), "use of closed network connection") {
			err = nil
		} else {
			err = fmt.Errorf("error serving control file: %v", err)
		}
	}
	return
}

func (rp *ReverseProxy) HandleSock(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	var err error
	var bufRead int
	buf := make([]byte, 1024)
	if bufRead, err = conn.Read(buf); err != nil {
		rp.LogErrorF("error reading control socket: %v\n", err)
		return
	}

	var cmd string
	var argv []string

	request := strings.TrimSpace(string(buf[:bufRead]))
	if RxSockCommand.MatchString(request) {

		m := RxSockCommand.FindAllStringSubmatch(request, 1)
		cmd = m[0][1]

	} else if RxSockCommandWithArgs.MatchString(request) {

		m := RxSockCommandWithArgs.FindAllStringSubmatch(request, 1)
		cmd = m[0][1]

		r := csv.NewReader(strings.NewReader(m[0][2]))
		r.Comma = ' '
		if argv, err = r.Read(); err != nil {
			rp.LogErrorF("invalid input - \"%v\" - %v", m[0][2], err)
			_, _ = conn.Write([]byte("ERR: invalid input\n"))
			return
		}

	} else {
		rp.LogErrorF("invalid input - \"%v\"\n", request)
		_, _ = conn.Write([]byte("ERR: invalid input\n"))
		return
	}

	var out string
	if out, err = rp.controlSocketProcessCommand(cmd, argv); err != nil {
		_, _ = conn.Write([]byte(fmt.Sprintf("ERR: %v\n", err)))
	} else if out != "" {
		_, _ = conn.Write([]byte(out + "\n"))
	} else {
		_, _ = conn.Write([]byte("OK\n"))
	}
}

func (rp *ReverseProxy) controlSocketProcessCommand(cmd string, argv []string) (out string, err error) {
	cmd = strings.ToLower(cmd)
	switch cmd {

	case "proxy-limits":
		out = rp.tracking.String()
		// rp.LogInfoF("[control] processed command: %v %v\n", cmd, argv)
		return

	case "nop":
		out = fmt.Sprintf("[control] processed command: %v %v", cmd, argv)
		rp.LogInfoF("%v\n", out)
		return

	default:
		err = fmt.Errorf("unknown command")
		rp.LogInfoF("[control] unknown command: %v %v\n", cmd, argv)
	}
	return
}
