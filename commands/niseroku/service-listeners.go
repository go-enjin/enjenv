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
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/sosedoff/gitkit"
	"golang.org/x/crypto/acme/autocert"

	"github.com/go-enjin/be/pkg/maps"
	bePath "github.com/go-enjin/be/pkg/path"
	"github.com/go-enjin/enjenv/pkg/basepath"
	beIo "github.com/go-enjin/enjenv/pkg/io"
)

func (s *Server) bindControlListener() (err error) {
	s.Lock()
	defer s.Unlock()
	if bePath.Exists(s.Config.Paths.Control) {
		if err = os.Remove(s.Config.Paths.Control); err != nil {
			return
		}
	}
	if s.sock, err = net.Listen("unix", s.Config.Paths.Control); err != nil {
		return
	}
	return
}

func (s *Server) bindBothHttpListeners() (err error) {
	s.Lock()
	defer s.Unlock()

	httpAddr := fmt.Sprintf("%v:%d", s.Config.BindAddr, s.Config.Ports.Http)
	httpsAddr := fmt.Sprintf("%v:%d", s.Config.BindAddr, s.Config.Ports.Https)

	lookupDomains := maps.Keys(s.LookupDomain)

	s.autocert = &autocert.Manager{
		Cache:      autocert.DirCache(s.Config.Paths.ProxySecrets),
		Prompt:     autocert.AcceptTOS,
		Email:      s.Config.AccountEmail,
		HostPolicy: autocert.HostWhitelist(lookupDomains...),
	}

	s.http = &http.Server{
		Addr:    httpAddr,
		Handler: http.HandlerFunc(s.Handler),
	}
	s.http.Handler = s.autocert.HTTPHandler(http.HandlerFunc(s.Handler))

	s.https = &http.Server{
		Addr:      httpsAddr,
		Handler:   s.autocert.HTTPHandler(http.HandlerFunc(s.Handler)),
		TLSConfig: s.autocert.TLSConfig(),
	}

	if s.httpListener, err = net.Listen("tcp", httpAddr); err != nil {
		return
	}
	s.httpsListener, err = net.Listen("tcp", httpsAddr)

	return
}

func (s *Server) bindOnlyHttpListener() (err error) {
	s.Lock()
	defer s.Unlock()

	addr := fmt.Sprintf("%v:%d", s.Config.BindAddr, s.Config.Ports.Http)
	s.http = &http.Server{
		Addr:    addr,
		Handler: http.HandlerFunc(s.Handler),
	}
	s.httpListener, err = net.Listen("tcp", addr)
	s.httpsListener = nil
	return
}

func (s *Server) bindGitListener() (err error) {
	s.Lock()
	defer s.Unlock()

	addr := fmt.Sprintf("%v:%d", s.Config.BindAddr, s.Config.Ports.Git)

	s.repo = gitkit.NewSSH(gitkit.Config{
		Dir:        s.Config.Paths.VarRepos,
		KeyDir:     s.Config.Paths.RepoSecrets,
		AutoCreate: false,
		Auth:       true,
		AutoHooks:  false,
		// Hooks: &gitkit.HookScripts{
		// 	PreReceive:  preReceiveHookSource,
		// 	PostReceive: postReceiveHookSource,
		// },
	})

	s.repo.PublicKeyLookupFunc = s.publicKeyLookupFunc

	err = s.repo.Listen(addr)
	return
}

func (s *Server) publicKeyLookupFunc(inputPubKey string) (pubkey *gitkit.PublicKey, err error) {
	var ok bool
	var inputKeyId string
	if _, _, _, inputKeyId, ok = parseSshKey(inputPubKey); !ok {
		err = fmt.Errorf("unable to parse SSH key: %v", inputPubKey)
		return
	}
	beIo.StdoutF("validating public key: %v\n", inputPubKey)
	for _, app := range s.Applications() {
		if app.SshKeyPresent(inputKeyId) {
			beIo.StdoutF("validated app pubkey: %v (%v)\n", app.Name, inputPubKey)
			pubkey = &gitkit.PublicKey{
				Id: inputKeyId,
			}
			return
		}
	}
	err = fmt.Errorf("pubkey not found")
	beIo.StderrF("app with pubkey not found: %v\n", inputPubKey)
	return
}

func (s *Server) updateGitHookScripts() (err error) {

	preReceiveHookSource := fmt.Sprintf(`#!/bin/bash
cat - | %v niseroku --config=%v app git-pre-receive-hook`,
		basepath.WhichBin(),
		s.Config.Source,
	)

	postReceiveHookSource := fmt.Sprintf(`#!/bin/bash
cat - | %v niseroku --config=%v app git-post-receive-hook`,
		basepath.WhichBin(),
		s.Config.Source,
	)

	for _, app := range s.Applications() {
		if app.RepoPath == "" {
			beIo.StdoutF("no hook updates possible, app repo path missing: %v\n", app.Name)
			continue
		}
		hookDir := app.RepoPath + "/hooks"
		if bePath.IsDir(hookDir) {
			if preReceiveHookPath := hookDir + "/pre-receive"; !bePath.IsFile(preReceiveHookPath) {
				if err = os.WriteFile(preReceiveHookPath, []byte(preReceiveHookSource), 0660); err != nil {
					beIo.StderrF("error writing git pre-receive hook: %v - %v\n", preReceiveHookPath, err)
				} else if err = os.Chmod(preReceiveHookPath, 0770); err != nil {
					beIo.StderrF("error changing mode of git pre-receive hook: %v - %v\n", preReceiveHookPath, err)
				}
			}
			if postReceiveHookPath := hookDir + "/post-receive"; !bePath.IsFile(postReceiveHookPath) {
				if err = os.WriteFile(postReceiveHookPath, []byte(postReceiveHookSource), 0660); err != nil {
					beIo.StderrF("error writing git post-receive hook: %v - %v\n", postReceiveHookPath, err)
				} else if err = os.Chmod(postReceiveHookPath, 0770); err != nil {
					beIo.StderrF("error changing mode of git post-receive hook: %v - %v\n", postReceiveHookPath, err)
				}
			}
		}
	}

	return
}