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
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/model"
	usersclient "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
	usersmock "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/mock"
)

func TestGetAdByID(t *testing.T) {
	ctx := context.Background()
	ad := domain.Ad{ID: 42, Title: "bike", Price: 5000, UserID: "user-1"}

	t.Run("nominal", func(t *testing.T) {
		m := mockdao.New(t)
		users := usersmock.New(t)
		m.On("GetAdByID", ctx, domain.AdID(42)).Return(ad, nil)
		users.On("GetUserByID", ctx, model.UserID("user-1")).
			Return(usersclient.User{UserID: "user-1", Nickname: "alice"}, nil)

		got, err := usecase.NewGetAdByIDFunc(m, users)(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, ad, got.Ad)
		assert.Equal(t, "alice", got.Username)
	})

	t.Run("returns the ad without username when the user service fails", func(t *testing.T) {
		m := mockdao.New(t)
		users := usersmock.New(t)
		m.On("GetAdByID", ctx, domain.AdID(42)).Return(ad, nil)
		users.On("GetUserByID", ctx, model.UserID("user-1")).
			Return(usersclient.User{}, errors.New("timeout"))

		got, err := usecase.NewGetAdByIDFunc(m, users)(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, ad, got.Ad)
		assert.Empty(t, got.Username)
	})

	t.Run("error no ad found", func(t *testing.T) {
		m := mockdao.New(t)
		users := usersmock.New(t) // no expectation: the user service must not be called
		m.On("GetAdByID", ctx, domain.AdID(99)).Return(domain.Ad{}, adsdao.ErrNotFound)

		_, err := usecase.NewGetAdByIDFunc(m, users)(ctx, 99)

		require.Error(t, err)
		assert.ErrorIs(t, err, adsdao.ErrNotFound)
	})

	t.Run("error from dao", func(t *testing.T) {
		m := mockdao.New(t)
		users := usersmock.New(t) // no expectation: the user service must not be called
		dbErr := errors.New("connection reset")
		m.On("GetAdByID", ctx, domain.AdID(1)).Return(domain.Ad{}, dbErr)

		_, err := usecase.NewGetAdByIDFunc(m, users)(ctx, 1)

		require.Error(t, err)
		assert.ErrorIs(t, err, dbErr)
	})
}
