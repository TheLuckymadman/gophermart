package handlers

import (
	"context"

	"github.com/TheLuckymadman/gophermart/internal/models"
)

type UserService interface {
	Create(ctx context.Context, u *models.Users) (string, error)
	Login(ctx context.Context, u *models.Users) (string, error)
}

type OrderService interface {
	NewOrder(ctx context.Context, u *models.Users, number string) error
	GetOrders(ctx context.Context, u *models.Users) ([]models.Order, error)
}

type BalanceService interface {
	GetBalance(ctx context.Context, u *models.Users) (int64, int64, error)
	Getwithdrawals(ctx context.Context, u *models.Users) ([]models.WithdrawalInternal, error)
	Withdraw(ctx context.Context, u *models.Users, w models.WithdrawalInternal) error
}
