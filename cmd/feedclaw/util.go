package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseDuration extends time.ParseDuration with day (d) and week (w) suffixes,
// which the CLI accepts for --since/--older-than (e.g. "7d", "2w", "24h").
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	switch suffix := s[len(s)-1]; suffix {
	case 'd', 'w':
		n, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		unit := 24 * time.Hour
		if suffix == 'w' {
			unit = 7 * 24 * time.Hour
		}
		return time.Duration(n * float64(unit)), nil
	default:
		return time.ParseDuration(s)
	}
}

// parseIDs converts positional string args into article IDs.
func parseIDs(args []string) ([]int64, error) {
	ids := make([]int64, 0, len(args))
	for _, a := range args {
		id, err := strconv.ParseInt(strings.TrimSpace(a), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid article id %q", a)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
