// Copyright (c) 2018  Patrick Wieschollek - https://github.com/PatWie
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
//
// Notice: this package is based on https://github.com/PatWie/cpuinfo
//         there is no licensing posted, treating as Public Domain

package cpuinfo

import (
	"fmt"
	"time"
)

type CProcess struct {
	Pid      int
	Ppid     int
	Pgrp     int
	Nice     int
	Threads  int
	TimePrev int64
	TimeThis int64
	Dirty    bool
	Active   bool
	Usage    float32
}

type Process struct {
	Pid     int
	Ppid    int
	Pgrp    int
	Nice    int
	Threads int
	Usage   float32
	MemUsed uint64
}

type Stats struct {
	MemUsed  uint64
	MemFree  uint64
	MemTotal uint64
	CpuUsage []float32
	Uptime   time.Duration
}

func (s Stats) String() (text string) {
	var cpuUsage string
	for idx, usage := range s.CpuUsage {
		if idx > 0 {
			cpuUsage += ", "
		}
		cpuUsage += fmt.Sprintf("%0.02f", usage*100.0)
	}
	text = fmt.Sprintf(`{
	"mem-used": %d,
	"mem-free": %d,
	"mem-total": %d,
	"cpu-usage": [%v],
	"uptime": "%s"
}`,
		s.MemUsed,
		s.MemFree,
		s.MemTotal,
		cpuUsage,
		s.UptimeString(),
	)
	return
}

func (s Stats) UptimeString() (up string) {
	seconds := s.Uptime.Seconds()
	hours := int(seconds / 3600)
	days := hours / 24
	minutes := int(seconds/60) % 60
	second := int(seconds) % 60
	hours -= days * 24
	up = fmt.Sprintf("%d days, %02d:%02d:%02d", days, hours, minutes, second)
	return
}