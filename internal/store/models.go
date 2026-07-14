package store

import "time"

// Feed mirrors a row in the feeds table.
type Feed struct {
	ID           int64      `json:"id"`
	URL          string     `json:"url"`
	SiteURL      string     `json:"site_url,omitempty"`
	Title        string     `json:"title"`
	Category     string     `json:"category"`
	ETag         string     `json:"etag,omitempty"`
	LastModified string     `json:"last_modified,omitempty"`
	LastFetchAt  *time.Time `json:"last_fetch_at,omitempty"`
	LastStatus   *int       `json:"last_status,omitempty"`
	ErrorCount   int        `json:"error_count"`
	Disabled     bool       `json:"disabled"`
	CreatedAt    time.Time  `json:"created_at"`
}

// Article mirrors a row in the articles table.
type Article struct {
	ID          int64      `json:"id"`
	FeedID      int64      `json:"feed_id"`
	GUID        string     `json:"guid"`
	URL         string     `json:"url"`
	Title       string     `json:"title"`
	Summary     string     `json:"summary,omitempty"`
	Content     string     `json:"content,omitempty"`
	FullContent string     `json:"full_content,omitempty"`
	Author      string     `json:"author,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	FetchedAt   time.Time  `json:"fetched_at"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
	Starred     bool       `json:"starred"`

	// FeedTitle/Category are joined in for convenience in listings; not stored
	// on the articles row itself.
	FeedTitle string `json:"feed_title,omitempty"`
	Category  string `json:"category,omitempty"`
}

// Digest is a day's grouped view of the unread articles.
type Digest struct {
	ID          int64         `json:"id"`
	Date        string        `json:"date"` // YYYY-MM-DD
	GeneratedAt time.Time     `json:"generated_at"`
	ModelNote   string        `json:"model_note,omitempty"`
	Themes      []DigestTheme `json:"themes"`
}

// DigestTheme is one thematic group within a digest.
type DigestTheme struct {
	ID           int64      `json:"id"`
	Position     int        `json:"position"`
	Name         string     `json:"name"`
	Summary      string     `json:"summary"`
	ArticleCount int        `json:"article_count"`
	Articles     []*Article `json:"articles,omitempty"` // populated by ThemeArticles
}

// ThemeInput is one theme as supplied to SaveDigest (before persistence).
type ThemeInput struct {
	Name       string
	Summary    string
	ArticleIDs []int64
}
