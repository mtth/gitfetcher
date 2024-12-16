package except

import (
	"log/slog"
)

const logErrKey = "err"

// LogErrAttr wraps an error into a loggable attribute.
func LogErrAttr(err error) slog.Attr {
	if err == nil {
		return slog.Group(logErrKey)
	}
	return slog.String(logErrKey, err.Error())
}
