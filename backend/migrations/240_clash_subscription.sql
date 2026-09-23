-- The document contains subscription credentials; expose only redacted views in admin APIs.
-- A singleton supports replacing the subscription while keeping node names / proxy IDs stable.
CREATE TABLE IF NOT EXISTS clash_subscription (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    document JSONB NOT NULL DEFAULT '{"nodes":[]}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO clash_subscription (id) VALUES (1) ON CONFLICT DO NOTHING;
