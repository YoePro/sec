package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Arbitrary canonical stdlib imports load their associated functions, while
// unresolved imports and missing static methods have distinct structured IDs.
// Rules: rules/projects/modules.md — §§3,8,9,21;
// rules/declarations/static.md — type-level methods.
func TestCLIImportedAssociatedFunctionAndDiagnostics(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"main.sec", "sec/stdlib/net/ip/ipv6.sec", "sec/stdlib/net/ip/ip_test.sec"} {
		data, err := os.ReadFile(filepath.Join("../../testdata/import_static", relative))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	otherOS := "windows"
	if runtime.GOOS == "windows" {
		otherOS = "linux"
	}
	inactive := filepath.Join(root, "sec", "stdlib", "net", "ip", "inactive.sec")
	if err := os.WriteFile(inactive, []byte("#target(os: \""+otherOS+"\", arch: \"any\")\nmodule other\n"), 0600); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "main.sec")
	data, _ := os.ReadFile(path)
	for _, test := range []struct{ source, id string }{
		{string(data), ""},
		{strings.ReplaceAll(string(data), ".Parse(", ".Missing("), "S1157"},
		{strings.ReplaceAll(string(data), `"net/ip"`, `"net/absent"`), "S1051"},
	} {
		if err := os.WriteFile(path, []byte(test.source), 0600); err != nil {
			t.Fatal(err)
		}
		_, diagnostics, code := runCLIForDiagnostics(t, "sema", path, "--diagnostic-format=json")
		if test.id == "" {
			if code != 0 {
				t.Fatal(diagnostics)
			}
			continue
		}
		items := decodeOccurrenceDocument(t, diagnostics).Occurrences
		if code == 0 || len(items) != 1 || items[0].ID == nil || *items[0].ID != test.id {
			t.Fatal(code, items)
		}
	}
	legacy := filepath.Join(root, "sec", "stdlib", "net", "ip", "zlegacy.sec")
	if err := os.WriteFile(legacy, []byte("module net\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	_, text, code := runCLIForDiagnostics(t, "sema", path, "--diagnostic-format=json")
	issues := decodeOccurrenceDocument(t, text).Occurrences
	if code == 0 || len(issues) != 1 || issues[0].ID == nil || *issues[0].ID != "S1158" {
		t.Fatal(code, issues)
	}

}
