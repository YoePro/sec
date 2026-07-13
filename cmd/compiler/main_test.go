package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

import "sec/internal/ast"

func TestStdlibModuleName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "fmt", want: "fmt"},
		{path: "std/fmt", want: "fmt"},
		{path: "io", want: "io"},
		{path: "std/io", want: "io"},
	}

	for _, tt := range tests {
		if got := stdlibModuleName(tt.path); got != tt.want {
			t.Fatalf("stdlibModuleName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestStdlibModulePath(t *testing.T) {
	got := stdlibModulePath("fmt", CompilerTarget{})
	want := "sec/stdlib/fmt/fmt.sec"
	if got != want {
		t.Fatalf("stdlibModulePath(%q) = %q, want %q", "fmt", got, want)
	}

	got = stdlibModulePath("io", CompilerTarget{OS: "linux", Arch: "amd64"})
	want = "sec/stdlib/io/write.linux.amd64.sec"
	if got != want {
		t.Fatalf("stdlibModulePath(%q) = %q, want %q", "io", got, want)
	}
}

func TestSourceIncludePath(t *testing.T) {
	got, ok := sourceIncludePath("platform/linux/amd64/raw_syscall", CompilerTarget{OS: "linux", Arch: "amd64"})
	if !ok {
		t.Fatal("sourceIncludePath did not accept platform import")
	}
	want := "sec/platform/linux/amd64/raw_syscall.sec"
	if got != want {
		t.Fatalf("sourceIncludePath platform = %q, want %q", got, want)
	}
}

func TestCollectSourceFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	files := []string{
		filepath.Join(dir, "a.sec"),
		filepath.Join(dir, "nested", "b.sec"),
		filepath.Join(dir, "ignore.txt"),
	}
	for _, file := range files {
		if err := os.WriteFile(file, []byte("module test\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := collectSourceFiles([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(dir, "a.sec"),
		filepath.Join(dir, "nested", "b.sec"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectSourceFiles(dir) = %#v, want %#v", got, want)
	}

	got, err = collectSourceFiles([]string{filepath.Join(dir, "*.sec")})
	if err != nil {
		t.Fatal(err)
	}
	want = []string{filepath.Join(dir, "a.sec")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectSourceFiles(glob) = %#v, want %#v", got, want)
	}
}

func TestSourcePathMatchesTarget(t *testing.T) {
	target := CompilerTarget{OS: "linux", Arch: "amd64"}
	tests := []struct {
		path string
		want bool
	}{
		{path: "sec/platform/linux/file.sec", want: true},
		{path: "sec/platform/linux/amd64/raw_syscall.sec", want: true},
		{path: "sec/platform/linux/arm64/raw_syscall.sec", want: false},
		{path: "sec/stdlib/fmt/fmt.sec", want: true},
	}

	for _, tt := range tests {
		if got := sourcePathMatchesTarget(tt.path, target); got != tt.want {
			t.Fatalf("sourcePathMatchesTarget(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestParseBuildCommandArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantInput string
		wantOut   string
		wantOK    bool
	}{
		{
			name:      "default output",
			args:      []string{"main.sec"},
			wantInput: "main.sec",
			wantOut:   "main",
			wantOK:    true,
		},
		{
			name:      "explicit output",
			args:      []string{"main.sec", "-o", "program"},
			wantInput: "main.sec",
			wantOut:   "program",
			wantOK:    true,
		},
		{
			name:   "missing input",
			args:   []string{"-o", "program"},
			wantOK: false,
		},
		{
			name:   "unknown flag",
			args:   []string{"main.sec", "--unknown"},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotInput, gotOut, gotOK := parseBuildCommandArgs(tt.args)
			if gotOK != tt.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotInput != tt.wantInput || gotOut != tt.wantOut {
				t.Fatalf("got (%q, %q), want (%q, %q)", gotInput, gotOut, tt.wantInput, tt.wantOut)
			}
		})
	}
}

func TestParseBuildCommandOptions(t *testing.T) {
	options, ok := parseBuildCommandOptions(
		[]string{"main.sec", "--target", "linux-amd64", "--keep-llvm", "--clang", "custom-clang", "-o", "program"},
		CompilerTarget{OS: "macos", Arch: "arm64"},
	)
	if !ok {
		t.Fatal("parseBuildCommandOptions returned ok=false")
	}
	if options.InputFile != "main.sec" {
		t.Fatalf("InputFile = %q, want main.sec", options.InputFile)
	}
	if options.OutputFile != "program" {
		t.Fatalf("OutputFile = %q, want program", options.OutputFile)
	}
	if options.Target != (CompilerTarget{OS: "linux", Arch: "amd64"}) {
		t.Fatalf("Target = %#v, want linux-amd64", options.Target)
	}
	if options.Clang != "custom-clang" {
		t.Fatalf("Clang = %q, want custom-clang", options.Clang)
	}
	if options.LLVMOutputFile != "program.ll" {
		t.Fatalf("LLVMOutputFile = %q, want program.ll", options.LLVMOutputFile)
	}
}

func TestParseCompilerTarget(t *testing.T) {
	target, ok := parseCompilerTarget("darwin-arm64")
	if !ok {
		t.Fatal("parseCompilerTarget returned ok=false")
	}
	if target != (CompilerTarget{OS: "macos", Arch: "arm64"}) {
		t.Fatalf("target = %#v, want macos-arm64", target)
	}

	if _, ok := parseCompilerTarget("linux"); ok {
		t.Fatal("parseCompilerTarget accepted target without arch")
	}
}

func TestFindTargetDefinition(t *testing.T) {
	target, ok := findTargetDefinition(CompilerTarget{OS: "linux", Arch: "amd64"})
	if !ok {
		t.Fatal("linux-amd64 target definition not found")
	}
	if target.LLVMTriple != "x86_64-pc-linux-gnu" {
		t.Fatalf("LLVMTriple = %q, want x86_64-pc-linux-gnu", target.LLVMTriple)
	}
	if target.Status != TargetImplemented {
		t.Fatalf("Status = %q, want %q", target.Status, TargetImplemented)
	}
	if !target.CanEmitLLVM || !target.CanLink || !target.CanRun {
		t.Fatalf("linux-amd64 capabilities are too low: %#v", target)
	}
}

func TestTargetCapabilities(t *testing.T) {
	if _, err := requireTargetCanEmitLLVM(CompilerTarget{OS: "linux", Arch: "arm64"}); err != nil {
		t.Fatalf("linux-arm64 should be able to emit LLVM: %v", err)
	}
	if _, err := requireTargetCanLink(CompilerTarget{OS: "linux", Arch: "arm64"}); err == nil {
		t.Fatal("linux-arm64 should not link yet")
	}
	if _, err := requireTargetCanEmitLLVM(CompilerTarget{OS: "baremetal", Arch: "cortex-m4"}); err == nil {
		t.Fatal("baremetal-cortex-m4 should not emit LLVM yet")
	}
}

func TestDefaultBuildOutputPath(t *testing.T) {
	tests := map[string]string{
		"main.sec":                "main",
		"testdata/build_test.sec": "testdata/build_test",
		"program":                 "program",
		"archive.v1/program.sec":  "archive.v1/program",
	}

	for input, want := range tests {
		if got := defaultBuildOutputPath(input); got != want {
			t.Fatalf("defaultBuildOutputPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeCompilerTarget(t *testing.T) {
	if got := normalizeTargetOS("darwin"); got != "macos" {
		t.Fatalf("normalizeTargetOS(%q) = %q, want macos", "darwin", got)
	}
	if got := normalizeTargetArch("arm"); got != "arm32" {
		t.Fatalf("normalizeTargetArch(%q) = %q, want arm32", "arm", got)
	}
}

func TestValidateProgramTarget(t *testing.T) {
	program := &ast.Program{
		Statements: []ast.Statement{
			&ast.TargetDirective{OS: "linux", Arch: "amd64"},
		},
	}

	if err := validateProgramTarget(program, CompilerTarget{OS: "linux", Arch: "amd64"}); err != nil {
		t.Fatalf("validateProgramTarget matching target returned error: %v", err)
	}

	err := validateProgramTarget(program, CompilerTarget{OS: "linux", Arch: "arm64"})
	if err == nil {
		t.Fatal("validateProgramTarget mismatch returned nil error")
	}
	want := "file target linux-amd64 does not match current target linux-arm64"
	if err.Error() != want {
		t.Fatalf("wrong error. got=%q want=%q", err.Error(), want)
	}
}

func TestValidateProgramTargetAnyArch(t *testing.T) {
	program := &ast.Program{
		Statements: []ast.Statement{
			&ast.TargetDirective{OS: "linux", Arch: "any"},
		},
	}

	if err := validateProgramTarget(program, CompilerTarget{OS: "linux", Arch: "amd64"}); err != nil {
		t.Fatalf("validateProgramTarget any arch returned error: %v", err)
	}
}
