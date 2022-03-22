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

package nodejs

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"

	beStrings "github.com/go-enjin/be/pkg/strings"
	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/cli/env"
	"github.com/go-enjin/be/pkg/cli/run"
	"github.com/go-enjin/be/pkg/context"
	"github.com/go-enjin/be/pkg/net"
	bePath "github.com/go-enjin/be/pkg/path"

	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/io"
	pkgRun "github.com/go-enjin/enjenv/pkg/run"
	"github.com/go-enjin/enjenv/pkg/system"
)

var (
	Tag          = "node"
	Name         = "nodejs"
	CacheDirName = "nodecache"
)

var (
	DefaultVersion = "16.14.0"
)

var (
	rxVersion = regexp.MustCompile(`^[v]??(\d+?\.\d+?\.\d+?)$`)
	rxTarName = regexp.MustCompile(`node-v(\d+?\.\d+?\.\d+?)-([a-z0-9]+?)-([a-z0-9]+?)\.tar\.gz`)
	rxShaSums = regexp.MustCompile(`(?ms)^([a-f0-9]{64})\s*([^\s]+?)\s*$`)
)

func init() {
	Tag = env.Get("ENJENV_NODEJS_TAG", Tag)
	tag := strings.ToUpper(Tag)
	Name = env.Get("ENJENV_"+tag+"_NAME", Name)
	CacheDirName = env.Get("ENJENV_"+tag+"_CACHE_DIR_NAME", CacheDirName)
	DefaultVersion = env.Get("ENJENV_DEFAULT_"+tag+"_VERSION", DefaultVersion)
}

type System struct {
	system.CSystem
}

func New() (s *System) {
	s = new(System)
	s.Init(s)
	return
}

func (s *System) Init(this interface{}) {
	s.CSystem.Init(this)
	s.Ctx = context.New()
	s.TagName = Name
	s.Version = DefaultVersion
	s.Url = "https://nodejs.org/dist"
	s.Root = fmt.Sprintf("%v/node-v%v-%v-x64", s.TagName, s.Version, runtime.GOOS)
	s.CSystem.Root = s.Root
	s.CSystem.TagName = s.TagName
	s.CSystem.Ctx = s.Ctx
	return
}

func (s *System) GetDefaultVersion() (version string) {
	return DefaultVersion
}

func (s *System) ExtraCommands(app *cli.App) (commands []*cli.Command) {
	commands = []*cli.Command{
		&cli.Command{
			HideHelpCommand: true,
			Name:            "node",
			Usage:           "wrapper for local bin/node",
			UsageText:       app.Name + " node -- [node arguments]",
			Action: func(ctx *cli.Context) (err error) {
				if err = s.Prepare(ctx); err != nil {
					return
				}
				argv := ctx.Args().Slice()
				if len(argv) > 0 {
					name := argv[0]
					args := argv[1:]
					_, err = s.NodeBin(name, args...)
					return
				}
				_, err = s.NodeBin("--help")
				return
			},
		},
		&cli.Command{
			HideHelpCommand: true,
			Name:            "npm",
			Usage:           "wrapper for local bin/npm",
			UsageText:       app.Name + " npm -- [npm arguments]",
			Action: func(ctx *cli.Context) (err error) {
				if err = s.Prepare(ctx); err != nil {
					return
				}
				argv := ctx.Args().Slice()
				if len(argv) > 0 {
					name := argv[0]
					args := argv[1:]
					_, err = s.NpmBin(name, args...)
					return
				}
				_, err = s.NpmBin("--help")
				return
			},
		},
		&cli.Command{
			HideHelpCommand: true,
			Name:            "npx",
			Usage:           "wrapper for local bin/npx",
			UsageText:       app.Name + " npx -- [npx arguments]",
			Action: func(ctx *cli.Context) (err error) {
				if err = s.Prepare(ctx); err != nil {
					return
				}
				argv := ctx.Args().Slice()
				if len(argv) > 0 {
					name := argv[0]
					args := argv[1:]
					_, err = s.NpxBin(name, args...)
					return
				}
				_, err = s.NpxBin("--help")
				return
			},
		},
		&cli.Command{
			HideHelpCommand: true,
			Name:            "yarn",
			Usage:           "wrapper for local yarn",
			UsageText:       app.Name + " yarn -- [yarn arguments]",
			Action: func(ctx *cli.Context) (err error) {
				if err = s.Prepare(ctx); err != nil {
					return
				}
				argv := ctx.Args().Slice()
				if len(argv) > 0 {
					name := argv[0]
					args := argv[1:]
					_, err = s.YarnBin(name, args...)
					return
				}
				_, err = s.YarnBin("--help")
				return
			},
		},
	}
	if scripts := s.MakeScriptCommands(app); scripts != nil {
		for _, script := range scripts {
			commands = append(commands, script)
		}
	}
	return
}

