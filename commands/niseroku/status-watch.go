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
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/go-enjin/be/pkg/maths"
	"github.com/go-enjin/be/pkg/slices"

	"github.com/dustin/go-humanize"
	"github.com/urfave/cli/v2"

	"github.com/go-curses/cdk"
	cenums "github.com/go-curses/cdk/lib/enums"
	"github.com/go-curses/cdk/lib/math"
	"github.com/go-curses/cdk/lib/paint"
	cstrings "github.com/go-curses/cdk/lib/strings"
	"github.com/go-curses/cdk/log"
	"github.com/go-curses/ctk"
	"github.com/go-curses/ctk/lib/enums"

	"github.com/go-enjin/be/pkg/maps"
	beStrings "github.com/go-enjin/be/pkg/strings"

	"github.com/go-enjin/enjenv/pkg/cpuinfo"
	"github.com/go-enjin/enjenv/pkg/globals"
	beIo "github.com/go-enjin/enjenv/pkg/io"
)

//go:embed status-watch.accelmap
var statusWatchAccelmap string

// Build Configuration Flags
// setting these will enable command line flags and their corresponding features
// use `go build -v -ldflags="-X 'github.com/go-enjin/enjenv/commands/niseroku.CdkIncludeLogFullPaths=false'"`
var (
	CdkIncludeProfiling     = "false"
	CdkIncludeLogFile       = "false"
	CdkIncludeLogFormat     = "false"
	CdkIncludeLogFullPaths  = "false"
	CdkIncludeLogLevel      = "false"
	CdkIncludeLogLevels     = "false"
	CdkIncludeLogTimestamps = "false"
	CdkIncludeLogOutput     = "false"
)

var (
	DefaultStatusWatchTtyPath = "/dev/tty"
	rxStripFloats             = regexp.MustCompile(`\.\d+`)
)

func init() {
	cdk.Build.Profiling = cstrings.IsTrue(CdkIncludeProfiling)
	cdk.Build.LogFile = cstrings.IsTrue(CdkIncludeLogFile)
	cdk.Build.LogFormat = cstrings.IsTrue(CdkIncludeLogFormat)
	cdk.Build.LogFullPaths = cstrings.IsTrue(CdkIncludeLogFullPaths)
	cdk.Build.LogLevel = cstrings.IsTrue(CdkIncludeLogLevel)
	cdk.Build.LogLevels = cstrings.IsTrue(CdkIncludeLogLevels)
	cdk.Build.LogTimestamps = cstrings.IsTrue(CdkIncludeLogTimestamps)
	cdk.Build.LogOutput = cstrings.IsTrue(CdkIncludeLogOutput)
	if cdk.Build.LogFile {
		log.DefaultLogPath = filepath.Join(os.TempDir(), "niseroku-status-watch.cdk.log")
	} else {
		log.DefaultLogPath = "/dev/null"
	}
}

type StatusWatch struct {
	cliCmd *Command
	ctkApp ctk.Application

	display   cdk.Display
	window    ctk.Window
	errDialog ctk.Dialog

	sysFrame    ctk.Frame
	sysLabelCPU ctk.Label
	sysValueCPU ctk.Label
	sysLabelMEM ctk.Label
	sysValueMEM ctk.Label

	srvFrame  ctk.Frame
	srvHeader ctk.Label
	srvScroll ctk.ScrolledViewport
	srvLabel  ctk.Label

	appFrame  ctk.Frame
	appHeader ctk.Label
	appScroll ctk.ScrolledViewport
	appLabel  ctk.Label

	plFrame  ctk.Frame
	plHeader ctk.Label
	plScroll ctk.ScrolledViewport
	plLabel  ctk.Label

	errLabel    ctk.Label
	statusLabel ctk.Label

	freq     time.Duration
	watching *Watching
	started  bool

	sync.RWMutex
}

func NewStatusWatch(cmd *Command, freq time.Duration, ttyPath string) (sw *StatusWatch, err error) {
	if ttyPath == "" {
		ttyPath = DefaultStatusWatchTtyPath
	}

	app := ctk.NewApplication(
		"niseroku-status-watch",
		"Niseroku status watch",
		"Basically 'top' for niseroku services and applications",
		globals.BuildVersion,
		"niseroku-status-watch",
		fmt.Sprintf("niseroku status watch - %v", globals.DisplayVersion),
		ttyPath,
	)
	if cliApp := app.CLI(); cliApp != nil {
		cliApp.UsageText = "enjenv niseroku status watch [options] -- [curses options]"
		cliApp.Flags = append(
			cliApp.Flags,
			[]cli.Flag{
				&cli.DurationFlag{
					Name:    "update-frequency",
					Usage:   "time.Duration between update cycles",
					Aliases: []string{"n"},
				},
			}...,
		)
	}
	sw = &StatusWatch{
		cliCmd: cmd,
		ctkApp: app,
		freq:   freq,
	}
	return
}

