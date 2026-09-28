package handler

import (
	"crypto/subtle"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"station-backend/internal/auth"
)

type stationLoginRequest struct {
	Mode       string `json:"mode"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	AccessCode string `json:"access_code"`
}

// StationLogin accepts regular account credentials on an unreserved station,
// or the active booking's code on a reserved station.
func (h *WSHandler) StationLogin(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req stationLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid login request"})
		return
	}
	if req.Mode != "account" && req.Mode != "access_code" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid login mode"})
		return
	}

	computerName := strings.TrimSpace(c.GetHeader("X-Computer-Name"))
	secret := c.GetHeader("X-Agent-Secret")
	hwid := strings.TrimSpace(c.GetHeader("X-Computer-HWID"))
	if computerName == "" || len(computerName) > 50 || len(secret) < 32 || len(secret) > 256 || hwid == "" || len(hwid) > 100 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Station authentication failed"})
		return
	}
	if req.Mode == "account" && (strings.TrimSpace(req.Username) == "" || len(req.Username) > 50 || len(req.Password) == 0 || len(req.Password) > 72) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username and password are required"})
		return
	}
	if req.Mode == "access_code" && !validAccessCode(req.AccessCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Access code must contain six digits"})
		return
	}

	tx, err := h.db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start login"})
		return
	}
	defer tx.Rollback()

	var computer struct {
		ID              int            `db:"id"`
		Name            string         `db:"name"`
		HWID            sql.NullString `db:"hwid"`
		AgentSecretHash sql.NullString `db:"agent_secret_hash"`
		Status          string         `db:"status"`
	}
	if err := tx.Get(&computer, "SELECT id, name, hwid, agent_secret_hash, status FROM computers WHERE name = ? AND is_active = 1 FOR UPDATE", computerName); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Station authentication failed"})
		return
	}
	if !computer.HWID.Valid || computer.HWID.String != hwid || !verifyStationSecret(secret, computer.AgentSecretHash, h.cfg.AgentSecret, h.cfg.AllowLegacyAgents) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Station authentication failed"})
		return
	}
	if allowed, retryAfter := h.stationLoginLimiter.Allow(computer.Name); !allowed {
		retrySeconds := int(retryAfter.Seconds())
		if retryAfter%time.Second != 0 {
			retrySeconds++
		}
		if retrySeconds < 1 {
			retrySeconds = 1
		}
		c.Header("Retry-After", strconv.Itoa(retrySeconds))
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many station login attempts; try again later"})
		return
	}
	if computer.Status == "maintenance" || computer.Status == "disabled" {
		c.JSON(http.StatusConflict, gin.H{"error": "This station is unavailable"})
		return
	}
	if !h.hub.IsAgentOnline(computer.Name) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Station agent is offline"})
		return
	}

	var booking struct {
		ID         int            `db:"id"`
		UserID     int            `db:"user_id"`
		AccessCode sql.NullString `db:"access_code"`
		EndTime    time.Time      `db:"end_time"`
	}
	bookingErr := tx.Get(&booking, `
		SELECT id, user_id, access_code, end_time FROM bookings
		WHERE computer_id = ? AND status = 'active'
		ORDER BY id DESC LIMIT 1 FOR UPDATE`, computer.ID)
	if bookingErr != nil && bookingErr != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check station reservation"})
		return
	}

	var userID int
	var username string
	var bookingID, usageLogID int
	var sessionEnd time.Time
	if bookingErr == nil {
		if req.Mode != "access_code" || !booking.AccessCode.Valid || subtle.ConstantTimeCompare([]byte(req.AccessCode), []byte(booking.AccessCode.String)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access code"})
			return
		}
		if !booking.EndTime.After(time.Now()) {
			c.JSON(http.StatusConflict, gin.H{"error": "This booking has expired"})
			return
		}
		if err := tx.Get(&username, "SELECT username FROM users WHERE id = ?", booking.UserID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load booking owner"})
			return
		}
		bookingID, userID, sessionEnd = booking.ID, booking.UserID, booking.EndTime
	} else {
		if req.Mode != "account" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
			return
		}
		var user struct {
			ID           int    `db:"id"`
			Username     string `db:"username"`
			PasswordHash string `db:"password_hash"`
		}
		if err := tx.Get(&user, "SELECT id, username, password_hash FROM users WHERE username = ? AND is_active = 1 FOR UPDATE", strings.TrimSpace(req.Username)); err == sql.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify station login"})
			return
		}
		if !auth.CheckPasswordHash(req.Password, user.PasswordHash) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
			return
		}
		userID, username = user.ID, user.Username

		var current struct {
			ID            int          `db:"id"`
			UserID        int          `db:"user_id"`
			SessionEndsAt sql.NullTime `db:"session_ends_at"`
		}
		usageErr := tx.Get(&current, `
			SELECT id, user_id, session_ends_at FROM usage_logs
			WHERE computer_id = ? AND booking_id IS NULL AND end_time IS NULL
			ORDER BY id DESC LIMIT 1 FOR UPDATE`, computer.ID)
		if usageErr != nil && usageErr != sql.ErrNoRows {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check current station session"})
			return
		}
		if usageErr == nil {
			if !current.SessionEndsAt.Valid || !current.SessionEndsAt.Time.After(time.Now()) {
				c.JSON(http.StatusConflict, gin.H{"error": "The current station session has expired"})
				return
			}
			if current.UserID != user.ID {
				c.JSON(http.StatusConflict, gin.H{"error": "This station is in use"})
				return
			}
			usageLogID, sessionEnd = current.ID, current.SessionEndsAt.Time
			if _, err := tx.Exec("UPDATE computers SET status = 'in_use' WHERE id = ? AND status = 'available'", computer.ID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restore station session"})
				return
			}
		} else {
			duration, err := readMaxSessionDuration(tx)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read session duration setting"})
				return
			}
			now := time.Now()
			sessionEnd = now.Add(time.Duration(duration) * time.Minute)
			result, err := tx.Exec(`
				INSERT INTO usage_logs (user_id, computer_id, booking_id, start_time, session_ends_at)
				VALUES (?, ?, NULL, ?, ?)`, user.ID, computer.ID, now, sessionEnd)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start station session"})
				return
			}
			id, err := result.LastInsertId()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start station session"})
				return
			}
			usageLogID = int(id)
			if _, err := tx.Exec("UPDATE computers SET status = 'in_use' WHERE id = ? AND status = 'available'", computer.ID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reserve station session"})
				return
			}
		}
	}

	if bookingID > 0 {
		if _, err := tx.Exec("UPDATE computers SET status = 'in_use' WHERE id = ?", computer.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to confirm station reservation"})
			return
		}
	}
	if !h.hub.SendCommandToAgent(computer.Name, "UNLOCK", map[string]interface{}{
		"user_id": userID, "username": username, "session_end": sessionEnd.Unix(),
	}) {
		// A failed write leaves station lock state uncertain; force the client to
		// lock its overlay when the WebSocket closes.
		h.hub.DisconnectAgent(computer.Name)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Station agent disconnected; try again"})
		return
	}
	// Keep Hub state inside the computer-row lock, so cancellation or a new
	// session cannot commit first and then be overwritten by this login.
	h.hub.Mu.Lock()
	if state := h.hub.Computers[computer.Name]; state != nil {
		state.Status = "in_use"
		state.CurrentBookingID = bookingID
		state.CurrentUsageLogID = usageLogID
		state.CurrentUserID = &userID
		state.CurrentUserName = &username
		state.SessionEndsAt = &sessionEnd
	}
	h.hub.Mu.Unlock()
	if err := tx.Commit(); err != nil {
		// Closing the connection makes the client lock its overlay if commit state is uncertain.
		h.hub.DisconnectAgent(computer.Name)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit station login"})
		return
	}

	h.hub.BroadcastStateChange(computer.Name)
	c.Set("user_id", userID)
	mode := "account"
	if bookingID > 0 {
		mode = "access_code"
	}
	recordAudit(c, h.db, "station_login", "computer", strconv.Itoa(computer.ID), "Station login succeeded using "+mode)
	c.JSON(http.StatusOK, gin.H{"message": "Station unlocked"})
}

func verifyStationSecret(secret string, storedHash sql.NullString, legacySecret string, allowLegacy bool) bool {
	if storedHash.Valid {
		return auth.VerifyAgentSecret(secret, storedHash.String)
	}
	return allowLegacy && len(legacySecret) >= 32 && len(legacySecret) <= 256 && subtle.ConstantTimeCompare([]byte(secret), []byte(legacySecret)) == 1
}

func validAccessCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, digit := range code {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}
