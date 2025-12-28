package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/TheLuckymadman/gophermart/internal/app"
	"github.com/TheLuckymadman/gophermart/internal/models"
	"github.com/TheLuckymadman/gophermart/internal/utils"
	"go.uber.org/zap"
)

type AccrualWorker struct {
	app *app.App
	db  OrderUpdateStorage
}

func NewAccrualWorker(a *app.App, db OrderUpdateStorage) *AccrualWorker {
	return &AccrualWorker{app: a, db: db}
}

func (w *AccrualWorker) OrderScheduler(ctx context.Context, workerID int, ch chan<- models.Order, interval int, maxOrdersCnt int) error {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()
	reqAccStatuses := []string{
		models.OrderAccrualStatusNew,
		models.OrderAccrualStatusRegistred,
		models.OrderAccrualStatusProcessing,
		models.OrderAccrualStatusReqFailed,
		models.OrderAccrualStatusNotRegistred,
	}
	f := func() error {
		orders, err := w.db.ListOrdersToEnqueue(
			ctx,
			reqAccStatuses,
			models.OrderStatusProcessing,
			models.OrderAccrualStatusEnqueued,
			maxOrdersCnt,
		)
		if err != nil {
			w.app.Logger.Error("getting orders error", zap.Int("worker id", workerID), zap.Error(err))
			return err
		}

		errDetected := false
		for i, order := range orders {
			select {
			case ch <- order:
			case <-ctx.Done():
				for _, o := range orders[i:] {
					rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					err = w.db.UpdOrderTmpStatus(rollbackCtx, o.Number, o.AccrualStatus, models.OrderStatusProcessing)
					if err != nil {
						w.app.Logger.Error("update orders error", zap.Int("worker id", workerID), zap.Error(err))
						errDetected = true
						continue
					}
				}
				if errDetected {
					return fmt.Errorf("update orders error, see errors above")
				}
				return nil
			}
		}
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := f(); err != nil {
				return err
			}
		}
	}
}

