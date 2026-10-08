package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unitActionFixture keeps all Sec regression inputs under testdata.
// Rules: rules/tooling/lsp.md — Unit actions.
func unitActionFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/lsp/unit_actions/" + name + ".sec")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestExtendedUnitCodeActions verifies every declared conversion family and
// applies exact edits back through the compiler. No rate or lossy cast is invented.
// Rules: rules/tooling/lsp.md — Unit actions, Safe fixes;
// rules/types/units.md — Explicit unit conversion functions, No hidden precision loss.
func TestExtendedUnitCodeActions(t *testing.T) {
	for _, test := range []struct{ name, replacement, title string }{
		{"declared", "Rotation(value)", "Use declared conversion"},
		{"factor", "EUR(value, rate)", "in-scope factor"},
		{"carrier", "int64(value)", "Change carrier exactly"},
		{"float_carrier", "float64(value)", "Change carrier exactly"},
		{"compound_carrier", "int64(value)", "Change carrier exactly"},
		{"float_exact", "float32(value)", "Change carrier exactly"},
	} {
		t.Run(test.name, func(t *testing.T) {
			uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
			text := unitActionFixture(t, test.name)
			reported := analyze(uri, text)
			actions := unitConversionCodeActions(uri, text, reported, sourceOverlay{})
			if len(actions) != 1 || !strings.Contains(actions[0].Title, test.title) {
				t.Fatalf("actions=%+v diagnostics=%+v", actions, reported)
			}
			fixed := applyTextEdits(text, actions[0].Edit.Changes[uri])
			if !strings.Contains(fixed, "return "+test.replacement) {
				t.Fatal(fixed)
			}
			for _, d := range analyze(uri, fixed) {
				if d.Severity == 1 {
					t.Fatal(d)
				}
			}
			stale := reported
			stale[0].Message = "obsolete diagnostic"
			if got := unitConversionCodeActions(uri, text, stale, sourceOverlay{}); len(got) != 0 {
				t.Fatal("stale action", got)
			}
		})
	}
	for _, name := range []string{"lossy", "float_lossy", "signed_lossy", "factor_absent", "factor_wrong", "factor_out_of_scope"} {
		t.Run(name, func(t *testing.T) {
			uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
			text := unitActionFixture(t, name)
			reported := analyze(uri, text)
			if len(reported) == 0 {
				t.Fatal("fixture did not produce a mismatch")
			}
			if got := unitConversionCodeActions(uri, text, reported, sourceOverlay{}); len(got) != 0 {
				t.Fatal("unproven conversion offered", got)
			}
		})
	}
}

// TestUnitCarrierActionsUseSelectedTarget proves the full value domain fits
// int on a 64-bit target and rejects truncation on the selected 32-bit target.
// Rules: rules/tooling/lsp.md — Target-aware analysis, Unit actions;
// rules/types/types.md — int and uint; rules/types/units.md — No hidden precision loss.
func TestUnitCarrierActionsUseSelectedTarget(t *testing.T) {
	root, uri, _ := targetSelectionFixture(t)
	text := unitActionFixture(t, "target_carrier")
	for _, variant := range []string{"small", "large"} {
		overlay := sourceOverlay{Targets: map[string]lspTargetSelection{normalizedSourcePath(root): {"app", variant}}}
		reported := analyze(uri, text, overlay)
		actions := unitConversionCodeActions(uri, text, reported, overlay)
		want := 0
		if variant == "large" {
			want = 1
		}
		if len(actions) != want {
			t.Fatalf("%s: actions=%+v diagnostics=%+v", variant, actions, reported)
		}
	}
}

// TestUnitImportActions uses the current unsaved source of an unimported module,
// preserves CRLF/comments and proves qualified unit references resolve afterward.
// Rules: rules/tooling/lsp.md — Unit actions, Snapshots;
// rules/projects/modules.md — §§7–9; types/units.md — Unit identity and named types.
func TestUnitImportActions(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "quantities")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".sec", "sec.toml"), []byte("[project]\nname = \"unit-actions\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	moduleFile := filepath.Join(dir, "quantities.sec")
	if err := os.WriteFile(moduleFile, []byte("module quantities\n"), 0644); err != nil {
		t.Fatal(err)
	}
	overlay := sourceOverlay{Sources: map[string]string{moduleFile: unitActionFixture(t, "unit_module")}}
	uri := uriFromPath(filepath.Join(root, "main.sec"))
	text := strings.ReplaceAll(unitActionFixture(t, "import"), "\n", "\r\n")
	reported := analyze(uri, text, overlay)
	actions := unitConversionCodeActions(uri, text, reported, overlay)
	if len(actions) != 1 {
		t.Fatalf("expected one deduplicated import action: %+v, %+v", actions, reported)
	}
	fixed := applyTextEdits(text, actions[0].Edit.Changes[uri])
	if !strings.Contains(fixed, "module main // keep header comment\r\nimport \"quantities\"\r\n") || !strings.Contains(fixed, "decimal<quantities.LocalLength>") {
		t.Fatal(fixed)
	}
	if got := analyze(uri, fixed, overlay); len(got) != 0 {
		t.Fatal(got)
	}
	if got := unitConversionCodeActions(uri, text, reported, sourceOverlay{}); len(got) != 0 {
		t.Fatal("closed overlay unit invented", got)
	}
	aliasText := unitActionFixture(t, "import_alias")
	aliasActions := unitConversionCodeActions(uri, aliasText, analyze(uri, aliasText, overlay), overlay)
	if len(aliasActions) != 1 {
		t.Fatal(aliasActions, analyze(uri, aliasText, overlay))
	}
	aliasFixed := applyTextEdits(aliasText, aliasActions[0].Edit.Changes[uri])
	if strings.Count(aliasFixed, "import ") != 1 || !strings.Contains(aliasFixed, "decimal<q.LocalLength>") {
		t.Fatal(aliasFixed)
	}
	if got := analyze(uri, aliasFixed, overlay); len(got) != 0 {
		t.Fatal(got)
	}
	privateOverlay := sourceOverlay{Sources: map[string]string{moduleFile: strings.ReplaceAll(unitActionFixture(t, "unit_module"), "LocalLength", "_LocalLength")}}
	privateText := strings.ReplaceAll(text, "LocalLength", "_LocalLength")
	if got := unitConversionCodeActions(uri, privateText, analyze(uri, privateText, privateOverlay), privateOverlay); len(got) != 0 {
		t.Fatal("private unit import offered", got)
	}
}
