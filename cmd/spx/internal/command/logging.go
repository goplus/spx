package command

import "github.com/goplus/spx/v2/cmd/spx/internal/logutil"

func enableDebugLogging() {
	logutil.EnableDebug()
}

func logDebugf(format string, args ...any) {
	logutil.Debugf(format, args...)
}

func logInfof(format string, args ...any) {
	logutil.Infof(format, args...)
}

func logWarnf(format string, args ...any) {
	logutil.Warnf(format, args...)
}

func logErrorf(format string, args ...any) {
	logutil.Errorf(format, args...)
}

func logFatalf(format string, args ...any) {
	logutil.Fatalf(format, args...)
}
