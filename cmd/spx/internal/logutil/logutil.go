package logutil

import spxlog "github.com/goplus/spx/v2/internal/log"

func EnableDebug() {
	spxlog.SetLevel(spxlog.LevelDebug)
}

func Debugf(format string, args ...any) {
	spxlog.Debug(format, args...)
}

func Infof(format string, args ...any) {
	spxlog.Info(format, args...)
}

func Warnf(format string, args ...any) {
	spxlog.Warn(format, args...)
}

func Errorf(format string, args ...any) {
	spxlog.Error(format, args...)
}

func Fatalf(format string, args ...any) {
	spxlog.Fatalf(format, args...)
}
