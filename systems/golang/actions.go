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
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/cli/env"
	"github.com/go-enjin/be/pkg/cli/git"
	"github.com/go-enjin/be/pkg/path"
	"github.com/go-enjin/be/pkg/slug"

	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/io"
)

func (s *System) ActionGoBuild(ctx *cli.Context) (err error) {
	if err = s.Prepare(ctx); err != nil {
		return
	}

	var appName, summary, version, release, envPrefix string
	if appName = ctx.String("be-app-name"); appName == "" {
		appName = BinName
	}
	if summary = ctx.String("be-summary"); summary == "" {
		summary = Summary
	}
	if version = ctx.String("be-version"); version == "" {
		version = Version
	}
	if release = ctx.String("be-release"); release == "" {
		release = git.MakeReleaseVersion()
	}
	if envPrefix = ctx.String("be-env-prefix"); envPrefix == "" {
		envPrefix = BeEnvPrefix
	}

	var gcFlags, ldFlags, asmFlags []string

	ldFlags = append(
		ldFlags,
		fmt.Sprintf("-X 'github.com/go-enjin/be/pkg/globals.AppName=%v'", appName),
		fmt.Sprintf("-X 'github.com/go-enjin/be/pkg/globals.Summary=%v'", summary),
		fmt.Sprintf("-X 'github.com/go-enjin/be/pkg/globals.Version=%v'", version),
		fmt.Sprintf("-X 'github.com/go-enjin/be/pkg/globals.Release=%v'", release),
		fmt.Sprintf("-X 'github.com/go-enjin/be/pkg/globals.EnvPrefix=%v'", envPrefix),
	)

	if ctx.Bool("finalize") && slug.SlugfilePresent() {
		ignoreName := ""
		if output := ctx.String("be-bin-name"); output != "" {
			ignoreName = output
		} else if ctx.IsSet("be-app-name") {
			ignoreName = appName
		} else {
			ignoreName = path.Base(path.Pwd())
		}
		var slugMap slug.ShaMap
		if slugMap, err = slug.BuildSlugMapIgnoring(ignoreName); err != nil {
			err = fmt.Errorf("error building slug map: %v", err)
			return
		}
		var slugIntegrity, sumsIntegrity string
		if slugIntegrity, err = slugMap.SlugIntegrity(); err != nil {
			err = fmt.Errorf("error checking slug integrity: %v", err)
			return
		}
		ldFlags = append(
			ldFlags,
			fmt.Sprintf("-X 'github.com/go-enjin/be/pkg/globals.SlugIntegrity=%v'", slugIntegrity),
			fmt.Sprintf("-X 'github.com/go-enjin/be/pkg/globals.SumsIntegrity=%v'", sumsIntegrity),
		)
		io.StdoutF("# finalizing slug integrity: %v\n", slugIntegrity)
	}

	if moreGcFlags := ctx.String("gcflags"); moreGcFlags != "" {
		gcFlags = append(gcFlags, moreGcFlags)
	}
	if moreLdFlags := ctx.String("ldflags"); moreLdFlags != "" {
		ldFlags = append(ldFlags, moreLdFlags)
	}

	var extra []string

	if ctx.Bool("optimize") {
		io.StdoutF("# optimizing for release build\n")
		_ = os.Setenv("GOROOT_FINAL", "go")

		var trimPaths []string
		trimPaths = append(trimPaths, basepath.EnjenvPath)

		if goPath := env.Get("GOPATH", ""); goPath != "" {
			trimPaths = append(trimPaths, goPath)
		}
		if thisPath := path.Pwd(); thisPath != "" {
			trimPaths = append(trimPaths, thisPath)
		}

		wd := strings.Join(trimPaths, ";")
		asmFlags = append(asmFlags, fmt.Sprintf("-trimpath='%v'", wd))
		gcFlags = append(gcFlags, fmt.Sprintf("-trimpath='%v'", wd))

		ldFlags = append(ldFlags, "-buildid=''", "-w", "-s")
		extra = append([]string{"-trimpath"}, extra...)
	} else {
		io.StdoutF("# optimizing for debug build\n")
		gcFlags = append(gcFlags, `-N -l`)
	}

	if len(asmFlags) > 0 {
		args := strings.Join(asmFlags, " ")
		extra = append([]string{"-asmflags", args}, extra...)
	}

	if len(gcFlags) > 0 {
		args := strings.Join(gcFlags, " ")
		extra = append([]string{"-gcflags", args}, extra...)
	}

	if len(ldFlags) > 0 {
		args := strings.Join(ldFlags, " ")
		extra = append([]string{"-ldflags", args}, extra...)
	}

	if output := ctx.String("be-bin-name"); output != "" {
		extra = append(extra, "-o", output)
	} else if ctx.IsSet("be-app-name") {
		extra = append(extra, "-o", appName)
	}

	extra = append(extra, "-modcacherw")

	if ctx.Bool("verbose") {
		extra = append(extra, "-v")
	}

	argv := ctx.Args().Slice()
	argv = append(extra, argv...)
	io.NotifyF("go build", "running go build for %v", appName)
	io.StdoutF("go build %v\n", argv)
	_, err = s.GoBin("build", argv...)
	return
}

func (s *System) ActionGoModLocal(ctx *cli.Context) (err error) {
	if err = s.Prepare(ctx); err != nil {
		return
	}
	if !path.IsFile("go.mod") {
		err = fmt.Errorf("go.mod is not present")
		return
	}
	argv := ctx.Args().Slice()
	argc := len(argv)
	if argc > 2 {
		err = fmt.Errorf("too many arguments given, see: --help")
		return
	}
	if argc == 2 {
		_, err = s.GoBin("mod", "edit", fmt.Sprintf("-replace=%v=%v", argv[0], argv[1]))
		return
	}
	var beLocalPath string
	if argc == 1 {
		if path.IsDir(argv[0]) {
			beLocalPath = argv[0]
		} else {
			beLocalPath = env.Get("BE_LOCAL_PATH", "")
		}
	} else {
		beLocalPath = env.Get("BE_LOCAL_PATH", "")
	}
	if beLocalPath != "" {
		_, err = s.GoBin("mod", "edit", fmt.Sprintf(
			"-replace=%v=%v",
			"github.com/go-enjin/be",
			beLocalPath,
		))
		return
	}
	err = fmt.Errorf("no arguments given and BE_LOCAL_PATH not set; see: --help")
	return
}

func (s *System) ActionGoModUnLocal(ctx *cli.Context) (err error) {
	if err = s.Prepare(ctx); err != nil {
		return
	}
	if !path.IsFile("go.mod") {
		err = fmt.Errorf("go.mod is not present")
		return
	}
	argv := ctx.Args().Slice()
	if len(argv) == 0 {
		_, err = s.GoBin("mod", "edit", "-dropreplace=github.com/go-enjin/be")
		return
	}
	for _, arg := range argv {
		_, err = s.GoBin("mod", "edit", fmt.Sprintf("-dropreplace=%v", arg))
		if err != nil {
			return
		}
	}
	return
}