package store

import (
	"database/sql"
	"strings"
	"time"
)

// articleColumns is the article/feed column list; order must match scanArticle.
const articleColumns = `a.id, a.feed_id, a.guid, a.url, a.title, a.summary, a.content, a.full_content,
	a.author, a.published_at, a.fetched_at, a.read_at, a.starred,
	f.title, f.category`

// articleFrom is the base FROM clause joining feeds for title/category.
const articleFrom = `FROM articles a JOIN feeds f ON f.id = a.feed_id`

// articleSelect is the full projection used by the fixed-shape listings.
const articleSelect = `SELECT ` + articleColumns + ` ` + articleFrom

type scanner interface{ Scan(...any) error }

func scanArticle(row scanner) (*Article, error) {
	var a Article
	var summary, content, full, author, feedTitle, category sql.NullString
	var starred int
	if err := row.Scan(
		&a.ID, &a.FeedID, &a.GUID, &a.URL, &a.Title, &summary, &content, &full,
		&author, &a.PublishedAt, &a.FetchedAt, &a.ReadAt, &starred,
		&feedTitle, &category,
	); err != nil {
		return nil, err
	}
	a.Summary = summary.String
	a.Content = content.String
	a.FullContent = full.String
	a.Author = author.String
	a.FeedTitle = feedTitle.String
	a.Category = category.String
	a.Starred = starred != 0
	return &a, nil
}

func scanArticles(rows *sql.Rows) ([]*Article, error) {
	defer func() { _ = rows.Close() }()
	var out []*Article
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ArticleByID returns a single article (with feed metadata and cached
// full_content) or ErrNotFound.
func (s *Store) ArticleByID(id int64) (*Article, error) {
	row := s.db.QueryRow(articleSelect+` WHERE a.id = ?`, id)
	a, err := scanArticle(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return a, err
}

// UnreadFilter narrows an unread listing. Zero values mean "no filter".
type UnreadFilter struct {
	Since    time.Duration // only articles newer than now-Since (by published/fetched time)
	Category string
	Limit    int
}

// ListUnread returns unread articles, newest first, applying the filter.
func (s *Store) ListUnread(f UnreadFilter) ([]*Article, error) {
	q := articleSelect + ` WHERE a.read_at IS NULL`
	var args []any
	if f.Since > 0 {
		q += ` AND COALESCE(a.published_at, a.fetched_at) >= ?`
		args = append(args, time.Now().UTC().Add(-f.Since))
	}
	if f.Category != "" {
		q += ` AND f.category = ?`
		args = append(args, f.Category)
	}
	q += ` ORDER BY COALESCE(a.published_at, a.fetched_at) DESC`
	if f.Limit > 0 {
		q += ` LIMIT ?`
		args = append(args, f.Limit)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	return scanArticles(rows)
}

// SetRead marks the given articles read (read=true, timestamp = now) or unread
// (read=false, NULL). Returns the number of rows affected.
func (s *Store) SetRead(ids []int64, read bool) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders, args := inClause(ids)
	var q string
	if read {
		q = `UPDATE articles SET read_at = CURRENT_TIMESTAMP WHERE read_at IS NULL AND id IN (` + placeholders + `)`
	} else {
		q = `UPDATE articles SET read_at = NULL WHERE id IN (` + placeholders + `)`
	}
	res, err := s.db.Exec(q, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// MarkReadOlderThan marks every still-unread article older than the cutoff as
// read. Age is measured by published_at, falling back to fetched_at.
func (s *Store) MarkReadOlderThan(d time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-d)
	res, err := s.db.Exec(
		`UPDATE articles SET read_at = CURRENT_TIMESTAMP
		 WHERE read_at IS NULL AND COALESCE(published_at, fetched_at) < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// SetStarred toggles the starred flag on the given articles.
func (s *Store) SetStarred(ids []int64, starred bool) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders, args := inClause(ids)
	val := 0
	if starred {
		val = 1
	}
	res, err := s.db.Exec(
		`UPDATE articles SET starred = ? WHERE id IN (`+placeholders+`)`,
		append([]any{val}, args...)...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// SetFullContent stores the readability-extracted HTML for an article.
func (s *Store) SetFullContent(id int64, html string) error {
	res, err := s.db.Exec(`UPDATE articles SET full_content = ? WHERE id = ?`, html, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Search runs an FTS5 query over title/summary/full_content and returns
// matching articles ordered by relevance.
func (s *Store) Search(query string, limit int) ([]*Article, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		articleSelect+` JOIN articles_fts fts ON fts.rowid = a.id
		 WHERE articles_fts MATCH ? ORDER BY rank LIMIT ?`,
		ftsQuery(query), limit)
	if err != nil {
		return nil, err
	}
	return scanArticles(rows)
}

// inClause builds "?,?,?" and the matching args slice for an IN (...) clause.
func inClause(ids []int64) (string, []any) {
	marks := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		marks[i] = "?"
		args[i] = id
	}
	return strings.Join(marks, ","), args
}

// ftsQuery makes a user query safe for FTS5 by quoting each token as a phrase,
// so characters like ':' or '-' are treated literally rather than as operators.
func ftsQuery(q string) string {
	fields := strings.Fields(q)
	for i, f := range fields {
		fields[i] = `"` + strings.ReplaceAll(f, `"`, `""`) + `"`
	}
	return strings.Join(fields, " ")
}
