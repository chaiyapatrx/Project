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

type settingsGetter interface {
	Get(dest interface{}, query string, args ...interface{}) error
}

func readMaxSessionDuration(db settingsGetter) (int, error) {
	const fallback = 120
	var value string
	err := db.Get(&value, "SELECT setting_value FROM system_settings WHERE setting_key = 'session_duration'")
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

func (h *BookingHandler) maxSessionDuration() (int, error) {
	return readMaxSessionDuration(h.db)
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
	var lockedUserID int
	if err := tx.Get(&lockedUserID, "SELECT id FROM users WHERE id = ? AND is_active = 1 FOR UPDATE", userID); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User account is unavailable"})
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
	var walkInSessions int
	if err := tx.Get(&walkInSessions, "SELECT COUNT(*) FROM usage_logs WHERE computer_id = ? AND booking_id IS NULL AND end_time IS NULL", req.ComputerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check current station session"})
		return
	}
	if walkInSessions > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Computer is currently in use"})
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

	// Reserve the station, but keep the overlay locked until its booking code is verified.
	delivered := h.hub.SendCommandToAgent(comp.Name, "LOCK", map[string]interface{}{
		"reason":    "booking_requires_access_code",
		"auth_mode": "access_code",
	})
	if !delivered {
		if !h.hub.SendCommandToAgent(comp.Name, "LOCK", map[string]interface{}{"reason": "booking_dispatch_failed", "auth_mode": "account"}) {
			h.hub.DisconnectAgent(comp.Name)
		}
		if err := tx.Rollback(); err != nil {
			log.Printf("[Booking] Failed to roll back booking %d after agent dispatch failure", bookingID)
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Computer agent disconnected; booking was not started"})
		return
	}

	// Keep Hub state inside the row-lock window and identify it by booking ID.
	h.hub.Mu.Lock()
	if state, exists := h.hub.Computers[comp.Name]; exists {
		state.Status = "in_use"
		state.CurrentBookingID = int(bookingID)
		state.CurrentUsageLogID = 0
		state.CurrentUserID = &userID
		state.CurrentUserName = &username
		state.SessionEndsAt = &endTime
	}
	h.hub.Mu.Unlock()
	if err := tx.Commit(); err != nil {
		// Verify the ambiguous commit while preserving computer-row ordering.
		recovery, recoveryErr := h.reconcileBookingCommit(bookingID, req.ComputerID)
		if recoveryErr != nil {
			// Leave ID-tagged state for the worker if DB reconciliation is unavailable.
			h.hub.DisconnectAgent(comp.Name)
			log.Printf("[Booking] Could not reconcile booking %d after commit error", bookingID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit booking"})
			return
		}
		if recovery != bookingCommitPersisted {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit booking"})
			return
		}
	}
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

type bookingCommitRecovery int

const (
	bookingCommitUnknown bookingCommitRecovery = iota
	bookingCommitPersisted
	bookingCommitNoActive
	bookingCommitOtherActive
)

func classifyBookingCommitRecovery(bookingStatus, computerStatus string, activeBookings int) bookingCommitRecovery {
	if bookingStatus == "active" && computerStatus == "in_use" {
		return bookingCommitPersisted
	}
	if activeBookings > 0 {
		return bookingCommitOtherActive
	}
	return bookingCommitNoActive
}

func cancellationCommitPersisted(bookingStatus string) bool {
	return bookingStatus == "cancelled"
}

func cancelCommitRequiresDisconnect(bookingStatus string) bool {
	return bookingStatus == "active"
}

func canClearCancelledBookingHub(bookingStatus, computerStatus string, activeBookings, activeUsage int) bool {
	return cancellationCommitPersisted(bookingStatus) && computerStatus == "available" && activeBookings == 0 && activeUsage == 0
}

func (h *BookingHandler) reconcileBookingCommit(bookingID int64, computerID int) (bookingCommitRecovery, error) {
	tx, err := h.db.Beginx()
	if err != nil {
		return bookingCommitUnknown, err
	}
	defer tx.Rollback()

	var computer struct {
		Name   string `db:"name"`
		Status string `db:"status"`
	}
	if err := tx.Get(&computer, "SELECT name, status FROM computers WHERE id = ? FOR UPDATE", computerID); err != nil {
		return bookingCommitUnknown, err
	}
	var bookingStatus string
	bookingErr := tx.Get(&bookingStatus, "SELECT status FROM bookings WHERE id = ? AND computer_id = ? FOR UPDATE", bookingID, computerID)
	if bookingErr != nil && bookingErr != sql.ErrNoRows {
		return bookingCommitUnknown, bookingErr
	}
	var activeBookings int
	if err := tx.Get(&activeBookings, `SELECT
		(SELECT COUNT(*) FROM bookings WHERE computer_id = ? AND status IN ('pending', 'active')) +
		(SELECT COUNT(*) FROM usage_logs WHERE computer_id = ? AND booking_id IS NULL AND end_time IS NULL)`, computerID, computerID); err != nil {
		return bookingCommitUnknown, err
	}
	recovery := classifyBookingCommitRecovery(bookingStatus, computer.Status, activeBookings)
	if recovery == bookingCommitPersisted || recovery == bookingCommitOtherActive {
		if err := tx.Rollback(); err != nil {
			log.Printf("[Booking] Failed to release commit verification lock for booking %d", bookingID)
		}
		return recovery, nil
	}

	if computer.Status == "in_use" {
		if _, err := tx.Exec("UPDATE computers SET status = 'available' WHERE id = ? AND status = 'in_use'", computerID); err != nil {
			return bookingCommitUnknown, err
		}
	}
	lockDelivered := h.hub.SendCommandToAgent(computer.Name, "LOCK", map[string]interface{}{"reason": "booking_commit_failed", "auth_mode": "account"})
	var previous hubBookingSnapshot
	clearedHub := false
	h.hub.Mu.Lock()
	if state, exists := h.hub.Computers[computer.Name]; exists && hub.MatchesBookingID(state.CurrentBookingID, int(bookingID)) {
		previous = hubBookingSnapshot{
			status:        state.Status,
			bookingID:     state.CurrentBookingID,
			currentUserID: state.CurrentUserID,
			currentUser:   state.CurrentUserName,
			sessionEndsAt: state.SessionEndsAt,
		}
		state.Status = computer.Status
		if state.Status == "in_use" {
			state.Status = "available"
		}
		clearHubBooking(state)
		clearedHub = true
	}
	h.hub.Mu.Unlock()
	if !lockDelivered {
		h.hub.DisconnectAgent(computer.Name)
	}
	if err := tx.Commit(); err != nil {
		if clearedHub {
			h.restoreHubBooking(computer.Name, previous)
		}
		return bookingCommitUnknown, err
	}
	h.hub.BroadcastStateChange(computer.Name)
	return bookingCommitNoActive, nil
}

type hubBookingSnapshot struct {
	status        string
	bookingID     int
	currentUserID *int
	currentUser   *string
	sessionEndsAt *time.Time
}

func clearHubBooking(state *hub.ComputerRuntimeState) {
	state.CurrentBookingID = 0
	state.CurrentUsageLogID = 0
	state.CurrentUserID = nil
	state.CurrentUserName = nil
	state.SessionEndsAt = nil
}

func canRestoreClearedHubBooking(state *hub.ComputerRuntimeState) bool {
	return state.Status == "available" && state.CurrentBookingID == 0 &&
		state.CurrentUsageLogID == 0 &&
		state.CurrentUserID == nil && state.CurrentUserName == nil && state.SessionEndsAt == nil
}

func (h *BookingHandler) restoreHubBooking(computerName string, previous hubBookingSnapshot) {
	h.hub.Mu.Lock()
	if state := h.hub.Computers[computerName]; state != nil && canRestoreClearedHubBooking(state) {
		state.Status = previous.status
		state.CurrentBookingID = previous.bookingID
		state.CurrentUserID = previous.currentUserID
		state.CurrentUserName = previous.currentUser
		state.SessionEndsAt = previous.sessionEndsAt
	}
	h.hub.Mu.Unlock()
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
	if err := tx.Get(&lockedID, "SELECT id FROM computers WHERE id = ? FOR UPDATE", initial.ComputerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to lock computer"})
		return
	}
	if err := tx.Get(&lockedID, "SELECT id FROM users WHERE id = ? FOR UPDATE", initial.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to lock booking owner"})
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
		// If delivery fails, disconnect the agent so its client locks on WS loss.
		if !h.hub.SendCommandToAgent(comp.Name, "LOCK", map[string]interface{}{"reason": "booking_cancelled", "auth_mode": "account"}) {
			h.hub.DisconnectAgent(comp.Name)
		}
	}

	clearCancelledHub := true
	if err := tx.Commit(); err != nil {
		// A failed COMMIT has an ambiguous result. The pre-commit LOCK used
		// account auth, so disconnect immediately; on reconnect AgentWS derives
		// the correct lock mode from the persisted booking state.
		if cancelCommitRequiresDisconnect(booking.Status) {
			h.hub.DisconnectAgent(comp.Name)
		}
		persisted, safeToClearHub, verifyErr := h.reconcileCancellationCommit(booking.ID, comp.ID, comp.Name)
		if verifyErr != nil {
			log.Printf("[Booking] Failed to verify cancellation outcome for booking %d", booking.ID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit cancellation"})
			return
		}
		if !persisted {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit cancellation"})
			return
		}
		clearCancelledHub = safeToClearHub
	}

	// Reset in-memory state after the database state has committed.
	if clearCancelledHub {
		h.hub.Mu.Lock()
		if state, exists := h.hub.Computers[comp.Name]; exists {
			if booking.Status == "active" && hub.MatchesBookingID(state.CurrentBookingID, booking.ID) {
				state.Status = "available"
				state.CurrentBookingID = 0
				state.CurrentUsageLogID = 0
				state.CurrentUserID = nil
				state.CurrentUserName = nil
				state.SessionEndsAt = nil
			}
		}
		h.hub.Mu.Unlock()
	}
	h.hub.BroadcastStateChange(comp.Name)
	recordAudit(c, h.db, "booking_cancelled", "booking", strconv.Itoa(bookingID), "Booking ended or cancelled")

	c.JSON(http.StatusOK, gin.H{"message": "Booking ended/cancelled successfully"})
}

func (h *BookingHandler) reconcileCancellationCommit(bookingID, computerID int, computerName string) (bool, bool, error) {
	tx, err := h.db.Beginx()
	if err != nil {
		return false, false, err
	}
	defer tx.Rollback()

	var computerStatus string
	if err := tx.Get(&computerStatus, "SELECT status FROM computers WHERE id = ? FOR UPDATE", computerID); err != nil {
		return false, false, err
	}
	var bookingStatus string
	if err := tx.Get(&bookingStatus, "SELECT status FROM bookings WHERE id = ? AND computer_id = ? FOR UPDATE", bookingID, computerID); err != nil {
		return false, false, err
	}
	persisted := cancellationCommitPersisted(bookingStatus)
	if !persisted {
		return false, false, nil
	}
	var activeBookings, activeUsage int
	if err := tx.Get(&activeBookings, "SELECT COUNT(*) FROM bookings WHERE computer_id = ? AND status = 'active'", computerID); err != nil {
		return false, false, err
	}
	if err := tx.Get(&activeUsage, "SELECT COUNT(*) FROM usage_logs WHERE computer_id = ? AND booking_id IS NULL AND end_time IS NULL", computerID); err != nil {
		return false, false, err
	}
	safeToClearHub := canClearCancelledBookingHub(bookingStatus, computerStatus, activeBookings, activeUsage)
	if safeToClearHub {
		h.hub.Mu.Lock()
		if state := h.hub.Computers[computerName]; state != nil && hub.MatchesBookingID(state.CurrentBookingID, bookingID) {
			state.Status = computerStatus
			clearHubBooking(state)
		}
		h.hub.Mu.Unlock()
	}
	return true, safeToClearHub, nil
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
	if err := tx.Get(&lockedID, "SELECT id FROM computers WHERE id = ? FOR UPDATE", initial.ComputerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to lock computer"})
		return
	}
	if err := tx.Get(&lockedID, "SELECT id FROM users WHERE id = ? FOR UPDATE", initial.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to lock booking owner"})
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
	if !booking.EndTime.After(time.Now()) {
		c.JSON(http.StatusConflict, gin.H{"error": "This booking has expired"})
		return
	}

	if newEndTime.Sub(booking.StartTime) > time.Duration(maxDuration)*time.Minute {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Cannot extend: session would exceed the configured maximum of %d minutes", maxDuration)})
		return
	}

	if _, err := tx.Exec("UPDATE bookings SET end_time = ? WHERE id = ? AND status = 'active'", newEndTime, bookingID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to extend booking"})
		return
	}
	var comp models.Computer
	if err := tx.Get(&comp, "SELECT id, name FROM computers WHERE id = ?", booking.ComputerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find associated computer"})
		return
	}

	h.hub.Mu.Lock()
	// Preserve row-lock ordering through the Hub update. A later extension or
	// cancellation must not be overwritten by this committed session expiry.
	if err := tx.Commit(); err != nil {
		h.hub.Mu.Unlock()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit booking extension"})
		return
	}
	if state, exists := h.hub.Computers[comp.Name]; exists {
		if hub.MatchesBookingID(state.CurrentBookingID, booking.ID) {
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