func (sw *StatusWatch) Run(ctx *cli.Context) (err error) {
	sw.ctkApp.Connect(cdk.SignalPrepare, "status-watch-prepare-handler", sw.prepare)
	sw.ctkApp.Connect(cdk.SignalStartup, "status-watch-startup-handler", sw.startup)
	sw.ctkApp.Connect(cdk.SignalShutdown, "status-watch-quit-handler", sw.shutdown)
	err = sw.ctkApp.Run(append([]string{"enjenv--niseroku--status--watch"}, ctx.Args().Slice()...))
	return
}

func (sw *StatusWatch) prepare(data []interface{}, argv ...interface{}) cenums.EventFlag {
	var ok bool
	var ctx *cli.Context
	if len(argv) >= 2 {
		if ctx, ok = argv[1].(*cli.Context); !ok {
			beIo.STDERR("internal error\n")
			return cenums.EVENT_STOP
		}
	} else {
		beIo.STDERR("internal error\n")
		return cenums.EVENT_STOP
	}
	freq := sw.freq
	if ctx.IsSet("update-frequency") {
		freq = ctx.Duration("update-frequency")
	}
	var err error
	if sw.watching, err = NewWatching(sw.cliCmd.config, freq, sw.update); err != nil {
		beIo.STDERR("error with new watching: %v\n", err)
		return cenums.EVENT_STOP
	}
	return cenums.EVENT_PASS
}

func (sw *StatusWatch) quitAction(argv ...interface{}) (handled bool) {
	sw.watching.Stop()
	sw.window.LogDebug("quit-accelerator called (ctrl+q)")
	sw.display.RequestQuit()
	handled = true
	return
}

func (sw *StatusWatch) showErrorWindow(text string) {
	if sw.errDialog != nil {
		if label, ok := sw.errDialog.GetContentArea().GetChildren()[0].(ctk.Label); ok {
			_ = label.SetMarkup(text)
		}
		sw.errDialog.Resize()
		sw.display.RequestDraw()
		sw.display.RequestShow()
		return
	}
	sw.errDialog = ctk.NewDialogWithButtons(
		"niseroku status watch", nil, enums.DialogModal,
	)
	vbox := sw.errDialog.GetContentArea()
	label := ctk.NewLabel(text)
	label.SetJustify(cenums.JUSTIFY_CENTER)
	label.SetLineWrapMode(cenums.WRAP_WORD)
	label.SetLineWrap(true)
	label.Show()
	vbox.PackStart(label, true, true, 1)
	sw.errDialog.ShowAll()
	sw.display.RequestDraw()
	sw.display.RequestShow()
}

func (sw *StatusWatch) hideErrorWindow() {
	sw.Lock()
	defer sw.Unlock()
	if sw.errDialog != nil {
		sw.errDialog.Hide()
		sw.errDialog.Destroy()
		sw.errDialog = nil
		sw.display.RequestDraw()
		sw.display.RequestShow()
	}
}

func (sw *StatusWatch) displayResizeHandler(data []interface{}, argv ...interface{}) cenums.EventFlag {
	if s := sw.display.Screen(); s != nil {
		if w, h := s.Size(); w < 80 || h < 24 {
			text := "\n\nniseroku status watch requires at least an 80x24 sized terminal"
			text += "\n\n"
			text += fmt.Sprintf("this terminal is %dx%d", w, h)
			sw.showErrorWindow(text)
		} else {
			sw.hideErrorWindow()
		}
	}
	return cenums.EVENT_PASS
}

