package helpers

import (
	"net/http"

	"go.uber.org/zap"
)

func WriteError(l *zap.Logger, w http.ResponseWriter, r *http.Request, logLevel string, msg string, status int, err error) {
	switch logLevel {
	case "error":
		if err != nil {
			l.Error(msg, zap.String("source ip", r.RemoteAddr), zap.Error(err))
		} else {
			l.Error(msg, zap.String("source ip", r.RemoteAddr))
		}
	case "warn":
		if err != nil {
			l.Warn(msg, zap.String("source ip", r.RemoteAddr), zap.Error(err))
		} else {
			l.Warn(msg, zap.String("source ip", r.RemoteAddr))
		}
	}
	http.Error(w, msg, status)
}
