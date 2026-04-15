package middleware

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type rateLimitStore struct {
	mu       sync.Mutex
	limiters map[string]*ipLimiter
	rps      rate.Limit
	burst    int
}

func (s *rateLimitStore) get(ip string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, ok := s.limiters[ip]; ok {
		entry.lastSeen = time.Now()
		return entry.limiter
	}

	l := rate.NewLimiter(s.rps, s.burst)
	s.limiters[ip] = &ipLimiter{limiter: l, lastSeen: time.Now()}
	return l
}

func (s *rateLimitStore) cleanup() {
	for {
		time.Sleep(3 * time.Minute)
		s.mu.Lock()
		cutoff := time.Now().Add(-5 * time.Minute)
		for ip, entry := range s.limiters {
			if entry.lastSeen.Before(cutoff) {
				delete(s.limiters, ip)
			}
		}
		s.mu.Unlock()
	}
}

func RateLimit(rps float64, burst int) func(http.Handler) http.Handler {
	store := &rateLimitStore{
		limiters: make(map[string]*ipLimiter),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
	go store.cleanup()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}

			limiter := store.get(ip)
			if !limiter.Allow() {
				w.Header().Set("Retry-After", strconv.Itoa(1))
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
