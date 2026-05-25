package api

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// spaFS is filled in once the Vue build artifact exists. For phase 1 we ship a
// placeholder so the binary still serves a sensible root page.
//
//go:embed placeholder/*
var spaFS embed.FS

func spaHandler(d Deps) http.Handler {
	sub, err := fs.Sub(spaFS, "placeholder")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "SPA assets missing", http.StatusInternalServerError)
		})
	}
	file := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(sub, path); err != nil {
			r2 := *r
			r2.URL.Path = "/"
			file.ServeHTTP(w, &r2)
			return
		}
		file.ServeHTTP(w, r)
	})
}