func (s *System) Prepare(ctx *cli.Context) (err error) {
	_ = io.SetupCustomIndent(ctx)

	if !RuntimeSupported() {
		err = fmt.Errorf("%v is not supported on %v/%v", Tag, runtime.GOOS, runtime.GOARCH)
		return
	}

	if err = s.CSystem.Prepare(ctx); err != nil {
		return
	}

	s.Root = fmt.Sprintf("%v/node-v%v-%v-x64", s.TagName, s.Version, runtime.GOOS)
	s.CSystem.Root = s.Root

	s.TarGz = fmt.Sprintf("node-v%v-%v-x64.tar.gz", s.Version, runtime.GOOS)
	s.TarGzPath = basepath.MakeEnjenvPath(s.TagName, s.TarGz)
	s.TarGzUrl = fmt.Sprintf("%v/v%v/%v", s.Url, s.Version, s.TarGz)
	/*
		// NODE_REPL_HISTORY?
		// NODE_CACHE NODE_ROOT
		${RUN} ${BUILD_BIN_PATH}/npm \
			--cache=${BUILD_PATH}/nodecache \
			--prefix="${NODE_ROOT}" \
			install \
			--global \
			yarn;
	*/
	s.Ctx.Set("NODE_ROOT", s.Root)
	s.Ctx.Set("NODE_CACHE", fmt.Sprintf("%v/%v", s.TagName, CacheDirName))
	// s.Ctx.Set("NODE_PATH", fmt.Sprintf("%v/lib/node_modules", s.Root))
	for k, v := range s.Ctx.AsMapStrings() {
		env.Set(k, basepath.MakeEnjenvPath(v))
	}
	env.Set("NODE_DISABLE_COLORS", "1")
	env.Set("NPM_DISABLE_COLORS", "1")
	env.Set("DISABLE_COLORS", "1")
	env.Set("FORCE_COLOR", "0")

	pkgRun.AddPathToEnv(basepath.MakeEnjenvPath(s.Root, "bin"))
	return
}

func (s *System) ExportString(ctx *cli.Context) (content string, err error) {
	path := basepath.MakeEnjenvPath(s.TagName)
	if bePath.IsDir(path) {
		content += fmt.Sprintf("export %v_VERSION=\"%v\"\n", strings.ToUpper(Tag), s.Version)
		for k, v := range s.Ctx.AsMapStrings() {
			value := basepath.MakeEnjenvPath(v)
			content += fmt.Sprintf("export %v=\"%v\"\n", k, value)
			env.Set(k, value)
		}
	}
	return
}

func (s *System) Export(ctx *cli.Context) (err error) {
	var content string
	if content, err = s.ExportString(ctx); err == nil {
		io.StdoutF(content)
	}
	return
}

func (s *System) UnExportString(ctx *cli.Context) (content string, err error) {
	path := basepath.MakeEnjenvPath(s.TagName)
	if bePath.IsDir(path) {
		content += fmt.Sprintf("unset %v_VERSION;\n", strings.ToUpper(Tag))
		for k, _ := range s.Ctx.AsMapStrings() {
			env.Set(k, "")
			content += fmt.Sprintf("unset %v;\n", k)
		}
	}
	return
}

