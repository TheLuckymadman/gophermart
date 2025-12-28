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
	"github.com/TheLuckymadman/gophermart/internal/http/helpers"
	"github.com/TheLuckymadman/gophermart/internal/http/middleware"
	"github.com/TheLuckymadman/gophermart/internal/models"

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
		helpers.WriteError(h.app.Logger, w, r, "warn", "wrong content type, use text/plain", http.StatusBadRequest, nil)
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
		helpers.WriteError(h.app.Logger, w, r, "warn", "cannot read order number from body", http.StatusBadRequest, err)
		return
	}
	number := strings.TrimSpace(string(body))
	ok, err = helpers.CheckNumber(number)
	if !ok {
		errMsg := fmt.Sprintf("incorrect order number: %v", err)
		helpers.WriteError(h.app.Logger, w, r, "warn", errMsg, http.StatusUnprocessableEntity, nil)
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
		warnMsg := fmt.Sprintf("another user already has an order with this number: %v", err)
		helpers.WriteError(h.app.Logger, w, r, "warn", warnMsg, http.StatusConflict, err)
		return
	case err != nil:
		helpers.WriteError(h.app.Logger, w, r, "error", "failed to create order", http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *OHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	h.app.Logger.Info("new request on orders", zap.String("source ip", r.RemoteAddr))
	ctx := r.Context()
	claims, ok := middleware.GetUsernameFromCtx(ctx)
	if !ok {
		helpers.WriteError(h.app.Logger, w, r, "error", "cannot get username from context", http.StatusInternalServerError, nil)
		return
	}
	user := models.Users{ID: claims.ID, Login: claims.Username}
	orders, err := h.srv.GetOrders(ctx, &user)

	if err != nil {
		errMsg := "cannot get orders"
		helpers.WriteError(h.app.Logger, w, r, "error", errMsg, http.StatusInternalServerError, err)
		return
	}
	if len(orders) == 0 {
		h.app.Logger.Info("no orders found", zap.String("source ip", r.RemoteAddr))
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
		errMsg := "cannot get orders"
		helpers.WriteError(h.app.Logger, w, r, "error", errMsg, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(body)
	if err != nil {
		h.app.Logger.Error("cannot write body", zap.String("source ip", r.RemoteAddr), zap.Error(err))
	}
}
