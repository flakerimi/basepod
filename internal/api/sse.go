package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func eventsSSEHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeErr(w, 500, "no_flush", "streaming unsupported")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch, cancel := d.Events.Subscribe(r.Context(), "")
		defer cancel()

		ping := time.NewTicker(30 * time.Second)
		defer ping.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-ping.C:
				fmt.Fprintf(w, ": ping\n\n")
				flusher.Flush()
			case evt, ok := <-ch:
				if !ok {
					return
				}
				b, _ := json.Marshal(evt)
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, b)
				flusher.Flush()
			}
		}
	}
}
