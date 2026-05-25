package git

import (
	"fmt"
	"os"
)

type Error struct {
	Op     string
	Repo   string
	Output string
	Err    error
}

func (e *Error) Error() string {
	return fmt.Sprintf("git %s %s: %v: %s", e.Op, e.Repo, e.Err, e.Output)
}

func (e *Error) Unwrap() error { return e.Err }

func pathEnv() string {
	if v := os.Getenv("PATH"); v != "" {
		return v
	}
	return "/usr/local/bin:/usr/bin:/bin:/opt/homebrew/bin"
}

func homeEnv() string {
	if v := os.Getenv("HOME"); v != "" {
		return v
	}
	return "/tmp"
}
