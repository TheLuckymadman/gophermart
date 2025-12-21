package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/TheLuckymadman/gophermart/internal/apperrors"
	"github.com/TheLuckymadman/gophermart/internal/models"
)

func (p *PGStorage) GetBalance(ctx context.Context, userID int) (int64, int64, error) {
	row := p.DB.QueryRowContext(ctx, `
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

func (p *PGStorage) Withdraw(ctx context.Context, userID int, amount int64, number string) (err error) {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		err = fmt.Errorf("tx creation on withdrawing: %w", err)
		return
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()
	row := tx.QueryRowContext(ctx, `
		SELECT balance
		FROM users
		WHERE id = $1
		FOR UPDATE
	`, userID)

	var balance int64
	if err = row.Scan(&balance); err != nil {
		err = fmt.Errorf("select for update users on withdrawing: %w", err)
		return
	}
	newBalance := balance - amount
	if newBalance < 0 {
		err = apperrors.ErrInsufficientFunds
		return
	}
	var withdrawalID int
	err = tx.QueryRowContext(ctx, `
		INSERT INTO withdrawals (number, withdrawn, user_id)
		VALUES ($1, $2, $3)	
		RETURNING id
	`, number, amount, userID).Scan(&withdrawalID)
	if err != nil {
		err = fmt.Errorf("insert into withdrawals on withdrawing: %w", err)
		return
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO operations (type, amount, user_id, withdrawal_id)
		VALUES ($1, $2, $3, $4)
	`, models.OperTypeWithdrawal, amount, userID, withdrawalID)
	if err != nil {
		err = fmt.Errorf("insert into operations on withdrawing: %w", err)
		return
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE users
		SET balance = $1, 
			withdrawn = withdrawn + $2
		WHERE id = $3
	`, newBalance, amount, userID)
	if err != nil {
		err = fmt.Errorf("update users on withdrawing: %w", err)
		return
	}
	return
}

func (p *PGStorage) ListWithdrawals(ctx context.Context, userID int) ([]models.WithdrawalInternal, error) {
	rows, err := p.DB.QueryContext(ctx, `
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
