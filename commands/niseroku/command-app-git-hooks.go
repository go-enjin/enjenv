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

	"github.com/sosedoff/gitkit"

	"github.com/go-corelibs/env"
	"github.com/go-corelibs/path"
	"github.com/go-corelibs/slices"
	"github.com/go-enjin/be/pkg/context"
	pkgIo "github.com/go-enjin/enjenv/pkg/io"
)

func (c *Command) enjinRepoGitHandlerSetup(config *Config, info *gitkit.HookInfo) (app *Application, err error) {

	tracking := context.New()
	defer func() {
		if err != nil {
			if pkgIo.LogFile != "" {
				pkgIo.STDERR("error: %v\n", err)
			}
			if len(tracking) > 0 {
				pkgIo.StderrF("error: %v - %#+v\n", err, tracking)
			} else {
				pkgIo.StderrF("error: %v\n", err)
			}
		}
	}()

	var envSshId string
	if envSshId = env.String("GITKIT_KEY", ""); envSshId == "" {
		err = fmt.Errorf("user credentials not found")
		return
	}

	if info == nil {
		err = fmt.Errorf("git info not present")
		return
	}

	tracking.Set("branch", info.RefName)

	repoName := path.Base(info.RepoName)
	var ok bool
	if app, ok = c.config.Applications[repoName]; !ok {
		err = fmt.Errorf("repository not found")
		tracking.Set("repoName", repoName)
		return
	} else if app.IsDeploying() {
		err = fmt.Errorf("application deployment in progress")
		return
	}

	for _, u := range c.config.Users {
		if u.HasKey(envSshId) {
			tracking.Set("userName", u.Name)
			tracking.Set("repoName", repoName)
			if slices.Within(repoName, u.Applications) || slices.Within("*", u.Applications) {
				break
			}
			app = nil
			err = fmt.Errorf("repository access denied")
			return
		}
	}
	if !tracking.Has("userName") {
		app = nil
		err = fmt.Errorf("user not found")
		tracking.Set("sshKeyId", envSshId)
		return
	}

	var forced bool
	if f, ee := gitkit.IsForcePush(info); ee == nil {
		forced = f
	}
	tracking.Set("forced", forced)

	tracking.Set("action", info.Action)
	if !slices.Present(info.Action, gitkit.BranchCreateAction, gitkit.BranchPushAction, gitkit.TagCreateAction) {
		err = fmt.Errorf("unsupported git action")
		return
	}

	var dists []Distribution
	var ae *AptEnjinConfig
	var ap *AptPackageConfig
	var aptApp *Application

	if ap = app.AptPackage; ap != nil {
		if ap.AptEnjin == "" {
			err = fmt.Errorf("apt-package.apt-enjin is empty")
			return
		} else if ap.Flavour == "" {
			err = fmt.Errorf("apt-package.flavour is empty")
			return
		} else if ap.Codename == "" {
			err = fmt.Errorf("apt-package.codename is empty")
			return
		} else if ap.Component == "" {
			err = fmt.Errorf("apt-package.component is empty")
			return
		} else if aptApp, ok = config.Applications[ap.AptEnjin]; !ok {
			err = fmt.Errorf("apt-package.apt-enjin not found: %v", ap.AptEnjin)
			return
		} else if ae = aptApp.AptEnjin; ae == nil {
			err = fmt.Errorf("apt-package.apt-enjin is not configured to be an apt-enjin: %v", ap.AptEnjin)
			return
		} else if !ae.Enable {
			err = fmt.Errorf("apt-enjin is not enabled")
			return
		} else if dists, ok = ae.Flavours[info.RefName]; !ok {
			err = fmt.Errorf("apt-enjin flavour not found: %v - %v", aptApp.Name, info.RefName)
			return
		}

		var codenameValid bool
		for _, dist := range dists {
			if codenameValid = dist.Codename == ap.Codename; codenameValid {
				if !slices.Within(ap.Component, dist.Components) {
					err = fmt.Errorf("apt-package.component is not provided by configured apt-enjin")
					return
				}
			}
		}
		if !codenameValid {
			err = fmt.Errorf("apt-package.codename is not provided by configured apt-enjin")
			return
		}
	}

	return
}
