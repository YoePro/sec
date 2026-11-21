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

type cBindingsCLIResult struct {
	Format      string `json:"format"`
	Version     int    `json:"version"`
	Operation   string `json:"operation"`
	Status      string `json:"status"`
	Changed     bool   `json:"changed"`
	Reason      string `json:"reason"`
	BeforeSHA   string `json:"before_sha256"`
	AfterSHA    string `json:"after_sha256"`
	Diagnostics []struct {
		ID string `json:"id"`
	} `json:"diagnostics"`
}

func decodeCBindingsResult(t *testing.T, out string) cBindingsCLIResult {
	t.Helper()
	var result cBindingsCLIResult
	decoder := json.NewDecoder(strings.NewReader(out))
	if err := decoder.Decode(&result); err != nil || decoder.More() {
		t.Fatalf("not exactly one JSON document: %v\n%s", err, out)
	}
	return result
}

// MD-021 §§ 4(3)-(4), 5(2)-(6), 5(9): every mutation, no-op idempotence,
// stale-hash rejection, failed-edit rollback, comment preservation,
// permission preservation and the stable JSON result contract.
func TestPlatformCBindingsMutations(t *testing.T) {
	file := filepath.Join(t.TempDir(), ".sec", "platform", "c-bindings.toml")
	if code, out := runCBindings(t, "init", "--id", "sec.hosted-c", "--version", "1.0.0", "--source-root", "sec/platform", "--file", file); code != 0 {
		t.Fatalf("init = %d %s", code, out)
	}
	if info, err := os.Stat(file); err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("init mode = %v %v", info, err)
	}
	original, _ := os.ReadFile(file)
	commented := strings.Replace(string(original), "[bindings]\n", "# keep this comment\n[bindings]\n", 1)
	if err := os.WriteFile(file, []byte(commented), 0o640); err != nil || os.Chmod(file, 0o640) != nil {
		t.Fatal(err)
	}

	steps := []struct {
		args    []string
		changed bool
		reason  string
	}{
		{[]string{"environment", "add", "linux-amd64-glibc", "--os", "linux", "--arch", "amd64", "--abi", "gnu", "--c-runtime", "glibc", "--c-runtime-versions", ">=2.31, <3.0"}, true, ""},
		{[]string{"environment", "add", "linux-amd64-glibc", "--os", "linux", "--arch", "amd64", "--abi", "gnu", "--c-runtime", "glibc", "--c-runtime-versions", ">=2.31, <3.0"}, false, "already-present"},
		{[]string{"environment", "add", "windows-amd64-ucrt", "--os", "windows", "--arch", "amd64", "--abi", "msvc", "--c-runtime", "ucrt"}, true, ""},
		{[]string{"module", "add", "c/common", "--common"}, true, ""},
		{[]string{"module", "add", "c/common", "--common"}, false, "already-present"},
		{[]string{"module", "add", "c/linux/glibc/amd64", "--environment", "linux-amd64-glibc"}, true, ""},
		{[]string{"module", "add", "c/windows/ucrt", "--environment", "windows-amd64-ucrt"}, true, ""},
		{[]string{"environment", "set", "linux-amd64-glibc", "--clear-c-runtime-versions"}, true, ""},
		{[]string{"package", "set", "--version", "1.1.0"}, true, ""},
		{[]string{"module", "remove", "c/windows/ucrt", "--environment", "windows-amd64-ucrt"}, true, ""},
		{[]string{"module", "remove", "c/windows/ucrt", "--environment", "windows-amd64-ucrt"}, false, "not-present"},
		{[]string{"environment", "remove", "windows-amd64-ucrt"}, true, ""},
		{[]string{"environment", "remove", "windows-amd64-ucrt"}, false, "not-present"},
	}
	for _, step := range steps {
		before, _ := os.ReadFile(file)
		args := append(append([]string{}, step.args...), "--file", file, "--json", "--if-match", *fileSHA256(before, true))
		code, out := runCBindings(t, args...)
		result := decodeCBindingsResult(t, out)
		after, _ := os.ReadFile(file)
		if code != 0 || result.Format != "sec.c-bindings-cli" || result.Version != 1 || result.Status != "complete" ||
			result.Operation != step.args[0]+"."+step.args[1] || result.Changed != step.changed || result.Reason != step.reason ||
			result.BeforeSHA != *fileSHA256(before, true) || result.AfterSHA != *fileSHA256(after, true) || bytes.Equal(before, after) == step.changed {
			t.Fatalf("%v = %d %s", step.args, code, out)
		}
	}
	final, _ := os.ReadFile(file)
	want := "schema_version = 1\n\n# keep this comment\n[bindings]\nid = \"sec.hosted-c\"\nversion = \"1.1.0\"\nsource_root = \"sec/platform\"\n\n" +
		"[modules]\ncommon = [\"c/common\"]\n\n[environments.linux-amd64-glibc]\nos = \"linux\"\narch = \"amd64\"\nabi = \"gnu\"\nc_runtime = \"glibc\"\nmodules = [\"c/linux/glibc/amd64\"]\n"
	if string(final) != want {
		t.Fatalf("final file:\n%s\nwant:\n%s", final, want)
	}
	if info, _ := os.Stat(file); info.Mode().Perm() != 0o640 {
		t.Fatalf("edit changed the file mode to %v", info.Mode().Perm())
	}

	rejected := []struct {
		args []string
		id   string
	}{
		{[]string{"module", "add", "c/x", "--common", "--if-match", strings.Repeat("0", 64)}, "c-bindings.stale-file"},
		{[]string{"environment", "add", "linux-amd64-glibc", "--os", "linux", "--arch", "arm64", "--abi", "gnu", "--c-runtime", "glibc"}, "c-bindings.environment-conflict"},
		{[]string{"environment", "set", "linux-amd64-glibc", "--os", "Linux"}, "c-bindings.unknown-target"},
		{[]string{"environment", "set", "linux-amd64-glibc", "--os", "darwin"}, "c-bindings.noncanonical-target"},
		{[]string{"module", "add", "../outside", "--common"}, "c-bindings.unsafe-path"},
		{[]string{"module", "add", "c/x", "--environment", "absent"}, "c-bindings.unknown-environment"},
	}
	for _, test := range rejected {
		code, out := runCBindings(t, append(append([]string{}, test.args...), "--file", file, "--json")...)
		result := decodeCBindingsResult(t, out)
		if after, _ := os.ReadFile(file); code != 2 || result.Status != "rejected" || result.Changed || len(result.Diagnostics) == 0 ||
			result.Diagnostics[0].ID != test.id || !bytes.Equal(after, final) {
			t.Fatalf("%v = %d %s", test.args, code, out)
		}
	}
	missing := filepath.Join(filepath.Dir(file), "absent.toml")
	if code, out := runCBindings(t, "module", "add", "c/x", "--common", "--file", missing, "--json"); code != 2 || !strings.Contains(out, "c-bindings.no-file") {
		t.Fatalf("missing file = %d %s", code, out)
	}
	for _, usage := range [][]string{
		{"module", "add", "c/x"},
		{"module", "add", "c/x", "--common", "--environment", "e"},
		{"environment", "add", "e", "--os", "linux"},
		{"environment", "set", "e"},
		{"environment", "set", "e", "--c-runtime-versions", ">=1.0", "--clear-c-runtime-versions"},
		{"environment", "remove", "e", "--os", "linux"},
		{"package", "set"},
		{"module", "add"},
		{"environment"},
	} {
		if code, _ := runCBindings(t, append(append([]string{}, usage...), "--file", file)...); code != 1 {
			t.Fatalf("usage %v = %d", usage, code)
		}
	}
	if after, _ := os.ReadFile(file); !bytes.Equal(after, final) {
		t.Fatal("a rejected command changed the file")
	}
}

// MD-021 § 5(2): the re-hash immediately before the rename rejects a file
// that changed after it was read, and nothing is written.
func TestReplaceFileAtomicallyRejectsConcurrentChange(t *testing.T) {
	file := filepath.Join(t.TempDir(), "c-bindings.toml")
	if err := os.WriteFile(file, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := fileSHA256([]byte("older"), true)
	if err := replaceFileAtomically(file, []byte("edited"), stale); err != errStaleFile {
		t.Fatalf("stale replace = %v", err)
	}
	if data, _ := os.ReadFile(file); string(data) != "first" {
		t.Fatalf("file = %q", data)
	}
	entries, _ := os.ReadDir(filepath.Dir(file))
	if len(entries) != 1 {
		t.Fatalf("temporary file left behind: %v", entries)
	}
}
