package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The CLI selects active source for its plan: a file-level @target for
// another OS is excluded, @target variants and @when read the plan's target
// and the manifest's typed [parameters], and the built program runs the
// selected declarations only.
// Rules: rules/foundations/attributes.md — §§ 13, 15–17, 21, 26–28; rules/projects/projects.md — § 24.
func TestCLISourceSelection(t *testing.T) {
	if _, err := os.Stat(filepath.Join(os.Getenv("SEC_MLIR_BIN"), "sec-mlir-opt")); err != nil {
		t.Skip("SEC_MLIR_BIN with sec-mlir-opt is required")
	}
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang is required")
	}
	root := t.TempDir()
	files := map[string]string{
		".sec/sec.toml": "[project]\nname = \"selection\"\n\n[version]\nmajor = 1\nminor = 0\nrevision = 0\nbuild = 0\nformat = '[major].[minor].[revision].[build]'\n\n[parameters]\naudit = true\nlevel = 2\n",
		"main.sec": `module main

@target(os: "linux")
fn Base() int {
    return 40
}

@target(os: "windows")
fn Base() int {
    return 90
}

@when(config.audit && config.level == 2)
fn Extra() int {
    return 2
}

@when(!config.audit)
fn Extra() int {
    return 50
}

fn main() int {
    return Base() + Extra() + Other()
}
`,
		"other.sec":   "@target(os: \"linux\")\n\nmodule main\n\nfn Other() int {\n    return 0\n}\n",
		"windows.sec": "@target(os: \"windows\")\n\nmodule main\n\nfn Other() int {\n    return 7\n}\n",
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
	output := filepath.Join(root, "program")
	_, stderr, code := runCLIForDiagnostics(t, "build", "--target", "linux-amd64", filepath.Join(root, "main.sec"), "-o", output)
	if code != 0 {
		t.Fatalf("build: %d %s", code, stderr)
	}
	err := exec.Command(output).Run()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 42 {
		t.Fatalf("selected program exit = %v, want 42", err)
	}
	broken := strings.Replace(files["main.sec"], "config.level == 2", "config.missing", 1)
	if err := os.WriteFile(filepath.Join(root, "main.sec"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code = runCLIForDiagnostics(t, "check", filepath.Join(root, "main.sec"))
	if code == 0 || !strings.Contains(stderr, "S1236") || !strings.Contains(stderr, "unknown compile-time parameter config.missing") {
		t.Fatalf("unknown parameter: %d %s", code, stderr)
	}
}
