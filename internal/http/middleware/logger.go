package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

type responseData struct {
	status int
	size   int
}

type ResponseWriterLogger struct {
	http.ResponseWriter
	responseData responseData
}

func (rwl *ResponseWriterLogger) WriteHeader(statusCode int) {
	rwl.responseData.status = statusCode
	rwl.ResponseWriter.WriteHeader(statusCode)
}

func (rwl *ResponseWriterLogger) Write(b []byte) (int, error) {
	if rwl.responseData.status == 0 {
		rwl.responseData.status = http.StatusOK
	}
	rwl.responseData.size = len(b)
	return rwl.ResponseWriter.Write(b)
}

func (mw *Middleware) Logger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		uri := r.RequestURI
		method := r.Method

		rwl := ResponseWriterLogger{
			w, responseData{},
		}

		h.ServeHTTP(&rwl, r)

		duration := time.Since(start)

		mw.app.Logger.Info("request:",
			zap.String("source ip", r.RemoteAddr),
			zap.String("uri", uri),
			zap.String("method", method),
			zap.Duration("duration", duration),
			zap.Int("r_status", rwl.responseData.status),
			zap.Int("r_size", rwl.responseData.size),
		)
	})
}
