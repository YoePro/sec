package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCBindings(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runPlatformCommand(append([]string{"c-bindings"}, args...), &stdout, &stderr)
	return code, stdout.String() + stderr.String()
}

// MD-021 §§ 4-5, 8.9 and cli.md § 8: init, show, validate and resolve.
func TestPlatformCBindingsReadOnlyCommands(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".sec"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `[project]
name = "demo"

[variant.linux-amd64]
os = "linux"
arch = "amd64"
abi = "gnu"
c_runtime = "glibc"
c_runtime_version = "2.39"

[variant.windows-amd64]
os = "windows"
arch = "amd64"
abi = "msvc"
c_runtime = "ucrt"
`
	if err := os.WriteFile(filepath.Join(root, ".sec", "sec.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, ".sec", "platform", "c-bindings.toml")

	if code, out := runCBindings(t, "init", "--id", "sec.hosted-c", "--version", "1.0.0", "--source-root", "sec/platform", "--file", file, "--json"); code != 0 || !strings.Contains(out, `"changed": true`) {
		t.Fatalf("init = %d %s", code, out)
	}
	if code, out := runCBindings(t, "init", "--id", "sec.hosted-c", "--version", "1.0.0", "--source-root", "sec/platform", "--file", file, "--json"); code != 0 || !strings.Contains(out, `"changed": false`) {
		t.Fatalf("repeated init = %d %s", code, out)
	}
	before, _ := os.ReadFile(file)
	if code, out := runCBindings(t, "init", "--id", "other", "--version", "1.0.0", "--source-root", "sec/platform", "--file", file, "--json"); code != 2 || !strings.Contains(out, "c-bindings.file-exists") {
		t.Fatalf("incompatible init = %d %s", code, out)
	}
	if after, _ := os.ReadFile(file); !bytes.Equal(before, after) {
		t.Fatal("a rejected init changed the file")
	}
	if code, out := runCBindings(t, "validate", "--file", file, "--json"); code != 0 || !strings.Contains(out, `"scope": "structural"`) || !strings.Contains(out, `"resolvable": false`) {
		t.Fatalf("draft validate = %d %s", code, out)
	}

	full := string(before) + `
[modules]
common = ["c/common"]

[environments.linux-amd64-glibc]
os = "linux"
arch = "amd64"
abi = "gnu"
c_runtime = "glibc"
c_runtime_versions = ">=2.31, <3.0"
modules = ["c/linux/glibc/amd64"]
`
	if err := os.WriteFile(file, []byte(full), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := runCBindings(t, "show", "--file", file, "--json")
	var shown struct {
		FileSHA string `json:"file_sha256"`
		Package struct {
			ID           string                     `json:"id"`
			Environments map[string]json.RawMessage `json:"environments"`
		} `json:"package"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &shown) != nil || len(shown.FileSHA) != 64 || shown.Package.ID != "sec.hosted-c" || len(shown.Package.Environments) != 1 {
		t.Fatalf("show = %d %s", code, out)
	}
	if code, out := runCBindings(t, "validate", "--file", file, "--variant", "linux-amd64", "--json"); code != 0 || !strings.Contains(out, `"scope": "variant"`) || !strings.Contains(out, `"environment": "linux-amd64-glibc"`) {
		t.Fatalf("variant validate = %d %s", code, out)
	}
	code, out = runCBindings(t, "resolve", "--file", file, "--variant", "linux-amd64", "--json")
	var report struct {
		Format, Status string
		Binding        *struct {
			Environment string   `json:"environment"`
			Modules     []string `json:"modules"`
			Fingerprint string   `json:"fingerprint"`
		}
	}
	if code != 0 || json.Unmarshal([]byte(out), &report) != nil || report.Format != "sec.c-bindings-resolution" || report.Binding == nil ||
		strings.Join(report.Binding.Modules, ",") != "c/common,c/linux/glibc/amd64" || len(report.Binding.Fingerprint) != 64 {
		t.Fatalf("resolve = %d %s", code, out)
	}
	if code, out := runCBindings(t, "resolve", "--file", file, "--variant", "windows-amd64", "--json"); code != 2 || !strings.Contains(out, `"binding": null`) || !strings.Contains(out, "c-bindings.no-candidate") {
		t.Fatalf("incompatible resolve = %d %s", code, out)
	}
	if code, _ := runCBindings(t, "resolve", "--file", file); code != 1 {
		t.Fatalf("resolve without --variant = %d", code)
	}
}
