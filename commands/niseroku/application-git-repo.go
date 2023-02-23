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

	bePath "github.com/go-enjin/be/pkg/path"
	"github.com/go-git/go-git/v5"

	"github.com/go-enjin/enjenv/pkg/service/common"
)

func (a *Application) SetupRepo() (err error) {
	a.Lock()
	defer a.Unlock()
	if a.GitRepo != nil {
		return
	}
	if !bePath.IsDir(a.RepoPath) {
		if err = bePath.Mkdir(a.RepoPath); err != nil {
			err = fmt.Errorf("error making application repo path: %v - %v", a.RepoPath, err)
			return
		}
		repoHooksPath := a.RepoPath + "/hooks"
		if err = bePath.Mkdir(repoHooksPath); err != nil {
			err = fmt.Errorf("error making application repo hooks path: %v - %v", repoHooksPath, err)
			return
		}
	}
	if a.GitRepo, err = git.PlainInit(a.RepoPath, true); err != nil && err == git.ErrRepositoryAlreadyExists {
		a.GitRepo, err = git.PlainOpen(a.RepoPath)
	}
	if ee := common.RepairOwnership(a.RepoPath, a.Config.RunAs.User, a.Config.RunAs.Group); ee != nil {
		a.LogErrorF("error repairing git repo ownership: %v - %v", a.RepoPath, ee)
	}
	return
}