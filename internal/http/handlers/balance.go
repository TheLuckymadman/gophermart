package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/TheLuckymadman/gophermart/internal/app"
	"github.com/TheLuckymadman/gophermart/internal/apperrors"
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
		h.app.Logger.Error("cannot get username from the request's context", zap.String("source ip", r.RemoteAddr))
		http.Error(w, "cannot get username from context", http.StatusInternalServerError)
		return
	}
	user := models.Users{ID: claims.ID, Login: claims.Username}
	balance, withdrawn, err := h.srv.GetBalance(ctx, &user)
	if err != nil {
		h.app.Logger.Error("cannot get balance", zap.String("source ip", r.RemoteAddr), zap.Error(err))
		errMsg := "cannot get balance"
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}
	payload := models.Balance{Current: utils.ConvertFromCents(balance), Withdrawn: utils.ConvertFromCents(withdrawn)}
	body, err := json.Marshal(payload)
	if err != nil {
		h.app.Logger.Error("cannot marshal balance", zap.String("source ip", r.RemoteAddr), zap.Error(err))
		errMsg := "cannot get balance"
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

func (h *BHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	h.app.Logger.Info("new withdrawal request", zap.String("source ip", r.RemoteAddr))
	ctx := r.Context()
	claims, ok := middleware.GetUsernameFromCtx(ctx)
	if !ok {
		h.app.Logger.Error("cannot get username from the request's context", zap.String("source ip", r.RemoteAddr))
		http.Error(w, "cannot get username from context", http.StatusInternalServerError)
		return
	}
	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		h.app.Logger.Warn("wrong content type", zap.String("source ip", r.RemoteAddr))
		http.Error(w, "wrong content type, use application/json", http.StatusBadRequest)
		return
	}
	var wr models.WithdrawalResponse
	err := json.NewDecoder(r.Body).Decode(&wr)
	if err != nil {
		h.app.Logger.Warn("decode body", zap.String("source ip", r.RemoteAddr))
		http.Error(w, "cannot read body with withdrawal amount", http.StatusBadRequest)
		return
	}
	ok, err = utils.CheckNumber(wr.Number)
	if !ok {
		h.app.Logger.Warn("incorrect order number", zap.String("source ip", r.RemoteAddr))
		errMsg := fmt.Sprintf("incorrect order number: %v", err)
		http.Error(w, errMsg, http.StatusUnprocessableEntity)
		return
	}
	wi := models.WithdrawalInternal{Number: wr.Number, AmountCents: utils.ConvertToCents(wr.Amount)}
	user := models.Users{ID: claims.ID, Login: claims.Username}
	h.app.Logger.Info("withdrawal request", zap.Any("wi", wi))
	err = h.srv.Withdraw(ctx, &user, wi)
	switch {
	case errors.Is(err, apperrors.ErrInsufficientFunds):
		h.app.Logger.Warn("insufficient funds", zap.String("source ip", r.RemoteAddr))
		http.Error(w, "insufficient funds", http.StatusPaymentRequired)
		return
	case err != nil:
		h.app.Logger.Error("cannot withdraw", zap.String("source ip", r.RemoteAddr), zap.Error(err))
		http.Error(w, "cannot withdraw", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *BHandler) Getwithdrawals(w http.ResponseWriter, r *http.Request) {
	h.app.Logger.Debug("new get withdrawals request", zap.String("source ip", r.RemoteAddr))
	ctx := r.Context()
	claims, ok := middleware.GetUsernameFromCtx(ctx)
	if !ok {
		h.app.Logger.Error("cannot get username from the request's context", zap.String("source ip", r.RemoteAddr))
		http.Error(w, "cannot get username from context", http.StatusInternalServerError)
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
		h.app.Logger.Error("cannot get withdrawals", zap.String("source ip", r.RemoteAddr), zap.Error(err))
		http.Error(w, "cannot get withdrawals", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(withdrawalResponses)
	if err != nil {
		h.app.Logger.Error("cannot encode http body", zap.String("source ip", r.RemoteAddr), zap.Error(err))
		http.Error(w, "cannot get withdrawals", http.StatusInternalServerError)
		return
	}
}
