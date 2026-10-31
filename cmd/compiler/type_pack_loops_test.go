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

// MD-117 pack iteration: each specialization builds the loop body once per
// concrete position (Size[T=int] and Size[T=bool] for a shared pack), and an
// owned scalar pack loop with index, continue, break and an empty pack runs
// through the single native track.
// Rules: rules/declarations/functions.md §38A(5)(3)-(7),(9);
// rules/control-flow/flowcontrol_for.md §13A; rules/compiler/semantic_ir.md §27A.
func TestTypePackLoopsCLI(t *testing.T) {
	stdout, output, code := runCLIForDiagnostics(t, "emit-ir", typePackCLIFixture(t, "pack_loop_shared.sec"), "--target", "linux-amd64", "-o", "-")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, want := range []string{"for-pack", `"main::Size[T=int](ref T)"`, `"main::Size[T=bool](ref T)"`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("missing %q:\n%s", want, stdout)
		}
	}
	if os.Getenv("SEC_MLIR_BIN") != "" {
		if _, output, code := runCLIForDiagnostics(t, "emit-sec-mlir", typePackCLIFixture(t, "pack_loop_shared.sec"), "--target", "linux-amd64"); code != 0 {
			t.Fatal(code, output)
		}
	}
}

func TestTypePackLoopsNative(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	for _, target := range []string{"linux-amd64"} {
		program := filepath.Join(t.TempDir(), "pack-loop")
		if _, output, code := runCLIForDiagnostics(t, "build", typePackCLIFixture(t, "pack_loop_owned.sec"), "--target", target, "-o", program); code != 0 {
			t.Fatal(code, output)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		output, err := exec.CommandContext(ctx, program).CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("pack loop program: %v %s", err, output)
		}
	}
}
