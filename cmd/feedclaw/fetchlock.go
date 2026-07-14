package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// errFetchLocked signals that a live fetch already holds the lock.
var errFetchLocked = errors.New("another fetch is already running")

func lockPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "feedclaw-fetch.lock"
	}
	return filepath.Join(dir, "feedclaw", "fetch.lock")
}

// acquireFetchLock takes an exclusive fetch lock. It returns errFetchLocked if a
// still-running fetch holds it; a stale lock (whose owning PID is dead) is
// reclaimed automatically. The returned release removes the lock.
func acquireFetchLock() (release func(), err error) {
	path := lockPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d\n%s\n", os.Getpid(), time.Now().Format(time.RFC3339))
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if pidAlive(readLockPID(path)) {
			return nil, errFetchLocked
		}
		// Stale lock from a dead process — reclaim and retry once.
		_ = os.Remove(path)
	}
	return nil, errFetchLocked
}

func readLockPID(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	first := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)[0]
	pid, _ := strconv.Atoi(strings.TrimSpace(first))
	return pid
}

// pidAlive reports whether a process with the given PID exists (Unix). Signal 0
// checks for existence without affecting the process; EPERM means it exists but
// is owned by another user.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
