package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The public compiler reports the primary syntax faults and prevents invalid
// callable artifacts, retaining independent source diagnostics when available.
// Rules: rules/compiler/parser_recovery.md §§14.7,15,46.2,82.2.
func TestLambdaReturnRecoveryCLI(t *testing.T) {
	for _, command := range []string{"check", "emit-ir"} {
		artifact := filepath.Join(t.TempDir(), "invalid.ir")
		args := []string{command, "../../testdata/parser/lambda_return_recovery/missing_invalid.sec", "--diagnostic-format=json"}
		if command == "emit-ir" {
			args = append(args, "-o", artifact)
		}
		_, output, code := runCLIForDiagnostics(t, args...)
		if code == 0 {
			t.Fatal("invalid lambda accepted")
		}
		if strings.Count(output, "lambda return type is required") != 2 {
			t.Fatal(output)
		}
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatal("invalid lambda emitted IR", err)
		}
	}
}
