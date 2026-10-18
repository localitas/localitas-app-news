package news

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	_ "github.com/localitas/localitas-go"
)

const DatabaseName = "news"

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func OpenStore(coreURL, dbID, token string) (*Store, error) {
	dsn := fmt.Sprintf("%s?database_id=%s&token=%s", coreURL, dbID, token)
	db, err := sql.Open("localitas", dsn)
	if err != nil {
		return nil, fmt.Errorf("open localitas db: %w", err)
	}
	return NewStore(db), nil
}

func (s *Store) Close() error { return s.db.Close() }

// Collections

func (s *Store) CreateCollection(ctx context.Context, userID, name, description, icon string) (*Collection, error) {
	id := newID()
	now := time.Now().UTC().Unix()
	if icon == "" {
		icon = "rss"
	}
	_, err := s.db.ExecContext(ctx, "INSERT INTO collections (id, user_id, name, description, icon, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)", id, userID, name, description, icon, now, now)
	if err != nil {
		return nil, err
	}
	return &Collection{ID: id, Name: name, Description: description, Icon: icon, CreatedAt: time.Unix(now, 0), UpdatedAt: time.Unix(now, 0)}, nil
}

func (s *Store) ListCollections(ctx context.Context, userID string) ([]*Collection, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.description, c.icon, c.created_at, c.updated_at, COUNT(s.feed_id)
		FROM collections c LEFT JOIN feed_subscriptions s ON s.collection_id = c.id AND s.user_id = c.user_id
		WHERE c.user_id = ?
		GROUP BY c.id ORDER BY c.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*Collection, 0)
	for rows.Next() {
		var c Collection
		var createdAt, updatedAt int64
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Icon, &createdAt, &updatedAt, &c.FeedCount); err != nil {
			return nil, err
		}
		c.CreatedAt = time.Unix(createdAt, 0)
		c.UpdatedAt = time.Unix(updatedAt, 0)
		out = append(out, &c)
	}
	return out, nil
}

// ErrNotFound is returned for a collection, subscription, article or tag
// that does not exist or is not the caller's: the two are indistinguishable
// to the caller.
var ErrNotFound = errors.New("not found")

// DeleteCollection removes userID's collection id.
func (s *Store) DeleteCollection(ctx context.Context, userID, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM collections WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	_, err = s.db.ExecContext(ctx, "UPDATE feed_subscriptions SET collection_id = NULL WHERE user_id = ? AND collection_id = ?", userID, id)
	return err
}

