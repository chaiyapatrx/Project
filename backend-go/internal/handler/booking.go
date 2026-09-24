package handler

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"station-backend/internal/hub"
	"station-backend/internal/models"
)

type BookingHandler struct {
	db  *sqlx.DB
	hub *hub.Hub
}

func NewBookingHandler(db *sqlx.DB, hub *hub.Hub) *BookingHandler {
	return &BookingHandler{db: db, hub: hub}
}

func (h *BookingHandler) maxSessionDuration() (int, error) {
	const fallback = 120
	var value string
	err := h.db.Get(&value, "SELECT setting_value FROM system_settings WHERE setting_key = 'session_duration'")
	if err == sql.ErrNoRows {
		return fallback, nil
	}
	if err != nil {
		return 0, err
	}
	duration, err := strconv.Atoi(value)
	if err != nil || duration < 30 || duration > 240 {
		return 0, fmt.Errorf("invalid session_duration setting")
	}
	return duration, nil
}

// generateAccessCode returns a cryptographically secure 6-digit access code.
func generateAccessCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

type CreateBookingRequest struct {
	ComputerID      int `json:"computer_id" binding:"required"`
	DurationMinutes int `json:"duration_minutes"`
}

func (h *BookingHandler) CreateBooking(c *gin.Context) {
	userID := c.GetInt("user_id")
	username := c.GetString("username")

	var req CreateBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	maxDuration, err := h.maxSessionDuration()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read session duration setting"})
		return
	}
	duration := req.DurationMinutes
	if duration == 0 {
		duration = maxDuration
	}
	if duration < 1 || duration > maxDuration {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Duration must be between 1 and %d minutes", maxDuration)})
		return
	}
	userRole := c.GetString("role")
	var maintenanceValue string
	err = h.db.Get(&maintenanceValue, "SELECT setting_value FROM system_settings WHERE setting_key = 'maintenance_mode'")
	if err != nil && err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read maintenance setting"})
		return
	}
	if err == nil {
		maintenance, parseErr := strconv.ParseBool(maintenanceValue)
		if parseErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid maintenance setting"})
			return
		}
		if maintenance && userRole != "admin" && userRole != "staff" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Bookings are disabled during maintenance"})
			return
		}
	}

	// Concurrency-safe booking transaction: Lock computer record for update to prevent TOCTOU double-booking
	tx, err := h.db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start database transaction"})
		return
	}
	defer tx.Rollback()
	var lockedUserID int
	if err := tx.Get(&lockedUserID, "SELECT id FROM users WHERE id = ? AND is_active = 1 FOR UPDATE", userID); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User account is unavailable"})
		return
	}

	// Verify computer is available with Row Lock
	var comp models.Computer
	err = tx.Get(&comp, `SELECT id, name, hwid, ip_address, mac_address, status, is_active, created_at, updated_at
		FROM computers WHERE id = ? AND is_active = 1 FOR UPDATE`, req.ComputerID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Computer not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error checking computer"})
		return
	}

	// Strictly prevent booking computers in maintenance or disabled status
	if comp.Status != "available" {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Computer is currently '%s' and unavailable for booking", comp.Status)})
		return
	}
	if !h.hub.IsAgentOnline(comp.Name) {
		c.JSON(http.StatusConflict, gin.H{"error": "Computer agent is offline"})
		return
	}

	// Check if user already has an active booking (limit 1 active station per user in production)
	if userRole != "admin" && userRole != "staff" {
		var userActiveBookings int
		if err := tx.Get(&userActiveBookings, "SELECT COUNT(*) FROM bookings WHERE user_id = ? AND status IN ('pending', 'active')", userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing bookings"})
			return
		}
		if userActiveBookings > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "You already have an active computer booking"})
			return
		}
	}

	// Check if already booked actively within transaction
	var activeCount int
	if err := tx.Get(&activeCount, "SELECT COUNT(*) FROM bookings WHERE computer_id = ? AND status IN ('pending', 'active')", req.ComputerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check computer bookings"})
		return
	}
	if activeCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Computer is already booked or active"})
		return
	}

	startTime := time.Now()
	endTime := startTime.Add(time.Duration(duration) * time.Minute)
	accessCode, err := generateAccessCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate access code"})
		return
	}

	res, err := tx.Exec(`
		INSERT INTO bookings (user_id, computer_id, start_time, end_time, access_code, status)
		VALUES (?, ?, ?, ?, ?, 'active')`,
		userID, req.ComputerID, startTime, endTime, accessCode,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create booking"})
		return
	}

	if _, err := tx.Exec("UPDATE computers SET status = 'in_use' WHERE id = ?", req.ComputerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reserve computer"})
		return
	}
	bookingID, err := res.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read booking ID"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit booking"})
		return
	}

	// Send instant UNLOCK command to the client agent
	delivered := h.hub.SendCommandToAgent(comp.Name, "UNLOCK", map[string]interface{}{
		"user_id":     userID,
		"username":    username,
		"session_end": endTime.Unix(),
	})
	if !delivered {
		rollback, rollbackErr := h.db.Beginx()
		if rollbackErr == nil {
			_, rollbackErr = rollback.Exec("UPDATE bookings SET status = 'cancelled' WHERE id = ? AND status = 'active'", bookingID)
			if rollbackErr == nil {
				_, rollbackErr = rollback.Exec("UPDATE computers SET status = 'available' WHERE id = ? AND status = 'in_use'", req.ComputerID)
			}
			if rollbackErr == nil {
				rollbackErr = rollback.Commit()
			} else {
				_ = rollback.Rollback()
			}
		}
		if rollbackErr != nil {
			log.Printf("[Booking] Failed to compensate booking %d after agent dispatch failure: %v", bookingID, rollbackErr)
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Computer agent disconnected; booking was not started"})
		return
	}

	// Update in-memory state only after the agent accepted the command.
	h.hub.Mu.Lock()
	if state, exists := h.hub.Computers[comp.Name]; exists {
		state.Status = "in_use"
		state.CurrentUserID = &userID
		state.CurrentUserName = &username
		state.SessionEndsAt = &endTime
	}
	h.hub.Mu.Unlock()
	h.hub.BroadcastStateChange(comp.Name)
	recordAudit(c, h.db, "booking_created", "booking", strconv.FormatInt(bookingID, 10), fmt.Sprintf("Booking started on station %d", req.ComputerID))

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Booking created successfully",
		"booking_id":  bookingID,
		"access_code": accessCode,
		"start_time":  startTime,
		"end_time":    endTime,
	})
}

