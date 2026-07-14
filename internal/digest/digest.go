// Package digest handles the daily grouped digest: parsing the agent-supplied
// grouping and persisting it (with a residual theme covering everything the
// agent left out).
package digest

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/marceloalmeidadev/feedclaw/internal/store"
)

// Input is the JSON document the agent produces for `digest save`.
type Input struct {
	Date      string  `json:"date,omitempty"`
	ModelNote string  `json:"model_note,omitempty"`
	Themes    []Theme `json:"themes"`
}

// Theme is one thematic grouping supplied by the agent.
type Theme struct {
	Name       string  `json:"name"`
	Summary    string  `json:"summary"`
	ArticleIDs []int64 `json:"article_ids"`
}

// ParseInput decodes a digest document, rejecting unknown fields to catch typos
// in the agent's output early.
func ParseInput(r io.Reader) (*Input, error) {
	dec := json.NewDecoder(io.LimitReader(r, 8<<20))
	dec.DisallowUnknownFields()
	var in Input
	if err := dec.Decode(&in); err != nil {
		return nil, fmt.Errorf("parse digest input: %w", err)
	}
	for i, t := range in.Themes {
		if t.Name == "" {
			return nil, fmt.Errorf("theme %d: name is required", i+1)
		}
	}
	return &in, nil
}

// Save persists the parsed input under date (the --date flag is authoritative
// over any date embedded in the JSON).
func Save(st *store.Store, date string, in *Input) (*store.Digest, error) {
	themes := make([]store.ThemeInput, len(in.Themes))
	for i, t := range in.Themes {
		themes[i] = store.ThemeInput{Name: t.Name, Summary: t.Summary, ArticleIDs: t.ArticleIDs}
	}
	return st.SaveDigest(date, in.ModelNote, themes)
}
