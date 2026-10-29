package usecase

import (
	"context"
	"fmt"
	"log/slog"

	adsdao "github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/dao"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/domain"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/model"
	usersclient "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
)

type (
	// GetAdByIDFunc returns a single ad. It fails with ErrorCodeNotFound when no ad matches the identifier.
	GetAdByIDFunc func(ctx context.Context, id domain.AdID) (AdDetails, error)

	AdDetails struct {
		domain.Ad
		// Username is the nickname of the ad owner. It is empty when the user service cannot provide it: the ad is still returned.
		Username string
	}

	getAdByID struct {
		adDAO      adsdao.DAO
		userClient usersclient.UserClient
	}
)

// NewGetAdByIDFunc returns a GetAdByIDFunc backed by the given DAO.
func NewGetAdByIDFunc(adDAO adsdao.DAO, userClient usersclient.UserClient) GetAdByIDFunc {
	return getAdByID{adDAO: adDAO, userClient: userClient}.getAdByID
}

func (uc getAdByID) getAdByID(ctx context.Context, id domain.AdID) (AdDetails, error) {
	found, err := uc.adDAO.GetAdByID(ctx, id)
	if err != nil {
		return AdDetails{}, fmt.Errorf("unable to get ad %s: %w", id, err)
	}

	return AdDetails{Ad: found, Username: uc.username(ctx, found.UserID)}, nil
}

// username returns the nickname of the given user, or an empty string when the user service fails. The ad is the core of the response and the username a display enrichment, so a failing user service must not make the ad unavailable.
func (uc getAdByID) username(ctx context.Context, userID domain.UserID) string {
	user, err := uc.userClient.GetUserByID(ctx, model.UserID(userID))
	if err != nil {
		slog.WarnContext(ctx, "unable to get the ad owner username", "user_id", userID, "error", err)

		return ""
	}

	return user.Nickname
}
