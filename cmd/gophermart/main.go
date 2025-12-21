package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"

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
	db, err := repository.NewPGDB(app, cfg.DatabaseURI, cfg.DBInitMode)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	workerSvc := service.NewAccrualWorker(app, db)
	ordersCh := make(chan models.Order, cfg.RateLimit)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		workerSvc.OrderScheduler(ctx, 1, ordersCh, cfg.DBReqFrequency, cfg.RateLimit)
	}()

	for id := range cfg.RateLimit {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			workerSvc.OrderProcessor(ctx, id, ordersCh, cfg.AccrualSystemAddress)
		}(id)
	}

	userSvc := service.NewUserService(app, db)
	orderSvc := service.NewOrderService(app, db)
	balanceSvc := service.NewBalanceService(app, db)
	mux := mux.NewMux(app, userSvc, orderSvc, balanceSvc)
	srv := http.Server{
		Addr:    cfg.RunAddress,
		Handler: mux,
	}

	serverErrCh := make(chan error)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
			stop()
		}
	}()

	logger.Info("server started",
		zap.String("interface", cfg.RunAddress),
		zap.Any("db init mode", cfg.DBInitMode),
	)

	select {
	case err := <-serverErrCh:
		logger.Error("http server error", zap.Error(err))
	case <-ctx.Done():
		{
			logger.Info("shutdown signal received, the system is shutting down...")
			logger.Info("waiting for workers to shutdown gracefully")
			wg.Wait()
			logger.Info("workers shutdown gracefully")

			shutdownCtx, stop := context.WithTimeout(ctx, time.Second*5)
			defer stop()

			logger.Info("trying to stop http server")
			if err := srv.Shutdown(shutdownCtx); err != nil {
				_ = srv.Close()
				return err
			}
			logger.Info("server stopped gracefully", zap.Int("goroutines left", runtime.NumGoroutine()))
		}
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
