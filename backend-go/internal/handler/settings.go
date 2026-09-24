package handler

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"station-backend/internal/models"
)

type SettingsHandler struct {
	db *sqlx.DB
}

func NewSettingsHandler(db *sqlx.DB) *SettingsHandler {
	return &SettingsHandler{db: db}
}

func (h *SettingsHandler) GetSettings(c *gin.Context) {
	var settings []models.SystemSetting
	err := h.db.Select(&settings, "SELECT * FROM system_settings")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
		return
	}

	result := make(map[string]string)
	for _, s := range settings {
		if allowedSettingKeys[s.SettingKey] {
			result[s.SettingKey] = s.SettingValue
		}
	}

	c.JSON(http.StatusOK, result)
}

// allowedSettingKeys restricts which configuration keys clients may write,
// preventing arbitrary key/value injection into system_settings.
var allowedSettingKeys = map[string]bool{
	"session_duration": true,
	"maintenance_mode": true,
}

func (h *SettingsHandler) UpdateSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Reject the whole request if any key is not on the allow-list.
	for key := range req {
		if !allowedSettingKeys[key] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown setting key: " + key})
			return
		}
	}
	if value, ok := req["session_duration"]; ok {
		duration, err := strconv.Atoi(value)
		if err != nil || duration < 30 || duration > 240 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session_duration must be between 30 and 240 minutes"})
			return
		}
	}
	if value, ok := req["maintenance_mode"]; ok {
		if _, err := strconv.ParseBool(value); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "maintenance_mode must be true or false"})
			return
		}
	}

	tx, err := h.db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start settings update"})
		return
	}
	defer tx.Rollback()
	for key, val := range req {
		if _, err := tx.Exec(`
			INSERT INTO system_settings (setting_key, setting_value)
			VALUES (?, ?)
			ON DUPLICATE KEY UPDATE setting_value = VALUES(setting_value)`,
			key, val,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update settings"})
			return
		}
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit settings update"})
		return
	}
	keys := make([]string, 0, len(req))
	for key := range req {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	recordAudit(c, h.db, "settings_updated", "system_settings", "", "Updated keys: "+strings.Join(keys, ", "))

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated successfully"})
}

func (h *SettingsHandler) GetUsageHistory(c *gin.Context) {
	logs := make([]models.UsageLog, 0)
	filter := ""
	switch c.Query("period") {
	case "", "all":
	case "day":
		filter = "WHERE l.start_time >= CURDATE()"
	case "week":
		filter = "WHERE l.start_time >= DATE_SUB(CURDATE(), INTERVAL WEEKDAY(CURDATE()) DAY)"
	case "month":
		filter = "WHERE YEAR(l.start_time) = YEAR(CURDATE()) AND MONTH(l.start_time) = MONTH(CURDATE())"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "period must be all, day, week, or month"})
		return
	}
	query := `
		SELECT l.*, u.full_name as user_name, COALESCE(u.department, '-') as department, c.name as computer_name
		FROM usage_logs l
		JOIN users u ON l.user_id = u.id
		JOIN computers c ON l.computer_id = c.id
		` + filter + `
		ORDER BY l.created_at DESC LIMIT 100`

	err := h.db.Select(&logs, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch usage history"})
		return
	}

	c.JSON(http.StatusOK, logs)
}

// GetAnalyticsSummary provides stats for both Admin and Exec Dashboards
func (h *SettingsHandler) GetAnalyticsSummary(c *gin.Context) {
	var totalUsers int
	var totalComputers int
	var activeBookings int
	var todaysBookings int

	queries := []struct {
		dest  interface{}
		query string
	}{
		{&totalUsers, "SELECT COUNT(*) FROM users WHERE is_active = 1"},
		{&totalComputers, "SELECT COUNT(*) FROM computers WHERE is_active = 1"},
		{&activeBookings, "SELECT COUNT(*) FROM bookings WHERE status = 'active'"},
		{&todaysBookings, "SELECT COUNT(*) FROM bookings WHERE DATE(created_at) = CURDATE()"},
	}
	for _, q := range queries {
		if err := h.db.Get(q.dest, q.query); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch analytics"})
			return
		}
	}

	var totalSessions int
	var avgDuration float64
	var uniqueUsers int

	queries = []struct {
		dest  interface{}
		query string
	}{
		{&totalSessions, "SELECT COUNT(*) FROM usage_logs"},
		{&avgDuration, "SELECT COALESCE(AVG(duration_minutes), 0) FROM usage_logs"},
		{&uniqueUsers, "SELECT COUNT(DISTINCT user_id) FROM usage_logs"},
	}
	for _, q := range queries {
		if err := h.db.Get(q.dest, q.query); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch analytics"})
			return
		}
	}

	// Calculate real student, staff, and other counts
	var studentCount, staffCount, otherCount int
	queries = []struct {
		dest  interface{}
		query string
	}{
		{&studentCount, "SELECT COUNT(*) FROM users WHERE role = 'student' AND is_active = 1"},
		{&staffCount, "SELECT COUNT(*) FROM users WHERE role = 'staff' AND is_active = 1"},
		{&otherCount, "SELECT COUNT(*) FROM users WHERE role NOT IN ('student', 'staff') AND is_active = 1"},
	}
	for _, q := range queries {
		if err := h.db.Get(q.dest, q.query); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch analytics"})
			return
		}
	}
	var activeComputers int
	if err := h.db.Get(&activeComputers, "SELECT COUNT(*) FROM computers WHERE is_active = 1 AND status = 'in_use'"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch analytics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		// Keys for AdminOverview.jsx
		"total_users":     totalUsers,
		"total_computers": totalComputers,
		"active_bookings": activeBookings,
		"todays_bookings": todaysBookings,

		// Keys for Executive dashboard
		"total_sessions":   totalSessions,
		"avg_duration_min": int(avgDuration),
		"unique_users":     uniqueUsers,
		"active_computers": activeComputers,
		"student_users":    studentCount,
		"staff_users":      staffCount,
		"other_users":      otherCount,
	})
}
