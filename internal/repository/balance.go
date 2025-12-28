package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/TheLuckymadman/gophermart/internal/app"
	"github.com/TheLuckymadman/gophermart/internal/apperrors"
	"github.com/TheLuckymadman/gophermart/internal/models"
)

type BalanceRepo struct {
	db  *sql.DB
	app *app.App
	*BaseRepo[struct{}]
}

func NewBalanceRepo(db *sql.DB, app *app.App) *BalanceRepo {
	return &BalanceRepo{db: db, app: app, BaseRepo: NewBaseRepo[struct{}](db)}
}

func (r *BalanceRepo) GetBalance(ctx context.Context, userID int) (int64, int64, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT balance, withdrawn
		FROM users
		WHERE id = $1
		`, userID)

	var balance int64
	var withdrawn int64
	err := row.Scan(&balance, &withdrawn)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, fmt.Errorf("user id %d not found", err)
		}
		err = fmt.Errorf("get balance: %w", err)
		return 0, 0, err
	}
	return balance, withdrawn, nil
}

func (r *BalanceRepo) Withdraw(ctx context.Context, userID int, amount int64, number string) (err error) {
	fn := func(tx *sql.Tx) (struct{}, error) {
		row := tx.QueryRowContext(ctx, `
		SELECT balance
		FROM users
		WHERE id = $1
		FOR UPDATE
		`, userID)

		var balance int64
		if err = row.Scan(&balance); err != nil {
			err = fmt.Errorf("select for update users on withdrawing: %w", err)
			return struct{}{}, err
		}
		newBalance := balance - amount
		if newBalance < 0 {
			err = apperrors.ErrInsufficientFunds
			return struct{}{}, err
		}
		var withdrawalID int
		err = tx.QueryRowContext(ctx, `
			INSERT INTO withdrawals (number, withdrawn, user_id)
			VALUES ($1, $2, $3)	
			RETURNING id
		`, number, amount, userID).Scan(&withdrawalID)
		if err != nil {
			err = fmt.Errorf("insert into withdrawals on withdrawing: %w", err)
			return struct{}{}, err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO operations (type, amount, user_id, withdrawal_id)
			VALUES ($1, $2, $3, $4)
		`, models.OperTypeWithdrawal, amount, userID, withdrawalID)
		if err != nil {
			err = fmt.Errorf("insert into operations on withdrawing: %w", err)
			return struct{}{}, err
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE users
			SET balance = $1, 
				withdrawn = withdrawn + $2
			WHERE id = $3
		`, newBalance, amount, userID)
		if err != nil {
			err = fmt.Errorf("update users on withdrawing: %w", err)
			return struct{}{}, err
		}
		return struct{}{}, nil
	}

	_, err = r.withTx(ctx, fn)

	return err
}

func (r *BalanceRepo) ListWithdrawals(ctx context.Context, userID int) ([]models.WithdrawalInternal, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT number, withdrawn, created_at
		FROM withdrawals
		WHERE user_id = $1 
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list withdrawals: %w", err)
	}
	defer rows.Close()

	var w models.WithdrawalInternal
	var withdrawals []models.WithdrawalInternal
	for rows.Next() {
		if err = rows.Scan(&w.Number, &w.AmountCents, &w.CreatedAt); err != nil {
			return nil, fmt.Errorf("list withdrawals: %w", err)
		}
		withdrawals = append(withdrawals, w)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("list withdrawals: %w", err)
	}
	return withdrawals, nil
}
