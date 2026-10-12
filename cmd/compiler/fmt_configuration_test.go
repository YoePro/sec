package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFmtProject(t *testing.T, manifest string, source string) (string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".sec"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, ".sec", "sec.toml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(root, "main.sec")
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return manifestPath, sourcePath
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// sec fmt resolves each file's options from explicit invocation options, its
// project's [format] table and the defaults; an explicit option never
// rewrites the manifest, and an invalid configuration stops the command
// before any file is modified. Language corrections stay off unless the
// project or the invocation enables them.
//
// Rules:
//   - rules/tooling/formatter.md — § 4(17)–(23)
//   - rules/corrections/applied/md027-md033-md036-md054-contextual-literals-formatter-configuration-correction-20261012.md — § 5.3
func TestFmtCommandUsesProjectFormatConfiguration(t *testing.T) {
	source := "module main\n\nfn F() int {\nreturn 1\n}\n"
	manifest := "[format]\nindentation_width = 4\n"
	manifestPath, sourcePath := writeFmtProject(t, "[format]\nindentation_width = 2\n", source)
	if err := runFmtCommand([]string{sourcePath}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, sourcePath); !strings.Contains(got, "\n  return 1\n") {
		t.Fatalf("project indentation_width = 2 not applied:\n%s", got)
	}

	manifestPath, sourcePath = writeFmtProject(t, manifest, source)
	if err := runFmtCommand([]string{"--indentation-width", "2", sourcePath}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, sourcePath); !strings.Contains(got, "\n  return 1\n") {
		t.Fatalf("--indentation-width 2 did not override the project:\n%s", got)
	}
	if got := readFile(t, manifestPath); got != manifest {
		t.Fatalf("explicit option rewrote the manifest:\n%s", got)
	}

	_, valid := writeFmtProject(t, manifest, source)
	_, invalid := writeFmtProject(t, "[format]\nformat_version = 2\n", source)
	err := runFmtCommand([]string{valid, invalid})
	if err == nil || !strings.Contains(err.Error(), "unsupported format_version 2") {
		t.Fatalf("unsupported format_version = %v", err)
	}
	if readFile(t, valid) != source || readFile(t, invalid) != source {
		t.Fatal("an invalid configuration modified a source file")
	}

	_, plain := writeFmtProject(t, "[format]\n", "module main\n\nenum State { Idle, Busy }\n\nfn F(state: State) bool {\n    return (state is Idle) == true\n}\n")
	if err := runFmtCommand([]string{plain}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, plain); !strings.Contains(got, "(state is Idle) == true") {
		t.Fatalf("language corrections ran without being enabled:\n%s", got)
	}
	_, enabled := writeFmtProject(t, "[format]\nlanguage_corrections = true\n", "module main\n\nenum State { Idle, Busy }\n\nfn F(state: State) bool {\n    return (state is Idle) == true\n}\n")
	if err := runFmtCommand([]string{enabled}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, enabled); !strings.Contains(got, "return state is Idle\n") {
		t.Fatalf("project-enabled language corrections did not run:\n%s", got)
	}
}
