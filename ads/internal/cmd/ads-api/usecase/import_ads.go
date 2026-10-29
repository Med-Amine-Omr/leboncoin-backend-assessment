package usecase

import (
	"context"
	"fmt"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
)

type (
	// ImportAdsFunc creates a batch of ads atomically: either every ad is created, or none is. When the idempotency key was already used by the owner, nothing is created and the identifiers of the first import are returned, so a client can safely retry.
	ImportAdsFunc func(ctx context.Context, req ImportAdsRequest) ([]domain.AdID, error)

	// ImportAdsRequest describes a bulk import. OwnerID is the authenticated user.
	ImportAdsRequest struct {
		OwnerID domain.UserID
		// IdempotencyKey identifies the import for its owner. Empty means the import is not protected against retries.
		IdempotencyKey string
		Ads            []ImportAdItem
	}

	// ImportAdItem describes one ad of a bulk import.
	ImportAdItem struct {
		Title      string
		PriceCents int64
		PhotoURL   string
	}

	importAds struct {
		adDAO adsdao.DAO
	}
)

// NewImportAdsFunc returns an ImportAdsFunc backed by the given DAO.
func NewImportAdsFunc(adDAO adsdao.DAO) ImportAdsFunc {
	return importAds{adDAO: adDAO}.importAds
}

func (uc importAds) importAds(ctx context.Context, req ImportAdsRequest) ([]domain.AdID, error) {
	tx, err := uc.adDAO.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to import ads: %w", err)
	}
	// Discards every change unless Commit succeeded; a no-op after a successful Commit.
	defer func() { _ = tx.Rollback() }()

	if req.IdempotencyKey != "" {
		claimed, previous, err := tx.ClaimImport(ctx, req.OwnerID, req.IdempotencyKey)
		if err != nil {
			return nil, fmt.Errorf("unable to import ads: %w", err)
		}

		if !claimed {
			return previous, nil
		}
	}

	adIDs := make([]domain.AdID, 0, len(req.Ads))
	for _, ad := range req.Ads {
		adID, err := tx.CreateAd(ctx, adsdao.CreateAdRequest{
			Title:      ad.Title,
			PriceCents: ad.PriceCents,
			PhotoURL:   ad.PhotoURL,
			OwnerID:    req.OwnerID,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to import ads: %w", err)
		}

		adIDs = append(adIDs, adID)
	}

	if req.IdempotencyKey != "" {
		if err := tx.CompleteImport(ctx, req.OwnerID, req.IdempotencyKey, adIDs); err != nil {
			return nil, fmt.Errorf("unable to import ads: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("unable to import ads: %w", err)
	}

	return adIDs, nil
}
