package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

const (
	// MaxPublicationsPerWindow is the number of ads a user can publish through POST /ads within any rolling PublicationWindow.
	MaxPublicationsPerWindow = 5
	// PublicationWindow is the rolling window of the publication quota.
	PublicationWindow = 24 * time.Hour
)

// ErrQuotaExceeded is returned when the user already published MaxPublicationsPerWindow ads within the rolling PublicationWindow.
var ErrQuotaExceeded = errors.New("ad publication quota exceeded")

type (
	// CreateAdFunc creates an ad owned by the calling user.
	CreateAdFunc func(ctx context.Context, req CreateAdRequest) (domain.AdID, error)

	// CreateAdRequest describes the ad to create. OwnerID is the authenticated user: a caller can never create an ad on behalf of someone else.
	CreateAdRequest struct {
		Title      string
		PriceCents int64
		PhotoURL   string
		OwnerID    domain.UserID
	}

	createAd struct {
		adDAO adsdao.DAO
	}
)

// NewCreateAdFunc returns a CreateAdFunc backed by the given DAO.
func NewCreateAdFunc(adDAO adsdao.DAO) CreateAdFunc {
	return createAd{adDAO: adDAO}.createAd
}

func (uc createAd) createAd(ctx context.Context, req CreateAdRequest) (domain.AdID, error) {
	tx, err := uc.adDAO.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	// Discards every change unless Commit succeeded; a no-op after a successful Commit.
	defer func() { _ = tx.Rollback() }()

	// Held until the transaction ends: a concurrent request of the same owner waits here, so it counts this publication too.
	if err := tx.LockOwnerPublications(ctx, req.OwnerID); err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	published, err := tx.CountRecentPublications(ctx, req.OwnerID, PublicationWindow)
	if err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	if published >= MaxPublicationsPerWindow {
		return 0, fmt.Errorf("unable to create ad: %w", ErrQuotaExceeded)
	}

	adID, err := tx.CreateAd(ctx, adsdao.CreateAdRequest{
		Title:      req.Title,
		PriceCents: req.PriceCents,
		PhotoURL:   req.PhotoURL,
		OwnerID:    req.OwnerID,
	})
	if err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	if err := tx.RecordPublication(ctx, req.OwnerID, adID); err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("unable to create ad: %w", err)
	}

	return adID, nil
}
