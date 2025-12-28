package service

import (
	"context"
	"fmt"

	"github.com/TheLuckymadman/gophermart/internal/app"
	"github.com/TheLuckymadman/gophermart/internal/auth"
	"github.com/TheLuckymadman/gophermart/internal/models"

	"go.uber.org/zap"
)

type UserService struct {
	app *app.App
	db  UserStorage
}

func NewUserService(a *app.App, db UserStorage) *UserService {
	return &UserService{app: a, db: db}
}

func (s *UserService) Create(ctx context.Context, u *models.Users) (string, error) {
	hash, err := auth.HashPassword(u.Password)
	if err != nil {
		s.app.Logger.Error("hashing password error:", zap.Error(err))
		return "", fmt.Errorf("hash password error")
	}
	u.Hash = hash
	id, err := s.db.CreateUser(ctx, u.Login, u.Hash)
	if err != nil {
		return "", fmt.Errorf("create user failed: %w", err)
	}
	return auth.IssueJWT(s.app.Key, id, u.Login, s.app.TokenExpTime)
}

func (s *UserService) Login(ctx context.Context, u *models.Users) (string, error) {
	user, err := s.db.GetUser(ctx, u.Login)
	if err != nil {
		s.app.Logger.Error("user nof found:", zap.Error(err))
		return "", fmt.Errorf("user nof found")
	}
	//fmt.Printf("db pwd: %v, req pwd: %v\n", user.Password, u.Password)
	match, err := auth.VerifyPassword(user.Password, u.Password)
	if err != nil {
		s.app.Logger.Error("password verification error:", zap.Error(err))
		return "", fmt.Errorf("password mismatch")
	}
	fmt.Printf("Password verification result: %v\n", match)
	s.app.Logger.Debug(
		"token issueing",
		zap.String("login", user.Login),
		zap.Int("user id", user.ID),
	)
	return auth.IssueJWT(s.app.Key, user.ID, user.Login, s.app.TokenExpTime)
}
