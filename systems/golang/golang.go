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

package golang

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/cli/env"
	"github.com/go-enjin/be/pkg/cli/git"
	"github.com/go-enjin/be/pkg/cli/run"
	"github.com/go-enjin/be/pkg/net"
	bePath "github.com/go-enjin/be/pkg/path"

	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/io"
	pkgRun "github.com/go-enjin/enjenv/pkg/run"
	"github.com/go-enjin/enjenv/pkg/system"
)

var (
	Tag               = "go"
	Name              = "golang"
	BinName           = bePath.Base(bePath.Pwd())
	Summary           = "Custom Go-Enjin Service"
	Version           = "v0.0.0"
	BeEnvPrefix       = strcase.ToScreamingSnake(BinName)
	GoEnvFileName     = "env"
	GoTmpDirName      = "tmp"
	GoCacheDirName    = "cache"
	GoModCacheDirName = "modcache"
)

var (
	DefaultVersion = "1.17.7"
)

var (
	rxVersion = regexp.MustCompile(`^(?:go)??(\d+?\.\d+?\.\d+?)$`)
	rxTarName = regexp.MustCompile(`go(\d+?\.\d+?\.\d+?)\.([a-z0-9]+?)-([a-z0-9]+?)\.tar\.gz`)
	rxTarGzs  = regexp.MustCompile(`(?ms)<tr(?:[^>]+?|)>\s*<td class="filename">\s*<a.+?href="/dl/([^"]+?\.tar\.gz)"[^<]*</a></td>\s*<td>[^<]*</td>\s*<td>[^<]*</td>\s*<td>[^<]*</td>\s*<td>[^<]*</td>\s*<td>\s*<tt>\s*(.+?)\s*</tt>\s*</td>\s*</tr>`)
)

func init() {
	Tag = env.Get("ENJENV_GOLANG_TAG", Tag)
	tag := strings.ToUpper(Tag)
	Name = env.Get("ENJENV_"+tag+"_NAME", Name)
	BinName = env.Get("ENJENV_"+tag+"_BIN_NAME", BinName)
	Summary = env.Get("ENJENV_"+tag+"_SUMMARY", Summary)
	Version = env.Get("ENJENV_"+tag+"_VERSION", Version)
	BeEnvPrefix = env.Get("ENJENV_"+tag+"_ENV_PREFIX", BeEnvPrefix)
	GoTmpDirName = env.Get("ENJENV_"+tag+"_TMP_DIR_NAME", GoTmpDirName)
	GoCacheDirName = env.Get("ENJENV_"+tag+"_CACHE_DIR_NAME", GoCacheDirName)
	GoModCacheDirName = env.Get("ENJENV_"+tag+"_MOD_CACHE_DIR_NAME", GoModCacheDirName)
	DefaultVersion = env.Get("ENJENV_DEFAULT_"+tag+"_VERSION", DefaultVersion)
}

type System struct {
	system.CSystem
}

func New() (s *System) {
	s = new(System)
	s.Init(s)
	return
}

func (s *System) Init(this interface{}) {
	s.CSystem.Init(this)
	s.TagName = Name
	s.Url = "https://go.dev/dl"
	s.Version = DefaultVersion
	s.Root = s.TagName + "/" + Tag
	s.CSystem.Root = s.Root
	s.CSystem.TagName = s.TagName
	return
}

func (s *System) GetDefaultVersion() (version string) {
	return DefaultVersion
}

func (s *System) installNancy() (err error) {
	tmpdir := s.Ctx.String("GOTMPDIR", env.Get("TMPDIR", "./tmp"))
	if !bePath.IsDir(tmpdir) {
		if err = bePath.Mkdir(tmpdir); err != nil {
			return
		}
	}
	var cwd string
	if cwd, err = os.Getwd(); err != nil {
		return
	}
	if err = os.Chdir(tmpdir); err != nil {
		return
	}
	defer os.Chdir(cwd)
	if !bePath.IsDir("nancy") {
		if _, err = git.Exe("clone", "https://github.com/sonatype-nexus-community/nancy.git"); err != nil {
			return
		}
	}
	if err = os.Chdir("nancy"); err != nil {
		return
	}
	if _, err = s.GoBin("build", "-v"); err != nil {
		return
	}
	dst := basepath.MakeEnjenvPath(s.Root, "bin", "nancy")
	if _, err = bePath.CopyFile("./nancy", dst); err != nil {
		return
	}
	if err = os.Chmod(dst, 0770); err != nil {
		return
	}
	return
}

func (s *System) nancyPresent() (ok bool) {
	nancy := basepath.MakeEnjenvPath(s.Root, "bin", "nancy")
	ok = bePath.IsFile(nancy)
	return
}

func (s *System) NancyBin(argv ...string) (status int, err error) {
	bin := basepath.MakeEnjenvPath(s.Root, "bin", "nancy")
	return run.Exe(bin, argv...)
}

