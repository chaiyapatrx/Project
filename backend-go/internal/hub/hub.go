package hub

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// In-Memory Computer State
type ComputerRuntimeState struct {
	ID              int             `json:"id"`
	Name            string          `json:"name"`
	HWID            string          `json:"hwid"`
	IPAddress       string          `json:"ip_address"`
	Status          string          `json:"status"` // available, in_use, maintenance, disabled
	IsOnline        bool            `json:"is_online"`
	CurrentUserID   *int            `json:"current_user_id,omitempty"`
	CurrentUserName *string         `json:"current_user_name,omitempty"`
	SessionEndsAt   *time.Time      `json:"session_ends_at,omitempty"`
	LastPing        time.Time       `json:"last_ping"`
	AgentConn       *websocket.Conn `json:"-"`
	AgentSecret     string          `json:"-"`
}

// ComputerStatus contains only fields safe to broadcast to authenticated web clients.
type ComputerStatus struct {
	ID              int        `json:"id"`
	Name            string     `json:"name"`
	Status          string     `json:"status"`
	IsOnline        bool       `json:"is_online"`
	CurrentUserName *string    `json:"current_user_name,omitempty"`
	SessionEndsAt   *time.Time `json:"session_ends_at,omitempty"`
}

func publicStatus(state *ComputerRuntimeState) ComputerStatus {
	status := ComputerStatus{ID: state.ID, Name: state.Name, Status: state.Status, IsOnline: state.IsOnline}
	if state.CurrentUserName != nil {
		name := *state.CurrentUserName
		status.CurrentUserName = &name
	}
	if state.SessionEndsAt != nil {
		end := *state.SessionEndsAt
		status.SessionEndsAt = &end
	}
	return status
}

// Hub coordinates all real-time events between Agents and Web clients
type Hub struct {
	// Key: computer name (e.g. "COM-01")
	Computers map[string]*ComputerRuntimeState
	// Web dashboard clients
	WebClients map[*websocket.Conn]bool

	// Per-connection write mutexes to guarantee gorilla/websocket concurrency safety.
	connLocksMu sync.Mutex
	connLocks   map[*websocket.Conn]*connectionLock

	Mu sync.RWMutex
}

type connectionLock struct {
	mu      sync.Mutex
	refs    int
	retired bool
}

var GlobalHub *Hub

func InitHub() *Hub {
	h := &Hub{
		Computers:  make(map[string]*ComputerRuntimeState),
		WebClients: make(map[*websocket.Conn]bool),
		connLocks:  make(map[*websocket.Conn]*connectionLock),
	}
	GlobalHub = h
	return h
}

// writeSafe ensures thread-safe, timeout-bounded writes on gorilla/websocket connections
func (h *Hub) writeSafe(conn *websocket.Conn, msgType int, data []byte) error {
	if conn == nil {
		return fmt.Errorf("nil connection")
	}
	h.connLocksMu.Lock()
	lock := h.connLocks[conn]
	if lock == nil {
		lock = &connectionLock{}
		h.connLocks[conn] = lock
	}
	lock.refs++
	h.connLocksMu.Unlock()
	lock.mu.Lock()
	defer func() {
		lock.mu.Unlock()
		h.connLocksMu.Lock()
		lock.refs--
		if lock.retired && lock.refs == 0 && h.connLocks[conn] == lock {
			delete(h.connLocks, conn)
		}
		h.connLocksMu.Unlock()
	}()
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	return conn.WriteMessage(msgType, data)
}

func (h *Hub) removeConnLock(conn *websocket.Conn) {
	h.connLocksMu.Lock()
	if lock := h.connLocks[conn]; lock != nil {
		lock.retired = true
		if lock.refs == 0 {
			delete(h.connLocks, conn)
		}
	}
	h.connLocksMu.Unlock()
}

