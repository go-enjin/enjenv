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

package system

import (
	"fmt"
	"os"
	"runtime"

	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/cli/env"
	"github.com/go-enjin/be/pkg/cli/tar"
	"github.com/go-enjin/be/pkg/hash/sha"
	"github.com/go-enjin/be/pkg/net"
	bePath "github.com/go-enjin/be/pkg/path"
	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/io"
)

var _ System = (*CSystem)(nil)

type System interface {
	Command

	Self() (self System)
	GetKnownSums() (sums map[string]string, err error)
	ParseVersionString(ver string) (version string, err error)
	ParseFileName(path string) (version, osName, osArch string, err error)
	MakeDirs() (err error)
	InitAction(ctx *cli.Context) (err error)
	VersionAction(ctx *cli.Context) (err error)
	CleanAction(ctx *cli.Context) (err error)
	ExportPathVariable(export bool)
	ExportAction(ctx *cli.Context) (err error)
	UnExportPathVariable(export bool)
	UnExportAction(ctx *cli.Context) (err error)
	InitSystem(ctx *cli.Context) (err error)
	PostInitSystem(ctx *cli.Context) (err error)
	Clean(ctx *cli.Context) (err error)
	Export(ctx *cli.Context) (err error)
	ExportString(ctx *cli.Context) (content string, err error)
	UnExport(ctx *cli.Context) (err error)
	UnExportString(ctx *cli.Context) (content string, err error)
	GetInstalledVersion() (version string, err error)
	GetDefaultVersion() (version string)
	IncludeCommands(app *cli.App) (commands []*cli.Command)
}

type CSystem struct {
	CCommand

	Url string

	Version   string
	Root      string
	TarGz     string
	TarGzPath string
	TarGzUrl  string
}

func (s *CSystem) Init(this interface{}) {
	s.CCommand.Init(this)
}

func (s *CSystem) Self() (self System) {
	if v, ok := s._this.(System); ok {
		self = v
		return
	}
	self = s
	return
}

func (s *CSystem) GetKnownSums() (sums map[string]string, err error) {
	sums = make(map[string]string)
	return
}

func (s *CSystem) ParseVersionString(ver string) (version string, err error) {
	err = fmt.Errorf("not implemented")
	return
}

func (s *CSystem) ParseFileName(path string) (version, osName, osArch string, err error) {
	err = fmt.Errorf("not implemented")
	return
}

func (s *CSystem) GetInstalledVersion() (version string, err error) {
	err = fmt.Errorf("not implemented")
	return
}

func (s *CSystem) GetDefaultVersion() (version string) {
	return "not implemented"
}

func (s *CSystem) MakeDirs() (err error) {
	for k, p := range s.Ctx.AsMapStrings() {
		pp := basepath.MakeEnjenvPath(p)
		if !bePath.Exists(pp) {
			if err = bePath.Mkdir(pp); err != nil {
				return
			}
			io.StdoutF("# making %v path: %v\n", k, pp)
		}
	}
	return
}

func (s *CSystem) InitAction(ctx *cli.Context) (err error) {
	io.NotifyF("%v init started", s.Self().Name())
	if err = s.Self().Prepare(ctx); err != nil {
		return
	}
	err = s.Self().InitSystem(ctx)
	return
}

func (s *CSystem) VersionAction(ctx *cli.Context) (err error) {
	if err = s.Self().Prepare(ctx); err != nil {
		return
	}
	var ver string
	if ver, err = s.Self().GetInstalledVersion(); err != nil {
		return
	}
	io.StdoutF("%v\n", ver)
	return
}

func (s *CSystem) CleanAction(ctx *cli.Context) (err error) {
	if err = s.Self().Prepare(ctx); err != nil {
		return
	}
	if err = s.Self().Clean(ctx); err != nil {
		return
	}
	return
}

func (s *CSystem) ExportPathVariable(export bool) {
	binDir := basepath.MakeEnjenvPath(s.Root, "bin")
	_ = env.SetPathRemoved(binDir)
	cleaned := env.SetPathPrefixed(binDir)
	if export {
		io.StdoutF("export PATH=\"%v\"\n", cleaned)
	}
	return
}

func (s *CSystem) ExportAction(ctx *cli.Context) (err error) {
	if err = s.Self().Prepare(ctx); err != nil {
		return
	}
	if err = s.Self().Export(ctx); err != nil {
		return
	}
	s.Self().ExportPathVariable(true)
	return
}

func (s *CSystem) UnExportPathVariable(export bool) {
	binDir := basepath.MakeEnjenvPath(s.Root, "bin")
	cleaned := env.SetPathRemoved(binDir)
	if export {
		io.StdoutF("export PATH=\"%v\"\n", cleaned)
	}
	return
}

func (s *CSystem) UnExportAction(ctx *cli.Context) (err error) {
	if err = s.Self().Prepare(ctx); err != nil {
		return
	}
	if err = s.Self().UnExport(ctx); err != nil {
		return
	}
	s.Self().UnExportPathVariable(true)
	return
}