// OwnsCollection reports whether userID owns collection id.
func (s *Store) OwnsCollection(ctx context.Context, userID, id string) error {
	var found string
	err := s.db.QueryRowContext(ctx, "SELECT id FROM collections WHERE id = ? AND user_id = ?", id, userID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// Feeds are one shared fetch cache keyed by URL; a user follows a feed
// through their subscription to it.

const feedColumns = "f.id, COALESCE(s.collection_id,''), f.url, f.title, f.description, f.site_url, f.image_url, f.is_active, COALESCE(f.last_fetched_at,0), COALESCE(f.fetch_error,''), f.created_at, f.updated_at"

// AddFeed subscribes userID to the feed at url, recording the feed the
// first time anyone subscribes to it.
func (s *Store) AddFeed(ctx context.Context, userID, url, title, collectionID string) (*Feed, error) {
	now := time.Now().UTC().Unix()
	if _, err := s.db.ExecContext(ctx, "INSERT OR IGNORE INTO feeds (id, url, title, created_at, updated_at) VALUES (?, ?, ?, ?, ?)", newID(), url, title, now, now); err != nil {
		return nil, err
	}
	var feedID string
	if err := s.db.QueryRowContext(ctx, "SELECT id FROM feeds WHERE url = ?", url).Scan(&feedID); err != nil {
		return nil, err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO feed_subscriptions (user_id, feed_id, collection_id, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, feed_id) DO UPDATE SET collection_id = excluded.collection_id`, userID, feedID, nullable(collectionID), now)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+feedColumns+" FROM feed_subscriptions s JOIN feeds f ON f.id = s.feed_id WHERE s.user_id = ? AND s.feed_id = ?", userID, feedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	feeds, err := scanFeeds(rows)
	if err != nil {
		return nil, err
	}
	if len(feeds) == 0 {
		return nil, ErrNotFound
	}
	return feeds[0], nil
}

func (s *Store) ListFeeds(ctx context.Context, userID string) ([]*Feed, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+feedColumns+" FROM feed_subscriptions s JOIN feeds f ON f.id = s.feed_id WHERE s.user_id = ? ORDER BY f.title", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFeeds(rows)
}

// DeleteFeed ends userID's subscription to feed id; a feed nobody follows
// any more is removed with its articles.
func (s *Store) DeleteFeed(ctx context.Context, userID, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM feed_subscriptions WHERE user_id = ? AND feed_id = ?", userID, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	unfollowed := "NOT EXISTS (SELECT 1 FROM feed_subscriptions WHERE feed_id = ?)"
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		"DELETE FROM user_article_tags WHERE article_id IN (SELECT id FROM articles WHERE feed_id = ?) AND " + unfollowed,
		"DELETE FROM article_states WHERE article_id IN (SELECT id FROM articles WHERE feed_id = ?) AND " + unfollowed,
		"DELETE FROM articles WHERE feed_id = ? AND " + unfollowed,
		"DELETE FROM feeds WHERE id = ? AND " + unfollowed,
	} {
		if _, err := tx.ExecContext(ctx, stmt, id, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListActiveFeeds returns the active feeds someone follows: the ones a sync
// fetches. Legacy subscriptions not yet given to an admin do not count.
func (s *Store) ListActiveFeeds(ctx context.Context) ([]*Feed, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT f.id, '', f.url, f.title, f.description, f.site_url, f.image_url, f.is_active, COALESCE(f.last_fetched_at,0), COALESCE(f.fetch_error,''), f.created_at, f.updated_at
		FROM feeds f WHERE f.is_active = 1 AND EXISTS (SELECT 1 FROM feed_subscriptions s WHERE s.feed_id = f.id AND s.user_id != '')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFeeds(rows)
}

func (s *Store) UpdateFeedAfterSync(ctx context.Context, feedID, title, description, siteURL, imageURL string) {
	now := time.Now().UTC().Unix()
	s.db.ExecContext(ctx, "UPDATE feeds SET title=?, description=?, site_url=?, image_url=?, last_fetched_at=?, fetch_error='', updated_at=? WHERE id=?", title, description, siteURL, imageURL, now, now, feedID)
}

func (s *Store) UpdateFeedError(ctx context.Context, feedID, errMsg string) {
	now := time.Now().UTC().Unix()
	s.db.ExecContext(ctx, "UPDATE feeds SET fetch_error=?, updated_at=? WHERE id=?", errMsg, now, feedID)
}

// Articles are shared; whether a user has read or starred one is theirs.

func (s *Store) InsertArticleIfNew(ctx context.Context, feedID string, a *ParsedArticle) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM articles WHERE feed_id = ? AND guid = ?", feedID, a.GUID).Scan(&exists)
	if err == nil {
		return false, nil
	}
	id := newID()
	now := time.Now().UTC().Unix()
	_, err = s.db.ExecContext(ctx, `INSERT INTO articles (id, feed_id, guid, title, description, content, link, author, image_url, published_at, fetched_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, feedID, a.GUID, a.Title, a.Description, a.Content, a.Link, a.Author, a.ImageURL, a.PublishedAt.Unix(), now)
	return err == nil, err
}

// userArticles selects the articles of the feeds the first argument
// subscribes to, with that user's read and starred state.
const userArticles = `SELECT a.id, a.feed_id, f.title, a.guid, a.title, a.description, a.content, a.link, a.author, a.image_url, a.published_at, a.fetched_at, COALESCE(st.is_read,0), COALESCE(st.is_starred,0)
	FROM articles a
	JOIN feed_subscriptions s ON s.feed_id = a.feed_id AND s.user_id = ?
	JOIN feeds f ON f.id = a.feed_id
	LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = s.user_id`

func (s *Store) ListArticles(ctx context.Context, userID, feedID string, limit, offset int) ([]*Article, error) {
	q := userArticles
	args := []any{userID}
	if feedID != "" {
		q += " WHERE a.feed_id = ?"
		args = append(args, feedID)
	}
	q += " ORDER BY a.published_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	return s.queryArticles(ctx, userID, q, args...)
}

func (s *Store) SearchArticles(ctx context.Context, userID, query string, limit int) ([]*Article, error) {
	q := userArticles + `
	JOIN articles_fts ON a.rowid = articles_fts.rowid
	WHERE articles_fts MATCH ?
	ORDER BY rank LIMIT ?`
	return s.queryArticles(ctx, userID, q, userID, query, limit)
}

func (s *Store) queryArticles(ctx context.Context, userID, q string, args ...any) ([]*Article, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	articles, err := scanArticles(rows)
	if err != nil {
		return nil, err
	}
	if err := s.attachTags(ctx, userID, articles); err != nil {
		return nil, err
	}
	return articles, nil
}

// subscribedArticle matches article ? of a feed user ? subscribes to.
const subscribedArticle = "FROM articles a JOIN feed_subscriptions s ON s.feed_id = a.feed_id WHERE a.id = ? AND s.user_id = ?"

// MarkArticleRead records that userID read article id, which must be in a
// feed they subscribe to.
func (s *Store) MarkArticleRead(ctx context.Context, userID, id string) error {
	now := time.Now().UTC().Unix()
	return expectRow(s.db.ExecContext(ctx, `INSERT INTO article_states (user_id, article_id, is_read, is_starred, updated_at)
		SELECT s.user_id, a.id, 1, 0, ? `+subscribedArticle+`
		ON CONFLICT(user_id, article_id) DO UPDATE SET is_read = 1, updated_at = excluded.updated_at`, now, id, userID))
}

// ToggleArticleStar flips whether userID starred article id, which must be
// in a feed they subscribe to.
func (s *Store) ToggleArticleStar(ctx context.Context, userID, id string) error {
	now := time.Now().UTC().Unix()
	return expectRow(s.db.ExecContext(ctx, `INSERT INTO article_states (user_id, article_id, is_read, is_starred, updated_at)
		SELECT s.user_id, a.id, 0, 1, ? `+subscribedArticle+`
		ON CONFLICT(user_id, article_id) DO UPDATE SET is_starred = 1 - article_states.is_starred, updated_at = excluded.updated_at`, now, id, userID))
}

func scanFeeds(rows *sql.Rows) ([]*Feed, error) {
	out := make([]*Feed, 0)
	for rows.Next() {
		var f Feed
		var lastFetched, createdAt, updatedAt int64
		var isActive int
		if err := rows.Scan(&f.ID, &f.CollectionID, &f.URL, &f.Title, &f.Description, &f.SiteURL, &f.ImageURL, &isActive, &lastFetched, &f.FetchError, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		f.IsActive = isActive == 1
		f.LastFetchedAt = time.Unix(lastFetched, 0)
		f.CreatedAt = time.Unix(createdAt, 0)
		f.UpdatedAt = time.Unix(updatedAt, 0)
		out = append(out, &f)
	}
	return out, rows.Err()
}

func scanArticles(rows *sql.Rows) ([]*Article, error) {
	out := make([]*Article, 0)
	for rows.Next() {
		var a Article
		var publishedAt, fetchedAt int64
		var isRead, isStarred int
		if err := rows.Scan(&a.ID, &a.FeedID, &a.FeedTitle, &a.GUID, &a.Title, &a.Description, &a.Content, &a.Link, &a.Author, &a.ImageURL, &publishedAt, &fetchedAt, &isRead, &isStarred); err != nil {
			return nil, err
		}
		a.PublishedAt = time.Unix(publishedAt, 0)
		a.FetchedAt = time.Unix(fetchedAt, 0)
		a.IsRead = isRead == 1
		a.IsStarred = isStarred == 1
		out = append(out, &a)
	}
	return out, rows.Err()
}

func (s *Store) attachTags(ctx context.Context, userID string, articles []*Article) error {
	for _, a := range articles {
		t, err := s.GetArticleTags(ctx, userID, a.ID)
		if err != nil {
			return err
		}
		a.Tags = t
	}
	return nil
}

// Tags belong to the user who made them.

const favoritesTag = "Favorites"

func (s *Store) CreateTag(ctx context.Context, userID, name, color string) (*Tag, error) {
	id := newID()
	now := time.Now().UTC().Unix()
	if color == "" {
		color = "#7d6b96"
	}
	_, err := s.db.ExecContext(ctx, "INSERT INTO user_tags (id, user_id, name, color, created_at) VALUES (?, ?, ?, ?, ?)", id, userID, name, color, now)
	if err != nil {
		return nil, err
	}
	return &Tag{ID: id, Name: name, Color: color, CreatedAt: time.Unix(now, 0)}, nil
}

// ListTags returns userID's tags, giving them a Favorites tag when they
// have none.
func (s *Store) ListTags(ctx context.Context, userID string) ([]*Tag, error) {
	tags, err := s.listTags(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, t := range tags {
		if t.Name == favoritesTag {
			return tags, nil
		}
	}
	now := time.Now().UTC().Unix()
	if _, err := s.db.ExecContext(ctx, "INSERT OR IGNORE INTO user_tags (id, user_id, name, color, created_at) VALUES (?, ?, ?, '#f59e0b', ?)", newID(), userID, favoritesTag, now); err != nil {
		return nil, err
	}
	return s.listTags(ctx, userID)
}

func (s *Store) listTags(ctx context.Context, userID string) ([]*Tag, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, color, created_at FROM user_tags WHERE user_id = ? ORDER BY name", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*Tag, 0)
	for rows.Next() {
		var t Tag
		var createdAt int64
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &createdAt); err != nil {
			return nil, err
		}
		t.CreatedAt = time.Unix(createdAt, 0)
		out = append(out, &t)
	}
	return out, rows.Err()
}

func (s *Store) DeleteTag(ctx context.Context, userID, id string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM user_article_tags WHERE tag_id IN (SELECT id FROM user_tags WHERE id = ? AND user_id = ?)", id, userID); err != nil {
		return err
	}
	return expectRow(s.db.ExecContext(ctx, "DELETE FROM user_tags WHERE id = ? AND user_id = ?", id, userID))
}

// TagArticle applies userID's tag to an article of a feed they subscribe to.
func (s *Store) TagArticle(ctx context.Context, userID, articleID, tagID string) error {
	now := time.Now().UTC().Unix()
	return expectRow(s.db.ExecContext(ctx, `INSERT INTO user_article_tags (article_id, tag_id, created_at)
		SELECT a.id, t.id, ? FROM articles a
		JOIN feed_subscriptions s ON s.feed_id = a.feed_id AND s.user_id = ?
		JOIN user_tags t ON t.id = ? AND t.user_id = s.user_id
		WHERE a.id = ?
		ON CONFLICT(article_id, tag_id) DO UPDATE SET created_at = user_article_tags.created_at`, now, userID, tagID, articleID))
}

func (s *Store) UntagArticle(ctx context.Context, userID, articleID, tagID string) error {
	return expectRow(s.db.ExecContext(ctx, "DELETE FROM user_article_tags WHERE article_id = ? AND tag_id IN (SELECT id FROM user_tags WHERE id = ? AND user_id = ?)", articleID, tagID, userID))
}

func (s *Store) GetArticleTags(ctx context.Context, userID, articleID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT t.name FROM user_tags t JOIN user_article_tags at ON t.id = at.tag_id WHERE at.article_id = ? AND t.user_id = ? ORDER BY t.name", articleID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// ListArticlesByTag returns the articles userID tagged with their tag tagID.
func (s *Store) ListArticlesByTag(ctx context.Context, userID, tagID string, limit, offset int) ([]*Article, error) {
	var found string
	err := s.db.QueryRowContext(ctx, "SELECT id FROM user_tags WHERE id = ? AND user_id = ?", tagID, userID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	q := userArticles + `
	JOIN user_article_tags at ON at.article_id = a.id
	WHERE at.tag_id = ?
	ORDER BY a.published_at DESC LIMIT ? OFFSET ?`
	return s.queryArticles(ctx, userID, q, userID, tagID, limit, offset)
}

// ClaimLegacyData gives userID what the household had before per-user
// ownership: the subscriptions, read and starred state, and tags the
// ownership migration set aside under user ”. Where userID already has the
// same row, theirs is kept. Running it again finds nothing to claim.
func (s *Store) ClaimLegacyData(ctx context.Context, userID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, st := range []struct {
		sql  string
		args []any
	}{
		{"UPDATE OR IGNORE feed_subscriptions SET user_id = ? WHERE user_id = ''", []any{userID}},
		{"DELETE FROM feed_subscriptions WHERE user_id = ''", nil},
		{"UPDATE feed_subscriptions SET collection_id = NULL WHERE user_id = ? AND collection_id IS NOT NULL AND collection_id NOT IN (SELECT id FROM collections WHERE user_id = ?)", []any{userID, userID}},
		{"UPDATE OR IGNORE article_states SET user_id = ? WHERE user_id = ''", []any{userID}},
		{"DELETE FROM article_states WHERE user_id = ''", nil},
		{"UPDATE OR IGNORE user_tags SET user_id = ? WHERE user_id = ''", []any{userID}},
		{`UPDATE OR IGNORE user_article_tags SET tag_id = (
			SELECT mine.id FROM user_tags mine JOIN user_tags legacy ON legacy.name = mine.name
			WHERE legacy.id = user_article_tags.tag_id AND mine.user_id = ?)
		WHERE tag_id IN (SELECT id FROM user_tags WHERE user_id = '')`, []any{userID}},
		{"DELETE FROM user_article_tags WHERE tag_id IN (SELECT id FROM user_tags WHERE user_id = '')", nil},
		{"DELETE FROM user_tags WHERE user_id = ''", nil},
		{"UPDATE collections SET user_id = ? WHERE user_id = ''", []any{userID}},
	} {
		if _, err := tx.ExecContext(ctx, st.sql, st.args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// expectRow turns a write that changed no row into ErrNotFound.
func expectRow(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
