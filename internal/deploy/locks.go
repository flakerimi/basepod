package deploy

import "sync"

// AppLocks gives a per-app mutex. Acquire returns ok=false if a deploy is
// already in progress for that app — the API can return 409.
//
// The map itself is protected; each value is a buffered channel of size 1
// (acts as a non-blocking mutex via send/receive).
type AppLocks struct {
	mu    sync.Mutex
	locks map[string]chan struct{}
}

func NewAppLocks() *AppLocks {
	return &AppLocks{locks: map[string]chan struct{}{}}
}

// TryAcquire returns true and a release func if no deploy is in progress for
// the named app. Otherwise returns false.
func (l *AppLocks) TryAcquire(name string) (func(), bool) {
	l.mu.Lock()
	ch, ok := l.locks[name]
	if !ok {
		ch = make(chan struct{}, 1)
		l.locks[name] = ch
	}
	l.mu.Unlock()

	select {
	case ch <- struct{}{}:
		return func() { <-ch }, true
	default:
		return nil, false
	}
}
