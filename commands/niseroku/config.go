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
	"sync"
	"time"

	"github.com/BurntSushi/toml"

	bePath "github.com/go-enjin/be/pkg/path"
)

var (
	DefaultGitPort      = 22
	DefaultHttpPort     = 80
	DefaultHttpsPort    = 443
	DefaultAppEndPort   = 4400
	DefaultAppStartPort = 4200

	DefaultRunAsUser  = "www-data"
	DefaultRunAsGroup = "www-data"

	DefaultBuildPack = "https://github.com/go-enjin/enjenv-heroku-buildpack.git"

	DefaultSlugStartupTimeout   = 5 * time.Minute
	DefaultOriginRequestTimeout = time.Minute
	DefaultReadyIntervalTimeout = time.Second

	DefaultRateLimitTTL        time.Duration = 8760 * time.Hour
	DefaultRateLimitMax        float64       = 150.0
	DefaultRateLimitBurst      int           = 0
	DefaultRateLimitMaxDelay   time.Duration = 2 * time.Second
	DefaultRateLimitDelayScale int           = 10
)

type Config struct {
	BindAddr     string `toml:"bind-addr,omitempty"`
	EnableSSL    bool   `toml:"enable-ssl,omitempty"`
	BuildPack    string `toml:"buildpack-path,omitempty"`
	AccountEmail string `toml:"account-email,omitempty"`
	LogFile      string `toml:"log-file,omitempty"`

	SlugNice int `toml:"slug-nice,omitempty"`

	IncludeSlugs IncludeSlugsConfig `toml:"include-slugs"`

	Timeouts TimeoutsConfig `toml:"timeouts,omitempty"`

	ProxyLimit RateLimit `toml:"proxy-limit,omitempty"`

	RunAs RunAsConfig `toml:"run-as,omitempty"`
	Ports PortsConfig `toml:"ports,omitempty"`
	Paths PathsConfig `toml:"paths"`

	Source   string `toml:"-"`
	NeedRoot bool   `toml:"-"`

	Users        []*User                 `toml:"-"`
	Applications map[string]*Application `toml:"-"`
	PortLookup   map[int]*Application    `toml:"-"`
	DomainLookup map[string]*Application `toml:"-"`

	sync.RWMutex
}

