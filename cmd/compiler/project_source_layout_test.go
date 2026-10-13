package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Rules: rules/projects/projects.md — §§14–15,48(2).
func TestCLIProjectSourceLayout(t *testing.T) {
	for _, directory := range []string{"cmd", "bin/store", ".sec/state", "services/internal"} {
		t.Run(directory, func(t *testing.T) {
			root := t.TempDir()
			write := func(relative, fixture string) string {
				t.Helper()
				data, err := os.ReadFile("../../testdata/projects/" + fixture)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, relative)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatal(err)
				}
				return path
			}
			write(".sec/sec.toml", "model/sec.toml")
			path := write(filepath.Join(directory, "main.sec"), "init/main.sec")
			_, output, code := runCLIForDiagnostics(t, "sema", path, "--diagnostic-format=json")
			if code == 0 || !strings.Contains(output, "S1149") {
				t.Fatal(code, output)
			}
			document := decodeOccurrenceDocument(t, output)
			if len(document.Occurrences) != 1 || document.Occurrences[0].Primary.Span.File != path {
				t.Fatal(document)
			}
		})
	}
}

// Rules: rules/projects/projects.md — §15.2(4); rules/projects/modules.md — §§3–5,13.
func TestCLIEntrySiblingsAndInternalAccess(t *testing.T) {
	root := t.TempDir()
	write := func(relative, fixture string) string {
		t.Helper()
		data, err := os.ReadFile("../../testdata/projects/" + fixture)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	write(".sec/sec.toml", "model/sec.toml")
	entry := write("cmd/sec/main.sec", "source_layout/entry.sec")
	write("cmd/sec/support.sec", "source_layout/support.sec")
	_, output, code := runCLIForDiagnostics(t, "sema", entry)
	if code != 0 {
		t.Fatal(code, output)
	}
	write("services/internal/store/store.sec", "source_layout/store.sec")
	allowed := write("services/client/client.sec", "source_layout/internal_import.sec")
	_, output, code = runCLIForDiagnostics(t, "sema", allowed)
	if code != 0 {
		t.Fatal(code, output)
	}
	denied := write("client/client.sec", "source_layout/internal_import.sec")
	_, output, code = runCLIForDiagnostics(t, "sema", denied, "--diagnostic-format=json")
	if code == 0 || !strings.Contains(output, "S1149") {
		t.Fatal(code, output)
	}
	write("bin/store/store.sec", "source_layout/store.sec")
	denied = write("client/client.sec", "source_layout/bin_import.sec")
	_, output, code = runCLIForDiagnostics(t, "sema", denied, "--diagnostic-format=json")
	if code == 0 || !strings.Contains(output, "S1149") {
		t.Fatal(code, output)
	}
	// Validation never removes user files from reserved output storage.
	if _, err := os.Stat(filepath.Join(root, "bin/store/store.sec")); err != nil {
		t.Fatal(err)
	}
}
