package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/TheLuckymadman/gophermart/internal/models"
)

func (p *PGStorage) CreateUser(ctx context.Context, login string, hash string) (int, error) {
	var id int
	err := p.DB.QueryRowContext(ctx, `INSERT INTO users (login, password)
		VALUES ($1, $2)
		RETURNING id
		`, login, hash).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create user: %w", err)
	}
	return id, nil
}

func (p *PGStorage) GetUser(ctx context.Context, login string) (*models.Users, error) {
	row := p.DB.QueryRowContext(ctx, `SELECT id, login, password
		FROM users
		WHERE login = $1`, login)
	u := models.Users{}
	err := row.Scan(&u.ID, &u.Login, &u.Password)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user %v not found", login)
		}
		return nil, err
	}
	return &u, nil
}