func (h *BookingHandler) GetMyBookings(c *gin.Context) {
	userID := c.GetInt("user_id")

	bookings := make([]models.Booking, 0)
	query := `
		SELECT b.*, u.full_name as user_name, u.username as user_username, c.name as computer_name
		FROM bookings b
		JOIN users u ON b.user_id = u.id
		JOIN computers c ON b.computer_id = c.id
		WHERE b.user_id = ?
		ORDER BY b.created_at DESC LIMIT 20`

	if err := h.db.Select(&bookings, query, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bookings"})
		return
	}
	c.JSON(http.StatusOK, bookings)
}

func (h *BookingHandler) ListAllBookings(c *gin.Context) {
	bookings := make([]models.Booking, 0)
	query := `
		SELECT b.*, u.full_name as user_name, u.username as user_username, c.name as computer_name
		FROM bookings b
		JOIN users u ON b.user_id = u.id
		JOIN computers c ON b.computer_id = c.id
		ORDER BY b.created_at DESC LIMIT 100`

	if err := h.db.Select(&bookings, query); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bookings"})
		return
	}
	c.JSON(http.StatusOK, bookings)
}

func (h *BookingHandler) CancelBooking(c *gin.Context) {
	idStr := c.Param("id")
	bookingID, err := strconv.Atoi(idStr)
	if err != nil || bookingID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid booking ID"})
		return
	}
	userID := c.GetInt("user_id")
	userRole := c.GetString("role")

	var initial struct {
		UserID     int `db:"user_id"`
		ComputerID int `db:"computer_id"`
	}
	err = h.db.Get(&initial, "SELECT user_id, computer_id FROM bookings WHERE id = ?", bookingID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
		return
	}

	tx, err := h.db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database transaction error"})
		return
	}
	defer tx.Rollback()
	var lockedID int
	if err := tx.Get(&lockedID, "SELECT id FROM users WHERE id = ? FOR UPDATE", initial.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to lock booking owner"})
		return
	}
	if err := tx.Get(&lockedID, "SELECT id FROM computers WHERE id = ? FOR UPDATE", initial.ComputerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to lock computer"})
		return
	}
	var booking models.Booking
	if err := tx.Get(&booking, "SELECT * FROM bookings WHERE id = ? FOR UPDATE", bookingID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
		return
	}
	if userRole != "admin" && userRole != "staff" && booking.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}
	if booking.Status != "active" && booking.Status != "pending" {
		c.JSON(http.StatusConflict, gin.H{"error": "Booking is already completed or cancelled"})
		return
	}

	result, err := tx.Exec("UPDATE bookings SET status = 'cancelled' WHERE id = ? AND status IN ('active', 'pending')", bookingID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update booking status"})
		return
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		c.JSON(http.StatusConflict, gin.H{"error": "Booking was already changed"})
		return
	}

	var comp struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}
	err = tx.Get(&comp, "SELECT id, name FROM computers WHERE id = ?", booking.ComputerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find associated computer"})
		return
	}

	now := time.Now()
	if booking.Status == "active" {
		if _, err := tx.Exec("UPDATE computers SET status = 'available' WHERE id = ? AND status = 'in_use'", booking.ComputerID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to release computer"})
			return
		}
		duration := int(now.Sub(booking.StartTime).Minutes())
		if duration < 1 {
			duration = 1
		}
		if _, err := tx.Exec(`
			INSERT INTO usage_logs (user_id, computer_id, booking_id, start_time, end_time, duration_minutes, termination_reason)
			VALUES (?, ?, ?, ?, ?, ?, 'cancelled')`,
			booking.UserID, booking.ComputerID, booking.ID, booking.StartTime, now, duration,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record usage log"})
			return
		}
		// Keep the computer row locked until LOCK is sent, preventing a new
		// booking from dispatching UNLOCK ahead of this cancellation command.
		h.hub.SendCommandToAgent(comp.Name, "LOCK", map[string]interface{}{"reason": "booking_cancelled"})
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit cancellation"})
		return
	}

	// Reset in-memory state after the database state has committed.
	h.hub.Mu.Lock()
	if state, exists := h.hub.Computers[comp.Name]; exists {
		matchesBooking := state.CurrentUserID != nil && *state.CurrentUserID == booking.UserID
		if state.CurrentUserID == nil && state.SessionEndsAt != nil && state.SessionEndsAt.Equal(booking.EndTime) {
			matchesBooking = true
		}
		if booking.Status == "active" && matchesBooking {
			state.Status = "available"
			state.CurrentUserID = nil
			state.CurrentUserName = nil
			state.SessionEndsAt = nil
		}
	}
	h.hub.Mu.Unlock()
	h.hub.BroadcastStateChange(comp.Name)
	recordAudit(c, h.db, "booking_cancelled", "booking", strconv.Itoa(bookingID), "Booking ended or cancelled")

	c.JSON(http.StatusOK, gin.H{"message": "Booking ended/cancelled successfully"})
}

