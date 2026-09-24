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
)

type WSHandler struct {
	hub      *hub.Hub
	cfg      *config.Config
	db       *sqlx.DB
	upgrader websocket.Upgrader
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

	return &WSHandler{hub: h, cfg: cfg, db: db, upgrader: upgrader}
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

	_ = h.hub.RegisterAgent(computerName, hwid, ip, comp.ID, commandSecret, conn)
	var currentSecretHash sql.NullString
	if err := h.db.Get(&currentSecretHash, "SELECT agent_secret_hash FROM computers WHERE id = ? AND is_active = 1", comp.ID); err != nil ||
		(currentSecretHash.Valid && !auth.VerifyAgentSecret(secret, currentSecretHash.String)) ||
		(!currentSecretHash.Valid && (!h.cfg.AllowLegacyAgents || h.cfg.AgentSecret == "" || subtle.ConstantTimeCompare([]byte(secret), []byte(h.cfg.AgentSecret)) != 1)) {
		_ = conn.Close()
		h.hub.UnregisterAgent(computerName, conn)
		return
	}
	defer h.hub.UnregisterAgent(computerName, conn)

	// Keep connection alive with Ping/Pong & bounded buffer
	conn.SetReadLimit(2048)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

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
