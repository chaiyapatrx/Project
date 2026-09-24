package handler

import (
	"log"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func recordAudit(c *gin.Context, db *sqlx.DB, action, targetType, targetID, details string) {
	var userID any
	if value, exists := c.Get("user_id"); exists {
		if id, ok := value.(int); ok {
			userID = id
		}
	}
	var target any
	if targetID != "" {
		target = targetID
	}
	if _, err := db.Exec(`
		INSERT INTO audit_logs (user_id, action, target_type, target_id, ip_address, details)
		VALUES (?, ?, ?, ?, ?, ?)`, userID, action, targetType, target, c.ClientIP(), details); err != nil {
		log.Printf("[Audit] Failed to record action %q for target %s: %v", action, targetType, err)
	}
}

func auditID(id int64) string {
	return strconv.FormatInt(id, 10)
}
