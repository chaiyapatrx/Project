package handler

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jmoiron/sqlx"
	"station-backend/internal/auth"
	"station-backend/internal/config"
	"station-backend/internal/hub"
	"station-backend/internal/middleware"
)

type WSHandler struct {
	hub                 *hub.Hub
	cfg                 *config.Config
	db                  *sqlx.DB
	stationLoginLimiter *middleware.IPRateLimiter
	upgrader            websocket.Upgrader
}

func NewWSHandler(h *hub.Hub, cfg *config.Config, db *sqlx.DB) *WSHandler {
	allowed := make(map[string]bool, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		allowed[strings.TrimSpace(o)] = true
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			// Non-browser clients (e.g. the Go agent) send no Origin header.
			// Browser-originated connections must match the allow-list to
			// prevent Cross-Site WebSocket Hijacking.
			if origin == "" {
				return true
			}
			return allowed[origin]
		},
	}

	return &WSHandler{hub: h, cfg: cfg, db: db, stationLoginLimiter: middleware.NewIPRateLimiter(5, time.Minute), upgrader: upgrader}
}

// AgentWS handles client agent connection with secret authentication
func (h *WSHandler) AgentWS(c *gin.Context) {
	computerName := strings.TrimSpace(c.Query("name"))
	hwid := c.Query("hwid")
	secret := c.GetHeader("X-Agent-Secret")

	if computerName == "" || len(computerName) > 50 || hwid == "" || len(hwid) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Machine name and hardware ID are required"})
		return
	}
	if len(secret) < 32 || len(secret) > 256 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized agent connection"})
		return
	}

	ip := c.ClientIP()
	var comp struct {
		ID              int            `db:"id"`
		HWID            sql.NullString `db:"hwid"`
		AgentSecretHash sql.NullString `db:"agent_secret_hash"`
		Status          string         `db:"status"`
	}
	err := h.db.Get(&comp, "SELECT id, hwid, agent_secret_hash, status FROM computers WHERE name = ? AND is_active = 1", computerName)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusForbidden, gin.H{"error": "Computer is not provisioned"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify agent computer"})
		return
	}
	if comp.Status == "disabled" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Computer is disabled"})
		return
	}

	commandSecret := ""
	if comp.AgentSecretHash.Valid && auth.VerifyAgentSecret(secret, comp.AgentSecretHash.String) {
		commandSecret = secret
	} else if !comp.AgentSecretHash.Valid && h.cfg.AllowLegacyAgents && h.cfg.AgentSecret != "" && subtle.ConstantTimeCompare([]byte(secret), []byte(h.cfg.AgentSecret)) == 1 {
		commandSecret = h.cfg.AgentSecret
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized agent connection"})
		return
	}

	if comp.HWID.Valid && comp.HWID.String != hwid {
		c.JSON(http.StatusForbidden, gin.H{"error": "Agent hardware ID does not match the registered computer"})
		return
	} else if !comp.HWID.Valid {
		result, err := h.db.Exec("UPDATE computers SET hwid = ?, ip_address = ? WHERE id = ? AND hwid IS NULL AND is_active = 1", hwid, c.ClientIP(), comp.ID)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Agent hardware ID is already registered"})
			return
		}
		if rows, _ := result.RowsAffected(); rows != 1 {
			c.JSON(http.StatusForbidden, gin.H{"error": "Agent hardware ID does not match the registered computer"})
			return
		}
	}
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WS] Failed to upgrade agent connection: %v", err)
		return
	}
	defer conn.Close()

	// Lock the station row while validating credentials and syncing its lock
	// state. Booking creation uses the same row lock, so it cannot dispatch a
	// stale LOCK after a new booking's UNLOCK.
	tx, err := h.db.Beginx()
	if err != nil {
		log.Printf("[WS] Failed to start agent sync transaction for %s: %v", computerName, err)
		return
	}
	defer tx.Rollback()
	var current struct {
		HWID            sql.NullString `db:"hwid"`
		AgentSecretHash sql.NullString `db:"agent_secret_hash"`
		Status          string         `db:"status"`
	}
	if err := tx.Get(&current, "SELECT hwid, agent_secret_hash, status FROM computers WHERE id = ? AND is_active = 1 FOR UPDATE", comp.ID); err != nil {
		log.Printf("[WS] Failed to lock agent computer %s: %v", computerName, err)
		return
	}
	if !current.HWID.Valid || current.HWID.String != hwid ||
		(current.AgentSecretHash.Valid && !auth.VerifyAgentSecret(secret, current.AgentSecretHash.String)) ||
		(!current.AgentSecretHash.Valid && (!h.cfg.AllowLegacyAgents || h.cfg.AgentSecret == "" || subtle.ConstantTimeCompare([]byte(secret), []byte(h.cfg.AgentSecret)) != 1)) {
		log.Printf("[WS] Agent credentials changed during connection for %s", computerName)
		return
	}

	command := "LOCK"
	hasActiveBooking := false
	err = tx.Get(new(int), "SELECT id FROM bookings WHERE computer_id = ? AND status = 'active' ORDER BY id DESC LIMIT 1 FOR UPDATE", comp.ID)
	if err == nil {
		hasActiveBooking = true
	} else if err != sql.ErrNoRows {
		log.Printf("[WS] Failed to read active session for %s: %v", computerName, err)
		return
	}
	var usage struct {
		ID            int          `db:"id"`
		UserID        int          `db:"user_id"`
		Username      string       `db:"-"`
		SessionEndsAt sql.NullTime `db:"session_ends_at"`
	}
	hasActiveUsage := false
	if !hasActiveBooking {
		err := tx.Get(&usage, `
			SELECT id, user_id, session_ends_at FROM usage_logs
			WHERE computer_id = ? AND booking_id IS NULL AND end_time IS NULL
			ORDER BY id DESC LIMIT 1 FOR UPDATE`, comp.ID)
		if err == nil {
			// Any open walk-in row is still a live DB session, even when an
			// older/partial row has no expiry. Restore NULL expiry as already
			// expired so SessionWorker can close and audit it instead of leaving
			// the station permanently blocked.
			hasActiveUsage = true
			if err := tx.Get(&usage.Username, "SELECT username FROM users WHERE id = ?", usage.UserID); err != nil {
				log.Printf("[WS] Failed to read walk-in user for %s: %v", computerName, err)
				return
			}
		} else if err != sql.ErrNoRows {
			log.Printf("[WS] Failed to read active walk-in session for %s: %v", computerName, err)
			return
		}
	}
	decision := decideAgentSync(current.Status, hasActiveBooking, hasActiveUsage)
	if !hasActiveBooking && !hasActiveUsage {
		h.hub.Mu.RLock()
		staleHubSession := hasStaleAgentHubSession(h.hub.Computers[computerName], current.Status)
		h.hub.Mu.RUnlock()
		decision.repairStaleState = decision.repairStaleState || staleHubSession
	}
	hubStatus := current.Status
	data := map[string]interface{}{"reason": "agent_connected", "auth_mode": decision.authMode}
	if hasActiveBooking && current.Status == "available" {
		if _, err := tx.Exec("UPDATE computers SET status = 'in_use' WHERE id = ? AND status = 'available'", comp.ID); err != nil {
			log.Printf("[WS] Failed to restore reserved station status for %s", computerName)
			return
		}
	}
	if hasActiveUsage && current.Status == "available" {
		if _, err := tx.Exec("UPDATE computers SET status = 'in_use' WHERE id = ? AND status = 'available'", comp.ID); err != nil {
			log.Printf("[WS] Failed to restore walk-in station status for %s", computerName)
			return
		}
	}
	if decision.repairStaleState && current.Status == "in_use" {
		result, err := tx.Exec("UPDATE computers SET status = 'available' WHERE id = ? AND status = 'in_use'", comp.ID)
		if err != nil {
			log.Printf("[WS] Failed to repair stale station status for %s", computerName)
			return
		}
		changed, err := result.RowsAffected()
		if err != nil || changed != 1 {
			log.Printf("[WS] Stale station status changed during sync for %s", computerName)
			return
		}
		hubStatus = "available"
	}
	h.hub.RegisterAgent(computerName, hwid, ip, comp.ID, commandSecret, conn)
	defer h.hub.UnregisterAgent(computerName, conn)
	if hasActiveUsage {
		end := restoredWalkInSessionEnd(usage.SessionEndsAt)
		h.hub.Mu.Lock()
		if state := h.hub.Computers[computerName]; state != nil {
			state.Status = "in_use"
			state.CurrentBookingID = 0
			state.CurrentUsageLogID = usage.ID
			state.CurrentUserID = &usage.UserID
			state.CurrentUserName = &usage.Username
			state.SessionEndsAt = &end
		}
		h.hub.Mu.Unlock()
	}
	var staleHubSnapshot agentHubSnapshot
	hasStaleHubSnapshot := false
	if decision.repairStaleState {
		h.hub.Mu.RLock()
		if state := h.hub.Computers[computerName]; state != nil {
			staleHubSnapshot = snapshotAgentHubState(state)
			hasStaleHubSnapshot = true
		}
		h.hub.Mu.RUnlock()
	}
	if !h.hub.SendCommandToAgent(computerName, command, data) {
		log.Printf("[WS] Failed to sync lock state for %s", computerName)
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("[WS] Failed to commit agent sync for %s", computerName)
		if decision.repairStaleState {
			clearAgentHubSessionIfSnapshotMatches(h.hub, computerName, staleHubSnapshot, hasStaleHubSnapshot, hubStatus)
		}
		_ = conn.Close()
		return
	}
	if decision.repairStaleState {
		clearAgentHubSessionIfSnapshotMatches(h.hub, computerName, staleHubSnapshot, hasStaleHubSnapshot, hubStatus)
	}
	h.hub.BroadcastStateChange(computerName)

	// Keep connection alive with Ping/Pong & bounded buffer
	conn.SetReadLimit(2048)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	stopPinging := startAgentPing(conn, agentPingInterval)
	defer stopPinging()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WS] Agent %s closed unexpectedly: %v", computerName, err)
			}
			break
		}

		// Agent heartbeat or telemetry
		var pingData struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(message, &pingData) == nil {
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		}
	}
}

