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
)

var (
	RxSockCommand         = regexp.MustCompile(`^\s*([a-z][-.a-z0-9]+?)\s*$`)
	RxSockCommandWithArgs = regexp.MustCompile(`^\s*([a-z][-.a-z0-9]+?)\s+(.+?)\s*$`)
)

func (s *Server) controlSocketListen() (err error) {
	for {
		var conn net.Conn
		if conn, err = s.sock.Accept(); err != nil {
			break
		}
		go s.HandleSock(conn)
	}
	return
}

func (s *Server) controlSocketServe() (err error) {
	if err = s.controlSocketListen(); err == net.ErrClosed {
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

func (s *Server) HandleSock(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	var err error
	var bufRead int
	buf := make([]byte, 1024)
	if bufRead, err = conn.Read(buf); err != nil {
		s.LogErrorF("error reading control socket: %v\n", err)
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
			s.LogErrorF("invalid input - \"%v\" - %v", m[0][2], err)
			_, _ = conn.Write([]byte("ERR: invalid input\n"))
			return
		}

	} else {
		s.LogErrorF("invalid input - \"%v\"\n", request)
		_, _ = conn.Write([]byte("ERR: invalid input\n"))
		return
	}

	var out string
	if out, err = s.controlSocketProcessCommand(cmd, argv); err != nil {
		_, _ = conn.Write([]byte(fmt.Sprintf("ERR: %v\n", err)))
	} else if out != "" {
		_, _ = conn.Write([]byte(out + "\n"))
	} else {
		_, _ = conn.Write([]byte("OK\n"))
	}
}

func (s *Server) controlSocketProcessCommand(cmd string, argv []string) (out string, err error) {
	cmd = strings.ToLower(cmd)
	switch cmd {

	case "shutdown":
		s.LogInfoF("[control] shutting down\n")
		s.Stop()
		return

	case "app-start":
		for _, arg := range argv {
			s.RLock()
			app, ok := s.LookupApp[arg]
			s.RUnlock()
			if !ok {
				s.LogInfoF("[control] app not found: %v\n", arg)
			} else if ee := s.StartAppSlug(app); ee != nil {
				s.LogErrorF("error starting app slug: %v - %v\n", app.Name, ee)
			} else {
				s.LogInfoF("[control] started app slug: %v\n", app.Name)
			}
		}
		return

	case "app-stop-all":
		for _, app := range s.Applications() {
			if slug := app.GetThisSlug(); slug != nil {
				if slug.IsReady() {
					slug.Stop()
					s.LogInfoF("[control] stopped app slug: %v\n", app.Name)
				} else {
					s.LogInfoF("[control] app slug already stopped: %v\n", app.Name)
				}
			} else {
				s.LogInfoF("[control] app slug not found: %v\n", app.Name)
			}
		}
		return

	case "app-stop":
		for _, arg := range argv {
			if app, ok := s.LookupApp[arg]; ok {
				if slug := app.GetThisSlug(); slug != nil {
					if slug.IsReady() {
						slug.Stop()
						s.LogInfoF("[control] stopped app slug: %v\n", arg)
					} else {
						s.LogInfoF("[control] app slug already stopped: %v\n", arg)
					}
				} else {
					s.LogInfoF("[control] app slug not found: %v\n", arg)
				}
			} else {
				s.LogInfoF("[control] app not found: %v\n", arg)
			}
		}
		return

	case "nop":
		s.LogInfoF("[control] processed command: %v %v\n", cmd, argv)
		return

	default:
		err = fmt.Errorf("unknown command")
		s.LogInfoF("[control] unknown command: %v %v\n", cmd, argv)
	}
	return
}