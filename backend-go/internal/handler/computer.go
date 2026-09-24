package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"station-backend/internal/auth"
	"station-backend/internal/hub"
	"station-backend/internal/models"
)

type ComputerHandler struct {
	db  *sqlx.DB
	hub *hub.Hub
}

func NewComputerHandler(db *sqlx.DB, hub *hub.Hub) *ComputerHandler {
	return &ComputerHandler{db: db, hub: hub}
}

func (h *ComputerHandler) ListComputers(c *gin.Context) {
	computers := make([]models.Computer, 0)
	err := h.db.Select(&computers, "SELECT id, name, status FROM computers WHERE is_active = 1 ORDER BY name ASC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch computers"})
		return
	}

	// Public station listings expose operational status only.
	type publicComputer struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Status   string `json:"status"`
		IsOnline bool   `json:"is_online"`
	}
	public := make([]publicComputer, len(computers))
	h.hub.Mu.RLock()
	for i := range computers {
		public[i] = publicComputer{ID: computers[i].ID, Name: computers[i].Name, Status: computers[i].Status}
		if runtimeState, exists := h.hub.Computers[computers[i].Name]; exists {
			public[i].IsOnline = runtimeState.IsOnline
			if runtimeState.Status != "" {
				public[i].Status = runtimeState.Status
			}
		}
	}
	h.hub.Mu.RUnlock()

	c.JSON(http.StatusOK, public)
}

func (h *ComputerHandler) ListComputersAdmin(c *gin.Context) {
	computers := make([]models.Computer, 0)
	if err := h.db.Select(&computers, `SELECT id, name, hwid, ip_address, mac_address, status, is_active, created_at, updated_at,
		(agent_secret_hash IS NOT NULL AND agent_secret_hash <> '') AS agent_key_configured
		FROM computers WHERE is_active = 1 ORDER BY name ASC`); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch computers"})
		return
	}
	h.hub.Mu.RLock()
	for i := range computers {
		if state := h.hub.Computers[computers[i].Name]; state != nil {
			computers[i].IsOnline = state.IsOnline
			computers[i].CurrentUserID = state.CurrentUserID
			computers[i].CurrentUserName = state.CurrentUserName
			computers[i].SessionEndsAt = state.SessionEndsAt
			if state.Status != "" {
				computers[i].Status = state.Status
			}
		}
	}
	h.hub.Mu.RUnlock()
	c.JSON(http.StatusOK, computers)
}

