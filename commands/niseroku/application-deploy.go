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
	"strconv"
	"time"

	bePath "github.com/go-enjin/be/pkg/path"

	"github.com/go-enjin/enjenv/pkg/service/common"
)

func (a *Application) lockDeploy() (err error) {
	if a.IsDeploying() {
		err = fmt.Errorf("deployment already in progress")
	} else {
		_ = os.WriteFile(a.DeployFile, []byte(strconv.Itoa(os.Getpid())), 0664)
	}
	return
}

func (a *Application) unlockDeploy() {
	if a.IsDeploying() {
		_ = os.Remove(a.DeployFile)
	}
}

func (a *Application) IsDeploying() (locked bool) {
	if bePath.Exists(a.DeployFile) {
		if proc, err := common.GetProcessFromPidFile(a.DeployFile); err == nil {
			if locked, err = proc.IsRunning(); locked {
				return
			}
		}
		// remove stale deploy file
		_ = os.Remove(a.DeployFile)
	}
	return
}

func (a *Application) Deploy() (err error) {
	if err = a.lockDeploy(); err != nil {
		return
	}

	if err = a.PrepareGpgSecrets(); err != nil {
		a.unlockDeploy()
		return
	}

	var label string
	var targetSlug *Slug
	thisSlug := a.GetThisSlug()
	nextSlug := a.GetNextSlug()

	switch {
	case thisSlug == nil && nextSlug == nil:
		err = fmt.Errorf("slug not found")
		a.unlockDeploy()
		return
	case thisSlug == nil && nextSlug != nil:
		label = "first"
		a.LogInfoF("deploying first: %v\n", nextSlug.Name)
		targetSlug = nextSlug
	case thisSlug != nil && nextSlug == nil:
		label = "this"
		a.LogInfoF("deploying this: %v\n", thisSlug.Name)
		targetSlug = thisSlug
	case thisSlug != nil && nextSlug != nil:
		targetSlug = nextSlug
		if thisSlug.Name == nextSlug.Name {
			label = "same"
			a.LogInfoF("deploying same: %v\n", thisSlug.Name)
		} else {
			label = "next"
			a.LogInfoF("deploying next: %v\n", nextSlug.Name)
		}
	}

	if err = a.migrateAppSlug(targetSlug); err != nil {
		a.LogErrorF("error migrating %v slug: %v\n", label, targetSlug.Name)
		a.unlockDeploy()
		return
	}
	targetSlug.RefreshWorkers()
	a.LogInfoF("migrated to %v slug: %v\n", label, targetSlug)
	a.unlockDeploy()
	<-a.awaitWorkersDone
	return
}

func (a *Application) migrateAppSlug(slug *Slug) (err error) {
	a.LogInfoF("migrating to app slug: %v\n", slug.Name)
	workersReady := make(chan bool)
	go func() {
		if err = slug.StartForegroundWorkers(workersReady); err != nil {
			a.LogErrorF("error starting slug: %v - %v\n", slug.Name, err)
		}
		a.awaitWorkersDone <- true
	}()
	<-workersReady
	if err == nil {
		err = a.transitionAppToNextSlug(slug.App)
	}
	return
}

func (a *Application) transitionAppToNextSlug(app *Application) (err error) {
	thisSlug := app.GetThisSlug()

	var nextSlug *Slug
	if nextSlug = app.GetNextSlug(); nextSlug != nil {
		app.ThisSlug = app.NextSlug
		app.NextSlug = ""

		if ee := app.Save(true); ee != nil {
			err = fmt.Errorf("error saving: %v - %v\n", app.Name, ee)
			return
		}
		if ee := a.Config.Reload(); ee != nil {
			err = fmt.Errorf("error reloading: %v - %v", app.Name, ee)
			return
		}

		if thisSlug == nil {
			// first deployment, nothing to clean up
		} else if thisSlug.Name == nextSlug.Name {
			// re-deployment, slug handles its own workers?
			thisSlug.StopAll()
		} else if !a.Config.KeepSlugs {
			if ee := thisSlug.Destroy(); ee != nil {
				a.LogErrorF("error destroying slug: %v - %v", thisSlug.Name, ee)
			}
		} else {
			stopped := thisSlug.StopAll()
			a.LogInfoF("slug stopped %d instances: %v", stopped, thisSlug.Name)
		}

		a.LogInfoF("app transitioned to slug: %v\n", nextSlug.Name)
	}

	time.Sleep(250 * time.Millisecond) // slight delay, allow os to settle down?
	a.LogInfoF("sending reverse-proxy reload signal\n")
	a.Config.SignalReloadReverseProxy()
	return
}