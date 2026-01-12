package runtime

import (
	"log/slog"
	"os"

	"github.com/goplus/ixgo"
)

// Logger defines the interface for runtime logging
type Logger interface {
	// Info logs an informational message
	Info(msg string, args ...any)
	// Error logs an error message
	Error(msg string, args ...any)
}

// slogLogger implements Logger using slog
type slogLogger struct {
	logger *slog.Logger
}

// Info logs an informational message
func (l *slogLogger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

// Error logs an error message
func (l *slogLogger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

var defaultLogger Logger = &slogLogger{
	logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
}

func logWithCaller(msg string, frame *ixgo.Frame) {
	if frs := frame.CallerFrames(); len(frs) > 0 {
		fr := frs[0]
		defaultLogger.Info(
			msg,
			"function", fr.Function,
			"file", fr.File,
			"line", fr.Line,
		)
	}
}

func logPanicInfo(info *ixgo.PanicInfo) {
	position := info.Position()
	defaultLogger.Error(
		"panic",
		"error", info.Error,
		"function", info.String(),
		"file", position.Filename,
		"line", position.Line,
		"column", position.Column,
	)
}
