CREATE TABLE IF NOT EXISTS categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    parent_id UUID REFERENCES categories (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('expense', 'income')),
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Groups (parent_id IS NULL) unique per owner+kind, case-insensitive.
CREATE UNIQUE INDEX IF NOT EXISTS idx_categories_group_unique
    ON categories (user_id, kind, lower(name)) WHERE parent_id IS NULL;

-- Subcategories unique per parent, case-insensitive.
CREATE UNIQUE INDEX IF NOT EXISTS idx_categories_sub_unique
    ON categories (user_id, parent_id, lower(name)) WHERE parent_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_categories_user ON categories (user_id);
