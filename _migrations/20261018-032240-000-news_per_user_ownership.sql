-- Migration: news_per_user_ownership
-- Created: 2026-10-18T03:22:40Z
--
-- Feeds and articles stay one shared fetch cache; what a user follows, has
-- read or starred, and how they tag, is theirs. Rows from before ownership
-- are copied under user '' and given to the first admin who signs in.
-- feeds.collection_id is no longer read (a subscription carries the
-- collection); it stays because SQLite cannot drop a foreign-key column.

CREATE TABLE IF NOT EXISTS feed_subscriptions (
    user_id TEXT NOT NULL,
    feed_id TEXT NOT NULL,
    collection_id TEXT,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (user_id, feed_id),
    FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE,
    FOREIGN KEY (collection_id) REFERENCES collections(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_feed_subscriptions_feed ON feed_subscriptions(feed_id);
CREATE INDEX IF NOT EXISTS idx_feed_subscriptions_collection ON feed_subscriptions(collection_id);

CREATE TABLE IF NOT EXISTS article_states (
    user_id TEXT NOT NULL,
    article_id TEXT NOT NULL,
    is_read INTEGER NOT NULL DEFAULT 0,
    is_starred INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (user_id, article_id),
    FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_tags (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    color TEXT NOT NULL DEFAULT '#7d6b96',
    created_at INTEGER NOT NULL,
    UNIQUE (user_id, name)
);

CREATE TABLE IF NOT EXISTS user_article_tags (
    article_id TEXT NOT NULL,
    tag_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (article_id, tag_id),
    FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES user_tags(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_user_article_tags_tag ON user_article_tags(tag_id);

INSERT OR IGNORE INTO feed_subscriptions (user_id, feed_id, collection_id, created_at)
    SELECT '', id, collection_id, created_at FROM feeds;

INSERT OR IGNORE INTO article_states (user_id, article_id, is_read, is_starred, updated_at)
    SELECT '', id, is_read, is_starred, fetched_at FROM articles WHERE is_read = 1 OR is_starred = 1;

INSERT OR IGNORE INTO user_tags (id, user_id, name, color, created_at)
    SELECT id, '', name, color, created_at FROM tags;

INSERT OR IGNORE INTO user_article_tags (article_id, tag_id, created_at)
    SELECT article_id, tag_id, created_at FROM article_tags;

DROP TABLE IF EXISTS article_tags;
DROP TABLE IF EXISTS tags;
DROP INDEX IF EXISTS idx_articles_is_read;
ALTER TABLE articles DROP COLUMN is_read;
ALTER TABLE articles DROP COLUMN is_starred;
