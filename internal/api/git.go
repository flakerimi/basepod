package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/flakerimi/basepod/internal/crypto"
	"github.com/flakerimi/basepod/internal/store/db"
)

// PUT /api/v1/apps/:name/git    — set or update git config
type gitConfigInput struct {
	URL        string `json:"url"`
	Branch     string `json:"branch"`
	Dockerfile string `json:"dockerfile"`
	// Either Token (PAT / app password) or Username+Password (CapRover-style).
	// Both are stored as one opaque secret blob: "user:password" if Username
	// is set, else just the token.
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
}

func putAppGitHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		a, err := d.Apps.GetByName(r.Context(), name)
		if err != nil {
			writeErr(w, 404, "not_found", "app not found")
			return
		}
		var body gitConfigInput
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, "bad_request", "invalid JSON")
			return
		}
		secret := body.Token
		if secret == "" && body.Password != "" {
			user := body.Username
			if user == "" {
				user = "x-access-token"
			}
			secret = user + ":" + body.Password
		}
		var enc []byte
		if secret != "" {
			enc, err = d.EnvCipher.Encrypt(secret)
			if err != nil {
				writeErr(w, 500, "encrypt_failed", err.Error())
				return
			}
		} else {
			// No new secret in this request — preserve the existing one.
			if row, gerr := d.Queries.GetAppGit(r.Context(), a.ID); gerr == nil {
				enc = row.GitTokenEncrypted
			}
		}
		if err := d.Queries.SetAppGit(r.Context(), db.SetAppGitParams{
			GitUrl:            body.URL,
			GitBranch:         body.Branch,
			GitDockerfile:     body.Dockerfile,
			GitTokenEncrypted: enc,
			UpdatedAt:         time.Now().Unix(),
			ID:                a.ID,
		}); err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		audit(r.Context(), d, "app.git.set", a.Name, map[string]any{"url": body.URL, "branch": body.Branch})
		writeJSON(w, 200, map[string]any{"ok": true})
	}
}

// GET /api/v1/apps/:name/git — returns config (without the secret), plus webhook URL.
func getAppGitHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		a, err := d.Apps.GetByName(r.Context(), name)
		if err != nil {
			writeErr(w, 404, "not_found", "app not found")
			return
		}
		row, _ := d.Queries.GetAppGit(r.Context(), a.ID)
		out := map[string]any{
			"url":            row.GitUrl,
			"branch":         row.GitBranch,
			"dockerfile":     row.GitDockerfile,
			"has_credential": len(row.GitTokenEncrypted) > 0,
			"has_webhook":    len(row.WebhookSecretEncrypted) > 0,
			"webhook_url":    webhookURL(r.Context(), d, a.Name),
		}
		writeJSON(w, 200, out)
	}
}

// POST /api/v1/apps/:name/webhook-secret — generate/rotate the shared HMAC secret.
func rotateWebhookSecretHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		a, err := d.Apps.GetByName(r.Context(), name)
		if err != nil {
			writeErr(w, 404, "not_found", "app not found")
			return
		}
		raw, err := crypto.NewKey() // 32 random bytes
		if err != nil {
			writeErr(w, 500, "rand_failed", err.Error())
			return
		}
		plain := hex.EncodeToString(raw)
		enc, err := d.EnvCipher.Encrypt(plain)
		if err != nil {
			writeErr(w, 500, "encrypt_failed", err.Error())
			return
		}
		if err := d.Queries.SetAppWebhookSecret(r.Context(), db.SetAppWebhookSecretParams{
			WebhookSecretEncrypted: enc,
			UpdatedAt:              time.Now().Unix(),
			ID:                     a.ID,
		}); err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		audit(r.Context(), d, "app.webhook.rotate", a.Name, nil)
		writeJSON(w, 200, map[string]any{
			"secret":      plain,
			"webhook_url": webhookURL(r.Context(), d, a.Name),
		})
	}
}

