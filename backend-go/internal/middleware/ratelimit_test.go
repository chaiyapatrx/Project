package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestAccountLoginLimitSharedAcrossIPsAndLoginRoutes(t *testing.T) {
	previous := accountLoginLimiter
	accountLoginLimiter = NewIPRateLimiter(10, time.Minute)
	defer func() { accountLoginLimiter = previous }()
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
		request.RemoteAddr = fmt.Sprintf("192.0.2.%d:1234", attempt+1)
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
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	if !AccountLoginAllowed(context, 43) {
		t.Fatal("one account's attempts blocked a different account")
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
