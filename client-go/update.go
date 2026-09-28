package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const maxAgentUpdateBytes = 16 << 20

type agentUpdateManifest struct {
	Version   string `json:"version"`
	SHA256    string `json:"sha256"`
	FileSize  int64  `json:"file_size"`
	CanUpdate bool   `json:"can_update"`
}

func checkForAgentUpdate(ctx context.Context, cfg *Config, hwid string) (bool, error) {
	endpoint, err := stationAPIURL(cfg.ServerURL, "/api/agent/update")
	if err != nil {
		return false, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	setAgentUpdateHeaders(request, cfg, hwid)
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNoContent {
		return false, nil
	}
	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("update manifest returned HTTP %d", response.StatusCode)
	}
	var release agentUpdateManifest
	if err := json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&release); err != nil {
		return false, err
	}
	if release.Version == agentVersion {
		return false, nil
	}
	if !release.CanUpdate {
		return false, nil
	}
	if release.Version == "" || len(release.Version) > 32 || len(release.SHA256) != 64 || strings.Trim(release.SHA256, "0123456789abcdef") != "" || release.FileSize < 1024 || release.FileSize > maxAgentUpdateBytes {
		return false, fmt.Errorf("server returned invalid Agent update metadata")
	}
	if failedUpdateSHA() == release.SHA256 {
		return false, fmt.Errorf("Agent update %s failed previously; see update.log or publish a new version", release.Version)
	}
	return downloadAndStartUpdate(ctx, client, cfg, hwid, release)
}

func setAgentUpdateHeaders(request *http.Request, cfg *Config, hwid string) {
	request.Header.Set("X-Computer-Name", cfg.MachineName)
	request.Header.Set("X-Computer-HWID", hwid)
	request.Header.Set("X-Agent-Secret", cfg.AgentSecret)
	request.Header.Set("X-Agent-Version", agentVersion)
	if failed := failedUpdateSHA(); failed != "" {
		request.Header.Set("X-Agent-Update-Failed-SHA", failed)
	}
}

func failedUpdateSHA() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(executable), "AUCCAgent-update-failed.txt"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func downloadAndStartUpdate(ctx context.Context, client *http.Client, cfg *Config, hwid string, release agentUpdateManifest) (bool, error) {
	executable, err := os.Executable()
	if err != nil {
		return false, err
	}
	installDir := filepath.Dir(executable)
	helper := filepath.Join(installDir, "AUCCUpdater.exe")
	if _, err := os.Stat(helper); err != nil {
		return false, fmt.Errorf("AUCCUpdater.exe missing; install the bootstrap version once")
	}
	endpoint, err := stationAPIURL(cfg.ServerURL, "/api/agent/releases/"+release.SHA256+"/file")
	if err != nil {
		return false, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	setAgentUpdateHeaders(request, cfg, hwid)
	downloadClient := *client
	downloadClient.Timeout = 2 * time.Minute
	response, err := downloadClient.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("Agent EXE download returned HTTP %d", response.StatusCode)
	}
	temp, err := os.CreateTemp(installDir, "AUCCAgent-update-*.exe")
	if err != nil {
		return false, fmt.Errorf("cannot stage Agent update: %w", err)
	}
	removeStage := true
	defer func() {
		if removeStage {
			_ = os.Remove(temp.Name())
		}
	}()
	digest := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(temp, digest), io.LimitReader(response.Body, maxAgentUpdateBytes+1))
	closeErr := temp.Close()
	if copyErr != nil || closeErr != nil || size != release.FileSize || hex.EncodeToString(digest.Sum(nil)) != release.SHA256 {
		return false, fmt.Errorf("Agent EXE size or SHA-256 does not match the published release")
	}
	preflightCtx, stop := context.WithTimeout(ctx, 10*time.Second)
	defer stop()
	output, err := exec.CommandContext(preflightCtx, temp.Name(), "--version").Output()
	if err != nil || strings.TrimSpace(string(output)) != release.Version {
		_ = os.WriteFile(filepath.Join(installDir, "AUCCAgent-update-failed.txt"), []byte(release.SHA256), 0600)
		return false, fmt.Errorf("Agent EXE version check failed")
	}
	command := exec.Command(helper, executable, temp.Name(), release.Version, release.SHA256)
	command.Dir = installDir
	if err := command.Start(); err != nil {
		return false, fmt.Errorf("cannot launch Agent updater: %w", err)
	}
	removeStage = false
	_ = command.Process.Release()
	return true, nil
}
