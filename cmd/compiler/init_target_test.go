package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/project"
	"sec/internal/sema"
)

// Rules: rules/projects/projects.md — §§17–19,39–45; initialization.md — §§20,25–27.
func TestInitTargetCLIAllKinds(t *testing.T) {
	root := t.TempDir()
	if output, err := runInitCLIInDirectory(t, root, "--minimal", "--name", "Project Name"); err != nil {
		t.Fatal(output, err)
	}
	manifestPath := filepath.Join(root, ".sec", "sec.toml")
	original, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	original = append(original, []byte("\n# user metadata must remain verbatim\n[parameters]\ntext = '''\n[target.fake]\n'''\n")...)
	if err := os.WriteFile(manifestPath, original, 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(manifestPath, 0640); err != nil {
		t.Fatal(err)
	}
	first, err := project.LoadManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ kind, name, source, file string }{
		{"command", "worker", "cmd/worker", "main.sec"},
		{"library", "codec", "codec", "codec.sec"},
		{"firmware", "board", "cmd/board", "main.sec"},
		{"test", "probe", "tests/probe", "probe_test.sec"},
	} {
		before, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		output, err := runInitCLIInDirectory(t, root, tc.kind, tc.name, "--dry-run")
		if err != nil || !strings.Contains(output, "edit manifest") || !strings.Contains(output, "create file") {
			t.Fatal(output, err)
		}
		if fileExists(filepath.Join(root, filepath.FromSlash(tc.source))) {
			t.Fatal("dry run created source")
		}
		got, _ := os.ReadFile(manifestPath)
		if !bytes.Equal(got, before) {
			t.Fatal("dry run changed manifest")
		}
		output, err = runInitCLIInDirectory(t, root, tc.kind, tc.name)
		if err != nil {
			t.Fatal(output, err)
		}
		resolved, err := project.LoadManifest(manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Facts().ProjectUUID != first.Facts().ProjectUUID || resolved.Facts().ProjectName != first.Facts().ProjectName || resolved.Facts().ProjectVersion != first.Facts().ProjectVersion {
			t.Fatal("Target changed Project identity/version")
		}
		version, found := resolved.TargetVersion(tc.name)
		if !found || version != first.Facts().ProjectVersion {
			t.Fatal("Target did not inherit version", version)
		}
		sourcePath := filepath.Join(root, filepath.FromSlash(tc.source), tc.file)
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), sourcePath))
		program := p.Parse().Program
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		analyzer := sema.NewAnalyzer()
		if tc.kind != "test" {
			if failures := analyzer.Analyze(program); len(failures) != 0 {
				t.Fatal(failures)
			}
			if failures := analyzer.ValidateProgramEntry(sema.ProgramTargetKind(tc.kind), programModulePath(program), lexer.Token{File: sourcePath, Line: 1, Column: 1}); len(failures) != 0 {
				t.Fatal(failures)
			}
		}
		current, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(current, original) {
			t.Fatal("unrelated manifest content changed")
		}
		info, err := os.Stat(manifestPath)
		if err != nil || info.Mode().Perm() != 0640 {
			t.Fatal("manifest permissions changed", info, err)
		}
		output, err = runInitCLIInDirectory(t, filepath.Join(root, filepath.FromSlash(tc.source)), tc.kind, tc.name)
		if err != nil {
			t.Fatal(output, err)
		}
		repeated, _ := os.ReadFile(manifestPath)
		if !bytes.Equal(current, repeated) {
			t.Fatal("repeated Target init changed manifest")
		}
	}
	stdout, stderr, code := runCLIForDiagnostics(t, "test", "--list", filepath.Join(root, "tests", "probe"))
	if code != 0 || !strings.Contains(stdout, "initial test") {
		t.Fatal(code, stdout, stderr)
	}
}

