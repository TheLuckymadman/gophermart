package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/TheLuckymadman/gophermart/internal/app"
	"github.com/TheLuckymadman/gophermart/internal/apperrors"
	"github.com/TheLuckymadman/gophermart/internal/http/helpers"
	"github.com/TheLuckymadman/gophermart/internal/http/middleware"
	"github.com/TheLuckymadman/gophermart/internal/models"
	"github.com/TheLuckymadman/gophermart/internal/utils"

	"go.uber.org/zap"
)

type BHandler struct {
	app *app.App
	srv BalanceService
}

func NewBHandler(app *app.App, srv BalanceService) *BHandler {
	return &BHandler{app: app, srv: srv}
}

func (h *BHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	h.app.Logger.Info("new request on balance", zap.String("source ip", r.RemoteAddr))
	ctx := r.Context()
	claims, ok := middleware.GetUsernameFromCtx(ctx)
	if !ok {
		helpers.WriteError(h.app.Logger, w, r, "error", "cannot get username from context", http.StatusInternalServerError, nil)
		return
	}
	user := models.Users{ID: claims.ID, Login: claims.Username}
	balance, withdrawn, err := h.srv.GetBalance(ctx, &user)
	if err != nil {
		helpers.WriteError(h.app.Logger, w, r, "error", "cannot get balance", http.StatusInternalServerError, err)
		return
	}
	payload := models.Balance{Current: utils.ConvertFromCents(balance), Withdrawn: utils.ConvertFromCents(withdrawn)}
	body, err := json.Marshal(payload)
	if err != nil {
		helpers.WriteError(h.app.Logger, w, r, "error", "cannot get balance", http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(body)
	if err != nil {
		h.app.Logger.Error("cannot write body", zap.String("source ip", r.RemoteAddr), zap.Error(err))
	}
}

func (h *BHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	h.app.Logger.Info("new withdrawal request", zap.String("source ip", r.RemoteAddr))
	ctx := r.Context()
	claims, ok := middleware.GetUsernameFromCtx(ctx)
	if !ok {
		helpers.WriteError(h.app.Logger, w, r, "error", "cannot get username from context", http.StatusInternalServerError, nil)
		return
	}
	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		helpers.WriteError(h.app.Logger, w, r, "warn", "wrong content type, use application/json", http.StatusBadRequest, nil)
		return
	}
	var wr models.WithdrawalResponse
	err := json.NewDecoder(r.Body).Decode(&wr)
	if err != nil {
		helpers.WriteError(h.app.Logger, w, r, "warn", "cannot read body with withdrawal amount", http.StatusBadRequest, err)
		return
	}
	ok, err = helpers.CheckNumber(wr.Number)
	if !ok {
		errMsg := fmt.Sprintf("incorrect order number: %v", err)
		helpers.WriteError(h.app.Logger, w, r, "warn", errMsg, http.StatusUnprocessableEntity, err)
		return
	}
	wi := models.WithdrawalInternal{Number: wr.Number, AmountCents: utils.ConvertToCents(wr.Amount)}
	user := models.Users{ID: claims.ID, Login: claims.Username}
	h.app.Logger.Info("withdrawal request", zap.Any("wi", wi))
	err = h.srv.Withdraw(ctx, &user, wi)
	switch {
	case errors.Is(err, apperrors.ErrInsufficientFunds):
		helpers.WriteError(h.app.Logger, w, r, "warn", "insufficient funds", http.StatusPaymentRequired, err)
		return
	case err != nil:
		helpers.WriteError(h.app.Logger, w, r, "error", "cannot withdraw", http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *BHandler) Getwithdrawals(w http.ResponseWriter, r *http.Request) {
	h.app.Logger.Debug("new get withdrawals request", zap.String("source ip", r.RemoteAddr))
	ctx := r.Context()
	claims, ok := middleware.GetUsernameFromCtx(ctx)
	if !ok {
		helpers.WriteError(h.app.Logger, w, r, "error", "cannot get username from context", http.StatusInternalServerError, nil)
		return
	}
	user := models.Users{ID: claims.ID, Login: claims.Username}
	withdrawals, err := h.srv.Getwithdrawals(ctx, &user)
	var withdrawalResponses []models.WithdrawalResponse
	for _, w := range withdrawals {
		withdrawalResponses = append(
			withdrawalResponses,
			models.WithdrawalResponse{Number: w.Number, Amount: utils.ConvertFromCents(w.AmountCents), CreatedAt: w.CreatedAt},
		)
	}
	if err != nil {
		helpers.WriteError(h.app.Logger, w, r, "error", "cannot get withdrawals", http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(withdrawalResponses); err != nil {
		helpers.WriteError(h.app.Logger, w, r, "error", "cannot get withdrawals", http.StatusInternalServerError, err)
		return
	}
}
