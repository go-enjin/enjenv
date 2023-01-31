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

	"github.com/dustin/go-humanize"
	"github.com/go-git/go-git/v5"
	"github.com/otiai10/copy"
	"github.com/sosedoff/gitkit"
	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/cli/env"
	"github.com/go-enjin/be/pkg/cli/run"
	bePath "github.com/go-enjin/be/pkg/path"
	beStrings "github.com/go-enjin/be/pkg/strings"

	pkgIo "github.com/go-enjin/enjenv/pkg/io"
	pkgRun "github.com/go-enjin/enjenv/pkg/run"
)

func (c *Command) actionGitPreReceiveHook(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}

	pkgIo.STDOUT("# initializing slug building process\n")

	receiver := gitkit.Receiver{
		MasterOnly: false,              // if set to true, only pushes to master branch will be allowed
		TmpDir:     c.config.Paths.Tmp, // directory for temporary git checkouts
		HandlerFunc: func(info *gitkit.HookInfo, tmpPath string) (err error) {
			var app *Application
			if app, err = c.enjinRepoGitHandlerSetup(c.config, info); err != nil {
				return
			}
			err = c.enjinRepoPreReceiveHandler(app, c.config, info, tmpPath) // your handler function
			return
		},
	}

	// Git hook data is provided via STDIN
	if err = receiver.Handle(os.Stdin); err != nil {
		return
	}

	return
}

func (c *Command) actionGitPostReceiveHook(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}

	receiver := gitkit.Receiver{
		MasterOnly: false,              // if set to true, only pushes to master branch will be allowed
		TmpDir:     c.config.Paths.Tmp, // directory for temporary git checkouts
		HandlerFunc: func(info *gitkit.HookInfo, tmpPath string) (err error) {
			var app *Application
			if app, err = c.enjinRepoGitHandlerSetup(c.config, info); err != nil {
				return
			}
			err = c.enjinRepoPostReceiveHandler(app, c.config, info, tmpPath) // your handler function
			return
		},
	}

	// Git hook data is provided via STDIN
	if err = receiver.Handle(os.Stdin); err != nil {
		return
	}

	return
}

func (c *Command) enjinRepoGitHandlerSetup(config *Config, info *gitkit.HookInfo) (app *Application, err error) {
	var envSshId string
	if envSshId = env.Get("GITKIT_KEY", ""); envSshId == "" {
		err = fmt.Errorf("missing git ssh-key")
		return
	}

	if app != nil && info.RefName != "main" {
		err = fmt.Errorf("invalid branch received")
		return
	}

	repoName := bePath.Base(info.RepoName)
	var ok bool
	if app, ok = c.config.Applications[repoName]; !ok {
		err = fmt.Errorf("repository not found: %v", repoName)
		return
	}

	for _, u := range c.config.Users {
		if u.HasKey(envSshId) {
			if beStrings.StringInSlices(repoName, u.Applications) || beStrings.StringInSlices("*", u.Applications) {
				pkgIo.StdoutF("validated user and repository: %v - %v\n", u.Name, repoName)
				return
			}
			app = nil
			err = fmt.Errorf("user")
			return
		}
	}

	app = nil
	err = fmt.Errorf("error finding user by ssh-key")
	return
}

func (c *Command) enjinRepoPreReceiveHandler(app *Application, config *Config, info *gitkit.HookInfo, tmpPath string) (err error) {
	// nop, all safety checks done in git handler setup
	return
}

func (c *Command) enjinRepoPostReceiveHandler(app *Application, config *Config, info *gitkit.HookInfo, tmpPath string) (err error) {

	tmpName := bePath.Base(tmpPath)
	buildDir := config.Paths.TmpBuild + "/" + tmpName
	cacheDir := config.Paths.VarCache + "/" + app.Name
	slugZip := config.Paths.VarSlugs + "/" + app.Name + "--" + info.NewRev + ".zip"
	buildPackClonePath := config.Paths.TmpClone + "/" + app.Name
	envDir := config.Paths.VarSettings + "/" + app.Name

	pkgIo.STDOUT("# preparing ENV_DIR...\n")
	if bePath.IsDir(envDir) {
		if err = os.RemoveAll(envDir); err != nil {
			err = fmt.Errorf("error removing enjin env path: %v - %v", envDir, err)
			return
		}
	}
	if err = bePath.Mkdir(envDir); err != nil {
		err = fmt.Errorf("error making enjin deployment path: %v - %v", envDir, err)
		return
	}
	if err = app.ApplySettings(envDir); err != nil {
		err = fmt.Errorf("error applying enjin settings: %v - %v", envDir, err)
		return
	}

	pkgIo.STDOUT("# preparing CACHE_DIR...\n")
	if !bePath.IsDir(cacheDir) {
		if err = bePath.Mkdir(cacheDir); err != nil {
			err = fmt.Errorf("error making enjin deployment path: %v - %v", cacheDir, err)
			return
		}
	}

	pkgIo.STDOUT("# preparing BUILD_DIR...\n")
	if err = copy.Copy(tmpPath, buildDir); err != nil {
		err = fmt.Errorf("error copying to enjin build path: %v - %v", buildDir, err)
		return
	}
	defer func() {
		// cleanup build dir, if success, zip is all that is needed
		pkgIo.STDOUT("# cleaning BUILD_DIR...\n")
		_ = os.RemoveAll(buildDir)
	}()

	pkgIo.STDOUT("# preparing enjenv buildpack...\n")
	var buildPack string
	if config.BuildPack != "" {
		buildPack = config.BuildPack
	} else {
		buildPack = DefaultBuildPack
	}
	if bePath.IsDir(config.BuildPack) {
		if err = copy.Copy(config.BuildPack, buildPackClonePath); err != nil {
			return
		}
	} else if _, err = git.PlainClone(buildPackClonePath, false, &git.CloneOptions{URL: buildPack}); err != nil {
		return
	}
	defer func() {
		pkgIo.STDOUT("# cleaning enjenv buildpack...\n")
		_ = os.RemoveAll(envDir)
		_ = os.RemoveAll(buildPackClonePath)
	}()

	var status int
	pkgIo.STDOUT("# buildpack: detected... ")
	if status, err = run.Exe(buildPackClonePath+"/bin/detect", buildDir); err != nil {
		err = fmt.Errorf("error detecting buildpack: %v", err)
		pkgIo.STDOUT("\n")
		return
	} else if status != 0 {
		pkgIo.STDOUT("\n")
		return
	}

	pkgIo.STDOUT("# buildpack: compiling...\n")
	if status, err = run.Exe(buildPackClonePath+"/bin/compile", buildDir, cacheDir, envDir); err != nil {
		return
	} else if status != 0 {
		return
	}

	pkgIo.STDOUT("# compressing built slug\n")
	pwd := bePath.Pwd()
	if err = os.Chdir(buildDir); err != nil {
		return
	}
	if status, err = run.Exe("zip", "--quiet", "--recurse-paths", slugZip, "."); err != nil {
		return
	} else if status != 0 {
		return
	}
	slugSize := "(nil)"
	if stat, ee := os.Stat(slugZip); ee != nil {
		pkgIo.STDERR("error getting slug file size: %v\n", ee)
	} else {
		slugSize = humanize.Bytes(uint64(stat.Size()))
	}
	pkgIo.STDOUT("# slug compressed size: %v\n", slugSize)

	if err = os.Chdir(pwd); err != nil {
		return
	}

	err = pkgRun.EnjenvExe("niseroku", "deploy-slug", slugZip)

	return
}