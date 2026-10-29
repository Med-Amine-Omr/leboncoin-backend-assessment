//go:build integration

package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao/postgres"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/usecase"
)

// Requires the database of docker-compose: go test -tags=integration ./ads/internal/cmd/ads-api/usecase/...
func TestCreateAdQuotaUnderConcurrency(t *testing.T) {
	// Fails with a clear error instead of hanging forever if the database is unreachable or a lock is never released.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://ads:ads@localhost:5432/ads?sslmode=disable"
	}

	dao, err := postgres.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = dao.Close() })

	// A fresh owner per run, so publications of previous runs do not count.
	owner := domain.UserID(fmt.Sprintf("00000000-0000-0000-0000-%012d", time.Now().UnixNano()%1_000_000_000_000))
	createAd := usecase.NewCreateAdFunc(dao)

	const attempts = 20
	var created, rejected atomic.Int32
	var wg sync.WaitGroup

	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := createAd(ctx, usecase.CreateAdRequest{
				Title:      "quota test",
				PriceCents: 100,
				PhotoURL:   "https://example.com/quota.jpg",
				OwnerID:    owner,
			})

			switch {
			case err == nil:
				created.Add(1)
			case errors.Is(err, usecase.ErrQuotaExceeded):
				rejected.Add(1)
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int32(usecase.MaxPublicationsPerWindow), created.Load())
	assert.Equal(t, int32(attempts-usecase.MaxPublicationsPerWindow), rejected.Load())
}
