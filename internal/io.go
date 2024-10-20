package gitfetcher

import (
	"io"
	"log/slog"
	"os"

	"github.com/adrg/xdg"
	"golang.org/x/term"
)

func init() {
	// We write logs to stderr if it is not a terminal (e.g. if pointing to the journal), otherwise we
	// write to a file.
	var writer io.Writer
	if term.IsTerminal(int(os.Stderr.Fd())) {
		if fp, err := xdg.StateFile("gitfetcher/log"); err == nil {
			if file, err := os.OpenFile(fp, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				writer = file
			}
		}
	}
	if writer == nil {
		writer = os.Stderr
	}
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))
}

const (
	logDataKey = "data"
	logErrKey  = "err"
)

func dataAttrs(attrs ...slog.Attr) slog.Attr {
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = any(attr)
	}
	return slog.Group(logDataKey, args...)
}

func errAttr(err error) slog.Attr {
	if err == nil {
		return slog.Group(logErrKey)
	}
	return slog.String(logErrKey, err.Error())
}
