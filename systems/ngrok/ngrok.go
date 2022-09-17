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

package ngrok

import (
	"fmt"
	"os"
	"regexp"

	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/cli/env"
	"github.com/go-enjin/be/pkg/net"
	bePath "github.com/go-enjin/be/pkg/path"
	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/system"
)

var (
	Tag  = "ngrok"
	Name = "ngrok"

	DownloadUrl     = "https://ngrok.com/download"
	rxDownloadLinks = regexp.MustCompile("(?ms)dl-link'\\)\\.attr\\('href',\\s*`(https://bin.equinox.io/.+?)`")
	rxTarName       = regexp.MustCompile(`ngrok-v3-stable-([a-z0-9]+)-([a-z0-9]+).([a-z][.a-z0-9]+)`)
)

type System struct {
	system.CSystem

	ReleaseUrl string
}

func New() (s *System) {
	s = new(System)
	s.Init(s)
	return
}

func (s *System) Init(this interface{}) {
	s.CSystem.Init(this)
	s.TagName = Name
	s.Root = s.TagName + "/installed"
	s.CSystem.Root = s.Root
	s.CSystem.TagName = s.TagName
	return
}

func (s *System) GetInstalledVersion() (version string, err error) {
	path := basepath.MakeEnjenvPath(s.Root)
	if bePath.IsDir(path) {
		if bePath.IsFile(path + "/bin/ngrok") {
			version = "latest"
			return
		}
		err = fmt.Errorf("ngrok not found: %v/bin/ngrok\n", path)
		return
	}
	err = fmt.Errorf("enjenv path not found: %v\n", path)
	return
}

func (s *System) GetDefaultVersion() (version string) {
	return "latest"
}

func (s *System) Prepare(ctx *cli.Context) (err error) {
	if err = s.CSystem.Prepare(ctx); err != nil {
		return
	}

	goos, goarch, goextn := RuntimeOS(), RuntimeArch(), RuntimeExtn()

	s.TarUnzip = goos == "darwin"

	if err = s.getReleaseUrl(goos, goarch, goextn); err != nil {
		return
	}

	s.TarGz = fmt.Sprintf("ngrok-v3-stable-%v-%v.%v", goos, goarch, goextn)
	s.TarGzPath = basepath.MakeEnjenvPath(s.TagName, s.TarGz)
	s.TarGzUrl = s.Url

	for k, v := range s.Ctx.AsMapStrings() {
		env.Set(k, basepath.MakeEnjenvPath(v))
	}
	return
}

func (s *System) getReleaseUrl(goos, goarch, goextn string) (err error) {
	var content string
	if content, err = net.Get(DownloadUrl); err != nil {
		return fmt.Errorf("error getting %v: %v", DownloadUrl, err)
	}

	if m := rxDownloadLinks.FindAllStringSubmatch(content, -1); len(m) > 0 {
		for idx := range m {
			if mm := rxTarName.FindAllStringSubmatch(m[idx][1], -1); len(mm) > 0 {
				dos, darch, dextn := mm[0][1], mm[0][2], mm[0][3]
				if dos == goos {
					if darch == goarch {
						if goextn == dextn {
							s.Url = m[idx][1]
							break
						}
					}
				}
			}
		}
	}

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
		err = fmt.Errorf("invalid ngrok archive name, see: https://ngrok.com/download")
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

func (s *System) PostInitSystem(ctx *cli.Context) (err error) {
	path := basepath.MakeEnjenvPath(s.TagName)
	if bePath.IsDir(path) {
		if bePath.IsFile(path + "/ngrok") {
			if err = bePath.Mkdir(path + "/installed/bin"); err != nil {
				err = fmt.Errorf("error making ngrok bin path: %v", err)
				return
			}
			if err = os.Rename(path+"/ngrok", path+"/installed/bin/ngrok"); err != nil {
				err = fmt.Errorf("error moving ngrok to bin path: %v", err)
				return
			}
		} else {
			err = fmt.Errorf("ngrok not found: %v/ngrok", path)
		}
	} else {
		err = fmt.Errorf("ngrok not installed: %v", path)
	}
	return
}

func (s *System) IncludeCommands(app *cli.App) (commands []*cli.Command) {
	appNamePrefix := app.Name + " " + Name
	commands = []*cli.Command{}
	if _, err := s.GetInstalledVersion(); err == nil {
		commands = append(
			commands,
			&cli.Command{
				Name:      "http",
				Category:  system.SystemCategory,
				Usage:     "start an HTTP tunnel",
				UsageText: appNamePrefix + " http -- [ngrok options]",
				Action: func(ctx *cli.Context) (err error) {

					return
				},
			},
		)
	}
	return
}
