package api

import "net/http"

// notImplemented is used by handlers that still need wiring.
func notImplemented(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusNotImplemented, "not_implemented", name+" not implemented yet")
	}
}
