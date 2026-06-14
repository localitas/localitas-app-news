package news

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
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
		SELECT c.id, c.name, c.description, c.icon, c.created_at, c.updated_at, COUNT(f.id)
		FROM collections c LEFT JOIN feeds f ON f.collection_id = c.id
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

func (s *Store) DeleteCollection(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM collections WHERE id = ?", id)
	return err
}

// Feeds

func (s *Store) AddFeed(ctx context.Context, url, title, collectionID string) (*Feed, error) {
	id := newID()
	now := time.Now().UTC().Unix()
	_, err := s.db.ExecContext(ctx, "INSERT INTO feeds (id, collection_id, url, title, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)", id, collectionID, url, title, now, now)
	if err != nil {
		return nil, err
	}
	return &Feed{ID: id, CollectionID: collectionID, URL: url, Title: title, IsActive: true, CreatedAt: time.Unix(now, 0), UpdatedAt: time.Unix(now, 0)}, nil
}

func (s *Store) ListFeeds(ctx context.Context) ([]*Feed, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, COALESCE(collection_id,''), url, title, description, site_url, image_url, is_active, COALESCE(last_fetched_at,0), COALESCE(fetch_error,''), created_at, updated_at FROM feeds ORDER BY title")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFeeds(rows)
}

func (s *Store) DeleteFeed(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM feeds WHERE id = ?", id)
	return err
}

func (s *Store) ListActiveFeeds(ctx context.Context) ([]*Feed, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, COALESCE(collection_id,''), url, title, description, site_url, image_url, is_active, COALESCE(last_fetched_at,0), COALESCE(fetch_error,''), created_at, updated_at FROM feeds WHERE is_active = 1")
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

// Articles

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

func (s *Store) ListArticles(ctx context.Context, feedID string, limit, offset int) ([]*Article, error) {
	q := "SELECT a.id, a.feed_id, COALESCE(f.title,''), a.guid, a.title, a.description, a.content, a.link, a.author, a.image_url, a.published_at, a.fetched_at, a.is_read, a.is_starred FROM articles a LEFT JOIN feeds f ON f.id = a.feed_id"
	args := []interface{}{}
	if feedID != "" {
		q += " WHERE a.feed_id = ?"
		args = append(args, feedID)
	}
	q += " ORDER BY a.published_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	articles, err := scanArticles(rows)
	if err != nil {
		return nil, err
	}
	s.attachTags(ctx, articles)
	return articles, nil
}

func (s *Store) MarkArticleRead(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE articles SET is_read = 1 WHERE id = ?", id)
	return err
}

func (s *Store) ToggleArticleStar(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE articles SET is_starred = CASE WHEN is_starred = 1 THEN 0 ELSE 1 END WHERE id = ?", id)
	return err
}

func (s *Store) SearchArticles(ctx context.Context, query string, limit int) ([]*Article, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.feed_id, COALESCE(f.title,''), a.guid, a.title, a.description, a.content, a.link, a.author, a.image_url, a.published_at, a.fetched_at, a.is_read, a.is_starred
		FROM articles a
		LEFT JOIN feeds f ON f.id = a.feed_id
		JOIN articles_fts ON a.rowid = articles_fts.rowid
		WHERE articles_fts MATCH ?
		ORDER BY rank LIMIT ?`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	articles, err := scanArticles(rows)
	if err != nil {
		return nil, err
	}
	s.attachTags(ctx, articles)
	return articles, nil
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
	return out, nil
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
	return out, nil
}

func (s *Store) attachTags(ctx context.Context, articles []*Article) {
	for _, a := range articles {
		t, err := s.GetArticleTags(ctx, a.ID)
		if err == nil {
			a.Tags = t
		}
	}
}

// Tags

func (s *Store) CreateTag(ctx context.Context, name, color string) (*Tag, error) {
	id := newID()
	now := time.Now().UTC().Unix()
	if color == "" {
		color = "#7d6b96"
	}
	_, err := s.db.ExecContext(ctx, "INSERT INTO tags (id, name, color, created_at) VALUES (?, ?, ?, ?)", id, name, color, now)
	if err != nil {
		return nil, err
	}
	return &Tag{ID: id, Name: name, Color: color, CreatedAt: time.Unix(now, 0)}, nil
}

func (s *Store) ListTags(ctx context.Context) ([]*Tag, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, color, created_at FROM tags ORDER BY name")
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
	return out, nil
}

func (s *Store) DeleteTag(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM tags WHERE id = ?", id)
	return err
}

func (s *Store) TagArticle(ctx context.Context, articleID, tagID string) error {
	now := time.Now().UTC().Unix()
	_, err := s.db.ExecContext(ctx, "INSERT OR IGNORE INTO article_tags (article_id, tag_id, created_at) VALUES (?, ?, ?)", articleID, tagID, now)
	return err
}

func (s *Store) UntagArticle(ctx context.Context, articleID, tagID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM article_tags WHERE article_id = ? AND tag_id = ?", articleID, tagID)
	return err
}

func (s *Store) GetArticleTags(ctx context.Context, articleID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT t.name FROM tags t JOIN article_tags at ON t.id = at.tag_id WHERE at.article_id = ? ORDER BY t.name", articleID)
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
	return out, nil
}

func (s *Store) ListArticlesByTag(ctx context.Context, tagID string, limit, offset int) ([]*Article, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT a.id, a.feed_id, COALESCE(f.title,''), a.guid, a.title, a.description, a.content, a.link, a.author, a.image_url, a.published_at, a.fetched_at, a.is_read, a.is_starred
		FROM articles a
		LEFT JOIN feeds f ON f.id = a.feed_id
		JOIN article_tags at ON a.id = at.article_id
		WHERE at.tag_id = ?
		ORDER BY a.published_at DESC LIMIT ? OFFSET ?`, tagID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	articles, err := scanArticles(rows)
	if err != nil {
		return nil, err
	}
	s.attachTags(ctx, articles)
	return articles, nil
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
