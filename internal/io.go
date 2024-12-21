package gitfetcher

import (
	"log/slog"
)

const (
	logDataKey = "data"
)

func dataAttrs(attrs ...slog.Attr) slog.Attr {
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = any(attr)
	}
	return slog.Group(logDataKey, args...)
}
