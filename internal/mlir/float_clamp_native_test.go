package mlir

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"sec/internal/testsupport"
)

// Exercise the P20 Clamp candidate/failure operation through the same native
// toolchain as source builds, without assuming physical Result/union support.
// No math runtime library is linked; the fixture reports its failing case ID.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§31,69,78,80.
func TestFloatClampNativeMatrix(t *testing.T) {
	runFloatNativeFixture(t, "../../mlir/test/Conversion/SecToLLVM/float-clamp-execution.mlir")
}

// runFloatNativeFixture uses the unified native stages and the host linker,
// deliberately without a math library. Fixtures return their failing case ID.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§23,31,77–80.
func runFloatNativeFixture(t *testing.T, fixture string) {
	t.Helper()
	testsupport.RequireSecMLIROptPath(t)
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("fixture selects linux/amd64")
	}
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang unavailable")
	}
	dir := t.TempDir()
	dialect, llvm, object, program := filepath.Join(dir, "llvm.mlir"), filepath.Join(dir, "module.ll"), filepath.Join(dir, "module.o"), filepath.Join(dir, "clamp")
	toolchain := NewToolchain("")
	if err := toolchain.LowerSecToLLVM(fixture, dialect, ""); err != nil {
		t.Fatal(err)
	}
	if err := toolchain.TranslateToLLVMIR(dialect, llvm); err != nil {
		t.Fatal(err)
	}
	if err := toolchain.CompileObject(llvm, object, "x86_64-pc-linux-gnu"); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(clang, object, "-o", program).CombinedOutput(); err != nil {
		t.Fatalf("link: %v\n%s", err, output)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, program).CombinedOutput(); err != nil {
		t.Fatalf("native float fixture failed: %v\n%s", err, output)
	}
}
