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
	db  Storage
}

func NewAccrualWorker(a *app.App, db Storage) *AccrualWorker {
	return &AccrualWorker{app: a, db: db}
}

func (w *AccrualWorker) OrderScheduler(ctx context.Context, workerID int, ch chan<- models.Order, interval int, maxOrdersCnt int) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()
	reqAccStatuses := []string{
		models.OrderAccrualStatusNew,
		models.OrderAccrualStatusRegistred,
		models.OrderAccrualStatusProcessing,
		models.OrderAccrualStatusReqFailed,
		models.OrderAccrualStatusNotRegistred,
	}
	f := func() {
		orders, err := w.db.ListOrdersToEnqueue(
			ctx,
			reqAccStatuses,
			models.OrderStatusProcessing,
			models.OrderAccrualStatusEnqueued,
			maxOrdersCnt,
		)
		if err != nil {
			w.app.Logger.Error("getting orders error", zap.Int("worker id", workerID), zap.Error(err))
			return
		}

		sent := make(map[int]bool)
		for _, order := range orders {
			select {
			case ch <- order:
				sent[order.ID] = true
			case <-ctx.Done():
				for _, o := range orders {
					if !sent[o.ID] {
						err = w.db.UpdOrderTmpStatus(ctx, o.Number, o.AccrualStatus, models.OrderStatusProcessing)
						if err != nil {
							w.app.Logger.Error("update orders error", zap.Int("worker id", workerID), zap.Error(err))
							continue
						}
					}
				}
				return
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f()
		}
	}
}

func (w *AccrualWorker) OrderProcessor(ctx context.Context, workerID int, ch <-chan models.Order, accrualAddr string) {
	client := &http.Client{}
	f := func(order models.Order) {
		err := w.db.UpdOrderTmpStatus(ctx, order.Number, models.OrderAccrualStatusChecking, models.OrderStatusProcessing)
		if err != nil {
			w.app.Logger.Error("update orders error", zap.Int("worker id", workerID), zap.Error(err))
			return
		}

		url := fmt.Sprintf("%s/api/orders/%s", accrualAddr, order.Number)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			w.app.Logger.Error("new http request", zap.Int("worker id", workerID), zap.Error(err))
			err = w.db.UpdOrderTmpStatus(ctx, order.Number, models.OrderAccrualStatusReqFailed, models.OrderStatusProcessing)
			if err != nil {
				w.app.Logger.Error("update orders error", zap.Int("worker id", workerID), zap.Error(err))
			}
			return
		}
		w.app.Logger.Info("request to accrual", zap.String("request", url))
		resp, err := client.Do(req)
		if err != nil {
			w.app.Logger.Error("http request", zap.Int("worker id", workerID), zap.Error(err))
			err = w.db.UpdOrderTmpStatus(ctx, order.Number, models.OrderAccrualStatusReqFailed, models.OrderStatusProcessing)
			if err != nil {
				w.app.Logger.Error("update orders error", zap.Int("worker id", workerID), zap.Error(err))
			}
			return
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
			}
			return
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
			}
			return
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			w.app.Logger.Error("http read body", zap.Int("worker id", workerID), zap.Error(err))
			err = w.db.UpdOrderTmpStatus(ctx, order.Number, models.OrderAccrualStatusReqFailed, models.OrderStatusProcessing)
			if err != nil {
				w.app.Logger.Error("update orders error", zap.Int("worker id", workerID), zap.Error(err))
			}
			return
		}
		w.app.Logger.Info("response from accrual", zap.String("response", string(body)))

		var accrualRespOrder models.AccrualRespOrder
		err = json.Unmarshal(body, &accrualRespOrder)
		if err != nil {
			w.app.Logger.Error("http unmarshal body", zap.Int("worker id", workerID), zap.Error(err))
			err = w.db.UpdOrderTmpStatus(ctx, order.Number, models.OrderAccrualStatusReqFailed, models.OrderStatusProcessing)
			if err != nil {
				w.app.Logger.Error("update order error", zap.Int("worker id", workerID), zap.Error(err))
			}
			return
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
				return
			}
		} else if accrualRespOrder.Status == models.OrderAccrualStatusProcessed ||
			accrualRespOrder.Status == models.OrderAccrualStatusInvalid {
			err = w.db.UpdOrderFinalStatus(ctx, order.Number, accrualRespOrder.Status, models.OrderStatusProcessed)
			if err != nil {
				w.app.Logger.Error("update order error", zap.Int("worker id", workerID), zap.Error(err))
			} else {
				return
			}
		} else {
			err = w.db.UpdOrderTmpStatus(ctx, order.Number, accrualRespOrder.Status, models.OrderStatusProcessing)
			if err != nil {
				w.app.Logger.Error("update order error", zap.Int("worker id", workerID), zap.Error(err))
			} else {
				return
			}
		}
		w.app.Logger.Error("update order error", zap.Int("worker id", workerID), zap.Error(err))
		err = w.db.UpdOrderTmpStatus(ctx, order.Number, models.OrderAccrualStatusReqFailed, models.OrderStatusProcessing)
		if err != nil {
			w.app.Logger.Error("update order error", zap.Int("worker id", workerID), zap.Error(err))
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case order, ok := <-ch:
			if !ok {
				w.app.Logger.Error("channel returned not ok", zap.Int("worker id", workerID))
				continue
			}
			f(order)
		}
	}
}
