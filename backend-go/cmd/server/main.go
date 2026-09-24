package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"station-backend/internal/config"
	"station-backend/internal/database"
	"station-backend/internal/handler"
	"station-backend/internal/hub"
	"station-backend/internal/middleware"
	"station-backend/internal/models"
	"station-backend/internal/worker"
)

func main() {
	log.Println("==================================================")
	log.Println(" Starting High-Performance Go Backend (MySQL + WS)")
	log.Println("==================================================")

	// 1. Load Configuration
	cfg := config.LoadConfig()

	// 2. Initialize Database Connection Pool
	db := database.InitDB(cfg)
	defer db.Close()

	// 3. Initialize In-Memory Hub (commands to agents are signed with AgentSecret)
	h := hub.InitHub()

	// 4. Pre-load computers into Hub from DB preserving real status and active sessions
	var comps []models.Computer
	if err := db.Select(&comps, "SELECT id, name, status FROM computers WHERE is_active = 1"); err != nil {
		log.Fatalf("[Hub] Failed to preload computers: %v", err)
	}
	for _, c := range comps {
		h.Computers[c.Name] = &hub.ComputerRuntimeState{
			ID:       c.ID,
			Name:     c.Name,
			Status:   c.Status,
			IsOnline: false,
		}
	}
	log.Printf("[Hub] Preloaded %d computers into memory", len(comps))

	// Restore any ongoing active bookings into memory state
	type activeBookingInfo struct {
		ComputerName string    `db:"computer_name"`
		UserID       int       `db:"user_id"`
		UserName     string    `db:"user_name"`
		EndTime      time.Time `db:"end_time"`
	}
	var activeBookings []activeBookingInfo
	if err := db.Select(&activeBookings, `
		SELECT c.name as computer_name, b.user_id, u.username as user_name, b.end_time
		FROM bookings b
		JOIN computers c ON c.id = b.computer_id
		JOIN users u ON u.id = b.user_id
		WHERE b.status = 'active' AND b.end_time > NOW()`); err != nil {
		log.Fatalf("[Hub] Failed to restore active sessions: %v", err)
	}
	for _, ab := range activeBookings {
		if state, exists := h.Computers[ab.ComputerName]; exists {
			state.Status = "in_use"
			state.CurrentUserID = &ab.UserID
			userNameCopy := ab.UserName
			state.CurrentUserName = &userNameCopy
			endTimeCopy := ab.EndTime
			state.SessionEndsAt = &endTimeCopy
		}
	}
	if len(activeBookings) > 0 {
		log.Printf("[Hub] Restored %d active booking sessions into memory", len(activeBookings))
	}

	// 5. Start Real-time Session Countdown Worker
	stopSessionWorker := worker.StartSessionWorker(db, h)
	defer stopSessionWorker()

	// 6. Router Setup
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), middleware.SecurityHeaders(), middleware.LimitRequestBody(1<<20))

	// Trusted proxies: only honor client-IP headers (X-Forwarded-For, etc.)
	// from explicitly trusted proxies. Empty list => trust none, so ClientIP()
	// falls back to the real remote address and cannot be spoofed.
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		log.Fatalf("[Server] Failed to set trusted proxies: %v", err)
	}

	// CORS Configuration: Explicit origins only with AllowCredentials
	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Agent-Secret", "X-CSRF-Token"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Handlers
	authH := handler.NewAuthHandler(db, cfg)
	compH := handler.NewComputerHandler(db, h)
	bookH := handler.NewBookingHandler(db, h)
	settH := handler.NewSettingsHandler(db)
	wsH := handler.NewWSHandler(h, cfg, db)
	loginRateLimit := middleware.RateLimitByIP(60, time.Minute)
	registrationRateLimit := middleware.RateLimitByIP(10, time.Hour)
	agentRateLimit := middleware.RateLimitByIP(120, time.Minute)

	// Unauthenticated health endpoints expose status only; readiness checks the database.
	r.GET("/health/live", middleware.Liveness())
	r.GET("/health/ready", middleware.Readiness(db))

	// --- Public Routes ---
	r.POST("/register", registrationRateLimit, authH.Register)
	r.POST("/token", loginRateLimit, authH.Login) // OAuth2 password compatible
	r.POST("/api/auth/login", loginRateLimit, authH.Login)
	r.POST("/api/auth/register", registrationRateLimit, authH.Register)
	r.GET("/computers", compH.ListComputers) // Public for guest/overview mode

	// --- WebSockets ---
	r.GET("/api/ws/agent", agentRateLimit, wsH.AgentWS) // Client Agent endpoint
	r.GET("/api/ws/monitor", wsH.MonitorWS)             // React Frontend endpoint

	// --- Protected Routes ---
	api := r.Group("")
	api.Use(middleware.AuthMiddleware(cfg, db))
	api.Use(middleware.CSRFMiddleware())
	{
		api.POST("/api/auth/logout", authH.Logout)
		api.GET("/users/me", authH.GetMe)
		api.GET("/api/auth/me", authH.GetMe)
		api.POST("/api/auth/change-password", authH.ChangePassword)
		api.GET("/users", middleware.RequireRoles("admin"), authH.ListUsers)
		api.POST("/admin/users", middleware.RequireRoles("admin"), authH.AdminCreateUser)
		api.PUT("/admin/users/:id/info", middleware.RequireRoles("admin"), authH.UpdateUserInfo)
		api.PUT("/admin/users/:id/reset-password", middleware.RequireRoles("admin"), authH.ResetUserPassword)
		api.DELETE("/admin/users/:id", middleware.RequireRoles("admin"), authH.DeleteUser)

		// Computers Operations
		api.GET("/admin/computers", middleware.RequireRoles("admin", "staff"), compH.ListComputersAdmin)
		api.POST("/computers", middleware.RequireRoles("admin"), compH.CreateComputer)
		api.POST("/api/admin/computers/:id/agent-secret", middleware.RequireRoles("admin"), compH.RotateAgentSecret)
		api.PUT("/computers/:id", middleware.RequireRoles("admin", "staff"), compH.UpdateComputer)
		api.DELETE("/computers/:id", middleware.RequireRoles("admin"), compH.DeleteComputer)
		api.POST("/api/admin/computers/:id/command", middleware.RequireRoles("admin", "staff"), compH.SendCommand)
		api.POST("/api/admin/computers/broadcast", middleware.RequireRoles("admin", "staff"), compH.BroadcastCommand)

		// Bookings
		api.POST("/bookings", bookH.CreateBooking)
		api.GET("/my-bookings", bookH.GetMyBookings)
		api.DELETE("/bookings/:id", bookH.CancelBooking)
		api.POST("/bookings/:id/extend", bookH.ExtendBooking)
		api.GET("/admin/bookings", middleware.RequireRoles("admin", "staff"), bookH.ListAllBookings)

		// Settings & Analytics
		api.GET("/api/admin/settings", middleware.RequireRoles("admin", "staff"), settH.GetSettings)
		api.PUT("/api/admin/settings", middleware.RequireRoles("admin"), settH.UpdateSettings)
		api.GET("/admin/usage-history", middleware.RequireRoles("admin", "staff", "executive"), settH.GetUsageHistory)
		api.GET("/dashboard/stats", middleware.RequireRoles("admin", "staff", "executive"), settH.GetAnalyticsSummary)
		api.GET("/api/admin/analytics/summary", middleware.RequireRoles("admin", "staff", "executive"), settH.GetAnalyticsSummary)
	}

	// 7. Graceful Server Startup & Shutdown
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12},
	}

	go func() {
		serve := srv.ListenAndServe
		protocol := "http"
		if cfg.TLSCertFile != "" {
			serve = func() error { return srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile) }
			protocol = "https"
		}
		log.Printf("[Server] Go Backend listening at %s://%s", protocol, cfg.ListenAddr)
		if err := serve(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Server] Listen error: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[Server] Shutting down Go Backend gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("[Server] Forced shutdown:", err)
	}

	log.Println("[Server] Go Backend exited cleanly.")
}
