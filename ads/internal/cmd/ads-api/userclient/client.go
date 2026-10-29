// Package userclient adapts the identity service HTTP client to the users/pkg.UserClient interface.
package userclient

import (
	"context"
	"fmt"

	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/common/model"
	usersclient "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
	usersapi "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/api"
)

// Client implements usersclient.UserClient on top of the identity service HTTP client.
type Client struct {
	api *usersapi.Client
}

// Compile-time check that Client satisfies the interface the use cases depend on.
var _ usersclient.UserClient = (*Client)(nil)

// New returns a Client backed by the given identity service HTTP client.
func New(api *usersapi.Client) *Client {
	return &Client{api: api}
}

// GetUserByID returns the user with the given ID. It fails with usersapi.ErrNotFound when the user does not exist.
func (c *Client) GetUserByID(ctx context.Context, userID model.UserID) (usersclient.User, error) {
	user, err := c.api.GetUserByID(ctx, string(userID))
	if err != nil {
		return usersclient.User{}, fmt.Errorf("unable to get user %s: %w", userID, err)
	}

	return toUser(user), nil
}

// ListUsers returns all users registered in the identity service.
func (c *Client) ListUsers(ctx context.Context) ([]usersclient.User, error) {
	users, err := c.api.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to list users: %w", err)
	}

	out := make([]usersclient.User, 0, len(users))
	for _, user := range users {
		out = append(out, toUser(user))
	}

	return out, nil
}

func toUser(u usersapi.User) usersclient.User {
	return usersclient.User{UserID: u.UserID, Nickname: u.Nickname}
}
