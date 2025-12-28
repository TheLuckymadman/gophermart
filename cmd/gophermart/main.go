package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/TheLuckymadman/gophermart/internal/app"
	"github.com/TheLuckymadman/gophermart/internal/config"
	mux "github.com/TheLuckymadman/gophermart/internal/http"
	"github.com/TheLuckymadman/gophermart/internal/models"
	"github.com/TheLuckymadman/gophermart/internal/repository"
	"github.com/TheLuckymadman/gophermart/internal/service"
)

func run() error {
	logger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("init logger %w", err)
	}
	defer logger.Sync()

	logger.Info("server is starting")
	cfg := config.Load()
	app := app.NewApp(logger, cfg.Key, cfg.TokenExpTime)
	storage, err := repository.NewStorage(app, cfg.DatabaseURI, cfg.DBInitMode)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	workerSvc := service.NewAccrualWorker(app, storage.OrderRepo)
	ordersCh := make(chan models.Order, cfg.RateLimit)

	g, ctx := errgroup.WithContext(signalCtx)
	g.Go(func() error {
		return workerSvc.OrderScheduler(ctx, 1, ordersCh, cfg.DBReqFrequency, cfg.RateLimit)
	})

	for id := 0; id < cfg.RateLimit; id++ {
		id := id
		g.Go(func() error {
			return workerSvc.OrderProcessor(ctx, id, ordersCh, cfg.AccrualSystemAddress)
		})
	}

	userSvc := service.NewUserService(app, storage.UserRepo)
	orderSvc := service.NewOrderService(app, storage.OrderRepo)
	balanceSvc := service.NewBalanceService(app, storage.BalanceRepo)
	mux := mux.NewMux(app, userSvc, orderSvc, balanceSvc)
	srv := http.Server{
		Addr:    cfg.RunAddress,
		Handler: mux,
	}

	g.Go(func() error {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	logger.Info("server started",
		zap.String("interface", cfg.RunAddress),
		zap.Any("db init mode", cfg.DBInitMode),
	)

	g.Go(func() error {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		logger.Info("shutting down http server")
		return srv.Shutdown(shutdownCtx)
	})

	g.Go(func() error {
		<-ctx.Done()
		close(ordersCh)
		return nil
	})

	if err := g.Wait(); err != nil {
		logger.Error("unrecoverable error occured", zap.Error(err))
		return err
	}
	logger.Info("server stopped gracefully", zap.Int("goroutines left", runtime.NumGoroutine()))

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
