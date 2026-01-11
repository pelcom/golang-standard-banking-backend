package handlers

import (
	"errors"

	"banking/internal/money"

	"github.com/shopspring/decimal"
)

var errInvalidAmount = errors.New("invalid amount")
var errInvalidRate = errors.New("invalid rate")

func parseAmountMinor(raw string) (int64, error) {
	amount, err := money.ParseMinor(raw)
	if err != nil || amount <= 0 {
		return 0, errInvalidAmount
	}
	return amount, nil
}

func parseRate(raw string) (decimal.Decimal, error) {
	rate, err := decimal.NewFromString(raw)
	if err != nil || rate.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, errInvalidRate
	}
	if rate.Exponent() < -6 {
		return decimal.Zero, errInvalidRate
	}
	return rate, nil
}
