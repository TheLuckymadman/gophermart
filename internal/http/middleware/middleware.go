package middleware

import (
	"net/http"

	"github.com/TheLuckymadman/gophermart/internal/app"
)

type Middleware struct {
	app *app.App
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
