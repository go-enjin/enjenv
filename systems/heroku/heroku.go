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

package heroku

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/cli/env"
	"github.com/go-enjin/be/pkg/cli/run"
	"github.com/go-enjin/be/pkg/net"
	bePath "github.com/go-enjin/be/pkg/path"
	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/system"
)

var (
	Tag  = "heroku"
	Name = "heroku"
)

var (
	rxTarName    = regexp.MustCompile(`heroku-(darwin|linux|windows)-(x86|arm|x64)\.tar\.gz`)
	rxVerTarName = regexp.MustCompile(`heroku-(v\d+?\.\d+?\.\d+?)-(darwin|linux|windows)-(x86|arm|x64)\.tar\.gz`)
	rxPackageVer = regexp.MustCompile(`(?ms)"version"\s*:\s*"([^"]+?)"`)
)

func init() {
	Tag = env.Get("ENJENV_HEROKU_TAG", Tag)
	tag := strings.ToUpper(Tag)
	Name = env.Get("ENJENV_"+tag+"_NAME", Name)
}

type ReleaseInfo struct {
	Version  string `json:"version"`
	Channel  string `json:"channel"`
	BaseDir  string `json:"baseDir"`
	Gz       string `json:"gz"`
	Xz       string `json:"xz"`
	Sha256Gz string `json:"sha256gz"`
	Sha256Xz string `json:"sha256xz"`
	Node     struct {
		Compatible  string `json:"compatible"`
		Recommended string `json:"recommended"`
	} `json:"node"`
}

type System struct {
	system.CSystem

	ReleaseUrl  string
	ReleaseInfo ReleaseInfo
}

func New() (s *System) {
	s = new(System)
	s.Init(s)
	return
}

func (s *System) Init(this interface{}) {
	s.CSystem.Init(this)
	s.TagName = Name
	s.Url = "https://cli-assets.heroku.com"
	s.ReleaseUrl = fmt.Sprintf("%v/%v-%v", s.Url, RuntimeOS(), RuntimeArch())
	s.Root = s.TagName + "/" + Tag
	s.CSystem.Root = s.Root
	s.CSystem.TagName = s.TagName
	return
}

func (s *System) GetDefaultVersion() (version string) {
	return "latest"
}

func (s *System) Prepare(ctx *cli.Context) (err error) {
	if !RuntimeSupported() {
		err = fmt.Errorf("%v is not supported on %v/%v", Tag, runtime.GOOS, runtime.GOARCH)
		return
	}

	if err = s.GetReleaseInfo(); err != nil {
		return
	}

	if err = s.CSystem.Prepare(ctx); err != nil {
		return
	}

	s.Root = s.TagName + "/" + Tag
	s.CSystem.Root = s.Root

	s.TarGz = fmt.Sprintf("heroku-%v-%v.tar.gz", RuntimeOS(), RuntimeArch())
	s.TarGzPath = basepath.MakeEnjenvPath(s.TagName, s.TarGz)
	s.TarGzUrl = fmt.Sprintf("%v/%v", s.Url, s.TarGz)

	for k, v := range s.Ctx.AsMapStrings() {
		env.Set(k, basepath.MakeEnjenvPath(v))
	}
	return
}

func (s *System) HerokuExe(name string, argv ...string) (status int, err error) {
	bin := basepath.MakeEnjenvPath(s.Root, "bin", "heroku")
	argv = append([]string{name}, argv...)
	return run.Exe(bin, argv...)
}

func (s *System) HerokuCmd(name string, argv ...string) (o, e string, status int, err error) {
	bin := basepath.MakeEnjenvPath(s.Root, "bin", "heroku")
	argv = append([]string{name}, argv...)
	return run.Cmd(bin, argv...)
}

func (s *System) GetInstalledVersion() (version string, err error) {
	path := basepath.MakeEnjenvPath(s.Root)
	if bePath.IsDir(path) {
		packageJson := fmt.Sprintf("%v/package.json", path)
		if bePath.IsFile(packageJson) {
			var data []byte
			if data, err = os.ReadFile(packageJson); err == nil {
				content := string(data)
				if rxPackageVer.MatchString(content) {
					m := rxPackageVer.FindAllStringSubmatch(content, 1)
					if len(m[0]) == 2 {
						version = m[0][1]
						return
					}
				}
				err = fmt.Errorf("error parsing package.json content: %v\n", content)
				return
			}
			err = fmt.Errorf("error reading VERSION: %v\n", err)
			return
		}
		err = fmt.Errorf("VERSION not found: %v\n", packageJson)
		return
	}
	err = fmt.Errorf("enjenv path not found: %v\n", path)
	return
}

func (s *System) ParseVersionString(ver string) (version string, err error) {
	if ver != "latest" {
		err = fmt.Errorf("only latest version supported")
		return
	}
	version = "latest"
	return
}

func (s *System) ParseFileName(path string) (version, osName, osArch string, err error) {
	if !bePath.IsFile(path) {
		err = fmt.Errorf("file not found")
		return
	}
	if !rxTarName.MatchString(path) {
		if !rxVerTarName.MatchString(path) {
			err = fmt.Errorf("invalid heroku archive name, see: https://devcenter.heroku.com/articles/heroku-cli")
			return
		}
		m := rxVerTarName.FindAllStringSubmatch(path, 1)
		version = m[0][1]
		osName = m[0][2]
		osArch = m[0][3]
		return
	}
	m := rxTarName.FindAllStringSubmatch(path, 1)
	version = "latest"
	osName = m[0][1]
	osArch = m[0][2]
	return
}

func (s *System) IsSupported(osName, archName string) (ok bool) {
	if CheckOS(osName) != "" {
		return CheckArch(archName) != ""
	}
	return
}

func (s *System) GetReleaseInfo() (err error) {
	var content string
	if content, err = net.Get(s.ReleaseUrl); err != nil {
		return
	}
	var data ReleaseInfo
	if err = json.Unmarshal([]byte(content), &data); err != nil {
		return
	}
	s.ReleaseInfo = data
	return
}

func (s *System) GetKnownSums() (sums map[string]string, err error) {
	versioned := filepath.Base(s.ReleaseInfo.Gz)
	sums = map[string]string{
		versioned: s.ReleaseInfo.Sha256Gz,
	}
	if rxVerTarName.MatchString(versioned) {
		m := rxVerTarName.FindAllStringSubmatch(versioned, 1)
		name := fmt.Sprintf("heroku-%v-%v.tar.gz", m[0][2], m[0][3])
		sums[name] = s.ReleaseInfo.Sha256Gz
	}
	io.StdoutF("known sums: %v\n", sums)
	return
}