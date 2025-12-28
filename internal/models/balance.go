package models

import "time"

type Balance struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type WithdrawalResponse struct {
	Number     string    `json:"order"`
	Amount     float64   `json:"sum"`
	CreatedAt time.Time `json:"processed_at,omitempty"`
}

type WithdrawalInternal struct {
	Number      string
	AmountCents int64
	CreatedAt   time.Time
}
