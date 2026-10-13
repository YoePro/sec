package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Rules: rules/projects/projects.md — §§43,44,45,50(1).
func TestInitDryRunCLI(t *testing.T) {
	root := t.TempDir()
	for _, minimal := range []bool{false, true} {
		name := "application"
		if minimal {
			name = "minimal"
		}
		args := []string{name, "--dry-run", "--name", "Example"}
		if minimal {
			args = append(args, "--minimal")
		}
		output, err := runInitCLIInDirectory(t, root, args...)
		if err != nil || !strings.Contains(output, "create file "+filepath.Join(root, name, ".sec", "sec.toml")) {
			t.Fatal(output, err)
		}
		if fileExists(filepath.Join(root, name)) {
			t.Fatal("dry run created a project directory")
		}
	}
	output, err := runInitCLIInDirectory(t, root, "--dry-run")
	if err != nil || !strings.Contains(output, "create file") {
		t.Fatal("dry run should not prompt", output, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("dry run changed existing empty root", entries, err)
	}
	// Actual creation must agree with the plan and remain stable on repeat init.
	output, err = runInitCLIInDirectory(t, root, "application", "--name", "Example")
	if err != nil {
		t.Fatal(output, err)
	}
	projectRoot := filepath.Join(root, "application")
	manifest := filepath.Join(projectRoot, ".sec", "sec.toml")
	before, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	beforeInfo, err := os.Stat(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--dry-run", "module", "domain/internal/storage"}, {"module", "domain/internal/storage", "--dry-run"}} {
		output, err = runInitCLIInDirectory(t, filepath.Join(projectRoot, "internal"), args...)
		if err != nil || !strings.Contains(output, filepath.Join(projectRoot, "domain", "internal", "storage", "storage.sec")) {
			t.Fatal(output, err)
		}
		if fileExists(filepath.Join(projectRoot, "domain")) {
			t.Fatal("module dry run created directories")
		}
	}
	for _, args := range [][]string{{"--dry-run"}, {"--name", "Different Name"}, {"--minimal", "--dry-run"}} {
		output, err = runInitCLIInDirectory(t, filepath.Join(projectRoot, "internal"), args...)
		if err != nil {
			t.Fatal(output, err)
		}
		if fileExists(filepath.Join(projectRoot, "internal", ".sec")) || fileExists(filepath.Join(projectRoot, "main.sec")) {
			t.Fatal("subdirectory invocation created another scaffold")
		}
	}
	after, err := os.ReadFile(manifest)
	if err != nil || string(after) != string(before) {
		t.Fatal("init changed existing manifest", err)
	}
	afterInfo, err := os.Stat(manifest)
	if err != nil || !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
		t.Fatal("no-op rewrote manifest", err)
	}
}

// Rules: rules/projects/projects.md — §§42,43,44(2–3),50(1).
func TestInitPreflightConflictCLI(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		t.Run(map[bool]string{false: "apply", true: "dry_run"}[dryRun], func(t *testing.T) {
			root := t.TempDir()
			// A late metadata-directory conflict must not leave earlier cmd/bin creates.
			user := filepath.Join(root, ".sec")
			if err := os.WriteFile(user, []byte("user data"), 0644); err != nil {
				t.Fatal(err)
			}
			args := []string{"--name", "Example"}
			if dryRun {
				args = append(args, "--dry-run")
			}
			output, err := runInitCLIInDirectory(t, root, args...)
			if err == nil || !strings.Contains(output, "conflict") || !strings.Contains(output, ".sec") {
				t.Fatal(output, err)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 1 {
				t.Fatal("conflict left early creates", entries, err)
			}
			got, err := os.ReadFile(user)
			if err != nil || string(got) != "user data" {
				t.Fatal("changed user conflict file", err)
			}
		})
	}
}

