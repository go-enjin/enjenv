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

#ifndef GOCODE_WRAPPER_H
#define GOCODE_WRAPPER_H

unsigned long long int read_cpu_tick();
unsigned long read_time_from_pid(int pid);
int read_stat_from_pid(int pid, int *ppid, unsigned long *time, int *nice, int *threads);
unsigned int num_cores();

#endif // GOCODE_WRAPPER_H