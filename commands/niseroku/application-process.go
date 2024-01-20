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

	"github.com/iancoleman/strcase"

	"github.com/go-corelibs/env"
	"github.com/go-corelibs/maps"
	"github.com/go-corelibs/path"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

func (a *Application) IsRunningReady() (runningReady bool) {
	if a.IsDeploying() {
		return
	}
	if slug := a.GetThisSlug(); slug != nil {
		running, ready := slug.IsRunningReady()
		if runningReady = running && ready; runningReady {
			return
		}
	}
	return
}

func (a *Application) GetThisSlug() (slug *Slug) {
	a.RLock()
	defer a.RUnlock()
	if a.ThisSlug != "" {
		if name := path.Base(a.ThisSlug); path.IsFile(a.ThisSlug) {
			if found, ok := a.Slugs[name]; ok {
				slug = found
				return
			}
		}
		a.ThisSlug = ""
	}
	return
}

func (a *Application) GetNextSlug() (slug *Slug) {
	a.RLock()
	defer a.RUnlock()
	if a.NextSlug != "" {
		if name := path.Base(a.NextSlug); path.IsFile(a.NextSlug) {
			if found, ok := a.Slugs[name]; ok {
				slug = found
				return
			}
		}
		a.NextSlug = ""
	}
	return
}

func (a *Application) ApplySettings(envDir string) (err error) {
	a.RLock()
	defer a.RUnlock()
	environ := a.OsEnviron()
	a.LogInfoF("applying settings: %q - %q\n", envDir, environ.Environ())
	err = environ.WriteEnvDir(envDir)
	return
}

func (a *Application) OsEnviron() (environment env.Env) {
	a.RLock()
	defer a.RUnlock()
	environment = env.New()
	environment.Import(os.Environ())
	for _, k := range maps.SortedKeys(a.Settings) {
		key := strcase.ToScreamingSnake(k)
		environment.Set(key, fmt.Sprintf("%v", a.Settings[k]))
	}
	if ae := a.AptEnjin; ae != nil {
		environment.Include(a.OsEnvironAptEnjinOnly())
	}
	return
}

func (a *Application) OsEnvironAptEnjinOnly() (aptEnv env.Env) {
	aptEnv = env.New()
	var ae *AptEnjinConfig
	if ae = a.AptEnjin; ae == nil {
		return
	}
	gnupgHome := filepath.Join(a.Config.Paths.AptSecrets, a.Name, ".gpg")
	aptEnv.Set("GNUPGHOME", gnupgHome)
	aptEnv.Set("SITEKEY", ae.SiteKey)
	aptEnv.Set("SITEURL", ae.SiteUrl)
	aptEnv.Set("SITENAME", ae.SiteName)
	aptEnv.Set("SITEMAIL", ae.SiteMail)
	aptEnv.Set("SITEMAINT", ae.SiteMaint)
	aptEnv.Set("AE_ARCHIVES", a.AptArchivesPath)
	aptEnv.Set("AE_BASEPATH", filepath.Join(a.AptBasePath, "apt-repository"))
	aptEnv.Set("AE_GPG_HOME", gnupgHome)

	if len(ae.GpgKeys) > 0 {
		// TODO: better signing key handling
		var gpgFile string
		var gpgKeys []string
		for _, name := range maps.SortedKeys(ae.GpgKeys) {
			gpgFile = filepath.Join(a.Config.Paths.AptSecrets, a.Name, name)
			gpgKeys = ae.GpgKeys[gpgFile]
			break
		}
		if gpgFile != "" && path.IsFile(gpgFile) {
			if aptEnv.Set("AE_GPG_FILE", gpgFile); len(gpgKeys) > 0 {
				aptEnv.Set("AE_SIGN_KEY", gpgKeys[0])
			}
		}
	}

	return
}

func (a *Application) PurgeGpgSecrets() (err error) {
	home := a.GetGpgHome()
	if path.IsDir(home) {
		if err = os.RemoveAll(home); err != nil {
			return
		}
	}
	return
}

func (a *Application) GetGpgHome() (home string) {
	home = filepath.Join(a.Config.Paths.AptSecrets, a.Name, ".gpg")
	return
}

func (a *Application) ImportGpgSecrets(other *Application) (info map[string][]string, err error) {
	if err = a.PurgeGpgSecrets(); err != nil {
		return
	}

	info = make(map[string][]string)
	home := a.GetGpgHome()

	if err = os.MkdirAll(home, 0700); err != nil {
		return
	}

	otherAptSecrets := filepath.Join(a.Config.Paths.AptSecrets, other.Name)

	found, _ := path.ListFiles(otherAptSecrets, true)
	for _, file := range found {
		if path.IsFile(file) && strings.HasSuffix(file, ".gpg") {
			a.LogInfoF("# importing from other gpg key: %v - %v", other.Name, file)
			if o, e, _, ee := common.Gpg(home, "--import", file); ee != nil {
				a.LogErrorF("error importing %v gpg key: %v - %v", a.Name, ee)
				if o != "" {
					a.LogInfoF("gpg import stdout: %v", o)
				}
				if e != "" {
					a.LogErrorF("gpg import stderr: %v", e)
				}
				continue
			}

			if fingerprints, ee := common.GpgShowOnly(home, file); ee != nil {
				a.LogErrorF("%v", ee)
			} else {
				keyFileName := filepath.Base(file)
				info[keyFileName] = fingerprints
			}
		}
	}

	return
}

func (a *Application) PrepareGpgSecrets() (err error) {

	var ae *AptEnjinConfig
	if ae = a.AptEnjin; ae == nil {
		return
	}

	if err = a.PurgeGpgSecrets(); err != nil {
		return
	}

	aptSecrets := filepath.Join(a.Config.Paths.AptSecrets, a.Name)
	home := a.GetGpgHome()

	if err = os.MkdirAll(home, 0700); err != nil {
		return
	}

	ae.GpgKeys = make(map[string][]string)

	found, _ := path.ListFiles(aptSecrets, true)
	for _, file := range found {
		if strings.HasSuffix(file, ".gpg") {
			// a.LogInfoF("# importing gpg key: %v", file)

			var ee error
			var o, e string
			if o, e, _, ee = common.Gpg(home, "--import", file); ee != nil {
				a.LogErrorF("error importing %v gpg key: %v - %v", a.Name, ee)
				if o != "" {
					a.LogInfoF("gpg import stdout: %v", o)
				}
				if e != "" {
					a.LogErrorF("gpg import stderr: %v", e)
				}
				continue
			}

			if fingerprints, ee := common.GpgShowOnly(home, file); ee != nil {
				a.LogErrorF("%v", ee)
			} else {
				keyFileName := filepath.Base(file)
				ae.GpgKeys[keyFileName] = fingerprints
			}
		}
	}

	// a.LogInfoF("# loaded apt-enjin gpg keys: %v - %+v", a.Name, ae.GpgKeys)

	return
}

func (a *Application) GetSlugWorkerByPid(pid int) (si *SlugWorker) {
	a.RLock()
	defer a.RUnlock()
	for _, slug := range a.Slugs {
		if si = slug.GetInstanceByPid(pid); si != nil {
			return
		}
	}
	return
}
