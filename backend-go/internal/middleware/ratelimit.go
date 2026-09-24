package middleware

import (
	"container/list"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const maxRateLimitClients = 10000

type rateLimitEntry struct {
	ip        string
	startedAt time.Time
	requests  int
}

// IPRateLimiter uses bounded, per-process fixed windows to protect expensive
// public endpoints without allowing attacker-controlled IPs to grow memory
// without limit. A reverse proxy/WAF is still needed for distributed abuse.
type IPRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]*list.Element
	lru     *list.List
}

func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
	if limit < 1 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &IPRateLimiter{
		limit:   limit,
		window:  window,
		entries: make(map[string]*list.Element),
		lru:     list.New(),
	}
}

func (l *IPRateLimiter) Allow(ip string) (bool, time.Duration) {
	return l.allowAt(ip, time.Now())
}

func (l *IPRateLimiter) allowAt(ip string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if element := l.entries[ip]; element != nil {
		entry := element.Value.(*rateLimitEntry)
		if now.Sub(entry.startedAt) >= l.window || now.Before(entry.startedAt) {
			entry.startedAt = now
			entry.requests = 0
		}
		if entry.requests >= l.limit {
			return false, entry.startedAt.Add(l.window).Sub(now)
		}
		entry.requests++
		l.lru.MoveToFront(element)
		return true, 0
	}

	if l.lru.Len() >= maxRateLimitClients {
		oldest := l.lru.Back()
		entry := oldest.Value.(*rateLimitEntry)
		delete(l.entries, entry.ip)
		l.lru.Remove(oldest)
	}
	entry := &rateLimitEntry{ip: ip, startedAt: now, requests: 1}
	l.entries[ip] = l.lru.PushFront(entry)
	return true, 0
}

func RateLimitByIP(limit int, window time.Duration) gin.HandlerFunc {
	limiter := NewIPRateLimiter(limit, window)
	return func(c *gin.Context) {
		allowed, retryAfter := limiter.Allow(c.ClientIP())
		if !allowed {
			seconds := int(retryAfter.Seconds())
			if retryAfter%time.Second != 0 {
				seconds++
			}
			if seconds < 1 {
				seconds = 1
			}
			c.Header("Retry-After", strconv.Itoa(seconds))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests; try again later"})
			return
		}
		c.Next()
	}
}
