package postgres

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNewMigrator_MissingDirectory_FailsBeforeTouchingTheDatabase(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "no-migrations-here")

	// A nil pool proves the directory is validated before any database use.
	_, err := NewMigrator(nil, dir)

	if err == nil || !strings.Contains(err.Error(), "migrations directory") {
		t.Fatalf("expected a migrations directory error, got %v", err)
	}
}
