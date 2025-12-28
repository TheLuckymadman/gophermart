package middleware

import (
	"context"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/TheLuckymadman/gophermart/internal/auth"
	"github.com/TheLuckymadman/gophermart/internal/models"
)

type ctxKeyClaims struct{}

func addUsernametoCtx(ctx context.Context, u *models.Claims) context.Context {
	return context.WithValue(ctx, ctxKeyClaims{}, u)
}

func GetUsernameFromCtx(ctx context.Context) (*models.Claims, bool) {
	u, ok := ctx.Value(ctxKeyClaims{}).(*models.Claims)
	return u, ok
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
		claims, err := auth.VerifyJWT(token, mw.app.Key)
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