func (s *System) UnExport(ctx *cli.Context) (err error) {
	var content string
	if content, err = s.UnExportString(ctx); err == nil {
		io.StdoutF(content)
	}
	return
}

func (s *System) GetInstalledVersion() (version string, err error) {
	path := basepath.MakeEnjenvPath(s.Root)
	if bePath.IsDir(path) {
		version = s.Version
		return
	}
	err = fmt.Errorf("NODE ROOT not found: %v\n", path)
	return
}

func (s *System) ParseVersionString(ver string) (version string, err error) {
	if !rxVersion.MatchString(ver) {
		err = fmt.Errorf("not a version string")
		return
	}
	m := rxVersion.FindAllStringSubmatch(ver, 1)
	version = m[0][1]
	return
}

func (s *System) ParseFileName(path string) (version, osName, osArch string, err error) {
	if !bePath.IsFile(path) {
		err = fmt.Errorf("file not found")
		return
	}
	if !rxTarName.MatchString(path) {
		err = fmt.Errorf("invalid nodejs archive name, see: %v", s.Url)
		return
	}
	m := rxTarName.FindAllStringSubmatch(path, 1)
	version = m[0][1]
	osName = m[0][2]
	osArch = m[0][3]
	return
}

func (s *System) GetKnownSums() (sums map[string]string, err error) {
	url := s.Url + "/v" + s.Version + "/SHASUMS256.txt"
	var content string
	if content, err = net.Get(url); err != nil {
		return
	}

	if !rxShaSums.MatchString(content) {
		err = fmt.Errorf("error parsing %v text content", url)
		return
	}

	matches := rxShaSums.FindAllStringSubmatch(content, -1)
	sums = make(map[string]string)
	for _, match := range matches {
		sums[match[2]] = match[1]
	}
	return
}

func (s *System) PostInitSystem(ctx *cli.Context) (err error) {
	io.StdoutF("# npm install --global yarn\n")
	if _, err = s.NpmBin("install", "--global", "yarn"); err == nil {
		if bin := basepath.MakeEnjenvPath(s.Root, "bin", "yarn"); bePath.IsFile(bin) {
			io.StdoutF("# yarn version: %v, path: %v\n", s.YarnVersion(), bin)
		} else {
			io.StderrF("# yarn not found, expected: %v\n", bin)
		}
	}
	return
}

func (s *System) NodeBin(name string, argv ...string) (status int, err error) {
	pkgRun.AddPathToEnv(basepath.MakeEnjenvPath(s.Root, "bin"))
	bin := basepath.MakeEnjenvPath(s.Root, "bin", "node")
	if !bePath.IsFile(bin) {
		err = fmt.Errorf("node not present")
		return
	}
	return run.Exe(bin, argv...)
}

func (s *System) NpmBin(name string, argv ...string) (status int, err error) {
	pkgRun.AddPathToEnv(basepath.MakeEnjenvPath(s.Root, "bin"))
	bin := basepath.MakeEnjenvPath(s.Root, "bin", "npm")
	if !bePath.IsFile(bin) {
		err = fmt.Errorf("npm not present")
		return
	}
	defCache := basepath.MakeEnjenvPath(s.Root, CacheDirName)
	nodeCache := s.Ctx.String("NODE_CACHE", defCache)
	nodeCache = basepath.MakeEnjenvPath(nodeCache)
	nodeRoot := basepath.MakeEnjenvPath(s.Root)
	args := []string{
		"--no-color",
		"--cache", nodeCache,
		"--prefix", nodeRoot,
		name,
	}
	return run.Exe(bin, append(args, argv...)...)
}

