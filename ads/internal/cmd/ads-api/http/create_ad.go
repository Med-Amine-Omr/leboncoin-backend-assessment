package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

type createAdBody struct {
	Title string `json:"title"`
	// Deprecated: use Price. Kept so existing web and mobile clients keep working.
	PriceCents int64 `json:"price_cents"`
	// Price is the price in euros, with at most 2 decimals. It is mutually exclusive with PriceCents.
	Price    *float64 `json:"price"`
	PhotoURL string   `json:"photo_url"`
}

type createAdResponse struct {
	ID int64 `json:"id"`
}

// handlerCreateAd serves POST /ads.
func handlerCreateAd(createAd usecase.CreateAdFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		var body createAdBody
		if err := decodeBody(r, &body); err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		priceCents, err := body.priceInCents()
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		adID, err := createAd(r.Context(), usecase.CreateAdRequest{
			Title:      body.Title,
			PriceCents: priceCents,
			PhotoURL:   body.PhotoURL,
			OwnerID:    middleware.UserIDFromHeader(r),
		})
		if err != nil {
			if errors.Is(err, usecase.ErrQuotaExceeded) {
				rw.WriteHeader(http.StatusTooManyRequests)
				return
			}

			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(rw).Encode(createAdResponse{ID: int64(adID)})
	}
}

// priceInCents returns the ad price in cents, whichever field the client used.
func (b createAdBody) priceInCents() (int64, error) {
	if b.Price == nil {
		return b.PriceCents, nil
	}

	if b.PriceCents != 0 {
		return 0, errors.New("price and price_cents are mutually exclusive")
	}

	return priceToCents(*b.Price)
}
