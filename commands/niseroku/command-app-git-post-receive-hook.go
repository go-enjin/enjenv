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
	"path/filepath"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/go-git/go-git/v5"
	cp "github.com/otiai10/copy"
	"github.com/sosedoff/gitkit"
	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/cli/run"
	bePath "github.com/go-enjin/be/pkg/path"
	beStrings "github.com/go-enjin/be/pkg/strings"

	pkgIo "github.com/go-enjin/enjenv/pkg/io"
	pkgRun "github.com/go-enjin/enjenv/pkg/run"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

func (c *Command) actionAppGitPostReceiveHook(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}

	pkgIo.STDOUT("# running slug building process\n")

	receiver := gitkit.Receiver{
		MasterOnly: false,
		TmpDir:     c.config.Paths.Tmp,
		HandlerFunc: func(info *gitkit.HookInfo, tmpPath string) (err error) {
			var app *Application
			if app, err = c.enjinRepoGitHandlerSetup(c.config, info); err != nil {
				return
			}
			err = c.enjinRepoPostReceiveHandler(app, c.config, info, tmpPath) // your handler function
			return
		},
	}

	err = receiver.Handle(os.Stdin)
	return
}

func (c *Command) enjinRepoPostReceiveHandler(app *Application, config *Config, info *gitkit.HookInfo, tmpPath string) (err error) {

	if info.RefName == "main" {
		err = c.enjinRepoRunBuildpackProcess(app, config, info.NewRev, tmpPath)
		return
	}

	var ok bool
	var aptApp *Application
	var ap *AptPackageConfig
	var ae *AptEnjinConfig
	var dists []Distribution

	if ap = app.AptPackage; ap == nil {
		err = fmt.Errorf("unsupported branch received: %v", info.RefName)
		return
	} else if aptApp, ok = c.config.Applications[ap.AptEnjin]; !ok {
		err = fmt.Errorf("apt-package.apt-enjin not found: %v", ap.AptEnjin)
		return
	} else if ae = aptApp.AptEnjin; ae == nil {
		err = fmt.Errorf("apt-package.apt-enjin not not an apt-enjin: %v", aptApp.Name)
		return
	} else if !ae.Enable {
		err = fmt.Errorf("apt-enjin not enabled: %v", aptApp.Name)
		return
	} else if dists, ok = ae.Flavours[info.RefName]; !ok {
		err = fmt.Errorf("apt-enjin flavour not supported: %v", info.RefName)
		return
	}

	var distribution *Distribution
	for _, dist := range dists {
		if dist.Codename == ap.Codename {
			distribution = &dist
			break
		}
	}
	if distribution == nil {
		err = fmt.Errorf("apt-package.codename not supported by apt-enjin: %v - %v", ap.Codename, aptApp.Name)
		return
	}

	tmpName := bePath.Base(tmpPath)
	buildDir := config.Paths.TmpBuild + "/" + tmpName

	pkgIo.STDOUT("# preparing BUILD_DIR...\n")
	if err = cp.Copy(tmpPath, buildDir); err != nil {
		err = fmt.Errorf("error copying to enjin build path: %v - %v", buildDir, err)
		return
	}
	defer func() {
		// cleanup build dir, if success, zip is all that is needed
		pkgIo.STDOUT("# cleaning BUILD_DIR...\n")
		_ = os.RemoveAll(buildDir)
	}()

	pwd := bePath.Pwd()
	if err = os.Chdir(buildDir); err != nil {
		err = fmt.Errorf("chdir error: %v - %v", buildDir, err)
		return
	}
	defer func() {
		_ = os.Chdir(pwd)
	}()

	if !bePath.IsFile("Procfile") {
		err = fmt.Errorf("application Procfile not found: %v - %v/Procfile", app.Name, buildDir)
		return
	}

	var procTypes map[string]string
	if procTypes, err = common.ReadProcfile("Procfile"); err != nil {
		err = fmt.Errorf("error reading Procfile: %v - %v", app.Name, err)
		return
	}

	var commandline string
	if commandline, ok = procTypes[info.RefName]; !ok {
		err = fmt.Errorf("application Procfile does not support %v: %v", info.RefName, app.Name)
		return
	}

	var name string
	var argv []string
	parts := strings.Split(commandline, " ")
	if numParts := len(parts); numParts == 0 {
		err = fmt.Errorf("application Procfile %v target command empty: %v", info.RefName, app.Name)
		return
	} else if numParts > 1 {
		name = parts[0]
		argv = parts[1:]
	} else {
		name = parts[0]
	}

	var gpgInfo map[string][]string
	if gpgInfo, err = app.ImportGpgSecrets(aptApp); err != nil {
		err = fmt.Errorf("error preparing gpg secrets: %v - %v", aptApp.Name, err)
		return
	}

	var signWith string
	for _, fingerprints := range gpgInfo {
		if beStrings.StringInSlices(distribution.SignWith, fingerprints) {
			signWith = distribution.SignWith
			break
		}
	}

	if signWith == "" {
		err = fmt.Errorf("distribution signing key not found: %v [%v] - %v", aptApp.Name, distribution.Codename, distribution.SignWith)
		return
	}

	gpgHome := app.GetGpgHome()
	if !bePath.Exists(gpgHome) {
		if err = os.MkdirAll(gpgHome, 0700); err != nil {
			err = fmt.Errorf("error making gpg home: %v - %v", app.Name, err)
			return
		}
	}

	appOsEnviron := append(
		app.OsEnviron(),
		"GNUPGHOME="+gpgHome,
		"AE_GPG_HOME="+gpgHome,
		"AE_SIGN_KEY="+signWith,
		"AE_ARCHIVES="+aptApp.AptArchivesPath,
		"UNTAGGED_COMMIT="+info.NewRev[:10],
	)

	pkgIo.STDOUT("# starting %v build process: %v - %v\n", info.RefName, name, argv)

	if err = run.ExeWith(&run.Options{Path: ".", Name: name, Argv: argv, Environ: appOsEnviron}); err != nil {
		return
	}

	pkgIo.STDOUT("# signaling for niseroku service reload\n")
	c.config.SignalReloadGitRepository()
	c.config.SignalReloadReverseProxy()
	return
}

