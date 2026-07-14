package main

import (
	"testing"
	"unicode/utf8"
)

func TestTruncateKeepsValidUTF8(t *testing.T) {
	// A pt-BR title full of multibyte characters. Byte-slicing would cut one in
	// half and yield invalid UTF-8 in the terminal.
	s := "Símbolo de operação não é ação — atenção às runas"
	got := truncate(s, 12)

	if !utf8.ValidString(got) {
		t.Fatalf("truncate produced invalid UTF-8: %q", got)
	}
	if n := utf8.RuneCountInString(got); n > 12 {
		t.Fatalf("expected at most 12 runes, got %d (%q)", n, got)
	}
	// Short strings are returned unchanged.
	if truncate("curto", 12) != "curto" {
		t.Fatal("short string should be returned unchanged")
	}
	// The truncation marker is appended.
	if got[len(got)-len("…"):] != "…" {
		t.Fatalf("expected ellipsis suffix, got %q", got)
	}
}
