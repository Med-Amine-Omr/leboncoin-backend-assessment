package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	mockdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/mock"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

func TestImportAds(t *testing.T) {
	ctx := context.Background()
	bike := adsdao.CreateAdRequest{Title: "bike", PriceCents: 5000, PhotoURL: "bike.jpg", OwnerID: "user-1"}
	car := adsdao.CreateAdRequest{Title: "car", PriceCents: 200000, PhotoURL: "car.jpg", OwnerID: "user-1"}
	items := []usecase.ImportAdItem{
		{Title: "bike", PriceCents: 5000, PhotoURL: "bike.jpg"},
		{Title: "car", PriceCents: 200000, PhotoURL: "car.jpg"},
	}

	t.Run("creates every ad in a single transaction", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("Begin", ctx).Return(nil)
		m.On("CreateAd", ctx, bike).Return(domain.AdID(10), nil)
		m.On("CreateAd", ctx, car).Return(domain.AdID(11), nil)
		m.On("Commit").Return(nil)
		m.On("Rollback").Return(nil)

		got, err := usecase.NewImportAdsFunc(m)(ctx, usecase.ImportAdsRequest{OwnerID: "user-1", Ads: items})

		require.NoError(t, err)
		assert.Equal(t, []domain.AdID{10, 11}, got)
	})

	t.Run("rolls back without committing when one creation fails", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("db down")
		m.On("Begin", ctx).Return(nil)
		m.On("CreateAd", ctx, bike).Return(domain.AdID(10), nil)
		m.On("CreateAd", ctx, car).Return(domain.AdID(0), dbErr)
		m.On("Rollback").Return(nil) // no Commit expectation: committing would fail the test

		_, err := usecase.NewImportAdsFunc(m)(ctx, usecase.ImportAdsRequest{OwnerID: "user-1", Ads: items})

		require.Error(t, err)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("claims the idempotency key and records the created ads", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("Begin", ctx).Return(nil)
		m.On("ClaimImport", ctx, domain.UserID("user-1"), "key-1").Return(true, nil, nil)
		m.On("CreateAd", ctx, bike).Return(domain.AdID(10), nil)
		m.On("CreateAd", ctx, car).Return(domain.AdID(11), nil)
		m.On("CompleteImport", ctx, domain.UserID("user-1"), "key-1", []domain.AdID{10, 11}).Return(nil)
		m.On("Commit").Return(nil)
		m.On("Rollback").Return(nil)

		got, err := usecase.NewImportAdsFunc(m)(ctx, usecase.ImportAdsRequest{
			OwnerID: "user-1", IdempotencyKey: "key-1", Ads: items,
		})

		require.NoError(t, err)
		assert.Equal(t, []domain.AdID{10, 11}, got)
	})

	t.Run("returns the first import without creating anything when the key was already used", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("Begin", ctx).Return(nil)
		m.On("ClaimImport", ctx, domain.UserID("user-1"), "key-1").Return(false, []domain.AdID{10, 11}, nil)
		m.On("Rollback").Return(nil) // no CreateAd expectation: creating an ad would fail the test

		got, err := usecase.NewImportAdsFunc(m)(ctx, usecase.ImportAdsRequest{
			OwnerID: "user-1", IdempotencyKey: "key-1", Ads: items,
		})

		require.NoError(t, err)
		assert.Equal(t, []domain.AdID{10, 11}, got)
	})

	t.Run("returns error when the transaction cannot start", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("db down")
		m.On("Begin", ctx).Return(dbErr)

		_, err := usecase.NewImportAdsFunc(m)(ctx, usecase.ImportAdsRequest{OwnerID: "user-1", Ads: items})

		assert.ErrorIs(t, err, dbErr)
	})
}
