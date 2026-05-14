package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func New(level string, file string) (*slog.Logger, func() error, error) {
	var slogLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	writers := []io.Writer{os.Stderr}
	closeFn := func() error { return nil }
	if file != "" {
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			return nil, nil, err
		}
		f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return nil, nil, err
		}
		writers = append(writers, f)
		closeFn = f.Close
	}

	handler := slog.NewJSONHandler(io.MultiWriter(writers...), &slog.HandlerOptions{Level: slogLevel})
	return slog.New(handler), closeFn, nil
}
