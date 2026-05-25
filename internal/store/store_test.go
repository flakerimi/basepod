package store

import (
	"context"
	"testing"
)

func TestOpenAndMigrate(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	for _, table := range []string{"users", "tokens", "sessions", "apps", "app_versions", "app_env", "app_volumes", "app_ports", "app_domains", "templates_installed", "settings", "audit_log"} {
		var n int
		err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n)
		if err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
}
