package config

import (
	"strings"
	"testing"
)

func TestBootstrapRejectsDocumentedCredentials(t *testing.T) {
	for _, pair := range [][2]string{
		{"ชื่อadminที่ต้องการ", "correct-horse-battery-staple"},
		{"initial-admin", "ใส่รหัสผ่าน12ถึง72bytes"},
		{"initial-admin", "REPLACE_WITH_STRONG_PASSWORD"},
		{"initial-admin", strings.Repeat("a", 32)},
		{"initial-admin", "short-password"},
		{"initial-admin", strings.Repeat("xy", 37)},
	} {
		if ValidateBootstrapCredentials(pair[0], pair[1]) == nil {
			t.Fatal("unsafe bootstrap credentials accepted")
		}
	}
	if err := ValidateBootstrapCredentials("initial-admin", "correct-horse-battery-staple"); err != nil {
		t.Fatal(err)
	}
	if !exampleSecret("ใส่ค่าสุ่มอย่างน้อย32bytes") || !exampleSecret(strings.Repeat("a", 64)) {
		t.Fatal("example signing secret accepted")
	}
}
