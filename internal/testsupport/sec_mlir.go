// Package testsupport contains test-only integration helpers shared across Go
// packages in the Sec repository.
package testsupport

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const secMLIRBinEnvironment = "SEC_MLIR_BIN"

// ConfiguredSecMLIROptPath resolves sec-mlir-opt from an explicitly configured
// absolute tool directory. Requiring an absolute directory keeps package tests
// independent of the working directory selected by go test.
//
// Package contract:
//   - package15-todo.md — P15-08
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §155 "Acceptance criteria"
func ConfiguredSecMLIROptPath(binDir string) (string, error) {
	if !filepath.IsAbs(binDir) {
		return "", fmt.Errorf("SEC_MLIR_BIN must be an absolute path, got %q", binDir)
	}
	return filepath.Join(filepath.Clean(binDir), "sec-mlir-opt"), nil
}

// SecMLIROptPathFromEnvironment returns the configured verifier path and
// reports false when SEC_MLIR_BIN is absent. A configured relative directory
// is an error rather than an instruction to depend on the caller's cwd.
//
// Package contract:
//   - package15-todo.md — P15-08
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §155 "Acceptance criteria"
func SecMLIROptPathFromEnvironment() (path string, configured bool, err error) {
	binDir := os.Getenv(secMLIRBinEnvironment)
	if binDir == "" {
		return "", false, nil
	}
	path, err = ConfiguredSecMLIROptPath(binDir)
	return path, true, err
}

// RequireSecMLIROptPath resolves the configured Sec MLIR verifier for a test
// and skips that test when the optional external toolchain is not configured.
// Invalid configured paths fail instead of silently skipping acceptance work.
//
// Package contract:
//   - package15-todo.md — P15-08
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §155 "Acceptance criteria"
func RequireSecMLIROptPath(t testing.TB) string {
	t.Helper()
	path, configured, err := SecMLIROptPathFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Skip("SEC_MLIR_BIN is not set")
	}
	return path
}
