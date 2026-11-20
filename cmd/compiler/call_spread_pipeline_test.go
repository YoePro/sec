package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/testsupport"
)

// The compiler transports prepared spread copies through Semantic IR and Sec
// MLIR. Native emission keeps its existing explicit fixed-array boundary.
// Rules: rules/declarations/spread.md §§3,4; semantic-ir spread correction.
func TestCallSpreadSemanticIRPipeline(t *testing.T) {
	source := filepath.Join("..", "..", "testdata", "semantic_ir", "spread_call_arguments.sec")
	text, stderr, code := runCLIForDiagnostics(t, "emit-ir", source, "-o", "-")
	if code != 0 || !strings.Contains(text, "array.extract") || !strings.Contains(text, "proof=constant") {
		t.Fatalf("emit-ir: %d %s %s", code, stderr, text)
	}
	testsupport.RequireSecMLIROptPath(t)
	text, stderr, code = runCLIForDiagnostics(t, "emit-sec-mlir", source, "-o", "-")
	if code != 0 || !strings.Contains(text, "sec.array.extract") {
		t.Fatalf("emit-sec-mlir: %d %s %s", code, stderr, text)
	}
	program := filepath.Join(t.TempDir(), "spread-call")
	_, stderr, code = runCLIForDiagnostics(t, "build", source, "--target", "linux-amd64", "-o", program)
	if code == 0 || !strings.Contains(stderr, "unsupported-lowering") || !strings.Contains(stderr, "sec.array.extract") {
		t.Fatalf("native array boundary lost: %d %s", code, stderr)
	}
	if _, err := os.Stat(program); !os.IsNotExist(err) {
		t.Fatalf("unsupported source produced an artifact: %v", err)
	}
}