func (sw *StatusWatch) startup(data []interface{}, argv ...interface{}) cenums.EventFlag {
	if app, d, _, _, _, ok := ctk.ArgvApplicationSignalStartup(argv...); ok {
		d.CaptureCtrlC()
		sw.display = d
		sw.display.Connect(cdk.SignalEventResize, "display-resize-handler", sw.displayResizeHandler)

		if err := sw.watching.Start(); err != nil {
			sw.showError("error starting watching: %v", err)
		}

		sw.window = ctk.NewWindowWithTitle("")
		sw.window.SetBorderWidth(0)
		sw.window.SetDecorated(false)
		sw.window.SetSensitive(true)
		sw.window.SetTitle(app.Title())

		accelMap := ctk.GetAccelMap()
		accelMap.LoadFromString(statusWatchAccelmap)

		ag := ctk.NewAccelGroup()
		ag.ConnectByPath("<Status-Watch-Window>/File/Quit", "quit-accel", sw.quitAction)
		ag.AccelConnect(cdk.KeySmallQ, cdk.ModCtrl, enums.ACCEL_VISIBLE, "quit-accel-ctrl-q", sw.quitAction)
		sw.window.AddAccelGroup(ag)

		windowVBox := sw.window.GetVBox()
		windowVBox.SetHomogeneous(false)

		contentVBox := ctk.NewVBox(false, 0)
		contentVBox.Show()
		windowVBox.PackStart(contentVBox, true, true, 0)

		rigLabel := func(text string) (l ctk.Label) {
			l, _ = ctk.NewLabelWithMarkup(text)
			l.SetLineWrap(false)
			l.SetLineWrapMode(cenums.WRAP_NONE)
			l.SetJustify(cenums.JUSTIFY_NONE)
			return
		}

		/* SYSTEM SECTION */

		sysFrameLabel := rigLabel("")
		sflTheme := sysFrameLabel.GetTheme()
		sflTheme.Content.FillRune = paint.DefaultNilRune
		sysFrameLabel.SetTheme(sflTheme)
		sysFrameLabel.SetUseMarkup(true)
		_ = sysFrameLabel.SetMarkup(dim("Host Metrics"))
		sysFrameLabel.Show()
		sw.sysFrame = ctk.NewFrameWithWidget(sysFrameLabel)
		sw.sysFrame.SetLabelAlign(0.0, 0.5)
		sw.sysFrame.SetSizeRequest(-1, 4)
		sw.sysFrame.Show()
		sysHBox := ctk.NewHBox(false, 1)
		sysHBox.Show()

		sysVBoxLabels := ctk.NewVBox(false, 0)
		sysVBoxLabels.Show()
		sysVBoxValues := ctk.NewVBox(false, 0)
		sysVBoxValues.Show()

		sw.sysLabelCPU = rigLabel("CPU:")
		sw.sysLabelCPU.SetSizeRequest(15, 1)
		sw.sysLabelCPU.Show()
		sysVBoxLabels.PackStart(sw.sysLabelCPU, false, false, 0)
		sw.sysValueCPU = rigLabel("")
		sw.sysValueCPU.SetSizeRequest(-1, 1)
		sw.sysValueCPU.Show()
		sysVBoxValues.PackEnd(sw.sysValueCPU, true, true, 0)

		sw.sysLabelMEM = rigLabel("MEM:")
		sw.sysLabelMEM.SetSizeRequest(15, 1)
		sw.sysLabelMEM.Show()
		sysVBoxLabels.PackStart(sw.sysLabelMEM, false, false, 0)
		sw.sysValueMEM = rigLabel("")
		sw.sysValueMEM.SetSizeRequest(-1, 1)
		sw.sysValueMEM.Show()
		sysVBoxValues.PackEnd(sw.sysValueMEM, false, false, 0)

		sw.sysFrame.Add(sysHBox)
		sysHBox.PackStart(sysVBoxLabels, false, true, 0)
		sysHBox.PackStart(sysVBoxValues, true, true, 0)
		contentVBox.PackStart(sw.sysFrame, false, true, 0)

		/* SERVICES SECTION */

		sw.srvHeader = rigLabel("")
		sw.srvHeader.SetUseMarkup(true)
		sw.srvHeader.SetSizeRequest(-1, 1)
		sw.srvHeader.Show()
		sw.srvFrame = ctk.NewFrameWithWidget(sw.srvHeader)
		sw.srvFrame.SetLabelAlign(0.0, -1.0)
		sw.srvFrame.SetSizeRequest(-1, 5)
		sw.srvFrame.Show()
		sw.srvLabel = rigLabel("")
		sw.srvLabel.SetSizeRequest(-1, 3)
		sw.srvLabel.SetUseMarkup(true)
		sw.srvLabel.Show()
		sw.srvScroll = ctk.NewScrolledViewport()
		sw.srvScroll.SetPolicy(enums.PolicyAutomatic, enums.PolicyAutomatic)
		sw.srvScroll.Show()
		sw.srvScroll.Add(sw.srvLabel)
		sw.srvFrame.Add(sw.srvScroll)
		contentVBox.PackStart(sw.srvFrame, false, true, 0)

		sw.appHeader = rigLabel("")
		sw.appHeader.SetUseMarkup(true)
		sw.appHeader.SetSizeRequest(-1, 1)
		sw.appHeader.Show()
		sw.appFrame = ctk.NewFrameWithWidget(sw.appHeader)
		sw.appFrame.SetLabelAlign(0.0, -1.0)
		sw.appFrame.Show()
		sw.appLabel = rigLabel("")
		sw.appLabel.SetSizeRequest(-1, -1)
		sw.appLabel.SetUseMarkup(true)
		sw.appLabel.Show()
		sw.appScroll = ctk.NewScrolledViewport()
		sw.appScroll.SetPolicy(enums.PolicyAutomatic, enums.PolicyAutomatic)
		// sw.appScroll.SetPolicy(enums.PolicyAlways, enums.PolicyAlways)
		sw.appScroll.Show()
		sw.appScroll.Add(sw.appLabel)
		sw.appFrame.Add(sw.appScroll)
		contentVBox.PackStart(sw.appFrame, true, true, 0)

		sw.plHeader = rigLabel("")
		sw.plHeader.SetUseMarkup(true)
		sw.plHeader.SetSizeRequest(-1, 1)
		sw.plHeader.Show()
		sw.plFrame = ctk.NewFrameWithWidget(sw.plHeader)
		// sw.plFrame = ctk.NewFrame("Proxy Limits (Remote IP Addresses)")
		sw.plFrame.SetLabelAlign(0.0, -1.0)
		sw.plFrame.Hide()
		sw.plLabel = rigLabel("")
		sw.plFrame.Add(sw.plLabel)
		sw.plScroll = ctk.NewScrolledViewport()
		sw.plScroll.SetPolicy(enums.PolicyAutomatic, enums.PolicyAutomatic)
		sw.plScroll.Show()
		sw.plScroll.Add(sw.plLabel)
		sw.plFrame.Add(sw.plScroll)
		contentVBox.PackStart(sw.plFrame, false, true, 0)

		sw.errLabel = rigLabel("")
		sw.errLabel.Hide()
		windowVBox.PackStart(sw.errLabel, false, false, 0)

		sw.statusLabel = rigLabel("")
		sw.statusLabel.SetUseMarkup(true)
		sw.statusLabel.SetLineWrap(true)
		sw.statusLabel.SetLineWrapMode(cenums.WRAP_WORD)
		sw.statusLabel.SetVisible(true)
		sw.statusLabel.SetJustify(cenums.JUSTIFY_CENTER)
		sw.statusLabel.Show()
		windowVBox.PackStart(sw.statusLabel, false, false, 0)

		sw.window.GrabFocus()
		sw.window.ShowAll()
		sw.errLabel.Hide()
		sw.plFrame.Hide()
		sw.Lock()
		sw.started = true
		sw.Unlock()
		sw.update()
		app.NotifyStartupComplete()
		return cenums.EVENT_PASS
	}
	return cenums.EVENT_STOP
}

