package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/TheLuckymadman/gophermart/internal/app"
	"github.com/TheLuckymadman/gophermart/internal/http/helpers"
	"github.com/TheLuckymadman/gophermart/internal/models"
)

type UHandler struct {
	app *app.App
	srv UserService
}

func NewUHandler(a *app.App, srv UserService) *UHandler {
	return &UHandler{app: a, srv: srv}
}

func (h *UHandler) Register(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		helpers.WriteError(h.app.Logger, w, r, "warn", "wrong content type, use application/json", http.StatusBadRequest, nil)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		helpers.WriteError(h.app.Logger, w, r, "warn", "read http body failed", http.StatusBadRequest, err)
		return
	}
	var user models.Users
	if err = json.Unmarshal(body, &user); err != nil {
		helpers.WriteError(h.app.Logger, w, r, "warn", "user registration failed, body reading failed", http.StatusBadRequest, err)
		return
	}
	token, err := h.srv.Create(r.Context(), &user)
	if err != nil {
		helpers.WriteError(h.app.Logger, w, r, "error", "user registration failed", http.StatusInternalServerError, err)
		return
	}
	payload := struct {
		Token string
		Login string
	}{
		Token: token,
		Login: user.Login,
	}
	body, err = json.Marshal(payload)
	if err != nil {
		helpers.WriteError(h.app.Logger, w, r, "error", "user registration failed", http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Authorization", "Bearer "+token)
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(body)
	if err != nil {
		h.app.Logger.Error("send body failed", zap.String("source ip", r.RemoteAddr), zap.Error(err))
	}
}

func (h *UHandler) Auth(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		helpers.WriteError(h.app.Logger, w, r, "warn", "wrong content type, use application/json", http.StatusBadRequest, nil)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		helpers.WriteError(h.app.Logger, w, r, "warn", "read http body failed", http.StatusBadRequest, err)
		return
	}
	var user models.Users
	if err = json.Unmarshal(body, &user); err != nil {
		helpers.WriteError(h.app.Logger, w, r, "warn", "user registration failed, body reading failed", http.StatusBadRequest, err)
		return
	}
	token, err := h.srv.Login(r.Context(), &user)
	if err != nil {
		helpers.WriteError(h.app.Logger, w, r, "error", "user verification failed", http.StatusInternalServerError, err)
		return
	}
	payload := struct {
		Token string
		Login string
	}{
		Token: token,
		Login: user.Login,
	}
	body, err = json.Marshal(payload)
	if err != nil {
		helpers.WriteError(h.app.Logger, w, r, "error", "user registration failed", http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Authorization", "Bearer "+token)
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(body)
	if err != nil {
		h.app.Logger.Error("send body failed", zap.String("source ip", r.RemoteAddr), zap.Error(err))
	} else {
		h.app.Logger.Info(
			"user succussfully authenticaed",
			zap.String("login", user.Login),
		)
	}
}
