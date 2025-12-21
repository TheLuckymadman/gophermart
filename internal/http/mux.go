package http

import (
	"net/http"

	"github.com/TheLuckymadman/gophermart/internal/app"
	"github.com/TheLuckymadman/gophermart/internal/http/handlers"
	"github.com/TheLuckymadman/gophermart/internal/http/middleware"
)

func NewMux(
	app *app.App,
	userSvc handlers.UserService,
	orderSvc handlers.OrderService,
	balanceSvc handlers.BalanceService,
) *http.ServeMux {
	mux := http.NewServeMux()
	m := middleware.NewMiddleware(app)
	uH := handlers.NewUHandler(app, userSvc)
	oH := handlers.NewOHandler(app, orderSvc)
	bH := handlers.NewBHandler(app, balanceSvc)
	//without auth
	mux.Handle("POST /api/user/register", m.Chain(m.Logger, m.Compressor)(http.HandlerFunc(uH.Register)))
	mux.Handle("POST /api/user/login", m.Chain(m.Logger, m.Compressor)(http.HandlerFunc(uH.Auth)))

	//with auth
	mux.Handle("POST /api/user/orders", m.Chain(m.Logger, m.Auth, m.Compressor)(http.HandlerFunc(oH.NewOrder)))
	mux.Handle("GET /api/user/orders", m.Chain(m.Logger, m.Auth, m.Compressor)(http.HandlerFunc(oH.GetOrders)))
	mux.Handle("GET /api/user/balance", m.Chain(m.Logger, m.Auth, m.Compressor)(http.HandlerFunc(bH.GetBalance)))
	mux.Handle("POST /api/user/balance/withdraw", m.Chain(m.Logger, m.Auth, m.Compressor)(http.HandlerFunc(bH.Withdraw)))
	mux.Handle("GET /api/user/withdrawals", m.Chain(m.Logger, m.Auth, m.Compressor)(http.HandlerFunc(bH.Getwithdrawals)))

	return mux
}