func (h *ComputerHandler) CreateComputer(c *gin.Context) {
	var req struct {
		Name   string `json:"name" binding:"required"`
		HWID   string `json:"hwid"`
		Status string `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 50 || len(req.HWID) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name must be 1-50 bytes and hardware ID at most 100 bytes"})
		return
	}

	status := "available"
	if req.Status != "" {
		status = strings.ToLower(strings.TrimSpace(req.Status))
	}
	if status != "available" && status != "maintenance" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid initial station status"})
		return
	}

	secret, secretHash, err := auth.GenerateAgentSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate agent credential"})
		return
	}

	var hwid any
	if req.HWID != "" {
		hwid = req.HWID
	}
	res, err := h.db.Exec("INSERT INTO computers (name, hwid, status, agent_secret_hash) VALUES (?, ?, ?, ?)", req.Name, hwid, status, secretHash)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Could not create station; its name or hardware ID may already exist"})
		return
	}

	id, err := res.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read station ID"})
		return
	}
	recordAudit(c, h.db, "computer_created", "computer", strconv.FormatInt(id, 10), "Station provisioned with a per-station agent credential")

	// Register into In-Memory Hub immediately
	h.hub.Mu.Lock()
	h.hub.Computers[req.Name] = &hub.ComputerRuntimeState{
		ID:       int(id),
		Name:     req.Name,
		Status:   status,
		IsOnline: false,
	}
	h.hub.Mu.Unlock()

	c.JSON(http.StatusCreated, gin.H{"id": id, "name": req.Name, "status": status, "agent_secret": secret})
}

func (h *ComputerHandler) RotateAgentSecret(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid computer ID"})
		return
	}
	secret, secretHash, err := auth.GenerateAgentSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate agent credential"})
		return
	}
	tx, err := h.db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to rotate agent credential"})
		return
	}
	var name string
	if err := tx.Get(&name, "SELECT name FROM computers WHERE id = ? AND is_active = 1 FOR UPDATE", id); err != nil {
		_ = tx.Rollback()
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Computer not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load computer"})
		}
		return
	}
	if _, err := tx.Exec("UPDATE computers SET agent_secret_hash = ? WHERE id = ?", secretHash, id); err != nil {
		_ = tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to rotate agent credential"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to rotate agent credential"})
		return
	}
	h.hub.DisconnectAgent(name)
	recordAudit(c, h.db, "agent_secret_rotated", "computer", strconv.Itoa(id), "Per-station agent credential rotated")
	c.JSON(http.StatusOK, gin.H{"agent_secret": secret})
}

func (h *ComputerHandler) UpdateComputer(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid computer ID"})
		return
	}

	var req struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	if req.Name != "" && len(req.Name) > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Station name must be at most 50 bytes"})
		return
	}
	if req.Status != "" && req.Status != "available" && req.Status != "maintenance" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid station status"})
		return
	}
	if req.Name == "" && req.Status == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one computer field must be supplied"})
		return
	}

	var oldComp models.Computer
	tx, err := h.db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update computer"})
		return
	}
	err = tx.Get(&oldComp, "SELECT name, status FROM computers WHERE id = ? AND is_active = 1 FOR UPDATE", id)
	if err != nil {
		_ = tx.Rollback()
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Computer not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load computer"})
		}
		return
	}
	if req.Name != "" && req.Name != oldComp.Name {
		_ = tx.Rollback()
		c.JSON(http.StatusConflict, gin.H{"error": "Station names cannot be changed after provisioning; update the agent configuration by creating a replacement station"})
		return
	}
	newStatus := req.Status
	if newStatus == "" {
		newStatus = oldComp.Status
	}
	if newStatus != oldComp.Status {
		var activeBookings int
		if err := tx.Get(&activeBookings, "SELECT COUNT(*) FROM bookings WHERE computer_id = ? AND status IN ('pending', 'active')", id); err != nil {
			_ = tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check active bookings"})
			return
		}
		if activeBookings > 0 {
			_ = tx.Rollback()
			c.JSON(http.StatusConflict, gin.H{"error": "End active bookings before changing this station status"})
			return
		}
	}
	if _, err := tx.Exec("UPDATE computers SET status = ? WHERE id = ?", newStatus, id); err != nil {
		_ = tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update computer"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update computer"})
		return
	}

	// Update In-Memory Hub state & broadcast change
	h.hub.Mu.Lock()
	if state, exists := h.hub.Computers[oldComp.Name]; exists {
		state.Status = newStatus
		go h.hub.BroadcastStateChange(state.Name)
	}
	h.hub.Mu.Unlock()
	recordAudit(c, h.db, "computer_status_changed", "computer", strconv.Itoa(id), "Station status changed to "+newStatus)

	c.JSON(http.StatusOK, gin.H{"message": "Computer updated successfully"})
}

func (h *ComputerHandler) DeleteComputer(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid computer ID"})
		return
	}

	tx, err := h.db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deactivate computer"})
		return
	}
	var comp models.Computer
	if err := tx.Get(&comp, "SELECT id, name FROM computers WHERE id = ? AND is_active = 1 FOR UPDATE", id); err != nil {
		_ = tx.Rollback()
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Computer not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load computer"})
		}
		return
	}
	var activeBookings int
	if err := tx.Get(&activeBookings, "SELECT COUNT(*) FROM bookings WHERE computer_id = ? AND status IN ('pending', 'active')", id); err != nil {
		_ = tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check active bookings"})
		return
	}
	if activeBookings > 0 {
		_ = tx.Rollback()
		c.JSON(http.StatusConflict, gin.H{"error": "Cancel active bookings before deactivating this station"})
		return
	}

	if _, err := tx.Exec("UPDATE computers SET is_active = 0, status = 'disabled' WHERE id = ? AND is_active = 1", id); err != nil {
		_ = tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete computer"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deactivate computer"})
		return
	}

	// Remove from In-Memory Hub
	if comp.Name != "" {
		h.hub.DisconnectAgent(comp.Name)
		h.hub.Mu.Lock()
		delete(h.hub.Computers, comp.Name)
		h.hub.Mu.Unlock()
	}
	recordAudit(c, h.db, "computer_deactivated", "computer", strconv.Itoa(id), "Station deactivated; usage history retained")

	c.JSON(http.StatusOK, gin.H{"message": "Computer deleted successfully"})
}

// SendCommand dispatches instant WebSocket command to client agent
func (h *ComputerHandler) SendCommand(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid computer ID"})
		return
	}

	var req struct {
		Command string `json:"command" binding:"required"` // LOCK, UNLOCK, REBOOT, SHUTDOWN, LOGOUT
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Command is required"})
		return
	}
	command := strings.ToUpper(strings.TrimSpace(req.Command))
	if !validAgentCommand(command) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported computer command"})
		return
	}

	var comp models.Computer
	err = h.db.Get(&comp, `SELECT id, name, hwid, ip_address, mac_address, status, is_active, created_at, updated_at
		FROM computers WHERE id = ? AND is_active = 1`, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Computer not found"})
		return
	}

	userRole := c.GetString("role")
	userID := c.GetInt("user_id")

	// Authorization Check:
	// Admin and Staff can send any command (LOCK, UNLOCK, REBOOT, SHUTDOWN)
	// Normal students/users can ONLY send UNLOCK to a computer they currently have an active booking on!
	if (command == "SHUTDOWN" || command == "REBOOT") && userRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins may shut down or restart a station"})
		return
	}
	if userRole != "admin" && userRole != "staff" {
		if command != "UNLOCK" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Students are only permitted to unlock their assigned station"})
			return
		}

		var hasBooking int
		err := h.db.Get(&hasBooking, "SELECT COUNT(*) FROM bookings WHERE computer_id = ? AND user_id = ? AND status = 'active'", id, userID)
		if err != nil || hasBooking == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have an active booking on this computer"})
			return
		}
	}

	// Dispatch instantly via Go Hub (Latency < 2ms)
	success := h.hub.SendCommandToAgent(comp.Name, command, map[string]interface{}{
		"initiated_by": c.GetString("username"),
	})

	// Log to audit_logs
	details := fmt.Sprintf("Command '%s' sent. Delivered: %v", command, success)
	targetType := "computer"
	targetID := strconv.Itoa(id)
	ip := c.ClientIP()
	_, _ = h.db.Exec(`
		INSERT INTO audit_logs (user_id, action, target_type, target_id, ip_address, details)
		VALUES (?, ?, ?, ?, ?, ?)`,
		userID, req.Command, targetType, targetID, ip, details,
	)

	c.JSON(http.StatusOK, gin.H{
		"message":   "Command dispatched",
		"delivered": success,
		"computer":  comp.Name,
		"command":   command,
	})
}

// BroadcastCommand sends command to all connected machines (e.g. emergency lock/shutdown/message)
func (h *ComputerHandler) BroadcastCommand(c *gin.Context) {
	var req struct {
		Command string                 `json:"command" binding:"required"`
		Data    map[string]interface{} `json:"data"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Command is required"})
		return
	}
	cmd := strings.ToUpper(strings.TrimSpace(req.Command))
	if !validAgentCommand(cmd) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported computer command"})
		return
	}
	if (cmd == "SHUTDOWN" || cmd == "REBOOT" || cmd == "UNLOCK") && c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins may broadcast this command"})
		return
	}
	if cmd == "MESSAGE" || cmd == "NOTIFICATION" {
		message, ok := req.Data["message"].(string)
		if !ok || strings.TrimSpace(message) == "" || len(message) > 500 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Message commands require a message of at most 500 bytes"})
			return
		}
	}

	if req.Data == nil {
		req.Data = make(map[string]interface{})
	}
	req.Data["initiated_by"] = c.GetString("username")

	count := h.hub.BroadcastCommandToAllAgents(cmd, req.Data)

	// Log audit
	userID := c.GetInt("user_id")
	details := fmt.Sprintf("Broadcast command '%s' sent to %d online machines", cmd, count)
	targetType := "all_computers"
	ip := c.ClientIP()
	_, _ = h.db.Exec(`
		INSERT INTO audit_logs (user_id, action, target_type, ip_address, details)
		VALUES (?, ?, ?, ?, ?)`,
		userID, cmd, targetType, ip, details,
	)

	c.JSON(http.StatusOK, gin.H{
		"message":         "Broadcast command processed",
		"command":         cmd,
		"delivered_count": count,
	})
}

func validAgentCommand(command string) bool {
	switch command {
	case "LOCK", "UNLOCK", "REBOOT", "SHUTDOWN", "MESSAGE", "NOTIFICATION":
		return true
	default:
		return false
	}
}
