package except

import (
	"fmt"
	"log/slog"
)

func Must(pred bool, msg string, args ...any) {
	if !pred {
		panic(fmt.Sprintf(msg, args...))
	}
}

func Require(err error) {
	Must(err == nil, "unexpected error: %v", err)
}

const logErrKey = "err"

// LogErrAttr wraps an error into a loggable attribute.
func LogErrAttr(err error) slog.Attr {
	if err == nil {
		return slog.Group(logErrKey)
	}
	return slog.String(logErrKey, err.Error())
}
