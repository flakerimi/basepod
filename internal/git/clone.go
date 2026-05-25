// Package git provides a thin shell wrapper around the local `git` binary,
// scoped to the operations BasePod's deploy pipeline needs: shallow clone of a
// branch (or specific commit) into a workdir.
package git

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

type CloneOptions struct {
	URL      string // https://github.com/owner/repo[.git]
	Branch   string // default: main
	Commit   string // optional pin
	Token    string // GitHub PAT for private repos (injected into URL)
	Dest     string // workdir to clone into
	Depth    int    // 0 = full clone (rare), default 1
}

// Clone clones URL into Dest at the requested branch or commit. Token, when
// non-empty, is injected as x-access-token in the URL — never logged.
func Clone(ctx context.Context, opts CloneOptions) error {
	if opts.URL == "" || opts.Dest == "" {
		return errors.New("git: URL and Dest required")
	}
	if opts.Branch == "" {
		opts.Branch = "main"
	}
	if opts.Depth == 0 {
		opts.Depth = 1
	}
	authURL, err := injectToken(opts.URL, opts.Token)
	if err != nil {
		return err
	}
	args := []string{
		"clone", "--depth", fmt.Sprintf("%d", opts.Depth),
		"--branch", opts.Branch, "--single-branch",
		authURL, opts.Dest,
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(envClean(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=true",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &Error{Op: "clone", Repo: redact(authURL), Output: string(out), Err: err}
	}
	if opts.Commit != "" {
		// Fetch the specific commit then check it out — depth=1 may not have it.
		fc := exec.CommandContext(ctx, "git", "-C", opts.Dest, "fetch", "--depth", "1", "origin", opts.Commit)
		fc.Env = cmd.Env
		if fout, ferr := fc.CombinedOutput(); ferr != nil {
			return &Error{Op: "fetch-commit", Repo: redact(authURL), Output: string(fout), Err: ferr}
		}
		co := exec.CommandContext(ctx, "git", "-C", opts.Dest, "checkout", "--detach", opts.Commit)
		if cout, cerr := co.CombinedOutput(); cerr != nil {
			return &Error{Op: "checkout", Repo: redact(authURL), Output: string(cout), Err: cerr}
		}
	}
	return nil
}

// Sha resolves the current HEAD SHA inside dest.
func Sha(ctx context.Context, dest string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", dest, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// injectToken rewrites https URLs to include x-access-token auth when token
// is set. Returns the URL unchanged for empty token or non-https schemes.
func injectToken(raw, token string) (string, error) {
	if token == "" {
		return raw, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("git: parse url: %w", err)
	}
	if u.Scheme != "https" {
		return raw, nil
	}
	u.User = url.UserPassword("x-access-token", token)
	return u.String(), nil
}

func redact(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.User != nil {
		u.User = url.User("***")
	}
	return u.String()
}

func envClean() []string {
	// We intentionally do NOT inherit the parent's GIT_* env so a misconfigured
	// dev machine can't leak credentials. PATH is preserved so `git` resolves.
	return []string{
		"PATH=" + pathEnv(),
		"HOME=" + homeEnv(),
		"LANG=C",
	}
}