func restoredWalkInSessionEnd(sessionEndsAt sql.NullTime) time.Time {
	if !sessionEndsAt.Valid {
		return time.Time{}
	}
	return sessionEndsAt.Time
}

const (
	agentPingInterval     = 20 * time.Second
	agentPingWriteTimeout = 2 * time.Second
)

func startAgentPing(conn *websocket.Conn, interval time.Duration) func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	ticker := time.NewTicker(interval)
	go func() {
		defer close(done)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(agentPingWriteTimeout)); err != nil {
					_ = conn.Close()
					return
				}
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}

type agentSyncDecision struct {
	command          string
	authMode         string
	repairStaleState bool
}

type agentHubSnapshot struct {
	bookingID     int
	usageLogID    int
	status        string
	userID        int
	hasUserID     bool
	userName      string
	hasUserName   bool
	sessionEnd    time.Time
	hasSessionEnd bool
}

func decideAgentSync(status string, hasActiveBooking, hasActiveUsage bool) agentSyncDecision {
	decision := agentSyncDecision{command: "LOCK", authMode: "account"}
	if hasActiveBooking {
		decision.authMode = "access_code"
	} else if status == "in_use" && !hasActiveUsage {
		decision.repairStaleState = true
	}
	return decision
}

func snapshotAgentHubState(state *hub.ComputerRuntimeState) agentHubSnapshot {
	snapshot := agentHubSnapshot{bookingID: state.CurrentBookingID, usageLogID: state.CurrentUsageLogID, status: state.Status}
	if state.CurrentUserID != nil {
		snapshot.userID = *state.CurrentUserID
		snapshot.hasUserID = true
	}
	if state.CurrentUserName != nil {
		snapshot.userName = *state.CurrentUserName
		snapshot.hasUserName = true
	}
	if state.SessionEndsAt != nil {
		snapshot.sessionEnd = *state.SessionEndsAt
		snapshot.hasSessionEnd = true
	}
	return snapshot
}

