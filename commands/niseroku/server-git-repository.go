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
	"os"
	"strings"
	"sync"

	"github.com/sosedoff/gitkit"
	"github.com/urfave/cli/v2"

	bePath "github.com/go-enjin/be/pkg/path"
	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/service"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

type GitRepository struct {
	service.Service

	config *Config

	repo *gitkit.SSH
}

func (c *Command) actionGitRepository(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	gr := NewGitRepository(c.config)
	if gr.IsRunning() {
		err = fmt.Errorf("git-repository already running")
		return
	}
	err = gr.Start()
	return
}

func NewGitRepository(config *Config) (gr *GitRepository) {
	gr = new(GitRepository)
	gr.Name = "git-repository"
	gr.User = config.RunAs.User
	gr.Group = config.RunAs.Group
	gr.PidFile = config.Paths.RepoPidFile
	gr.LogFile = config.LogFile
	gr.config = config
	gr.ServeFn = gr.Serve
	gr.BindFn = gr.Bind
	gr.StopFn = gr.Stop
	gr.ReloadFn = gr.Reload
	gr.RestartFn = gr.Restart
	return
}

func (gr *GitRepository) Bind() (err error) {

	if err = gr.config.PrepareDirectories(); err != nil {
		err = fmt.Errorf("error preparing directories: %v", err)
		return
	}

	gr.Lock()
	defer gr.Unlock()

	addr := fmt.Sprintf("%v:%d", gr.config.BindAddr, gr.config.Ports.Git)

	gr.repo = gitkit.NewSSH(gitkit.Config{
		Dir:        gr.config.Paths.VarRepos,
		KeyDir:     gr.config.Paths.RepoSecrets,
		AutoCreate: false,
		Auth:       true,
		AutoHooks:  false,
		// Hooks: &gitkit.HookScripts{
		// 	PreReceive:  preReceiveHookSource,
		// 	PostReceive: postReceiveHookSource,
		// },
	})

	gr.repo.PublicKeyLookupFunc = gr.publicKeyLookupFunc

	err = gr.repo.Listen(addr)
	return
}

func (gr *GitRepository) Serve() (err error) {

	go gr.HandleSIGHUP()
	go gr.HandleSIGUSR1()

	// SIGINT+TERM handler
	idleConnectionsClosed := make(chan struct{})
	go func() {
		go gr.HandleSIGINT()
		close(idleConnectionsClosed)
	}()

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		gr.LogInfoF("starting repo service: %d\n", gr.config.Ports.Git)
		if err = gr.repo.Serve(); err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				err = nil
			} else {
				gr.LogErrorF("error running repo service: %v\n", err)
			}
		}
		wg.Done()
	}()

	gr.LogInfoF("all services running")
	if wg.Wait(); err == nil {
		gr.LogInfoF("awaiting idle connections")
		<-idleConnectionsClosed
		gr.LogInfoF("all idle connections closed")
	}
	return
}

func (gr *GitRepository) Stop() (err error) {
	gr.Lock()
	defer gr.Unlock()
	if gr.repo != nil {
		gr.LogInfoF("shutting down repo service")
		if ee := gr.repo.Stop(); ee != nil {
			gr.LogErrorF("error shutting down repo service: %v", ee)
		}
	}
	return
}

func (gr *GitRepository) Reload() (err error) {
	gr.Lock()
	defer gr.Unlock()
	gr.LogInfoF("git-repository reloading\n")
	err = gr.config.Reload()
	return
}

func (gr *GitRepository) Restart() (err error) {
	err = fmt.Errorf("git-repository restart not implemented")
	return
}

func (gr *GitRepository) publicKeyLookupFunc(inputPubKey string) (pubkey *gitkit.PublicKey, err error) {
	var ok bool
	var inputKeyId string
	if _, _, _, inputKeyId, ok = common.ParseSshKey(inputPubKey); !ok {
		err = fmt.Errorf("unable to parse SSH key: %v", inputPubKey)
		return
	}
	gr.RLock()
	defer gr.RUnlock()
	for _, u := range gr.config.Users {
		if u.HasKey(inputKeyId) {
			gr.LogInfoF("validated user by ssh-key: %v\n", u.Name)
			pubkey = &gitkit.PublicKey{
				Id: inputKeyId,
			}
			return
		}
	}
	err = fmt.Errorf("ssh-key not found")
	gr.LogErrorF("user not found by ssh-key: %v\n", inputPubKey)
	return
}

const (
	gPreReceiveHookTemplate  = "#!/bin/bash\ncat - | %v niseroku --config=%v app git-pre-receive-hook"
	gPostReceiveHookTemplate = "#!/bin/bash\ncat - | %v niseroku --config=%v app git-post-receive-hook"
)

func (gr *GitRepository) updateGitHookScripts() (err error) {

	preReceiveHookSource := fmt.Sprintf(gPreReceiveHookTemplate, basepath.WhichBin(), gr.config.Source)
	postReceiveHookSource := fmt.Sprintf(gPostReceiveHookTemplate, basepath.WhichBin(), gr.config.Source)

	for _, app := range gr.config.Applications {
		if app.RepoPath == "" {
			gr.LogInfoF("no hook updates possible, app repo path missing: %v\n", app.Name)
			continue
		}
		hookDir := app.RepoPath + "/hooks"
		if bePath.IsDir(hookDir) {
			if preReceiveHookPath := hookDir + "/pre-receive"; !bePath.IsFile(preReceiveHookPath) {
				if err = os.WriteFile(preReceiveHookPath, []byte(preReceiveHookSource), 0660); err != nil {
					gr.LogErrorF("error writing git pre-receive hook: %v - %v\n", preReceiveHookPath, err)
				} else if err = os.Chmod(preReceiveHookPath, 0770); err != nil {
					gr.LogErrorF("error changing mode of git pre-receive hook: %v - %v\n", preReceiveHookPath, err)
				}
			}
			if postReceiveHookPath := hookDir + "/post-receive"; !bePath.IsFile(postReceiveHookPath) {
				if err = os.WriteFile(postReceiveHookPath, []byte(postReceiveHookSource), 0660); err != nil {
					gr.LogErrorF("error writing git post-receive hook: %v - %v\n", postReceiveHookPath, err)
				} else if err = os.Chmod(postReceiveHookPath, 0770); err != nil {
					gr.LogErrorF("error changing mode of git post-receive hook: %v - %v\n", postReceiveHookPath, err)
				}
			}
		}
	}

	return
}