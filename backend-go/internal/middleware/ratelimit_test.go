package middleware

import (
	"testing"
	"time"
)

func TestIPRateLimiterWindowAndIsolation(t *testing.T) {
	limiter := NewIPRateLimiter(2, time.Minute)
	start := time.Now()
	for range 2 {
		if allowed, _ := limiter.allowAt("192.0.2.1", start); !allowed {
			t.Fatal("request within limit was rejected")
		}
	}
	if allowed, retry := limiter.allowAt("192.0.2.1", start); allowed || retry <= 0 {
		t.Fatal("request over limit was not rejected with a retry delay")
	}
	if allowed, _ := limiter.allowAt("192.0.2.2", start); !allowed {
		t.Fatal("one client's limit affected another client")
	}
	if allowed, _ := limiter.allowAt("192.0.2.1", start.Add(time.Minute)); !allowed {
		t.Fatal("request after the window was rejected")
	}
}
