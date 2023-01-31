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
	"bytes"
	"fmt"
	"os"
	"strconv"
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
	BindAddr     string `toml:"bind-addr"`
	EnableSSL    bool   `toml:"enable-ssl"`
	AccountEmail string `toml:"account-email"`
	BuildPack    string `toml:"buildpack-path"`
	LogFile      string `toml:"log-file"`

	SlugNice int `toml:"slug-nice"`

	IncludeSlugs IncludeSlugsConfig `toml:"include-slugs"`

	Timeouts TimeoutsConfig `toml:"timeouts"`

	ProxyLimit RateLimit `toml:"proxy-limit"`

	RunAs RunAsConfig `toml:"run-as"`
	Ports PortsConfig `toml:"ports"`
	Paths PathsConfig `toml:"paths"`

	Source   string `toml:"-"`
	NeedRoot bool   `toml:"-"`

	Users        []*User                 `toml:"-"`
	Applications map[string]*Application `toml:"-"`
	PortLookup   map[int]*Application    `toml:"-"`
	DomainLookup map[string]*Application `toml:"-"`

	tomlMetaData toml.MetaData
	tomlComments TomlComments
	sync.RWMutex
}

type IncludeSlugsConfig struct {
	OnStart bool `toml:"on-start"`
	OnStop  bool `toml:"on-stop"`
}

type TimeoutsConfig struct {
	SlugStartup   time.Duration `toml:"slug-startup"`
	ReadyInterval time.Duration `toml:"ready-interval"`
	OriginRequest time.Duration `toml:"origin-request"`
}

type RunAsConfig struct {
	User  string `toml:"user"`
	Group string `toml:"group"`
}

type PortsConfig struct {
	Git      int `toml:"git"`
	Http     int `toml:"http"`
	Https    int `toml:"https"`
	AppEnd   int `toml:"app-end"`
	AppStart int `toml:"app-start"`
}

type RateLimit struct {
	TTL        time.Duration `toml:"ttl"`
	Max        float64       `toml:"max"`
	Burst      int           `toml:"burst"`
	MaxDelay   time.Duration `toml:"max-delay"`
	DelayScale int           `toml:"delay-scale"`
	LogAllowed bool          `toml:"log-allowed"`
	LogDelayed bool          `toml:"log-delayed"`
	LogLimited bool          `toml:"log-limited"`
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

func WriteDefaultConfig(niserokuConfig string) (err error) {
	var config *Config
	if config, err = validateConfig(niserokuConfig, &Config{
		SlugNice: 0,
		IncludeSlugs: IncludeSlugsConfig{
			OnStart: true,
			OnStop:  false,
		},
		Paths: PathsConfig{
			Etc: "/etc/niseroku",
			Tmp: "/var/lib/niseroku/tmp",
			Var: "/var/lib/niseroku",
		},
	}); err != nil {
		return
	}
	err = config.Save(false)
	return
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
	var contents string
	if b, ee := os.ReadFile(niserokuConfig); ee != nil {
		err = ee
		return
	} else {
		contents = string(b)
	}

	var tmd toml.MetaData
	if tmd, err = toml.Decode(contents, &cfg); err != nil {
		return
	} else {
		cfg.tomlMetaData = tmd
	}

	if tcs, ee := ParseComments(contents); ee != nil {
		err = ee
		return
	} else {
		cfg.tomlComments = MergeConfigToml(tcs, true)
	}

	if config, err = validateConfig(niserokuConfig, &cfg); err != nil {
		return
	}

	err = loadUsersApps(config)
	return
}

func validateConfig(niserokuConfig string, cfg *Config) (config *Config, err error) {

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
		tomlMetaData: cfg.tomlMetaData,
		tomlComments: cfg.tomlComments,
	}
	return
}