func (s *System) NpxBin(name string, argv ...string) (status int, err error) {
	pkgRun.AddPathToEnv(basepath.MakeEnjenvPath(s.Root, "bin"))
	bin := basepath.MakeEnjenvPath(s.Root, "bin", "npx")
	if !bePath.IsFile(bin) {
		err = fmt.Errorf("npx not present")
		return
	}
	defCache := basepath.MakeEnjenvPath(s.Root, CacheDirName)
	nodeCache := s.Ctx.String("NODE_CACHE", defCache)
	nodeCache = basepath.MakeEnjenvPath(nodeCache)
	nodeRoot := basepath.MakeEnjenvPath(s.Root)
	args := []string{
		"--no-color",
		"--cache", nodeCache,
		"--prefix", nodeRoot,
		name,
	}
	return run.Exe(bin, append(args, argv...)...)
}

func (s *System) YarnBin(name string, argv ...string) (status int, err error) {
	pkgRun.AddPathToEnv(basepath.MakeEnjenvPath(s.Root, "bin"))
	bin := basepath.MakeEnjenvPath(s.Root, "bin", "yarn")
	if !bePath.IsFile(bin) {
		err = fmt.Errorf("yarn not present")
		return
	}
	return run.Exe(bin, append([]string{name}, argv...)...)
}

func (s *System) YarnCmd(name string, argv ...string) (stdout, stderr string, status int, err error) {
	pkgRun.AddPathToEnv(basepath.MakeEnjenvPath(s.Root, "bin"))
	bin := basepath.MakeEnjenvPath(s.Root, "bin", "yarn")
	if !bePath.IsFile(bin) {
		err = fmt.Errorf("yarn not present")
		return
	}
	return run.Cmd(bin, append([]string{name}, argv...)...)
}

func (s *System) YarnVersion() (version string) {
	if o, _, _, err := s.YarnCmd("--version"); err == nil {
		version = strings.TrimSpace(o)
		return
	}
	return
}

var rxPackageScripts = regexp.MustCompile(`(?ms)"scripts"\s*:\s*{(.+?)},??`)
var rxPackageScriptsLines = regexp.MustCompile(`(?ms)"([^"]+?)"\s*:\s*"\s*([^"]+?)\s*"\s*,??`)

func (s *System) ListPackageDirs(path string) (dirs []string, err error) {
	if paths, err := bePath.ListDirs("."); err == nil {
		for _, dir := range paths {
			if dir[0:2] == "./" {
				dir = dir[2:]
			}
			switch dir {
			case ".git", ".svn", "CVS":
				continue
			}
			if dirPkg := dir + "/package.json"; bePath.IsFile(dirPkg) {
				dirs = append(dirs, dir)
			}
		}
	}
	return
}

func (s *System) PackageJsonsPresent(dirs []string) (packageJsons []string) {
	if bePath.IsFile("package.json") {
		packageJsons = append(packageJsons, "package.json")
	}
	for _, dir := range dirs {
		packageJsons = append(packageJsons, dir+"/package.json")
	}
	return
}

func (s *System) PackagesWithoutModules(dirs []string) (withoutModules []string) {
	if bePath.IsFile("package.json") {
		if !bePath.IsDir("node_modules") {
			withoutModules = append(withoutModules, ".")
		}
	}
	for _, dir := range dirs {
		if !bePath.IsDir(dir + "/node_modules") {
			withoutModules = append(withoutModules, dir)
		}
	}
	return
}

