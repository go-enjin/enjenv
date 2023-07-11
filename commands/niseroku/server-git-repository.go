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
	"sync"

	"github.com/knqyf263/go-deb-version"
	"github.com/sosedoff/gitkit"

	"github.com/go-enjin/be/pkg/cli/run"
	"github.com/go-enjin/be/pkg/log"
	"github.com/go-enjin/be/pkg/maps"
	bePath "github.com/go-enjin/be/pkg/path"

	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/profiling"
	pkgRun "github.com/go-enjin/enjenv/pkg/run"
	"github.com/go-enjin/enjenv/pkg/service"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

type GitRepository struct {
	service.Service

	config *Config

	repo  *gitkit.SSH
	gkcfg gitkit.Config
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
	return
}

func (gr *GitRepository) Bind() (err error) {

	if err = gr.performReload(); err != nil {
		err = fmt.Errorf("error performing reload on startup: %v", err)
		return
	}

	gr.Lock()
	defer gr.Unlock()

	addr := fmt.Sprintf("%v:%d", gr.config.BindAddr, gr.config.Ports.Git)

	gr.gkcfg = gitkit.Config{
		Dir:        gr.config.Paths.VarRepos,
		KeyDir:     gr.config.Paths.RepoSecrets,
		Auth:       true,
		AutoHooks:  false,
		AutoCreate: false,
		// Hooks: &gitkit.HookScripts{
		// 	PreReceive:  preReceiveHookSource,
		// 	PostReceive: postReceiveHookSource,
		// },
	}
	if err = gr.gkcfg.Setup(); err != nil {
		err = fmt.Errorf("error setting up git config: %v", err)
		return
	}
	gr.repo = gitkit.NewSSH(gr.gkcfg)

	gr.repo.PublicKeyLookupFunc = gr.publicKeyLookupFunc

	err = gr.repo.Listen(addr)
	return
}

