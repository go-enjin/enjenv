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
	"time"

	"github.com/BurntSushi/toml"

	bePath "github.com/go-enjin/be/pkg/path"
)

var (
	DefaultGitPort   = 22
	DefaultHttpPort  = 80
	DefaultHttpsPort = 443

	DefaultRunAsUser  = "www-data"
	DefaultRunAsGroup = "www-data"

	DefaultBuildPack = "https://github.com/go-enjin/enjenv-heroku-buildpack.git"

	DefaultSlugStartupTimeout   = 5 * time.Minute
	DefaultOriginRequestTimeout = time.Minute
	DefaultReadyIntervalTimeout = time.Second
)

type Config struct {
	BindAddr     string `toml:"bind-addr,omitempty"`
	EnableSSL    bool   `toml:"enable-ssl,omitempty"`
	BuildPack    string `toml:"buildpack-path,omitempty"`
	AccountEmail string `toml:"account-email,omitempty"`
	LogFile      string `toml:"log-file,omitempty"`

	Timeouts TimeoutsConfig `toml:"timeouts,omitempty"`

	RunAs RunAsConfig `toml:"run-as,omitempty"`
	Ports PortsConfig `toml:"ports,omitempty"`
	Paths PathsConfig `toml:"paths"`

	Source   string `toml:"-"`
	NeedRoot bool   `toml:"-"`
}

type TimeoutsConfig struct {
	SlugStartup   time.Duration `toml:"slug-startup,omitempty"`
	ReadyInterval time.Duration `toml:"ready-interval,omitempty"`
	OriginRequest time.Duration `toml:"origin-request,omitempty"`
}

type RunAsConfig struct {
	User  string `toml:"user,omitempty"`
	Group string `toml:"group,omitempty"`
}

type PortsConfig struct {
	Git      int `toml:"git,omitempty"`
	Http     int `toml:"http,omitempty"`
	Https    int `toml:"https,omitempty"`
	AppEnd   int `toml:"app-end,omitempty"`
	AppStart int `toml:"app-start,omitempty"`
}

type PathsConfig struct {
	Etc string `toml:"etc"`
	Var string `toml:"var"`
	Tmp string `toml:"tmp"`

	EtcApps      string `toml:"-"` // EtcApps contains all app.toml files
	TmpRun       string `toml:"-"` // TmpRun is used when running enjenv slugs
	TmpClone     string `toml:"-"` // TmpClone is used during deployment for buildpack clones
	TmpBuild     string `toml:"-"` // TmpBuild is used during deployment for app build directories
	VarLogs      string `toml:"-"` // VarLogs is where slug log files are stored
	VarRepos     string `toml:"-"` // VarRepos is where git repos are stored
	VarCache     string `toml:"-"` // VarCache is where build cache directories as stored
	VarSlugs     string `toml:"-"` // VarSlugs is where slug archives are stored
	VarSettings  string `toml:"-"` // VarSettings is where slug env directories are stored
	RepoSecrets  string `toml:"-"` // RepoSecrets is where ssh-keys are stored
	ProxySecrets string `toml:"-"` // ProxySecrets is where ssl-certs are stored

	Control string `toml:"-"` // Control is the path for local unix socket file
	PidFile string `toml:"-"` // PidFile is the path for the process ID file
}

