package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if len(os.Args) != 5 {
		return
	}
	current, staged, version, expectedSHA := os.Args[1], os.Args[2], os.Args[3], os.Args[4]
	if !strings.EqualFold(filepath.Base(current), "AUCCAgent.exe") || filepath.Dir(current) != filepath.Dir(staged) || strings.EqualFold(current, staged) || len(expectedSHA) != 64 || strings.Trim(expectedSHA, "0123456789abcdef") != "" {
		return
	}
	defer os.Remove(staged)
	logPath := filepath.Join(filepath.Dir(current), "update.log")
	if file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); err == nil {
		defer file.Close()
		log.SetOutput(file)
	}
	marker := filepath.Join(filepath.Dir(current), "AUCCAgent-update-failed.txt")
	if err := replaceAgent(current, staged, version, expectedSHA); err != nil {
		_ = os.WriteFile(marker, []byte(expectedSHA), 0600)
		log.Printf("Agent update failed: %v", err)
	} else {
		_ = os.Remove(marker)
	}
}

func replaceAgent(current, staged, version, expectedSHA string) error {
	digest, err := fileSHA256(staged)
	if err != nil || digest != expectedSHA {
		return restartCurrent(current, fmt.Errorf("staged Agent SHA-256 mismatch"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	check := exec.CommandContext(ctx, staged, "--version")
	output, err := check.Output()
	if err != nil || strings.TrimSpace(string(output)) != version {
		return restartCurrent(current, fmt.Errorf("staged Agent version mismatch"))
	}
	backup, err := os.CreateTemp(filepath.Dir(current), "AUCCAgent-backup-*.exe")
	if err != nil {
		return restartCurrent(current, err)
	}
	backupPath := backup.Name()
	old, err := os.Open(current)
	if err != nil {
		_ = backup.Close()
		_ = os.Remove(backupPath)
		return restartCurrent(current, err)
	}
	_, copyErr := io.Copy(backup, old)
	_ = old.Close()
	closeErr := backup.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(backupPath)
		return restartCurrent(current, fmt.Errorf("cannot back up installed Agent"))
	}
	if err := replaceWithRetry(staged, current); err != nil {
		_ = os.Remove(backupPath)
		return restartCurrent(current, fmt.Errorf("cannot replace installed Agent: %w", err))
	}
	command := exec.Command(current)
	command.Dir = filepath.Dir(current)
	if err := command.Start(); err != nil {
		return restoreAgent(backupPath, current, err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		return restoreAgent(backupPath, current, fmt.Errorf("new Agent exited within 15 seconds: %v", err))
	case <-time.After(15 * time.Second):
		_ = os.Remove(backupPath)
		log.Printf("Agent version %s started successfully", version)
		return nil
	}
}

func restoreAgent(backup, current string, cause error) error {
	if err := replaceWithRetry(backup, current); err != nil {
		return fmt.Errorf("%v; rollback failed: %w", cause, err)
	}
	command := exec.Command(current)
	command.Dir = filepath.Dir(current)
	if err := command.Start(); err != nil {
		return fmt.Errorf("%v; old Agent could not restart: %w", cause, err)
	}
	_ = command.Process.Release()
	return cause
}

func restartCurrent(current string, cause error) error {
	command := exec.Command(current)
	command.Dir = filepath.Dir(current)
	if err := command.Start(); err != nil {
		return fmt.Errorf("%v; installed Agent could not restart: %w", cause, err)
	}
	_ = command.Process.Release()
	return cause
}

func replaceWithRetry(source, target string) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		err := os.Rename(source, target)
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
