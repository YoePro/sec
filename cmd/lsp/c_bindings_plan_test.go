package main

import (
	"os"
	"path/filepath"
	"testing"

	platformtarget "sec/internal/platform/target"
)

// MD-021 § 8.6(5) and lsp.md "Target-aware analysis": the editor resolves
// the same Variant binding environment as the CLI, so a project source using
// c:: aliases and the binding module itself analyze without S1078/S1248.
func TestLSPUsesVariantCBindingEnvironment(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		".sec/sec.toml": "[project]\nname = \"demo\"\n\n[version]\nmajor = 1\nminor = 0\nrevision = 0\nbuild = 0\nformat = \"[major].[minor].[revision]\"\n\n" +
			"[variant.linux-amd64]\nos = \"linux\"\narch = \"amd64\"\nabi = \"gnu\"\nc_runtime = \"glibc\"\nc_bindings = { id = \"sec.hosted-c\" }\n",
		".sec/platform/c-bindings.toml": "schema_version = 1\n\n[bindings]\nid = \"sec.hosted-c\"\nversion = \"1.0.0\"\nsource_root = \"sec/platform\"\n\n" +
			"[environments.linux-amd64-glibc]\nos = \"linux\"\narch = \"amd64\"\nabi = \"gnu\"\nc_runtime = \"glibc\"\nmodules = [\"c/linux\"]\n",
		"sec/platform/c/linux/linux.sec": "module linux\n\n@cAlias(name: \"c::stddef::size_t\", target: c::ulong)\n",
		"app/app.sec":                    "module app\n\nfn Use() void {\n    let size: c::stddef::size_t := 1\n    discard size\n}\n",
	}
	for name, text := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	linux := platformtarget.Target{OS: "linux", Arch: "amd64"}
	for _, name := range []string{"app/app.sec", "sec/platform/c/linux/linux.sec"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		for _, d := range semanticDiagnosticsForTarget("file://"+path, files[name], sourceOverlay{}, linux) {
			if d.Severity == 1 {
				t.Fatalf("%s: unexpected error %s %s", name, d.Code, d.Message)
			}
		}
	}
	windows := platformtarget.Target{OS: "windows", Arch: "amd64"}
	path := filepath.Join(root, "app", "app.sec")
	found := false
	for _, d := range semanticDiagnosticsForTarget("file://"+path, files["app/app.sec"], sourceOverlay{}, windows) {
		found = found || d.Code == "S1078"
	}
	if !found {
		t.Fatal("a target without a Variant must keep S1078")
	}
}
