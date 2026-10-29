-- Records every bulk import made with an idempotency key, so a retried import returns the ads created the first time instead of creating them again.
CREATE TABLE ad_imports (
    owner_id        UUID        NOT NULL,
    idempotency_key TEXT        NOT NULL,
    ad_ids          JSONB       NOT NULL DEFAULT '[]',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (owner_id, idempotency_key)
);
