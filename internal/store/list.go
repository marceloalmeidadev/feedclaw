package store

import "strings"

// ArticleFilter is the flexible article query used by the HTTP API. Zero values
// mean "no filter".
type ArticleFilter struct {
	Status   string // "unread" | "read" | "starred" | "" (all)
	Category string
	ThemeID  int64
	Query    string // FTS5 match
	Page     int    // 1-based
	PerPage  int
}

const (
	defaultPerPage = 50
	maxPerPage     = 200
)

// ListArticles returns a page of articles matching the filter, newest first
// (or by relevance when a query is given), plus the total match count.
func (f ArticleFilter) paging() (limit, offset int) {
	perPage := f.PerPage
	switch {
	case perPage <= 0:
		perPage = defaultPerPage
	case perPage > maxPerPage:
		perPage = maxPerPage
	}
	page := f.Page
	if page <= 0 {
		page = 1
	}
	return perPage, (page - 1) * perPage
}

// build assembles the FROM clause (with any needed joins), the WHERE conditions,
// their args and the ORDER BY clause for the filter.
func (f ArticleFilter) build() (from string, conds []string, args []any, order string) {
	from = "articles a JOIN feeds f ON f.id = a.feed_id"
	order = "ORDER BY COALESCE(a.published_at, a.fetched_at) DESC"

	if f.ThemeID > 0 {
		from += " JOIN digest_theme_articles ta ON ta.article_id = a.id"
		conds = append(conds, "ta.theme_id = ?")
		args = append(args, f.ThemeID)
	}
	if f.Query != "" {
		from += " JOIN articles_fts fts ON fts.rowid = a.id"
		conds = append(conds, "articles_fts MATCH ?")
		args = append(args, ftsQuery(f.Query))
		order = "ORDER BY rank"
	}
	switch f.Status {
	case "unread":
		conds = append(conds, "a.read_at IS NULL")
	case "read":
		conds = append(conds, "a.read_at IS NOT NULL")
	case "starred":
		conds = append(conds, "a.starred = 1")
	}
	if f.Category != "" {
		conds = append(conds, "f.category = ?")
		args = append(args, f.Category)
	}
	return from, conds, args, order
}

func whereClause(conds []string) string {
	if len(conds) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conds, " AND ")
}

// ListArticles runs the filtered, paginated query and returns the page plus the
// total number of matches (ignoring pagination).
func (s *Store) ListArticles(f ArticleFilter) (articles []*Article, total int, err error) {
	from, conds, args, order := f.build()
	where := whereClause(conds)

	if err = s.db.QueryRow(`SELECT COUNT(*) FROM `+from+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit, offset := f.paging()
	q := `SELECT ` + articleColumns + ` FROM ` + from + where + ` ` + order + ` LIMIT ? OFFSET ?`
	rows, err := s.db.Query(q, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	articles, err = scanArticles(rows)
	return articles, total, err
}

// CategoryCount is the unread count for one category.
type CategoryCount struct {
	Category string `json:"category"`
	Unread   int    `json:"unread"`
}

// Stats holds the badge counters shown by the UI.
type Stats struct {
	UnreadTotal  int             `json:"unread_total"`
	StarredTotal int             `json:"starred_total"`
	ByCategory   []CategoryCount `json:"by_category"`
}

// Stats returns unread/starred totals and per-category unread counts.
func (s *Store) Stats() (*Stats, error) {
	var st Stats
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM articles WHERE read_at IS NULL`).Scan(&st.UnreadTotal); err != nil {
		return nil, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM articles WHERE starred = 1`).Scan(&st.StarredTotal); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT f.category, COUNT(*) FROM articles a JOIN feeds f ON f.id = a.feed_id
		 WHERE a.read_at IS NULL GROUP BY f.category ORDER BY f.category`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var c CategoryCount
		if err := rows.Scan(&c.Category, &c.Unread); err != nil {
			return nil, err
		}
		st.ByCategory = append(st.ByCategory, c)
	}
	return &st, rows.Err()
}
