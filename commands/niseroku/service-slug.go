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

package niseroku

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

/*

	- start/stop repo slugs
	- log stdout/stderr per-slug
	- IsReady() method checks if port is open
	- replace running slug with newly deployed slug

*/

func (s *Server) startAppSlugs() (err error) {
	for _, app := range s.Applications() {
		if app.Maintenance {
			continue
		}
		if ee := s.StartAppSlug(app); ee != nil && !strings.Contains(ee.Error(), "app slugs not found") {
			err = ee
			return
		}
	}
	return
}

func (s *Server) StartAppSlug(app *Application) (err error) {
	var slug *Slug
	if slug, err = s.PrepareAppSlug(app); err != nil {
		return
	}
	s.LogInfoF("starting: %v on port %d\n", slug.Name, app.Origin.Port)
	go s.handleAppSlugStart(slug)
	go s.awaitAppSlugReady(slug)
	return
}

func (s *Server) PrepareAppSlug(app *Application) (slug *Slug, err error) {
	if err = app.LoadAllSlugs(); err != nil {
		err = fmt.Errorf("error setting up app slugs: %v - %v", app.Name, err)
		return
	}

	if slug = app.GetNextSlug(); slug == nil {
		if slug = app.GetThisSlug(); slug == nil {
			err = fmt.Errorf("app slugs not found: %v", app.Name)
			return
		}
	}

	if err = slug.Unpack(); err != nil {
		err = fmt.Errorf("error unpacking this slug: %v - %v", app.Name, err)
		return
	}

	slug.Port = app.Origin.Port
	return
}

func (s *Server) getNextAvailablePort() (port int) {
	delta := s.Config.Ports.AppEnd - s.Config.Ports.AppStart
	for loop := delta; loop > 0; loop -= 1 {
		port = rand.Intn(delta) + s.Config.Ports.AppStart
		if _, exists := s.LookupPort[port]; !exists {
			break
		}
	}
	return
}

func (s *Server) migrateAppSlugs(running []*Slug) (err error) {

	for _, app := range s.Applications() {
		if ee := app.LoadAllSlugs(); ee != nil {
			s.LogErrorF("error loading all app slugs: %v - %v\n", app.Name, ee)
			continue
		}

		thisSlug := app.GetThisSlug()
		nextSlug := app.GetNextSlug()

		if thisSlug != nil && nextSlug != nil {
			// migrating to next from this
			if thisSlug.IsRunning() {
				nextSlug.Port = s.getNextAvailablePort()
			} else {
				nextSlug.Port = app.Origin.Port
			}
			if ee := s.migrateAppSlug(nextSlug); ee != nil {
				s.LogErrorF("error migrating to next slug: %v - %v\n", nextSlug.Name, app.Origin.Port)
				continue
			}
		} else if thisSlug == nil && nextSlug != nil {
			// setting to next
			if !nextSlug.IsRunning() {
				nextSlug.Port = app.Origin.Port
				if ee := s.migrateAppSlug(nextSlug); ee != nil {
					s.LogErrorF("error starting next slug: %v - %v\n", nextSlug.Name, app.Origin.Port)
					continue
				}
			}
		} else if thisSlug != nil && nextSlug == nil {
			// restarting this
			if !thisSlug.IsRunning() {
				if ee := thisSlug.Start(app.Origin.Port); ee != nil {
					s.LogErrorF("error starting this slug: %v - %v\n", thisSlug.Name, app.Origin.Port)
				}
			} else {
				s.LogInfoF("this slug already running: %v\n", thisSlug.Name)
			}
		} else {
			// app has no known slugs
			continue
		}

	}

	slugInSlugs := func(slug *Slug, slugs []*Slug) (ok bool) {
		for _, wrs := range slugs {
			if ok = wrs.Name == slug.Name; ok {
				return
			}
		}
		return
	}

	var keep []*Slug

	for _, app := range s.Applications() {
		if slug := app.GetThisSlug(); slug != nil {
			if slugInSlugs(slug, running) {
				keep = append(keep, slug)
			}
		}
	}

	for _, slug := range running {
		if !slugInSlugs(slug, keep) {
			if slug.IsRunning() {
				slug.Stop()
				s.LogInfoF("slug was running previously and is not running now: %v\n", slug.Archive)
				// if ee := slug.Destroy(); ee != nil {
				// 	s.LogErrorF("error destroying old slug: %v - %v", slug.Archive, ee)
				// }
			}
		}
	}

	return
}

func (s *Server) migrateAppSlug(slug *Slug) (err error) {
	if err = slug.Unpack(); err != nil {
		err = fmt.Errorf("error unpacking this slug: %v - %v", slug.Name, err)
		return
	}
	s.LogInfoF("starting: %v on port %d\n", slug.Name, slug.Port)
	go s.handleAppSlugStart(slug)
	go s.awaitAppSlugReady(slug)
	return
}

func (s *Server) handleAppSlugStart(slug *Slug) {
	if err := slug.Start(slug.Port); err != nil {
		s.LogErrorF("error starting slug: %v - %v\n", slug.Name, err)
	}
	return
}

func (s *Server) awaitAppSlugReady(slug *Slug) {
	slugStartupTimeout := slug.GetSlugStartupTimeout()
	readyIntervalTimeout := slug.GetReadyIntervalTimeout()

	s.LogInfoF("awaiting slug startup: %v - %v\n", slug.Name, slugStartupTimeout)
	start := time.Now()
	for now := time.Now(); now.Sub(start) < slugStartupTimeout; now = time.Now() {
		if isAddressPortOpenWithTimeout(slug.App.Origin.Host, slug.Port, readyIntervalTimeout) {
			s.LogInfoF("slug ready: %v on port %d (%v)\n", slug.Name, slug.Port, time.Now().Sub(start))
			if err := s.transitionAppToNextSlug(slug.App); err != nil {
				s.LogErrorF("error transitioning app to next slug: %v\n", err)
			}
			return
		}
	}
	s.LogInfoF("slug startup timeout reached: %v on port %d\n", slug.Name, slug.Port)
	slug.Stop()
	return
}

func (s *Server) transitionAppToNextSlug(app *Application) (err error) {
	thisSlug := app.GetThisSlug()
	var nextSlug *Slug
	if nextSlug = app.GetNextSlug(); nextSlug != nil {
		// regular deployment

		nextSlug.App.ThisSlug = nextSlug.App.NextSlug
		nextSlug.App.NextSlug = ""
		nextSlug.App.Origin.Port = nextSlug.Port

		if err = nextSlug.App.Save(); err != nil {
			err = fmt.Errorf("error saving app after transitioning: %v - %v\n", app.Name, err)
		} else {
			s.LogInfoF("app transitioned to slug: %v\n", nextSlug.Name)
		}

		s.Lock()
		s.LookupPort[nextSlug.Port] = nextSlug.App
		if thisSlug != nil && thisSlug.Port != nextSlug.Port {
			delete(s.LookupPort, thisSlug.Port)
		}
		s.Unlock()

		if thisSlug == nil {
			// first deployment, nothing to clean up
		} else {
			if err = thisSlug.Destroy(); err != nil {
				err = fmt.Errorf("error destroying slug: %v - %v", thisSlug.Name, err)
			}
		}
	}
	return
}