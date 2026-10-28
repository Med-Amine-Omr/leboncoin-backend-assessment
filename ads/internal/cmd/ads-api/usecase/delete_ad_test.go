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

func TestDeleteAd(t *testing.T) {
	ctx := context.Background()
	req := usecase.DeleteAdRequest{ID: 42, OwnerID: "user-1"}

	t.Run("nominal", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("DeleteAdByID", ctx, domain.AdID(42), domain.UserID("user-1")).Return(nil)

		err := usecase.NewDeleteAdFunc(m)(ctx, req)

		require.NoError(t, err)
	})

	t.Run("error ad not found or not owned", func(t *testing.T) {
		m := mockdao.New(t)
		m.On("DeleteAdByID", ctx, domain.AdID(42), domain.UserID("user-1")).Return(adsdao.ErrNotFound)

		err := usecase.NewDeleteAdFunc(m)(ctx, req)

		require.Error(t, err)
		assert.ErrorIs(t, err, adsdao.ErrNotFound)
	})

	t.Run("error from dao", func(t *testing.T) {
		m := mockdao.New(t)
		dbErr := errors.New("connection reset")
		m.On("DeleteAdByID", ctx, domain.AdID(42), domain.UserID("user-1")).Return(dbErr)

		err := usecase.NewDeleteAdFunc(m)(ctx, req)

		require.Error(t, err)
		assert.ErrorIs(t, err, dbErr)
	})
}
