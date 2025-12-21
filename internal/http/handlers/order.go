package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/TheLuckymadman/gophermart/internal/app"
	"github.com/TheLuckymadman/gophermart/internal/apperrors"
	"github.com/TheLuckymadman/gophermart/internal/http/middleware"
	"github.com/TheLuckymadman/gophermart/internal/models"
	"github.com/TheLuckymadman/gophermart/internal/utils"

	"go.uber.org/zap"
)

type OHandler struct {
	app *app.App
	srv OrderService
}

func NewOHandler(a *app.App, srv OrderService) *OHandler {
	return &OHandler{app: a, srv: srv}
}

func (h *OHandler) NewOrder(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(r.Header.Get("Content-Type"), "text/plain") {
		h.app.Logger.Warn("wrong content type", zap.String("source ip", r.RemoteAddr))
		http.Error(w, "wrong content type, use text/plain", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	claims, ok := middleware.GetUsernameFromCtx(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.app.Logger.Warn("cannot read order number from body", zap.String("source ip", r.RemoteAddr))
		http.Error(w, "cannot read order number from body", http.StatusBadRequest)
		return
	}
	number := strings.TrimSpace(string(body))
	ok, err = utils.CheckNumber(number)
	if !ok {
		h.app.Logger.Warn("incorrect order number", zap.String("source ip", r.RemoteAddr), zap.Error(err))
		errMsg := fmt.Sprintf("incorrect order number: %v", err)
		http.Error(w, errMsg, http.StatusUnprocessableEntity)
		return
	}
	h.app.Logger.Info(
		"new order request",
		zap.String("source ip", r.RemoteAddr),
		zap.String("login", claims.Username),
		zap.Int("id", claims.ID),
		zap.String("order", number),
	)
	user := models.Users{ID: claims.ID, Login: claims.Username}
	err = h.srv.NewOrder(ctx, &user, number)
	switch {
	case errors.Is(err, apperrors.ErrUserAlreadyHasOrder):
		h.app.Logger.Warn("user already has an order with this number", zap.String("source ip", r.RemoteAddr), zap.Error(err))
		w.WriteHeader(http.StatusOK)
		return
	case errors.Is(err, apperrors.ErrAnotherUserAlreadyHasOrder):
		h.app.Logger.Warn("incorrect order number", zap.String("source ip", r.RemoteAddr), zap.Error(err))
		warnMsg := fmt.Sprintf("another user already has an order with this number: %v", err)
		http.Error(w, warnMsg, http.StatusConflict)
		return
	case err != nil:
		h.app.Logger.Error("failed to create order", zap.String("source ip", r.RemoteAddr), zap.Error(err))
		http.Error(w, "failed to create order", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *OHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	h.app.Logger.Info("new request on orders", zap.String("source ip", r.RemoteAddr))
	ctx := r.Context()
	claims, ok := middleware.GetUsernameFromCtx(ctx)
	if !ok {
		h.app.Logger.Error("cannot get username from the request's context", zap.String("source ip", r.RemoteAddr))
		http.Error(w, "cannot get username from context", http.StatusInternalServerError)
		return
	}
	user := models.Users{ID: claims.ID, Login: claims.Username}
	orders, err := h.srv.GetOrders(ctx, &user)

	if err != nil {
		h.app.Logger.Error("cannot get orders", zap.String("source ip", r.RemoteAddr), zap.Error(err))
		errMsg := "cannot get orders"
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}
	if len(orders) == 0 {
		h.app.Logger.Info("no orders found", zap.String("source ip", r.RemoteAddr))
		//errMsg := "no orders found"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var orderResponses []models.OrderResponse
	for _, o := range orders {
		orderResponses = append(orderResponses, models.OrderResponse{
			Number:    o.Number,
			Status:    o.Status,
			Accrual:   float64(o.Accrual) / 100,
			CreatedAt: o.ProcessedAt,
		},
		)
	}
	body, err := json.Marshal(orderResponses)
	if err != nil {
		h.app.Logger.Error("cannot marshal orders", zap.String("source ip", r.RemoteAddr), zap.Error(err))
		errMsg := "cannot get orders"
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(body)
	if err != nil {
		h.app.Logger.Error("cannot write body", zap.String("source ip", r.RemoteAddr), zap.Error(err))
	}
}