// signCommand builds a signed payload covering the action, timestamp, and data.
func signCommand(secret, command string, ts int64, data map[string]interface{}) map[string]interface{} {
	unsigned := struct {
		Action    string                 `json:"action"`
		Timestamp int64                  `json:"timestamp"`
		Data      map[string]interface{} `json:"data"`
	}{command, ts, data}
	canonical, err := json.Marshal(unsigned)
	payload := map[string]interface{}{"action": command, "timestamp": ts, "data": data}
	if err == nil && secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(canonical)
		payload["signature"] = hex.EncodeToString(mac.Sum(nil))
	}
	return payload
}

// RegisterAgent handles incoming agent connections and sets ID atomically before broadcast
func (h *Hub) RegisterAgent(name, hwid, ip string, compID int, secret string, conn *websocket.Conn) *ComputerRuntimeState {
	h.Mu.Lock()

	state, exists := h.Computers[name]
	if !exists {
		state = &ComputerRuntimeState{
			Name:     name,
			HWID:     hwid,
			Status:   "available",
			IsOnline: true,
		}
		h.Computers[name] = state
	}
	previousConn := state.AgentConn

	state.ID = compID
	state.HWID = hwid
	state.IPAddress = ip
	state.IsOnline = true
	state.LastPing = time.Now()
	state.AgentConn = conn
	state.AgentSecret = secret

	h.Mu.Unlock()
	if previousConn != nil && previousConn != conn {
		_ = previousConn.Close()
	}
	go h.BroadcastStateChange(name)
	log.Printf("[Hub] Agent connected: %s (ID: %d, HWID: %s, IP: %s)", name, compID, hwid, ip)
	return state
}

// DisconnectAgent closes the current connection after credential rotation.
func (h *Hub) DisconnectAgent(name string) {
	h.Mu.Lock()
	state := h.Computers[name]
	var conn *websocket.Conn
	if state != nil {
		conn = state.AgentConn
		state.AgentConn = nil
		state.AgentSecret = ""
		state.IsOnline = false
	}
	h.Mu.Unlock()
	if conn != nil {
		_ = conn.Close()
		h.removeConnLock(conn)
		go h.BroadcastStateChange(name)
	}
}

// UnregisterAgent handles disconnection
func (h *Hub) UnregisterAgent(name string, conn *websocket.Conn) {
	h.Mu.Lock()
	broadcast := false
	if state, exists := h.Computers[name]; exists {
		if state.AgentConn == conn {
			state.IsOnline = false
			state.AgentConn = nil
			broadcast = true
			log.Printf("[Hub] Agent disconnected: %s", name)
		}
	}
	h.Mu.Unlock()
	h.removeConnLock(conn)
	if broadcast {
		go h.BroadcastStateChange(name)
	}
}

func (h *Hub) IsAgentOnline(name string) bool {
	h.Mu.RLock()
	defer h.Mu.RUnlock()
	state := h.Computers[name]
	return state != nil && state.IsOnline && state.AgentConn != nil
}

// SendCommandToAgent sends a real-time command (LOCK, UNLOCK, REBOOT, SHUTDOWN) directly to the machine
func (h *Hub) SendCommandToAgent(name string, command string, data map[string]interface{}) bool {
	h.Mu.RLock()
	state, exists := h.Computers[name]
	var conn *websocket.Conn
	var secret string
	if exists {
		conn = state.AgentConn
		secret = state.AgentSecret
	}
	h.Mu.RUnlock()

	if !exists || conn == nil || secret == "" {
		log.Printf("[Hub] Cannot send command '%s' to %s: Agent offline or not connected", command, name)
		return false
	}

	payload := signCommand(secret, command, time.Now().Unix(), data)
	msg, err := json.Marshal(payload)
	if err != nil {
		return false
	}

	if err := h.writeSafe(conn, websocket.TextMessage, msg); err != nil {
		log.Printf("[Hub] Error writing command to %s: %v", name, err)
		return false
	}

	log.Printf("[Hub] Command '%s' dispatched instantly to %s", command, name)
	return true
}

