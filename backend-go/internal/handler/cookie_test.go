package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"station-backend/internal/config"
)

func TestSessionCookieSecurityFlagsAndClearing(t *testing.T) {
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	cfg := &config.Config{CookieSecure: true, CookieSameSite: "strict"}
	setAuthCookies(context, cfg, "token-test", "csrf-test", 60)
	cookies := response.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("got %d cookies, want 2", len(cookies))
	}
	for _, cookie := range cookies {
		if !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Domain != "" || cookie.MaxAge != 60 {
			t.Fatalf("incorrect session cookie attributes for %s", cookie.Name)
		}
		if cookie.HttpOnly != (cookie.Name == authCookieName) {
			t.Fatalf("incorrect HttpOnly flag for %s", cookie.Name)
		}
	}
	response = httptest.NewRecorder()
	context, _ = gin.CreateTestContext(response)
	clearAuthCookies(context, cfg)
	for _, cookie := range response.Result().Cookies() {
		if cookie.MaxAge >= 0 || cookie.Value != "" || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
			t.Fatalf("cookie %s was not securely expired", cookie.Name)
		}
	}
}
