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
	"github.com/iancoleman/strcase"

	"github.com/go-enjin/be/pkg/maps"
	bePath "github.com/go-enjin/be/pkg/path"

	beIo "github.com/go-enjin/enjenv/pkg/io"
)

type Application struct {
	Name        string   `toml:"-"`
	BinName     string   `toml:"bin-name,omitempty"`
	Domains     []string `toml:"domains,omitempty"`
	Maintenance bool     `toml:"maintenance,omitempty"`
	ThisSlug    string   `toml:"this-slug,omitempty"`
	NextSlug    string   `toml:"next-slug,omitempty"`

	Settings map[string]interface{} `toml:"settings,omitempty"`

	Origin   AppOrigin   `toml:"origin"`
	Timeouts AppTimeouts `toml:"timeouts,omitempty"`

	Slugs     map[string]*Slug `toml:"-"`
	Config    *Config          `toml:"-"`
	Source    string           `toml:"-"`
	GitRepo   *git.Repository  `toml:"-"`
	RepoPath  string           `toml:"-"`
	ErrorLog  string           `toml:"-"`
	AccessLog string           `toml:"-"`
	NoticeLog string           `toml:"-"`

	sync.RWMutex
}

func NewApplication(source string, config *Config) (app *Application, err error) {
	app = &Application{
		Source: source,
		Config: config,
		Slugs:  make(map[string]*Slug),
	}
	err = app.Load()
	return
}

func (a *Application) Load() (err error) {
	a.Lock()
	defer a.Unlock()

	if _, err = toml.DecodeFile(a.Source, &a); err != nil {
		return
	}

	a.Name = bePath.Base(a.Source)

	if a.BinName != "" {
		a.BinName = filepath.Base(a.BinName)
	}

	a.RepoPath = fmt.Sprintf("%v/%v.git", a.Config.Paths.VarRepos, a.Name)
	a.ErrorLog = fmt.Sprintf("%s/%v.error.log", a.Config.Paths.VarLogs, a.Name)
	a.AccessLog = fmt.Sprintf("%s/%v.access.log", a.Config.Paths.VarLogs, a.Name)
	a.NoticeLog = fmt.Sprintf("%s/%v.info.log", a.Config.Paths.VarLogs, a.Name)

	switch {
	case a.Origin.Scheme == "":
		err = fmt.Errorf("scheme setting not found")
	case a.Origin.Host == "":
		err = fmt.Errorf("host setting not found")
	case len(a.Domains) == 0:
		err = fmt.Errorf("domains setting not found")
	case a.ThisSlug != "":
		if !bePath.IsFile(a.ThisSlug) {
			a.ThisSlug = ""
		}
	}

	return
}

func (a *Application) Save() (err error) {
	a.RLock()
	defer a.RUnlock()
	var buffer bytes.Buffer
	if err = toml.NewEncoder(&buffer).Encode(a); err != nil {
		return
	}
	err = os.WriteFile(a.Source, buffer.Bytes(), 0660)
	return
}

func (a *Application) String() string {
	a.RLock()
	defer a.RUnlock()
	return fmt.Sprintf("*%s{\"%s\":[%v]}", a.Name, a.Origin.String(), strings.Join(a.Domains, ","))
}

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
	return
}

func (a *Application) LoadAllSlugs() (err error) {
	var files []string
	if files, err = bePath.ListFiles(a.Config.Paths.VarSlugs); err != nil {
		return
	}
	for _, file := range files {
		name := bePath.Base(file)
		if strings.HasPrefix(name, a.Name+"-") {
			a.RLock()
			if _, exists := a.Slugs[name]; exists {
				a.RUnlock()
				continue
			}
			a.RUnlock()
			if slug, ee := NewSlugFromZip(a, file); ee != nil {
				a.LogErrorF("error making slug from %v: %v\n", file, ee)
			} else {
				a.Lock()
				a.Slugs[slug.Name] = slug
				a.Unlock()
			}
		}
	}
	return
}

func (a *Application) GetThisSlug() (slug *Slug) {
	a.RLock()
	defer a.RUnlock()
	if a.ThisSlug != "" {
		name := bePath.Base(a.ThisSlug)
		slug, _ = a.Slugs[name]
	}
	return
}

func (a *Application) GetNextSlug() (slug *Slug) {
	a.RLock()
	defer a.RUnlock()
	if a.NextSlug != "" {
		name := bePath.Base(a.NextSlug)
		slug, _ = a.Slugs[name]
	}
	return
}

func (a *Application) ApplySettings(envDir string) (err error) {
	// a.LogInfoF("applying settings to: %v\n", envDir)
	a.RLock()
	defer a.RUnlock()
	for _, k := range maps.SortedKeys(a.Settings) {
		key := strcase.ToScreamingSnake(k)
		value := fmt.Sprintf("%v", a.Settings[k])
		if err = os.WriteFile(envDir+"/"+key, []byte(value), 0660); err != nil {
			return
		}
	}
	return
}

func (a *Application) OsEnviron() (environment []string) {
	a.RLock()
	defer a.RUnlock()
	environment = os.Environ()
	for _, k := range maps.SortedKeys(a.Settings) {
		key := strcase.ToScreamingSnake(k)
		environment = append(environment, fmt.Sprintf("%v=%v", key, a.Settings[k]))
	}
	return
}

func (a *Application) LogInfoF(format string, argv ...interface{}) {
	prefix := fmt.Sprintf("[%v] ", a.Name)
	beIo.AppendF(a.NoticeLog, prefix+format, argv...)
}

func (a *Application) LogAccessF(status int, remoteAddr string, r *http.Request) {
	beIo.AppendF(a.AccessLog,
		"[%v] [%v] %v - %v - (%d) - %v %v\n",
		a.Name,
		time.Now().Format("20060102-150405"),
		remoteAddr,
		r.Host,
		status,
		r.Method,
		r.URL.Path,
	)
}

func (a *Application) LogError(err error) {
	beIo.AppendF(a.ErrorLog, "[%v] %v", a.Name, err.Error())
}

func (a *Application) LogErrorF(format string, argv ...interface{}) {
	prefix := fmt.Sprintf("[%v] ", a.Name)
	beIo.AppendF(a.ErrorLog, prefix+format, argv...)
}