package userclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.mpi-internal.com/leboncoin/backend-hiring-assessment/ads/internal/cmd/ads-api/userclient"
	usersclient "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg"
	usersapi "github.mpi-internal.com/leboncoin/backend-hiring-assessment/users/pkg/api"
)

func newClient(t *testing.T, mux *http.ServeMux) *userclient.Client {
	t.Helper()

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return userclient.New(usersapi.NewClient(srv.URL, 0))
}

func TestClientGetUserByID(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the user mapped to the interface type", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v1/users/{userID}", func(rw http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "user-1", r.PathValue("userID"))
			_ = json.NewEncoder(rw).Encode(usersapi.User{UserID: "user-1", Nickname: "alice"})
		})

		got, err := newClient(t, mux).GetUserByID(ctx, "user-1")

		require.NoError(t, err)
		assert.Equal(t, usersclient.User{UserID: "user-1", Nickname: "alice"}, got)
	})

	t.Run("keeps ErrNotFound reachable", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /v1/users/{userID}", func(rw http.ResponseWriter, _ *http.Request) {
			rw.WriteHeader(http.StatusNotFound)
		})

		_, err := newClient(t, mux).GetUserByID(ctx, "unknown")

		require.Error(t, err)
		assert.ErrorIs(t, err, usersapi.ErrNotFound)
	})
}

func TestClientListUsers(t *testing.T) {
	ctx := context.Background()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/users", func(rw http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(rw).Encode([]usersapi.User{
			{UserID: "user-1", Nickname: "alice"},
			{UserID: "user-2", Nickname: "bob"},
		})
	})

	got, err := newClient(t, mux).ListUsers(ctx)

	require.NoError(t, err)
	assert.Equal(t, []usersclient.User{
		{UserID: "user-1", Nickname: "alice"},
		{UserID: "user-2", Nickname: "bob"},
	}, got)
}
