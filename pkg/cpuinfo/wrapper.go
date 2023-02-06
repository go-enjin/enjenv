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

// #include "wrapper.h"
import (
	"C"
)

func CpuTick() (t int64) {
	return int64(C.read_cpu_tick())
}

func TimeFromPid(pid int) (t int64) {
	return int64(C.read_time_from_pid(C.int(pid)))
}

func NumCores() (n int) {
	return int(C.num_cores())
}

func GetPidStats(pid int) (t int64, ppid, nice, threads int) {
	cppid := C.int(0)
	ctime := C.ulong(0)
	cnice := C.int(0)
	cthreads := C.int(0)
	if ok := C.read_stat_from_pid(C.int(pid), &cppid, &ctime, &cnice, &cthreads); ok > 0 {
		t = int64(ctime)
		ppid = int(cppid)
		nice = int(cnice)
		threads = int(cthreads)
	}
	return
}