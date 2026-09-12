-- Cover the kind-filtered list/summary queries (WHERE user_id + kind + occurred_on).
CREATE INDEX IF NOT EXISTS idx_transactions_user_kind_occurred
    ON transactions (user_id, kind, occurred_on DESC, created_at DESC);
