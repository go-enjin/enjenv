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
}