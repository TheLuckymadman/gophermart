package service

import (
	"context"

	"github.com/TheLuckymadman/gophermart/internal/app"
	"github.com/TheLuckymadman/gophermart/internal/models"
)

type BalanceService struct {
	app *app.App
	db  Storage
}

func NewBalanceService(app *app.App, db Storage) *BalanceService {
	return &BalanceService{app: app, db: db}
}

func (b *BalanceService) GetBalance(ctx context.Context, u *models.Users) (int64, int64, error) {
	return b.db.GetBalance(ctx, u.ID)
}

func (b *BalanceService) Getwithdrawals(ctx context.Context, u *models.Users) ([]models.WithdrawalInternal, error) {
	return b.db.ListWithdrawals(ctx, u.ID)
}

func (b *BalanceService) Withdraw(ctx context.Context, u *models.Users, w models.WithdrawalInternal) error {
	return b.db.Withdraw(ctx, u.ID, w.AmountCents, w.Number)
}