// Rules: rules/projects/projects.md — §§40–43.
func TestInitTargetConflictAndRollback(t *testing.T) {
	root := t.TempDir()
	if output, err := runInitCLIInDirectory(t, root, "--minimal", "--name", "Example"); err != nil {
		t.Fatal(output, err)
	}
	path := filepath.Join(root, ".sec", "sec.toml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	options := initCommandOptions{ProjectDir: root, TargetKind: "command", TargetName: "worker", Target: hostCompilerTarget()}
	plan, _, err := planInitTarget(options)
	if err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected manifest commit failure")
	err = plan.applyWithEdits(createInitFile, func(edit initManifestEdit) (func() error, error) { return nil, injected })
	if !errors.Is(err, injected) || fileExists(filepath.Join(root, "cmd")) {
		t.Fatal("manifest failure left source files", err)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, before) {
		t.Fatal("failed edit changed manifest")
	}
	plan, _, err = planInitTarget(options)
	if err != nil {
		t.Fatal(err)
	}
	err = plan.applyWithEdits(createInitFile, func(edit initManifestEdit) (func() error, error) {
		undo, err := replaceInitManifest(edit)
		if err != nil {
			return undo, err
		}
		return undo, injected
	})
	if !errors.Is(err, injected) || fileExists(filepath.Join(root, "cmd")) {
		t.Fatal("post-commit failure did not roll back", err)
	}
	got, _ = os.ReadFile(path)
	if !bytes.Equal(got, before) {
		t.Fatal("post-commit rollback did not restore manifest")
	}
	if output, err := runInitCLIInDirectory(t, root, "command", "worker"); err != nil {
		t.Fatal(output, err)
	}
	current, _ := os.ReadFile(path)
	if output, err := runInitCLIInDirectory(t, root, "library", "worker", "--dry-run"); err == nil || !strings.Contains(output, "conflict") {
		t.Fatal(output, err)
	}
	got, _ = os.ReadFile(path)
	if !bytes.Equal(got, current) || fileExists(filepath.Join(root, "worker")) {
		t.Fatal("conflict changed project")
	}
	entries, err := os.ReadDir(filepath.Join(root, ".sec"))
	if err != nil || len(entries) != 1 {
		t.Fatal("transaction leaked temp/lock files", entries, err)
	}
}

// Rules: rules/projects/projects.md — §§39–43; modules.md — §4.
func TestInitTargetPreservesExistingDeclarations(t *testing.T) {
	root := t.TempDir()
	if output, err := runInitCLIInDirectory(t, root, "--minimal", "--name", "Example"); err != nil {
		t.Fatal(output, err)
	}
	path := filepath.Join(root, ".sec", "sec.toml")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	existing := string(original) + "\n# custom artifact/entry spelling\n[target.owned]\nkind = 'command'\nsource = 'cmd/owned'\nartifact = 'custom-worker'\nvariants = ['" + hostCompilerTarget().String() + "']\n[target.owned.version]\nbuild = 91\n"
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../testdata/projects/init_target/owned.sec")
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "cmd", "owned", "main.sec")
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, source, 0644); err != nil {
		t.Fatal(err)
	}
	beforeInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if output, err := runInitCLIInDirectory(t, root, "command", "owned"); err != nil {
		t.Fatal(output, err)
	}
	got, _ := os.ReadFile(path)
	afterInfo, _ := os.Stat(path)
	if string(got) != existing || !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
		t.Fatal("rewrote compatible manifest")
	}
	got, _ = os.ReadFile(destination)
	if !bytes.Equal(got, source) {
		t.Fatal("replaced compatible user entry")
	}
	resolved, err := project.LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	version, found := resolved.TargetVersion("owned")
	if !found || version.Build != 91 {
		t.Fatal("lost version override", version)
	}
	// An existing compatible section can gain only its requested missing fields.
	partial := string(original) + "\n# partial declaration\n[target.extra]\nkind = 'library' # preserve comment\n[target.extra.parameters]\nthreshold = 7\n"
	if err := os.WriteFile(path, []byte(partial), 0644); err != nil {
		t.Fatal(err)
	}
	if output, err := runInitCLIInDirectory(t, root, "library", "extra"); err != nil {
		t.Fatal(output, err)
	}
	got, _ = os.ReadFile(path)
	if !bytes.Contains(got, []byte("kind = 'library' # preserve comment")) || !bytes.Contains(got, []byte("[target.extra.parameters]\nthreshold = 7")) || bytes.Count(got, []byte("[target.extra]")) != 1 {
		t.Fatal(string(got))
	}
}

// Rules: rules/projects/projects.md — §§6,17,39,42–44; initialization.md — §21.
func TestInitTargetRejectsInvalidRequests(t *testing.T) {
	root := t.TempDir()
	if output, err := runInitCLIInDirectory(t, root, "--minimal", "--name", "Example"); err != nil {
		t.Fatal(output, err)
	}
	for _, args := range [][]string{{"command", "../escape"}, {"library", "UpperCase"}, {"test", "bad/name"}, {"firmware"}, {"command", "valid", "--minimal"}} {
		if output, err := runInitCLIInDirectory(t, root, args...); err == nil {
			t.Fatal(args, output)
		}
	}
	if fileExists(filepath.Join(root, "cmd")) || fileExists(filepath.Join(root, "tests")) {
		t.Fatal("invalid arguments wrote sources")
	}
	source, err := os.ReadFile("../../testdata/projects/init_target/bad_entry.sec")
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "cmd", "worker", "main.sec")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, source, 0644); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, ".sec", "sec.toml")
	before, _ := os.ReadFile(manifest)
	if output, err := runInitCLIInDirectory(t, root, "command", "worker", "--dry-run"); err == nil || !strings.Contains(output, "entry conflict") {
		t.Fatal(output, err)
	}
	after, _ := os.ReadFile(manifest)
	if !bytes.Equal(before, after) {
		t.Fatal("entry conflict changed manifest")
	}
}

// Rules: rules/projects/projects.md — §§17,19,39–43.
func TestInitTargetAdditionalVariantAndChangedManifest(t *testing.T) {
	root := t.TempDir()
	if output, err := runInitCLIInDirectory(t, root, "--minimal", "--name", "Example"); err != nil {
		t.Fatal(output, err)
	}
	if output, err := runInitCLIInDirectory(t, root, "command", "armworker", "--target", "linux-arm64"); err != nil {
		t.Fatal(output, err)
	}
	path := filepath.Join(root, ".sec", "sec.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := project.ParseManifestDocument(string(data))
	if err != nil {
		t.Fatal(err)
	}
	variant, err := doc.Table("variant", "linux-arm64")
	if err != nil || variant["arch"] != "arm64" {
		t.Fatal(variant, err)
	}
	options := initCommandOptions{ProjectDir: root, TargetKind: "command", TargetName: "later", Target: hostCompilerTarget()}
	plan, _, err := planInitTarget(options)
	if err != nil {
		t.Fatal(err)
	}
	changed := append(append([]byte(nil), data...), []byte("\n# concurrent user edit\n")...)
	if err := os.WriteFile(path, changed, 0644); err != nil {
		t.Fatal(err)
	}
	if err := plan.apply(createInitFile); err == nil {
		t.Fatal("accepted changed manifest")
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, changed) || fileExists(filepath.Join(root, "cmd", "later")) {
		t.Fatal("changed manifest conflict lost user edits or left source")
	}
}
