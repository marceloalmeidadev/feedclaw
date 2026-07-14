package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// ResidualThemeName is the engine-generated theme that captures every unread
// article the agent did not place in a theme, guaranteeing the digest covers
// all unread articles.
const ResidualThemeName = "Outros"

// SaveDigest persists (replacing any existing digest for the same date) a
// grouped digest. Every referenced article must exist and be unread at save
// time; unreferenced unread articles are collected into a residual "Outros"
// theme so the digest always covers the full unread set.
func (s *Store) SaveDigest(date, modelNote string, themes []ThemeInput) (*Digest, error) {
	if date == "" {
		return nil, errors.New("digest date is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }() // no-op once committed

	referenced, err := validateThemeArticles(tx, themes)
	if err != nil {
		return nil, err
	}
	residual, err := residualUnread(tx, referenced)
	if err != nil {
		return nil, err
	}

	final := append([]ThemeInput(nil), themes...)
	if len(residual) > 0 {
		final = append(final, ThemeInput{Name: ResidualThemeName, ArticleIDs: residual})
	}

	if _, err := tx.Exec(`DELETE FROM digests WHERE date = ?`, date); err != nil {
		return nil, err
	}
	res, err := tx.Exec(`INSERT INTO digests (date, model_note) VALUES (?, ?)`, date, nullString(modelNote))
	if err != nil {
		return nil, fmt.Errorf("insert digest: %w", err)
	}
	digestID, _ := res.LastInsertId()

	for i, th := range final {
		if err := insertTheme(tx, digestID, i+1, th); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetDigest(date)
}

// validateThemeArticles checks every referenced article exists and is unread,
// returning the set of referenced ids.
func validateThemeArticles(tx *sql.Tx, themes []ThemeInput) (map[int64]bool, error) {
	referenced := make(map[int64]bool)
	for _, th := range themes {
		for _, id := range th.ArticleIDs {
			var isUnread int
			err := tx.QueryRow(`SELECT read_at IS NULL FROM articles WHERE id = ?`, id).Scan(&isUnread)
			switch {
			case err == sql.ErrNoRows:
				return nil, fmt.Errorf("article %d does not exist", id)
			case err != nil:
				return nil, err
			case isUnread == 0:
				return nil, fmt.Errorf("article %d is already read", id)
			}
			referenced[id] = true
		}
	}
	return referenced, nil
}

// residualUnread returns the unread article ids not present in referenced.
func residualUnread(tx *sql.Tx, referenced map[int64]bool) ([]int64, error) {
	rows, err := tx.Query(`SELECT id FROM articles WHERE read_at IS NULL ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var residual []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if !referenced[id] {
			residual = append(residual, id)
		}
	}
	return residual, rows.Err()
}

func insertTheme(tx *sql.Tx, digestID int64, position int, th ThemeInput) error {
	res, err := tx.Exec(
		`INSERT INTO digest_themes (digest_id, position, name, summary) VALUES (?, ?, ?, ?)`,
		digestID, position, th.Name, th.Summary)
	if err != nil {
		return fmt.Errorf("insert theme %q: %w", th.Name, err)
	}
	themeID, _ := res.LastInsertId()
	for _, articleID := range th.ArticleIDs {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO digest_theme_articles (theme_id, article_id) VALUES (?, ?)`,
			themeID, articleID); err != nil {
			return fmt.Errorf("link article %d: %w", articleID, err)
		}
	}
	return nil
}

// GetDigest returns the digest for a date with its themes and article counts.
func (s *Store) GetDigest(date string) (*Digest, error) {
	var d Digest
	var modelNote sql.NullString
	err := s.db.QueryRow(
		`SELECT id, date, generated_at, model_note FROM digests WHERE date = ?`, date).
		Scan(&d.ID, &d.Date, &d.GeneratedAt, &modelNote)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	d.ModelNote = modelNote.String

	rows, err := s.db.Query(
		`SELECT t.id, t.position, t.name, t.summary, COUNT(ta.article_id)
		 FROM digest_themes t
		 LEFT JOIN digest_theme_articles ta ON ta.theme_id = t.id
		 WHERE t.digest_id = ?
		 GROUP BY t.id ORDER BY t.position`, d.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var th DigestTheme
		if err := rows.Scan(&th.ID, &th.Position, &th.Name, &th.Summary, &th.ArticleCount); err != nil {
			return nil, err
		}
		d.Themes = append(d.Themes, th)
	}
	return &d, rows.Err()
}

// LatestDigestDate returns the most recent digest date, or ErrNotFound if none.
func (s *Store) LatestDigestDate() (string, error) {
	var date string
	err := s.db.QueryRow(`SELECT date FROM digests ORDER BY date DESC LIMIT 1`).Scan(&date)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return date, err
}

// ThemeByID returns a theme's metadata.
func (s *Store) ThemeByID(themeID int64) (*DigestTheme, error) {
	var th DigestTheme
	err := s.db.QueryRow(
		`SELECT id, position, name, summary FROM digest_themes WHERE id = ?`, themeID).
		Scan(&th.ID, &th.Position, &th.Name, &th.Summary)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &th, err
}

// ThemeArticles returns all articles belonging to a theme, newest first.
func (s *Store) ThemeArticles(themeID int64) ([]*Article, error) {
	rows, err := s.db.Query(
		articleSelect+` JOIN digest_theme_articles ta ON ta.article_id = a.id
		 WHERE ta.theme_id = ?
		 ORDER BY COALESCE(a.published_at, a.fetched_at) DESC`, themeID)
	if err != nil {
		return nil, err
	}
	return scanArticles(rows)
}

// MarkReadAllInTheme marks every still-unread article in a theme as read.
func (s *Store) MarkReadAllInTheme(themeID int64) (int64, error) {
	res, err := s.db.Exec(
		`UPDATE articles SET read_at = CURRENT_TIMESTAMP
		 WHERE read_at IS NULL AND id IN (
		   SELECT article_id FROM digest_theme_articles WHERE theme_id = ?
		 )`, themeID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
