package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindDir_TargetInParentDirectory_ReturnsAbsolutePath(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "data", "input")
	nested := filepath.Join(root, "src", "backend")
	for _, dir := range []string{target, nested} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(nested)

	got, err := FindDir("data/input")
	if err != nil {
		t.Fatalf("FindDir returned error: %v", err)
	}
	if !sameDir(t, got, target) {
		t.Fatalf("FindDir = %q, want %q", got, target)
	}
}

func TestFindDir_TargetMissing_ReturnsError(t *testing.T) {
	t.Chdir(t.TempDir())

	if _, err := FindDir("definitely/not/present"); err == nil {
		t.Fatal("expected an error for a missing directory")
	}
}

func sameDir(t *testing.T, a, b string) bool {
	t.Helper()
	ia, errA := os.Stat(a)
	ib, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ia, ib)
}
