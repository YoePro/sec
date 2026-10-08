package diagnostics

import (
	"os"
	"strings"
	"testing"
)

// TestDefaultDiagnosticStableIdentities pins both allocated identities, their
// semantic namespace, family, and mandatory error policy, including lowering.
// Rules: rules/types/default_values.md — "Diagnostics";
// rules/tooling/diagnostics.md — §§4–5.
func TestDefaultDiagnosticStableIdentities(t *testing.T) {
	for _, want := range []Definition{
		{ID: "S1136", Name: "types.default-cycle", Family: "types", DefaultSeverity: SeverityError, Mandatory: true},
		{ID: "S1137", Name: "backend.default-left-undefined", Family: "backend", DefaultSeverity: SeverityError, Mandatory: true},
	} {
		got, ok := Lookup(want.ID)
		if !ok || got != want {
			t.Fatalf("definition %s = %+v (%v), want %+v", want.ID, got, ok, want)
		}
	}
}

// TestDefaultRulebookDiagnosticInventory checks the complete normative list,
// so adding the final two identities cannot conceal another unregistered name.
// Rules: rules/types/default_values.md — "Diagnostics".
func TestDefaultRulebookDiagnosticInventory(t *testing.T) {
	data, err := os.ReadFile("../../rules/types/default_values.md")
	if err != nil {
		t.Fatal(err)
	}
	_, section, found := strings.Cut(string(data), "\n# Diagnostics\n")
	if !found {
		t.Fatal("missing normative diagnostic section")
	}
	_, section, found = strings.Cut(section, "```text\n")
	if !found {
		t.Fatal("missing normative diagnostic inventory")
	}
	inventory, _, found := strings.Cut(section, "```")
	if !found {
		t.Fatal("unterminated diagnostic inventory")
	}
	names := map[string]Definition{}
	for _, definition := range All() {
		if !definition.Retired {
			names[definition.Name] = definition
		}
	}
	for _, name := range strings.Fields(inventory) {
		definition, exists := names[name]
		if !exists || !definition.Mandatory || definition.DefaultSeverity != SeverityError {
			t.Fatalf("normative default diagnostic %s is not a registered mandatory error: %+v", name, definition)
		}
	}
}
