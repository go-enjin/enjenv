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
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/go-curses/cdk"
	cenums "github.com/go-curses/cdk/lib/enums"
	cstrings "github.com/go-curses/cdk/lib/strings"
	"github.com/go-curses/ctk"
	"github.com/go-curses/ctk/lib/enums"
	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/maps"
	beStrings "github.com/go-enjin/be/pkg/strings"
	"github.com/go-enjin/enjenv/pkg/globals"
	beIo "github.com/go-enjin/enjenv/pkg/io"
)

//go:embed status-watch.accelmap
var statusWatchAccelmap string

// Build Configuration Flags
// setting these will enable command line flags and their corresponding features
// use `go build -v -ldflags="-X 'github.com/go-enjin/enjenv/commands/niseroku.IncludeLogFullPaths=false'"`
var (
	IncludeProfiling          = "false"
	IncludeLogFile            = "true"
	IncludeLogFormat          = "false"
	IncludeLogFullPaths       = "false"
	IncludeLogLevel           = "true"
	IncludeLogLevels          = "false"
	IncludeLogTimestamps      = "false"
	IncludeLogTimestampFormat = "false"
	IncludeLogOutput          = "true"
)

func init() {
	cdk.Build.Profiling = cstrings.IsTrue(IncludeProfiling)
	cdk.Build.LogFile = cstrings.IsTrue(IncludeLogFile)
	cdk.Build.LogFormat = cstrings.IsTrue(IncludeLogFormat)
	cdk.Build.LogFullPaths = cstrings.IsTrue(IncludeLogFullPaths)
	cdk.Build.LogLevel = cstrings.IsTrue(IncludeLogLevel)
	cdk.Build.LogLevels = cstrings.IsTrue(IncludeLogLevels)
	cdk.Build.LogTimestamps = cstrings.IsTrue(IncludeLogTimestamps)
	cdk.Build.LogTimestampFormat = cstrings.IsTrue(IncludeLogTimestampFormat)
	cdk.Build.LogOutput = cstrings.IsTrue(IncludeLogOutput)
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
	sysLabelUT  ctk.Label
	sysValueUT  ctk.Label

	srvFrame ctk.Frame
	srvLabel ctk.Label

	appFrame ctk.Frame
	appLabel ctk.Label

	plFrame ctk.Frame
	plLabel ctk.Label

	errLabel    ctk.Label
	statusLabel ctk.Label

	freq     time.Duration
	watching *Watching
	started  bool

	sync.RWMutex
}

func NewStatusWatch(cmd *Command, freq time.Duration) (sw *StatusWatch, err error) {
	app := ctk.NewApplication(
		"niseroku",
		"Niseroku status watch",
		"Basically 'top' for niseroku services and applications",
		globals.BuildVersion,
		"status-watch",
		fmt.Sprintf("niseroku status watch - %v", globals.DisplayVersion),
		"/dev/tty",
	)
	sw = &StatusWatch{
		cliCmd: cmd,
		ctkApp: app,
		freq:   freq,
	}
	if sw.watching, err = NewWatching(cmd.config, freq, sw.update); err != nil {
		return
	}
	app.Connect(cdk.SignalStartup, "status-watch-startup-handler", sw.startup)
	app.Connect(cdk.SignalShutdown, "status-watch-quit-handler", sw.shutdown)
	return
}

func (sw *StatusWatch) Run(ctx *cli.Context) (err error) {
	err = sw.ctkApp.Run(ctx.Args().Slice())
	return
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
	sw.errDialog.Show()
	sw.display.RequestDraw()
	sw.display.RequestShow()
}

func (sw *StatusWatch) hideErrorWindow() {
	if sw.errDialog != nil {
		sw.errDialog.Hide()
		sw.errDialog.Destroy()
		sw.display.RequestDraw()
		sw.display.RequestShow()
	}
}

