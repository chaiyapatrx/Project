package worker

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	"station-backend/internal/hub"
)

type SessionWorker struct {
	db  *sqlx.DB
	hub *hub.Hub
}

func StartSessionWorker(db *sqlx.DB, h *hub.Hub) func() {
	worker := &SessionWorker{db: db, hub: h}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				worker.checkExpiredSessions()
			case <-ctx.Done():
				return
			}
		}
	}()
	log.Println("[Worker] Real-time session countdown worker started (1s ticker)")
	return func() {
		cancel()
		<-done
	}
}

// expiredSession is a snapshot captured under the hub lock so that all
// side effects (agent commands, broadcasts, DB writes) can run *after* the
// lock is released. This avoids re-entering the non-reentrant hub RWMutex,
// which previously caused a deadlock when a session expired.
type expiredSession struct {
	compName   string
	userID     int
	sessionEnd time.Time
}

func (w *SessionWorker) checkExpiredSessions() {
	now := time.Now()

	// Phase 1: snapshot expired sessions under the hub lock.
	var expired []expiredSession

	w.hub.Mu.Lock()
	for compName, state := range w.hub.Computers {
		if state.Status == "in_use" && state.SessionEndsAt != nil && now.After(*state.SessionEndsAt) {
			userID := 0
			if state.CurrentUserID != nil {
				userID = *state.CurrentUserID
			}
			expired = append(expired, expiredSession{compName: compName, userID: userID, sessionEnd: *state.SessionEndsAt})
		}
	}
	w.hub.Mu.Unlock()

	// Phase 2: persist each expiration and send LOCK while its computer row is locked.
	for _, e := range expired {
		log.Printf("[Worker] Session EXPIRED for %s (User ID: %d). Forcing Lock...", e.compName, e.userID)
		w.expireSession(e, now)
	}
}

func (w *SessionWorker) expireSession(expired expiredSession, now time.Time) {
	tx, err := w.db.Beginx()
	if err != nil {
		log.Printf("[Worker] Transaction start error for %s: %v", expired.compName, err)
		return
	}
	defer tx.Rollback()

	var computerID int
	if err := tx.Get(&computerID, "SELECT id FROM computers WHERE name = ? FOR UPDATE", expired.compName); err != nil {
		log.Printf("[Worker] Failed to lock computer %s: %v", expired.compName, err)
		return
	}
	var booking struct {
		ID        int       `db:"id"`
		UserID    int       `db:"user_id"`
		StartTime time.Time `db:"start_time"`
		EndTime   time.Time `db:"end_time"`
	}
	err = tx.Get(&booking, "SELECT id, user_id, start_time, end_time FROM bookings WHERE computer_id = ? AND user_id = ? AND status = 'active' ORDER BY id DESC LIMIT 1 FOR UPDATE", computerID, expired.userID)
	if err == sql.ErrNoRows {
		var activeBookings int
		if err := tx.Get(&activeBookings, "SELECT COUNT(*) FROM bookings WHERE computer_id = ? AND status = 'active'", computerID); err != nil {
			log.Printf("[Worker] Failed to check current booking for %s: %v", expired.compName, err)
			return
		}
		if activeBookings > 0 {
			_ = tx.Commit()
			return
		}
		if activeBookings == 0 {
			if _, err := tx.Exec("UPDATE computers SET status = 'available' WHERE id = ? AND status = 'in_use'", computerID); err != nil {
				log.Printf("[Worker] Failed to release stale computer %s: %v", expired.compName, err)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			log.Printf("[Worker] Transaction commit failed for %s: %v", expired.compName, err)
			return
		}
	} else if err != nil {
		log.Printf("[Worker] Failed to read active booking for %s: %v", expired.compName, err)
		return
	} else if booking.EndTime.After(now) {
		end := booking.EndTime
		w.hub.Mu.Lock()
		if state := w.hub.Computers[expired.compName]; state != nil && state.CurrentUserID != nil && *state.CurrentUserID == expired.userID {
			state.SessionEndsAt = &end
		}
		w.hub.Mu.Unlock()
		w.hub.BroadcastStateChange(expired.compName)
		return
	} else {
		duration := int(now.Sub(booking.StartTime).Minutes())
		if duration < 1 {
			duration = 1
		}
		if _, err := tx.Exec(`
			INSERT INTO usage_logs (user_id, computer_id, booking_id, start_time, end_time, duration_minutes, termination_reason)
			VALUES (?, ?, ?, ?, ?, ?, 'timeout')`,
			booking.UserID, computerID, booking.ID, booking.StartTime, now, duration,
		); err != nil {
			log.Printf("[Worker] Failed to insert usage log for %s: %v", expired.compName, err)
			return
		}
		result, err := tx.Exec("UPDATE bookings SET status = 'completed' WHERE id = ? AND status = 'active'", booking.ID)
		if err != nil {
			log.Printf("[Worker] Failed to complete booking for %s: %v", expired.compName, err)
			return
		}
		changed, err := result.RowsAffected()
		if err != nil || changed != 1 {
			log.Printf("[Worker] Booking %d was already changed before expiration", booking.ID)
			return
		}
		if _, err := tx.Exec("UPDATE computers SET status = 'available' WHERE id = ? AND status = 'in_use'", computerID); err != nil {
			log.Printf("[Worker] Failed to release computer %s: %v", expired.compName, err)
			return
		}
		w.hub.SendCommandToAgent(expired.compName, "LOCK", map[string]interface{}{"reason": "session_expired"})
		if err := tx.Commit(); err != nil {
			log.Printf("[Worker] Transaction commit failed for %s: %v", expired.compName, err)
			return
		}
	}

	w.hub.Mu.Lock()
	if state := w.hub.Computers[expired.compName]; state != nil && state.SessionEndsAt != nil && state.SessionEndsAt.Equal(expired.sessionEnd) && (state.CurrentUserID == nil || *state.CurrentUserID == expired.userID) {
		state.Status = "available"
		state.CurrentUserID = nil
		state.CurrentUserName = nil
		state.SessionEndsAt = nil
	}
	w.hub.Mu.Unlock()
	w.hub.BroadcastStateChange(expired.compName)
}
