package store

import (
	"crypto/sha256"
	"encoding/hex"
)

// FallbackGUID derives a stable guid for items whose feed omits one, so dedup
// on (feed_id, guid) still works across fetches.
func FallbackGUID(url, title string) string {
	sum := sha256.Sum256([]byte(url + "\x00" + title))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// UpsertArticle inserts an article if (feed_id, guid) is new. Existing rows are
// left untouched so read state and cached full_content survive re-fetches.
// Returns inserted=true only when a new row was created.
func (s *Store) UpsertArticle(a *Article) (inserted bool, err error) {
	if a.GUID == "" {
		a.GUID = FallbackGUID(a.URL, a.Title)
	}
	res, err := s.db.Exec(
		`INSERT INTO articles (feed_id, guid, url, title, summary, content, author, published_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(feed_id, guid) DO NOTHING`,
		a.FeedID, a.GUID, a.URL, a.Title, nullString(a.Summary),
		nullString(a.Content), nullString(a.Author), a.PublishedAt)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		a.ID, _ = res.LastInsertId()
	}
	return n > 0, nil
}

// CountArticles returns the total number of stored articles.
func (s *Store) CountArticles() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM articles`).Scan(&n)
	return n, err
}

// CountUnread returns the number of unread articles.
func (s *Store) CountUnread() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM articles WHERE read_at IS NULL`).Scan(&n)
	return n, err
}
