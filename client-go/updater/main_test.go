package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReplaceWithRetryPreservesTargetUntilReplacement(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "AUCCAgent.exe")
	staged := filepath.Join(dir, "staged.exe")
	if err := os.WriteFile(current, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := replaceWithRetry(staged, current); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(current)
	if err != nil || string(data) != "new" {
		t.Fatalf("replacement failed: data=%q err=%v", data, err)
	}
}

func TestFailedNewAgentRestoresOldVersion(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "AUCCAgent.exe")
	staged := filepath.Join(dir, "staged.exe")
	buildStubAgent(t, current, "0.1.0")
	buildStubAgent(t, staged, "0.2.0")
	content, err := os.ReadFile(staged)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	if err := replaceAgent(current, staged, "0.2.0", hex.EncodeToString(digest[:])); err == nil {
		t.Fatal("expected fast-exiting new Agent to trigger rollback")
	}
	output, err := exec.Command(current, "--version").Output()
	if err != nil || strings.TrimSpace(string(output)) != "0.1.0" {
		t.Fatalf("old Agent not restored: output=%q err=%v", output, err)
	}
}

func TestVersionPreflightTimesOutWithoutReplacingInstalledAgent(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "AUCCAgent.exe")
	staged := filepath.Join(dir, "staged.exe")
	buildStubAgent(t, current, "0.1.0")
	buildStubAgent(t, staged, "0.2.0")
	t.Setenv("AUCC_TEST_HANG_VERSION", "0.2.0")
	digest, err := fileSHA256(staged)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if err := replaceAgent(current, staged, "0.2.0", digest); err == nil {
		t.Fatal("hung version preflight accepted")
	}
	if time.Since(started) > 20*time.Second {
		t.Fatal("version preflight exceeded its timeout")
	}
	output, err := exec.Command(current, "--version").Output()
	if err != nil || strings.TrimSpace(string(output)) != "0.1.0" {
		t.Fatalf("installed Agent changed: output=%q err=%v", output, err)
	}
}

func buildStubAgent(t *testing.T, output, version string) {
	t.Helper()
	source := filepath.Join(filepath.Dir(output), version+".go")
	code := "package main\nimport (\"fmt\";\"os\";\"time\")\nfunc main(){if len(os.Args)>1 && os.Args[1]==\"--version\" {if os.Getenv(\"AUCC_TEST_HANG_VERSION\")==\"" + version + "\" {time.Sleep(time.Minute)};fmt.Print(\"" + version + "\")}}"
	if err := os.WriteFile(source, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := exec.Command("go", "build", "-o", output, source).CombinedOutput()
	if err != nil {
		t.Fatalf("build stub Agent: %v: %s", err, result)
	}
}
