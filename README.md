# Way of working

This repository is read-only, click on "Use this template" => "Create a new repository" and work on your copy just as you would on a live one. \
Once you are satisfied with your work, please send us your repository url.

# Hiring instructions

Please refer to [these instructions](INSTRUCTIONS.md).

# Ads API

[![CI](https://github.com/Med-Amine-Omr/leboncoin-backend-assessment/actions/workflows/ci.yml/badge.svg)](https://github.com/Med-Amine-Omr/leboncoin-backend-assessment/actions/workflows/ci.yml)

A small Go HTTP API around a single domain: *ads* (classified ads).

The API needs an up-and-running PostgreSQL database and the identity (users) service, both launched by the provided Docker Compose file.

## Requirements

- Go 1.26 or later
- Docker with the Compose plugin

## Launching the stack

```bash
docker compose up -d --build
```

This starts PostgreSQL with the schema and seed data already applied, the `users` service and the `ads` API.

Migrations live in `ads/internal/cmd/ads-api/dao/postgres/migrations` and are only applied when the database volume is created. After adding a migration, recreate it:

```bash
docker compose down -v && docker compose up -d --build
```

### Configuration

| Variable           | Default                                                  | Description                                  |
|--------------------|----------------------------------------------------------|----------------------------------------------|
| `HTTP_ADDR`        | `:8080`                                                  | Address the API listens on                   |
| `DATABASE_URL`     | `postgres://ads:ads@localhost:5432/ads?sslmode=disable`  | PostgreSQL connection string                 |
| `IDENTITY_URL`     | `http://localhost:8081`                                  | Base URL of the identity (users) service     |
| `IDENTITY_TIMEOUT` | `2s` (`500ms` in Docker Compose)                         | Upper bound of every call to the identity service |

## Model

There are 2 entities in this sample:
- ad (title, price, etc...)
- user (id and nickname only), owned by the identity service

## API

The full contract is described in [`openapi.yaml`](ads/internal/cmd/ads-api/specs/openapi.yaml).

| Method   | Path          | Auth | Description                                                                                  |
|----------|---------------|------|----------------------------------------------------------------------------------------------|
| `POST`   | `/ads`        | yes  | Create an ad (at most 5 per user per rolling 24 hours)                                       |
| `GET`    | `/ads/{id}`   | no   | Get a single ad, with its owner's `username`                                                  |
| `GET`    | `/ads`        | no   | List ads (filterable by `title`, `user_id`, `min_price_cents`, `max_price_cents`)            |
| `DELETE` | `/ads/{id}`   | yes  | Delete one of your ads                                                                       |
| `POST`   | `/ads/import` | yes  | Bulk ad import, atomic, retryable with an `Idempotency-Key` header                           |

```bash
# Health check
curl -s -v http://localhost:8080/health

# List ads (with optional filters)
curl -s 'http://localhost:8080/ads?title=bike&min_price_cents=1000'

# Get a single ad
curl -s http://localhost:8080/ads/1

# Create an ad (requires Authorization header)
curl -s -X POST http://localhost:8080/ads \
  -H 'Authorization: 00000000-0000-0000-0000-000000000001' \
  -H 'Content-Type: application/json' \
  -d '{"title":"Mountain bike","price":120.00,"photo_url":"https://example.com/photos/bike.jpg"}'

# Delete one of your ads
curl -s -X DELETE http://localhost:8080/ads/3 \
  -H 'Authorization: 00000000-0000-0000-0000-000000000001'

# Bulk import, safe to retry with the same key
curl -s -X POST http://localhost:8080/ads/import \
  -H 'Authorization: 00000000-0000-0000-0000-000000000001' \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: 7f9c2ba4-e88f-4f1a-9a3c-3b1d2a6e5c10' \
  -d '{"ads":[{"title":"Bike","price":50,"photo_url":"https://example.com/bike.jpg"},{"title":"Car","price":2000,"photo_url":"https://example.com/car.jpg"}]}'
```

## Authentication

To pass authentication information to a call, simply add the `Authorization` header with the user ID in plain text.

It will be extracted by the service.

```
Authorization: 00000000-0000-0000-0000-000000000001
```

## Tests

Unit tests (no external dependency):

```shell
go test ./...
```

Integration tests run against a real PostgreSQL (the one of Docker Compose by default, or `DATABASE_URL`):

```shell
docker compose up -d ads-database
go test -tags=integration ./...
```

The [CI](.github/workflows/ci.yml) runs on every pull request and every push to `main`: `gofmt` check, `go vet`, unit tests with the race detector, and integration tests against a PostgreSQL service. `main` is protected: a pull request cannot be merged unless both jobs pass.

# Technical choices

Each change was delivered as its own pull request, with its tests and its OpenAPI update. The pull requests describe the reasoning in more detail.

## 1. `GetAdByID` bug

The use case queried the DAO with id `0` instead of the requested id. The existing test could not catch it because the mock accepted any id (`mock.Anything`); tests now assert the exact arguments passed to the DAO.

## 2. Delete an ad

- Ownership is enforced in a **single atomic query** (`DELETE ... WHERE id = $1 AND user_id = $2`): no read-then-delete, no race.
- An ad owned by another user answers **404, not 403**, so the API does not disclose which ids exist.
- The owner always comes from the authenticated context, never from the request.

## 3. Owner username

- The provided HTTP client (`users/pkg/api`) does not satisfy the `users/pkg.UserClient` interface. Since `/users` is out of scope, a small **adapter** (`ads-api/userclient`) bridges them; the use case only depends on the interface.
- **Graceful degradation**: if the identity service fails or is slow, the ad is still returned with `"username": null` and a warning is logged. The ad is the core of the response; the username is a display enrichment.
- `GET /ads/{id}` is the hottest endpoint, so the identity call is bounded by a short timeout.

## 4. `price_cents` renamed to `price` (float)

Renaming the field would break the existing web and mobile clients, and floats cannot represent money exactly. Instead:

- `price` (euros) is **added next to** `price_cents`, which is kept and marked deprecated (code and OpenAPI). Writes accept either field, not both.
- Storage and domain **stay in integer cents**; the conversion only happens at the HTTP boundary. A price with more than 2 decimals is rejected rather than silently rounded.
- Migration plan: announce the deprecation, let clients move to `price`, monitor `price_cents` usage, remove it in the next major version. A `/v2` or a string price were considered and discarded.

## 5. Bulk import

Two root causes: each ad was inserted in its own transaction (a failure mid-batch left a partial import, duplicated on retry), and a lost response made even a successful import duplicate on retry.

- The whole batch now runs in **one transaction**: either every ad is created, or none is.
- An optional **`Idempotency-Key`** header (industry standard) makes retries safe: the key is reserved with `INSERT ... ON CONFLICT DO NOTHING` on `(owner_id, idempotency_key)`, and a replay returns the ids of the first import without creating anything, even under concurrent retries.
- The import has its own use case, so the publication quota does not apply to it.

## 6. Publication quota

- A **publication log** (`ad_publications`), written only by `POST /ads`: imports are neither limited nor counted, and deleting an ad does not free the quota (otherwise delete/re-create would bypass it).
- **Concurrency**: a naive count-then-insert lets concurrent requests exceed the quota. The check and the insert run in one transaction, serialized per user with `pg_advisory_xact_lock`. It does not block other users and works across several API instances.
- The window is measured with the **database clock**, so API instances with drifting clocks agree.
- An integration test fires 20 concurrent creations for one user and checks that exactly 5 succeed. It caught a connection-pool deadlock that unit tests with mocks could not see.

## Other fixes

- `TestMock_SearchAds` was failing since the initial commit: the test required an error on a success case.
- `POST /ads` read the owner with the deprecated `UserIDFromHeader`; it now uses the authenticated context, like the other routes.
- The OpenAPI spec was synchronized with the implemented endpoints.

## Possible improvements

- Use a migration tool (e.g. golang-migrate) instead of the Docker init scripts.
- Cache usernames with a short TTL, and add a circuit breaker on the identity service.
- Reject an `Idempotency-Key` replayed with a different body (422), and purge old import keys and publications.
- Add a `Retry-After` header on 429, and make the quota configurable.
- Add `min_price` / `max_price` filters next to the cents ones.
- Return error bodies (e.g. RFC 9457 problem details) instead of bare status codes.
