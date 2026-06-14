INSERT OR IGNORE INTO tags (id, name, color, created_at)
VALUES (hex(randomblob(16)), 'Favorites', '#f59e0b', strftime('%s','now'));
