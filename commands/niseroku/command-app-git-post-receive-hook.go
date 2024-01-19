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
	"regexp"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/go-git/go-git/v5"
	cp "github.com/otiai10/copy"
	"github.com/sosedoff/gitkit"
	"github.com/urfave/cli/v2"

	"github.com/go-corelibs/chdirs"
	"github.com/go-corelibs/path"
	"github.com/go-corelibs/slices"

	"github.com/go-enjin/be/pkg/cli/run"
	"github.com/go-enjin/enjenv/pkg/globals"
	pkgIo "github.com/go-enjin/enjenv/pkg/io"
	pkgRun "github.com/go-enjin/enjenv/pkg/run"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

func makeCommandAppGitPostReceiveHook(c *Command, app *cli.App) (cmd *cli.Command) {
	cmd = &cli.Command{
		Name:   "git-post-receive-hook",
		Action: c.actionAppGitPostReceiveHook,
		Hidden: true,
	}
	return
}

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
			err = c.enjinRepoPostReceiveHandler(app, c.config, info, tmpPath)
			return
		},
	}

	err = receiver.Handle(os.Stdin)
	return
}

func (c *Command) enjinRepoPostReceiveHandler(app *Application, config *Config, info *gitkit.HookInfo, tmpPath string) (err error) {

	var bpi buildPackInfo
	if bpi, err = c.enjinRepoPrepareBuildpackProcess(app, config, info, tmpPath); err != nil {
		return
	}

	defer func() {
		if ee := c.enjinRepoCleanupBuildpackProcess(app, config, info.NewRev, tmpPath); ee != nil {
			pkgIo.STDERR("error cleaning up buildpack process: %v - %v", app.Name, ee)
		}
	}()

	switch bpi.detected {

	case "enjin-slug":
		err = c.enjinRepoBuildEnjinSlug(bpi)
		return

	case "apt-package":
		err = c.enjinRepoBuildAptPackage(bpi)
		return

	}

	err = fmt.Errorf("unsupported buildpack deployment: %+v", bpi)
	return
}

type buildPackInfo struct {
	app      *Application
	config   *Config
	info     *gitkit.HookInfo
	tmpPath  string
	tmpName  string
	buildDir string
	cacheDir string
	cloneDir string
	envDir   string
	detected string
}

func (c *Command) enjinRepoPrepareBuildpackProcess(app *Application, config *Config, info *gitkit.HookInfo, tmpPath string) (bpi buildPackInfo, err error) {

	bpi = buildPackInfo{
		app:     app,
		config:  config,
		info:    info,
		tmpPath: tmpPath,
	}
	bpi.tmpName = path.Base(tmpPath)
	bpi.buildDir = config.Paths.TmpBuild + "/" + bpi.tmpName
	bpi.cacheDir = config.Paths.VarCache + "/" + app.Name
	bpi.cloneDir = config.Paths.TmpClone + "/" + app.Name
	bpi.envDir = config.Paths.VarSettings + "/" + app.Name

	pkgIo.STDOUT("# preparing ENV_DIR...\n")
	if err = os.RemoveAll(bpi.envDir); err != nil {
		err = fmt.Errorf("error removing enjin env path: %v - %v", bpi.envDir, err)
		return
	} else if err = path.MkdirAll(bpi.envDir); err != nil {
		err = fmt.Errorf("error making enjin env path: %v - %v", bpi.envDir, err)
		return
	} else if err = app.ApplySettings(bpi.envDir); err != nil {
		err = fmt.Errorf("error applying enjin env path: %v - %v", bpi.envDir, err)
		return
	}

	pkgIo.STDOUT("# preparing CACHE_DIR...\n")
	if !path.IsDir(bpi.cacheDir) {
		if err = path.MkdirAll(bpi.cacheDir); err != nil {
			err = fmt.Errorf("error making enjin deployment path: %v - %v", bpi.cacheDir, err)
			return
		}
	}

	pkgIo.STDOUT("# preparing BUILD_DIR...\n")
	if err = cp.Copy(tmpPath, bpi.buildDir); err != nil {
		err = fmt.Errorf("error copying to enjin build path: %v - %v", bpi.buildDir, err)
		return
	}

	pkgIo.STDOUT("# preparing enjenv buildpack...\n")
	var buildPack string
	if config.BuildPack != "" {
		buildPack = config.BuildPack
	} else {
		buildPack = DefaultBuildPack
	}
	if path.IsDir(config.BuildPack) {
		if err = cp.Copy(config.BuildPack, bpi.cloneDir); err != nil {
			return
		}
	} else if _, err = git.PlainClone(bpi.cloneDir, false, &git.CloneOptions{URL: buildPack}); err != nil {
		return
	}

	pkgIo.STDOUT("# buildpack: detecting...\n")

	var status int
	var o string
	if o, _, status, err = run.Cmd(bpi.cloneDir+"/bin/detect", bpi.buildDir); err != nil {
		err = fmt.Errorf("error running buildpack detection: %v", err)
		return
	} else if status > 0 {
		err = fmt.Errorf("buildpack exited with non-zero status: %d", status)
		return
	} else if bpi.detected = strings.TrimSpace(o); bpi.detected == "" {
		err = fmt.Errorf("buildpack did not detect any enjin")
		return
	}

	pkgIo.STDOUT("# buildpack: detected: %v\n", bpi.detected)

	return
}