func loadUsersApps(config *Config) (err error) {

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

func (c *Config) Save(keepCustomComments bool) (err error) {
	c.RLock()
	defer c.RUnlock()
	var buffer bytes.Buffer
	if err = toml.NewEncoder(&buffer).Encode(c); err != nil {
		return
	}
	contents := string(buffer.Bytes())
	var modified string
	tcs := MergeConfigToml(c.tomlComments, keepCustomComments)
	if modified, err = ApplyComments(contents, tcs); err != nil {
		return
	}
	err = os.WriteFile(c.Source, []byte(modified), 0660)
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
	c.tomlMetaData = cfg.tomlMetaData
	c.tomlComments = cfg.tomlComments
	return
}

func (c *Config) GetTomlValue(key string) (v interface{}) {
	switch key {
	case "buildpack-path":
		v = c.BuildPack
	case "log-file":
		v = c.LogFile
	case "enable-ssl":
		v = c.EnableSSL
	case "account-email":
		v = c.AccountEmail
	case "slug-nice":
		v = c.SlugNice
	case "include-slugs.on-start":
		v = c.IncludeSlugs.OnStart
	case "include-slugs.on-stop":
		v = c.IncludeSlugs.OnStop
	case "timeouts.slug-startup":
		v = c.Timeouts.SlugStartup
	case "timeouts.ready-interval":
		v = c.Timeouts.ReadyInterval
	case "timeouts.origin-request":
		v = c.Timeouts.OriginRequest
	case "proxy-limit.ttl":
		v = c.ProxyLimit.TTL
	case "proxy-limit.max":
		v = c.ProxyLimit.Max
	case "proxy-limit.burst":
		v = c.ProxyLimit.Burst
	case "proxy-limit.max-delay":
		v = c.ProxyLimit.MaxDelay
	case "proxy-limit.delay-scale":
		v = c.ProxyLimit.DelayScale
	case "proxy-limit.log-allowed":
		v = c.ProxyLimit.LogAllowed
	case "proxy-limit.log-delayed":
		v = c.ProxyLimit.LogDelayed
	case "proxy-limit.log-limited":
		v = c.ProxyLimit.LogLimited
	case "run-as.user":
		v = c.RunAs.User
	case "run-as.group":
		v = c.RunAs.Group
	case "ports.git":
		v = c.Ports.Git
	case "ports.http":
		v = c.Ports.Http
	case "ports.https":
		v = c.Ports.Https
	case "ports.app-start":
		v = c.Ports.AppStart
	case "ports.app-end":
		v = c.Ports.AppEnd
	case "paths.etc":
		v = c.Paths.Etc
	case "paths.tmp":
		v = c.Paths.Tmp
	case "paths.var":
		v = c.Paths.Var
	}
	return
}

func (c *Config) SetTomlValue(key string, v string) (err error) {
	switch key {
	case "buildpack-path":
		c.BuildPack, err = c.parseStringValue(v)
	case "log-file":
		c.LogFile, err = c.parseStringValue(v)
	case "enable-ssl":
		c.EnableSSL, err = c.parseBoolValue(v)
	case "account-email":
		c.AccountEmail, err = c.parseStringValue(v)
	case "slug-nice":
		c.SlugNice, err = c.parseIntValue(v)
	case "include-slugs.on-start":
		c.IncludeSlugs.OnStart, err = c.parseBoolValue(v)
	case "include-slugs.on-stop":
		c.IncludeSlugs.OnStop, err = c.parseBoolValue(v)
	case "timeouts.slug-startup":
		c.Timeouts.SlugStartup, err = c.parseTimeDurationValue(v)
	case "timeouts.ready-interval":
		c.Timeouts.ReadyInterval, err = c.parseTimeDurationValue(v)
	case "timeouts.origin-request":
		c.Timeouts.OriginRequest, err = c.parseTimeDurationValue(v)
	case "proxy-limit.ttl":
		c.ProxyLimit.TTL, err = c.parseTimeDurationValue(v)
	case "proxy-limit.max":
		c.ProxyLimit.Max, err = c.parseFloatValue(v)
	case "proxy-limit.burst":
		c.ProxyLimit.Burst, err = c.parseIntValue(v)
	case "proxy-limit.max-delay":
		c.ProxyLimit.MaxDelay, err = c.parseTimeDurationValue(v)
	case "proxy-limit.delay-scale":
		c.ProxyLimit.DelayScale, err = c.parseIntValue(v)
	case "proxy-limit.log-allowed":
		c.ProxyLimit.LogAllowed, err = c.parseBoolValue(v)
	case "proxy-limit.log-delayed":
		c.ProxyLimit.LogDelayed, err = c.parseBoolValue(v)
	case "proxy-limit.log-limited":
		c.ProxyLimit.LogLimited, err = c.parseBoolValue(v)
	case "run-as.user":
		c.RunAs.User, err = c.parseStringValue(v)
	case "run-as.group":
		c.RunAs.Group, err = c.parseStringValue(v)
	case "ports.git":
		c.Ports.Git, err = c.parsePortValue(v)
	case "ports.http":
		c.Ports.Http, err = c.parsePortValue(v)
	case "ports.https":
		c.Ports.Https, err = c.parsePortValue(v)
	case "ports.app-start":
		c.Ports.AppStart, err = c.parsePortValue(v)
	case "ports.app-end":
		c.Ports.AppEnd, err = c.parsePortValue(v)
	case "paths.etc":
		c.Paths.Etc, err = c.parseStringValue(v)
	case "paths.tmp":
		c.Paths.Tmp, err = c.parseStringValue(v)
	case "paths.var":
		c.Paths.Var, err = c.parseStringValue(v)
	default:
		err = fmt.Errorf("key not found")
	}
	return
}

func (c *Config) parseBoolValue(v string) (parsed bool, err error) {
	parsed, err = strconv.ParseBool(v)
	return
}

func (c *Config) parseFloatValue(v string) (parsed float64, err error) {
	parsed, err = strconv.ParseFloat(v, 64)
	return
}

func (c *Config) parseIntValue(v string) (parsed int, err error) {
	parsed, err = strconv.Atoi(v)
	return
}

func (c *Config) parseStringValue(v string) (parsed string, err error) {
	parsed = v
	return
}

func (c *Config) parsePortValue(v string) (parsed int, err error) {
	if parsed, err = c.parseIntValue(v); err == nil {
		if parsed < 1 || parsed > 65534 {
			err = fmt.Errorf("port out of range: 1 to 65534")
		}
	}
	return
}

func (c *Config) parseTimeDurationValue(v string) (parsed time.Duration, err error) {
	parsed, err = time.ParseDuration(v)
	return
}