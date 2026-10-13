package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"sec/internal/project"
)

func buildCLIProject(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "cmd", "worker")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../testdata/projects/init/main.sec")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "main.sec"), source, 0644); err != nil {
		t.Fatal(err)
	}
	text := `[project]
name='Worker Project'
uuid='550e8400-e29b-41d4-a716-446655440000'
[version]
major=1
minor=2
revision=3
build=40
format='[major].[minor].[revision].[build]'
[build]
profile='debug'
[target.worker]
kind='command'
source='cmd/worker'
variants=['first', 'second']
[target.worker.version]
build=10 # visible local baseline
[target.worker.build_counter]
enabled=true
[variant.first]
os='linux'
arch='amd64'
[variant.second]
os='linux'
arch='amd64'
`
	path := filepath.Join(root, ".sec", "sec.toml")
	if err := os.WriteFile(path, []byte(text), 0640); err != nil {
		t.Fatal(err)
	}
	stub, err := os.ReadFile("../../testdata/projects/build_tools/clang_stub.sh")
	if err != nil {
		t.Fatal(err)
	}
	tool := filepath.Join(root, "clang-stub")
	if err := os.WriteFile(tool, stub, 0755); err != nil {
		t.Fatal(err)
	}
	return root, path, tool
}

func runBuildCLIInDirectory(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	command := exec.Command(os.Args[0], append([]string{"-test.run=^TestLexerCLIProcess$", "--", "build"}, args...)...)
	command.Dir = root
	command.Env = append(os.Environ(), "SEC_LEXER_CLI_TEST_PROCESS=1")
	data, err := command.CombinedOutput()
	return string(data), err
}

// Rules: rules/projects/projects.md — §§29(3–12),30(2–5),31,34(2).
func TestBuildCLIMultipleVariantsAndExplicitNumber(t *testing.T) {
	root, path, tool := buildCLIProject(t)
	output, err := runBuildCLIInDirectory(t, root, "worker", "--clang", tool, "--keep-llvm")
	if err != nil || !strings.Contains(output, "built worker version 1.2.3.11") {
		t.Fatal(output, err)
	}
	manifest, err := project.LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	version, _ := manifest.TargetVersion("worker")
	if version.Build != 11 || manifest.Facts().ProjectVersion.Build != 40 {
		t.Fatal("committed per Variant or wrong scope", version)
	}
	for _, name := range []string{"first", "second"} {
		file := filepath.Join(root, "bin", "worker", name, "debug", "worker.ll")
		data, err := os.ReadFile(file)
		if err != nil || !strings.Contains(string(data), `Sec build version "1.2.3.11" artifact `) {
			t.Fatal(file, string(data), err)
		}
	}
	before, _ := os.ReadFile(path)
	for _, number := range []string{"0", "77"} {
		output, err = runBuildCLIInDirectory(t, root, "worker", "--clang", tool, "--keep-llvm", "--build-number", number)
		if err != nil || !strings.Contains(output, "built worker version 1.2.3."+number) {
			t.Fatal(output, err)
		}
		after, _ := os.ReadFile(path)
		if string(after) != string(before) {
			t.Fatal("explicit build rewrote manifest")
		}
		for _, name := range []string{"first", "second"} {
			data, err := os.ReadFile(filepath.Join(root, "bin", "worker", name, "debug", "worker.ll"))
			if err != nil || !strings.Contains(string(data), fmt.Sprintf("version %q", "1.2.3."+number)) {
				t.Fatal(string(data), err)
			}
		}
	}
}

