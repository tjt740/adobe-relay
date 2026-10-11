-- Separate from account.extra: cookie pushes/admin edits cannot erase the cap.
CREATE TABLE IF NOT EXISTS adobe_cookie_recovery_attempts (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    attempt_id TEXT NOT NULL,
    attempts INTEGER NOT NULL CHECK (attempts BETWEEN 1 AND 2),
    last_attempt_at TIMESTAMPTZ NOT NULL,
    next_attempt_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ
);