func (c *Command) enjinRepoCleanupBuildpackProcess(app *Application, config *Config, commitId, tmpPath string) (err error) {

	tmpName := path.Base(tmpPath)
	buildDir := config.Paths.TmpBuild + "/" + tmpName
	buildPackClonePath := config.Paths.TmpClone + "/" + app.Name
	envDir := config.Paths.VarSettings + "/" + app.Name

	pkgIo.STDOUT("# cleaning ENV_DIR...\n")
	_ = os.RemoveAll(envDir)
	pkgIo.STDOUT("# cleaning BUILD_DIR...\n")
	_ = os.RemoveAll(buildDir)
	pkgIo.STDOUT("# cleaning enjenv buildpack...\n")
	_ = os.RemoveAll(buildPackClonePath)
	return
}

func (c *Command) enjinRepoBuildEnjinSlug(bpi buildPackInfo) (err error) {

	if bpi.info.RefName != "main" {
		err = fmt.Errorf("unsupported enjin slug target branch: %v", bpi.info.RefName)
		return
	}

	commitId := bpi.info.NewRev
	slugZip := bpi.config.Paths.VarSlugs + "/" + bpi.app.Name + "--" + commitId + ".zip"

	if ae := bpi.app.AptEnjin; ae != nil {
		procfile := filepath.Join(bpi.buildDir, "Procfile")
		if path.IsFile(procfile) {
			if procTypes, ee := common.ReadProcfile(procfile); ee != nil {
				pkgIo.STDERR("apt-enjin Procfile error: %v\n", ee)
			} else {
				if eee := bpi.app.PrepareGpgSecrets(); eee != nil {
					pkgIo.STDERR("apt-enjin prepare gpg error: %v\n", eee)
				}
				environ := bpi.app.OsEnviron()
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
							Path:    bpi.buildDir,
							Name:    name,
							Argv:    args,
							Environ: environ.Environ(),
						}); eee != nil {
							pkgIo.STDERR("error running Procfile %v: %v\nenv: %+v\n", flavour, eee, environ.Environ())
						} else {
							pkgIo.STDOUT("# apt-enjin: Procfile %v process completed\n", flavour)
						}
					}
				}
			}
		}
	}

	var status int
	pkgIo.STDOUT("# buildpack: compiling...\n")
	if status, err = run.Exe(bpi.cloneDir+"/bin/compile", bpi.buildDir, bpi.cacheDir, bpi.envDir); err != nil {
		return
	} else if status != 0 {
		return
	}

	if err = chdirs.Push(bpi.buildDir); err != nil {
		return
	}
	defer chdirs.Pop()

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

	err = pkgRun.EnjenvExe("niseroku", "deploy-slug", slugZip)
	return
}

var RxExportLine = regexp.MustCompile(`^\s*export (.+?)=(['"]?)(.+?)(['"]?)\s*$`)