// Rules: rules/projects/projects.md — §29(6–12).
func TestBuildCLIFailurePreservesCounterAndReleasesLock(t *testing.T) {
	root, path, tool := buildCLIProject(t)
	before, _ := os.ReadFile(path)
	text := strings.ReplaceAll(string(before), "second", "reject")
	if err := os.WriteFile(path, []byte(text), 0640); err != nil {
		t.Fatal(err)
	}
	output, err := runBuildCLIInDirectory(t, root, "worker", "--clang", tool)
	if err == nil || strings.Contains(output, "built worker version") {
		t.Fatal(output, err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != text {
		t.Fatal("failed second Variant committed candidate")
	}
	// Changing back after failure reuses candidate 11, proving both reservation
	// cancellation and OS lock release on the compiler's immediate exit path.
	if err := os.WriteFile(path, before, 0640); err != nil {
		t.Fatal(err)
	}
	output, err = runBuildCLIInDirectory(t, root, "worker", "--clang", tool)
	if err != nil || !strings.Contains(output, "built worker version 1.2.3.11") {
		t.Fatal(output, err)
	}
}

// Rules: rules/projects/projects.md — §§8(3),30(1–4).
func TestParseBuildNumberOptions(t *testing.T) {
	for _, number := range []string{"0", "10", "9223372036854775807"} {
		options, ok := parseBuildCommandOptions([]string{"worker", "--build-number", number}, hostCompilerTarget())
		if !ok || options.BuildNumber == nil {
			t.Fatal(number, options, ok)
		}
	}
	for _, args := range [][]string{
		{"worker", "--build-number"}, {"worker", "--build-number", "-1"},
		{"worker", "--build-number", "9223372036854775808"}, {"worker", "--build-number", "1.5"},
		{"worker", "--build-number", "1", "--build-number", "2"},
	} {
		if _, ok := parseBuildCommandOptions(args, hostCompilerTarget()); ok {
			t.Fatal(args)
		}
	}
}

// Rules: rules/projects/projects.md — §§29(5–7),30; rules/compiler/linking.md — native artifacts.
func TestBuildCLINativeFileAndProjectScope(t *testing.T) {
	if hostCompilerTarget().String() != "linux-amd64" {
		t.Skip("native LLVM linker currently supports linux-amd64")
	}
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang unavailable")
	}
	root, path, _ := buildCLIProject(t)
	before, _ := os.ReadFile(path)
	// Project-scoped storage affects no local field and a filtered file build
	// still commits exactly one logical build candidate.
	text := strings.Replace(string(before), "[target.worker.version]\nbuild=10 # visible local baseline\n", "", 1)
	text = strings.Replace(text, "enabled=true", "enabled=true\nscope='project'", 1)
	if err := os.WriteFile(path, []byte(text), 0640); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(root, "cmd", "worker", "main.sec")
	binary := filepath.Join(root, "native")
	output, err := runBuildCLIInDirectory(t, root, input, "-o", binary, "--clang", clang, "--keep-llvm")
	if err != nil || !strings.Contains(output, "built worker version 1.2.3.41") {
		t.Fatal(output, err)
	}
	manifest, err := project.LoadManifest(path)
	if err != nil || manifest.Facts().ProjectVersion.Build != 41 {
		t.Fatal(manifest, err)
	}
	if err := exec.Command(binary).Run(); err == nil {
		t.Fatal("expected fixture exit code 7")
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 7 {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	output, err = runBuildCLIInDirectory(t, root, input, "-o", binary, "--clang", clang, "--build-number", "8")
	if err != nil || !strings.Contains(output, "built worker version 1.2.3.8") {
		t.Fatal(output, err)
	}
	final, _ := os.ReadFile(path)
	if string(final) != string(after) {
		t.Fatal("explicit native build mutated project storage")
	}
}

// Rules: rules/projects/projects.md — §29(5–12).
func TestBuildCLIFilterAndConcurrentEdit(t *testing.T) {
	root, path, tool := buildCLIProject(t)
	before, _ := os.ReadFile(path)
	text := strings.Replace(string(before), "variants=['first', 'second']", "variants=['first', 'second', 'windows']", 1)
	text += "[variant.windows]\nos='windows'\narch='amd64'\n"
	if err := os.WriteFile(path, []byte(text), 0640); err != nil {
		t.Fatal(err)
	}
	// A filtered build compiles its selected Variants only and consumes one
	// candidate, even though another configured platform cannot link yet.
	output, err := runBuildCLIInDirectory(t, root, "worker", "--target", "linux-amd64", "--clang", tool)
	if err != nil || !strings.Contains(output, "built worker version 1.2.3.11") {
		t.Fatal(output, err)
	}
	current, _ := os.ReadFile(path)
	t.Setenv("SEC_TEST_EDIT_MANIFEST", path)
	output, err = runBuildCLIInDirectory(t, root, filepath.Join(root, "cmd", "worker", "main.sec"), "-o", filepath.Join(root, "binary"), "--clang", tool)
	if err == nil || !strings.Contains(output, "build commit error") || strings.Contains(output, "built worker version") {
		t.Fatal(output, err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(current)+"\n# external editor\n" {
		t.Fatal("commit overwrote concurrent manifest edit")
	}
}

// Rules: rules/projects/projects.md — §29(8–10).
func TestBuildCLIParallelProcesses(t *testing.T) {
	root, path, tool := buildCLIProject(t)
	var group sync.WaitGroup
	results := make(chan string, 2)
	for i := 0; i < 2; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			output, err := runBuildCLIInDirectory(t, root, "worker", "--clang", tool)
			if err != nil {
				t.Error(output, err)
			}
			results <- output
		}()
	}
	group.Wait()
	close(results)
	seen := map[string]bool{}
	for output := range results {
		for _, version := range []string{"1.2.3.11", "1.2.3.12"} {
			if strings.Contains(output, "built worker version "+version) {
				seen[version] = true
			}
		}
	}
	manifest, err := project.LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	version, _ := manifest.TargetVersion("worker")
	if len(seen) != 2 || version.Build != 12 {
		t.Fatal("parallel processes did not serialize", seen, version)
	}
}
