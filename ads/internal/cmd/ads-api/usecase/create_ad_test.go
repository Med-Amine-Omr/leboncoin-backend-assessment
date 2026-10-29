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

func TestCreateAd(t *testing.T) {
	ctx := context.Background()
	owner := domain.UserID("user-1")
	req := usecase.CreateAdRequest{Title: "bike", PriceCents: 5000, PhotoURL: "bike.jpg", OwnerID: owner}
	daoReq := adsdao.CreateAdRequest{Title: "bike", PriceCents: 5000, PhotoURL: "bike.jpg", OwnerID: owner}

	// expectQuotaCheck sets the expectations shared by every case that reaches the quota check.
	expectQuotaCheck := func(m *mockdao.Mock, published int) {
		m.On("Begin", ctx).Return(nil)
		m.On("LockOwnerPublications", ctx, owner).Return(nil)
		m.On("CountRecentPublications", ctx, owner, usecase.PublicationWindow).Return(published, nil)
		m.On("Rollback").Return(nil)
	}

	t.Run("creates the ad and records the publication below the quota", func(t *testing.T) {
		m := mockdao.New(t)
		expectQuotaCheck(m, usecase.MaxPublicationsPerWindow-1)
		m.On("CreateAd", ctx, daoReq).Return(domain.AdID(42), nil)
		m.On("RecordPublication", ctx, owner, domain.AdID(42)).Return(nil)
		m.On("Commit").Return(nil)

		got, err := usecase.NewCreateAdFunc(m)(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(42), got)
	})

	t.Run("rejects the ad when the quota is reached", func(t *testing.T) {
		m := mockdao.New(t)
		expectQuotaCheck(m, usecase.MaxPublicationsPerWindow) // no CreateAd expectation: creating would fail the test

		_, err := usecase.NewCreateAdFunc(m)(ctx, req)

		require.Error(t, err)
		assert.ErrorIs(t, err, usecase.ErrQuotaExceeded)
	})

	t.Run("does not commit when the creation fails", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("db down")
		expectQuotaCheck(m, 0)
		m.On("CreateAd", ctx, daoReq).Return(domain.AdID(0), dbErr)

		_, err := usecase.NewCreateAdFunc(m)(ctx, req)

		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("does not commit when the publication cannot be recorded", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("db down")
		expectQuotaCheck(m, 0)
		m.On("CreateAd", ctx, daoReq).Return(domain.AdID(42), nil)
		m.On("RecordPublication", ctx, owner, domain.AdID(42)).Return(dbErr)

		_, err := usecase.NewCreateAdFunc(m)(ctx, req)

		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("returns error when the owner lock fails", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("lock timeout")
		m.On("Begin", ctx).Return(nil)
		m.On("LockOwnerPublications", ctx, owner).Return(dbErr)
		m.On("Rollback").Return(nil)

		_, err := usecase.NewCreateAdFunc(m)(ctx, req)

		assert.ErrorIs(t, err, dbErr)
	})
}