// POST /api/v1/apps/:name/webhook  — called by GitHub/GitLab/etc on push.
//
// GitHub: X-Hub-Signature-256 header carries the HMAC.
// Generic: X-Basepod-Signature-256 fallback.
func webhookDeliveryHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		a, err := d.Apps.GetByName(r.Context(), name)
		if err != nil {
			writeErr(w, 404, "not_found", "app not found")
			return
		}
		row, err := d.Queries.GetAppGit(r.Context(), a.ID)
		if err != nil || len(row.WebhookSecretEncrypted) == 0 {
			writeErr(w, 403, "no_webhook", "no webhook secret configured")
			return
		}
		secret, err := d.EnvCipher.Decrypt(row.WebhookSecretEncrypted)
		if err != nil {
			writeErr(w, 500, "decrypt_failed", err.Error())
			return
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeErr(w, 400, "read_failed", err.Error())
			return
		}
		sig := firstHeader(r, "X-Hub-Signature-256", "X-Basepod-Signature-256")
		if !verifyHMAC(secret, sig, raw) {
			writeErr(w, 403, "bad_signature", "HMAC mismatch")
			return
		}
		// Parse push payload (GitHub format covers the common case).
		var push struct {
			Ref        string `json:"ref"`
			HeadCommit struct {
				ID string `json:"id"`
			} `json:"head_commit"`
		}
		_ = json.Unmarshal(raw, &push)
		commit := push.HeadCommit.ID
		branch := strings.TrimPrefix(push.Ref, "refs/heads/")

		// Branch filter — only deploy if configured branch matches.
		configured := row.GitBranch
		if configured == "" {
			configured = "main"
		}
		if branch != "" && configured != "" && branch != configured {
			writeJSON(w, 200, map[string]any{"skipped": true, "reason": "branch_mismatch", "branch": branch, "configured": configured})
			return
		}

		token := ""
		if len(row.GitTokenEncrypted) > 0 {
			if t, err := d.EnvCipher.Decrypt(row.GitTokenEncrypted); err == nil {
				token = t
			}
		}
		audit(r.Context(), d, "app.webhook.delivery", a.Name, map[string]any{"commit": commit, "branch": branch})
		handleGitDeploy(w, r, d, a, row.GitUrl, configured, commit, extractTokenForGit(token))
	}
}

func extractTokenForGit(stored string) string {
	if stored == "" {
		return ""
	}
	// If stored as "user:pass", git accepts it as-is in URL. We pass only the
	// raw password portion when user == "x-access-token"; otherwise we keep
	// the full pair so url.UserPassword can split it later. injectToken sets
	// the username as "x-access-token" always, which works for GitHub PATs and
	// GitLab "oauth2"/PAT. For username+password style we re-encode.
	if idx := strings.IndexByte(stored, ':'); idx > 0 && stored[:idx] != "x-access-token" {
		// User explicitly supplied user:pass. Leave it intact — the git pkg
		// rewrites only when token is non-empty, and we'll abuse the format
		// by passing the full pair as the token. Less than ideal but it
		// works for GitLab/Bitbucket. (Refactor candidate.)
		return stored
	}
	if idx := strings.IndexByte(stored, ':'); idx > 0 {
		return stored[idx+1:]
	}
	return stored
}

func webhookURL(ctx context.Context, d Deps, name string) string {
	root, _ := d.Queries.GetSetting(ctx, "root_domain")
	adminSub, _ := d.Queries.GetSetting(ctx, "admin_subdomain")
	if adminSub == "" {
		adminSub = "bp"
	}
	if root != "" {
		return "https://" + adminSub + "." + root + "/api/v1/apps/" + name + "/webhook"
	}
	return "/api/v1/apps/" + name + "/webhook"
}

func verifyHMAC(secret, header string, body []byte) bool {
	if header == "" || secret == "" {
		return false
	}
	header = strings.TrimPrefix(header, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(header))
}

func firstHeader(r *http.Request, keys ...string) string {
	for _, k := range keys {
		if v := r.Header.Get(k); v != "" {
			return v
		}
	}
	return ""
}
