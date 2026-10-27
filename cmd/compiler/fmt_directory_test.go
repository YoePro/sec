package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	fmtDirtySource     = "module main\n\nfn F() int {\nreturn 1\n}\n"
	fmtMalformedSource = "fn Broken() void {\n\tlet values := [1, 2\n"
)

func writeFmtFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// sec fmt expands directories recursively (MD-037): every exact `.sec` file
// of the explicit subtree, tests and internal modules included, formatted
// with the project's [format] table. Hidden directories, testdata, the
// project-reserved bin and .sec, non-Sec files and symbolic links are not
// touched, while an explicitly named fixture still formats. Overlapping
// arguments format a file once, and the check report is sorted.
//
// Rules:
//   - rules/tooling/formatter.md — §§ 28(9)–28(13)
//   - rules/corrections/MD-099-MD-100-MD-037-formatter-alignment-and-selection-correction.md — §§ 5–8, 10(3)–(5)
func TestFmtSelectsProjectDirectories(t *testing.T) {
	manifestPath, _ := writeFmtProject(t, "[format]\nindentation_width = 2\n", fmtDirtySource)
	root := filepath.Dir(filepath.Dir(manifestPath))
	selected := []string{"main.sec", "lib/util/util.sec", "lib/util/util_test.sec", "internal/state/state.sec", "unused/unused.sec"}
	excluded := []string{"testdata/fixture.sec", "bin/out.sec", ".hidden/hidden.sec", ".sec/state/cache.sec", "notes.txt", "lib/util/util.sec.bak"}
	for _, path := range append(selected[1:], excluded...) {
		writeFmtFile(t, filepath.Join(root, path), fmtDirtySource)
	}
	if err := os.Symlink(filepath.Join(root, "lib"), filepath.Join(root, "lib", "loop")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "main.sec"), filepath.Join(root, "alias.sec")); err != nil {
		t.Fatal(err)
	}

	err := runFmtCommand([]string{"--check", root, filepath.Join(root, "main.sec")})
	if err == nil {
		t.Fatal("check reported no noncanonical files")
	}
	var want []string
	for _, path := range []string{"internal/state/state.sec", "lib/util/util.sec", "lib/util/util_test.sec", "main.sec", "unused/unused.sec"} {
		want = append(want, filepath.Join(root, path)+": format.noncanonical-source: formatting would change this file")
	}
	if err.Error() != strings.Join(want, "\n") {
		t.Fatalf("check report:\n%s\nwant:\n%s", err, strings.Join(want, "\n"))
	}
	for _, path := range append(selected, excluded...) {
		if readFile(t, filepath.Join(root, path)) != fmtDirtySource {
			t.Fatalf("--check modified %s", path)
		}
	}

	if err := runFmtCommand([]string{root}); err != nil {
		t.Fatal(err)
	}
	for _, path := range selected {
		if got := readFile(t, filepath.Join(root, path)); !strings.Contains(got, "\n  return 1\n") {
			t.Errorf("%s not formatted with the project's indentation_width = 2:\n%s", path, got)
		}
	}
	for _, path := range excluded {
		if readFile(t, filepath.Join(root, path)) != fmtDirtySource {
			t.Errorf("directory scan modified excluded %s", path)
		}
	}
	if target, err := os.Readlink(filepath.Join(root, "alias.sec")); err != nil || target != filepath.Join(root, "main.sec") {
		t.Errorf("the symbolic link was replaced: %q %v", target, err)
	}
	if err := runFmtCommand([]string{filepath.Join(root, "testdata", "fixture.sec")}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(root, "testdata", "fixture.sec")); !strings.Contains(got, "\n  return 1\n") {
		t.Errorf("an explicitly named fixture was not formatted:\n%s", got)
	}
}

// Directory inputs fail closed: an explicit symbolic link, a missing path and
// a nested project boundary are diagnostics with no writes, an invalid
// configuration found in preflight stops every write, an empty directory
// succeeds without edits, and a scan outside any project uses the defaults.
//
// Rules:
//   - rules/tooling/formatter.md — §§ 28(10)–28(13)
func TestFmtDirectoryPreflight(t *testing.T) {
	standalone := t.TempDir()
	dirty := filepath.Join(standalone, "a", "dirty.sec")
	writeFmtFile(t, dirty, fmtDirtySource)

	link := filepath.Join(standalone, "link.sec")
	if err := os.Symlink(dirty, link); err != nil {
		t.Fatal(err)
	}
	if err := runFmtCommand([]string{link}); err == nil || !strings.Contains(err.Error(), "symbolic links are not followed") {
		t.Fatalf("explicit symbolic link: %v", err)
	}
	if err := runFmtCommand([]string{filepath.Join(standalone, "missing")}); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing path: %v", err)
	}

	_, invalid := writeFmtProject(t, "[format]\nformat_version = 2\n", fmtDirtySource)
	if err := runFmtCommand([]string{standalone, invalid}); err == nil || !strings.Contains(err.Error(), "unsupported format_version 2") {
		t.Fatalf("invalid configuration: %v", err)
	}
	if readFile(t, dirty) != fmtDirtySource {
		t.Fatal("a file was written before preflight failed")
	}

	nestedManifest, _ := writeFmtProject(t, "[format]\n", fmtDirtySource)
	outer := filepath.Dir(filepath.Dir(nestedManifest))
	writeFmtFile(t, filepath.Join(outer, "inner", ".sec", "sec.toml"), "[format]\n")
	writeFmtFile(t, filepath.Join(outer, "inner", "inner.sec"), fmtDirtySource)
	if err := runFmtCommand([]string{outer}); err == nil || !strings.Contains(err.Error(), "project boundary") {
		t.Fatalf("nested project: %v", err)
	}
	if readFile(t, filepath.Join(outer, "main.sec")) != fmtDirtySource {
		t.Fatal("a file was written despite the project boundary conflict")
	}

	if err := runFmtCommand([]string{t.TempDir()}); err != nil {
		t.Fatalf("empty directory: %v", err)
	}
	if err := runFmtCommand([]string{standalone}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, dirty); !strings.Contains(got, "\n    return 1\n") {
		t.Fatalf("standalone scan did not use the defaults:\n%s", got)
	}
}

// A malformed file in a bulk run is left byte-for-byte unchanged and
// reported, its valid siblings are still formatted, and the run fails.
//
// Rules:
//   - rules/tooling/formatter.md — §§ 25, 28(13)
func TestFmtDirectoryKeepsMalformedFiles(t *testing.T) {
	root := t.TempDir()
	broken := filepath.Join(root, "broken.sec")
	dirty := filepath.Join(root, "dirty.sec")
	writeFmtFile(t, broken, fmtMalformedSource)
	writeFmtFile(t, dirty, fmtDirtySource)

	err := runFmtCommand([]string{root})
	if err == nil || !strings.Contains(err.Error(), broken+": format.malformed-source:") {
		t.Fatalf("malformed bulk run: %v", err)
	}
	if readFile(t, broken) != fmtMalformedSource {
		t.Fatal("the malformed file was modified")
	}
	if got := readFile(t, dirty); !strings.Contains(got, "\n    return 1\n") {
		t.Fatalf("the valid sibling was not formatted:\n%s", got)
	}
}
