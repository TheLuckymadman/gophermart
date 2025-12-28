package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

type ResponseWriterCompressor struct {
	http.ResponseWriter
	gzip *gzip.Writer
}

func (rwc *ResponseWriterCompressor) WriteHeader(statusCode int) {
	rwc.ResponseWriter.WriteHeader(statusCode)
}

func (rwc *ResponseWriterCompressor) Write(b []byte) (int, error) {
	return rwc.gzip.Write(b)
}

func (mw *Middleware) Compressor(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			mw.app.Logger.Debug("body decompression is starting", zap.String("source ip", r.RemoteAddr))
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				mw.app.Logger.Error("decompress failed:", zap.String("source ip", r.RemoteAddr), zap.Error(err))
				http.Error(w, "decompressing failed", http.StatusBadRequest)
				return
			}
			defer gz.Close()

			r.Body = io.NopCloser(gz)
			r.Header.Del("Content-Encoding")
		}

		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			mw.app.Logger.Debug("body compression is requested", zap.String("source ip", r.RemoteAddr))
			gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
			if err != nil {
				mw.app.Logger.Error("compression failed:", zap.String("source ip", r.RemoteAddr), zap.Error(err))
				http.Error(w, "compression failed", http.StatusInternalServerError)
				return
			}
			defer gz.Close()

			rwc := ResponseWriterCompressor{w, gz}
			rwc.ResponseWriter.Header().Set("Content-Encoding", "gzip")
			h.ServeHTTP(&rwc, r)
			return
		}

		h.ServeHTTP(w, r)
	})
}
