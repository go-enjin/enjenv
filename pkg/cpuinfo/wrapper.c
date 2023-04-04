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

#include <stdio.h>
#include <stdbool.h>
#include <unistd.h>

#define MAX_NAME 128

// read total cpu time
unsigned long long int read_cpu_tick() {
  unsigned long long int usertime, nicetime, systemtime, idletime;
  unsigned long long int ioWait, irq, softIrq, steal, guest, guestnice;
  usertime = nicetime = systemtime = idletime = 0;
  ioWait = irq = softIrq = steal = guest = guestnice = 0;

  FILE *fp;
  fp = fopen("/proc/stat", "r");
  if (fp != NULL) {
    if (fscanf(fp,   "cpu  %16llu %16llu %16llu %16llu %16llu %16llu %16llu %16llu %16llu %16llu",
               &usertime, &nicetime, &systemtime, &idletime,
               &ioWait, &irq, &softIrq, &steal, &guest, &guestnice) == EOF) {
      fclose(fp);
      return 0;
    } else {
      fclose(fp);
      return usertime + nicetime + systemtime + idletime + ioWait + irq + softIrq + steal + guest + guestnice;
    }
  }else{
    return 0;
  }
}

// read cpu tick for a specific process
unsigned long read_time_from_pid(int pid) {

  char fn[MAX_NAME + 1];
  char name[MAX_NAME + 1];
  snprintf(fn, sizeof fn, "/proc/%i/stat", pid);

  unsigned long utime = 0;
  unsigned long stime = 0;

  FILE *fp;
  fp = fopen(fn, "r");
  if (fp != NULL) {
    bool ans = fscanf(fp, "%*d %s %*c %*d %*d %*d %*d %*d %*u %*u %*u %*u %*u %lu"
                      "%lu %*ld %*ld %*d %*d %*d %*d %*u %*lu %*ld",
                      name, &utime, &stime) != EOF;
    fclose(fp);
    if (!ans) {
      return 0;
    }

    return utime + stime;
  }
  return 0;
}

// read cpu tick for a specific process
int read_stat_from_pid(int pid, int *ppid, int *pgrp, unsigned long *time, int *nice, int *threads, unsigned long *starttime) {
  char pidStatFile[MAX_NAME + 1];
  snprintf(pidStatFile, sizeof pidStatFile, "/proc/%i/stat", pid);

  unsigned long _utime = 0;
  unsigned long _stime = 0;
  unsigned long _starttime = 0;
  int _ppid = 0;
  int _pgrp = 0;
  int _nice = 0;
  int _threads = 0;

  FILE *fp;
  fp = fopen(pidStatFile, "r");
  if (fp != NULL) {
    bool ans = fscanf(fp,
                      "%*d "  //pid
                      "%*s "  //comm
                      "%*c "  //state
                      "%d "   //ppid
                      "%d "   //pgrp
                      "%*d "  //session
                      "%*d "  //tty_nr
                      "%*d "  //tpgid
                      "%*u "  //flags
                      "%*u "  //minflt
                      "%*u "  //cminflt
                      "%*u "  //majflt
                      "%*u "  //cmajflt
                      "%lu "  //utime
                      "%lu "  //stime
                      "%*ld " //cutime
                      "%*ld " //cstime
                      "%*d "  //priority
                      "%ld "  //nice
                      "%ld "  //num_threads
                      "%*d "  //itrealvalue
                      "%llu " //starttime
                      "%*lu " //vsize
                      "%*ld", //rss
                      &_ppid, &_pgrp, &_utime, &_stime, &_nice, &_threads, &_starttime) != EOF;
    fclose(fp);
    if (!ans) {
      return false;
    }

    *ppid = _ppid;
    *pgrp = _pgrp;
    *time = _utime + _stime;
    *nice = _nice;
    *threads = _threads;
    *starttime = _starttime;
    return true;
  }

  return false;
}

// return number of cores
unsigned int num_cores(){
  return sysconf(_SC_NPROCESSORS_ONLN);
}