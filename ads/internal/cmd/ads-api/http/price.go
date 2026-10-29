package http

import (
	"errors"
	"math"
)

// maxExactCents is the largest amount of cents a float64 represents exactly (2^53).
const maxExactCents = 1 << 53

var errInvalidPrice = errors.New("price must be a finite amount with at most 2 decimals")

// centsToPrice converts an amount in cents, the unit used internally, to the decimal price exposed by the API.
func centsToPrice(cents int64) float64 {
	return float64(cents) / 100
}

// priceToCents converts a decimal price received by the API to cents. It rejects prices with more than 2 decimals instead of silently rounding them.
func priceToCents(price float64) (int64, error) {
	scaled := price * 100
	cents := math.Round(scaled)

	if math.IsNaN(scaled) || math.Abs(cents) > maxExactCents || math.Abs(scaled-cents) > 1e-6 {
		return 0, errInvalidPrice
	}

	return int64(cents), nil
}