func (s *System) MakeScriptCommands(app *cli.App) (commands []*cli.Command) {
	if _, err := s.GetInstalledVersion(); err != nil {
		return
	}

	var iniPaths []string
	var pkgPaths []string

	if dirs, err := s.ListPackageDirs("."); err == nil {
		iniPaths = s.PackagesWithoutModules(dirs)
		pkgPaths = s.PackageJsonsPresent(dirs)
	}

	packages := make(map[string]map[string]string)

	for _, pkg := range pkgPaths {
		dir := bePath.Base(bePath.Dir(pkg))
		if dir == "" {
			dir, _ = bePath.Abs(pkg)
			dir = bePath.Base(bePath.Dir(dir))
		}
		if beStrings.StringInStrings(dir, iniPaths...) {
			continue
		}
		if contentBytes, err := bePath.ReadFile(pkg); err == nil {
			content := string(contentBytes)
			if rxPackageScripts.MatchString(content) {
				if m := rxPackageScripts.FindAllStringSubmatch(content, 1); len(m) == 1 {
					scripting := m[0][1]
					if rxPackageScriptsLines.MatchString(scripting) {
						if ml := rxPackageScriptsLines.FindAllStringSubmatch(scripting, -1); len(ml) > 0 {
							if _, ok := packages[dir]; !ok {
								packages[dir] = make(map[string]string)
							}
							for _, mli := range ml {
								name, value := mli[1], mli[2]
								packages[dir][name] = value
							}
						}
					}
				}
			}
		}
	}

	pms := []string{"npm"}
	if s.YarnVersion() != "" {
		pms = []string{"yarn"}
	}

	for _, pm := range pms {
		if len(iniPaths) > 0 {
			for _, dir := range iniPaths {
				dirName := dir
				if dirName == "." {
					absDir, _ := bePath.Abs(".")
					dirName = bePath.Base(absDir)
				}
				cmdCategory := s.Name() + " " + system.SystemCategory + " " + dirName
				commands = append(
					commands,
					&cli.Command{
						Name:      pm + "-" + dirName + "--install",
						Usage:     fmt.Sprintf("%v install node_modules for %v", pm, dirName),
						UsageText: app.Name + " " + pm + "-" + dirName + "--install",
						Category:  cmdCategory,
						Action:    s.makePackageInstallFunc(pm, dir),
					},
				)
			}
		}

		if len(packages) > 0 {
			for dir, scripts := range packages {
				cmdCategory := s.Name() + " " + system.SystemCategory + " " + dir

				usageText := app.Name + " " + dir + " script [scripts...]"
				var names []string
				for n, _ := range scripts {
					names = append(names, n)
				}
				if len(names) > 0 {
					usageText = "\n\t" + usageText + "\n"
					usageText += "\n\t# available script targets:"
					sort.Strings(names)
					for _, n := range names {
						usageText += "\n\t" + app.Name + " " + dir + " " + n
					}
				}

				commands = append(
					commands,
					&cli.Command{
						Name:      pm + "-" + dir,
						Usage:     fmt.Sprintf("run one or more %v scripts in sequence, aborting on first error", pm),
						UsageText: usageText,
						Category:  cmdCategory,
						Action:    s.makePackageSystemFunc(pm, dir, scripts),
					},
					&cli.Command{
						Name:      dir + "-" + pm,
						Usage:     fmt.Sprintf("run %v (from %v)", pm, dir),
						UsageText: app.Name + " " + dir + "-" + pm + " -- [yarn arguments]",
						Category:  cmdCategory,
						Action: func(ctx *cli.Context) (err error) {
							if err = s.Prepare(ctx); err != nil {
								return
							}
							argv := ctx.Args().Slice()
							err = s.runPackageCommand("yarn", dir, argv[0], argv[1:]...)
							return
						},
					},
					&cli.Command{
						Name:      dir + "-" + pm + "-audit-report",
						Usage:     fmt.Sprintf("run %v audit (from %v) and report the results", pm, dir),
						UsageText: app.Name + " " + dir + "-" + pm + "-audit-report",
						Category:  cmdCategory,
						Action: func(ctx *cli.Context) (err error) {
							if err = s.Prepare(ctx); err != nil {
								return
							}
							o, _, _ := pkgRun.EnjenvCmd(dir+"-"+pm, "audit")
							msg := ""
							for _, line := range strings.Split(o, "\n") {
								if strings.Contains(line, "Packages audited") {
									msg += strings.TrimSpace(line)
								} else if strings.Contains(line, "Severity:") {
									if msg != "" {
										msg += "; "
									}
									msg += strings.TrimSpace(line)
								}
							}
							io.NotifyF(dir+"-"+pm+"-audit", msg)
							return
						},
					},
				)

				for name, script := range scripts {
					cmdName := pm + "-" + dir + "-" + name
					cmdUsage := fmt.Sprintf("run the %v %v (%v) script", dir, name, pm)
					cmdUsageText := fmt.Sprintf(
						"\n\t%v %v -- [%v options]\n\n\t# execute actual commands: %v\n\t%v %v",
						app.Name, cmdName, name, script, app.Name, cmdName,
					)

					commands = append(
						commands,
						&cli.Command{
							Name:      cmdName,
							Category:  cmdCategory,
							Usage:     cmdUsage,
							UsageText: cmdUsageText,
							Action:    s.makePackageScriptFunc(pm, dir, name),
						},
					)
				}
			}
		}
	}
	return
}

