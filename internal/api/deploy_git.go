package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"github.com/flakerimi/basepod/internal/apps"
	"github.com/flakerimi/basepod/internal/deploy"
	gitpkg "github.com/flakerimi/basepod/internal/git"
)

// handleGitDeployFromStored uses the URL/branch/token saved on the app.
func handleGitDeployFromStored(w http.ResponseWriter, r *http.Request, d Deps, a *apps.App, commit string) {
	row, err := d.Queries.GetAppGit(r.Context(), a.ID)
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	if row.GitUrl == "" {
		writeErr(w, 400, "no_git", "no git config stored — pass url/branch in body or call POST /apps/:name/git first")
		return
	}
	token := ""
	if len(row.GitTokenEncrypted) > 0 {
		token, err = d.EnvCipher.Decrypt(row.GitTokenEncrypted)
		if err != nil {
			writeErr(w, 500, "decrypt_failed", err.Error())
			return
		}
	}
	handleGitDeploy(w, r, d, a, row.GitUrl, row.GitBranch, commit, token)
}

func handleGitDeploy(w http.ResponseWriter, r *http.Request, d Deps, a *apps.App, gitURL, branch, commit, token string) {
	if gitURL == "" {
		writeErr(w, 400, "bad_request", "git.url required")
		return
	}
	if branch == "" {
		branch = "main"
	}
	topic := "app:" + a.Name + ":deploy"

	workdir := filepath.Join(d.Cfg.WorkDir(), a.Name, "git")
	_ = os.RemoveAll(workdir)
	if err := os.MkdirAll(filepath.Dir(workdir), 0o755); err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	verID, _ := d.Apps.RecordVersion(r.Context(), a.ID, "pending", "", "cloning")
	audit(r.Context(), d, "app.deploy", a.Name, map[string]any{
		"mode": "git", "url": gitURL, "branch": branch, "commit": commit,
	})
	d.Events.Publish(topic, "started", map[string]any{"kind": "git", "url": gitURL, "branch": branch})

	go func() {
		ctx := context.Background()
		if err := gitpkg.Clone(ctx, gitpkg.CloneOptions{
			URL: gitURL, Branch: branch, Commit: commit, Token: token, Dest: workdir,
		}); err != nil {
			updateVersionStatus(ctx, d, verID, err)
			d.Events.Publish(topic, "failed", map[string]any{"error": err.Error()})
			return
		}
		sha, _ := gitpkg.Sha(ctx, workdir)
		d.Events.Publish(topic, "cloned", map[string]any{"commit": sha})

		// Dockerfile location from stored git_dockerfile if any.
		dockerfile := "Dockerfile"
		if row, err := d.Queries.GetAppGit(ctx, a.ID); err == nil && row.GitDockerfile != "" {
			dockerfile = row.GitDockerfile
		}

		res, err := d.Builder.BuildFromDir(ctx, a.Name, workdir, dockerfile, func(line string) {
			d.Events.Publish(topic, "log", map[string]any{"line": line})
		})
		if err != nil {
			updateVersionStatus(ctx, d, verID, err)
			d.Events.Publish(topic, "failed", map[string]any{"error": err.Error()})
			return
		}
		d.Events.Publish(topic, "built", map[string]any{"tag": res.Tag, "version": res.Version, "commit": sha})

		err = d.Orchestrator.Deploy(ctx, deploy.Request{
			App: a, ImageTag: res.Tag, Version: res.Version, Strategy: a.DeployStrategy,
		})
		updateVersionStatus(ctx, d, verID, err)
		if err != nil {
			d.Events.Publish(topic, "failed", map[string]any{"error": err.Error()})
			return
		}
		d.Events.Publish(topic, "deployed", map[string]any{"version": res.Version, "commit": sha})
	}()
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued"})
}
