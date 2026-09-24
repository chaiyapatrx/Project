package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"station-backend/internal/auth"
	"station-backend/internal/config"
)

func AuthMiddleware(cfg *config.Config, db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prefer the HttpOnly cookie (browser clients); fall back to the
		// Authorization: Bearer header (non-browser / legacy clients).
		tokenStr := ""
		if cookie, err := c.Cookie("auth_token"); err == nil && cookie != "" {
			tokenStr = cookie
		} else {
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
				c.Abort()
				return
			}
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization header format"})
				c.Abort()
				return
			}
			tokenStr = parts[1]
		}

		claims, err := auth.ValidateToken(tokenStr, cfg)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Read authorization from the database so role changes take effect immediately.
		var current struct {
			Role         string `db:"role"`
			IsActive     bool   `db:"is_active"`
			TokenVersion int    `db:"token_version"`
		}
		err = db.Get(&current, "SELECT role, is_active, token_version FROM users WHERE id = ?", claims.UserID)
		if err != nil || !current.IsActive || claims.TokenVersion != current.TokenVersion {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User account is inactive or revoked"})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", current.Role)
		c.Next()
	}
}

// CSRFMiddleware enforces the double-submit-cookie pattern for state-changing
// requests that authenticate via the auth cookie. Requests using a Bearer
// header (non-browser clients) are not subject to CSRF and are skipped.
func CSRFMiddleware() gin.HandlerFunc {
	safeMethods := map[string]bool{
		http.MethodGet:     true,
		http.MethodHead:    true,
		http.MethodOptions: true,
	}

	return func(c *gin.Context) {
		if safeMethods[c.Request.Method] {
			c.Next()
			return
		}

		// Only cookie-authenticated requests need CSRF protection.
		authCookie, err := c.Cookie("auth_token")
		if err != nil || authCookie == "" {
			c.Next()
			return
		}

		csrfCookie, err := c.Cookie("csrf_token")
		csrfHeader := c.GetHeader("X-CSRF-Token")
		if err != nil || csrfCookie == "" || csrfHeader == "" ||
			subtle.ConstantTimeCompare([]byte(csrfCookie), []byte(csrfHeader)) != 1 {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid or missing CSRF token"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func RequireRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			c.Abort()
			return
		}

		roleStr := userRole.(string)
		for _, r := range roles {
			if r == roleStr {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions for this resource"})
		c.Abort()
	}
}
