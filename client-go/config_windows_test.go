package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestStationConfigRemovesInheritedPublicAccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"agent_secret":"test-only"}`), 0600); err != nil {
		t.Fatal(err)
	}
	public, err := windows.SecurityDescriptorFromString("D:(A;;FA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := public.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
	if err := protectStationConfig(path); err != nil {
		t.Fatal(err)
	}
	private, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := private.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("station credential ACL still inherits folder permissions")
	}
	permissions := private.String()
	for _, principal := range []string{"WD", "BU", "AU"} {
		if strings.Contains(permissions, ";;;"+principal+")") {
			t.Fatal("public accounts retain station credential access")
		}
	}
	if _, err := os.ReadFile(path); err != nil {
		t.Fatalf("station user lost credential access: %v", err)
	}
}
