package gitfetcher

import (
	"errors"
	"io"
	"log/slog"
	"os"

	"github.com/adrg/xdg"
)

// init initializes the default logger.
func init() {
	var errs []error

	fp, ok := os.LookupEnv("LOGS_DIRECTORY")
	if !ok {
		var err error
		fp, err = xdg.StateFile("gitfetcher/log")
		if err != nil {
			errs = append(errs, err)
			fp = "gitfetcher.log"
		}
	}

	var writer io.Writer
	if file, err := os.OpenFile(fp, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		writer = file
	} else {
		errs = append(errs, err)
		writer = os.Stdout
	}

	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))
	if len(errs) > 0 {
		slog.Error("Log setup failed.", errAttr(errors.Join(errs...)))
	}
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
