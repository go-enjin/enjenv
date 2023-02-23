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
	"math/rand"
	"time"
)

func (c *Config) GetUnusedPort() (port int) {
	c.RLock()
	defer c.RUnlock()
	rand.New(rand.NewSource(time.Now().UnixMicro()))
	delta := c.Ports.AppEnd - c.Ports.AppStart
	for loop := delta; loop > 0; loop -= 1 {
		port = rand.Intn(delta) + c.Ports.AppStart
		if _, exists := c.PortLookup[port]; !exists {
			if _, reserved := c.ReservePorts[port]; !reserved {
				break
			}
		}
	}
	return
}

func (c *Config) AddToPortLookup(port int, app *Application) {
	c.Lock()
	defer c.Unlock()
	c.PortLookup[port] = app
}

func (c *Config) RemoveFromPortLookup(port int) {
	c.Lock()
	defer c.Unlock()
	delete(c.PortLookup, port)
}

func (c *Config) ReservePort(port int, app *Application) (err error) {
	c.Lock()
	defer c.Unlock()
	if existingApp, exists := c.ReservePorts[port]; exists {
		err = fmt.Errorf("port already reserved by: %v", existingApp.Name)
		return
	}
	c.ReservePorts[port] = app
	return
}

func (c *Config) RemovePortReservation(port int) {
	c.Lock()
	defer c.Unlock()
	delete(c.ReservePorts, port)
}

func (c *Config) PromotePortReservation(port int) (err error) {
	c.Lock()
	defer c.Unlock()
	if app, reservationExists := c.ReservePorts[port]; reservationExists {
		c.PortLookup[port] = app
		delete(c.ReservePorts, port)
		return
	}
	err = fmt.Errorf("port not reserved")
	return
}