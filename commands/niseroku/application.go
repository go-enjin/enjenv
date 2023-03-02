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
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/go-git/go-git/v5"

	bePath "github.com/go-enjin/be/pkg/path"

	beIo "github.com/go-enjin/enjenv/pkg/io"
)

type Application struct {
	Maintenance bool     `toml:"maintenance,omitempty"`
	Domains     []string `toml:"domains,omitempty"`

	AptPackage *AptPackageConfig `toml:"apt-package,omitempty"`
	AptEnjin   *AptEnjinConfig   `toml:"apt-enjin,omitempty"`

	Workers map[string]int `toml:"workers,omitempty"`

	Timeouts AppTimeouts `toml:"timeouts,omitempty"`

	Settings map[string]interface{} `toml:"settings,omitempty"`

	Origin AppOrigin `toml:"origin"`

	ThisSlug string `toml:"this-slug,omitempty"`
	NextSlug string `toml:"next-slug,omitempty"`

	Name      string           `toml:"-"`
	Slugs     map[string]*Slug `toml:"-"`
	Config    *Config          `toml:"-"`
	Source    string           `toml:"-"`
	GitRepo   *git.Repository  `toml:"-"`
	PidFile   string           `toml:"-"`
	RepoPath  string           `toml:"-"`
	ErrorLog  string           `toml:"-"`
	AccessLog string           `toml:"-"`
	NoticeLog string           `toml:"-"`

	AptBasePath       string `toml:"-"`
	AptArchivesPath   string `toml:"-"`
	AptRepositoryPath string `toml:"-"`

	awaitWorkersDone chan bool

	tomlComments TomlComments

	sync.RWMutex
}

func LoadApplications(config *Config) (foundApps map[string]*Application, err error) {
	var appConfigs []string
	if appConfigs, err = bePath.ListFiles(config.Paths.EtcApps); err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			err = nil
		}
		return
	}
	foundApps = make(map[string]*Application)
	for _, appConfig := range appConfigs {
		if !strings.HasSuffix(appConfig, ".toml") {
			continue
		}
		var app *Application
		if app, err = NewApplication(appConfig, config); err != nil {
			return
		}
		if _, exists := foundApps[app.Name]; exists {
			err = fmt.Errorf("app already exists: %v (%v)", app.Name, appConfig)
			return
		}
		foundApps[app.Name] = app
	}
	return
}

func NewApplication(source string, config *Config) (app *Application, err error) {
	app = &Application{
		Source:           source,
		Config:           config,
		Slugs:            make(map[string]*Slug),
		awaitWorkersDone: make(chan bool, 1),
		Workers:          make(map[string]int),
	}
	if err = app.Load(); err != nil {
		return
	}
	if err = app.LoadAllSlugs(); err != nil {
		err = fmt.Errorf("error loading app slugs: %v - %v", app.Name, err)
	}
	return
}

