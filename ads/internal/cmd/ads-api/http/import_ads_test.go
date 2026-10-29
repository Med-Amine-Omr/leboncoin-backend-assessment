package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/middleware"
)

func newImportAdsServer(fn usecase.ImportAdsFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /ads/import", middleware.RequireAuth(handlerImportAds(fn)))

	return httptest.NewServer(mux)
}

func postImport(t *testing.T, url string, body any, authHeader, idempotencyKey string) *http.Response {
	t.Helper()

	b, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	return resp
}

func TestHandlerImportAds(t *testing.T) {
	body := importAdsBody{Ads: []createAdBody{
		{Title: "bike", PriceCents: 5000, PhotoURL: "https://example.com/bike.jpg"},
		{Title: "car", PriceCents: 200000, PhotoURL: "https://example.com/car.jpg"},
	}}

	t.Run("returns 201 with all generated IDs on success", func(t *testing.T) {
		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			return []domain.AdID{10, 11}, nil
		})
		defer srv.Close()

		resp := postImport(t, srv.URL+"/ads/import", body, "user-1", "")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var got importAdsResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, []int64{10, 11}, got.IDs)
	})

	t.Run("passes the owner, the items and the idempotency key to the usecase", func(t *testing.T) {
		var received usecase.ImportAdsRequest
		srv := newImportAdsServer(func(_ context.Context, req usecase.ImportAdsRequest) ([]domain.AdID, error) {
			received = req
			return []domain.AdID{10, 11}, nil
		})
		defer srv.Close()

		resp := postImport(t, srv.URL+"/ads/import", body, "user-42", "key-1")
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, usecase.ImportAdsRequest{
			OwnerID:        "user-42",
			IdempotencyKey: "key-1",
			Ads: []usecase.ImportAdItem{
				{Title: "bike", PriceCents: 5000, PhotoURL: "https://example.com/bike.jpg"},
				{Title: "car", PriceCents: 200000, PhotoURL: "https://example.com/car.jpg"},
			},
		}, received)
	})

	t.Run("returns 201 with empty IDs when body has no ads", func(t *testing.T) {
		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			return []domain.AdID{}, nil
		})
		defer srv.Close()

		resp := postImport(t, srv.URL+"/ads/import", importAdsBody{Ads: []createAdBody{}}, "user-1", "")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var got importAdsResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Empty(t, got.IDs)
	})

	t.Run("returns 401 when Authorization header is missing", func(t *testing.T) {
		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			t.Fatal("usecase should not be called")
			return nil, nil
		})
		defer srv.Close()

		resp := postImport(t, srv.URL+"/ads/import", body, "", "")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("returns 400 on invalid body", func(t *testing.T) {
		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			t.Fatal("usecase should not be called")
			return nil, nil
		})
		defer srv.Close()

		req, err := http.NewRequest(http.MethodPost, srv.URL+"/ads/import", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "user-1")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 400 and imports nothing when one price is invalid", func(t *testing.T) {
		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			t.Fatal("usecase should not be called")
			return nil, nil
		})
		defer srv.Close()

		valid, invalid := 50.0, 19.999
		resp := postImport(t, srv.URL+"/ads/import", importAdsBody{Ads: []createAdBody{
			{Title: "bike", Price: &valid},
			{Title: "scooter", Price: &invalid},
		}}, "user-1", "")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 400 when the idempotency key is too long", func(t *testing.T) {
		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			t.Fatal("usecase should not be called")
			return nil, nil
		})
		defer srv.Close()

		resp := postImport(t, srv.URL+"/ads/import", body, "user-1", strings.Repeat("k", 256))
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns 500 when the usecase errors", func(t *testing.T) {
		srv := newImportAdsServer(func(_ context.Context, _ usecase.ImportAdsRequest) ([]domain.AdID, error) {
			return nil, errors.New("db down")
		})
		defer srv.Close()

		resp := postImport(t, srv.URL+"/ads/import", body, "user-1", "")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}
