package main

// Semantic exit codes for `feedclaw fetch`. They are the contract the OpenClaw
// on-exit pipeline reads: the agent (digest flow) is woken only on 0 and 20.
const (
	exitOK          = 0  // success, there are new unread articles
	exitNothingNew  = 10 // success, nothing new — do not wake the LLM
	exitPartial     = 20 // some feeds failed, but there are new articles
	exitNetworkFail = 30 // total failure: no feed reachable
	exitConfigError = 40 // config / database inaccessible
	exitLocked      = 50 // another fetch is already running
)

// exitError carries a process exit code out of a cobra RunE. main() unwraps it
// to call os.Exit with the code; a nil inner err means a clean non-zero exit
// (e.g. code 10 "nothing new") that must NOT print an "error:" line.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *exitError) Unwrap() error { return e.err }