func (sw *StatusWatch) showError(format string, argv ...interface{}) {
	message := fmt.Sprintf(format, argv...)
	sw.Lock()
	_ = sw.errLabel.SetMarkup(message)
	sw.errLabel.Show()
	sw.window.GetVBox().Resize()
	sw.display.RequestDraw()
	sw.display.RequestShow()
	sw.Unlock()
	go func() {
		<-time.After(4 * time.Second)
		sw.Lock()
		if sw.errLabel.IsVisible() {
			_ = sw.errLabel.SetMarkup("")
			sw.errLabel.Hide()
		}
		sw.Unlock()
	}()
}

func (sw *StatusWatch) shutdown(_ []interface{}, _ ...interface{}) cenums.EventFlag {
	beIo.STDOUT("niseroku status watch has ended.\n")
	return cenums.EVENT_PASS
}

func (sw *StatusWatch) update() {
	sw.RLock()
	if !sw.started {
		sw.RUnlock()
		return
	}
	sw.RUnlock()
	sw.window.LockDraw()
	sw.refresh()
	sw.window.UnlockDraw()
	sw.display.RequestDraw()
	sw.display.RequestShow()
}

func (sw *StatusWatch) gather() (snapshot *WatchSnapshot, proxyLimits string, err error) {
	if err = sw.cliCmd.config.Reload(); err != nil {
		return
	}
	if response, ee := sw.cliCmd.config.CallProxyControlCommand("proxy-limits"); ee == nil {
		proxyLimits = response
	}
	snapshot = sw.watching.Snapshot()
	return
}

