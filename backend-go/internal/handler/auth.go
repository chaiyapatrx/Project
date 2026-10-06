package handler

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"station-backend/internal/auth"
	"station-backend/internal/config"
	"station-backend/internal/middleware"
	"station-backend/internal/models"
)

type AuthHandler struct {
	db  *sqlx.DB
	cfg *config.Config
}

func NewAuthHandler(db *sqlx.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{db: db, cfg: cfg}
}

type LoginRequest struct {
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=15,max=128"`
	FullName string `json:"full_name" binding:"required,max=100"`
	// Role is intentionally omitted: public self-registration always creates a
	// student account (enforced server-side). Never trust a client-supplied role.
	Department *string `json:"department" binding:"omitempty,max=100"`
	UserType   *string `json:"user_type" binding:"omitempty,max=50"`
}

func passwordLengthValid(password string) bool {
	return utf8.RuneCountInString(password) >= 15 && len(password) <= 72
}

func (h *AuthHandler) Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	if username == "" || password == "" {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err == nil {
			username = req.Username
			password = req.Password
		}
	}

	username = strings.TrimSpace(username)
	if username == "" || utf8.RuneCountInString(username) > 50 || password == "" || len(password) > 72 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username and password required"})
		return
	}

	var user models.User
	err := h.db.Get(&user, "SELECT * FROM users WHERE username = ? AND is_active = 1", username)
	if err == sql.ErrNoRows {
		// Match the bcrypt cost for an existing account without revealing whether
		// the submitted username exists through a fast rejection.
		_ = auth.CheckPasswordHash(password, auth.DummyPasswordHash)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Incorrect username or password"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	if !middleware.AccountLoginAllowed(c, user.ID) {
		return
	}

	// Password validation strictly using Bcrypt
	isMatched := auth.CheckPasswordHash(password, user.PasswordHash)
	if !isMatched {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Incorrect username or password"})
		return
	}

	token, err := auth.GenerateToken(&user, h.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	csrf, err := generateCSRFToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to establish session"})
		return
	}

	// Set the token in an HttpOnly cookie (primary, XSS-resistant transport)
	// plus a readable CSRF cookie for the double-submit pattern.
	setAuthCookies(c, h.cfg, token, csrf, h.cfg.JWTExpiresIn*60)

	response := gin.H{
		"csrf_token": csrf,
		"user": gin.H{
			"id":         user.ID,
			"username":   user.Username,
			"full_name":  user.FullName,
			"role":       user.Role,
			"department": user.Department,
		},
	}
	// The browser SPA uses the HttpOnly cookie. Keep bearer tokens only on the
	// OAuth-compatible endpoint for non-browser clients.
	if c.FullPath() == "/token" {
		response["access_token"] = token
		response["token_type"] = "bearer"
	}
	c.Set("user_id", user.ID)
	recordAudit(c, h.db, "login_succeeded", "user", strconv.Itoa(user.ID), "Session created")
	c.JSON(http.StatusOK, response)
}

// Logout invalidates all tokens for this account and clears the session cookies.
func (h *AuthHandler) Logout(c *gin.Context) {
	if _, err := h.db.Exec("UPDATE users SET token_version = token_version + 1 WHERE id = ?", c.GetInt("user_id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke session"})
		return
	}
	clearAuthCookies(c, h.cfg)
	recordAudit(c, h.db, "logout", "user", strconv.Itoa(c.GetInt("user_id")), "Sessions revoked")
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func (h *AuthHandler) Register(c *gin.Context) {
	if !h.cfg.SelfRegistrationEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "Self-registration is disabled"})
		return
	}
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !passwordLengthValid(req.Password) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 15 characters and at most 72 bytes"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.FullName = strings.TrimSpace(req.FullName)
	if utf8.RuneCountInString(req.Username) < 3 || req.FullName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username and full name are required"})
		return
	}

	// Check if username already exists
	var count int
	if err := h.db.Get(&count, "SELECT COUNT(*) FROM users WHERE username = ?", req.Username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check username"})
		return
	}
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username already exists"})
		return
	}

	// Public self-registration is strictly restricted to student role
	role := "student"

	hashed, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	res, err := h.db.Exec(`
		INSERT INTO users (username, password_hash, full_name, role, department, user_type)
		VALUES (?, ?, ?, ?, ?, ?)`,
		req.Username, hashed, req.FullName, role, req.Department, req.UserType,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		return
	}

	id, _ := res.LastInsertId()
	c.Set("user_id", int(id))
	recordAudit(c, h.db, "self_registration", "user", auditID(id), "Student account created")
	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"user_id": id,
	})
}

