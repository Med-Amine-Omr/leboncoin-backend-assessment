package http

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCentsToPrice(t *testing.T) {
	assert.Equal(t, 19.99, centsToPrice(1999))
	assert.Equal(t, 150.0, centsToPrice(15000))
	assert.Equal(t, 0.0, centsToPrice(0))
}

func TestPriceToCents(t *testing.T) {
	valid := []struct {
		price float64
		want  int64
	}{
		{price: 19.99, want: 1999},
		{price: 0.1, want: 10},
		{price: 150, want: 15000},
		{price: 0, want: 0},
	}
	for _, tc := range valid {
		got, err := priceToCents(tc.price)

		require.NoError(t, err, "price %v", tc.price)
		assert.Equal(t, tc.want, got, "price %v", tc.price)
	}

	invalid := []float64{19.999, 0.001, math.Inf(1), math.NaN(), 1e300}
	for _, price := range invalid {
		_, err := priceToCents(price)

		assert.ErrorIs(t, err, errInvalidPrice, "price %v", price)
	}
}
