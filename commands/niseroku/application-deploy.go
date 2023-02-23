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
	"time"
)

func (a *Application) Deploy() (err error) {

	thisSlug := a.GetThisSlug()
	nextSlug := a.GetNextSlug()

	if nextSlug != nil {
		if thisSlug.Name == nextSlug.Name {
			a.LogInfoF("re-deploying: %v\n", thisSlug)
		} else {
			a.LogInfoF("deploying next: %v\n", nextSlug)
		}
	} else {
		thisSlug.ReloadSlugInstances()
		a.LogInfoF("deploying this: %v\n", thisSlug)
	}

	var tgtSlug *Slug

	switch {
	case nextSlug != nil:
		tgtSlug = nextSlug
	case thisSlug != nil:
		tgtSlug = thisSlug
	default:
		err = fmt.Errorf("slug not found")
		return
	}

	if err = a.migrateAppSlug(tgtSlug); err != nil {
		a.LogErrorF("error migrating next slug: %v\n", tgtSlug.Name)
		return
	}
	tgtSlug.ReloadSlugInstances()
	a.LogInfoF("migrated to slug: %v\n", tgtSlug)
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