func (gr *GitRepository) Serve() (err error) {

	go gr.HandleSIGHUP()

	// SIGINT+TERM handler
	idleConnectionsClosed := make(chan struct{})
	go func() {
		gr.HandleSIGINT()
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
	profiling.Stop()
	return
}

func (gr *GitRepository) Reload() (err error) {
	gr.LogInfoF("git-repository reloading config")
	if err = gr.config.Reload(); err != nil {
		gr.LogErrorF("error reloading config: %v", err)
		return
	}
	gr.LogInfoF("git-repository config reloaded")
	err = gr.performReload()
	return
}

func (gr *GitRepository) performReload() (err error) {
	gr.Lock()
	defer gr.Unlock()

	for _, app := range maps.ValuesSortedByKeys(gr.config.Applications) {
		if ee := app.SetupRepo(); ee != nil {
			gr.LogErrorF("error updating git repo setup: %v - %v", app.Name, ee)
		}
	}

	if ee := gr.updateGitHookScripts(); ee != nil {
		gr.LogErrorF("error updating git hook scripts: %v", ee)
	} else {
		gr.LogInfoF("git hook scripts updated")
	}

	if ee := gr.updateAptEnjins(); ee != nil {
		gr.LogErrorF("error updating apt-enjins: %v", ee)
	}

	gr.LogInfoF("starting fix-fs process")
	if _, ee := pkgRun.EnjenvBg(gr.config.LogFile, gr.config.LogFile, "niseroku", "fix-fs"); ee != nil {
		gr.LogErrorF("error fixing filesystem: %v", ee)
	}

	return
}

func (gr *GitRepository) publicKeyLookupFunc(inputPubKey string) (pubkey *gitkit.PublicKey, err error) {
	var ok bool
	var comment, inputKeyId string
	if _, _, comment, inputKeyId, ok = common.ParseSshKey(inputPubKey); !ok {
		err = fmt.Errorf("unable to parse SSH key: %v", inputPubKey)
		return
	} else if comment == "" {
		comment = "nil"
	}
	gr.RLock()
	defer gr.RUnlock()
	for _, u := range gr.config.Users {
		if u.HasKey(inputKeyId) {
			gr.LogInfoF("validated user with ssh-key: %v (%v)\n", u.Name, comment)
			pubkey = &gitkit.PublicKey{
				Id: inputKeyId,
			}
			return
		}
	}
	err = fmt.Errorf("user not found")
	gr.LogErrorF("user not found with ssh-key: %v\n", inputPubKey)
	return
}

const (
	gPreReceiveHookTemplate  = "#!/bin/bash\ncat - | %v niseroku --config=%v app git-pre-receive-hook\n"
	gPostReceiveHookTemplate = "#!/bin/bash\ncat - | %v niseroku --config=%v app git-post-receive-hook\n"
)

func (gr *GitRepository) updateGitHookScripts() (err error) {

	binPath := basepath.WhichBin()
	preReceiveHookSource := fmt.Sprintf(gPreReceiveHookTemplate, binPath, gr.config.Source)
	postReceiveHookSource := fmt.Sprintf(gPostReceiveHookTemplate, binPath, gr.config.Source)

	for _, app := range gr.config.Applications {
		if app.RepoPath == "" {
			gr.LogInfoF("no hook updates possible, app repo path missing: %v\n", app.Name)
			continue
		}
		hookDir := app.RepoPath + "/hooks"
		if !bePath.IsDir(hookDir) {
			if err = os.Mkdir(hookDir, 0770); err != nil {
				gr.LogErrorF("error making git hooks directory: %v - %v\n", hookDir, err)
				continue
			}
		}
		preReceiveHookPath := hookDir + "/pre-receive"
		if err = os.WriteFile(preReceiveHookPath, []byte(preReceiveHookSource), 0660); err != nil {
			gr.LogErrorF("error writing git pre-receive hook: %v - %v\n", preReceiveHookPath, err)
		} else if err = os.Chmod(preReceiveHookPath, 0770); err != nil {
			gr.LogErrorF("error changing mode of git pre-receive hook: %v - %v\n", preReceiveHookPath, err)
		}
		postReceiveHookPath := hookDir + "/post-receive"
		if err = os.WriteFile(postReceiveHookPath, []byte(postReceiveHookSource), 0660); err != nil {
			gr.LogErrorF("error writing git post-receive hook: %v - %v\n", postReceiveHookPath, err)
		} else if err = os.Chmod(postReceiveHookPath, 0770); err != nil {
			gr.LogErrorF("error changing mode of git post-receive hook: %v - %v\n", postReceiveHookPath, err)
		}
		if ee := common.RepairOwnership(hookDir, gr.User, gr.Group); ee != nil {
			gr.LogErrorF("error repairing ownership of git-hooks: %v - %v", hookDir, ee)
		}
	}

	return
}

func (gr *GitRepository) updateAptEnjins() (err error) {

	var restarts []string

	for _, app := range gr.config.Applications {
		var ae *AptEnjinConfig
		if ae = app.AptEnjin; ae == nil {
			continue
		}
		if ae.Enable {
			var restart bool

			for flavour, dists := range ae.Flavours {
				flavourPath := filepath.Join(app.AptRepositoryPath, flavour)
				confDir := filepath.Join(flavourPath, "conf")
				if err = bePath.Mkdir(confDir); err != nil {
					return
				}

				distsFile := filepath.Join(confDir, "distributions")
				distsContent := Distributions(dists).String()
				if err = os.WriteFile(distsFile, []byte(distsContent), 0660); err != nil {
					return
				}

				for _, dist := range dists {
					archivesPath := filepath.Join(app.AptArchivesPath, flavour)
					if err = bePath.Mkdir(archivesPath); err != nil {
						return
					}

					var changed bool
					if changed, err = gr.processAptRepository(app, ae, dist.Codename, flavourPath, archivesPath); err != nil {
						return
					}
					if changed {
						restart = true
					}
				}
			}

			if restart {
				restarts = append(restarts, app.Name)
			}
		}
	}

	if len(restarts) > 0 && gr.config.IsReverseProxyRunning() {
		for _, appName := range restarts {
			go func(appName string) {
				gr.LogInfoF("restarting apt-enjin application: %v", appName)
				if _, ee := pkgRun.EnjenvBg(gr.config.LogFile, "-", "niseroku", "--config", gr.config.Source, "app", "restart", appName); ee != nil {
					gr.LogErrorF("error calling niseroku app restart %v: %v", appName, ee)
				}
			}(appName)
		}
	}
	return
}

func (gr *GitRepository) processAptRepository(app *Application, ae *AptEnjinConfig, codename, flavourPath, archivesPath string) (changed bool, err error) {
	var found []string
	if found, err = bePath.ListFiles(archivesPath); err != nil {
		return
	}

	if err = app.PrepareGpgSecrets(); err != nil {
		return
	}

	appOsEnviron := app.OsEnviron()

	listInfos := gr.repreproList(flavourPath, codename, appOsEnviron)

	var processDSC []*ParsedDebianFile
	var processDEB []*ParsedDebianFile

	for _, file := range found {
		if strings.HasSuffix(file, ".dsc") {

			if parsed, ok := ParseDebianDscFilename(file); ok {
				var proceed bool = true
				if entries, ok := listInfos[parsed.Name]; ok {
					for _, entry := range entries {
						if entry.Architecture == "source" {
							if entry.Version.GreaterThan(parsed.Version) || entry.Version.Equal(parsed.Version) {
								proceed = false
								break
							}
						}
					}
				}
				if proceed {
					processDSC = append(processDSC, parsed)
				}
			} else {
				gr.LogErrorF("error parsing debian dsc filename: %v", file)
			}

		} else if strings.HasSuffix(file, ".deb") {

			if parsed, ok := ParseDebianDebFilename(file); ok {
				var proceed bool = true
				if entries, ok := listInfos[parsed.Name]; ok {
					for _, entry := range entries {
						if parsed.Arch == "all" {
							if entry.Architecture == "source" {
								continue
							} else {
								proceed = false
								break
							}
						} else if entry.Architecture != parsed.Arch {
							continue
						} else if entry.Version.GreaterThan(parsed.Version) || entry.Version.Equal(parsed.Version) {
							proceed = false
							break
						}
					}
				}
				if proceed {
					processDEB = append(processDEB, parsed)
				}
			} else {
				gr.LogErrorF("error parsing debian dsc filename: %v", file)
			}

		}
	}

	processParsedList := func(list []*ParsedDebianFile) (unique map[string]*ParsedDebianFile) {
		unique = make(map[string]*ParsedDebianFile)
		for _, parsed := range list {
			if _, exists := unique[parsed.Name]; !exists {
				unique[parsed.Name] = parsed
			} else {
				if unique[parsed.Name].Version.LessThan(parsed.Version) {
					unique[parsed.Name] = parsed
				}
			}
		}
		return
	}

	uniqueDSC := processParsedList(processDSC)
	for _, name := range maps.SortedKeys(uniqueDSC) {
		dsc := uniqueDSC[name]
		gr.LogInfoF("apt-repository processing [dsc]:\nrepository=%v\ntarget=%v", flavourPath, dsc.File)
		gr.reprepro("includedsc", flavourPath, codename, dsc.File, gr.LogFile, appOsEnviron)
	}

	uniqueDEB := processParsedList(processDEB)
	for _, name := range maps.SortedKeys(uniqueDEB) {
		deb := uniqueDEB[name]
		gr.LogInfoF("apt-repository processing [deb]:\nrepository=%v\ntarget=%v", flavourPath, deb.File)
		gr.reprepro("includedeb", flavourPath, codename, deb.File, gr.LogFile, appOsEnviron)
	}

	// this is not fun, always changed if any packages present
	changed = len(uniqueDSC) > 0 || len(uniqueDEB) > 0
	return
}

var RxDscFileName = regexp.MustCompile(`^\s*(.+?)_(.+?)\.dsc\s*$`)

type ParsedDebianFile struct {
	Type    string
	File    string
	Name    string
	Arch    string
	RawVer  string
	Version version.Version
}

func ParseDebianDscFilename(file string) (parsed *ParsedDebianFile, ok bool) {
	filename := filepath.Base(file)
	if ok = RxDscFileName.MatchString(filename); ok {
		m := RxDscFileName.FindAllStringSubmatch(filename, 1)
		if v, err := version.NewVersion(m[0][2]); err != nil {
			ok = false
			log.ErrorF("error parsing dsc version: %v - %v", filename, err)
		} else {
			parsed = &ParsedDebianFile{
				Type:    "dsc",
				File:    file,
				Name:    m[0][1],
				RawVer:  m[0][2],
				Version: v,
			}
		}
	}
	return
}

var RxDebFileName = regexp.MustCompile(`^\s*(.+?)_(.+?)_(.+?)\.u?deb\s*$`)

func ParseDebianDebFilename(file string) (parsed *ParsedDebianFile, ok bool) {
	filename := filepath.Base(file)
	if ok = RxDebFileName.MatchString(filename); ok {
		m := RxDebFileName.FindAllStringSubmatch(filename, 1)
		if v, err := version.NewVersion(m[0][2]); err != nil {
			ok = false
			log.ErrorF("error parsing deb version: %v - %v", filename, err)
		} else {
			parsed = &ParsedDebianFile{
				Type:    "deb",
				File:    file,
				Name:    m[0][1],
				Arch:    m[0][3],
				RawVer:  m[0][2],
				Version: v,
			}
		}
	}
	return
}

var RxRepreproList = regexp.MustCompile(`^\s*([^|]+)\|([^|]+)\|([^|]+):\s*(\S+)\s*(\S+)\s*$`)

type RepreproListEntry struct {
	Codename     string
	Component    string
	Architecture string
	Name         string
	RawVer       string
	Version      version.Version
}

func (gr *GitRepository) repreproList(flavourPath, codename string, appOsEnviron []string) (info map[string][]RepreproListEntry) {
	info = make(map[string][]RepreproListEntry)
	argv := []string{"-b", flavourPath, "list", codename}
	if o, _, _, err := run.CmdWith(&run.Options{
		Path:    flavourPath,
		Name:    "reprepro",
		Argv:    argv,
		Environ: appOsEnviron,
	}); err != nil {
		gr.LogErrorF("reprepro CmdWith error: %v - %v", argv, err)
	} else {
		for _, line := range strings.Split(o, "\n") {
			if RxRepreproList.MatchString(line) {
				m := RxRepreproList.FindAllStringSubmatch(line, 1)
				codeName, component, arch, name, ver := m[0][1], m[0][2], m[0][3], m[0][4], m[0][5]
				if v, ee := version.NewVersion(ver); ee != nil {
					log.ErrorF("error parsing deb version: %v - %v", line, ee)
				} else {
					info[name] = append(info[name], RepreproListEntry{
						Codename:     codeName,
						Component:    component,
						Architecture: arch,
						Name:         name,
						RawVer:       ver,
						Version:      v,
					})
				}
			}
		}
	}
	return
}

func (gr *GitRepository) reprepro(operation string, flavourPath, codename, archive, logFile string, appOsEnviron []string) {
	argv := []string{"-s", "-s", "-b", flavourPath, operation, codename, archive}
	if err := run.ExeWith(&run.Options{
		Path:    flavourPath,
		Name:    "reprepro",
		Stdout:  logFile,
		Stderr:  logFile,
		Argv:    argv,
		Environ: appOsEnviron,
	}); err != nil {
		gr.LogErrorF("reprepro ExeWith error: %v - %v", argv, err)
	} else {
		gr.LogInfoF("reprepro ExeWith success: %v", argv)
	}
}