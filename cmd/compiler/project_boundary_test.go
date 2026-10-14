package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Rules: rules/projects/projects.md — §§4,36,48 (discovery before writes/lowering).
func TestCLIProjectBoundaryStopsSourceAndTargetCommands(t *testing.T) {
	root := t.TempDir()
	manifest, err := os.ReadFile("../../testdata/projects/boundary/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../testdata/projects/boundary/main.sec")
	if err != nil {
		t.Fatal(err)
	}
	write := func(path string, data []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	outer := filepath.Join(root, ".sec", "sec.toml")
	child := filepath.Join(root, "child", ".sec", "sec.toml")
	parentSource := filepath.Join(root, "cmd", "sec", "main.sec")
	childSource := filepath.Join(root, "child", "cmd", "sec", "main.sec")
	output := filepath.Join(root, "out.ll")
	write(outer, manifest)
	write(child, manifest)
	write(parentSource, source)
	write(childSource, source)
	write(output, []byte("existing user output"))
	for _, path := range []string{parentSource, childSource} {
		for _, args := range [][]string{{"sema", path}, {"check", path}, {"analyse", path}, {"emit-ir", path}, {"emit-llvm", path, "-o", output}, {"build", path, "-o", output}, {"test", "--list", filepath.Dir(path)}} {
			args = append(args, "--diagnostic-format=json")
			_, data, code := runCLIForDiagnostics(t, args...)
			if code == 0 {
				t.Fatal(args, data)
			}
			items := decodeOccurrenceDocument(t, data).Occurrences
			if len(items) != 1 || items[0].ID == nil || *items[0].ID != "S1154" || items[0].Primary == nil || items[0].Primary.Span.File != child || items[0].Primary.Span.Start.Line != 2 {
				t.Fatal(args, items)
			}
		}
	}
	data, err := runBuildCLIInDirectory(t, root, "sec", "--diagnostic-format=json")
	if err == nil || !bytes.Contains([]byte(data), []byte("S1154")) {
		t.Fatal(data, err)
	}
	for _, path := range []string{outer, child} {
		data, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(data, manifest) {
			t.Fatal("validation changed manifest", path, err)
		}
	}
	dataBytes, err := os.ReadFile(output)
	if err != nil || string(dataBytes) != "existing user output" {
		t.Fatal("validation overwrote user output", err)
	}
}
