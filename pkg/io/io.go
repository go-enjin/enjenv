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

package io

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/cli/git"
	"github.com/go-enjin/be/pkg/cli/run"
	"github.com/go-enjin/be/pkg/globals"
	"github.com/go-enjin/be/pkg/notify"
)

var (
	SlackChannel string
	CustomIndent = ""
	BinName      = filepath.Base(os.Args[0])
	LogFile      string
)

func GitTagRelVer() (gitTag, relVer string) {
	if gitTag, _ = git.Describe(); gitTag == "" {
		gitTag = "untagged"
	}
	relVer = git.MakeCustomVersion("release", "c", "d")
	return
}

func GitTagRelVerString() (out string) {
	var gitTag, relVer string
	if gitTag, _ = git.Describe(); gitTag == "" {
		gitTag = "untagged"
	}
	relVer = git.MakeCustomVersion("release", "c", "d")
	out = gitTag + " (" + relVer + ")"
	return
}

func getSlackPrefix() (prefix string) {
	if notifyPrefix := os.Getenv("_ENJENV_NOTIFY_PREFIX"); notifyPrefix != "" {
		prefix = fmt.Sprintf("(%v) *%v %v*", BinName, globals.Hostname, notifyPrefix)
	} else {
		prefix = fmt.Sprintf("(%v) *%v %v*", BinName, globals.Hostname, GitTagRelVerString())
	}
	return
}

func notifySlack(tag string, message string) {
	var channel string
	if channel = notify.SlackUrl(SlackChannel); channel == "" {
		return
	}
	prefix := getSlackPrefix()
	messages := ""
	count := 0
	lines := strings.Split(message, "\n")
	if len(lines) == 1 {
		messages = strings.TrimSpace(lines[0])
		count = 1
	} else {
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if len(line) > 0 {
				messages += "\t" + line + "\n"
				count += 1
			}
		}
	}
	if messages != "" {
		var output string
		if count > 1 {
			output = fmt.Sprintf("%v:\t_(%v)_\n%v", prefix, tag, messages)
		} else {
			output = fmt.Sprintf("%v:\t%v\t_(%v)_", prefix, strings.TrimSpace(messages), tag)
		}
		if err := notify.SlackF(channel, output); err != nil {
			StderrF("error notifying slack channel: %v\n", err)
		}
	}
}

func SetupSlackIfPresent(ctx *cli.Context) (err error) {
	channel := SlackChannel
	if channel == "" {
		channel = ctx.String("slack")
	}
	if channel != "" {
		if webhook := notify.SlackUrl(channel); webhook != "" {
			_ = os.Setenv("ENJENV_SLACK", channel)
			SlackChannel = webhook
			// StdoutF("# using slack channel: %v\n", channel)
			return
		}
		err = fmt.Errorf("invalid slack channel given: %v", channel)
	}
	return
}

func SetupCustomIndent(ctx *cli.Context) (err error) {
	if customIndent := ctx.String("custom-indent"); customIndent != "" {
		CustomIndent = customIndent
		run.CustomExeIndent = customIndent
	}
	return
}

func NotifyF(tag, format string, argv ...interface{}) {
	format = strings.TrimSpace(format)
	msg := fmt.Sprintf(fmt.Sprintf("%v\n", strings.TrimSpace(format)), argv...)
	stdout(CustomIndent + "# " + tag + ": " + msg)
	notifySlack(tag, msg)
}

func StdoutF(format string, argv ...interface{}) {
	stdout(CustomIndent+format, argv...)
}

func StderrF(format string, argv ...interface{}) {
	stderr(CustomIndent+format, argv...)
}

func FatalF(format string, argv ...interface{}) {
	// _, _ = fmt.Fprintf(os.Stderr, CustomIndent+format, argv...)
	stderr(CustomIndent+format, argv...)
	os.Exit(1)
}

// ErrorF wraps fmt.Errorf and also issues a NotifyF with the error message
func ErrorF(format string, argv ...interface{}) (err error) {
	err = fmt.Errorf(strings.TrimSpace(format), argv...)
	NotifyF("error", err.Error())
	return
}

func STDOUT(format string, argv ...interface{}) {
	_, _ = fmt.Fprintf(os.Stdout, format, argv...)
}

func STDERR(format string, argv ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, format, argv...)
}

func stdout(format string, argv ...interface{}) {
	if LogFile == "" {
		_, _ = fmt.Fprintf(os.Stdout, format, argv...)
		return
	}
	var err error
	var fh *os.File
	if fh, err = os.OpenFile(LogFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644); err != nil {
		// _, _ = fmt.Fprintf(os.Stderr, "[stdout] error opening %v: %v\n", LogFile, err)
		_, _ = fmt.Fprintf(os.Stdout, "[stdout] "+format, argv...)
		return
	}
	_, _ = fh.WriteString(fmt.Sprintf(format, argv...))
	_ = fh.Close()
	return
}

func stderr(format string, argv ...interface{}) {
	if LogFile == "" {
		_, _ = fmt.Fprintf(os.Stderr, format, argv...)
		return
	}
	var err error
	var fh *os.File
	if fh, err = os.OpenFile(LogFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644); err != nil {
		// _, _ = fmt.Fprintf(os.Stderr, "[stderr] error opening %v: %v\n", LogFile, err)
		_, _ = fmt.Fprintf(os.Stderr, "[stderr] "+format, argv...)
		return
	}
	_, _ = fh.WriteString(fmt.Sprintf("ERR "+format, argv...))
	_ = fh.Close()
	return
}

func AppendF(logfile, format string, argv ...interface{}) {
	format = strings.TrimSpace(format)
	var err error
	var fh *os.File
	if fh, err = os.OpenFile(logfile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644); err != nil {
		// _, _ = fmt.Fprintf(os.Stderr, "error opening %v: %v\n", LogFile, err)
		_, _ = fmt.Fprintf(os.Stdout, "ERR "+format+"\n", argv...)
		return
	}
	_, _ = fh.WriteString(fmt.Sprintf(format+"\n", argv...))
	_ = fh.Close()
}
