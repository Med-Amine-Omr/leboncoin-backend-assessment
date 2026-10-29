package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

type getAdResponse struct {
	ID         int64     `json:"id"`
	Title      string    `json:"title"`
	PriceCents int64     `json:"price_cents"`
	PhotoURL   string    `json:"photo_url"`
	UserID     string    `json:"user_id"`
	Username   *string   `json:"username"`
	CreatedAt  time.Time `json:"created_at"`
}

func adToGetAdResponse(d usecase.AdDetails) getAdResponse {
	resp := getAdResponse{
		ID:         int64(d.ID),
		Title:      d.Title,
		PriceCents: d.Price,
		PhotoURL:   d.PhotoURL,
		UserID:     d.UserID.String(),
		CreatedAt:  d.CreatedAt.UTC(),
	}

	// The username is null rather than absent when the user service could not provide it, so clients get a stable schema.
	if d.Username != "" {
		resp.Username = &d.Username
	}

	return resp
}

func handlerGetAd(getAd usecase.GetAdByIDFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		adID, err := adIDFromURL(r)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)

			return
		}

		details, err := getAd(r.Context(), adID)
		if err != nil {
			if errors.Is(err, adsdao.ErrNotFound) {
				rw.WriteHeader(http.StatusNotFound)
			} else {
				rw.WriteHeader(http.StatusInternalServerError)
			}

			return
		}

		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(adToGetAdResponse(details))
	}
}
