package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
	"station-backend/internal/config"
)

// Minimal SQL driver tests fail-closed bootstrap without a live production DB.
type bootstrapProbe struct {
	marker    string
	existing  bool
	failure   string
	inserted  bool
	committed bool
	hash      string
}
type bootstrapDriver struct{ probe *bootstrapProbe }
type bootstrapConn struct{ probe *bootstrapProbe }
type bootstrapTx struct{ probe *bootstrapProbe }
type bootstrapRows struct {
	column string
	values []driver.Value
}

var bootstrapDriverID atomic.Uint64

func (d bootstrapDriver) Open(string) (driver.Conn, error) { return bootstrapConn{d.probe}, nil }
func (c bootstrapConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (c bootstrapConn) Close() error              { return nil }
func (c bootstrapConn) Begin() (driver.Tx, error) { return bootstrapTx{c.probe}, nil }
func (tx bootstrapTx) Commit() error {
	if tx.probe.failure == "commit" {
		return errors.New("private database details")
	}
	tx.probe.committed = true
	return nil
}
func (tx bootstrapTx) Rollback() error     { return nil }
func (r *bootstrapRows) Columns() []string { return []string{r.column} }
func (r *bootstrapRows) Close() error      { return nil }
func (r *bootstrapRows) Next(dest []driver.Value) error {
	if len(r.values) == 0 {
		return io.EOF
	}
	dest[0], r.values = r.values[0], r.values[1:]
	return nil
}
func (c bootstrapConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.probe.failure == "query" {
		return nil, errors.New("private database details")
	}
	if strings.Contains(query, "system_settings") {
		return &bootstrapRows{"setting_value", []driver.Value{c.probe.marker}}, nil
	}
	rows := &bootstrapRows{column: "id"}
	if c.probe.existing {
		rows.values = []driver.Value{int64(1)}
	}
	return rows, nil
}
func (c bootstrapConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if strings.Contains(query, "INSERT INTO users") {
		if c.probe.failure == "insert" {
			return nil, errors.New("private database details")
		}
		c.probe.inserted = true
		c.probe.hash = args[1].Value.(string)
	}
	return driver.RowsAffected(1), nil
}
func TestBootstrapIsOptInOneTimeAndFailsClosed(t *testing.T) {
	if err := ensureSuperAdmin(nil, &config.Config{}); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name, marker, failure           string
		existing, wantInsert, wantError bool
	}{
		{"new", "false", "", false, true, false},
		{"existing admin", "false", "", true, false, false},
		{"completed but account deleted", "true", "", false, false, false},
		{"database check failed", "false", "query", false, false, true},
		{"insert failed", "false", "insert", false, false, true},
		{"commit failed", "false", "commit", false, true, true},
		{"corrupt marker", "unknown", "", false, false, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			probe := &bootstrapProbe{marker: scenario.marker, existing: scenario.existing, failure: scenario.failure}
			name := "bootstrap-probe-" + strings.Repeat("x", int(bootstrapDriverID.Add(1)))
			sql.Register(name, bootstrapDriver{probe})
			db, err := sql.Open(name, "")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			cfg := &config.Config{BootstrapAdmin: true, SAdminUsername: "initial-admin", SAdminPassword: "correct-horse-battery-staple"}
			err = ensureSuperAdmin(sqlx.NewDb(db, name), cfg)
			if (err != nil) != scenario.wantError || probe.inserted != scenario.wantInsert {
				t.Fatal("unexpected bootstrap outcome")
			}
			if err != nil && (probe.committed || strings.Contains(err.Error(), "private")) {
				t.Fatal("failed bootstrap committed or leaked database detail")
			}
			if cfg.SAdminPassword != "" {
				t.Fatal("bootstrap password retained in runtime config")
			}
			if probe.inserted && bcrypt.CompareHashAndPassword([]byte(probe.hash), []byte("correct-horse-battery-staple")) != nil {
				t.Fatal("bootstrap did not store a password hash")
			}
		})
	}
}
