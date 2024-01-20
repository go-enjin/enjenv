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
	"syscall"

	"github.com/urfave/cli/v2"

	"github.com/go-corelibs/path"

	beIo "github.com/go-enjin/enjenv/pkg/io"
	pkgRun "github.com/go-enjin/enjenv/pkg/run"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

func makeCommandAppRename(c *Command, app *cli.App) (cmd *cli.Command) {
	cmd = &cli.Command{
		Name:      "rename",
		Usage:     "rename an application",
		UsageText: app.Name + " niseroku app rename <old> <new>",
		Action:    c.actionAppRename,
	}
	return
}

func (c *Command) actionAppRename(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	beIo.LogFile = ""

	argv := ctx.Args().Slice()
	argc := len(argv)
	if argc != 2 {
		cli.ShowSubcommandHelpAndExit(ctx, 1)
	}

	if syscall.Getuid() != 0 {
		err = fmt.Errorf("'enjenv niseroku app rename' command requires super user privileges")
		return
	}

	oldName := argv[0]
	newName := argv[1]

	if app, ok := c.config.Applications[newName]; ok {
		err = fmt.Errorf("'%v' exists already, cannot rename", app.Name)
		return
	}

	var oldApp *Application
	if app, ok := c.config.Applications[oldName]; ok {
		if app.IsDeploying() {
			err = fmt.Errorf("application deployment in progress: %v", app.Name)
			return
		}
		beIo.STDOUT("# stopping: %v\n", oldName)
		_ = app.SendStopSignal()
		oldApp = app
	} else {
		err = fmt.Errorf("'%v' does not exist, nothing to rename", oldName)
		return
	}

	// - rename this-slug and next-slug values and files
	if oldApp.ThisSlug != "" {
		if basename := filepath.Base(oldApp.ThisSlug); RxSlugArchiveName.MatchString(basename) {
			m := RxSlugArchiveName.FindAllStringSubmatch(basename, 1)
			if name := m[0][1]; name == oldName {
				newSlug := filepath.Join(c.config.Paths.VarSlugs, newName+"--"+m[0][2]+".zip")
				if ee := os.Rename(oldApp.ThisSlug, newSlug); ee != nil {
					beIo.STDERR("error renaming slug: %v - %v\n", oldApp.ThisSlug, ee)
				} else {
					oldApp.ThisSlug = newSlug
					_ = common.RepairOwnership(newSlug, c.config.RunAs.User, c.config.RunAs.Group)
					beIo.STDOUT("# renamed: %v\n", newSlug)
				}
			}
		}
	}
	if oldApp.NextSlug != "" {
		if basename := filepath.Base(oldApp.NextSlug); RxSlugArchiveName.MatchString(basename) {
			m := RxSlugArchiveName.FindAllStringSubmatch(basename, 1)
			if name := m[0][1]; name == oldName {
				newSlug := filepath.Join(c.config.Paths.VarSlugs, newName+"--"+m[0][2]+".zip")
				if ee := os.Rename(oldApp.NextSlug, newSlug); ee != nil {
					beIo.STDERR("error renaming slug: %v - %v\n", oldApp.NextSlug, ee)
				} else {
					oldApp.NextSlug = newSlug
					_ = common.RepairOwnership(newSlug, c.config.RunAs.User, c.config.RunAs.Group)
					beIo.STDOUT("# renamed: %v\n", newSlug)
				}
			}
		}
	}
	if ee := oldApp.Save(true); ee != nil {
		err = fmt.Errorf("error saving %v: %v", oldApp.Name, ee)
		return
	} else {
		_ = common.RepairOwnership(oldApp.Source, c.config.RunAs.User, c.config.RunAs.Group)
	}

	// - rename config.toml
	oldToml := filepath.Join(c.config.Paths.EtcApps, oldName+".toml")
	newToml := filepath.Join(c.config.Paths.EtcApps, newName+".toml")
	if err = os.Rename(oldToml, newToml); err != nil {
		err = fmt.Errorf("error renaming %v: %v\n", newToml, err)
		return
	} else {
		_ = common.RepairOwnership(newToml, c.config.RunAs.User, c.config.RunAs.Group)
		beIo.STDOUT("# renamed: %v\n", newToml)
	}

	// - rename all other slug filenames
	var filenames []string
	if filenames, err = path.ListFiles(c.config.Paths.VarSlugs, false); err != nil {
		err = fmt.Errorf("error listing var-slug files: %v\n", err)
		return
	}
	for _, filename := range filenames {
		if basename := filepath.Base(filename); RxSlugArchiveName.MatchString(basename) {
			m := RxSlugArchiveName.FindAllStringSubmatch(basename, 1)
			if name := m[0][1]; name == oldName {
				newSlug := filepath.Join(c.config.Paths.VarSlugs, newName+"--"+m[0][2]+".zip")
				if ee := os.Rename(filename, newSlug); ee != nil {
					beIo.STDERR("error renaming slug: %v - %v\n", filename, ee)
				} else {
					_ = common.RepairOwnership(newSlug, c.config.RunAs.User, c.config.RunAs.Group)
					beIo.STDOUT("# renamed: %v\n", newSlug)
				}
			}
		}
	}

	// - rename git-repository
	if path.IsDir(oldApp.RepoPath) {
		newRepo := filepath.Join(c.config.Paths.VarRepos, newName+".git")
		if ee := os.Rename(oldApp.RepoPath, newRepo); ee != nil {
			beIo.STDERR("error renaming repo: %v - %v\n", oldApp.RepoPath, ee)
		} else {
			_ = common.RepairOwnership(newRepo, c.config.RunAs.User, c.config.RunAs.Group)
			beIo.STDOUT("# renamed: %v\n", newRepo)
		}
	}

	// - rename caches.d
	oldCacheD := filepath.Join(oldApp.Config.Paths.VarCache, oldName)
	if path.IsDir(oldCacheD) {
		newCacheD := filepath.Join(oldApp.Config.Paths.VarCache, newName)
		if ee := os.Rename(oldCacheD, newCacheD); ee != nil {
			beIo.STDERR("error renaming cache.d: %v - %v\n", newCacheD, ee)
		} else {
			_ = common.RepairOwnership(newCacheD, c.config.RunAs.User, c.config.RunAs.Group)
			beIo.STDOUT("# renamed: %v\n", newCacheD)
		}
	}

	// - rename log files
	var logfiles []string
	if logfiles, err = path.ListFiles(c.config.Paths.VarLogs, false); err != nil {
		err = fmt.Errorf("error listing var-log files: %v\n", err)
		return
	}
	for _, logfile := range logfiles {
		if basename := filepath.Base(logfile); RxLogFileName.MatchString(basename) {
			// this ignores log rotated files
			m := RxLogFileName.FindAllStringSubmatch(basename, 1)
			if name := m[0][1]; name == oldName {
				var newLogName string
				if m[0][2] == "" {
					newLogName = newName + ".log"
				} else {
					newLogName = newName + "." + m[0][2] + ".log"
				}
				newLog := filepath.Join(c.config.Paths.VarLogs, newLogName)
				if ee := os.Rename(logfile, newLog); ee != nil {
					beIo.STDERR("error renaming log: %v - %v\n", logfile, ee)
				} else {
					_ = common.RepairOwnership(newLog, c.config.RunAs.User, c.config.RunAs.Group)
					beIo.STDOUT("# renamed: %v\n", newLog)
				}
			}
		}
	}

	// - reload this config instance
	if err = c.config.Reload(); err != nil {
		err = fmt.Errorf("error reloading niseroku configuration files: %v", err)
		return
	}

	// - start new app name
	if app, ok := c.config.Applications[newName]; ok {
		if _, _, eee := pkgRun.EnjenvCmd("niseroku", "--config", c.config.Source, "app", "start", "--force", app.Name); eee != nil {
			err = fmt.Errorf("error starting %v application: %v\n", app.Name, eee)
			return
		}
	}

	// - reload niseroku services
	c.config.SignalReloadGitRepository()
	c.config.SignalReloadReverseProxy()
	return
}