func (c *Command) enjinRepoRunBuildpackProcess(app *Application, config *Config, commitId, tmpPath string) (err error) {

	tmpName := bePath.Base(tmpPath)
	buildDir := config.Paths.TmpBuild + "/" + tmpName
	cacheDir := config.Paths.VarCache + "/" + app.Name
	slugZip := config.Paths.VarSlugs + "/" + app.Name + "--" + commitId + ".zip"
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
	defer func() {
		pkgIo.STDOUT("# cleaning ENV_DIR...\n")
		_ = os.RemoveAll(envDir)
	}()
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
	if err = cp.Copy(tmpPath, buildDir); err != nil {
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
		if err = cp.Copy(config.BuildPack, buildPackClonePath); err != nil {
			return
		}
	} else if _, err = git.PlainClone(buildPackClonePath, false, &git.CloneOptions{URL: buildPack}); err != nil {
		return
	}
	defer func() {
		pkgIo.STDOUT("# cleaning enjenv buildpack...\n")
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

	if ae := app.AptEnjin; ae != nil {
		procfile := filepath.Join(buildDir, "Procfile")
		if bePath.IsFile(procfile) {
			if procTypes, ee := common.ReadProcfile(procfile); ee != nil {
				pkgIo.STDERR("apt-enjin Procfile error: %v\n", ee)
			} else {
				if eee := app.PrepareGpgSecrets(); eee != nil {
					pkgIo.STDERR("apt-enjin prepare gpg error: %v\n", eee)
				}
				osEnviron := app.OsEnviron()
				for flavour, _ := range ae.Flavours {
					if command, ok := procTypes[flavour]; ok {
						pkgIo.STDOUT("# apt-enjin: detected Procfile target - %v\n", flavour)
						parts := strings.Split(command, " ")
						var name string
						var args []string
						if numParts := len(parts); numParts > 0 {
							name = parts[0]
							if numParts > 1 {
								args = parts[1:]
							}
						}
						if eee := run.ExeWith(&run.Options{
							Path:    buildDir,
							Name:    name,
							Argv:    args,
							Environ: osEnviron,
						}); eee != nil {
							pkgIo.STDERR("error running Procfile %v: %v\nenv: %+v\n", flavour, eee, osEnviron)
						} else {
							pkgIo.STDOUT("# apt-enjin: Procfile %v process completed\n", flavour)
						}
					}
				}
			}
		}
	}

	pkgIo.STDOUT("# buildpack: compiling...\n")
	if status, err = run.Exe(buildPackClonePath+"/bin/compile", buildDir, cacheDir, envDir); err != nil {
		return
	} else if status != 0 {
		return
	}

	pwd := bePath.Pwd()
	if err = os.Chdir(buildDir); err != nil {
		return
	}

	pkgIo.STDOUT("# compressing built slug\n")
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