func (s *CSystem) PostInitSystem(ctx *cli.Context) (err error) {
	return
}

func (s *CSystem) InitSystem(ctx *cli.Context) (err error) {
	var useFile string
	name := s.Self().Name()
	if ver := ctx.String(name); ver != "" {
		if s.Version, err = s.Self().ParseVersionString(ver); err != nil {
			var osName, osArch string
			if s.Version, osName, osArch, err = s.Self().ParseFileName(ver); err != nil {
				err = fmt.Errorf("--%v argument not a version string or file, error: %v", name, err)
				return
			} else {
				if osName != runtime.GOOS {
					err = fmt.Errorf("--%v archive is for the wrong os name, expecting: %v", name, runtime.GOOS)
					return
				}
				if osArch != runtime.GOARCH && osArch != "x64" {
					err = fmt.Errorf("--%v archive is for the wrong os arch, expecting: x64 or %v", name, runtime.GOARCH)
					return
				}
				useFile = ver
				io.NotifyF("%v version (from file): %v", name, s.Version)
			}
		} else {
			io.NotifyF("%v version (requested): %v", name, s.Version)
		}
	} else {
		io.NotifyF("%v version (default): %v", name, s.Version)
	}

	if argv := ctx.Args().Slice(); len(argv) >= 1 {
		basepath.EnjenvPath, _ = bePath.Abs(argv[0])
		basepath.EnjenvPath += "/" + basepath.EnjenvDirName
	}

	force := ctx.Bool("force")
	sDir := basepath.MakeEnjenvPath(s.TagName)
	sRootPath := basepath.MakeEnjenvPath(s.Root)

	if bePath.IsDir(sDir) {
		if !force {
			err = fmt.Errorf("%v directory exists, use --force to overwrite", s.Name())
			return
		}
		if bePath.IsDir(sRootPath) {
			bePath.ChmodAll(sRootPath)
			if err = os.RemoveAll(sRootPath); err != nil {
				return
			}
		}
	} else if err = os.MkdirAll(sDir, 0770); err != nil {
		return
	}

	// the version may have changed
	if err = s.Self().Prepare(ctx); err != nil {
		return
	}

	var sums map[string]string
	if sums, err = s.Self().GetKnownSums(); err != nil {
		return
	}

	if _, ok := sums[s.TarGz]; !ok {
		err = fmt.Errorf("%v shasum not found", s.TarGz)
		return
	}

	if useFile != "" && bePath.IsFile(useFile) {
		if _, err = bePath.CopyFile(useFile, s.TarGzPath); err != nil {
			return
		}
	}

	if bePath.IsFile(s.TarGzPath) {
		io.StdoutF("# using archive: %v\n", s.TarGzPath)
	} else {
		io.StdoutF("# downloading: %v\n", s.TarGzUrl)
		if err = net.Download(s.TarGzUrl, s.TarGzPath); err != nil {
			return
		}
	}

	io.StdoutF("# checking shasum: %v\n", s.TarGzPath)
	if err = sha.VerifyFile64(sums[s.TarGz], s.TarGzPath); err != nil {
		return
	}

	if bePath.IsDir(sRootPath) {
		io.StdoutF("# found installation: %v\n", sRootPath)
	} else {
		sRootPath = basepath.MakeEnjenvPath(s.TagName)
		io.StdoutF("# extracting to: %v\n", sRootPath)
		if _, err = tar.UnTarGz(s.TarGzPath, sRootPath); err != nil {
			return
		}
	}

	tmpPath := basepath.MakeEnjenvPath(TmpDirName)
	if !bePath.IsDir(tmpPath) {
		if err = bePath.Mkdir(tmpPath); err != nil {
			return
		}
	}

	if err = s.Self().MakeDirs(); err != nil {
		return
	}

	if err = s.Self().PostInitSystem(ctx); err != nil {
		return
	}

	io.NotifyF("%v init complete", s.Self().Name())
	return
}

func (s *CSystem) Clean(ctx *cli.Context) (err error) {
	path := basepath.MakeEnjenvPath(s.TagName)
	if bePath.IsDir(path) {
		if !ctx.Bool("force") {
			err = fmt.Errorf("not cleaning local %v environment: %v (missing --force)", s.Self().Name(), path)
			return
		}
		io.NotifyF("cleaning local %v environment: %v", s.Self().Name(), path)
		bePath.ChmodAll(path)
		err = os.RemoveAll(path)
		return
	}
	io.NotifyF("nothing to clean for %v", s.Self().Name())
	return
}

func (s *CSystem) ExportString(ctx *cli.Context) (content string, err error) {
	err = fmt.Errorf("not implemented")
	return
}

func (s *CSystem) Export(ctx *cli.Context) (err error) {
	return
}

func (s *CSystem) UnExportString(ctx *cli.Context) (content string, err error) {
	err = fmt.Errorf("not implemented")
	return
}

func (s *CSystem) UnExport(ctx *cli.Context) (err error) {
	return
}

func (s *CSystem) IncludeCommands(app *cli.App) (commands []*cli.Command) {
	return
}