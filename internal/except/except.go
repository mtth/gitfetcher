package except

import (
	"fmt"
	"log/slog"
)

// Must panics if the input predicate is false.
func Must(pred bool, msg string, args ...any) {
	if !pred {
		panic(fmt.Sprintf(msg, args...))
	}
}

const logErrKey = "err"

// LogErrAttr wraps an error into a loggable attribute.
func LogErrAttr(err error) slog.Attr {
	if err == nil {
		return slog.Group(logErrKey)
	}
	return slog.String(logErrKey, err.Error())
}
