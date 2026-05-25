package api

import (
	"context"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// caddyCheckHandler responds 200 if `host` is a known domain (root, admin
// subdomain, app subdomain, or attached custom domain). Caddy's on-demand TLS
// calls this before issuing a cert for a hostname it sees in a TLS request.
func caddyCheckHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("domain")))
		if host == "" {
			host = strings.ToLower(r.URL.Query().Get("host"))
		}
		if host == "" {
			http.Error(w, "missing domain", 400)
			return
		}
		root, _ := d.Queries.GetSetting(r.Context(), "root_domain")
		adminSub, _ := d.Queries.GetSetting(r.Context(), "admin_subdomain")
		if adminSub == "" {
			adminSub = "bp"
		}
		root = strings.ToLower(strings.TrimSpace(root))
		if root != "" {
			if host == root || host == adminSub+"."+root {
				w.WriteHeader(200)
				return
			}
			// any <app>.<root>?
			if strings.HasSuffix(host, "."+root) {
				name := strings.TrimSuffix(host, "."+root)
				if _, err := d.Apps.GetByName(r.Context(), name); err == nil {
					w.WriteHeader(200)
					return
				}
			}
		}
		// custom domain attached to any app
		doms, _ := d.Queries.ListAllDomains(r.Context())
		for _, dr := range doms {
			if strings.EqualFold(dr.Domain, host) {
				w.WriteHeader(200)
				return
			}
		}
		http.Error(w, "unknown host", 404)
	}
}

func systemInfoHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out := map[string]any{
			"version":  d.Version,
			"go":       runtime.Version(),
			"os":       runtime.GOOS,
			"arch":     runtime.GOARCH,
			"cpus":     runtime.NumCPU(),
			"podman":   nil,
			"caddy":    false,
		}
		if d.Podman != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if v, err := d.Podman.Version(ctx); err == nil {
				out["podman"] = v
			}
		}
		if d.Caddy != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			out["caddy"] = d.Caddy.Ping(ctx) == nil
		}
		writeJSON(w, 200, out)
	}
}

func systemStorageHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(d.Cfg.DataDir, &stat); err != nil {
			writeErr(w, 500, "statfs", err.Error())
			return
		}
		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bavail * uint64(stat.Bsize)
		used := total - free
		writeJSON(w, 200, map[string]any{
			"data_dir":  d.Cfg.DataDir,
			"total":     total,
			"used":      used,
			"available": free,
		})
	}
}

func systemPruneHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cmd := exec.CommandContext(r.Context(), "podman", "system", "prune", "-af")
		out, err := cmd.CombinedOutput()
		if err != nil {
			writeErr(w, 500, "prune_failed", string(out))
			return
		}
		writeJSON(w, 200, map[string]any{"output": string(out)})
	}
}

func systemProcessesHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.Podman == nil {
			writeJSON(w, 200, map[string]any{"containers": []any{}})
			return
		}
		cs, err := d.Podman.ContainerList(r.Context(), true)
		if err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"containers": cs})
	}
}
