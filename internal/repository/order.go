package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/lib/pq"

	"github.com/TheLuckymadman/gophermart/internal/app"
	"github.com/TheLuckymadman/gophermart/internal/apperrors"
	"github.com/TheLuckymadman/gophermart/internal/models"
)

type OrderRepo struct {
	db  *sql.DB
	app *app.App
	*BaseRepo[any]
}

func NewOrderRepo(db *sql.DB, app *app.App) (*OrderRepo, error) {
	orderRepo := &OrderRepo{db: db, app: app}
	if err := orderRepo.ResetProcessingOrdersRetry(context.Background()); err != nil {
		return nil, fmt.Errorf("cannot reset order retries: %w", err)
	}
	return &OrderRepo{db: db, app: app, BaseRepo: NewBaseRepo[any](db)}, nil
}

func (r *OrderRepo) scanOrders(rows *sql.Rows) ([]models.Order, error) {
	var orders []models.Order
	for rows.Next() {
		var order models.Order
		var processedAt *time.Time
		err := rows.Scan(&order.ID, &order.Number, &order.Status, &order.Accrual, &order.AccrualStatus,
			&order.RetryCnt, &order.CreatedAt, &processedAt, &order.UserID)
		if err != nil {
			return nil, fmt.Errorf("read orders: %w", err)
		}
		if processedAt != nil {
			order.ProcessedAt = *processedAt
		} else {
			order.ProcessedAt = time.Time{}
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read orders: %w", err)
	}

	return orders, nil
}

func (r *OrderRepo) CreateOrder(ctx context.Context, userID int, number string) (err error) {
	fn := func(tx *sql.Tx) (any, error) {
		var orderID int
		err = tx.QueryRowContext(ctx, `
			INSERT INTO orders (number, status, accrual_status, user_id)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (number) DO NOTHING
			RETURNING id
			`, number, models.OrderStatusNew, models.OrderAccrualStatusNew, userID).Scan(&orderID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				var conflictUserID int
				err = tx.QueryRowContext(ctx, `
				SELECT user_id
				FROM orders
				WHERE number = $1
				`, number).Scan(&conflictUserID)
				if err != nil {
					return struct{}{}, fmt.Errorf("select user_id in orders by number: %w", err)
				}
				if conflictUserID == userID {
					return struct{}{}, apperrors.ErrUserAlreadyHasOrder
				}
				return struct{}{}, apperrors.ErrAnotherUserAlreadyHasOrder
			} else {
				err = fmt.Errorf("insert order: %w", err)
				return struct{}{}, err
			}
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO operations (type, user_id, order_id)
			VALUES ($1,$2,$3)`, models.OperTypeAccrual, userID, orderID)
		if err != nil {
			err = fmt.Errorf("insert operation: %w", err)
			return struct{}{}, err
		}
		return struct{}{}, nil
	}
	_, err = r.withTx(ctx, fn)
	return err
}

func (r *OrderRepo) ListOrdersByUser(ctx context.Context, userID int) ([]models.Order, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, number, status, accrual, accrual_status, retry_cnt, created_at, processed_at, user_id
		FROM orders
		WHERE user_id = $1
		ORDER BY created_at DESC
		`, userID)
	if err != nil {
		return nil, fmt.Errorf("read orders: %w", err)
	}
	defer rows.Close()
	return r.scanOrders(rows)
}

func (r *OrderRepo) ListOrdersByStatus(ctx context.Context, statuses []string) ([]models.Order, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, number, status, accrual, accrual_status, retry_cnt, created_at, processed_at, user_id
		FROM orders
		WHERE status = ANY($1)`, pq.Array(statuses))
	if err != nil {
		return nil, fmt.Errorf("read orders: %w", err)
	}
	defer rows.Close()
	return r.scanOrders(rows)
}

func (r *OrderRepo) ListOrdersToEnqueue(
	ctx context.Context,
	reqStatuses []string,
	updStatus string,
	updAccStatus string,
	maxOrdersCnt int,
) (m []models.Order, err error) {
	fn := func(tx *sql.Tx) (any, error) {
		rows, err := tx.QueryContext(ctx, `
		SELECT id, number, status, accrual, accrual_status, retry_cnt, created_at, processed_at, user_id 
		FROM orders
		WHERE accrual_status = ANY($1)
			AND (
				accrual_status = 'NEW' 
				OR
				processed_at + LEAST(
					(exp(retry_cnt) * interval '1 second'),
					interval '1 hour'
				) <= now()
			)
		FOR UPDATE SKIP LOCKED
		LIMIT $2
		`, reqStatuses, maxOrdersCnt)
		if err != nil {
			err = fmt.Errorf("select orders for update: %w", err)
			return nil, err
		}
		defer rows.Close()

		var order models.Order
		var orders []models.Order
		var ids []int
		var processedAt *time.Time
		for rows.Next() {
			err := rows.Scan(
				&order.ID,
				&order.Number,
				&order.Status,
				&order.Accrual,
				&order.AccrualStatus,
				&order.RetryCnt,
				&order.CreatedAt,
				&processedAt,
				&order.UserID)
			if err != nil {
				err = fmt.Errorf("scan orders for update: %w", err)
				return nil, err
			}
			if processedAt != nil {
				order.ProcessedAt = *processedAt
			} else {
				order.ProcessedAt = time.Time{}
			}
			orders = append(orders, order)
			ids = append(ids, order.ID)
		}
		if err = rows.Err(); err != nil {
			err = fmt.Errorf("scan orders for update: %w", err)
			return nil, err
		}
		_, err = tx.ExecContext(ctx, `UPDATE orders 
			SET status = $1,
				accrual_status = $2
			WHERE id = ANY($3)
			`, updStatus, updAccStatus, ids)
		if err != nil {
			err = fmt.Errorf("update orders: %w", err)
			return nil, err
		}
		return orders, nil
	}
	result, err := r.withTx(ctx, fn)
	if err != nil {
		return nil, err
	}
	orders := result.([]models.Order)
	return orders, err
}

func (r *OrderRepo) UpdOrderTmpStatus(ctx context.Context, number string, updAccStatus string, updStatus string) (err error) {
	_, err = r.db.ExecContext(ctx, `
		UPDATE orders 
		SET accrual_status = $1,
			status = $2,
			retry_cnt = retry_cnt + 1,
			processed_at = now()
		WHERE number = $3
		`, updAccStatus, updStatus, number)
	if err != nil {
		err = fmt.Errorf("update orders: %w", err)
		return
	}
	return
}

func (r *OrderRepo) UpdOrderFinalStatus(ctx context.Context, number string, updAccStatus string, updStatus string) (err error) {
	_, err = r.db.ExecContext(ctx, `
		UPDATE orders 
		SET accrual_status = $1,
			status = $2,
			processed_at = now()
		WHERE number = $3
		`, updAccStatus, updStatus, number)
	if err != nil {
		err = fmt.Errorf("update orders: %w", err)
		return
	}
	return
}

func (r *OrderRepo) UpdOrderStatusAndUsrBalance(
	ctx context.Context,
	number string,
	updAccStatus string,
	updStatus string,
	accrual int64,
	orderID int) (err error) {
	fn := func(tx *sql.Tx) (any, error) {
		var userID int
		err = tx.QueryRowContext(ctx, `
			SELECT user_id 
			FROM orders
			WHERE number = $1
			FOR UPDATE
			`, number).Scan(&userID)
		if err != nil {
			err = fmt.Errorf("select for update orders: %w", err)
			return struct{}{}, err
		}
		err = tx.QueryRowContext(ctx, `
			SELECT id 
			FROM users
			WHERE id = $1
			FOR UPDATE
			`, userID).Scan(&userID)
		if err != nil {
			err = fmt.Errorf("select for update users: %w", err)
			return struct{}{}, err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO operations (type, amount, user_id, order_id)
			VALUES ($1,$2,$3,$4)
			`, models.OperTypeAccrual, accrual, userID, orderID)
		if err != nil {
			err = fmt.Errorf("insert operation: %w", err)
			return struct{}{}, err
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE orders
			SET accrual_status = $1,
				status = $2,
				accrual = $3,
				processed_at = now()
			WHERE number = $4
			`, updAccStatus, updStatus, accrual, number)
		if err != nil {
			err = fmt.Errorf("update orders: %w", err)
			return struct{}{}, err
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE users
			SET balance = users.balance + $1
			WHERE id = $2
			`, accrual, userID)
		if err != nil {
			err = fmt.Errorf("update users: %w", err)
			return struct{}{}, err
		}
		return struct{}{}, nil
	}
	_, err = r.withTx(ctx, fn)
	return err
}

func (r *OrderRepo) ResetProcessingOrdersRetry(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE orders
		SET retry_cnt = 0
		WHERE status = 'PROCESSING' 
	`)
	if err != nil {
		err = fmt.Errorf("update users: %w", err)
		return err
	}
	return nil
}