func InitConfig(niserokuConfig string) (config *Config, err error) {

	if niserokuConfig == "" {
		err = fmt.Errorf("missing --config")
		return
	} else if !bePath.IsFile(niserokuConfig) {
		err = fmt.Errorf("not a file: %v", niserokuConfig)
		return
	} else if niserokuConfig, err = bePath.Abs(niserokuConfig); err != nil {
		return
	}

	var cfg Config
	if _, err = toml.DecodeFile(niserokuConfig, &cfg); err != nil {
		return
	}

	if cfg.Paths.Etc == "" || cfg.Paths.Var == "" || cfg.Paths.Tmp == "" {
		err = fmt.Errorf("missing one or more of etc-dir, var-dir and/or tmp-dir settings")
		return
	}
	if cfg.Paths.Etc, err = bePath.Abs(cfg.Paths.Etc); err != nil {
		return
	} else if cfg.Paths.Var, err = bePath.Abs(cfg.Paths.Var); err != nil {
		return
	} else if cfg.Paths.Tmp, err = bePath.Abs(cfg.Paths.Tmp); err != nil {
		return
	}

	appsPath := cfg.Paths.Etc + "/apps.d"
	proxySecrets := cfg.Paths.Etc + "/secrets.proxy.d"
	// etcRepoPath := cfg.Paths.Etc + "/repos.d"
	varReposPath := cfg.Paths.Var + "/repos.d"
	repoSecrets := cfg.Paths.Etc + "/secrets.repos.d"
	tmpRun := cfg.Paths.Tmp + "/runner.d"
	tmpClone := cfg.Paths.Tmp + "/clones.d"
	tmpBuild := cfg.Paths.Tmp + "/builds.d"
	varLogs := cfg.Paths.Var + "/logs.d"
	varCache := cfg.Paths.Var + "/caches.d"
	varSlugs := cfg.Paths.Var + "/slugs.d"
	varSettings := cfg.Paths.Var + "/settings.d"

	if cfg.EnableSSL && cfg.AccountEmail == "" {
		err = fmt.Errorf("enable-ssl requires account-email setting")
		return
	}

	if cfg.BuildPack == "" {
		cfg.BuildPack = DefaultBuildPack
	}

	pidFile := cfg.Paths.Var + "/" + Name + ".pid"
	controlFile := cfg.Paths.Var + "/" + Name + ".sock"

	var needRootUser bool
	checkPort := func(port, defaultPort int) (validPort int, err error) {
		if port <= 0 {
			validPort = defaultPort
		} else if port <= 1024 {
			needRootUser = true
			validPort = port
		} else if port < 65525 {
			validPort = port
		} else {
			err = fmt.Errorf("port out of range [1-65534]")
		}
		return
	}
	var gitPort, httpPort, httpsPort int
	if gitPort, err = checkPort(cfg.Ports.Git, DefaultGitPort); err != nil {
		err = fmt.Errorf("[git] %v", err)
		return
	} else if httpPort, err = checkPort(cfg.Ports.Http, DefaultHttpPort); err != nil {
		err = fmt.Errorf("[http] %v", err)
		return
	} else if httpsPort, err = checkPort(cfg.Ports.Https, DefaultHttpsPort); err != nil {
		err = fmt.Errorf("[https] %v", err)
		return
	}

	var runAsUser, runAsGroup string
	if runAsUser = cfg.RunAs.User; runAsUser == "" {
		runAsUser = DefaultRunAsUser
		runAsGroup = DefaultRunAsGroup
	} else if runAsGroup = cfg.RunAs.Group; runAsGroup == "" {
		runAsGroup = runAsUser
	}

	var slugStartupTimeout, originRequestTimeout, readyIntervalTimeout time.Duration
	if cfg.Timeouts.SlugStartup > 0 {
		slugStartupTimeout = cfg.Timeouts.SlugStartup
	} else {
		slugStartupTimeout = DefaultSlugStartupTimeout
	}
	if cfg.Timeouts.OriginRequest > 0 {
		originRequestTimeout = cfg.Timeouts.OriginRequest
	} else {
		originRequestTimeout = DefaultOriginRequestTimeout
	}
	if cfg.Timeouts.ReadyInterval > 0 {
		readyIntervalTimeout = cfg.Timeouts.ReadyInterval
	} else {
		readyIntervalTimeout = DefaultReadyIntervalTimeout
	}

	var appEndPort, appStartPort int
	if cfg.Ports.AppEnd > 0 {
		appEndPort = cfg.Ports.AppEnd
	} else {
		appEndPort = 4400
	}
	if cfg.Ports.AppStart > 0 {
		appStartPort = cfg.Ports.AppStart
	} else {
		appStartPort = 4200
	}

	if appStartPort >= appEndPort {
		err = fmt.Errorf("app end port %d is less than (or equal to) the start port %d", appEndPort, appStartPort)
		return
	}
	if appStartPort <= 0 {
		appStartPort = 4200
	}
	if appEndPort <= 0 {
		appEndPort = 4400
	}

	var bindAddr string
	if bindAddr = cfg.BindAddr; bindAddr == "" {
		bindAddr = "0.0.0.0"
	}

	config = &Config{
		Source:       niserokuConfig,
		LogFile:      cfg.LogFile,
		NeedRoot:     needRootUser,
		BindAddr:     bindAddr,
		EnableSSL:    cfg.EnableSSL,
		BuildPack:    cfg.BuildPack,
		AccountEmail: cfg.AccountEmail,
		Timeouts: TimeoutsConfig{
			SlugStartup:   slugStartupTimeout,
			ReadyInterval: readyIntervalTimeout,
			OriginRequest: originRequestTimeout,
		},
		RunAs: RunAsConfig{
			User:  runAsUser,
			Group: runAsGroup,
		},
		Ports: PortsConfig{
			Git:      gitPort,
			Http:     httpPort,
			Https:    httpsPort,
			AppEnd:   appEndPort,
			AppStart: appStartPort,
		},
		Paths: PathsConfig{
			Etc:          cfg.Paths.Etc,
			Var:          cfg.Paths.Var,
			Tmp:          cfg.Paths.Tmp,
			EtcApps:      appsPath,
			TmpRun:       tmpRun,
			TmpClone:     tmpClone,
			TmpBuild:     tmpBuild,
			VarLogs:      varLogs,
			VarRepos:     varReposPath,
			VarCache:     varCache,
			VarSlugs:     varSlugs,
			VarSettings:  varSettings,
			RepoSecrets:  repoSecrets,
			ProxySecrets: proxySecrets,
			PidFile:      pidFile,
			Control:      controlFile,
		},
	}

	return
}

func (c *Config) PrepareDirectories() (err error) {

	for _, p := range []string{
		c.Paths.Etc,
		c.Paths.EtcApps,
		c.Paths.Tmp,
		c.Paths.TmpRun,
		c.Paths.TmpClone,
		c.Paths.TmpBuild,
		c.Paths.Var,
		c.Paths.VarLogs,
		c.Paths.VarSlugs,
		c.Paths.VarSettings,
		c.Paths.VarCache,
		c.Paths.VarRepos,
		c.Paths.RepoSecrets,
		c.Paths.ProxySecrets,
	} {
		if err = bePath.Mkdir(p); err != nil {
			err = fmt.Errorf("error preparing directory: %v - %v", p, err)
			return
		}
	}
	return
}