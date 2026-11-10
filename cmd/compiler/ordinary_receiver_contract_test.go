package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOrdinaryReceiverContractCLI rejects whole-self ownership transfers before IR emission.
// Rules: rules/memory/ownership.md — §9; rules/declarations/functions.md — §§22,24–25.
func TestOrdinaryReceiverContractCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/ordinary_receiver_contract_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, command := range []string{"check", "emit-ir"} {
		artifact := filepath.Join(t.TempDir(), "invalid.ir")
		args := []string{command, "../../testdata/sema/ordinary_receiver_contract_invalid.sec", "--diagnostic-format=json"}
		if command == "emit-ir" {
			args = append(args, "-o", artifact)
		}
		_, output, code = runCLIForDiagnostics(t, args...)
		if code != 3 {
			t.Fatal(code, output)
		}
		if doc := decodeOccurrenceDocument(t, output); doc.Summary.Errors != 3 {
			t.Fatal(output)
		}
		if !strings.Contains(output, "ordinary method cannot consume complete self") {
			t.Fatal(output)
		}
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatal("invalid whole-self consumption emitted an artifact", err)
		}
	}
}
