package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

const (
	// idempotencyKeyHeader lets a client retry an import safely: replaying a key returns the identifiers of the first import without creating anything.
	idempotencyKeyHeader    = "Idempotency-Key"
	maxIdempotencyKeyLength = 255
)

type importAdsBody struct {
	Ads []createAdBody `json:"ads"`
}

type importAdsResponse struct {
	IDs []int64 `json:"ids"`
}

func handlerImportAds(importAds usecase.ImportAdsFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		var body importAdsBody
		if err := decodeBody(r, &body); err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		key := strings.TrimSpace(r.Header.Get(idempotencyKeyHeader))
		if len(key) > maxIdempotencyKeyLength {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		// Validate every item before importing anything.
		items := make([]usecase.ImportAdItem, 0, len(body.Ads))
		for _, item := range body.Ads {
			priceCents, err := item.priceInCents()
			if err != nil {
				rw.WriteHeader(http.StatusBadRequest)
				return
			}

			items = append(items, usecase.ImportAdItem{
				Title:      item.Title,
				PriceCents: priceCents,
				PhotoURL:   item.PhotoURL,
			})
		}

		adIDs, err := importAds(r.Context(), usecase.ImportAdsRequest{
			OwnerID:        domain.UserID(middleware.UserIDFromRequest(r)),
			IdempotencyKey: key,
			Ads:            items,
		})
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		ids := make([]int64, 0, len(adIDs))
		for _, adID := range adIDs {
			ids = append(ids, int64(adID))
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(rw).Encode(importAdsResponse{IDs: ids})
	}
}
