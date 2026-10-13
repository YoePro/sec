package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sec/internal/project"
)

// TestInitMinimalCLI creates the normative two-entry scaffold through the actual
// command, confirms root Target discovery, and type-checks its entry module.
// Rules: rules/projects/projects.md — §14(8),§37(1–8),§40.
func TestInitMinimalCLI(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "minimal")
	stdout, stderr, code := runCLIForDiagnostics(t, "init", dir, "--minimal", "--name", "Small App", "--target", "linux-amd64", "--profile", "debug")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "initialized Sec project") {
		t.Fatal(code, stdout, stderr)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !reflect.DeepEqual(names, []string{".sec", "main.sec"}) {
		t.Fatal(names)
	}
	mainPath := filepath.Join(dir, "main.sec")
	source, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(source) != defaultMainSource() {
		t.Fatalf("unexpected entry: %s", source)
	}
	manifest := filepath.Join(dir, ".sec", "sec.toml")
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	targets := project.ParseManifestTargets(string(data))
	target, ok := project.TargetForSource(targets, dir, mainPath)
	if !ok || target.Name != "small-app" || target.Kind != "command" || target.Source != "." {
		t.Fatal(target, targets)
	}
	if !strings.Contains(string(data), `profile = "debug"`) || !strings.Contains(string(data), "[profile.debug]") {
		t.Fatal(string(data))
	}
	versions, errors := project.ParseManifestVersions(string(data))
	if len(errors) != 0 || !versions.HasProject {
		t.Fatal(versions, errors)
	}
	version, err := versions.Project.Render()
	if err != nil || version != "0.1.0.0" {
		t.Fatal(version, err)
	}
	_, stderr, code = runCLIForDiagnostics(t, "sema", mainPath)
	if code != 0 {
		t.Fatal(code, stderr)
	}
	// Repeated init with either layout leaves an existing minimal Project alone.
	for _, args := range [][]string{{"init", dir, "--minimal", "--name", "Small App"}, {"init", dir, "--name", "Small App"}} {
		_, stderr, code = runCLIForDiagnostics(t, args...)
		if code != 0 || stderr != "" {
			t.Fatal(code, stderr)
		}
		got, err := os.ReadFile(manifest)
		if err != nil || string(got) != string(data) {
			t.Fatal("manifest or UUID changed", err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 2 {
			t.Fatal("layout changed", entries, err)
		}
	}
}

// TestInitMinimalPreservesSourceAndBoundary verifies additive source safety and
// the single-manifest boundary without creating a nested minimal scaffold.
// Rules: rules/projects/projects.md — §§4,37(8),40,42,53(3).
func TestInitMinimalPreservesSourceAndBoundary(t *testing.T) {
	dir := t.TempDir()
	existing, err := os.ReadFile("../../testdata/projects/init/main.sec")
	if err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.sec")
	if err := os.WriteFile(mainPath, existing, 0644); err != nil {
		t.Fatal(err)
	}
	options := initCommandOptions{ProjectDir: dir, ProjectName: "Example", Target: CompilerTarget{OS: "linux", Arch: "amd64"}, Profile: "server", Minimal: true}
	if err := initProject(options); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(mainPath)
	if err != nil || string(got) != string(existing) {
		t.Fatal("existing root source changed", err)
	}
	options.ProjectDir = filepath.Join(dir, "nested")
	if err := initProject(options); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(options.ProjectDir); !os.IsNotExist(err) {
		t.Fatal("nested scaffold was created", err)
	}
}

// TestInitMinimalDoesNotExtendApplication checks that layout selection cannot
// silently add a second root module to an existing application manifest.
// Rules: rules/projects/projects.md — §14(9),§40.
func TestInitMinimalDoesNotExtendApplication(t *testing.T) {
	dir := t.TempDir()
	options := initCommandOptions{ProjectDir: dir, ProjectName: "Example", Target: CompilerTarget{OS: "linux", Arch: "amd64"}, Profile: "server"}
	if err := initProject(options); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(dir, ".sec", "sec.toml")
	before, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	options.Minimal = true
	if err := initProject(options); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(manifest)
	if err != nil || string(after) != string(before) {
		t.Fatal("existing manifest changed", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "main.sec")); !os.IsNotExist(err) {
		t.Fatal("extra root module created", err)
	}
}

// TestParseInitMinimalOptions keeps build-profile selection independent of
// scaffold layout and rejects repeated or malformed minimal options.
// Rules: rules/projects/projects.md — §22,§37(6–8).
func TestParseInitMinimalOptions(t *testing.T) {
	host := CompilerTarget{OS: "linux", Arch: "amd64"}
	for _, args := range [][]string{{"--minimal"}, {"example", "--profile", "server", "--minimal"}, {"--minimal", "example", "--profile", "minimal"}} {
		options, ok := parseInitCommandOptions(args, host)
		if !ok || !options.Minimal {
			t.Fatal(args, options, ok)
		}
	}
	for _, args := range [][]string{{"--minimal", "--minimal"}, {"--minimal=true"}} {
		if _, ok := parseInitCommandOptions(args, host); ok {
			t.Fatal("accepted invalid options", args)
		}
	}
	options, ok := parseInitCommandOptions([]string{"--profile", "minimal"}, host)
	if !ok || options.Minimal || options.Profile != "minimal" {
		t.Fatal("build profile altered scaffold selection", options, ok)
	}
}
