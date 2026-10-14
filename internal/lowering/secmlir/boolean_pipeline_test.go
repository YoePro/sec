package secmlir

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

func booleanFixtureModule(t *testing.T, fixture string) *semantic.Module {
	t.Helper()
	path := "../../../testdata/semantic_ir/" + fixture + "/valid.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), path))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatal(p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{MaxPackage: 12})
	if err != nil {
		t.Fatal(err)
	}
	if err := semantic.Verify(module); err != nil {
		t.Fatal(err)
	}
	return module
}

// executeBooleanFixture validates and executes the maintained Sec MLIR pipeline.
// Rules: rules/foundations/operators.md — "Logical operators", "Boolean equality".
func executeBooleanFixture(t *testing.T, fixture string) {
	t.Helper()
	secOpt := testsupport.RequireSecMLIROptPath(t)
	binDir := os.Getenv("SEC_LEGACY_MLIR_BIN")
	if binDir == "" {
		t.Skip("SEC_LEGACY_MLIR_BIN must name the LLVM/MLIR tool directory")
	}
	if !filepath.IsAbs(binDir) {
		t.Fatal("SEC_LEGACY_MLIR_BIN must be absolute")
	}
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("native execution requires linux-amd64")
	}
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Fatal(err)
	}
	output, err := Emit(booleanFixtureModule(t, fixture), testPlan(64))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source.mlir")
	core := filepath.Join(dir, "core.mlir")
	llvm := filepath.Join(dir, "llvm.mlir")
	ir := filepath.Join(dir, "bool.ll")
	binary := filepath.Join(dir, "bool-native")
	if err := os.WriteFile(source, output, 0600); err != nil {
		t.Fatal(err)
	}
	harness, err := filepath.Abs("../../../testdata/semantic_ir/" + fixture + "/native.c")
	if err != nil {
		t.Fatal(err)
	}
	commands := [][]string{
		{secOpt, source, "--sec-lower-scalar-core", "--sec-lower-trivial-core", "--verify-each", "-o", core},
		{filepath.Join(binDir, "mlir-opt"), core, "--convert-arith-to-llvm", "--convert-cf-to-llvm", "--convert-func-to-llvm", "--finalize-memref-to-llvm", "--convert-index-to-llvm", "--reconcile-unrealized-casts", "--verify-each", "-o", llvm},
		{filepath.Join(binDir, "mlir-translate"), llvm, "--mlir-to-llvmir", "-o", ir},
		{clang, "-target", "x86_64-pc-linux-gnu", ir, harness, "-o", binary},
		{binary},
	}
	for _, command := range commands {
		if data, err := exec.Command(command[0], command[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", filepath.Base(command[0]), err, data)
		}
	}
}
