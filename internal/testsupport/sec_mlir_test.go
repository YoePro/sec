package testsupport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfiguredSecMLIRBinRequiresAbsolutePathForPackages13Through15(t *testing.T) {
	if _, err := ConfiguredSecMLIROptPath("build/sec-mlir/bin"); err == nil || !strings.Contains(err.Error(), "must be an absolute path") {
		t.Fatalf("relative SEC_MLIR_BIN error = %v, want absolute-path rejection", err)
	}

	want := filepath.Join(t.TempDir(), "tools", "sec-mlir-opt")
	got, err := ConfiguredSecMLIROptPath(filepath.Dir(want))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("configured tool path = %q, want %q", got, want)
	}
}

func TestSecMLIROptPathIsIndependentOfPackageWorkingDirectory(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(original); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	}()

	root := t.TempDir()
	binDir := filepath.Join(root, "toolchain", "bin")
	t.Setenv(secMLIRBinEnvironment, binDir)
	workingDirectory := filepath.Join(root, "go-package", "nested")
	if err := os.MkdirAll(workingDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workingDirectory); err != nil {
		t.Fatal(err)
	}

	got, configured, err := SecMLIROptPathFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("absolute SEC_MLIR_BIN was not detected")
	}
	want := filepath.Join(binDir, "sec-mlir-opt")
	if got != want {
		t.Fatalf("tool path from nested package cwd = %q, want %q", got, want)
	}
}