func (sw *StatusWatch) displayResizeHandler(data []interface{}, argv ...interface{}) cenums.EventFlag {
	if s := sw.display.Screen(); s != nil {
		w, h := s.Size()
		if w < 80 || h < 24 {
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

		if err := sw.watching.Start(); err != nil {
			sw.showError("error starting watching: %v", err)
		}

		d.CaptureCtrlC()
		sw.display = d

		sw.display.Connect(cdk.SignalEventResize, "display-resize-handler", sw.displayResizeHandler)

		sw.window = ctk.NewWindowWithTitle( /*app.Title()*/ "")
		sw.window.SetBorderWidth(0)
		sw.window.SetDecorated(false)
		sw.window.SetSensitive(true)

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

		sw.sysFrame = ctk.NewFrame("")
		sw.sysFrame.SetLabelAlign(0.0, 0.5)
		sw.sysFrame.SetSizeRequest(-1, 5)
		sw.sysFrame.Show()
		sysHBox := ctk.NewHBox(false, 1)
		sysHBox.Show()

		sysVBoxLabels := ctk.NewVBox(false, 0)
		sysVBoxLabels.Show()
		sysVBoxValues := ctk.NewVBox(false, 0)
		sysVBoxValues.Show()

		sw.sysLabelUT = rigLabel("Uptime:")
		sw.sysLabelUT.SetSizeRequest(20, 1)
		sw.sysLabelUT.Show()
		sysVBoxLabels.PackStart(sw.sysLabelUT, false, false, 0)
		sw.sysValueUT = rigLabel("")
		sw.sysValueUT.SetSizeRequest(-1, 1)
		sw.sysValueUT.Show()
		sysVBoxValues.PackEnd(sw.sysValueUT, false, false, 0)

		sw.sysLabelCPU = rigLabel("CPU Usage:")
		sw.sysLabelCPU.SetSizeRequest(20, 1)
		sw.sysLabelCPU.Show()
		sysVBoxLabels.PackStart(sw.sysLabelCPU, false, false, 0)
		sw.sysValueCPU = rigLabel("")
		sw.sysValueCPU.SetSizeRequest(-1, 1)
		sw.sysValueCPU.Show()
		sysVBoxValues.PackEnd(sw.sysValueCPU, false, false, 0)

		sw.sysLabelMEM = rigLabel("Memory:")
		sw.sysLabelMEM.SetSizeRequest(20, 1)
		sw.sysLabelMEM.Show()
		sysVBoxLabels.PackStart(sw.sysLabelMEM, false, false, 0)
		sw.sysValueMEM = rigLabel("")
		sw.sysValueMEM.SetSizeRequest(-1, 1)
		sw.sysValueMEM.Show()
		sysVBoxValues.PackEnd(sw.sysValueMEM, false, false, 0)

		sw.sysFrame.Add(sysHBox)
		sysHBox.PackStart(sysVBoxLabels, true, true, 0)
		sysHBox.PackStart(sysVBoxValues, true, true, 0)
		contentVBox.PackStart(sw.sysFrame, false, true, 0)

		/* SERVICES SECTION */

		sw.srvFrame = ctk.NewFrame("")
		sw.srvFrame.SetLabelAlign(0.0, 0.5)
		sw.srvFrame.SetSizeRequest(-1, 5)
		sw.srvFrame.Show()
		sw.srvLabel = rigLabel("")
		sw.srvLabel.SetSizeRequest(-1, 3)
		sw.srvLabel.SetUseMarkup(true)
		sw.srvLabel.Show()
		sw.srvFrame.Add(sw.srvLabel)
		contentVBox.PackStart(sw.srvFrame, false, true, 0)

		sw.appFrame = ctk.NewFrame("")
		sw.appFrame.SetLabelAlign(0.0, 0.5)
		sw.appFrame.Show()
		sw.appLabel = rigLabel("")
		sw.appLabel.SetSizeRequest(-1, 3)
		sw.appLabel.SetUseMarkup(true)
		sw.appLabel.Show()
		sw.appFrame.Add(sw.appLabel)
		contentVBox.PackStart(sw.appFrame, true, true, 0)

		sw.plFrame = ctk.NewFrame("Proxy Limits (Remote IP Addresses)")
		sw.plFrame.SetLabelAlign(0.0, 0.5)
		sw.plFrame.Hide()
		sw.plLabel = rigLabel("")
		sw.plFrame.Add(sw.plLabel)
		sw.plLabel.Show()
		contentVBox.PackStart(sw.plFrame, false, true, 0)

		sw.errLabel = rigLabel("")
		sw.errLabel.Hide()
		windowVBox.PackStart(sw.errLabel, false, false, 0)

		sw.statusLabel = rigLabel("niseroku status watch - " + globals.DisplayVersion)
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
	if proxyLimits, err = sw.cliCmd.config.CallProxyControlCommand("proxy-limits"); err != nil {
		return
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

func formatPercNumber[T maps.Number](stat T, max float64) (text string) {
	switch {
	case float64(stat) >= max:
		text = fmt.Sprintf(`<span foreground="red">%d</span>`, int64(stat))
	case float64(stat) > max/2:
		text = fmt.Sprintf(`<span foreground="yellow">%d</span>`, int64(stat))
	default:
		text = fmt.Sprintf(`<span foreground="white">%d</span>`, int64(stat))
	}
	return
}

func (sw *StatusWatch) refreshWatching(snapshot *WatchSnapshot, proxyLimits string) {
	rTotal, dTotal, rHosts, rAddrs, dHosts, dAddrs := parseProxyLimits(proxyLimits)
	stats := &snapshot.Stats

	var cpuUsage float32 = 0.0
	if len(stats.CpuUsage) > 0 {
		for _, usage := range stats.CpuUsage {
			cpuUsage += usage
		}
		cpuUsage = cpuUsage / float32(len(stats.CpuUsage)) * 100.0
	}

	memUsed := humanize.Bytes(stats.MemUsed * 1024)
	memTotal := humanize.Bytes(stats.MemTotal * 1024)
	memPerc := float64(stats.MemUsed) / float64(stats.MemTotal) * 100.0

	/* SYSTEM SECTION */

	_ = sw.sysValueUT.SetMarkup(stats.UptimeString())

	_ = sw.sysLabelCPU.SetMarkup(fmt.Sprintf("CPU Usage: %s%%", formatPercFloat(cpuUsage, "%.2f")))
	buf := bytes.NewBuffer([]byte(""))
	tw := tabwriter.NewWriter(io.Writer(buf), 8, 2, 2, ' ', tabwriter.FilterHTML)
	_, _ = tw.Write([]byte(fmt.Sprintf("[ ")))
	for idx, usage := range stats.CpuUsage {
		if idx > 0 {
			_, _ = tw.Write([]byte("\t"))
		}
		if idx > 3 {
			_, _ = tw.Write([]byte("..."))
			break
		} else {
			_, _ = tw.Write([]byte(formatPercFloat(usage*100.0, "%6.2f")))
		}
	}
	_, _ = tw.Write([]byte(fmt.Sprintf(" ]\n")))
	_ = tw.Flush()
	_ = sw.sysValueCPU.SetMarkup(buf.String())

	_ = sw.sysLabelMEM.SetMarkup(fmt.Sprintf("MEM Usage: %s%%", formatPercFloat(memPerc, "%.2f")))
	buf = bytes.NewBuffer([]byte(""))
	tw = tabwriter.NewWriter(io.Writer(buf), 8, 2, 2, ' ', tabwriter.FilterHTML)
	_, _ = tw.Write([]byte(fmt.Sprintf("Used:\t%s,\tTotal:\t%s\n", memUsed, memTotal)))
	_ = tw.Flush()
	_ = sw.sysValueMEM.SetMarkup(buf.String())

	/* SERVICES SECTION */

	writeEntry := func(tw *tabwriter.Writer, stat WatchProc, requests, delayed int64) {
		// var pid, ports, nice, cpu, mem, num, threads, reqDelay string
		pid, ports, nice, cpu, mem, num, threads, reqDelay := "-", "-", "-", "-", "-", "-", "-", "-"
		if stat.Pid > 0 {
			pid = strconv.Itoa(stat.Pid)
			nice = fmt.Sprintf("%+2d", stat.Nice)
			cpu = formatPercFloat(stat.Cpu, "%.2f")
			mem = formatPercFloat(stat.Mem, "%.2f")
			num = fmt.Sprintf("%d", stat.Num)
			threads = fmt.Sprintf("%d", stat.Threads)
			reqDelay = formatPercNumber(requests, sw.cliCmd.config.ProxyLimit.Max)
			if delayed > 0 {
				delay := formatPercNumber(delayed, sw.cliCmd.config.ProxyLimit.Max)
				reqDelay += " (" + delay + ")"
			}
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
		var commitId string = "--------"
		if app, ok := sw.cliCmd.config.Applications[stat.Name]; ok {
			if app.ThisSlug != "" {
				if RxSlugArchiveName.MatchString(app.ThisSlug) {
					m := RxSlugArchiveName.FindAllStringSubmatch(app.ThisSlug, 1)
					fullCommitId := m[0][2]
					commitId = fullCommitId[:8]
				}
			}
		} else if beStrings.StringInStrings(stat.Name, "reverse-proxy", "git-repository") {
			commitId = globals.BuildBinHash[:8]
		}
		_, _ = tw.Write([]byte(fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s/%s\t%s\n", stat.Name, commitId, pid, ports, cpu, nice, mem, num, threads, reqDelay)))
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
	_, _ = tw.Write([]byte("SERVICE" + pad + "\tID\tPID\tPORT\tCPU\tPRI\tMEM\tP/T\tREQ\n"))
	for _, stat := range snapshot.Services {
		if stat.Name == "reverse-proxy" {
			writeEntry(tw, stat, rTotal, dTotal)
		} else {
			writeEntry(tw, stat, 0, 0)
		}
	}
	_ = tw.Flush()
	_ = sw.srvLabel.SetMarkup(buf.String())

	/* APPLICATIONS SECTION */

	buf = bytes.NewBuffer([]byte(""))
	tw = tabwriter.NewWriter(io.Writer(buf), 6, 0, 2, ' ', tabwriter.FilterHTML)
	_, _ = tw.Write([]byte("APPLICATION\tID\tPID\tPORT\tCPU\tPRI\tMEM\tP/T\tREQ\n"))
	for _, stat := range snapshot.Applications {
		var current, delayed int64
		for _, app := range sw.cliCmd.config.Applications {
			if app.Name == stat.Name {
				for _, domain := range app.Domains {
					for limitDomain, count := range rHosts {
						if domain == limitDomain {
							current += count
							break
						}
					}
					for limitDomain, count := range dHosts {
						if domain == limitDomain {
							delayed += count
							break
						}
					}
				}
				break
			}
		}
		writeEntry(tw, stat, current, delayed)
	}
	_ = tw.Flush()
	_ = sw.appLabel.SetMarkup(buf.String())

	/* PROXY LIMITS SECTION */

	if len(rAddrs) > 0 {
		buf = bytes.NewBuffer([]byte(""))
		tw = tabwriter.NewWriter(io.Writer(buf), 8, 2, 2, ' ', tabwriter.FilterHTML)
		_, _ = tw.Write([]byte("REMOTE IP ADDRESS\tREQUESTS\tDELAYED\n"))
		max := sw.cliCmd.config.ProxyLimit.Max
		for _, key := range maps.SortedKeys(rAddrs) {
			requests := rAddrs[key]
			line := key + "\t"
			line += formatPercNumber(requests, max)
			line += "\t"
			if delayed, ok := dAddrs[key]; ok {
				line += formatPercNumber(delayed, max)
			} else {
				line += formatPercNumber(0, max)
			}
			line += "\n"
			_, _ = tw.Write([]byte(line))
		}
		_ = tw.Flush()
		_ = sw.plLabel.SetMarkup(buf.String())
		sw.plFrame.SetSizeRequest(-1, len(rAddrs)+3)
		sw.plFrame.Show()
	} else {
		sw.plFrame.Hide()
	}
	sw.window.GetVBox().Resize()
}