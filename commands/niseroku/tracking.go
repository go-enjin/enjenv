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

	"github.com/go-enjin/be/pkg/maps"
)

type Tracking struct {
	data map[string]int64

	sync.RWMutex
}

func NewTracking() (t *Tracking) {
	t = new(Tracking)
	t.data = make(map[string]int64)
	return
}

func (t *Tracking) Get(key string) (value int64) {
	t.RLock()
	defer t.RUnlock()
	if v, ok := t.data[key]; ok {
		value = v
	} else {
		value = -1
	}
	return
}

func (t *Tracking) String() (summary string) {
	t.RLock()
	defer t.RUnlock()
	if v, ok := t.data["__total__"]; ok {
		summary += fmt.Sprintf("__total__=%d\n", v)
	} else {
		summary += "__total__=0\n"
	}
	for _, key := range maps.SortedKeys(t.data) {
		if key == "__total__" || t.data[key] <= 0 {
			continue
		}
		summary += fmt.Sprintf("%s=%d\n", key, t.data[key])
	}
	return
}

func (t *Tracking) Increment(keys ...string) {
	t.Lock()
	defer t.Unlock()
	for _, key := range keys {
		if _, ok := t.data[key]; ok {
			t.data[key] += 1
		} else {
			t.data[key] = 1
		}
	}
}

func (t *Tracking) Decrement(keys ...string) {
	t.Lock()
	defer t.Unlock()
	for _, key := range keys {
		if _, ok := t.data[key]; ok {
			t.data[key] -= 1
		} else {
			t.data[key] = 0
		}
	}
}