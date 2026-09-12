package auth

import (
	"net"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

// Unauthenticated auth endpoints run cost-12 bcrypt, so per-IP throttling
// keeps a single client from exhausting CPU or the 5-connection pool.
const (
	authRatePerSecond = 1
	authBurst         = 5
)

// ipLimiter is a per-IP token bucket shared by register + login.
// ponytail: unbounded IP map; add LRU/eviction if the app meets wide,
// hostile IP space. Personal scale does not.
type ipLimiter struct {
	mu    sync.Mutex
	ips   map[string]*rate.Limiter
	limit rate.Limit
	burst int
}

func newIPLimiter() *ipLimiter {
	return &ipLimiter{
		ips:   make(map[string]*rate.Limiter),
		limit: rate.Limit(authRatePerSecond),
		burst: authBurst,
	}
}

func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	lim, ok := l.ips[ip]
	if !ok {
		lim = rate.NewLimiter(l.limit, l.burst)
		l.ips[ip] = lim
	}
	return lim.Allow()
}

// Limit wraps next with the per-IP throttle. 429 on exhaustion.
func (h *Handler) Limit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.limiter == nil || !h.limiter.allow(clientIP(r)) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too_many_requests"})
			return
		}
		next(w, r)
	}
}

// clientIP prefers Fly's proxy header, which is not client-spoofable when
// the app is only reachable through Fly's edge. Falls back to the peer
// address for local/direct runs.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("Fly-Client-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
