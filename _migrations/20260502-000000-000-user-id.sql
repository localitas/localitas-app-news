ALTER TABLE collections ADD COLUMN user_id TEXT NOT NULL DEFAULT '';
UPDATE collections SET user_id = '2b9af8b9-856a-4710-9ab6-58fe4eccdf24' WHERE user_id = '';
CREATE INDEX IF NOT EXISTS idx_collections_user ON collections(user_id);
