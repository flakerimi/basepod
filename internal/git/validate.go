package git

import (
	"fmt"
	"net/url"
	"strings"
)

// validateRef rejects URLs / branches / commits that could be interpreted as
// command-line options (starting with `-`) or use unexpected schemes. We pass
// `--` to git as well as a defense in depth, but argument-level validation is
// cheaper than relying on git to honor the separator.
func validateRef(rawURL, branch, commit string) error {
	if rawURL == "" {
		return fmt.Errorf("git: empty url")
	}
	if strings.HasPrefix(rawURL, "-") {
		return fmt.Errorf("git: url cannot start with -")
	}
	// Allowed schemes: https (preferred), http, ssh, plus git@host:owner/repo shorthand.
	if !strings.HasPrefix(rawURL, "git@") {
		u, err := url.Parse(rawURL)
		if err != nil {
			return fmt.Errorf("git: invalid url: %w", err)
		}
		switch u.Scheme {
		case "https", "http", "ssh", "git":
		default:
			return fmt.Errorf("git: scheme %q not allowed (use https, ssh, or git@host)", u.Scheme)
		}
		if u.Host == "" {
			return fmt.Errorf("git: missing host in url")
		}
	}
	if strings.HasPrefix(branch, "-") {
		return fmt.Errorf("git: branch cannot start with -")
	}
	if strings.ContainsAny(branch, " \t\n\r\v\f") {
		return fmt.Errorf("git: branch must not contain whitespace")
	}
	if strings.HasPrefix(commit, "-") {
		return fmt.Errorf("git: commit cannot start with -")
	}
	// commit SHA — allow 7..64 hex chars or empty
	if commit != "" {
		if len(commit) < 7 || len(commit) > 64 {
			return fmt.Errorf("git: commit must be a 7-64 char SHA")
		}
		for _, c := range commit {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return fmt.Errorf("git: commit must be hex SHA")
			}
		}
	}
	return nil
}
