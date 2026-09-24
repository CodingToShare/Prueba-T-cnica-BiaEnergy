// Package workspace locates repository-level directories such as the source
// dataset and the database migrations, so commands behave the same whether
// they are started from the repository root or from src/backend.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// maxParentLevels bounds the upward search so a missing directory fails fast.
const maxParentLevels = 6

// FindDir walks up from the working directory and returns the absolute path
// of the first existing directory named rel (for example "data/input").
func FindDir(rel string) (string, error) {
	start, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}

	dir := start
	for range maxParentLevels + 1 {
		candidate := filepath.Join(dir, filepath.FromSlash(rel))
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("directory %q not found in %s or its parent directories", rel, start)
}