type AdminCreateUserRequest struct {
	Username   string  `json:"username" binding:"required,min=3,max=50"`
	Password   string  `json:"password" binding:"required,min=15,max=128"`
	FullName   string  `json:"full_name" binding:"required,max=100"`
	Role       string  `json:"role" binding:"required"` // admin, staff, executive, student
	Department *string `json:"department" binding:"omitempty,max=100"`
	UserType   *string `json:"user_type" binding:"omitempty,max=50"`
}

func (h *AuthHandler) AdminCreateUser(c *gin.Context) {
	var req AdminCreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !passwordLengthValid(req.Password) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 15 characters and at most 72 bytes"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.FullName = strings.TrimSpace(req.FullName)
	if utf8.RuneCountInString(req.Username) < 3 || req.FullName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username and full name are required"})
		return
	}

	validRoles := map[string]bool{"admin": true, "staff": true, "executive": true, "student": true}
	if !validRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role. Must be admin, staff, executive, or student"})
		return
	}

	var count int
	if err := h.db.Get(&count, "SELECT COUNT(*) FROM users WHERE username = ?", req.Username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check username"})
		return
	}
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username already exists"})
		return
	}

	hashed, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	res, err := h.db.Exec(`
		INSERT INTO users (username, password_hash, full_name, role, department, user_type)
		VALUES (?, ?, ?, ?, ?, ?)`,
		req.Username, hashed, req.FullName, req.Role, req.Department, req.UserType,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	id, _ := res.LastInsertId()
	recordAudit(c, h.db, "user_created", "user", auditID(id), "Account created by administrator")
	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully",
		"user_id": id,
		"role":    req.Role,
	})
}