func (sw *StatusWatch) refresh() {
	if snapshot, proxyLimits, err := sw.gather(); err != nil {
		sw.showError(fmt.Sprintf("error gathering watching data: %v", err))
	} else {
		sw.refreshWatching(snapshot, proxyLimits)
	}
}

func formatPercFloat[T float32 | float64](stat T, format string) (text string) {
	switch {
	case stat > 95.0:
		text = fmt.Sprintf(`<span foreground="white" background="red">`+format+`</span>`, stat)
	case stat > 75.0:
		text = fmt.Sprintf(`<span foreground="red" background="yellow">`+format+`</span>`, stat)
	case stat > 50.0:
		text = fmt.Sprintf(`<span foreground="black" background="yellow">`+format+`</span>`, stat)
	case stat > 25.0:
		text = fmt.Sprintf(`<span foreground="yellow">`+format+`</span>`, stat)
	default:
		text = fmt.Sprintf(`<span foreground="white">`+format+`</span>`, stat)
	}
	return
}

func formatMemUsed(kbUsed uint64, max uint64) (text string) {
	size := humanize.Bytes(kbUsed * 1024)
	// percent := fmt.Sprintf("%05.1f", float64(kbUsed)/float64(max)*100.0)
	// percent := fmt.Sprintf("%05.1f", float64(kbUsed)/float64(max)*100.0)
	switch {
	case kbUsed >= max/2:
		text = fmt.Sprintf(`<span foreground="white" background="red">%s</span>`, size)
	case kbUsed > (1024 * 512):
		text = fmt.Sprintf(`<span foreground="yellow">%s</span>`, size)
	default:
		text = fmt.Sprintf(`<span foreground="white">%s</span>`, size)
	}
	return
}

func formatPercNumber[T maths.Number](stat T, max float64) (text string) {
	switch {
	case float64(stat) >= max:
		text = fmt.Sprintf(`<span foreground="white" background="red">%d</span>`, int64(stat))
	case float64(stat) > max/2:
		text = fmt.Sprintf(`<span foreground="yellow">%d</span>`, int64(stat))
	default:
		text = fmt.Sprintf(`<span foreground="white">%d</span>`, int64(stat))
	}
	return
}

func dim(s string) string {
	return `<span weight="dim">` + s + `</span>`
}

func tDim(s string, n int) (out string) {
	tmpl := dim(s)
	for i := 0; i < n; i++ {
		if i > 0 {
			out += "\t"
		}
		out += tmpl
	}
	return
}

