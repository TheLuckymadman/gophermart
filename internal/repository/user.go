package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/TheLuckymadman/gophermart/internal/models"
	"github.com/TheLuckymadman/gophermart/internal/app"
)

type UserRepo struct {
	db  *sql.DB
	app *app.App
}

func NewUserRepo(db *sql.DB, app *app.App) *UserRepo {
	return &UserRepo{db: db, app: app}
}

func (r *UserRepo) CreateUser(ctx context.Context, login string, hash string) (int, error) {
	var id int
	err := r.db.QueryRowContext(ctx, `INSERT INTO users (login, password)
		VALUES ($1, $2)
		RETURNING id
		`, login, hash).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create user: %w", err)
	}
	return id, nil
}

func (r *UserRepo) GetUser(ctx context.Context, login string) (*models.Users, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, login, password
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
