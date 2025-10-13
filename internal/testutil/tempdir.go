package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/atomicstack/gotmuxcc/internal/trace"
)

var (
	rootOnce sync.Once
	rootDir  string
	rootErr  error
)

func projectRoot() (string, error) {
	rootOnce.Do(func() {
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			rootErr = fmt.Errorf("testutil: unable to determine caller location")
			return
		}
		internalDir := filepath.Dir(file)
		root := filepath.Clean(filepath.Join(internalDir, "..", ".."))
		if stat, err := os.Stat(filepath.Join(root, "go.mod")); err != nil || stat.IsDir() {
			if err != nil {
				rootErr = fmt.Errorf("testutil: unable to stat go.mod: %w", err)
				return
			}
			rootErr = fmt.Errorf("testutil: expected go.mod to be a file at %q", root)
			return
		}
		rootDir = root
	})
	return rootDir, rootErr
}

// TempDir creates a temporary directory inside the repository root that will
// be cleaned up when the provided test completes.
func TempDir(t testing.TB) string {
	t.Helper()

	root, err := projectRoot()
	if err != nil {
		t.Fatalf("testutil.TempDir: %v", err)
	}

	base := filepath.Join(root, ".testtmp")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("testutil.TempDir: failed to create base directory: %v", err)
	}

	dir, err := os.MkdirTemp(base, "gotmuxcc-")
	if err != nil {
		t.Fatalf("testutil.TempDir: failed to create temporary directory: %v", err)
	}
	trace.Printf("testutil", "TempDir created base=%q dir=%q", base, dir)

	t.Cleanup(func() {
		trace.Printf("testutil", "TempDir cleanup dir=%q", dir)
		_ = os.RemoveAll(dir)
	})

	return dir
}
