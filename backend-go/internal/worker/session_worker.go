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
	bookingID  int
	usageLogID int
	userID     int
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
			expired = append(expired, expiredSession{compName: compName, bookingID: state.CurrentBookingID, usageLogID: state.CurrentUsageLogID, userID: userID})
		}
	}
	w.hub.Mu.Unlock()

	// Phase 2: persist each expiration and send LOCK while its computer row is locked.
	for _, e := range expired {
		log.Printf("[Worker] Session EXPIRED for %s (User ID: %d). Forcing Lock...", e.compName, e.userID)
		if e.usageLogID > 0 {
			w.expireWalkInSession(e, now)
		} else {
			w.expireSession(e, now)
		}
	}
}

func (w *SessionWorker) expireWalkInSession(expired expiredSession, now time.Time) {
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
	var session struct {
		UserID        int          `db:"user_id"`
		StartTime     time.Time    `db:"start_time"`
		SessionEndsAt sql.NullTime `db:"session_ends_at"`
	}
	err = tx.Get(&session, `
		SELECT user_id, start_time, session_ends_at FROM usage_logs
		WHERE id = ? AND computer_id = ? AND booking_id IS NULL AND end_time IS NULL FOR UPDATE`, expired.usageLogID, computerID)
	if err == sql.ErrNoRows {
		var active int
		if err := tx.Get(&active, `SELECT
			(SELECT COUNT(*) FROM bookings WHERE computer_id = ? AND status IN ('pending', 'active')) +
			(SELECT COUNT(*) FROM usage_logs WHERE computer_id = ? AND booking_id IS NULL AND end_time IS NULL)`, computerID, computerID); err != nil {
			log.Printf("[Worker] Failed to check replacement session for %s: %v", expired.compName, err)
			return
		}
		if active > 0 {
			return
		}
		if _, err := tx.Exec("UPDATE computers SET status = 'available' WHERE id = ? AND status = 'in_use'", computerID); err != nil {
			log.Printf("[Worker] Failed to release stale computer %s: %v", expired.compName, err)
			return
		}
		if !w.hub.SendCommandToAgent(expired.compName, "LOCK", map[string]interface{}{"reason": "session_expired", "auth_mode": "account"}) {
			w.hub.DisconnectAgent(expired.compName)
		}
	} else if err != nil {
		log.Printf("[Worker] Failed to read usage session for %s: %v", expired.compName, err)
		return
	} else {
		if session.SessionEndsAt.Valid && session.SessionEndsAt.Time.After(now) {
			end := session.SessionEndsAt.Time
			w.hub.Mu.Lock()
			if state := w.hub.Computers[expired.compName]; state != nil && state.CurrentUsageLogID == expired.usageLogID {
				state.SessionEndsAt = &end
			}
			w.hub.Mu.Unlock()
			return
		}
		duration := int(now.Sub(session.StartTime).Minutes())
		if duration < 1 {
			duration = 1
		}
		result, err := tx.Exec(`
			UPDATE usage_logs SET end_time = ?, session_ends_at = NULL, duration_minutes = ?, termination_reason = 'timeout'
			WHERE id = ? AND end_time IS NULL`, now, duration, expired.usageLogID)
		if err != nil {
			log.Printf("[Worker] Failed to close usage session for %s: %v", expired.compName, err)
			return
		}
		changed, err := result.RowsAffected()
		if err != nil || changed != 1 {
			return
		}
		var active int
		if err := tx.Get(&active, `SELECT
			(SELECT COUNT(*) FROM bookings WHERE computer_id = ? AND status IN ('pending', 'active')) +
			(SELECT COUNT(*) FROM usage_logs WHERE computer_id = ? AND booking_id IS NULL AND end_time IS NULL)`, computerID, computerID); err != nil {
			log.Printf("[Worker] Failed to check replacement session for %s: %v", expired.compName, err)
			return
		}
		if active == 0 {
			if _, err := tx.Exec("UPDATE computers SET status = 'available' WHERE id = ? AND status = 'in_use'", computerID); err != nil {
				log.Printf("[Worker] Failed to release computer %s: %v", expired.compName, err)
				return
			}
			if !w.hub.SendCommandToAgent(expired.compName, "LOCK", map[string]interface{}{"reason": "session_expired", "auth_mode": "account"}) {
				w.hub.DisconnectAgent(expired.compName)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		w.hub.DisconnectAgent(expired.compName)
		log.Printf("[Worker] Transaction commit failed for %s: %v", expired.compName, err)
		return
	}

	w.hub.Mu.Lock()
	if state := w.hub.Computers[expired.compName]; state != nil && state.CurrentUsageLogID == expired.usageLogID {
		if state.Status == "in_use" {
			state.Status = "available"
		}
		state.CurrentBookingID = 0
		state.CurrentUsageLogID = 0
		state.CurrentUserID = nil
		state.CurrentUserName = nil
		state.SessionEndsAt = nil
	}
	w.hub.Mu.Unlock()
	w.hub.BroadcastStateChange(expired.compName)
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
	err = tx.Get(&booking, "SELECT id, user_id, start_time, end_time FROM bookings WHERE id = ? AND computer_id = ? AND status = 'active' FOR UPDATE", expired.bookingID, computerID)
	if err == sql.ErrNoRows {
		var activeBookings int
		if err := tx.Get(&activeBookings, `SELECT
			(SELECT COUNT(*) FROM bookings WHERE computer_id = ? AND status IN ('pending', 'active')) +
			(SELECT COUNT(*) FROM usage_logs WHERE computer_id = ? AND booking_id IS NULL AND end_time IS NULL)`, computerID, computerID); err != nil {
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
			if !w.hub.SendCommandToAgent(expired.compName, "LOCK", map[string]interface{}{"reason": "session_expired", "auth_mode": "account"}) {
				w.hub.DisconnectAgent(expired.compName)
			}
		}
		if err := tx.Commit(); err != nil {
			w.hub.DisconnectAgent(expired.compName)
			log.Printf("[Worker] Transaction commit failed for %s: %v", expired.compName, err)
			return
		}
	} else if err != nil {
		log.Printf("[Worker] Failed to read active booking for %s: %v", expired.compName, err)
		return
	} else if booking.EndTime.After(now) {
		end := booking.EndTime
		w.hub.Mu.Lock()
		if state := w.hub.Computers[expired.compName]; state != nil && hub.MatchesBookingID(state.CurrentBookingID, expired.bookingID) {
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
		if !w.hub.SendCommandToAgent(expired.compName, "LOCK", map[string]interface{}{"reason": "session_expired", "auth_mode": "account"}) {
			w.hub.DisconnectAgent(expired.compName)
		}
		if err := tx.Commit(); err != nil {
			w.hub.DisconnectAgent(expired.compName)
			log.Printf("[Worker] Transaction commit failed for %s: %v", expired.compName, err)
			return
		}
	}

	w.hub.Mu.Lock()
	if state := w.hub.Computers[expired.compName]; state != nil && hub.MatchesBookingID(state.CurrentBookingID, expired.bookingID) {
		if state.Status == "in_use" {
			state.Status = "available"
		}
		state.CurrentBookingID = 0
		state.CurrentUsageLogID = 0
		state.CurrentUserID = nil
		state.CurrentUserName = nil
		state.SessionEndsAt = nil
	}
	w.hub.Mu.Unlock()
	w.hub.BroadcastStateChange(expired.compName)
}
