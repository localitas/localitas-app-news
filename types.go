package news

import "time"

type Collection struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	FeedCount   int       `json:"feed_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Feed struct {
	ID            string    `json:"id"`
	CollectionID  string    `json:"collection_id,omitempty"`
	URL           string    `json:"url"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	SiteURL       string    `json:"site_url"`
	ImageURL      string    `json:"image_url"`
	IsActive      bool      `json:"is_active"`
	LastFetchedAt time.Time `json:"last_fetched_at,omitempty"`
	FetchError    string    `json:"fetch_error,omitempty"`
	ArticleCount  int       `json:"article_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Article struct {
	ID          string    `json:"id"`
	FeedID      string    `json:"feed_id"`
	FeedTitle   string    `json:"feed_title,omitempty"`
	GUID        string    `json:"guid"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Content     string    `json:"content,omitempty"`
	Link        string    `json:"link"`
	Author      string    `json:"author"`
	ImageURL    string    `json:"image_url"`
	PublishedAt time.Time `json:"published_at"`
	FetchedAt   time.Time `json:"fetched_at"`
	IsRead      bool      `json:"is_read"`
	IsStarred   bool      `json:"is_starred"`
	Tags        []string  `json:"tags,omitempty"`
}

type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}
