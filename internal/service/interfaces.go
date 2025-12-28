package service

import (
	"context"

	"github.com/TheLuckymadman/gophermart/internal/models"
)

type UserStorage interface {
	CreateUser(ctx context.Context, login string, hash string) (int, error)
	GetUser(ctx context.Context, login string) (*models.Users, error)
}

type OrderStorage interface {
	CreateOrder(ctx context.Context, userID int, number string) (err error)
	ListOrdersByUser(ctx context.Context, userID int) ([]models.Order, error)
}

type OrderUpdateStorage interface {
	ListOrdersToEnqueue(ctx context.Context, reqStatuses []string, updStatus string, updAccStatus string, maxOrdersCnt int) (m []models.Order, err error)
	UpdOrderTmpStatus(ctx context.Context, number string, updAccStatus string, updStatus string) (err error)
	UpdOrderStatusAndUsrBalance(ctx context.Context, number string, updAccStatus string, updStatus string, accrual int64, orderID int) (err error)
	UpdOrderFinalStatus(ctx context.Context, number string, updAccStatus string, updStatus string) (err error)	
}

type BalanceStorage interface { 
	GetBalance(ctx context.Context, userID int) (int64, int64, error)
	Withdraw(ctx context.Context, userID int, amount int64, number string) (err error)
	ListWithdrawals(ctx context.Context, userID int) ([]models.WithdrawalInternal, error)
}