func (a *Application) Load() (err error) {
	a.Lock()
	defer a.Unlock()

	var contents string
	if b, ee := os.ReadFile(a.Source); ee != nil {
		err = ee
		return
	} else {
		contents = string(b)
	}

	var md toml.MetaData
	if md, err = toml.Decode(contents, &a); err != nil {
		return
	}

	if !md.IsDefined("workers") {
		a.Workers["web"] = 1
	}

	if tcs, ee := ParseComments(contents); ee != nil {
		err = ee
		return
	} else {
		a.tomlComments = MergeApplicationToml(tcs, true)
	}

	a.Name = bePath.Base(a.Source)

	a.RepoPath = fmt.Sprintf("%v/%v.git", a.Config.Paths.VarRepos, a.Name)
	a.ErrorLog = fmt.Sprintf("%s/%v.error.log", a.Config.Paths.VarLogs, a.Name)
	a.AccessLog = fmt.Sprintf("%s/%v.access.log", a.Config.Paths.VarLogs, a.Name)
	a.NoticeLog = fmt.Sprintf("%s/%v.info.log", a.Config.Paths.VarLogs, a.Name)

	if a.AptPackage != nil {
		// apt-packages do not require a valid origin or having any domains
		if a.AptPackage.AptEnjin == "" {
			a.AptPackage = nil
		} else {
			a.AptBasePath = filepath.Join(a.Config.Paths.VarAptRoot, a.AptPackage.AptEnjin)
			a.AptArchivesPath = filepath.Join(a.AptBasePath, "apt-archives")
			a.AptRepositoryPath = filepath.Join(a.AptBasePath, "apt-repository")
		}
	} else {
		// standard enjins and apt-enjins require origin and at least one domain
		switch {
		case a.Origin.Scheme == "":
			err = fmt.Errorf("scheme setting not found")
		case a.Origin.Host == "":
			err = fmt.Errorf("host setting not found")
		case len(a.Domains) == 0:
			err = fmt.Errorf("domains setting not found")
		}
	}

	if a.AptEnjin != nil {
		a.AptBasePath = fmt.Sprintf("%v/%v", a.Config.Paths.VarAptRoot, a.Name)
		a.AptArchivesPath = filepath.Join(a.AptBasePath, "apt-archives")
		a.AptRepositoryPath = filepath.Join(a.AptBasePath, "apt-repository")
	}

	if a.ThisSlug != "" && !bePath.IsFile(a.ThisSlug) {
		a.ThisSlug = ""
	}
	if a.NextSlug != "" && !bePath.IsFile(a.NextSlug) {
		a.NextSlug = ""
	}

	return
}

func (a *Application) Save(keepCustomComments bool) (err error) {
	a.RLock()
	defer a.RUnlock()
	var buffer bytes.Buffer
	if err = toml.NewEncoder(&buffer).Encode(a); err != nil {
		return
	}
	contents := string(buffer.Bytes())
	var modified string
	tcs := MergeApplicationToml(a.tomlComments, keepCustomComments)
	if modified, err = ApplyComments(contents, tcs); err != nil {
		return
	}
	err = os.WriteFile(a.Source, []byte(modified), 0660)
	return
}

func (a *Application) String() string {
	a.RLock()
	defer a.RUnlock()
	return fmt.Sprintf("*%s{\"%s\":[%v]}", a.Name, a.Origin.String(), strings.Join(a.Domains, ","))
}

func (a *Application) LoadAllSlugs() (err error) {
	a.Lock()
	defer a.Unlock()

	var files []string
	if files, err = bePath.ListFiles(a.Config.Paths.VarSlugs); err != nil {
		return
	}

	for _, file := range files {
		name := bePath.Base(file)
		if _, exists := a.Slugs[name]; !exists {
			if strings.HasPrefix(name, a.Name+"--") {
				if slug, ee := NewSlugFromZip(a, file); ee != nil {
					a.LogErrorF("error making slug from %v: %v\n", file, ee)
				} else {
					a.Slugs[slug.Name] = slug
				}
			}
		}
	}
	return
}

func (a *Application) LogInfoF(format string, argv ...interface{}) {
	beIo.AppendF(a.NoticeLog, "["+a.Name+"] "+format, argv...)
}

func (a *Application) LogAccessF(status int, remoteAddr string, r *http.Request, start time.Time) {
	beIo.AppendF(a.AccessLog,
		"[%v] [%v] %v - %v - (%d) - %v %v (%v)\n",
		a.Name,
		time.Now().Format("20060102-150405"),
		remoteAddr,
		r.Host,
		status,
		r.Method,
		r.URL.Path,
		time.Now().Sub(start).String(),
	)
}

func (a *Application) LogError(err error) {
	beIo.AppendF(a.ErrorLog, "[%v] %v", a.Name, err.Error())
}

func (a *Application) LogErrorF(format string, argv ...interface{}) {
	prefix := fmt.Sprintf("[%v] ", a.Name)
	beIo.AppendF(a.ErrorLog, prefix+format, argv...)
}