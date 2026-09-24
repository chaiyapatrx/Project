package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"station-backend/internal/config"
)

const (
	authCookieName = "auth_token"
	csrfCookieName = "csrf_token"
	csrfHeaderName = "X-CSRF-Token"
)

// sameSiteMode maps a config string to Gin's http.SameSite value.
func sameSiteMode(mode string) http.SameSite {
	switch mode {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

// generateCSRFToken returns a random, URL-safe CSRF token.
func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// setAuthCookies sets the HttpOnly session cookie and the readable CSRF cookie.
// maxAge is in seconds.
func setAuthCookies(c *gin.Context, cfg *config.Config, token, csrf string, maxAge int) {
	c.SetSameSite(sameSiteMode(cfg.CookieSameSite))

	// HttpOnly auth cookie: not accessible to JavaScript, mitigating XSS token theft.
	c.SetCookie(authCookieName, token, maxAge, "/", cfg.CookieDomain, cfg.CookieSecure, true)

	// CSRF cookie: readable by JS so the SPA can echo it back in a header
	// (double-submit-cookie pattern). NOT HttpOnly by design.
	c.SetCookie(csrfCookieName, csrf, maxAge, "/", cfg.CookieDomain, cfg.CookieSecure, false)
}

// clearAuthCookies expires both cookies.
func clearAuthCookies(c *gin.Context, cfg *config.Config) {
	c.SetSameSite(sameSiteMode(cfg.CookieSameSite))
	c.SetCookie(authCookieName, "", -1, "/", cfg.CookieDomain, cfg.CookieSecure, true)
	c.SetCookie(csrfCookieName, "", -1, "/", cfg.CookieDomain, cfg.CookieSecure, false)
}
