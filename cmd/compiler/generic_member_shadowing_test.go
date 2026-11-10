package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenericMemberShadowingCLI shares Sema's mandatory name diagnostics and
// prevents an invalid generic scope from producing a Semantic IR artifact.
// Rules: rules/foundations/names_scopes_visibility.md — §§8,15,20;
// rules/declarations/generics.md — §9.
func TestGenericMemberShadowingCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/names/generic_member_shadowing/valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, command := range []string{"check", "emit-ir"} {
		artifact := filepath.Join(t.TempDir(), "invalid.ir")
		args := []string{command, "../../testdata/names/generic_member_shadowing/invalid.sec", "--diagnostic-format=json"}
		if command == "emit-ir" {
			args = append(args, "-o", artifact)
		}
		_, output, code := runCLIForDiagnostics(t, args...)
		if code != 3 {
			t.Fatal(code, output)
		}
		if doc := decodeOccurrenceDocument(t, output); doc.Summary.Errors != 8 {
			t.Fatal(output)
		}
		if strings.Count(output, `"id": "S1105"`) != 8 {
			t.Fatal(output)
		}
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatal("invalid scope produced IR", err)
		}
	}
}
