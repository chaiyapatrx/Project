package handler

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"station-backend/internal/auth"
)

type passwordChangeDriver struct{}
type passwordChangeConn struct {
	scenario, oldHash string
	userID            int64
}
type passwordChangeRows struct {
	hash    string
	version int64
	userID  int64
	read    bool
}

func init() { sql.Register("password-change-test", passwordChangeDriver{}) }

var passwordChangeTestID int64 = 10000

func (passwordChangeDriver) Open(name string) (driver.Conn, error) {
	scenario, idString, _ := strings.Cut(name, ":")
	userID, err := strconv.ParseInt(idString, 10, 64)
	if err != nil {
		return nil, err
	}
	hash, err := auth.HashPassword("old-password-test")
	return &passwordChangeConn{scenario: scenario, oldHash: hash, userID: userID}, err
}
func (*passwordChangeConn) Close() error { return nil }
func (*passwordChangeConn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("unexpected transaction")
}
func (*passwordChangeConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("unexpected statement")
}
func (c *passwordChangeConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if !strings.Contains(query, "SELECT * FROM users") {
		return nil, fmt.Errorf("unexpected query")
	}
	version := int64(7)
	if c.scenario == "revoked-before-read" {
		version = 8
	}
	return &passwordChangeRows{hash: c.oldHash, version: version, userID: c.userID}, nil
}
func (c *passwordChangeConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if strings.Contains(query, "INSERT INTO audit_logs") {
		return driver.RowsAffected(1), nil
	}
	if !strings.Contains(query, "UPDATE users SET password_hash") {
		return nil, fmt.Errorf("unexpected update")
	}
	if c.scenario != "unchanged" {
		// Simulate a security decision committed after the password read. Only
		// an update comparing both original credentials and version may fail.
		guarded := strings.Contains(query, "AND is_active = 1 AND password_hash = ? AND token_version = ?") &&
			len(args) == 4 && args[1].Value == c.userID && args[2].Value == c.oldHash && args[3].Value == int64(7)
		if guarded {
			return driver.RowsAffected(0), nil
		}
	}
	return driver.RowsAffected(1), nil
}
func (*passwordChangeRows) Columns() []string {
	return []string{"id", "password_hash", "token_version"}
}
func (*passwordChangeRows) Close() error { return nil }
func (r *passwordChangeRows) Next(values []driver.Value) error {
	if r.read {
		return io.EOF
	}
	r.read = true
	values[0], values[1], values[2] = r.userID, r.hash, r.version
	return nil
}

func TestPasswordChangeCannotOverwriteNewerSecurityDecision(t *testing.T) {
	for _, scenario := range []string{"unchanged", "password-reset", "logout", "deactivation", "role-change", "revoked-before-read"} {
		t.Run(scenario, func(t *testing.T) {
			userID := atomic.AddInt64(&passwordChangeTestID, 1)
			db, err := sql.Open("password-change-test", fmt.Sprintf("%s:%d", scenario, userID))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			h := NewAuthHandler(sqlx.NewDb(db, "password-change-test"), nil)
			router := gin.New()
			router.POST("/password", func(c *gin.Context) {
				c.Set("user_id", int(userID))
				c.Set("token_version", 7)
				h.ChangePassword(c)
			})
			request := httptest.NewRequest(http.MethodPost, "/password", strings.NewReader(`{"old_password":"old-password-test","new_password":"new-password-test"}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			want := http.StatusConflict
			if scenario == "unchanged" {
				want = http.StatusOK
			} else if scenario == "revoked-before-read" {
				want = http.StatusUnauthorized
			}
			if response.Code != want {
				t.Fatalf("password change status = %d, want %d", response.Code, want)
			}
		})
	}
}

func TestLoginRejectsOversizedCredentialsBeforeDatabaseAccess(t *testing.T) {
	h := NewAuthHandler(nil, nil)
	router := gin.New()
	router.POST("/login", h.Login)
	for _, body := range []string{
		`{"username":"` + strings.Repeat("a", 51) + `","password":"password-test"}`,
		`{"username":"student","password":"` + strings.Repeat("a", 73) + `"}`,
	} {
		request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("oversized credentials returned status %d", response.Code)
		}
	}
}

func TestNewPasswordPolicy(t *testing.T) {
	for _, test := range []struct {
		name, password string
		want           bool
	}{
		{"short", strings.Repeat("a", 14), false},
		{"minimum", strings.Repeat("a", 15), true},
		{"bcrypt maximum", strings.Repeat("a", 72), true},
		{"too many bytes", strings.Repeat("a", 73), false},
		{"unicode minimum", strings.Repeat("ก", 15), true},
		{"unicode short", strings.Repeat("ก", 14), false},
		{"unicode too many bytes", strings.Repeat("ก", 25), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := passwordLengthValid(test.password); got != test.want {
				t.Fatalf("passwordLengthValid() = %v, want %v", got, test.want)
			}
		})
	}
}
