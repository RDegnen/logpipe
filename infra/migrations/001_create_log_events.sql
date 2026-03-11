CREATE TABLE IF NOT EXISTS log_events (
    event_id    TEXT        NOT NULL PRIMARY KEY,
    tenant_id   TEXT        NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    level       TEXT        NOT NULL,
    body        JSONB       NOT NULL,
    inserted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_log_events_tenant_observed
    ON log_events (tenant_id, observed_at DESC);