func matchesAgentHubSnapshot(state *hub.ComputerRuntimeState, snapshot agentHubSnapshot) bool {
	if state.CurrentBookingID != snapshot.bookingID || state.CurrentUsageLogID != snapshot.usageLogID || state.Status != snapshot.status {
		return false
	}
	if (state.CurrentUserID != nil) != snapshot.hasUserID || (snapshot.hasUserID && *state.CurrentUserID != snapshot.userID) {
		return false
	}
	if (state.CurrentUserName != nil) != snapshot.hasUserName || (snapshot.hasUserName && *state.CurrentUserName != snapshot.userName) {
		return false
	}
	if (state.SessionEndsAt != nil) != snapshot.hasSessionEnd || (snapshot.hasSessionEnd && !state.SessionEndsAt.Equal(snapshot.sessionEnd)) {
		return false
	}
	return true
}

func hasStaleAgentHubSession(state *hub.ComputerRuntimeState, databaseStatus string) bool {
	if state == nil {
		return false
	}
	return state.Status != databaseStatus || state.CurrentBookingID != 0 || state.CurrentUsageLogID != 0 ||
		state.CurrentUserID != nil || state.CurrentUserName != nil || state.SessionEndsAt != nil
}

func clearAgentHubSessionIfSnapshotMatches(h *hub.Hub, computerName string, snapshot agentHubSnapshot, hasSnapshot bool, databaseStatus string) {
	if !hasSnapshot {
		return
	}
	h.Mu.Lock()
	if state := h.Computers[computerName]; state != nil && matchesAgentHubSnapshot(state, snapshot) {
		state.Status = databaseStatus
		state.CurrentBookingID = 0
		state.CurrentUsageLogID = 0
		state.CurrentUserID = nil
		state.CurrentUserName = nil
		state.SessionEndsAt = nil
	}
	h.Mu.Unlock()
}

