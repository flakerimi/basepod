package api

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// loginLimiter is a token-bucket per source IP. Tuned for human login attempts:
// 5 attempts per minute, refilling at 1/minute. Cleaned lazily.
type loginLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens   int
	last     time.Time
}

var (
	loginRL = &loginLimiter{buckets: map[string]*bucket{}}

	loginMaxBurst  = 5
	loginRefill    = time.Minute // one token per minute
)

func (l *loginLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	now := time.Now()
	if !ok {
		l.buckets[key] = &bucket{tokens: loginMaxBurst - 1, last: now}
		return true
	}
	elapsed := now.Sub(b.last)
	gained := int(elapsed / loginRefill)
	if gained > 0 {
		b.tokens += gained
		if b.tokens > loginMaxBurst {
			b.tokens = loginMaxBurst
		}
		b.last = now
	}
	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}

func loginRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if !loginRL.allow(host) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "too many login attempts; try again in a minute", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
