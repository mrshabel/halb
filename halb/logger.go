package halb

import (
	"log/slog"
	"os"
)

type Logger struct {
	*slog.Logger
}

func NewLogger(debug bool) *Logger {
	// setup log level
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{
		Level: level,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	if debug {
		handler = (*slog.JSONHandler)(slog.NewTextHandler(os.Stdout, opts))
	}

	logger := slog.New(handler)
	// set global default logger
	slog.SetDefault(logger)

	return &Logger{logger}
}
