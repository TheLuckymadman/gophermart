package models

import (
	"time"
)

const (
	// collect rewards status
	OrderStatusNew        = "NEW"
	OrderStatusProcessing = "PROCESSING"
	OrderStatusInvalid    = "INVALID"
	OrderStatusProcessed  = "PROCESSED"

	// accrual statuses
	OrderAccrualStatusNew          = "NEW"
	OrderAccrualStatusEnqueued     = "ENQUEUED"
	OrderAccrualStatusChecking     = "CHECKING"
	OrderAccrualStatusReqFailed    = "REQ_FAILED"
	OrderAccrualStatusRegistred    = "REGISTRED"
	OrderAccrualStatusNotRegistred = "NOT_REGISTRED"
	OrderAccrualStatusInvalid      = "INVALID"
	OrderAccrualStatusProcessing   = "PROCESSING"
	OrderAccrualStatusProcessed    = "PROCESSED"

	OperTypeAccrual    = "ACCRUAL"
	OperTypeWithdrawal = "Withdrawal"
)

type Order struct {
	ID            int       `json:"-"`
	Number        string    `json:"number"`
	Status        string    `json:"status"`
	Accrual       int64     `json:"accrual,omitempty"`
	AccrualStatus string    `json:"-"`
	RetryCnt      int       `kson:"-"`
	CreatedAt     time.Time `json:"uploaded_at"`
	ProcessedAt   time.Time `json:"-"`
	UserID        int       `json:"-"`
	SentToChan    bool      `json:"-"`
}

type OrderResponse struct {
	Number    string    `json:"number"`
	Status    string    `json:"status"`
	Accrual   float64   `json:"accrual,omitempty"`
	CreatedAt time.Time `json:"uploaded_at"`
}

type AccrualRespOrder struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual"`
}
