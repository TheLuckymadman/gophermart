package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type BaseRepo[T any] struct {
	db *sql.DB
}

func NewBaseRepo[T any](db *sql.DB) *BaseRepo[T] {
	return &BaseRepo[T]{db: db}
}

func (r *BaseRepo[T]) withTx(ctx context.Context, fn func(*sql.Tx) (T, error)) (T, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		err = fmt.Errorf("tx creation: %w", err)
		return *new(T), err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	result, err := fn(tx)
	if err != nil {
		return *new(T), err
	}
	err = tx.Commit()
	if err != nil {
		return *new(T), fmt.Errorf("tx commit: %w", err)
	}
	committed = true
	return result, nil
}