type IncludeSlugsConfig struct {
	OnStart bool `toml:"on-start,omitempty"`
	OnStop  bool `toml:"on-stop,omitempty"`
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

type RateLimit struct {
	TTL        time.Duration `toml:"ttl,omitempty"`
	Max        float64       `toml:"max,omitempty"`
	Burst      int           `toml:"burst,omitempty"`
	MaxDelay   time.Duration `toml:"max-delay,omitempty"`
	DelayScale int           `toml:"delay-scale,omitempty"`
	LogAllowed bool          `toml:"log-allowed,omitempty"`
	LogDelayed bool          `toml:"log-delayed,omitempty"`
	LogLimited bool          `toml:"log-limited,omitempty"`
}

type PathsConfig struct {
	Etc string `toml:"etc"`
	Var string `toml:"var"`
	Tmp string `toml:"tmp"`

	EtcApps      string `toml:"-"` // EtcApps contains all app.toml files
	EtcUsers     string `toml:"-"` // EtcUsers contains all the user.toml files
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

	RepoPidFile  string `toml:"-"` // RepoPidFile is the path for the git-repository service process ID file
	ProxyPidFile string `toml:"-"` // ProxyPidFile is the path for the reverse-proxy service process ID file
}

func LoadConfig(niserokuConfig string) (config *Config, err error) {

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

	if cfg.SlugNice < -10 && cfg.SlugNice > 20 {
		err = fmt.Errorf("slug-nice value out of range: -10 to 20")
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
	usersPath := cfg.Paths.Etc + "/users.d"
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

	repoPidFile := cfg.Paths.Var + "/git-repository.pid"
	proxyPidFile := cfg.Paths.Var + "/reverse-proxy.pid"

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
		appEndPort = DefaultAppEndPort
	}
	if cfg.Ports.AppStart > 0 {
		appStartPort = cfg.Ports.AppStart
	} else {
		appStartPort = DefaultAppStartPort
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
		SlugNice:     cfg.SlugNice,
		IncludeSlugs: cfg.IncludeSlugs,
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
		ProxyLimit: RateLimit{
			TTL:        CheckAB(cfg.ProxyLimit.TTL, DefaultRateLimitTTL, cfg.ProxyLimit.TTL >= 0),
			Max:        CheckAB(cfg.ProxyLimit.Max, DefaultRateLimitMax, cfg.ProxyLimit.Max > 0),
			Burst:      CheckAB(cfg.ProxyLimit.Burst, DefaultRateLimitBurst, cfg.ProxyLimit.Burst > 0),
			MaxDelay:   CheckAB(cfg.ProxyLimit.MaxDelay, DefaultRateLimitMaxDelay, cfg.ProxyLimit.MaxDelay > 0),
			DelayScale: CheckAB(cfg.ProxyLimit.DelayScale, DefaultRateLimitDelayScale, cfg.ProxyLimit.DelayScale > 0),
			LogAllowed: cfg.ProxyLimit.LogAllowed,
			LogDelayed: cfg.ProxyLimit.LogDelayed,
			LogLimited: cfg.ProxyLimit.LogLimited,
		},
		Paths: PathsConfig{
			Etc:          cfg.Paths.Etc,
			Var:          cfg.Paths.Var,
			Tmp:          cfg.Paths.Tmp,
			EtcApps:      appsPath,
			EtcUsers:     usersPath,
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
			RepoPidFile:  repoPidFile,
			ProxyPidFile: proxyPidFile,
		},
	}

	if config.Users, err = LoadUsers(config.Paths.EtcUsers); err != nil {
		return
	}

	if config.Applications, err = LoadApplications(config); err != nil {
		return
	}

	config.PortLookup = make(map[int]*Application)
	config.DomainLookup = make(map[string]*Application)
	for _, app := range config.Applications {
		if _, exists := config.PortLookup[app.Origin.Port]; exists {
			err = fmt.Errorf("port %d duplicated by: %v", app.Origin.Port, app.Source)
			return
		}
		config.PortLookup[app.Origin.Port] = app
		for _, domain := range app.Domains {
			if _, exists := config.DomainLookup[domain]; exists {
				err = fmt.Errorf("domain %v duplicated by: %v", domain, app.Source)
				return
			}
			config.DomainLookup[domain] = app
		}
		if err = app.LoadAllSlugs(); err != nil {
			err = fmt.Errorf("error loading all slugs: %v", err)
			return
		}
	}

	return
}

func (c *Config) Reload() (err error) {
	var cfg *Config
	if cfg, err = LoadConfig(c.Source); err != nil {
		return
	}
	err = c.MergeConfig(cfg)
	return
}

func (c *Config) MergeConfig(cfg *Config) (err error) {
	c.Source = cfg.Source
	c.LogFile = cfg.LogFile
	c.NeedRoot = cfg.NeedRoot
	c.BindAddr = cfg.BindAddr
	c.EnableSSL = cfg.EnableSSL
	c.BuildPack = cfg.BuildPack
	c.AccountEmail = cfg.AccountEmail
	c.IncludeSlugs = cfg.IncludeSlugs
	c.Timeouts.SlugStartup = cfg.Timeouts.SlugStartup
	c.Timeouts.ReadyInterval = cfg.Timeouts.ReadyInterval
	c.Timeouts.OriginRequest = cfg.Timeouts.OriginRequest
	c.ProxyLimit.TTL = cfg.ProxyLimit.TTL
	c.ProxyLimit.Max = cfg.ProxyLimit.Max
	c.ProxyLimit.Burst = cfg.ProxyLimit.Burst
	c.ProxyLimit.MaxDelay = cfg.ProxyLimit.MaxDelay
	c.ProxyLimit.DelayScale = cfg.ProxyLimit.DelayScale
	c.ProxyLimit.LogAllowed = cfg.ProxyLimit.LogAllowed
	c.ProxyLimit.LogDelayed = cfg.ProxyLimit.LogDelayed
	c.ProxyLimit.LogLimited = cfg.ProxyLimit.LogLimited
	c.RunAs.User = cfg.RunAs.User
	c.RunAs.Group = cfg.RunAs.Group
	c.Ports.Git = cfg.Ports.Git
	c.Ports.Http = cfg.Ports.Http
	c.Ports.Https = cfg.Ports.Https
	c.Ports.AppEnd = cfg.Ports.AppEnd
	c.Ports.AppStart = cfg.Ports.AppStart
	c.Paths.Etc = cfg.Paths.Etc
	c.Paths.Var = cfg.Paths.Var
	c.Paths.Tmp = cfg.Paths.Tmp
	c.Paths.EtcApps = cfg.Paths.EtcApps
	c.Paths.EtcUsers = cfg.Paths.EtcUsers
	c.Paths.TmpRun = cfg.Paths.TmpRun
	c.Paths.TmpClone = cfg.Paths.TmpClone
	c.Paths.TmpBuild = cfg.Paths.TmpBuild
	c.Paths.VarLogs = cfg.Paths.VarLogs
	c.Paths.VarRepos = cfg.Paths.VarRepos
	c.Paths.VarCache = cfg.Paths.VarCache
	c.Paths.VarSlugs = cfg.Paths.VarSlugs
	c.Paths.VarSettings = cfg.Paths.VarSettings
	c.Paths.RepoSecrets = cfg.Paths.RepoSecrets
	c.Paths.ProxySecrets = cfg.Paths.ProxySecrets
	c.Paths.Control = cfg.Paths.Control
	c.Paths.PidFile = cfg.Paths.PidFile
	c.Paths.RepoPidFile = cfg.Paths.RepoPidFile
	c.Paths.ProxyPidFile = cfg.Paths.ProxyPidFile
	c.Users = cfg.Users
	c.Applications = cfg.Applications
	c.PortLookup = cfg.PortLookup
	c.DomainLookup = cfg.DomainLookup
	return
}