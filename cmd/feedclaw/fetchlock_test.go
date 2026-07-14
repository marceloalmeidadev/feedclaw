package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFetchLockExclusive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	release, err := acquireFetchLock()
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	// A second acquire while the first is held must be refused.
	if _, err := acquireFetchLock(); err != errFetchLocked {
		t.Fatalf("expected errFetchLocked, got %v", err)
	}
	release()
	// After release, acquiring again must succeed.
	release2, err := acquireFetchLock()
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	release2()
}

func TestStaleLockReclaimed(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(lockPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	// A lock owned by a long-dead PID must be reclaimed, not treated as held.
	if err := os.WriteFile(lockPath(), []byte("99999999\nold\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	release, err := acquireFetchLock()
	if err != nil {
		t.Fatalf("stale lock should be reclaimed, got %v", err)
	}
	release()
}

func TestPidAlive(t *testing.T) {
	if !pidAlive(os.Getpid()) {
		t.Error("the test's own PID should be alive")
	}
	if pidAlive(99999999) {
		t.Error("an absurd PID should be reported dead")
	}
	if pidAlive(0) {
		t.Error("PID 0 should be reported dead")
	}
}
