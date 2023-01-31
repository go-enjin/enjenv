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
	"syscall"
	"time"

	"github.com/go-enjin/enjenv/pkg/service/common"
)

func (a *Application) Deploy() (err error) {
	a.await = make(chan bool)

	thisSlug := a.GetThisSlug()
	nextSlug := a.GetNextSlug()

	if thisSlug != nil {
		if proc, _ := thisSlug.GetBinProcess(); proc != nil {
			a.deployPid = int(proc.Pid)
		}
	}

	if nextSlug != nil {
		if thisSlug.Name == nextSlug.Name {
			a.LogInfoF("re-deploying: %v\n", thisSlug)
		} else {
			a.LogInfoF("deploying: %v\n", nextSlug)
		}
	} else {
		a.LogInfoF("deploying: %v\n", thisSlug)
	}

	if thisSlug != nil && nextSlug != nil {
		// migrating to next from this
		nextSlug.Port = a.Config.GetUnusedPort()
		if ee := a.migrateAppSlug(nextSlug); ee != nil {
			a.LogErrorF("error migrating to next slug: %v\n", nextSlug)
		} else {
			a.LogInfoF("migrated slug: %v\n", nextSlug)
			<-a.await
		}
	} else if thisSlug == nil && nextSlug != nil {
		// setting to next from nil
		nextSlug.Port = a.Config.GetUnusedPort()
		if ee := a.migrateAppSlug(nextSlug); ee != nil {
			a.LogErrorF("error starting next slug: %v on port %v\n", nextSlug.Name, nextSlug.Port)
		} else {
			a.LogInfoF("migrated slug: %v on port %d\n", nextSlug, nextSlug.Port)
			<-a.await
		}
	} else if thisSlug != nil && nextSlug == nil {
		// start this if not running already
		if !thisSlug.IsRunning() {
			thisSlug.Port = a.Origin.Port
			if ee := thisSlug.StartForeground(thisSlug.Port); ee != nil {
				a.LogErrorF("error starting this slug: %v (%d) - %v\n", thisSlug.Name, thisSlug.Port, ee)
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

func (a *Application) migrateAppSlug(slug *Slug) (err error) {
	if err = slug.Unpack(); err != nil {
		err = fmt.Errorf("error unpacking this slug: %v - %v", slug.Name, err)
		return
	}
	a.LogInfoF("starting: %v on port %d\n", slug.Name, slug.Port)
	wg := &sync.WaitGroup{}
	go func() {
		a.handleAppSlugStart(slug)
		a.await <- true
	}()
	wg.Add(1)
	go func() {
		a.awaitAppSlugReady(slug)
		wg.Done()
	}()
	wg.Wait()
	return
}

func (a *Application) handleAppSlugStart(slug *Slug) {
	if err := slug.StartForeground(slug.Port); err != nil {
		a.LogErrorF("error starting slug: %v - %v\n", slug.Name, err)
	}
	return
}

func (a *Application) awaitAppSlugReady(slug *Slug) {
	slugStartupTimeout := slug.GetSlugStartupTimeout()
	readyIntervalTimeout := slug.GetReadyIntervalTimeout()

	a.LogInfoF("polling slug startup: %v - %v\n", slug.Name, slugStartupTimeout)
	start := time.Now()
	for now := time.Now(); now.Sub(start) < slugStartupTimeout; now = time.Now() {
		if common.IsAddressPortOpenWithTimeout(slug.App.Origin.Host, slug.Port, readyIntervalTimeout) {
			a.LogInfoF("slug ready: %v on port %d (%v)\n", slug.Name, slug.Port, time.Now().Sub(start))
			if err := a.transitionAppToNextSlug(slug.App); err != nil {
				a.LogErrorF("error transitioning app to next slug: %v\n", err)
			}
			return
		}
	}

	a.LogInfoF("slug startup timeout reached: %v on port %d\n", slug.Name, slug.Port)
	slug.Stop()
	return
}

func (a *Application) transitionAppToNextSlug(app *Application) (err error) {
	thisSlug := app.GetThisSlug()
	var nextSlug *Slug
	if nextSlug = app.GetNextSlug(); nextSlug != nil {
		nextSlug.App.ThisSlug = nextSlug.App.NextSlug
		nextSlug.App.NextSlug = ""
		nextSlug.App.Origin.Port = nextSlug.Port

		if ee := nextSlug.App.Save(true); ee != nil {
			err = fmt.Errorf("error saving: %v - %v\n", app.Name, ee)
			return
		}
		if ee := a.Config.Reload(); ee != nil {
			err = fmt.Errorf("error reloading: %v - %v", app.Name, ee)
			return
		}

		a.LogInfoF("app transitioned to next slug: %v\n", nextSlug.Name)

		time.Sleep(250 * time.Millisecond) // slight delay, allow os to settle down?
		a.LogInfoF("sending reverse-proxy reload signal\n")
		a.Config.SignalReloadReverseProxy()

		if thisSlug == nil {
			// first deployment, nothing to clean up
		} else if thisSlug.Name == nextSlug.Name {
			if a.deployPid > 0 {
				if proc, ee := common.GetProcessFromPid(a.deployPid); ee == nil {
					if ee = proc.SendSignal(syscall.SIGTERM); ee != nil {
						a.LogErrorF("error sending SIGTERM to previous slug instance: %v", ee)
					}
				}
				a.deployPid = 0
			}
		} else if ee := thisSlug.Destroy(); ee != nil {
			a.LogErrorF("error destroying slug: %v - %v", thisSlug.Name, ee)
		}
	}
	return
}