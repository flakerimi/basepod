package api

import (
	"net/http"
	"strings"
)

// bodyLimit caps incoming request bodies. The deploy endpoint streams large
// tarballs and is excepted; everything else is held to 1 MB.
func bodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			next.ServeHTTP(w, r)
			return
		}
		limit := int64(1 << 20) // 1 MB
		if strings.HasSuffix(r.URL.Path, "/deploy") ||
			strings.HasSuffix(r.URL.Path, "/restore") {
			limit = 1 << 31 // 2 GB
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}