func (sw *StatusWatch) refreshWatching(snapshot *WatchSnapshot, proxyLimits string) {
	stats := &snapshot.Stats
	ppl := parseProxyLimits(proxyLimits)

	var cpuUsage float32 = 0.0
	if len(stats.CpuUsage) > 0 {
		for _, usage := range stats.CpuUsage {
			cpuUsage += usage
		}
		cpuUsage = cpuUsage / float32(len(stats.CpuUsage)) * 100.0
	}

	memUsed := strings.ReplaceAll(humanize.Bytes(stats.MemUsed*1024), " ", "")
	memTotal := strings.ReplaceAll(humanize.Bytes(stats.MemTotal*1024), " ", "")

	swpUsed := strings.ReplaceAll(humanize.Bytes(stats.SwapUsed*1024), " ", "")
	swpTotal := strings.ReplaceAll(humanize.Bytes(stats.SwapTotal*1024), " ", "")

	totUsed := strings.ReplaceAll(humanize.Bytes((stats.MemUsed+stats.SwapUsed)*1024), " ", "")
	totTotal := strings.ReplaceAll(humanize.Bytes((stats.MemTotal+stats.SwapTotal)*1024), " ", "")
	totPerc := float64(stats.MemUsed+stats.SwapUsed) / float64(stats.MemTotal+stats.SwapTotal) * 100.0

	/* STATUS ROW */
	statusFields := []string{
		globals.OsHostname,
		"Uptime: " + stats.UptimeString(),
		"Every: " + sw.freq.String(),
		`[F10] to quit`,
	}

	_ = sw.statusLabel.SetMarkup(strings.Join(statusFields, " "+dim("~")+" "))

	/* SYSTEM SECTION */

	_ = sw.sysLabelCPU.SetMarkup(fmt.Sprintf("CPU: %s%%", formatPercFloat(cpuUsage, "%.2f")))
	buf := bytes.NewBuffer([]byte(""))
	tw := tabwriter.NewWriter(io.Writer(buf), 8, 2, 2, ' ', tabwriter.FilterHTML)
	_, _ = tw.Write([]byte(dim("Core: [") + "\t"))
	for idx, usage := range stats.CpuUsage {
		if idx > 0 {
			_, _ = tw.Write([]byte("\t"))
		}
		if idx > 3 {
			_, _ = tw.Write([]byte(dim("...")))
			break
		} else {
			_, _ = tw.Write([]byte(formatPercFloat(usage*100.0, "%.2f") + "%"))
		}
	}
	_, _ = tw.Write([]byte(fmt.Sprintf("\t" + dim("]") + "\n")))
	_ = tw.Flush()
	_ = sw.sysValueCPU.SetMarkup(buf.String())
	_ = sw.sysLabelMEM.SetMarkup(fmt.Sprintf("MEM: %s%%", formatPercFloat(totPerc, "%.2f")))

	buf = bytes.NewBuffer([]byte(""))
	tw = tabwriter.NewWriter(io.Writer(buf), 8, 2, 2, ' ', tabwriter.FilterHTML)

	var memInfo string
	memInfo += dim("Total:") + " " + totUsed + " / " + totTotal
	memInfo += "\t"
	memInfo += dim("Real:") + " " + memUsed + " / " + memTotal
	memInfo += "\t"
	memInfo += dim("Swap:") + " " + swpUsed + " / " + swpTotal

	_, _ = tw.Write([]byte(memInfo))
	_ = tw.Flush()
	_ = sw.sysValueMEM.SetMarkup(buf.String())

	/* SERVICES SECTION */

	formatReqDelay := func(requests, delayed int64) (reqDelay string) {
		reqDelay = formatPercNumber(requests, sw.cliCmd.config.ProxyLimit.Max)
		if delayed > 0 {
			delay := formatPercNumber(delayed, sw.cliCmd.config.ProxyLimit.Max)
			reqDelay += " (" + delay + ")"
		}
		return
	}

	writeEntry := func(tw *tabwriter.Writer, appName, commitId string, stat WatchProc, requests, delayed int64, includeHash bool) {
		pid, ports, nice, cpu, mem, numThreads, reqDelay, uptime := dim("-"), dim("-"), dim("-"), dim("-"), dim("-"), dim("-/-"), dim("-"), dim("-")
		if stat.Pid > 0 {
			pid = strconv.Itoa(stat.Pid)
			nice = fmt.Sprintf("%+2d", stat.Nice-20)
			cpu = formatPercFloat(stat.Cpu, "%.2f")
			mem = formatMemUsed(stat.Mem, stats.MemTotal)
			num := fmt.Sprintf("%d", stat.Num)
			threads := fmt.Sprintf("%d", stat.Threads)
			// created := (stat.Created / uint64(cpuinfo.ClockTicks())) + cpuinfo.BootEpoch()
			created := stat.Created / uint64(cpuinfo.ClockTicks())
			// d := time.Unix(int64(created), 0)
			if created > 0 {
				d := cpuinfo.BootTime().Add(time.Duration(int64(time.Second) * int64(created)))
				uptime = fmt.Sprintf("%v", time.Now().Sub(d))
			}
			if rxStripFloats.MatchString(uptime) {
				uptime = rxStripFloats.ReplaceAllString(uptime, "")
			}
			numThreads = num + dim("/") + threads
			reqDelay = formatReqDelay(requests, delayed)
		}
		if len(stat.Ports) > 0 {
			ports = ""
			for idx, port := range stat.Ports {
				if idx > 0 {
					ports += ","
				}
				ports += strconv.Itoa(port)
			}
		}
		if slices.Present(stat.Name, "reverse-proxy", "git-repository") {
			commitId = globals.BuildBinHash[:8]
		}
		_, _ = tw.Write([]byte(fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", stat.Name, commitId, pid, ports, cpu, nice, mem, numThreads, reqDelay, uptime)))
	}

	applyHeaderContent := func(input string, top, tgt ctk.Label) {
		lines := strings.Split(buf.String(), "\n")
		numLines := len(lines)
		longestLine := 0
		if numLines > 2 {
			// remove trailing empty line
			if beStrings.Empty(lines[numLines-1]) {
				lines = lines[:numLines-1]
				numLines -= 1
			}
		}
		for _, line := range lines {
			clean := RxTangoTags.ReplaceAllString(line, "")
			if lineLen := len(clean); lineLen > longestLine {
				longestLine = lineLen
			}
		}
		if numLines >= 2 {
			_ = top.SetMarkup(dim(lines[0]))
			tgt.SetSizeRequest(longestLine, numLines-1)
			_ = tgt.SetMarkup(strings.Join(lines[1:], "\n"))
			return
		}
		if numLines == 1 {
			_ = top.SetMarkup(dim(lines[0]))
		} else {
			_ = top.SetMarkup("")
		}
		tgt.SetSizeRequest(-1, -1)
		_ = tgt.SetMarkup("")
	}

	var biggest int
	for _, app := range snapshot.Applications {
		nameLen := len(app.Name)
		if nameLen > biggest {
			biggest = nameLen
		}
	}
	if biggest < 11 {
		biggest = 11
	}
	pad := strings.Repeat(" ", biggest-7)

	buf = bytes.NewBuffer([]byte(""))
	tw = tabwriter.NewWriter(io.Writer(buf), 6, 0, 2, ' ', tabwriter.FilterHTML)
	_, _ = tw.Write([]byte("SERVICE" + pad + "\tVER\tPID\tPORT\tCPU\tPRI\tMEM\tP/T\tREQ\tUPTIME\n"))

	for _, stat := range snapshot.Services {
		if stat.Name == "reverse-proxy" {
			writeEntry(tw, stat.Name, "", stat, ppl.TotalRequest, ppl.TotalDelayed, false)
		} else if stat.Name == "git-repository" {
			writeEntry(tw, stat.Name, "", stat, 0, 0, false)
		}
	}
	_ = tw.Flush()

	applyHeaderContent(buf.String(), sw.srvHeader, sw.srvLabel)

	/* APPLICATIONS SECTION */

	buf = bytes.NewBuffer([]byte(""))
	pad = strings.Repeat(" ", biggest-11)
	tw = tabwriter.NewWriter(io.Writer(buf), 6, 0, 2, ' ', tabwriter.FilterHTML)
	_, _ = tw.Write([]byte("APPLICATION" + pad + "\tGIT\tPID\tPORT\tCPU\tPRI\tMEM\tP/T\tREQ\tUPTIME\n"))

	appSnaps := make(map[string][]WatchProc)
	for _, stat := range snapshot.Applications {
		if app, ok := sw.cliCmd.config.Applications[stat.Name]; ok {
			appSnaps[app.Name] = append(appSnaps[app.Name], stat)
		}
	}

	// beIo.StdoutF("app stat: %+v\n", appSnaps)

	appOrder := maps.SortedKeys(sw.cliCmd.config.Applications)
	sort.Slice(appOrder, func(j, i int) (less bool) {
		a, b := appOrder[i], appOrder[j]
		aa, _ := sw.cliCmd.config.Applications[a]
		ba, _ := sw.cliCmd.config.Applications[b]
		switch {
		case aa.ThisSlug == "" && ba.ThisSlug != "":
			return true
		case aa.ThisSlug != "" && ba.ThisSlug == "":
			return false
		case aa.Maintenance && !ba.Maintenance:
			return true
		}
		aSlug := aa.GetThisSlug()
		bSlug := ba.GetThisSlug()
		switch {
		case aSlug == nil && bSlug != nil:
			return true
		case aSlug != nil && bSlug == nil:
			return false
		case aSlug == nil && bSlug == nil:
			return false
		}
		aNumWorkers := aSlug.GetNumWorkers()
		bNumWorkers := bSlug.GetNumWorkers()
		switch {
		case aNumWorkers == 0 && bNumWorkers >= 1:
			return true
		case aNumWorkers >= 1 && bNumWorkers == 0:
			return false
		}
		less = false
		return
	})

	for _, appName := range appOrder {
		app, _ := sw.cliCmd.config.Applications[appName]

		var skip bool
		if len(app.Workers) == 0 {
			skip = true
		} else if v, ok := app.Workers["web"]; ok {
			skip = v <= 0
		}
		if skip {
			continue
		}

		var appStats []WatchProc

		if v, ok := appSnaps[app.Name]; ok {
			appStats = v
		} else {
			_, _ = tw.Write([]byte(app.Name + "\t" + tDim("-", 6) + "\t" + dim("-/-") + "\t" + dim("-") + "\t" + dim("-") + "\n"))
			continue
		}

		appThisSlug := app.GetThisSlug()
		appNextSlug := app.GetNextSlug()
		slugInstances := make(map[int]*SlugWorker)
		statInstances := make(map[int]WatchProc)
		for _, as := range appStats {
			if as.Pid > 0 {
				if si := app.GetSlugWorkerByPid(as.Pid); si != nil {
					slugInstances[as.Pid] = si
					statInstances[as.Pid] = as
				}
			}
		}

		trimCommit := func(commit string) (trimmed string) {
			if len(commit) > 8 {
				trimmed = commit[:8]
			} else {
				trimmed = commit
			}
			return
		}

		if len(slugInstances) == 0 {
			if appThisSlug != nil && appNextSlug != nil {
				thisCommit := trimCommit(appThisSlug.Commit)
				nextCommit := trimCommit(appNextSlug.Commit)
				_, _ = tw.Write([]byte(app.Name + "\t \t \t \t \t \t \t \t" + dim("-") + "\n"))
				_, _ = tw.Write([]byte(" " + dim("|-") + " web:" + "-this-" + "\t" + thisCommit + "\t" + tDim("-", 5) + "\t" + dim("-/-") + "\t" + dim("-") + "\t" + dim("-") + "\n"))
				_, _ = tw.Write([]byte(" " + dim("`-") + " web:" + "-next-" + "\t" + nextCommit + "\t" + tDim("-", 5) + "\t" + dim("-/-") + "\t" + dim("-") + "\t" + dim("-") + "\n"))
			} else if appThisSlug != nil {
				commit := trimCommit(appThisSlug.Commit)
				_, _ = tw.Write([]byte(app.Name + "\t" + commit + "\t" + tDim("-", 5) + "\t" + dim("-/-") + "\t" + dim("-") + "\t" + dim("-") + "\n"))
			} else if appNextSlug != nil {
				commit := trimCommit(appNextSlug.Commit)
				_, _ = tw.Write([]byte(app.Name + "\t" + commit + "\t" + tDim("-", 5) + "\t" + dim("-/-") + "\t" + dim("-") + "\t" + dim("-") + "\n"))
			} else {
				_, _ = tw.Write([]byte(app.Name + "\t" + tDim("-", 6) + "\t" + dim("-/-") + "\t" + dim("-") + "\t" + dim("-") + "\n"))
			}
			continue
		}

		var current, delayed int64
		current, _ = ppl.Request.Apps[app.Name]
		delayed, _ = ppl.Delayed.Apps[app.Name]

		if len(statInstances) == 1 {
			for pid, st := range statInstances {
				si, _ := slugInstances[pid]
				writeEntry(tw, app.Name, si.Slug.Commit[:8], st, current, delayed, false)
				break
			}
			continue
		}

		_, _ = tw.Write([]byte(app.Name + "\t \t \t \t \t \t \t \t" + formatReqDelay(current, delayed) + "\t \n"))
		numInstances := len(slugInstances)
		var count int
		for pid, si := range slugInstances {
			var label string
			if count != numInstances-1 {
				label = " " + dim("|-") + " web:" + si.Hash[:6]
			} else {
				label = " " + dim("`-") + " web:" + si.Hash[:6]
			}
			st, _ := statInstances[pid]
			st.Name = label

			port := strconv.Itoa(si.Port)
			var c, d int64
			if v, ok := ppl.Request.Ports[port]; ok {
				c = v
			}
			if v, ok := ppl.Delayed.Ports[port]; ok {
				d = v
			}

			writeEntry(tw, app.Name, si.Slug.Commit[:8], st, c, d, false)
			count += 1
		}

	}

	_ = tw.Flush()
	applyHeaderContent(buf.String(), sw.appHeader, sw.appLabel)

	/* PROXY LIMITS SECTION */

	if numAddrs := len(ppl.Request.Addrs); numAddrs > 0 {
		buf = bytes.NewBuffer([]byte(""))
		tw = tabwriter.NewWriter(io.Writer(buf), 8, 2, 2, ' ', tabwriter.FilterHTML)
		_, _ = tw.Write([]byte("REMOTE\tREQUESTS\tDELAYED\n"))
		max := sw.cliCmd.config.ProxyLimit.Max
		for _, key := range maps.SortedKeys(ppl.Request.Addrs) {
			requests := ppl.Request.Addrs[key]
			line := key + "\t"
			line += formatPercNumber(requests, max)
			line += "\t"
			if delayed, ok := ppl.Delayed.Addrs[key]; ok {
				line += formatPercNumber(delayed, max)
			} else {
				line += formatPercNumber(0, max)
			}
			line += "\n"
			_, _ = tw.Write([]byte(line))
		}
		_ = tw.Flush()
		applyHeaderContent(buf.String(), sw.plHeader, sw.plLabel)
		height := math.CeilI(numAddrs+3, 5)
		sw.plFrame.SetSizeRequest(-1, height)
		sw.plFrame.Show()
	} else {
		sw.plFrame.Hide()
	}
	sw.window.GetVBox().Resize()
}