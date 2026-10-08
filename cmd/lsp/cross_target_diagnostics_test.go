package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const crossTargetSharedSource = "module storage\n\nfn Use(handle: Handle) int {\n    return handle.Extra()\n}\n"

func writeCrossTargetModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"shared.sec":               crossTargetSharedSource,
		"native.linux.amd64.sec":   "#target(os: \"linux\", arch: \"amd64\")\nmodule storage\n\ntype Handle struct {\n}\n\nimpl Handle {\n    fn Extra() int {\n        return 1\n    }\n}\n",
		"native.windows.amd64.sec": "#target(os: \"windows\", arch: \"amd64\")\nmodule storage\n\ntype Handle struct {\n}\n",
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "shared.sec")
}

// A target-independent document is analyzed for every target of its module;
// a diagnostic that occurs only on some targets states where it applies.
//
// Rules:
//   - rules/tooling/lsp.md — "Multi-target diagnostics"
func TestCrossTargetDiagnosticsRecordApplicability(t *testing.T) {
	path := writeCrossTargetModule(t)
	uri := uriFromPath(path)
	result, ok := computeCrossTargetDiagnostics(uri, crossTargetSharedSource, sourceOverlay{Sources: map[string]string{}})
	if !ok || strings.Join(result.targets, ",") != "linux-amd64,windows-amd64" {
		t.Fatalf("result = %#v, %v", result, ok)
	}
	if len(result.perTarget["linux-amd64"]) != 0 || len(result.perTarget["windows-amd64"]) == 0 {
		t.Fatalf("per target = %#v", result.perTarget)
	}
	merged := mergeCrossTargetDiagnostics(nil, result)
	if len(merged) != 1 || !strings.HasPrefix(merged[0].Message, "[windows-amd64] ") || !strings.Contains(merged[0].Message, "applies to 1 of 2 targets: windows-amd64") {
		t.Fatalf("merged = %#v", merged)
	}
	// The same diagnostic reported by the active analysis gains only the
	// applicability note.
	active := []diagnostic{result.perTarget["windows-amd64"][0]}
	merged = mergeCrossTargetDiagnostics(active, result)
	if len(merged) != 1 || strings.HasPrefix(merged[0].Message, "[") || !strings.Contains(merged[0].Message, "applies to 1 of 2 targets") {
		t.Fatalf("merged active = %#v", merged)
	}
}

func TestCrossTargetDiagnosticsSkipTargetSpecificDocuments(t *testing.T) {
	path := writeCrossTargetModule(t)
	linux := filepath.Join(filepath.Dir(path), "native.linux.amd64.sec")
	data, err := os.ReadFile(linux)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := computeCrossTargetDiagnostics(uriFromPath(linux), string(data), sourceOverlay{Sources: map[string]string{}}); ok {
		t.Fatal("a #target document has only its own target")
	}
}
