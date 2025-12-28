package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"

	"github.com/TheLuckymadman/gophermart/internal/app"
)

type Storage struct {
	DB          *sql.DB
	App         *app.App
	UserRepo    *UserRepo
	OrderRepo   *OrderRepo
	BalanceRepo *BalanceRepo
}

func NewStorage(
	app *app.App,
	dsn string,
	mode DBInitMode,
) (*Storage, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open DB: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping DB: %w", err)
	}

	switch mode {
	case ExtManaged:
		log.Println("External managed DB is chosen")
	case IntManagedForce:
		log.Println("Internal managed DB is chosen with recreating DB entities")
		err := RunMigration(db, "migrations", DOWN)
		if err != nil {
			return nil, fmt.Errorf("init DB status %w", err)
		}
		err = RunMigration(db, "migrations", UP)
		if err != nil {
			return nil, fmt.Errorf("init DB status %w", err)
		}
	case IntManaged:
		log.Println("Internal managed DB is chosen, with creating DB entities if they don't exist")
		err := RunMigration(db, "migrations", UP)
		if err != nil {
			return nil, fmt.Errorf("init DB status %w", err)
		}
	default:
		log.Println("Wrong mode, use external managed DB as default")
	}

	userRepo := NewUserRepo(db, app)
	orderRepo, err := NewOrderRepo(db, app)
	if err != nil {
		return nil, err
	}
	balanceRepo := NewBalanceRepo(db, app)
	storage := Storage{DB: db, App: app, UserRepo: userRepo, OrderRepo: orderRepo, BalanceRepo: balanceRepo}

	return &storage, nil
}

type DBInitMode int

func (m *DBInitMode) UnmarshalText(text []byte) error {
	s := strings.ToLower(strings.TrimSpace(string(text)))
	switch s {
	case "external", "0":
		*m = ExtManaged
	case "internal", "1":
		*m = IntManaged
	case "reset", "forced", "2":
		*m = IntManagedForce
	}
	return nil
}

func (m *DBInitMode) MarshalText() (string, error) {
	switch *m {
	case 0:
		return "external", nil
	case 1:
		return "internal", nil
	case 2:
		return "reset", nil
	default:
		return "", fmt.Errorf("error unknown DBInitMode value %d", *m)
	}
}

const (
	ExtManaged DBInitMode = iota
	IntManaged
	IntManagedForce
)

type MigrationCMD int

const (
	UP MigrationCMD = iota
	DOWN
)

func RunMigration(db *sql.DB, migrationsDir string, cmd MigrationCMD) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("migrate: open driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsDir,
		"postgres", driver)
	if err != nil {
		return fmt.Errorf("migrate: new instance: %w", err)
	}
	switch cmd {
	case UP:
		err = m.Up()
		if err != nil {
			log.Printf("migrate up: %v", err)
		}
	case DOWN:
		log.Printf("Force down\n")
		v, d, err := m.Version()
		fmt.Printf("current DB version %v\n", v)
		if err != nil {
			if errors.Is(err, migrate.ErrNilVersion) {
				log.Printf("DB is empty and clean\n")
				return nil
			}
			return err
		}
		if d {
			log.Printf("DB is in a dirty state\n")
			err = m.Force(int(v))
			if err != nil {
				return err
			}
			_, _, err = m.Version()
			if err != nil {
				return err
			}
			log.Printf("DB dirty state now is: %t\n", d)
		}
		for !errors.Is(err, migrate.ErrNilVersion) {
			fmt.Printf("DB version before down: %v\n", v)
			err = m.Steps(-1)
			if err != nil {
				return fmt.Errorf("migrating down err: %w", err)
			}
			v, _, err = m.Version()
			fmt.Printf("DB version after down: %v\n", v)
		}
		if !errors.Is(err, migrate.ErrNilVersion) {
			return fmt.Errorf("migrate down result: %w", err)
		}
	default:
		return fmt.Errorf("wrong migrate cmd %d", cmd)
	}
	if err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

func (p *Storage) Close() error {
	return p.DB.Close()
}

func (p *Storage) PingDB(ctx context.Context) error {
	type result struct{}
	f := func() (result, error) {
		return result{}, p.DB.PingContext(ctx)
	}
	_, err := f()
	return err
}
