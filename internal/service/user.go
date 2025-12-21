package service

import (
	"context"
	"fmt"

	"github.com/TheLuckymadman/gophermart/internal/app"
	"github.com/TheLuckymadman/gophermart/internal/models"
	"github.com/TheLuckymadman/gophermart/internal/utils"
	"go.uber.org/zap"
)

type UserService struct {
	app *app.App
	db  Storage
}

func NewUserService(a *app.App, db Storage) *UserService {
	return &UserService{app: a, db: db}
}

func (s *UserService) Create(ctx context.Context, u *models.Users) (string, error) {
	hash, err := utils.HashPassword(u.Password)
	if err != nil {
		s.app.Logger.Error("hashing password error:", zap.Error(err))
		return "", fmt.Errorf("hash password error")
	}
	u.Hash = hash
	id, err := s.db.CreateUser(ctx, u.Login, u.Hash)
	if err != nil {
		return "", fmt.Errorf("create user failed: %w", err)
	}
	return utils.IssueJWT(s.app.Key, id, u.Login, s.app.TokenExpTime)
}

func (s *UserService) Login(ctx context.Context, u *models.Users) (string, error) {
	user, err := s.db.GetUser(ctx, u.Login)
	if err != nil {
		s.app.Logger.Error("user nof found:", zap.Error(err))
		return "", fmt.Errorf("user nof found")
	}
	//fmt.Printf("db pwd: %v, req pwd: %v\n", user.Password, u.Password)
	match, err := utils.VerifyPassword(user.Password, u.Password)
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
	return utils.IssueJWT(s.app.Key, user.ID, user.Login, s.app.TokenExpTime)
}

