package main

import (
	"os"
	"path/filepath"
	"reflect"
	lspserver "sec/internal/lsp/server"
	"testing"

	platformtarget "sec/internal/platform/target"
)

// Settings-only manifests preserve standalone CLI/LSP scalar agreement. A real
// target request, malformed configuration or a declared incomplete target must
// never acquire host widths merely because target resolution failed.
// Rules: rules/tooling/lsp.md — Diagnostics, Configuration, Snapshots;
// rules/platform/platform_model.md — §26.
func TestLSPHostScalarPlanWithSettings(t *testing.T) {
	definition, ok := platformtarget.Find(platformtarget.Host())
	if !ok {
		t.Fatal("host target is not registered")
	}
	plan, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, text               string
		allow, overlay, selected bool
	}{
		{name: "diagnostics", text: "[diagnostics.rules]\n\"ownership.redundant-discard\"='info'\n", allow: true},
		{name: "format", text: "[format]\nindentation_width=2\n", allow: true},
		{name: "quoted inline", text: "\"analysis\"={lsp_depth='deep'}\n", allow: true},
		{name: "string is not target", text: "notes='''\n[target.not_declared]\n'''\n", allow: true},
		{name: "overlay settings", text: "[analysis]\nlsp_depth='deep'\n", allow: true, overlay: true},
		{name: "incomplete target", text: "[target.app]\nsource='.'\n"},
		{name: "variant only", text: "[variant.arm]\nos='linux'\narch='armv7'\n"},
		{name: "malformed", text: "[diagnostics.rules\n"},
		{name: "overlay target", text: "target={app={source='.'}}\n", overlay: true},
		{name: "unknown selection", text: "[format]\nindentation_width=2\n", selected: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			manifest := filepath.Join(root, ".sec", "sec.toml")
			if err := os.MkdirAll(filepath.Dir(manifest), 0755); err != nil {
				t.Fatal(err)
			}
			disk := tc.text
			if tc.overlay {
				disk = "[format]\nindentation_width=4\n"
			}
			if err := os.WriteFile(manifest, []byte(disk), 0600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "main.sec")
			overlay := sourceOverlay{}
			if tc.overlay {
				overlay.Sources = map[string]string{manifest: tc.text}
			}
			if tc.selected {
				overlay.Targets = map[string]lspTargetSelection{normalizedSourcePath(root): {LogicalTarget: "missing", Variant: "missing"}}
			}
			if got := lspAllowsHostScalarPlan(path, overlay); got != tc.allow {
				t.Fatalf("host fallback=%t, want %t", got, tc.allow)
			}
			analyzer := newLSPAnalyzerForInputs(uriFromPath(path), nil, overlay)
			want := int64(0)
			if tc.allow {
				want = int64(plan.PointerWidthBits)
			}
			if got := analyzer.Types()["int"].BitWidth; got != want {
				t.Fatalf("int width=%d, want %d", got, want)
			}
		})
	}
}

// Both individual and background module diagnostics must retain the same
// project-policy-controlled discard advice under a settings-only manifest.
// Rules: rules/tooling/lsp.md — Diagnostics, Configuration;
// rules/control-flow/discard.md — §§37–38.
func TestDiscardAdviceBatchMatchesStandalone(t *testing.T) {
	for _, name := range []string{"redundant_discard_valid.sec", "explicit_discard_advice.sec"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("../../testdata/tooling/" + name)
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			manifest := filepath.Join(root, ".sec", "sec.toml")
			if err := os.MkdirAll(filepath.Dir(manifest), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(manifest, []byte("[diagnostics.rules]\n\"ownership.redundant-discard\"='info'\n\"ownership.useless-explicit-discard\"='info'\n\"ownership.redundant-void-discard\"='info'\n"), 0600); err != nil {
				t.Fatal(err)
			}
			uri := uriFromPath(filepath.Join(root, "main.sec"))
			single := analyze(uri, string(data))
			if len(single) == 0 {
				t.Fatal("discard advice disappeared")
			}
			batch := analyzeDiagnosticBatch([]lspserver.Snapshot{{URI: uri, Text: string(data)}}, sourceOverlay{})
			if !reflect.DeepEqual(single, batch[uri]) {
				t.Fatalf("standalone/batch disagree:\n%+v\n%+v", single, batch[uri])
			}
		})
	}
}
