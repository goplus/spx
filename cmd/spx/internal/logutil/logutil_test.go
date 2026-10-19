package logutil

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	spxlog "github.com/goplus/spx/v3/internal/log"
)

func TestLoggingLevelsFormattingAndStreams(t *testing.T) {
	var stdout, stderr bytes.Buffer
	spxlog.SetLevel(spxlog.LevelInfo)
	spxlog.SetStdoutOutput(&stdout)
	spxlog.SetStderrOutput(&stderr)
	t.Cleanup(func() {
		spxlog.SetLevel(spxlog.LevelInfo)
		spxlog.SetStdoutOutput(os.Stdout)
		spxlog.SetStderrOutput(os.Stderr)
	})
	Debugf("suppressed %d", 1)
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("debug output was not filtered at info level")
	}
	EnableDebug()
	for _, tt := range []struct {
		name string
		log  func(string, ...any)
		err  bool
	}{
		{"DEBUG", Debugf, false},
		{"INFO", Infof, false},
		{"WARN", Warnf, false},
		{"ERROR", Errorf, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stdout.Reset()
			stderr.Reset()
			tt.log("value=%d literal=%%", 42)
			output, other := stdout.String(), stderr.String()
			if tt.err {
				output, other = other, output
			}
			if !strings.Contains(output, "["+tt.name+"] [SPX] value=42 literal=%") || other != "" {
				t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestFatalLogsAndExits(t *testing.T) {
	if os.Getenv("SPX_LOGUTIL_TEST_FATAL") == "1" {
		Fatalf("fatal value=%d", 42)
		t.Fatal("Fatalf returned")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestFatalLogsAndExits$")
	command.Env = append(os.Environ(), "SPX_LOGUTIL_TEST_FATAL=1")
	if dir := flag.Lookup("test.gocoverdir"); dir != nil && dir.Value.String() != "" {
		command.Args = append(command.Args, "-test.gocoverdir="+dir.Value.String())
		command.Env = append(command.Env, "GOCOVERDIR="+dir.Value.String())
	}
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	var exit *exec.ExitError
	if err := command.Run(); !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("fatal exit = %v, stderr=%s", err, &stderr)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "[ERROR] [SPX] fatal value=42") {
		t.Fatalf("fatal output: stdout=%q stderr=%q", &stdout, &stderr)
	}
}
