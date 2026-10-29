-- Logs every ad published through POST /ads, for the rolling publication quota.
-- Deleting an ad keeps its publication, so the quota cannot be bypassed by deleting and re-creating ads.
-- Bulk imports are not logged: the quota does not apply to them.
CREATE TABLE ad_publications (
    id           BIGSERIAL   PRIMARY KEY,
    owner_id     UUID        NOT NULL,
    ad_id        BIGINT      NOT NULL,
    published_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ad_publications_owner_published_at_idx ON ad_publications (owner_id, published_at);
