package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runInitModuleCLI(t *testing.T, dir, path string) (string, error) {
	t.Helper()
	return runInitCLIInDirectory(t, dir, "module", path)
}

func runInitCLIInDirectory(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=^TestLexerCLIProcess$", "--", "init"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "SEC_LEXER_CLI_TEST_PROCESS=1")
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// Rules: rules/projects/projects.md — §§38,40,42,45; rules/projects/modules.md — §§3,4,7.
func TestInitModuleCLI(t *testing.T) {
	root := t.TempDir()
	_, stderr, code := runCLIForDiagnostics(t, "init", root, "--minimal", "--name", "Example")
	if code != 0 {
		t.Fatal(code, stderr)
	}
	manifest := filepath.Join(root, ".sec", "sec.toml")
	before, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"inventory/orders", "inventory/internal/storage", "type", "unicode/räksmörgås"} {
		output, err := runInitModuleCLI(t, root, path)
		if err != nil || !strings.Contains(output, "initialized Sec module") {
			t.Fatal(output, err)
		}
		name := filepath.Base(path)
		source := filepath.Join(root, filepath.FromSlash(path), name+".sec")
		got, err := os.ReadFile(source)
		if err != nil || string(got) != "module "+name+"\n" {
			t.Fatal(string(got), err)
		}
		output, err = runInitModuleCLI(t, filepath.Join(root, "inventory"), path)
		if err != nil || !strings.Contains(output, "already exists") {
			t.Fatal(output, err)
		}
	}
	user, err := os.ReadFile("../../testdata/projects/init_module/orders/orders.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "inventory", "orders", "orders.sec")
	if err := os.WriteFile(source, user, 0644); err != nil {
		t.Fatal(err)
	}
	if output, err := runInitModuleCLI(t, root, "inventory/orders"); err != nil {
		t.Fatal(output, err)
	}
	got, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(got, user) {
		t.Fatal("overwrote user source", err)
	}
	after, err := os.ReadFile(manifest)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("module creation changed manifest", err)
	}
}

// Rules: rules/projects/projects.md — §§15,38,42–43; rules/projects/modules.md — §§4,7,8.
func TestInitModuleRejectsPathsAndConflicts(t *testing.T) {
	root := t.TempDir()
	_, stderr, code := runCLIForDiagnostics(t, "init", root, "--minimal", "--name", "Example")
	if code != 0 {
		t.Fatal(code, stderr)
	}
	for _, path := range []string{"/absolute", "../escape", "a/../bad", "a//bad", "a/", "a/./bad", `a\bad`, "C:/bad", "bin/orders", ".sec/orders", "a/.sec/orders", "platform/orders", "cmd", "internal", "domain/internal", "a/bad-name", "a/ra\u0308ks"} {
		if output, err := runInitModuleCLI(t, root, path); err == nil {
			t.Fatalf("accepted %q: %s", path, output)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 {
		t.Fatal("invalid path created directories", entries, err)
	}
	if err := os.MkdirAll(filepath.Join(root, "inventory", "orders"), 0755); err != nil {
		t.Fatal(err)
	}
	conflict, err := os.ReadFile("../../testdata/projects/init_module/conflict/conflict.sec")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inventory", "orders", "other.sec"), conflict, 0644); err != nil {
		t.Fatal(err)
	}
	if output, err := runInitModuleCLI(t, root, "inventory/orders"); err == nil || !strings.Contains(output, "source conflict") {
		t.Fatal(output, err)
	}
	if fileExists(filepath.Join(root, "inventory", "orders", "orders.sec")) {
		t.Fatal("conflict created starter")
	}
	if err := os.WriteFile(filepath.Join(root, "blocked"), []byte("user file"), 0644); err != nil {
		t.Fatal(err)
	}
	if output, err := runInitModuleCLI(t, root, "blocked/orders"); err == nil {
		t.Fatal(output)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if output, err := runInitModuleCLI(t, root, "linked/orders"); err == nil {
		t.Fatal(output)
	}
	if fileExists(filepath.Join(outside, "orders")) {
		t.Fatal("escaped project through symlink")
	}
	if output, err := runInitModuleCLI(t, outside, "orders"); err == nil {
		t.Fatal(output)
	}
}

// Rules: rules/projects/projects.md — §§4,7,38,42–43.
func TestInitModuleManifestPreflight(t *testing.T) {
	root := t.TempDir()
	_, stderr, code := runCLIForDiagnostics(t, "init", root, "--minimal", "--name", "Example")
	if code != 0 {
		t.Fatal(code, stderr)
	}
	manifest := filepath.Join(root, ".sec", "sec.toml")
	original, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, contents := range []string{"[project]\nname = 'Example'\nuuid = '00000000-0000-0000-0000-000000000000'\n", "[project]\nname = 'Example'\n"} {
		if err := os.WriteFile(manifest, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
		if output, err := runInitModuleCLI(t, root, "inventory/orders"); err == nil {
			t.Fatal(output)
		}
		if fileExists(filepath.Join(root, "inventory")) {
			t.Fatal("invalid manifest created source directories")
		}
	}
	if err := os.WriteFile(manifest, original, 0644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "inventory", ".sec")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "sec.toml"), original, 0644); err != nil {
		t.Fatal(err)
	}
	if output, err := runInitModuleCLI(t, root, "inventory/orders"); err == nil || !strings.Contains(output, "nested project") {
		t.Fatal(output, err)
	}
	if fileExists(filepath.Join(root, "inventory", "orders")) {
		t.Fatal("nested boundary created sources")
	}
}