// MonitorWS handles React Frontend web socket connection with strict token enforcement
func (h *WSHandler) MonitorWS(c *gin.Context) {
	// Prefer the HttpOnly auth cookie; allow the websocket subprotocol for legacy clients.
	token := ""
	if cookie, err := c.Cookie("auth_token"); err == nil && cookie != "" {
		token = cookie
	}
	if token == "" {
		token = c.GetHeader("Sec-WebSocket-Protocol")
	}

	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication token is required for monitor websocket"})
		return
	}

	claims, err := auth.ValidateToken(token, h.cfg)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		return
	}
	var current struct {
		Role         string `db:"role"`
		IsActive     bool   `db:"is_active"`
		TokenVersion int    `db:"token_version"`
	}
	if err := h.db.Get(&current, "SELECT role, is_active, token_version FROM users WHERE id = ?", claims.UserID); err != nil || !current.IsActive || claims.TokenVersion != current.TokenVersion {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User account is inactive or session was revoked"})
		return
	}
	if current.Role != "admin" && current.Role != "staff" && current.Role != "executive" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Monitor access is restricted"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WS] Failed to upgrade web client: %v", err)
		return
	}
	defer conn.Close()

	h.hub.RegisterWebClient(conn)
	defer h.hub.UnregisterWebClient(conn)

	// Keep-alive: Server sends ping every 30s to keep browser connection warm
	conn.SetReadLimit(1024)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	stopPing := make(chan struct{})
	defer close(stopPing)

	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				var active bool
				var version int
				if err := h.db.QueryRowx("SELECT is_active, token_version FROM users WHERE id = ?", claims.UserID).Scan(&active, &version); err != nil || !active || version != claims.TokenVersion {
					_ = conn.Close()
					return
				}
				if err := h.hub.PingWebClient(conn); err != nil {
					return
				}
			case <-stopPing:
				return
			}
		}
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	}
}