func (h *AuthHandler) GetMe(c *gin.Context) {
	userID := c.GetInt("user_id")

	var user models.User
	err := h.db.Get(&user, "SELECT id, username, full_name, role, department, user_type, created_at FROM users WHERE id = ?", userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) ListUsers(c *gin.Context) {
	users := make([]models.User, 0)
	err := h.db.Select(&users, "SELECT id, username, full_name, role, department, user_type, is_active, created_at FROM users ORDER BY id ASC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	c.JSON(http.StatusOK, users)
}

func (h *AuthHandler) UpdateUserInfo(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	var req struct {
		Role       *string `json:"role" binding:"omitempty,oneof=admin staff executive student"`
		Department *string `json:"department" binding:"omitempty,max=100"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Role == nil && req.Department == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one user field must be supplied"})
		return
	}
	if req.Role != nil {
		validRoles := map[string]bool{"admin": true, "staff": true, "executive": true, "student": true}
		if !validRoles[*req.Role] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role"})
			return
		}
	}

	tx, err := h.db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}
	defer tx.Rollback()
	var currentRole string
	if err := tx.Get(&currentRole, "SELECT role FROM users WHERE id = ? AND is_active = 1 FOR UPDATE", userID); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found or inactive"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load user"})
		}
		return
	}
	if req.Role != nil && currentRole == "admin" && *req.Role != "admin" {
		if userID == c.GetInt("user_id") {
			c.JSON(http.StatusConflict, gin.H{"error": "You cannot remove your own administrator role"})
			return
		}
		admins, err := lockActiveAdmins(tx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify administrator accounts"})
			return
		}
		if len(admins) < 2 {
			c.JSON(http.StatusConflict, gin.H{"error": "The last active administrator cannot be demoted"})
			return
		}
	}
	if _, err := tx.Exec(`
		UPDATE users
		SET role = COALESCE(?, role), department = COALESCE(?, department), token_version = token_version + 1
		WHERE id = ? AND is_active = 1`, req.Role, req.Department, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}
	recordAudit(c, h.db, "user_updated", "user", strconv.Itoa(userID), "Role or department changed; sessions revoked")

	c.JSON(http.StatusOK, gin.H{"message": "User info updated successfully"})
}

func (h *AuthHandler) ResetUserPassword(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	var req struct {
		NewPassword string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil || !passwordLengthValid(req.NewPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 15 characters and at most 72 bytes"})
		return
	}

	hashed, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	res, err := h.db.Exec("UPDATE users SET password_hash = ?, token_version = token_version + 1 WHERE id = ? AND is_active = 1", hashed, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset password"})
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found or inactive"})
		return
	}
	recordAudit(c, h.db, "password_reset", "user", strconv.Itoa(userID), "Password reset by administrator; sessions revoked")

	c.JSON(http.StatusOK, gin.H{"message": "User password reset successfully"})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	currentUserID := c.GetInt("user_id")
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil || !passwordLengthValid(req.NewPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "New password must be at least 15 characters and at most 72 bytes"})
		return
	}

	var user models.User
	err := h.db.Get(&user, "SELECT * FROM users WHERE id = ? AND is_active = 1", currentUserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if !middleware.AccountLoginAllowed(c, user.ID) {
		return
	}
	if user.TokenVersion != c.GetInt("token_version") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Session was revoked; sign in again"})
		return
	}
	if !auth.CheckPasswordHash(req.OldPassword, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password is incorrect"})
		return
	}

	hashed, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// A password reset, logout, role change, or deactivation after verification
	// invalidates this request. Never overwrite a newer security decision.
	result, err := h.db.Exec(`UPDATE users SET password_hash = ?, token_version = token_version + 1
		WHERE id = ? AND is_active = 1 AND password_hash = ? AND token_version = ?`,
		hashed, currentUserID, user.PasswordHash, user.TokenVersion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}
	changed, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify password update"})
		return
	}
	if changed != 1 {
		c.JSON(http.StatusConflict, gin.H{"error": "Account changed during password update; sign in again"})
		return
	}
	recordAudit(c, h.db, "password_changed", "user", strconv.Itoa(currentUserID), "Password changed; sessions revoked")

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

func (h *AuthHandler) DeleteUser(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	currentUserID := c.GetInt("user_id")

	// Prevent admin from deleting themselves
	if userID == currentUserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete your own admin account"})
		return
	}

	tx, err := h.db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}
	defer tx.Rollback()
	var role string
	if err := tx.Get(&role, "SELECT role FROM users WHERE id = ? AND is_active = 1 FOR UPDATE", userID); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found or already inactive"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load user"})
		}
		return
	}
	if role == "admin" {
		admins, err := lockActiveAdmins(tx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify administrator accounts"})
			return
		}
		if len(admins) < 2 {
			c.JSON(http.StatusConflict, gin.H{"error": "The last active administrator cannot be deactivated"})
			return
		}
	}
	var activeBookings int
	if err := tx.Get(&activeBookings, "SELECT COUNT(*) FROM bookings WHERE user_id = ? AND status IN ('pending', 'active')", userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check active bookings"})
		return
	}
	if activeBookings > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "End the user's active bookings before deactivating the account"})
		return
	}
	var activeWalkInSessions int
	if err := tx.Get(&activeWalkInSessions, "SELECT COUNT(*) FROM usage_logs WHERE user_id = ? AND booking_id IS NULL AND end_time IS NULL", userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check active station sessions"})
		return
	}
	if activeWalkInSessions > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "End the user's active station sessions before deactivating the account"})
		return
	}
	if _, err := tx.Exec("UPDATE users SET is_active = 0, token_version = token_version + 1 WHERE id = ? AND is_active = 1", userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deactivate user"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deactivate user"})
		return
	}
	recordAudit(c, h.db, "user_deactivated", "user", strconv.Itoa(userID), "Account deactivated; sessions revoked")

	c.JSON(http.StatusOK, gin.H{"message": "User deactivated successfully"})
}

func lockActiveAdmins(tx *sqlx.Tx) ([]int, error) {
	var adminIDs []int
	err := tx.Select(&adminIDs, "SELECT id FROM users WHERE role = 'admin' AND is_active = 1 ORDER BY id FOR UPDATE")
	return adminIDs, err
}