// RegisterWebClient adds a browser monitor client
func (h *Hub) RegisterWebClient(conn *websocket.Conn) {
	h.Mu.Lock()
	h.WebClients[conn] = true
	h.Mu.Unlock()

	// Send current snapshot immediately to newly connected client
	snapshot := h.GetSnapshot()
	msg, _ := json.Marshal(map[string]interface{}{
		"type":    "SNAPSHOT",
		"payload": snapshot,
	})
	_ = h.writeSafe(conn, websocket.TextMessage, msg)
}

// UnregisterWebClient removes browser client
func (h *Hub) UnregisterWebClient(conn *websocket.Conn) {
	h.Mu.Lock()
	delete(h.WebClients, conn)
	h.Mu.Unlock()
	h.removeConnLock(conn)
}

// PingWebClient uses the same write lock as broadcasts to satisfy gorilla/websocket's one-writer rule.
func (h *Hub) PingWebClient(conn *websocket.Conn) error {
	return h.writeSafe(conn, websocket.PingMessage, nil)
}

// BroadcastCommandToAllAgents sends a command to every online agent concurrently without blocking Hub Mutex
func (h *Hub) BroadcastCommandToAllAgents(command string, data map[string]interface{}) int {
	type agentConn struct {
		name   string
		conn   *websocket.Conn
		secret string
	}
	h.Mu.RLock()
	onlineAgents := make([]agentConn, 0, len(h.Computers))
	for name, state := range h.Computers {
		if state.IsOnline && state.AgentConn != nil && state.AgentSecret != "" {
			onlineAgents = append(onlineAgents, agentConn{name: name, conn: state.AgentConn, secret: state.AgentSecret})
		}
	}
	h.Mu.RUnlock()

	var wg sync.WaitGroup
	var deliveredCount int64

	for _, agent := range onlineAgents {
		wg.Add(1)
		go func(agent agentConn) {
			defer wg.Done()
			payload := signCommand(agent.secret, command, time.Now().Unix(), data)
			msg, err := json.Marshal(payload)
			if err != nil {
				return
			}
			if err := h.writeSafe(agent.conn, websocket.TextMessage, msg); err == nil {
				atomic.AddInt64(&deliveredCount, 1)
				log.Printf("[Hub] Broadcast command '%s' delivered to %s", command, agent.name)
			}
		}(agent)
	}

	wg.Wait()
	return int(deliveredCount)
}

// GetComputerState returns the runtime state for a computer by name, or nil.
func (h *Hub) GetComputerState(name string) *ComputerRuntimeState {
	h.Mu.RLock()
	defer h.Mu.RUnlock()
	return h.Computers[name]
}

// BroadcastStateChange notifies all web clients of a single machine update.
func (h *Hub) BroadcastStateChange(name string) {
	h.Mu.RLock()
	state := h.Computers[name]
	if state == nil {
		h.Mu.RUnlock()
		return
	}
	payload := publicStatus(state)
	clients := make([]*websocket.Conn, 0, len(h.WebClients))
	for client := range h.WebClients {
		clients = append(clients, client)
	}
	h.Mu.RUnlock()

	msg, err := json.Marshal(map[string]interface{}{
		"type":    "COMPUTER_STATUS_CHANGED",
		"payload": payload,
	})
	if err != nil {
		return
	}

	var dead []*websocket.Conn
	for _, client := range clients {
		if err := h.writeSafe(client, websocket.TextMessage, msg); err != nil {
			dead = append(dead, client)
		}
	}

	// Remove any failed clients under a write lock.
	if len(dead) > 0 {
		h.Mu.Lock()
		for _, client := range dead {
			client.Close()
			delete(h.WebClients, client)
			h.removeConnLock(client)
		}
		h.Mu.Unlock()
	}
}

// GetSnapshot returns full runtime state of all computers
func (h *Hub) GetSnapshot() []ComputerStatus {
	h.Mu.RLock()
	defer h.Mu.RUnlock()

	list := make([]ComputerStatus, 0, len(h.Computers))
	for _, c := range h.Computers {
		list = append(list, publicStatus(c))
	}
	return list
}
