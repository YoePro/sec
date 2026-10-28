package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runBulkFmtCLI executes the real dispatcher with project-relative arguments.
// Rules: rules/tooling/formatter.md — §28(9)–(13).
func runBulkFmtCLI(t *testing.T, directory string, arguments ...string) (int, string) {
	t.Helper()
	args := append([]string{"-test.run=^TestFmtCheckCLIProcess$", "--", "fmt"}, arguments...)
	command := exec.Command(os.Args[0], args...)
	command.Dir = directory
	command.Env = append(os.Environ(), "SEC_FMT_CHECK_TEST_PROCESS=1")
	output, err := command.CombinedOutput()
	if err == nil {
		return 0, string(output)
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode(), string(output)
	}
	t.Fatal(err)
	return -1, ""
}

// MD-037 completes project-directory --check/--fix: selection is deduplicated,
// check never writes, corrections require opt-in, and malformed siblings remain
// unchanged while valid files are processed. Configurations and modes survive.
// Rules: rules/tooling/formatter.md — §§4(18)–(23),25–27,28(9)–(14).
func TestFmtBulkCheckAndFixCLI(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/bulk_fix_input.sec")
	if err != nil {
		t.Fatal(err)
	}
	broken, err := os.ReadFile("../../testdata/formatter/bulk_balanced_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	manifest, first := writeFmtProject(t, "[format]\nindentation_width = 2\n", fmtDirtySource)
	root := filepath.Dir(filepath.Dir(manifest))
	alias := filepath.Join(root, "nested", "alias.sec")
	bad := filepath.Join(root, "z_broken.sec")
	excluded := filepath.Join(root, "testdata", "fixture.sec")
	writeFmtFile(t, alias, string(input))
	writeFmtFile(t, bad, string(broken))
	writeFmtFile(t, excluded, fmtDirtySource)
	if err := os.Chmod(alias, 0o640); err != nil {
		t.Fatal(err)
	}
	beforeManifest := readFile(t, manifest)
	beforeInfo, err := os.Stat(alias)
	if err != nil {
		t.Fatal(err)
	}
	code, report := runBulkFmtCLI(t, root, "--check", "--fix", ".", "nested/alias.sec", "nested/../nested/alias.sec")
	if code != 1 || strings.Count(report, "nested/alias.sec: format.noncanonical-source:") != 1 || !strings.Contains(report, "z_broken.sec: format.malformed-source:") {
		t.Fatalf("check/fix status=%d:\n%s", code, report)
	}
	afterInfo, err := os.Stat(alias)
	if err != nil {
		t.Fatal(err)
	}
	if readFile(t, alias) != string(input) || !os.SameFile(beforeInfo, afterInfo) || !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
		t.Fatal("check modified source or its identity")
	}
	code, report = runBulkFmtCLI(t, root, "--fix", ".")
	if code != 1 || !strings.Contains(report, "z_broken.sec: format.malformed-source:") {
		t.Fatalf("fix status=%d: %s", code, report)
	}
	if got := readFile(t, alias); !strings.Contains(got, "type UserID uint64") {
		t.Fatalf("correction not applied:\n%s", got)
	}
	if readFile(t, bad) != string(broken) || readFile(t, excluded) != fmtDirtySource || readFile(t, manifest) != beforeManifest {
		t.Fatal("bulk formatting modified malformed/excluded input or manifest")
	}
	if got := readFile(t, first); !strings.Contains(got, "\n  return 1\n") {
		t.Fatalf("project configuration lost:\n%s", got)
	}
	info, err := os.Stat(alias)
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("permissions changed: %v", err)
	}
	if err := os.Remove(bad); err != nil {
		t.Fatal(err)
	}
	if code, report = runBulkFmtCLI(t, root, "--check", "--fix", "."); code != 0 || report != "" {
		t.Fatalf("formatted project not canonical: %d %s", code, report)
	}
	// Ordinary directory mode must not silently enable corrections.
	writeFmtFile(t, alias, string(input))
	if code, report = runBulkFmtCLI(t, root, "."); code != 1 || !strings.Contains(report, "format.malformed-source:") || readFile(t, alias) != string(input) {
		t.Fatalf("ordinary mode repaired syntax: %d %s", code, report)
	}
}

// Changed files must never be replaced with output from an older snapshot.
// Tests simulate edits/replacement after selection without timing-sensitive races.
// Rules: rules/tooling/formatter.md — §28(12)–(13); MD-037 §7(3).
func TestFmtFrozenSelectionRejectsChangedFiles(t *testing.T) {
	for _, kind := range []string{"edit", "replacement", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "source.sec")
			writeFmtFile(t, path, fmtDirtySource)
			selected, err := selectFmtSources([]string{root})
			if err != nil || len(selected) != 1 {
				t.Fatalf("selection: %v", err)
			}
			switch kind {
			case "edit":
				writeFmtFile(t, path, "// editor change\n")
			case "replacement":
				replacement := filepath.Join(root, "new.sec")
				writeFmtFile(t, replacement, fmtDirtySource)
				if err := os.Rename(replacement, path); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target := filepath.Join(root, "target.sec")
				writeFmtFile(t, target, "// target\n")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			before := readFile(t, path)
			err = replaceFormattedFile(selected[0], []byte("// stale format\n"))
			if err == nil || !strings.Contains(err.Error(), "source changed after formatter preflight") {
				t.Fatalf("stale write: %v", err)
			}
			if readFile(t, path) != before {
				t.Fatal("changed file overwritten")
			}
			if kind == "symlink" {
				if _, err := os.Readlink(path); err != nil {
					t.Fatal("symlink replaced")
				}
			}
			leftovers, err := filepath.Glob(filepath.Join(root, ".sec-fmt-*"))
			if err != nil || len(leftovers) != 0 {
				t.Fatalf("temporary output left behind: %v %v", leftovers, err)
			}
		})
	}
}

// Reserved project directories remain excluded when supplied as directory
// roots (including descendants); explicit source files still override discovery
// exclusions. A project-relative '.' remains a real request to scan the project.
// Rules: rules/tooling/formatter.md — §28(9)–(11); MD-037 §§5–6.
func TestFmtExplicitExcludedDirectoryRoots(t *testing.T) {
	manifest, _ := writeFmtProject(t, "[format]\n", fmtDirtySource)
	root := filepath.Dir(filepath.Dir(manifest))
	for _, directory := range []string{"testdata", "bin/generated", ".sec/cache", ".hidden"} {
		path := filepath.Join(root, directory, "input.sec")
		writeFmtFile(t, path, fmtDirtySource)
		if code, report := runBulkFmtCLI(t, root, directory); code != 0 || report != "" {
			t.Fatalf("excluded directory %s: %d %s", directory, code, report)
		}
		if readFile(t, path) != fmtDirtySource {
			t.Fatalf("excluded subtree formatted: %s", directory)
		}
		if code, report := runBulkFmtCLI(t, root, filepath.Join(directory, "input.sec")); code != 0 || report != "" {
			t.Fatalf("explicit file %s: %d %s", directory, code, report)
		}
		if readFile(t, path) == fmtDirtySource {
			t.Fatalf("explicit file not formatted: %s", directory)
		}
	}
}
