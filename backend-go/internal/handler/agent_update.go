package handler

import (
	"crypto/sha256"
	"database/sql"
	"debug/pe"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"station-backend/internal/auth"
)

const maxAgentReleaseBytes = 16 << 20

var agentReleaseVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

type agentRelease struct {
	ID        int       `db:"id" json:"id"`
	Version   string    `db:"version" json:"version"`
	SHA256    string    `db:"sha256" json:"sha256"`
	FileSize  int64     `db:"file_size" json:"file_size"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type AgentUpdateHandler struct {
	db  *sqlx.DB
	dir string
}

func NewAgentUpdateHandler(db *sqlx.DB, dir string) (*AgentUpdateHandler, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &AgentUpdateHandler{db: db, dir: dir}, nil
}

func (h *AgentUpdateHandler) Upload(c *gin.Context) {
	version := strings.TrimSpace(c.PostForm("version"))
	if !agentReleaseVersion.MatchString(version) || len(version) > 32 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Version must be MAJOR.MINOR.PATCH"})
		return
	}
	file, err := c.FormFile("file")
	if err != nil || file.Size < 1024 || file.Size > maxAgentReleaseBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Select an Agent EXE smaller than 16 MB"})
		return
	}
	source, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot read Agent EXE"})
		return
	}
	defer source.Close()
	temp, err := os.CreateTemp(h.dir, "upload-*.tmp")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot stage Agent EXE"})
		return
	}
	defer os.Remove(temp.Name())
	digest := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(temp, digest), io.LimitReader(source, maxAgentReleaseBytes+1))
	closeErr := temp.Close()
	if copyErr != nil || closeErr != nil || size != file.Size || size > maxAgentReleaseBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Agent EXE upload is incomplete"})
		return
	}
	image, err := pe.Open(temp.Name())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Upload a Windows Agent EXE"})
		return
	}
	machine := image.FileHeader.Machine
	_ = image.Close()
	if machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Agent EXE must target Windows x64"})
		return
	}
	sha := hex.EncodeToString(digest.Sum(nil))
	target := filepath.Join(h.dir, sha+".exe")
	if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) {
		if err := os.Rename(temp.Name(), target); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot publish Agent EXE"})
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot inspect Agent EXE storage"})
		return
	}
	result, err := h.db.Exec("INSERT INTO agent_releases (version, sha256, file_size) VALUES (?, ?, ?)", version, sha, size)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Version or Agent EXE already uploaded"})
		return
	}
	id, _ := result.LastInsertId()
	recordAudit(c, h.db, "agent_release_uploaded", "agent_release", strconv.FormatInt(id, 10), "Uploaded Agent version "+version)
	c.JSON(http.StatusCreated, gin.H{"id": id, "version": version, "sha256": sha, "file_size": size})
}

func (h *AgentUpdateHandler) List(c *gin.Context) {
	releases := make([]agentRelease, 0)
	if err := h.db.Select(&releases, "SELECT id, version, sha256, file_size, created_at FROM agent_releases ORDER BY id DESC"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot list Agent versions"})
		return
	}
	active, err := h.activeRelease()
	if err != nil && err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot read active Agent version"})
		return
	}
	activeID := 0
	if err == nil {
		activeID = active.ID
	}
	c.JSON(http.StatusOK, gin.H{"releases": releases, "active_id": activeID})
}

func (h *AgentUpdateHandler) Activate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Agent version ID"})
		return
	}
	var release agentRelease
	if err := h.db.Get(&release, "SELECT id, version, sha256, file_size, created_at FROM agent_releases WHERE id = ?", id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent version not found"})
		return
	}
	if _, err := os.Stat(filepath.Join(h.dir, release.SHA256+".exe")); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Agent EXE missing from server storage"})
		return
	}
	if _, err := h.db.Exec(`INSERT INTO system_settings (setting_key, setting_value) VALUES ('agent_release_id', ?)
		ON DUPLICATE KEY UPDATE setting_value = VALUES(setting_value)`, strconv.Itoa(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot activate Agent version"})
		return
	}
	recordAudit(c, h.db, "agent_release_activated", "agent_release", strconv.Itoa(id), "Activated Agent version "+release.Version)
	c.JSON(http.StatusOK, gin.H{"version": release.Version, "active": true})
}

func (h *AgentUpdateHandler) Pause(c *gin.Context) {
	if _, err := h.db.Exec("DELETE FROM system_settings WHERE setting_key = 'agent_release_id'"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot pause Agent updates"})
		return
	}
	recordAudit(c, h.db, "agent_release_paused", "agent_release", "", "Paused Agent rollout")
	c.JSON(http.StatusOK, gin.H{"active": false})
}

func (h *AgentUpdateHandler) activeRelease() (agentRelease, error) {
	var release agentRelease
	err := h.db.Get(&release, `SELECT r.id, r.version, r.sha256, r.file_size, r.created_at
		FROM agent_releases r JOIN system_settings s ON s.setting_key = 'agent_release_id'
		AND s.setting_value = CAST(r.id AS CHAR) LIMIT 1`)
	return release, err
}

func (h *AgentUpdateHandler) authenticateAgent(c *gin.Context) bool {
	name, hwid, secret := c.GetHeader("X-Computer-Name"), c.GetHeader("X-Computer-HWID"), c.GetHeader("X-Agent-Secret")
	if name == "" || len(name) > 50 || hwid == "" || len(hwid) > 100 || len(secret) < 32 || len(secret) > 256 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized Agent"})
		return false
	}
	var station struct {
		HWID            sql.NullString `db:"hwid"`
		AgentSecretHash sql.NullString `db:"agent_secret_hash"`
	}
	if err := h.db.Get(&station, "SELECT hwid, agent_secret_hash FROM computers WHERE name = ? AND is_active = 1 AND status <> 'disabled'", name); err != nil || !station.HWID.Valid || station.HWID.String != hwid || !station.AgentSecretHash.Valid || !auth.VerifyAgentSecret(secret, station.AgentSecretHash.String) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized Agent"})
		return false
	}
	return true
}

func (h *AgentUpdateHandler) Manifest(c *gin.Context) {
	if !h.authenticateAgent(c) {
		return
	}
	version := c.GetHeader("X-Agent-Version")
	if agentReleaseVersion.MatchString(version) && len(version) <= 32 {
		_, _ = h.db.Exec("UPDATE computers SET agent_version = ? WHERE name = ? AND hwid = ?", version, c.GetHeader("X-Computer-Name"), c.GetHeader("X-Computer-HWID"))
	}
	release, err := h.activeRelease()
	if err == sql.ErrNoRows {
		_, _ = h.db.Exec("UPDATE computers SET agent_update_error = NULL WHERE name = ? AND hwid = ?", c.GetHeader("X-Computer-Name"), c.GetHeader("X-Computer-HWID"))
		c.Status(http.StatusNoContent)
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot read Agent version"})
		return
	}
	var updateError any
	if c.GetHeader("X-Agent-Update-Failed-SHA") == release.SHA256 {
		updateError = "Automatic update failed; inspect update.log on this station"
	}
	_, _ = h.db.Exec("UPDATE computers SET agent_update_error = ? WHERE name = ? AND hwid = ?", updateError, c.GetHeader("X-Computer-Name"), c.GetHeader("X-Computer-HWID"))
	var stationStatus string
	if err := h.db.Get(&stationStatus, "SELECT status FROM computers WHERE name = ? AND hwid = ? AND is_active = 1", c.GetHeader("X-Computer-Name"), c.GetHeader("X-Computer-HWID")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot check station state"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"version": release.Version, "sha256": release.SHA256, "file_size": release.FileSize, "can_update": stationStatus != "in_use"})
}

func (h *AgentUpdateHandler) Download(c *gin.Context) {
	if !h.authenticateAgent(c) {
		return
	}
	sha := c.Param("sha")
	if len(sha) != 64 || strings.Trim(sha, "0123456789abcdef") != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Agent release"})
		return
	}
	var release agentRelease
	if err := h.db.Get(&release, "SELECT id, version, sha256, file_size, created_at FROM agent_releases WHERE sha256 = ?", sha); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent release not found"})
		return
	}
	file := filepath.Join(h.dir, sha+".exe")
	if _, err := os.Stat(file); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent EXE missing"})
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=AUCCAgent.exe")
	c.File(file)
}