func (s *System) makePackageInstallFunc(p, d string) func(ctx *cli.Context) (err error) {
	return func(ctx *cli.Context) (err error) {
		if err = s.Prepare(ctx); err != nil {
			return
		}
		wd := bePath.Pwd()
		if d != "." {
			_ = os.Chdir(d)
			io.NotifyF("nodejs", "running %v install (%v)", p, d)
		} else {
			io.NotifyF("nodejs", "running %v install (./)", p)
		}
		if p == "yarn" {
			_, err = s.YarnBin("install")
		} else {
			_, err = s.NpmBin("install")
		}
		if d != "." {
			_ = os.Chdir(wd)
		}
		return
	}
}

func (s *System) makePackageScriptFunc(p, d, n string) func(ctx *cli.Context) (err error) {
	// wtf go lol, when embedding these funcs within a loop making the *cli.Command instances,
	// go uses the same function address each time, thus though there have been multiple commands
	// created, they all amount to invoking just the last one added regardless of which one was
	// actually invoked, the solution is to "make" the return func middleware way used in net/http
	return func(ctx *cli.Context) (err error) {
		if err = s.Prepare(ctx); err != nil {
			return
		}
		err = s.runPackageScript(p, d, n, ctx.Args().Slice()...)
		return
	}
}

func (s *System) runPackageCommand(pm, dir, name string, argv ...string) (err error) {
	wd := bePath.Pwd()
	switch dir {
	case "npm", "yarn":
	default:
		_ = os.Chdir(dir)
	}
	if pm == "yarn" {
		_, err = s.YarnBin(name, argv...)
	} else {
		argv = append([]string{name}, argv...)
		_, err = s.NpmBin("run", argv...)
	}
	switch dir {
	case "npm", "yarn":
	default:
		_ = os.Chdir(wd)
	}
	return
}

func (s *System) runPackageScript(pm, dir, name string, argv ...string) (err error) {
	wd := bePath.Pwd()
	switch dir {
	case "npm", "yarn":
	default:
		_ = os.Chdir(dir)
	}
	io.NotifyF("nodejs", "running %v %v", pm, name)
	if pm == "yarn" {
		_, err = s.YarnBin(name, argv...)
	} else {
		argv = append([]string{name}, argv...)
		_, err = s.NpmBin("run", argv...)
	}
	switch dir {
	case "npm", "yarn":
	default:
		_ = os.Chdir(wd)
	}
	return
}

func (s *System) makePackageSystemFunc(p, d string, scripts map[string]string) func(ctx *cli.Context) (err error) {
	return func(ctx *cli.Context) (err error) {
		if err = s.Prepare(ctx); err != nil {
			return
		}
		err = s.runPackageSystem(ctx, p, d, scripts)
		return
	}
}

func (s *System) runPackageSystem(ctx *cli.Context, pm, dir string, scripts map[string]string) (err error) {
	if err = s.Prepare(ctx); err != nil {
		return
	}
	var targets []string
	argv := ctx.Args().Slice()
	if len(argv) == 0 {
		cli.ShowCommandHelpAndExit(ctx, dir, 1)
	}
	for _, arg := range argv {
		if _, ok := scripts[arg]; !ok {
			err = fmt.Errorf("%v is not a valid %v script name", arg, pm)
			return
		}
		targets = append(targets, arg)
	}
	for _, target := range targets {
		if err = s.runPackageScript(pm, dir, target); err != nil {
			return
		}
	}
	return
}