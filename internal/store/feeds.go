package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("not found")

const feedColumns = `id, url, site_url, title, category, etag, last_modified,
	last_fetch_at, last_status, error_count, disabled, created_at`

func scanFeed(row interface{ Scan(...any) error }) (*Feed, error) {
	var f Feed
	var siteURL, etag, lastMod sql.NullString
	if err := row.Scan(&f.ID, &f.URL, &siteURL, &f.Title, &f.Category, &etag,
		&lastMod, &f.LastFetchAt, &f.LastStatus, &f.ErrorCount, &f.Disabled,
		&f.CreatedAt); err != nil {
		return nil, err
	}
	f.SiteURL = siteURL.String
	f.ETag = etag.String
	f.LastModified = lastMod.String
	return &f, nil
}

// AddFeed inserts a feed. If the url already exists the existing feed is
// returned with created=false (idempotent import).
func (s *Store) AddFeed(url, title, siteURL, category string) (feed *Feed, created bool, err error) {
	if title == "" {
		title = url
	}
	res, err := s.db.Exec(
		`INSERT INTO feeds (url, title, site_url, category) VALUES (?, ?, ?, ?)
		 ON CONFLICT(url) DO NOTHING`,
		url, title, nullString(siteURL), category)
	if err != nil {
		return nil, false, fmt.Errorf("insert feed: %w", err)
	}
	affected, _ := res.RowsAffected()
	f, err := s.FeedByURL(url)
	if err != nil {
		return nil, false, err
	}
	return f, affected > 0, nil
}

// FeedByURL looks up a feed by its xmlUrl.
func (s *Store) FeedByURL(url string) (*Feed, error) {
	row := s.db.QueryRow(`SELECT `+feedColumns+` FROM feeds WHERE url = ?`, url)
	f, err := scanFeed(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return f, err
}

// FeedByID looks up a feed by primary key.
func (s *Store) FeedByID(id int64) (*Feed, error) {
	row := s.db.QueryRow(`SELECT `+feedColumns+` FROM feeds WHERE id = ?`, id)
	f, err := scanFeed(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return f, err
}

// ListFeeds returns all feeds ordered by category then title.
func (s *Store) ListFeeds() ([]*Feed, error) {
	rows, err := s.db.Query(`SELECT ` + feedColumns + ` FROM feeds ORDER BY category, title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Feed
	for rows.Next() {
		f, err := scanFeed(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// RemoveFeed deletes a feed (and cascades its articles) by url.
func (s *Store) RemoveFeed(url string) error {
	res, err := s.db.Exec(`DELETE FROM feeds WHERE url = ?`, url)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkFetchSuccess records a successful fetch: updates caching headers, status,
// timestamp and resets the consecutive error counter.
func (s *Store) MarkFetchSuccess(feedID int64, status int, etag, lastMod string) error {
	_, err := s.db.Exec(
		`UPDATE feeds SET last_status = ?, etag = ?, last_modified = ?,
		 last_fetch_at = CURRENT_TIMESTAMP, error_count = 0 WHERE id = ?`,
		status, nullString(etag), nullString(lastMod), feedID)
	return err
}

// MarkFetchError increments the consecutive error counter and records status.
func (s *Store) MarkFetchError(feedID int64, status int) error {
	_, err := s.db.Exec(
		`UPDATE feeds SET last_status = ?, last_fetch_at = CURRENT_TIMESTAMP,
		 error_count = error_count + 1 WHERE id = ?`,
		nullInt(status), feedID)
	return err
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullInt(i int) any {
	if i == 0 {
		return nil
	}
	return i
}
