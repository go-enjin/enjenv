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
	"regexp"
	"strings"

	beIo "github.com/go-enjin/enjenv/pkg/io"
)

var (
	RxSockCommand         = regexp.MustCompile(`^\s*([a-z][-.a-z0-9]+?)\s*$`)
	RxSockCommandWithArgs = regexp.MustCompile(`^\s*([a-z][-.a-z0-9]+?)\s+(.+?)\s*$`)
)

func (s *Server) sockServe() (err error) {
	if err = s.sockListen(); err == net.ErrClosed {
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

func (s *Server) sockListen() (err error) {
	for {
		var conn net.Conn
		if conn, err = s.sock.Accept(); err != nil {
			break
		}
		go s.HandleSock(conn)
	}
	return
}

func (s *Server) HandleSock(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	var err error
	var bufRead int
	buf := make([]byte, 1024)
	if bufRead, err = conn.Read(buf); err != nil {
		beIo.StderrF("error reading enjin-proxy sock request: %v\n", err)
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
			beIo.StderrF("ERR: invalid input - \"%v\" - %v", m[0][2], err)
			_, _ = conn.Write([]byte("ERR: invalid input\n"))
			return
		}

	} else {
		beIo.StderrF("ERR: invalid input - \"%v\"\n", request)
		_, _ = conn.Write([]byte("ERR: invalid input\n"))
		return
	}

	var out string
	if out, err = s.sockProcessInput(cmd, argv); err != nil {
		_, _ = conn.Write([]byte(fmt.Sprintf("ERR: %v\n", err)))
	} else if out != "" {
		_, _ = conn.Write([]byte(out + "\n"))
	} else {
		_, _ = conn.Write([]byte("OK\n"))
	}
}

func (s *Server) sockProcessInput(cmd string, argv []string) (out string, err error) {
	cmd = strings.ToLower(cmd)
	switch cmd {

	case "nop":
		beIo.StdoutF("processed command: %v %v\n", cmd, argv)
		return

	default:
		err = fmt.Errorf("unknown command")
		beIo.StdoutF("unknown command: %v %v\n", cmd, argv)
	}
	return
}