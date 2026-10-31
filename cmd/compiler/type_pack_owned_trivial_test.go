package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sec/internal/testsupport"
)

// Trivial owned pack positions use ordinary value arguments through verified
// Semantic IR/Sec MLIR, while resource-owning elements retain a named boundary.
// Rules: rules/declarations/functions.md §38A(3),(6),(9);
// rules/compiler/semantic_ir.md §27A; rules/compiler/monomorphization.md §11.2,11.4.
func TestTrivialOwnedTypePackCLI(t *testing.T) {
	for _, name := range []string{"owned_scalar.sec", "owned_trivial.sec", "static_owned_boundary.sec"} {
		for _, command := range []string{"emit-ir", "emit-sec-mlir"} {
			if command == "emit-sec-mlir" && os.Getenv("SEC_MLIR_BIN") == "" {
				continue
			}
			args := []string{command, typePackCLIFixture(t, name), "--target", "linux-amd64"}
			if command == "emit-ir" {
				args = append(args, "-o", "-")
			}
			stdout, output, code := runCLIForDiagnostics(t, args...)
			if code != 0 || !strings.Contains(stdout, "Count[...Types<") {
				t.Fatal(name, command, code, stdout, output)
			}
		}
	}
	stdout, output, code := runCLIForDiagnostics(t, "emit-ir", typePackCLIFixture(t, "owned_trivial_move_boundary.sec"), "--target", "linux-amd64", "-o", "-")
	if code != 4 || stdout != "" || !strings.Contains(output, "[unsupported-lowering]") {
		t.Fatal(code, stdout, output)
	}

}

// A scalar-only owned pack has no reference/cleanup dependency, so its complete
// source-to-binary path preserves exact arity, copy-source reuse and once-only
// argument calls through the single compilation track.
// Rules: rules/declarations/functions.md §38A(4–6),(9);
// rules/compiler/semantic_ir.md §27A(2–4).
func TestTrivialOwnedTypePackNative(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	program := filepath.Join(t.TempDir(), "owned-pack")
	_, output, code := runCLIForDiagnostics(t, "build", typePackCLIFixture(t, "owned_trivial.sec"), "--target", "linux-amd64", "-o", program)
	if code != 0 {
		t.Fatal(code, output)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, program).CombinedOutput(); err != nil {
		t.Fatalf("owned pack program: %v %s", err, output)
	}
}
