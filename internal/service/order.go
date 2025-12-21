package service

import (
	"context"
	"fmt"

	"github.com/TheLuckymadman/gophermart/internal/app"
	"github.com/TheLuckymadman/gophermart/internal/models"
)

type OrderService struct {
	app *app.App
	db  Storage
}

func NewOrderService(app *app.App, db Storage) *OrderService {
	return &OrderService{app: app, db: db}
}

func (s *OrderService) NewOrder(ctx context.Context, u *models.Users, number string) error {
	err := s.db.CreateOrder(ctx, u.ID, number)
	if err != nil {
		return fmt.Errorf("new order: %w", err)
	}
	return nil
}

func (s *OrderService) GetOrders(ctx context.Context, u *models.Users) ([]models.Order, error) {
	orders, err := s.db.ListOrdersByUser(ctx, u.ID)
	if err != nil {
		return nil, fmt.Errorf("get orders: %w", err)
	}
	return orders, nil
}
