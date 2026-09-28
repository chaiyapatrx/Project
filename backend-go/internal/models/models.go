package models

import (
	"time"
)

type User struct {
	ID           int       `db:"id" json:"id"`
	Username     string    `db:"username" json:"username"`
	PasswordHash string    `db:"password_hash" json:"-"`
	FullName     string    `db:"full_name" json:"full_name"`
	Role         string    `db:"role" json:"role"` // admin, staff, executive, student
	Department   *string   `db:"department" json:"department"`
	UserType     *string   `db:"user_type" json:"user_type"`
	IsActive     bool      `db:"is_active" json:"is_active"`
	TokenVersion int       `db:"token_version" json:"-"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

type Computer struct {
	ID                 int       `db:"id" json:"id"`
	Name               string    `db:"name" json:"name"`
	HWID               *string   `db:"hwid" json:"hwid"`
	IPAddress          *string   `db:"ip_address" json:"ip_address"`
	MacAddress         *string   `db:"mac_address" json:"mac_address"`
	Status             string    `db:"status" json:"status"` // available, in_use, maintenance, disabled
	IsActive           bool      `db:"is_active" json:"is_active"`
	AgentVersion       *string   `db:"agent_version" json:"agent_version"`
	AgentUpdateError   *string   `db:"agent_update_error" json:"agent_update_error"`
	CreatedAt          time.Time `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time `db:"updated_at" json:"updated_at"`
	AgentKeyConfigured bool      `db:"agent_key_configured" json:"agent_key_configured"`

	// In-memory runtime state (populated by Go Hub)
	IsOnline        bool       `db:"-" json:"is_online"`
	CurrentUserID   *int       `db:"-" json:"current_user_id"`
	CurrentUserName *string    `db:"-" json:"current_user_name"`
	SessionEndsAt   *time.Time `db:"-" json:"session_ends_at"`
}

type Booking struct {
	ID         int       `db:"id" json:"id"`
	UserID     int       `db:"user_id" json:"user_id"`
	ComputerID int       `db:"computer_id" json:"computer_id"`
	StartTime  time.Time `db:"start_time" json:"start_time"`
	EndTime    time.Time `db:"end_time" json:"end_time"`
	AccessCode *string   `db:"access_code" json:"access_code"`
	Status     string    `db:"status" json:"status"` // pending, active, completed, cancelled, expired
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`

	// Joins for API responses
	UserName     string `db:"user_name" json:"user_name,omitempty"`
	UserUsername string `db:"user_username" json:"user_username,omitempty"`
	ComputerName string `db:"computer_name" json:"computer_name,omitempty"`
}

type UsageLog struct {
	ID                int        `db:"id" json:"id"`
	UserID            int        `db:"user_id" json:"user_id"`
	ComputerID        int        `db:"computer_id" json:"computer_id"`
	BookingID         *int       `db:"booking_id" json:"booking_id"`
	StartTime         time.Time  `db:"start_time" json:"start_time"`
	EndTime           *time.Time `db:"end_time" json:"end_time"`
	SessionEndsAt     *time.Time `db:"session_ends_at" json:"session_ends_at,omitempty"`
	DurationMinutes   int        `db:"duration_minutes" json:"duration_minutes"`
	TerminationReason string     `db:"termination_reason" json:"termination_reason"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`

	// Joins for API responses
	UserName     string `db:"user_name" json:"user_name,omitempty"`
	ComputerName string `db:"computer_name" json:"computer_name,omitempty"`
	Department   string `db:"department" json:"department,omitempty"`
}

type SystemSetting struct {
	SettingKey   string    `db:"setting_key" json:"setting_key"`
	SettingValue string    `db:"setting_value" json:"setting_value"`
	Description  *string   `db:"description" json:"description"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

type AuditLog struct {
	ID         int       `db:"id" json:"id"`
	UserID     *int      `db:"user_id" json:"user_id"`
	Action     string    `db:"action" json:"action"`
	TargetType *string   `db:"target_type" json:"target_type"`
	TargetID   *string   `db:"target_id" json:"target_id"`
	IPAddress  *string   `db:"ip_address" json:"ip_address"`
	Details    *string   `db:"details" json:"details"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}
