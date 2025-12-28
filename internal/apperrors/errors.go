package apperrors

import "errors"

var ErrInsufficientFunds = errors.New("insufficient funds")
var ErrUserAlreadyHasOrder = errors.New("user already has an order with this number")
var ErrAnotherUserAlreadyHasOrder = errors.New("another user already has an order with this number")
