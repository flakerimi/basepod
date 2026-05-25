package api

import (
	"bufio"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func logsSSEHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.Podman == nil {
			writeErr(w, 503, "podman_unavailable", "podman not connected")
			return
		}
		name := chi.URLParam(r, "name")
		if _, err := d.Apps.GetByName(r.Context(), name); err != nil {
			writeErr(w, 404, "not_found", "app not found")
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeErr(w, 500, "no_flush", "streaming unsupported")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		stream, err := d.Podman.ContainerLogsStream(r.Context(), name, true)
		if err != nil {
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
			flusher.Flush()
			return
		}
		defer stream.Close()

		scanner := bufio.NewScanner(stream)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			fmt.Fprintf(w, "data: %s\n\n", scanner.Text())
			flusher.Flush()
		}
	}
}
