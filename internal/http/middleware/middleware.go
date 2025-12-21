package middleware

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/TheLuckymadman/gophermart/internal/app"
	"github.com/TheLuckymadman/gophermart/internal/models"
	"github.com/TheLuckymadman/gophermart/internal/utils"
	"go.uber.org/zap"
)

type Middleware struct {
	app *app.App
}

type ctxKeyClaims struct{}

func addUsernametoCtx(ctx context.Context, u *models.Claims) context.Context {
	return context.WithValue(ctx, ctxKeyClaims{}, u)
}

func GetUsernameFromCtx(ctx context.Context) (*models.Claims, bool) {
	u, ok := ctx.Value(ctxKeyClaims{}).(*models.Claims)
	return u, ok
}

func NewMiddleware(a *app.App) *Middleware {
	return &Middleware{app: a}
}

type MiddlewareFunc func(http.Handler) http.Handler

func (mw *Middleware) Chain(m ...MiddlewareFunc) MiddlewareFunc {
	return func(h http.Handler) http.Handler {
		for i := len(m) - 1; i >= 0; i-- {
			h = m[i](h)
		}
		return h
	}
}

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

func (mw *Middleware) Auth(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenHeader := r.Header.Get("Authorization")
		if tokenHeader == "" {
			mw.app.Logger.Info("request is not authorized, header Authorization not found",
				zap.String("source ip", r.RemoteAddr))
			http.Error(w, "request is denied", http.StatusUnauthorized)
			return
		}
		parts := strings.SplitN(tokenHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			mw.app.Logger.Info("request is not authorized, header Authorization is incorrect",
				zap.String("source ip", r.RemoteAddr))
			http.Error(w, "request is denied", http.StatusUnauthorized)
			return
		}
		token := parts[1]
		claims, err := utils.VerifyJWT(token, mw.app.Key)
		if err != nil {
			mw.app.Logger.Info("request is not authorized on JWT verifying",
				zap.String("source ip", r.RemoteAddr), zap.Error(err))
			http.Error(w, "request is denied", http.StatusUnauthorized)
			return
		}
		ctx := addUsernametoCtx(r.Context(), claims)
		r = r.WithContext(ctx)
		mw.app.Logger.Info("request is authorized successfully",
			zap.String("source ip", r.RemoteAddr))
		h.ServeHTTP(w, r)
	})
}

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