func (s *System) Prepare(ctx *cli.Context) (err error) {
	_ = io.SetupCustomIndent(ctx)
	if err = s.CSystem.Prepare(ctx); err != nil {
		return
	}
	if err = io.SetupSlackIfPresent(ctx); err != nil {
		return
	}
	s.Root = s.TagName + "/" + Tag
	s.CSystem.Root = s.Root

	s.TarGz = fmt.Sprintf("go%v.%v-%v.tar.gz", s.Version, runtime.GOOS, runtime.GOARCH)
	s.TarGzPath = basepath.MakeEnjenvPath(s.TagName, s.TarGz)
	s.TarGzUrl = fmt.Sprintf("%v/%v", s.Url, s.TarGz)

	s.Ctx.Set("GOROOT", s.Root)
	s.Ctx.Set("GOENV", s.TagName+"/"+GoEnvFileName)
	s.Ctx.Set("GOTMPDIR", s.TagName+"/"+GoTmpDirName)
	s.Ctx.Set("GOCACHE", s.TagName+"/"+GoCacheDirName)
	s.Ctx.Set("GOMODCACHE", s.TagName+"/"+GoModCacheDirName)
	for k, v := range s.Ctx.AsMapStrings() {
		env.Set(k, basepath.MakeEnjenvPath(v))
	}
	env.Set("GOFLAGS", s.goFlagsWithModCacheRw())

	pkgRun.AddPathToEnv(basepath.MakeEnjenvPath(s.Root, "go", "bin"))

	return
}

func (s *System) goFlagsWithModCacheRw() (goFlags string) {
	goFlags = env.Get("GOFLAGS", "")
	if !strings.Contains(goFlags, "-modcacherw") {
		if goFlags != "" {
			goFlags += " "
		}
		goFlags += "-modcacherw"
	}
	return
}

func (s *System) ExportString(ctx *cli.Context) (content string, err error) {
	path := basepath.MakeEnjenvPath(s.TagName)
	if bePath.IsDir(path) {
		content += fmt.Sprintf("export %v_VERSION=\"%v\"\n", strings.ToUpper(Tag), s.Version)
		goFlags := s.goFlagsWithModCacheRw()
		content += fmt.Sprintf("export GOFLAGS=\"%v\"\n", goFlags)
		env.Set("GOFLAGS", goFlags)
		for k, v := range s.Ctx.AsMapStrings() {
			value := basepath.MakeEnjenvPath(v)
			content += fmt.Sprintf("export %v=\"%v\"\n", k, value)
			env.Set(k, value)
		}
	}
	return
}

func (s *System) Export(ctx *cli.Context) (err error) {
	var content string
	if content, err = s.ExportString(ctx); err == nil {
		io.StdoutF(content)
	}
	return
}

func (s *System) UnExportString(ctx *cli.Context) (content string, err error) {
	path := basepath.MakeEnjenvPath(s.TagName)
	if bePath.IsDir(path) {
		content += fmt.Sprintf("unset %v_VERSION;\n", strings.ToUpper(Tag))
		for k, _ := range s.Ctx.AsMapStrings() {
			env.Set(k, "")
			content += fmt.Sprintf("unset %v;\n", k)
		}
	}
	return
}

func (s *System) UnExport(ctx *cli.Context) (err error) {
	var content string
	if content, err = s.UnExportString(ctx); err == nil {
		io.StdoutF(content)
	}
	return
}

func (s *System) GetInstalledVersion() (version string, err error) {
	path := basepath.MakeEnjenvPath(s.Root)
	if bePath.IsDir(path) {
		goVerFile := fmt.Sprintf("%v/VERSION", path)
		if bePath.IsFile(goVerFile) {
			var data []byte
			if data, err = os.ReadFile(goVerFile); err == nil {
				content := string(data)
				if len(content) > 2 {
					version = content[2:]
					return
				} else {
					err = fmt.Errorf("error parsing VERSION content: %v\n", content)
				}
			} else {
				err = fmt.Errorf("error reading VERSION: %v\n", err)
			}
		} else {
			err = fmt.Errorf("VERSION not found: %v\n", goVerFile)
		}
	} else {
		err = fmt.Errorf("GOROOT not found: %v\n", path)
	}
	return
}

func (s *System) ParseVersionString(ver string) (version string, err error) {
	if !rxVersion.MatchString(ver) {
		err = fmt.Errorf("not a version string")
		return
	}
	m := rxVersion.FindAllStringSubmatch(ver, 1)
	version = m[0][1]
	return
}

func (s *System) ParseFileName(path string) (version, osName, osArch string, err error) {
	if !bePath.IsFile(path) {
		err = fmt.Errorf("file not found")
		return
	}
	if !rxTarName.MatchString(path) {
		err = fmt.Errorf("invalid go archive name, see: %v", s.Url)
		return
	}
	m := rxTarName.FindAllStringSubmatch(path, 1)
	version = m[0][1]
	osName = m[0][2]
	osArch = m[0][3]
	return
}

func (s *System) GetKnownSums() (sums map[string]string, err error) {
	var content string
	if content, err = net.Get(s.Url); err != nil {
		return
	}
	if !rxTarGzs.MatchString(content) {
		err = fmt.Errorf("error parising %v content", s.Url)
		return
	}
	matches := rxTarGzs.FindAllStringSubmatch(content, -1)
	sums = make(map[string]string)
	for _, match := range matches {
		sums[match[1]] = match[2]
	}
	return
}

func (s *System) MakeDirs() (err error) {
	for k, p := range s.Ctx.AsMapStrings() {
		pp := basepath.MakeEnjenvPath(p)
		switch k {
		case "GOENV":
			if !bePath.Exists(pp) {
				if err = os.WriteFile(pp, []byte(""), 0660); err != nil {
					return
				}
				io.StdoutF("# making %v file: %v\n", k, pp)
			}
		}
	}
	err = s.CSystem.MakeDirs()
	return
}

func (s *System) GoBin(name string, argv ...string) (status int, err error) {
	bin := basepath.MakeEnjenvPath(s.Root, "bin", "go")
	argv = append([]string{name}, argv...)
	return run.Exe(bin, argv...)
}