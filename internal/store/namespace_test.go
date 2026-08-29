package store

import (
	"strings"
	"testing"
)

func TestNamespaceFromDSN(t *testing.T) {
	tests := []struct {
		dsn  string
		want string
	}{
		{"postgres://faas:faas@localhost:5433/lowcode?sslmode=disable&search_path=faas", "faas"},
		{"postgres://faas:faas@localhost:5433/lowcode?search_path=custom,public", "custom"},
		{"postgres://faas:faas@localhost:5433/app", ""},
	}
	for _, tt := range tests {
		if got := namespaceFromDSN(tt.dsn); got != tt.want {
			t.Errorf("namespaceFromDSN(%q)=%q want %q", tt.dsn, got, tt.want)
		}
	}
}

func TestMigrateSQL(t *testing.T) {
	body := "CREATE TABLE IF NOT EXISTS actions (id int);\n"
	got := migrateSQL("faas", body)
	if !strings.Contains(got, `CREATE SCHEMA IF NOT EXISTS "faas"`) || !strings.Contains(got, body) {
		t.Fatalf("got %q", got)
	}
	if migrateSQL("", body) != body {
		t.Fatalf("empty namespace should not prefix SQL")
	}
	if migrateSQL("public", body) != body {
		t.Fatalf("public should not CREATE SCHEMA")
	}
}