func (w *AccrualWorker) OrderProcessor(ctx context.Context, workerID int, ch <-chan models.Order, accrualAddr string) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	f := func(order models.Order) error {
		err := w.db.UpdOrderTmpStatus(ctx, order.Number, models.OrderAccrualStatusChecking, models.OrderStatusProcessing)
		if err != nil {
			w.app.Logger.Error("update orders error", zap.Int("worker id", workerID), zap.Error(err))
			return err
		}

		url := fmt.Sprintf("%s/api/orders/%s", accrualAddr, order.Number)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			w.app.Logger.Error("new http request", zap.Int("worker id", workerID), zap.Error(err))
			err = w.db.UpdOrderTmpStatus(ctx, order.Number, models.OrderAccrualStatusReqFailed, models.OrderStatusProcessing)
			if err != nil {
				w.app.Logger.Error("update orders error", zap.Int("worker id", workerID), zap.Error(err))
				return err
			}
			return nil
		}
		w.app.Logger.Info("request to accrual", zap.String("request", url))
		resp, err := client.Do(req)
		if err != nil {
			w.app.Logger.Error("http request", zap.Int("worker id", workerID), zap.Error(err))
			err = w.db.UpdOrderTmpStatus(ctx, order.Number, models.OrderAccrualStatusReqFailed, models.OrderStatusProcessing)
			if err != nil {
				w.app.Logger.Error("update orders error", zap.Int("worker id", workerID), zap.Error(err))
				return err
			}
			return nil
		}
		defer resp.Body.Close()

		var httpErrResp bool
		var accrualStatus string

		switch resp.StatusCode {
		case http.StatusOK:
			httpErrResp = false
		case http.StatusNoContent:
			accrualStatus = models.OrderAccrualStatusNotRegistred
			httpErrResp = false
			err = w.db.UpdOrderTmpStatus(ctx, order.Number, accrualStatus, models.OrderStatusProcessing)
			if err != nil {
				w.app.Logger.Error("update orders error", zap.Int("worker id", workerID), zap.Error(err))
				return err
			}
			return nil
		case http.StatusTooManyRequests:
			accrualStatus = models.OrderAccrualStatusReqFailed
			httpErrResp = true
		default:
			accrualStatus = models.OrderAccrualStatusReqFailed
			httpErrResp = true
		}

		if httpErrResp {
			w.app.Logger.Error(
				"http request status",
				zap.Int("worker id", workerID),
				zap.String("accrual status", accrualStatus),
				zap.Int("response http code", resp.StatusCode),
			)
			err = w.db.UpdOrderTmpStatus(ctx, order.Number, accrualStatus, models.OrderStatusProcessing)
			if err != nil {
				w.app.Logger.Error("update orders error", zap.Int("worker id", workerID), zap.Error(err))
				return err
			}
			return nil
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			w.app.Logger.Error("http read body", zap.Int("worker id", workerID), zap.Error(err))
			err = w.db.UpdOrderTmpStatus(ctx, order.Number, models.OrderAccrualStatusReqFailed, models.OrderStatusProcessing)
			if err != nil {
				w.app.Logger.Error("update orders error", zap.Int("worker id", workerID), zap.Error(err))
				return err
			}
			return nil
		}
		w.app.Logger.Info("response from accrual", zap.String("response", string(body)))

		var accrualRespOrder models.AccrualRespOrder
		err = json.Unmarshal(body, &accrualRespOrder)
		if err != nil {
			w.app.Logger.Error("http unmarshal body", zap.Int("worker id", workerID), zap.Error(err))
			err = w.db.UpdOrderTmpStatus(ctx, order.Number, models.OrderAccrualStatusReqFailed, models.OrderStatusProcessing)
			if err != nil {
				w.app.Logger.Error("update order error", zap.Int("worker id", workerID), zap.Error(err))
				return err
			}
			return nil
		}
		w.app.Logger.Info("response from accrual", zap.Any("paylod", accrualRespOrder))

		if accrualRespOrder.Accrual != 0 {
			err = w.db.UpdOrderStatusAndUsrBalance(
				ctx,
				order.Number,
				models.OrderAccrualStatusProcessed,
				models.OrderStatusProcessed,
				utils.ConvertToCents(accrualRespOrder.Accrual),
				order.ID,
			)
			if err != nil {
				w.app.Logger.Error("update order error", zap.Int("worker id", workerID), zap.Error(err))
			} else {
				return nil
			}
		} else if accrualRespOrder.Status == models.OrderAccrualStatusProcessed ||
			accrualRespOrder.Status == models.OrderAccrualStatusInvalid {
			err = w.db.UpdOrderFinalStatus(ctx, order.Number, accrualRespOrder.Status, models.OrderStatusProcessed)
			if err != nil {
				w.app.Logger.Error("update order error", zap.Int("worker id", workerID), zap.Error(err))
			} else {
				return nil
			}
		} else {
			err = w.db.UpdOrderTmpStatus(ctx, order.Number, accrualRespOrder.Status, models.OrderStatusProcessing)
			if err != nil {
				w.app.Logger.Error("update order error", zap.Int("worker id", workerID), zap.Error(err))
			} else {
				return nil
			}
		}
		w.app.Logger.Error("update order error", zap.Int("worker id", workerID), zap.Error(err))
		err = w.db.UpdOrderTmpStatus(ctx, order.Number, models.OrderAccrualStatusReqFailed, models.OrderStatusProcessing)
		if err != nil {
			w.app.Logger.Error("update order error", zap.Int("worker id", workerID), zap.Error(err))
		}
		return fmt.Errorf("orderProcessor error occured, see errors above")
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case order, ok := <-ch:
			if !ok {
				return nil
			}
			if err := f(order); err != nil {
				return err
			}
		}
	}
}
