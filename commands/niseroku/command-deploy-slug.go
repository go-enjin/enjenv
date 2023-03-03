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
	"time"

	"github.com/urfave/cli/v2"

	bePath "github.com/go-enjin/be/pkg/path"

	beIo "github.com/go-enjin/enjenv/pkg/io"
	pkgRun "github.com/go-enjin/enjenv/pkg/run"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

func (c *Command) actionDeploySlug(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	if ctx.Bool("verbose") {
		beIo.LogFile = ""
	}

	if ctx.NArg() == 0 {
		cli.ShowCommandHelpAndExit(ctx, "deploy-slug", 1)
	}

	var needsRestart []string

	hasErr := false
	argv := ctx.Args().Slice()
	for _, arg := range argv {
		if !bePath.IsFile(arg) {
			beIo.StderrF("error: not a file - %v\n", arg)
			continue
		}
		slugPath, _ := filepath.Abs(arg)
		slugName := filepath.Base(slugPath)
		if !RxSlugArchiveName.MatchString(slugPath) {
			beIo.StderrF("error: invalid slug file name - %v\n", slugName)
			continue
		}
		m := RxSlugArchiveName.FindAllStringSubmatch(slugPath, 1)
		slugAppName := m[0][1]
		if app, ok := c.config.Applications[slugAppName]; ok {
			needsRestart = append(needsRestart, app.Name)
			slugDestPath := c.config.Paths.VarSlugs + "/" + slugName
			if ee := os.Rename(slugPath, slugDestPath); ee != nil {
				beIo.StderrF("error moving slug: %v\n", ee)
				hasErr = true
				continue
			}
			_ = c.config.RunAsChown(slugDestPath)
			app.NextSlug = slugDestPath
			if app.ThisSlug == "" {
				beIo.StdoutF("# creating %v next slug: %v\n", app.Name, slugName)
			} else {
				beIo.StdoutF("# updating %v next slug: %v\n", app.Name, slugName)
			}
			if err = app.Save(true); err != nil {
				_ = c.config.RunAsChown(app.Source)
				beIo.StderrF("error saving %v config: %v\n", app.Name, err)
				hasErr = true
				continue
			}
		} else {
			hasErr = true
			beIo.StderrF("unknown slug app name: %v - %v\n", slugAppName, slugPath)
			continue
		}
	}

	if hasErr {
		err = fmt.Errorf("errors encountered, deployment halted\n")
		beIo.StderrF("%v\n", err)
		return
	}

	if err = common.DropPrivilegesTo(c.config.RunAs.User, c.config.RunAs.Group); err != nil {
		return
	}

	if c.config, err = LoadConfig(c.config.Source); err != nil {
		err = fmt.Errorf("error reloading niseroku configurations: %v\n", err)
		return
	}

	for _, appName := range needsRestart {
		app, _ := c.config.Applications[appName]
		if _, _, ee := pkgRun.EnjenvCmd("niseroku", "--config", c.config.Source, "app", "start", app.Name); ee != nil {
			beIo.StderrF("error restarting application: %v\n", app.Name)
		} else {
			beIo.StdoutF("# restarting application: %v\n", app.Name)
		}
	}

	time.Sleep(2500 * time.Millisecond) // necessary to allow background processes time
	beIo.StdoutF("# slug deployment completed, signaling services to reload\n")

	c.config.SignalReloadReverseProxy()
	c.config.SignalReloadGitRepository()

	return
}