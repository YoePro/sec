package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPropertyEffectsCLI rejects panic-capable accessor invocation before IR
// production and keeps infallible sibling accessors usable from @noPanic.
// Rules: rules/declarations/properties.md — §14; rules/errors/panic.md — §21.
func TestPropertyEffectsCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/property_effects/valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, command := range []string{"check", "emit-ir"} {
		artifact := filepath.Join(t.TempDir(), "property.ir")
		args := []string{command, "../../testdata/sema/property_effects/callers_invalid.sec", "--diagnostic-format=json"}
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
		if strings.Count(output, `"id": "S1092"`) != 3 || !strings.Contains(output, "Value.get") || !strings.Contains(output, "Value.set") {
			t.Fatal(output)
		}
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatal("invalid accessor effects emitted IR", err)
		}
	}
}