// Rules: rules/projects/projects.md — §§4,42–45; rules/projects/modules.md — §4.
func TestInitSourceAndManifestConflicts(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "cmd", "example", "main.sec")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	conflict, err := os.ReadFile("../../testdata/projects/init_module/conflict/conflict.sec")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, conflict, 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--name", "Example"}, {"--name", "Example", "--dry-run"}} {
		output, err := runInitCLIInDirectory(t, root, args...)
		if err == nil || !strings.Contains(output, "source conflict") {
			t.Fatal(output, err)
		}
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 1 || entries[0].Name() != "cmd" {
			t.Fatal("source conflict left scaffold", entries, err)
		}
	}
	// A pre-existing child project cannot be enclosed in a newly initialized one.
	child := filepath.Join(t.TempDir(), "child")
	if output, err := runInitCLIInDirectory(t, filepath.Dir(child), "child", "--minimal", "--name", "Child"); err != nil {
		t.Fatal(output, err)
	}
	output, err := runInitCLIInDirectory(t, filepath.Dir(child), "--dry-run", "--name", "Parent")
	if err == nil || !strings.Contains(output, "existing child manifest") {
		t.Fatal(output, err)
	}
	if fileExists(filepath.Join(filepath.Dir(child), ".sec")) {
		t.Fatal("nested conflict created parent manifest")
	}
}

// Rules: rules/projects/projects.md — §44(1).
func TestParseInitDryRunOptions(t *testing.T) {
	target := CompilerTarget{OS: "linux", Arch: "amd64"}
	for _, args := range [][]string{{"--dry-run"}, {"--dry-run", "module", "inventory/orders"}, {"module", "inventory/orders", "--dry-run"}} {
		options, ok := parseInitCommandOptions(args, target)
		if !ok || !options.DryRun {
			t.Fatal(args, options, ok)
		}
	}
	for _, args := range [][]string{{"--dry-run", "--dry-run"}, {"module", "--dry-run"}, {"--dry-run=true"}, {"module", "orders", "--minimal"}} {
		if _, ok := parseInitCommandOptions(args, target); ok {
			t.Fatal("accepted malformed arguments", args)
		}
	}
	options, ok := parseInitCommandOptions([]string{"--name", "--dry-run"}, target)
	if !ok || options.DryRun || options.ProjectName != "--dry-run" {
		t.Fatal("treated name value as flag", options, ok)
	}
	options, ok = parseInitCommandOptions([]string{"module", "inventory/orders", "--dry-run"}, target)
	if !ok || !reflect.DeepEqual([]string{options.ModulePath, options.ProjectDir}, []string{"inventory/orders", "."}) {
		t.Fatal(options, ok)
	}
}

// Rules: rules/projects/projects.md — §§37,42–44; §39(3) command entry.
func TestInitPreflightDuplicateEntry(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "cmd", "example")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../testdata/projects/init/main.sec")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "existing.sec"), source, 0644); err != nil {
		t.Fatal(err)
	}
	output, err := runInitCLIInDirectory(t, root, "--dry-run", "--name", "Example")
	if err == nil || !strings.Contains(output, "exactly one main function") {
		t.Fatal(output, err)
	}
	if fileExists(filepath.Join(directory, "main.sec")) || fileExists(filepath.Join(root, ".sec")) {
		t.Fatal("duplicate entry conflict changed the project")
	}
}

// Rules: rules/projects/projects.md — §§8,43(1–2),44(2).
func TestInitDryRunValidatesExistingVersion(t *testing.T) {
	root := t.TempDir()
	if output, err := runInitCLIInDirectory(t, root, "--minimal", "--name", "Example"); err != nil {
		t.Fatal(output, err)
	}
	manifest := filepath.Join(root, ".sec", "sec.toml")
	original, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	invalid := strings.Replace(string(original), "major = 0", "major = -1", 1)
	if err := os.WriteFile(manifest, []byte(invalid), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--dry-run"}, {"module", "orders", "--dry-run"}, {"--name", "Example"}} {
		output, err := runInitCLIInDirectory(t, root, args...)
		if err == nil || !strings.Contains(output, "version.major") {
			t.Fatal(output, err)
		}
	}
	got, err := os.ReadFile(manifest)
	if err != nil || string(got) != invalid || fileExists(filepath.Join(root, "orders")) {
		t.Fatal("invalid version init changed project", err)
	}
}
