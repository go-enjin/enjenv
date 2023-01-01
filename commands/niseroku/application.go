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
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/go-git/go-git/v5"
	"github.com/iancoleman/strcase"

	"github.com/go-enjin/be/pkg/maps"
	bePath "github.com/go-enjin/be/pkg/path"
	beIo "github.com/go-enjin/enjenv/pkg/io"
)

type Application struct {
	Name     string                 `toml:"name"`
	Binary   string                 `toml:"binary,omitempty"`
	Scheme   string                 `toml:"scheme,omitempty"`
	Host     string                 `toml:"host,omitempty"`
	Port     int                    `toml:"port,omitempty,omitempty"`
	Domains  []string               `toml:"domains,omitempty"`
	SshKeys  []string               `toml:"ssh-keys,omitempty"`
	Settings map[string]interface{} `toml:"settings,omitempty"`

	ThisSlug string `toml:"this-slug,omitempty"`
	NextSlug string `toml:"next-slug,omitempty"`

	Timeouts AppTimeouts `toml:"timeouts,omitempty"`

	Source     string           `toml:"-"`
	RepoPath   string           `toml:"-"`
	Repository *git.Repository  `toml:"-"`
	Config     *Config          `toml:"-"`
	Slugs      map[string]*Slug `toml:"-"`
}

type AppTimeouts struct {
	SlugStartup   *time.Duration `toml:"slug-startup,omitempty"`
	ReadyInterval *time.Duration `toml:"ready-interval,omitempty"`
	OriginRequest *time.Duration `toml:"origin-request,omitempty"`
}

func NewApplication(source string, config *Config) (app *Application, err error) {
	app = &Application{
		Source: source,
		Config: config,
	}
	if err = app.Load(); err == nil {
		beIo.StdoutF("loaded application: %v\n", app.Name)
	}
	return
}

func (a *Application) Load() (err error) {
	if _, err = toml.DecodeFile(a.Source, &a); err != nil {
		return
	}

	switch {
	case a.Binary == "":
		err = fmt.Errorf("binary setting not found")
	case a.Scheme == "":
		err = fmt.Errorf("scheme setting not found")
	case a.Host == "":
		err = fmt.Errorf("host setting not found")
	case len(a.Domains) == 0:
		err = fmt.Errorf("domains setting not found")
	case len(a.SshKeys) == 0:
		err = fmt.Errorf("ssh-keys setting not found")
	case a.ThisSlug != "":
		if !bePath.IsFile(a.ThisSlug) {
			beIo.StderrF("slug setting found, slug file not found, clearing setting\n")
			a.ThisSlug = ""
		}
	}

	a.RepoPath = fmt.Sprintf("%v/%v.git", a.Config.Paths.VarRepos, a.Name)

	return
}

func (a *Application) Save() (err error) {
	var buffer bytes.Buffer
	if err = toml.NewEncoder(&buffer).Encode(a); err != nil {
		return
	}
	err = os.WriteFile(a.Source, buffer.Bytes(), 0660)
	return
}

func (a *Application) String() string {
	return fmt.Sprintf("*%s{\"%s://%s:%d\":[%v]}", a.Name, a.Scheme, a.Host, a.Port, strings.Join(a.Domains, ","))
}

func (a *Application) SetupRepo() (err error) {
	if a.Repository != nil {
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
	if a.Repository, err = git.PlainInit(a.RepoPath, true); err != nil && err == git.ErrRepositoryAlreadyExists {
		a.Repository, err = git.PlainOpen(a.RepoPath)
	}
	return
}

func (a *Application) LoadAllSlugs() (err error) {
	var files []string
	if files, err = bePath.ListFiles(a.Config.Paths.VarSlugs); err != nil {
		return
	}
	a.Slugs = make(map[string]*Slug)
	for _, file := range files {
		name := bePath.Base(file)
		if strings.HasPrefix(name, a.Name+"-") {
			if slug, ee := NewSlugFromZip(a, file); ee != nil {
				beIo.StderrF("error making slug from %v: %v\n", file, ee)
			} else {
				a.Slugs[slug.Name] = slug
			}
		}
	}
	return
}

func (a *Application) GetThisSlug() (slug *Slug) {
	if a.ThisSlug != "" {
		name := bePath.Base(a.ThisSlug)
		slug, _ = a.Slugs[name]
	}
	return
}

func (a *Application) GetNextSlug() (slug *Slug) {
	if a.NextSlug != "" {
		name := bePath.Base(a.NextSlug)
		slug, _ = a.Slugs[name]
	}
	return
}

func (a *Application) SshKeyPresent(input string) (present bool) {
	var err error
	if present, err = a.HasSshKey(input); err != nil {
		beIo.StderrF("error checking ssh key: %v - %v - %v\n", a.Name, input, err)
	}
	return
}

func (a *Application) HasSshKey(input string) (present bool, err error) {
	if _, _, _, id, ok := parseSshKey(input); ok {
		for _, key := range a.SshKeys {
			if _, _, _, keyId, valid := parseSshKey(key); valid {
				if id == keyId {
					err = nil
					present = true
					return
				}
			} else {
				err = fmt.Errorf("invalid app ssh-key present: %v", key)
			}
		}
	} else {
		err = fmt.Errorf("invalid app ssh-key input: %v", input)
	}
	return
}

func (a *Application) ApplySettings(envDir string) (err error) {
	beIo.StdoutF("applying settings to: %v\n", envDir)
	for _, k := range maps.SortedKeys(a.Settings) {
		key := strcase.ToScreamingSnake(k)
		value := fmt.Sprintf("%v", a.Settings[k])
		if err = os.WriteFile(envDir+"/"+key, []byte(value), 0660); err != nil {
			return
		}
		beIo.StdoutF("wrote: %v=\"%v\"\n", key, value)
	}
	return
}

func (a *Application) OsEnviron() (environment []string) {
	for _, k := range maps.SortedKeys(a.Settings) {
		key := strcase.ToScreamingSnake(k)
		environment = append(environment, fmt.Sprintf("%v=%v", key, a.Settings[k]))
	}
	return
}