package mlir

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestVerifySecUsesSecAwareDriver(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX fake executable")
	}

	dir := t.TempDir()
	input := filepath.Join(dir, "input.mlir")
	if err := os.WriteFile(input, []byte("module {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tool := filepath.Join(dir, "sec-mlir-opt")
	script := fmt.Sprintf("#!/bin/sh\n[ \"$1\" = %q ] && [ \"$2\" = -o ] && [ \"$3\" = %q ]\n", input, os.DevNull)
	if err := os.WriteFile(tool, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	if err := NewToolchain(dir).VerifySec(input); err != nil {
		t.Fatalf("VerifySec failed: %v", err)
	}
}
