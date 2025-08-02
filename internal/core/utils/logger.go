package utils

import (
	"log/slog"
	"os"
)

type Logger struct {
	*slog.Logger
}

func NewLogger(level, format string) *Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	return &Logger{Logger: logger}
}

func (l *Logger) WithRepository(repo string) *Logger {
	return &Logger{Logger: l.Logger.With("repository", repo)}
}

func (l *Logger) WithOperation(op string) *Logger {
	return &Logger{Logger: l.Logger.With("operation", op)}
}

func (l *Logger) WithError(err error) *Logger {
	return &Logger{Logger: l.Logger.With("error", err)}
}
