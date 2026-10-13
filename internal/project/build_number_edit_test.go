package project

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Rules: rules/projects/projects.md — §§5(4–6),27(2–3),29(11),49(5).
func TestBuildNumberEditExactSyntax(t *testing.T) {
	cases := []struct{ name, target, text string }{
		{"project", "", "# keep\n[version] # keep header\n  build\t= +1_234  # keep comment\n"},
		{"target", "worker", "[version]\nbuild = 11\n[target.worker.version]\nbuild=+1_234 # selected\n[target.other.version]\nbuild=22\n"},
		{"quoted", "worker", "['target'.\"worker\".'version']\n\"build\" = +1_234"},
		{"escaped", "", "[\"ver\\u0073ion\"]\n\"bu\\u0069ld\" = +1_234\n"},
		{"dotted", "", "version . 'build' = +1_234\n"},
		{"nested dotted", "worker", "[target]\nworker.version.build = +1_234\n"},
		{"inline", "", "version = {major=1, 'build' = +1_234, format='[build]'} # keep\n"},
		{"nested inline", "worker", "target.worker = {kind='command', version={build=+1_234, minor=2}}\n"},
		{"inline dotted", "worker", "target = { worker.version.build = +1_234, other.version.build = 33 }\n"},
		{"distractors", "", "[version]\nbuild=+1_234\n[parameters]\ntext='''\n[version]\nbuild=+1_234\n'''\narray=[{build=+1_234}, '+1_234']\n[[example]]\nbuild=+1_234\n"},
	}
	for _, tc := range cases {
		for _, newline := range []string{"\n", "\r\n"} {
			t.Run(tc.name+strings.ReplaceAll(newline, "\n", "LF"), func(t *testing.T) {
				original := strings.ReplaceAll(tc.text, "\n", newline)
				document, err := ParseManifestDocument(original)
				if err != nil {
					t.Fatal(err)
				}
				after, err := document.ReplaceBuildNumber(tc.target, 98765)
				want := strings.Replace(original, "+1_234", "98765", 1)
				if err != nil || after != want {
					t.Fatalf("edit: %v\ngot: %q\nwant: %q", err, after, want)
				}
				// Compare semantic trees too: only the selected build may differ.
				updated, err := ParseManifestDocument(after)
				if err != nil {
					t.Fatal(err)
				}
				path := []string{"version"}
				if tc.target != "" {
					path = []string{"target", tc.target, "version"}
				}
				oldTable, _ := document.Table(path...)
				newTable, _ := updated.Table(path...)
				if newTable["build"] != int64(98765) {
					t.Fatal(newTable)
				}
				newTable["build"] = oldTable["build"]
				if !reflect.DeepEqual(document.Values, updated.Values) {
					t.Fatal("unrelated semantic value changed")
				}
				if document.Text != original {
					t.Fatal("original snapshot mutated")
				}
			})
		}
	}
}

// Rules: rules/projects/projects.md — §§8(3),28.1(3),49(5).
func TestBuildNumberEditRejectsInvalidStorage(t *testing.T) {
	cases := []struct {
		name, target, text string
		value              uint64
	}{
		{"missing project", "", "[parameters]\nbuild=12", 4},
		{"inherited target", "worker", "[version]\nbuild=12\n[target.worker]\nkind='command'", 4},
		{"string", "", "[version]\nbuild='12'", 4},
		{"float", "", "[version]\nbuild=12.0", 4},
		{"negative", "", "[version]\nbuild=-12", 4},
		{"hex capability", "", "[version]\nbuild=0xff", 4},
		{"array", "", "[[version]]\nbuild=12", 4},
		{"duplicate", "", "[version]\nbuild=12\nbuild=13", 4},
		{"malformed", "", "[version]\nbuild=", 4},
		{"overflow", "", "[version]\nbuild=12", math.MaxInt64 + 1},
		{"bad target key", "worker.version", "[version]\nbuild=12", 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			edit, err := PlanBuildNumberEdit("unused", tc.text, tc.target, tc.value)
			if err == nil || edit != nil {
				t.Fatal("accepted invalid edit", edit, err)
			}
		})
	}
	// Mutable decoded maps cannot redirect an edit away from its original text.
	document, _ := ParseManifestDocument("[version]\nbuild=+1_234")
	document.Values["version"] = "corrupted map"
	after, err := document.ReplaceBuildNumber("", 1234)
	if err != nil || after != document.Text {
		t.Fatal("equal value must retain original spelling", after, err)
	}
}

// Rules: rules/projects/projects.md — §§8(3),11,29(8–12),49(5).
func TestBuildNumberEditCommitAndConflict(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sec.toml")
	original := manifestFixture(t)
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(text), 0640); err != nil {
			t.Fatal(err)
		}
	}
	read := func() string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	write(original)
	edit, err := PlanBuildNumberEdit(path, original, "sec", math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}
	if read() != original {
		t.Fatal("planning wrote the manifest")
	}
	if err := edit.Commit(); err != nil {
		t.Fatal(err)
	}
	versions, failures := ParseManifestVersions(read())
	if len(failures) != 0 || versions.Project.Build != 123 || *versions.TargetOverrides["sec"].Build != math.MaxInt64 || *versions.TargetOverrides["lsp"].Build != 97 {
		t.Fatal(versions, failures)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0640 {
		t.Fatal("changed permissions", info.Mode())
	}
	if err := edit.Commit(); err == nil || read() != edit.Text() {
		t.Fatal("stale snapshot accepted", err)
	}
	write(original)
	// Unrelated external edits also survive a rejected commit.
	concurrent := original + "\n# another writer\n"
	write(concurrent)
	if err := edit.Commit(); err == nil || read() != concurrent {
		t.Fatal("lost external edit", err)
	}
	// A no-op checks the snapshot but leaves bytes, inode and mtime intact.
	write(original)
	stamp := time.Unix(1234567890, 0)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(path)
	noop, _ := PlanBuildNumberEdit(path, original, "", 123)
	if err := noop.Commit(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(path)
	if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) || read() != original {
		t.Fatal("no-op rewrote the manifest")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatal("left staged files", entries)
	}
}

// Rules: rules/projects/projects.md — §§29(9–12),43(4),49(5).
func TestManifestSnapshotUndoAndSymlink(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sec.toml")
	before, after := []byte("[version]\nbuild=1\n"), []byte("[version]\nbuild=2\n")
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	undo, err := ReplaceManifestSnapshot(path, before, after)
	if err != nil {
		t.Fatal(err)
	}
	if err := undo(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != string(before) {
		t.Fatal("undo did not restore bytes")
	}
	undo, err = ReplaceManifestSnapshot(path, before, after)
	if err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, "other.toml")
	if err := os.WriteFile(other, after, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(other, path); err != nil {
		t.Fatal(err)
	}
	if err := undo(); err == nil {
		t.Fatal("undo overwrote a replaced file")
	}
	link := filepath.Join(root, "link.toml")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceManifestSnapshot(link, after, before); err == nil {
		t.Fatal("followed manifest symlink")
	}
}
