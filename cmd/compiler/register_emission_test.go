package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sec/internal/testsupport"
	"strings"
	"testing"
)

// The public high-level inspection path verifies real source register tables
// and operations before publishing bytes. This does not admit physical lowering.
// Rules: rules/compiler/compiler_pipeline.md — §§59–60;
// rules/mlir/sec_mlir_dialect.md — v11.41.1a (MD-114 §11(11)).
func TestDetachedRegistersEmitVerifiedSecMLIR(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	fixture, err := filepath.Abs("../../testdata/semantic_ir/register_detached_values.sec")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "registers.mlir")
	command := exec.Command(os.Args[0], "-test.run=^TestLexerCLIProcess$", "--", "emit-sec-mlir", fixture, "-o", output, "--mlir-bin", filepath.Dir(tool))
	command.Env = append(os.Environ(), "SEC_LEXER_CLI_TEST_PROCESS=1")
	if diagnostic, err := command.CombinedOutput(); err != nil {
		t.Fatalf("register inspection: %v\n%s", err, diagnostic)
	}
	text, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"sec.registers = [", "#sec.register_def", "sec.register.extract", "sec.register.update", "sec.register.from_bits", "sec.register.to_bits"} {
		if !strings.Contains(string(text), want) {
			t.Fatalf("missing %s", want)
		}
	}
}
