package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Editor import discovery and static lookup must agree with CLI and retain
// explanatory diagnostics instead of treating failed imports as absent values.
// Rules: rules/projects/modules.md — §§3,8,9,21;
// rules/tooling/lsp.md — Diagnostics and snapshots.
func TestLSPImportedAssociatedFunctionAndDiagnostics(t *testing.T) {
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
		items := analyze(uriFromPath(path), test.source)
		if test.id == "" {
			if len(items) != 0 {
				t.Fatal(items)
			}
			continue
		}
		if len(items) != 1 || items[0].Code != test.id || !strings.Contains(items[0].Message, "help:") {
			t.Fatal(items)
		}
	}
	legacy := filepath.Join(root, "sec", "stdlib", "net", "ip", "zlegacy.sec")
	if err := os.WriteFile(legacy, []byte("module net\n"), 0600); err != nil {
		t.Fatal(err)
	}

	items := analyze(uriFromPath(path), string(data))
	if len(items) != 1 || items[0].Code != "S1158" || len(items[0].RelatedInformation) != 1 || !strings.Contains(items[0].Message, "module net") {
		t.Fatal(items)
	}

}
