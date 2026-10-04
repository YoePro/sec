package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const lensInterfaceSource = "module storage\n\ninterface Native {\n    fn Size() int\n}\n"

func writeLensModule(t *testing.T, windowsSize string) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"native.sec":               lensInterfaceSource,
		"native.linux.amd64.sec":   "#target(os: \"linux\", arch: \"amd64\")\nmodule storage\n\ntype Handle struct {\n}\n\nimpl Handle implements Native {\n    fn Size() int {\n        return 1\n    }\n}\n",
		"native.windows.amd64.sec": "#target(os: \"windows\", arch: \"amd64\")\nmodule storage\n\ntype Handle struct {\n}\n\nimpl Handle implements Native {\n    fn Size(" + windowsSize + ") int {\n        return 1\n    }\n}\n",
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "native.sec")
}

func TestInterfaceConformanceLensSummarizesEveryTarget(t *testing.T) {
	path := writeLensModule(t, "")
	lenses := interfaceConformanceCodeLenses(uriFromPath(path), lensInterfaceSource, sourceOverlay{})
	if len(lenses) != 1 || lenses[0].Command == nil || lenses[0].Command.Title != "✓ Handle conforms on 2 targets" {
		t.Fatalf("lenses = %#v", lenses)
	}
	if lenses[0].Range.Start.Line != 2 || lenses[0].Command.Command != showLocationsCommand {
		t.Fatalf("lens = %#v", lenses[0])
	}
}

func TestInterfaceConformanceLensNamesFailingTargetsAndMembers(t *testing.T) {
	path := writeLensModule(t, "extra: int")
	lenses := interfaceConformanceCodeLenses(uriFromPath(path), lensInterfaceSource, sourceOverlay{})
	if len(lenses) != 1 || lenses[0].Command == nil {
		t.Fatalf("lenses = %#v", lenses)
	}
	title := lenses[0].Command.Title
	if !strings.HasPrefix(title, "✗ conforms on 1 of 2 targets") || !strings.Contains(title, "windows-amd64: Size") || strings.Contains(title, "linux-amd64:") {
		t.Fatalf("title = %q", title)
	}
	failing := lenses[0].Command.Arguments[2].([]location)
	if len(failing) != 1 || !strings.HasSuffix(failing[0].URI, "native.windows.amd64.sec") {
		t.Fatalf("failing locations = %#v", failing)
	}
}
