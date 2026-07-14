package main

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	cases := map[string]time.Duration{
		"24h":  24 * time.Hour,
		"7d":   7 * 24 * time.Hour,
		"2w":   14 * 24 * time.Hour,
		"30m":  30 * time.Minute,
		"1.5d": time.Duration(1.5 * float64(24*time.Hour)),
	}
	for in, want := range cases {
		got, err := parseDuration(in)
		if err != nil {
			t.Errorf("parseDuration(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parseDuration(%q) = %v, want %v", in, got, want)
		}
	}
	if _, err := parseDuration("nonsense"); err == nil {
		t.Error("expected error for invalid duration")
	}
	if _, err := parseDuration(""); err == nil {
		t.Error("expected error for empty duration")
	}
}

func TestParseIDs(t *testing.T) {
	ids, err := parseIDs([]string{"1", "42", " 7 "})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 42 || ids[2] != 7 {
		t.Fatalf("unexpected ids: %v", ids)
	}
	if _, err := parseIDs([]string{"1", "x"}); err == nil {
		t.Error("expected error for non-numeric id")
	}
}