// ExtendBooking adds additional minutes to an active booking session (e.g. +30m, +60m)
func (h *BookingHandler) ExtendBooking(c *gin.Context) {
	idStr := c.Param("id")
	bookingID, err := strconv.Atoi(idStr)
	if err != nil || bookingID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid booking ID"})
		return
	}
	userID := c.GetInt("user_id")
	userRole := c.GetString("role")
	maxDuration, err := h.maxSessionDuration()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read session duration setting"})
		return
	}

	var req struct {
		AddMinutes int `json:"add_minutes" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil || req.AddMinutes <= 0 || req.AddMinutes > 180 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Extension minutes must be between 1 and 180"})
		return
	}

	var initial struct {
		UserID     int `db:"user_id"`
		ComputerID int `db:"computer_id"`
	}
	if err := h.db.Get(&initial, "SELECT user_id, computer_id FROM bookings WHERE id = ? AND status = 'active'", bookingID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Active booking not found"})
		return
	}
	tx, err := h.db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database transaction error"})
		return
	}
	defer tx.Rollback()
	var lockedID int
	if err := tx.Get(&lockedID, "SELECT id FROM users WHERE id = ? FOR UPDATE", initial.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to lock booking owner"})
		return
	}
	if err := tx.Get(&lockedID, "SELECT id FROM computers WHERE id = ? FOR UPDATE", initial.ComputerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to lock computer"})
		return
	}
	var booking models.Booking
	if err := tx.Get(&booking, "SELECT * FROM bookings WHERE id = ? AND status = 'active' FOR UPDATE", bookingID); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Active booking was already changed"})
		return
	}

	if userRole != "admin" && userRole != "staff" && booking.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	newEndTime := booking.EndTime.Add(time.Duration(req.AddMinutes) * time.Minute)

	if newEndTime.Sub(booking.StartTime) > time.Duration(maxDuration)*time.Minute {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Cannot extend: session would exceed the configured maximum of %d minutes", maxDuration)})
		return
	}

	if _, err := tx.Exec("UPDATE bookings SET end_time = ? WHERE id = ? AND status = 'active'", newEndTime, bookingID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to extend booking"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit booking extension"})
		return
	}

	var comp models.Computer
	if err := h.db.Get(&comp, "SELECT id, name FROM computers WHERE id = ?", booking.ComputerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find associated computer"})
		return
	}

	h.hub.Mu.Lock()
	if state, exists := h.hub.Computers[comp.Name]; exists {
		if state.CurrentUserID != nil && *state.CurrentUserID == booking.UserID {
			state.SessionEndsAt = &newEndTime
		}
	}
	h.hub.Mu.Unlock()
	h.hub.BroadcastStateChange(comp.Name)
	recordAudit(c, h.db, "booking_extended", "booking", strconv.Itoa(bookingID), fmt.Sprintf("Session extended by %d minutes", req.AddMinutes))

	c.JSON(http.StatusOK, gin.H{
		"message":      fmt.Sprintf("Session extended by %d minutes", req.AddMinutes),
		"new_end_time": newEndTime,
	})
}
