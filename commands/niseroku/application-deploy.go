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
	"sync"
	"time"

	"github.com/go-enjin/enjenv/pkg/service/common"
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
		a.LogInfoF("deploying this: %v\n", thisSlug)
	}

	if nextSlug != nil {
		// setting to next from nil
		idx := nextSlug.NumInstances()
		if ee := a.migrateAppSlug(nextSlug, idx); ee != nil {
			a.LogErrorF("error migrating next slug: %v\n", nextSlug.Name)
		} else {
			a.LogInfoF("migrated slug: %v\n", nextSlug)
		}
		<-a.await
	} else if thisSlug != nil {
		// start this if not running already
		if !thisSlug.IsRunning(0) {
			unusedPort := a.Config.GetUnusedPort()
			if ee := thisSlug.StartForeground(unusedPort); ee != nil {
				a.LogErrorF("error starting this slug: %v (%d) - %v\n", thisSlug.Name, unusedPort, ee)
			}
		} else {
			a.LogErrorF("slug already running: %v\n", thisSlug.Name)
			// err = fmt.Errorf("slug already running")
		}
	} else {
		// app has no known slugs
		err = fmt.Errorf("slug not found")
	}

	return
}

func (a *Application) migrateAppSlug(slug *Slug, idx int) (err error) {
	a.LogInfoF("migrating to app slug [%d]: %v\n", idx, slug.Name)
	unusedPort := a.Config.GetUnusedPort()
	wg := &sync.WaitGroup{}
	go func() {
		if ee := slug.StartForeground(unusedPort); ee != nil {
			a.LogErrorF("error starting slug: %v - %v\n", slug.Name, ee)
		}
		a.await <- true
	}()
	wg.Add(1)
	go func() {
		a.awaitAppSlugReady(slug, idx, unusedPort)
		wg.Done()
	}()
	wg.Wait()
	return
}

func (a *Application) awaitAppSlugReady(slug *Slug, idx, slugPort int) {
	slugStartupTimeout := slug.GetSlugStartupTimeout()
	readyIntervalTimeout := slug.GetReadyIntervalTimeout()
	// slugPort := slug.GetPort(idx)

	a.LogInfoF("polling slug startup: %v - %v\n", slug.Name, slugStartupTimeout)
	start := time.Now()
	for now := time.Now(); now.Sub(start) < slugStartupTimeout; now = time.Now() {
		// slugPort = slug.GetPort(idx)
		if common.IsAddressPortOpenWithTimeout(slug.App.Origin.Host, slugPort, readyIntervalTimeout) {
			a.LogInfoF("slug ready: %v on port %d (%v)\n", slug.Name, slugPort, time.Now().Sub(start))
			if err := a.transitionAppToNextSlug(slug.App, idx); err != nil {
				a.LogErrorF("error transitioning app to next slug: %v\n", err)
			}
			return
		}
	}

	a.LogInfoF("slug startup timeout reached: %v on port %d\n", slug.Name, slugPort)
	slug.Stop(idx)
	return
}

func (a *Application) transitionAppToNextSlug(app *Application, idx int) (err error) {
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
			numInstances := thisSlug.NumInstances()
			if numInstances > 1 {
				if idx > 0 {
					thisSlug.Stop(0)
				} else {
					thisSlug.Stop(1)
				}
			}
		} else if !a.Config.KeepSlugs {
			if ee := thisSlug.Destroy(); ee != nil {
				a.LogErrorF("error destroying slug: %v - %v", thisSlug.Name, ee)
			}
		} else {
			stopped := thisSlug.StopAll()
			a.LogInfoF("slug stopped %d instances: %v", stopped, thisSlug.Name)
		}

		a.LogInfoF("app transitioned to next slug: %v\n", nextSlug.Name)
		time.Sleep(250 * time.Millisecond) // slight delay, allow os to settle down?
		a.LogInfoF("sending reverse-proxy reload signal\n")
		a.Config.SignalReloadReverseProxy()
	}
	return
}