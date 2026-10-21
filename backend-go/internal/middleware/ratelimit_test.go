package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestAccountLoginLimitSharedAcrossLoginRoutesPerIP(t *testing.T) {
	previous, previousWide := accountLoginLimiter, accountWideLoginLimiter
	accountLoginLimiter = NewIPRateLimiter(10, time.Minute)
	accountWideLoginLimiter = NewIPRateLimiter(300, time.Minute)
	defer func() { accountLoginLimiter, accountWideLoginLimiter = previous, previousWide }()
	router := gin.New()
	for _, route := range []string{"/browser", "/station"} {
		router.POST(route, func(c *gin.Context) {
			if AccountLoginAllowed(c, 42) {
				c.Status(http.StatusNoContent)
			}
		})
	}
	for attempt := 0; attempt < 11; attempt++ {
		route := "/browser"
		if attempt%2 == 1 {
			route = "/station"
		}
		request := httptest.NewRequest(http.MethodPost, route, nil)
		request.RemoteAddr = "192.0.2.1:1234"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		want := http.StatusNoContent
		if attempt == 10 {
			want = http.StatusTooManyRequests
			if response.Header().Get("Retry-After") == "" {
				t.Fatal("account limit lacks Retry-After")
			}
		}
		if response.Code != want {
			t.Fatalf("attempt %d status = %d, want %d", attempt+1, response.Code, want)
		}
	}
	for _, other := range []struct {
		user int
		ip   string
	}{{43, "192.0.2.1"}, {42, "192.0.2.2"}} {
		request := httptest.NewRequest(http.MethodPost, "/browser", nil)
		request.RemoteAddr = other.ip + ":1234"
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = request
		if !AccountLoginAllowed(context, other.user) {
			t.Fatalf("guesses from 192.0.2.1 blocked user %d at %s", other.user, other.ip)
		}
	}
}

func TestAccountWideLoginCapStopsSpreadOutGuessing(t *testing.T) {
	previous, previousWide := accountLoginLimiter, accountWideLoginLimiter
	accountLoginLimiter = NewIPRateLimiter(10, time.Minute)
	accountWideLoginLimiter = NewIPRateLimiter(3, time.Minute)
	defer func() { accountLoginLimiter, accountWideLoginLimiter = previous, previousWide }()
	for attempt := 0; attempt < 4; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/browser", nil)
		request.RemoteAddr = fmt.Sprintf("192.0.2.%d:1234", attempt+1)
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = request
		if got := AccountLoginAllowed(context, 42); got != (attempt < 3) {
			t.Fatalf("attempt %d from a new IP: allowed=%v", attempt+1, got)
		}
	}
}

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