func (c *Command) enjinRepoBuildAptPackage(bpi buildPackInfo) (err error) {

	var ok bool
	var aptApp *Application
	var ap *AptPackageConfig
	var ae *AptEnjinConfig
	var dists []Distribution

	if ap = bpi.app.AptPackage; ap == nil {
		err = fmt.Errorf("unsupported branch received: %v", bpi.info.RefName)
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
	} else if dists, ok = ae.Flavours[bpi.info.RefName]; !ok {
		err = fmt.Errorf("apt-enjin flavour not supported: %v", bpi.info.RefName)
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

	environ := bpi.app.OsEnviron()
	enjenvPath := bpi.cacheDir
	environ.Set("ENJENV_PATH", enjenvPath)
	golangVersion := environ.String("ENJENV_BUILDPACK_GOLANG", globals.DefaultGolangVersion)

	if err = pkgRun.EnjenvExeWith(bpi.buildDir, environ.Environ(), "init", "--force", "--golang", golangVersion); err != nil {
		err = pkgIo.ErrorF("enjenv golang init error: %v", err)
		return
	}

	var exportOutput string
	if exportOutput, _, err = pkgRun.EnjenvCmdWith(bpi.buildDir, environ.Environ(), "export"); err != nil {
		err = pkgIo.ErrorF("enjenv export env error: %v", err)
		return
	}

	pkgIo.STDOUT("export output:\n%v\n", exportOutput)

	for _, line := range strings.Split(exportOutput, "\n") {
		if RxExportLine.MatchString(line) {
			m := RxExportLine.FindAllStringSubmatch(line, 1)
			k, v := m[0][1], m[0][3]
			environ.Set(k, v)
		}
	}

	if err = chdirs.Push(bpi.buildDir); err != nil {
		err = fmt.Errorf("chdir error: %v - %v", bpi.buildDir, err)
		return
	}
	defer chdirs.Pop()

	if !path.IsFile("Procfile") {
		err = fmt.Errorf("application Procfile not found: %v - %v/Procfile", bpi.app.Name, bpi.buildDir)
		return
	}

	var procTypes map[string]string
	if procTypes, err = common.ReadProcfile("Procfile"); err != nil {
		err = fmt.Errorf("error reading Procfile: %v - %v", bpi.app.Name, err)
		return
	}

	var commandline string
	if commandline, ok = procTypes[bpi.info.RefName]; !ok {
		err = fmt.Errorf("application Procfile does not support %v: %v", bpi.info.RefName, bpi.app.Name)
		return
	}

	var name string
	var makeFlavourArgv []string
	parts := strings.Split(commandline, " ")
	if numParts := len(parts); numParts == 0 {
		err = fmt.Errorf("application Procfile %v target command empty: %v", bpi.info.RefName, bpi.app.Name)
		return
	} else if numParts > 1 {
		name = parts[0]
		makeFlavourArgv = parts[1:]
	} else {
		name = parts[0]
	}

	var gpgInfo map[string][]string
	if gpgInfo, err = bpi.app.ImportGpgSecrets(aptApp); err != nil {
		err = fmt.Errorf("error preparing gpg secrets: %v - %v", aptApp.Name, err)
		return
	}

	var signWith string
	for _, fingerprints := range gpgInfo {
		if slices.Within(distribution.SignWith, fingerprints) {
			signWith = distribution.SignWith
			break
		}
	}

	if signWith == "" {
		err = fmt.Errorf("distribution signing key not found: %v [%v] - %v", aptApp.Name, distribution.Codename, distribution.SignWith)
		return
	}

	gpgHome := bpi.app.GetGpgHome()
	if !path.Exists(gpgHome) {
		if err = os.MkdirAll(gpgHome, 0700); err != nil {
			err = fmt.Errorf("error making gpg home: %v - %v", bpi.app.Name, err)
			return
		}
	}

	environ.Set("GNUPGHOME", gpgHome)
	environ.Set("AE_GPG_HOME", gpgHome)
	environ.Set("AE_SIGN_KEY", signWith)
	environ.Set("AE_ARCHIVES", aptApp.AptArchivesPath)
	environ.Set("UNTAGGED_COMMIT", bpi.info.NewRev[:10])

	pkgIo.STDOUT("# starting %v build process: %v - %v\n", bpi.info.RefName, name, makeFlavourArgv)

	if err = run.ExeWith(&run.Options{Path: ".", Name: name, Argv: makeFlavourArgv, Environ: environ.Environ()}); err != nil {
		return
	}

	pkgIo.STDOUT("# signaling niseroku git-repository reload\n")
	c.config.SignalReloadGitRepository()
	return
}
