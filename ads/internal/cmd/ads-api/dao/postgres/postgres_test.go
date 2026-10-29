package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

var adCols = []string{"id", "title", "price_cents", "photo_url", "user_id", "created_at"}

func newMockDAO(t *testing.T) (*DAO, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return &DAO{db: db}, mock
}

func TestSearchAds(t *testing.T) {
	ctx := context.Background()
	createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("returns matching ads", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		rows := sqlmock.NewRows(adCols).
			AddRow(int64(1), "bike", int64(5000), "https://example.com/bike.jpg", "user-1", createdAt).
			AddRow(int64(2), "scooter", int64(12000), "https://example.com/scooter.jpg", "user-2", createdAt)
		mock.ExpectQuery("SELECT").
			WithArgs("", int64(0), int64(0), "").
			WillReturnRows(rows)

		got, err := dao.SearchAds(ctx, adsdao.SearchAdsRequest{})

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, domain.AdID(1), got[0].ID)
		assert.Equal(t, "bike", got[0].Title)
		assert.Equal(t, int64(5000), got[0].Price)
		assert.Equal(t, domain.AdID(2), got[1].ID)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns nil when no ads match", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectQuery("SELECT").
			WithArgs("", int64(0), int64(0), "bike").
			WillReturnRows(sqlmock.NewRows(adCols))

		got, err := dao.SearchAds(ctx, adsdao.SearchAdsRequest{Title: "bike"})

		require.NoError(t, err)
		assert.Nil(t, got)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when query fails", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("db down")
		mock.ExpectQuery("SELECT").
			WithArgs("", int64(0), int64(0), "").
			WillReturnError(boom)

		_, err := dao.SearchAds(ctx, adsdao.SearchAdsRequest{})

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestGetAdByID(t *testing.T) {
	ctx := context.Background()
	createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("returns ad on success", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		rows := sqlmock.NewRows(adCols).
			AddRow(int64(42), "mountain bike", int64(15000), "https://example.com/bike.jpg", "user-1", createdAt)
		mock.ExpectQuery("SELECT").
			WithArgs(int64(42)).
			WillReturnRows(rows)

		got, err := dao.GetAdByID(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(42), got.ID)
		assert.Equal(t, "mountain bike", got.Title)
		assert.Equal(t, int64(15000), got.Price)
		assert.Equal(t, domain.UserID("user-1"), got.UserID)
		assert.Equal(t, createdAt, got.CreatedAt)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrNotFound when no row", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectQuery("SELECT").
			WithArgs(int64(99)).
			WillReturnRows(sqlmock.NewRows(adCols))

		_, err := dao.GetAdByID(ctx, 99)

		require.Error(t, err)
		assert.ErrorIs(t, err, adsdao.ErrNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error on query failure", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("connection reset")
		mock.ExpectQuery("SELECT").
			WithArgs(int64(1)).
			WillReturnError(boom)

		_, err := dao.GetAdByID(ctx, 1)

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestCreateAd(t *testing.T) {
	ctx := context.Background()
	req := adsdao.CreateAdRequest{
		Title:      "mountain bike",
		PriceCents: 15000,
		PhotoURL:   "https://example.com/bike.jpg",
		OwnerID:    "user-1",
	}

	t.Run("returns generated ID on success", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))

		got, err := dao.CreateAd(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, domain.AdID(7), got)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrUnknownOwner on FK violation", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		pgErr := &pgconn.PgError{Code: "23503"}
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnError(pgErr)

		_, err := dao.CreateAd(ctx, req)

		require.Error(t, err)
		assert.ErrorIs(t, err, adsdao.ErrUnknownOwner)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error on other failures", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("db down")
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnError(boom)

		_, err := dao.CreateAd(ctx, req)

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		assert.NotErrorIs(t, err, adsdao.ErrUnknownOwner)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestDeleteAdByID(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes the ad owned by the user", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectExec("DELETE").
			WithArgs(int64(42), "user-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := dao.DeleteAdByID(ctx, 42, "user-1")

		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns ErrNotFound when no row is deleted", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectExec("DELETE").
			WithArgs(int64(42), "user-2").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := dao.DeleteAdByID(ctx, 42, "user-2")

		require.Error(t, err)
		assert.ErrorIs(t, err, adsdao.ErrNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error on query failure", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("connection reset")
		mock.ExpectExec("DELETE").
			WithArgs(int64(1), "user-1").
			WillReturnError(boom)

		err := dao.DeleteAdByID(ctx, 1, "user-1")

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		assert.NotErrorIs(t, err, adsdao.ErrNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTransaction(t *testing.T) {
	ctx := context.Background()
	req := adsdao.CreateAdRequest{
		Title:      "mountain bike",
		PriceCents: 15000,
		PhotoURL:   "https://example.com/bike.jpg",
		OwnerID:    "user-1",
	}

	t.Run("commits every call made on the transaction", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectBegin()
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(2)))
		mock.ExpectCommit()

		tx, err := dao.Begin(ctx)
		require.NoError(t, err)

		first, err := tx.CreateAd(ctx, req)
		require.NoError(t, err)
		second, err := tx.CreateAd(ctx, req)
		require.NoError(t, err)

		require.NoError(t, tx.Commit())
		assert.Equal(t, domain.AdID(1), first)
		assert.Equal(t, domain.AdID(2), second)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rolls back without committing", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectBegin()
		mock.ExpectQuery("INSERT").
			WithArgs(req.Title, req.PriceCents, req.PhotoURL, string(req.OwnerID)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
		mock.ExpectRollback()

		tx, err := dao.Begin(ctx)
		require.NoError(t, err)

		_, err = tx.CreateAd(ctx, req)
		require.NoError(t, err)

		require.NoError(t, tx.Rollback())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when the transaction cannot be opened", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("db down")
		mock.ExpectBegin().WillReturnError(boom)

		tx, err := dao.Begin(ctx)

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		assert.Nil(t, tx)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rollback after commit does nothing", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectBegin()
		mock.ExpectCommit()

		tx, err := dao.Begin(ctx)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())

		require.NoError(t, tx.Rollback())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rejects a nested transaction", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectBegin()
		mock.ExpectRollback()

		tx, err := dao.Begin(ctx)
		require.NoError(t, err)

		_, nestedErr := tx.(*DAO).Begin(ctx)

		require.Error(t, nestedErr)
		require.NoError(t, tx.Rollback())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("refuses commit and rollback outside a transaction", func(t *testing.T) {
		dao, _ := newMockDAO(t)

		require.Error(t, dao.Commit())
		require.Error(t, dao.Rollback())
	})
}

func TestClaimImport(t *testing.T) {
	ctx := context.Background()

	t.Run("claims a new key", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectExec("INSERT INTO ad_imports").
			WithArgs("user-1", "key-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		claimed, adIDs, err := dao.ClaimImport(ctx, "user-1", "key-1")

		require.NoError(t, err)
		assert.True(t, claimed)
		assert.Nil(t, adIDs)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns the ads of the import that already used the key", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectExec("INSERT INTO ad_imports").
			WithArgs("user-1", "key-1").
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery("SELECT ad_ids").
			WithArgs("user-1", "key-1").
			WillReturnRows(sqlmock.NewRows([]string{"ad_ids"}).AddRow("[10,11]"))

		claimed, adIDs, err := dao.ClaimImport(ctx, "user-1", "key-1")

		require.NoError(t, err)
		assert.False(t, claimed)
		assert.Equal(t, []domain.AdID{10, 11}, adIDs)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when the claim fails", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("db down")
		mock.ExpectExec("INSERT INTO ad_imports").
			WithArgs("user-1", "key-1").
			WillReturnError(boom)

		_, _, err := dao.ClaimImport(ctx, "user-1", "key-1")

		assert.ErrorIs(t, err, boom)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestCompleteImport(t *testing.T) {
	ctx := context.Background()

	dao, mock := newMockDAO(t)
	mock.ExpectExec("UPDATE ad_imports").
		WithArgs("user-1", "key-1", "[10,11]").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := dao.CompleteImport(ctx, "user-1", "key-1", []domain.AdID{10, 11})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLockOwnerPublications(t *testing.T) {
	ctx := context.Background()

	t.Run("takes the owner lock inside a transaction", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectBegin()
		mock.ExpectExec("pg_advisory_xact_lock").
			WithArgs("user-1").
			WillReturnResult(sqlmock.NewResult(0, 0))

		tx, err := dao.Begin(ctx)
		require.NoError(t, err)

		err = tx.LockOwnerPublications(ctx, "user-1")

		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("refuses to lock outside a transaction", func(t *testing.T) {
		dao, mock := newMockDAO(t)

		err := dao.LockOwnerPublications(ctx, "user-1")

		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestCountRecentPublications(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the number of publications in the window", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		mock.ExpectQuery("SELECT count").
			WithArgs("user-1", float64(86400)).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

		got, err := dao.CountRecentPublications(ctx, "user-1", 24*time.Hour)

		require.NoError(t, err)
		assert.Equal(t, 3, got)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when the query fails", func(t *testing.T) {
		dao, mock := newMockDAO(t)
		boom := errors.New("db down")
		mock.ExpectQuery("SELECT count").
			WithArgs("user-1", float64(86400)).
			WillReturnError(boom)

		_, err := dao.CountRecentPublications(ctx, "user-1", 24*time.Hour)

		assert.ErrorIs(t, err, boom)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestRecordPublication(t *testing.T) {
	ctx := context.Background()

	dao, mock := newMockDAO(t)
	mock.ExpectExec("INSERT INTO ad_publications").
		WithArgs("user-1", int64(42)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := dao.RecordPublication(ctx, "user-1", 42)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
