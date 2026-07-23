package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
		[]string{"main.sec", "--target", "linux-amd64", "--pipeline", "mlir", "--keep-mlir", "--keep-llvm", "--mlir-bin", "/opt/mlir/bin", "--clang", "custom-clang", "-o", "program"},
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
	if options.MLIROutputFile != "program.mlir" {
		t.Fatalf("MLIROutputFile = %q, want program.mlir", options.MLIROutputFile)
	}
	if options.MLIRBin != "/opt/mlir/bin" {
		t.Fatalf("MLIRBin = %q, want /opt/mlir/bin", options.MLIRBin)
	}
	if options.Pipeline != "mlir" {
		t.Fatalf("Pipeline = %q, want mlir", options.Pipeline)
	}
}

func TestParseEmitMLIRCommandArgs(t *testing.T) {
	options, ok := parseEmitMLIRCommandArgs(
		[]string{"main.sec", "-o", "main.mlir", "--target", "linux-amd64", "--mlir-bin", "/opt/mlir/bin", "--verify"},
		CompilerTarget{OS: "macos", Arch: "arm64"},
	)
	if !ok {
		t.Fatal("parseEmitMLIRCommandArgs returned ok=false")
	}
	if options.InputFile != "main.sec" {
		t.Fatalf("InputFile = %q, want main.sec", options.InputFile)
	}
	if options.OutputFile != "main.mlir" {
		t.Fatalf("OutputFile = %q, want main.mlir", options.OutputFile)
	}
	if options.Target != (CompilerTarget{OS: "linux", Arch: "amd64"}) {
		t.Fatalf("Target = %#v, want linux-amd64", options.Target)
	}
	if options.MLIRBin != "/opt/mlir/bin" {
		t.Fatalf("MLIRBin = %q, want /opt/mlir/bin", options.MLIRBin)
	}
	if !options.Verify {
		t.Fatal("Verify = false, want true")
	}
}

func TestEmitOutputDashMeansStdout(t *testing.T) {
	_, outputFile, _, ok := parseEmitLLVMCommandArgs(
		[]string{"main.sec", "-o", "-"},
		CompilerTarget{OS: "linux", Arch: "amd64"},
	)
	if !ok {
		t.Fatal("parseEmitLLVMCommandArgs returned ok=false")
	}
	if outputFile != "-" {
		t.Fatalf("outputFile = %q, want -", outputFile)
	}

	options, ok := parseEmitMLIRCommandArgs(
		[]string{"main.sec", "-o", "-"},
		CompilerTarget{OS: "linux", Arch: "amd64"},
	)
	if !ok {
		t.Fatal("parseEmitMLIRCommandArgs returned ok=false")
	}
	if options.OutputFile != "-" {
		t.Fatalf("OutputFile = %q, want -", options.OutputFile)
	}
}

func TestRejectDashPrefixedOutputFiles(t *testing.T) {
	if _, _, _, ok := parseEmitLLVMCommandArgs([]string{"main.sec", "-o", "-bad.ll"}, CompilerTarget{}); ok {
		t.Fatal("parseEmitLLVMCommandArgs accepted dash-prefixed output file")
	}
	if _, ok := parseEmitMLIRCommandArgs([]string{"main.sec", "-o", "-bad.mlir"}, CompilerTarget{}); ok {
		t.Fatal("parseEmitMLIRCommandArgs accepted dash-prefixed output file")
	}
	if _, ok := parseBuildCommandOptions([]string{"main.sec", "-o", "-"}, CompilerTarget{}); ok {
		t.Fatal("parseBuildCommandOptions accepted stdout output")
	}
	if _, ok := parseBuildCommandOptions([]string{"main.sec", "-o", "-bad"}, CompilerTarget{}); ok {
		t.Fatal("parseBuildCommandOptions accepted dash-prefixed output file")
	}
}

func TestParseInitCommandOptions(t *testing.T) {
	options, ok := parseInitCommandOptions(
		[]string{"demo", "--name", "My Project", "--target", "linux-amd64", "--profile", "cli"},
		CompilerTarget{OS: "macos", Arch: "arm64"},
	)
	if !ok {
		t.Fatal("parseInitCommandOptions returned ok=false")
	}
	if options.ProjectDir != "demo" {
		t.Fatalf("ProjectDir = %q, want demo", options.ProjectDir)
	}
	if options.ProjectName != "My Project" {
		t.Fatalf("ProjectName = %q, want My Project", options.ProjectName)
	}
	if options.Target != (CompilerTarget{OS: "linux", Arch: "amd64"}) {
		t.Fatalf("Target = %#v, want linux-amd64", options.Target)
	}
	if options.Profile != "cli" {
		t.Fatalf("Profile = %q, want cli", options.Profile)
	}
}

func TestInitProjectCreatesScaffold(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "myproj")
	options := initCommandOptions{
		ProjectDir:  dir,
		ProjectName: "My Project",
		Target:      CompilerTarget{OS: "linux", Arch: "amd64"},
		Profile:     "server",
	}

	if err := initProject(options); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		filepath.Join(dir, "cmd", "main"),
		filepath.Join(dir, "bin"),
		filepath.Join(dir, ".sec"),
		filepath.Join(dir, "internal"),
	} {
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			t.Fatalf("%s was not created as directory", path)
		}
	}

	mainSource, err := os.ReadFile(filepath.Join(dir, "cmd", "main", "main.sec"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mainSource), "fn main() int") {
		t.Fatalf("main.sec missing main function:\n%s", mainSource)
	}

	config, err := os.ReadFile(filepath.Join(dir, ".sec", "sec.toml"))
	if err != nil {
		t.Fatal(err)
	}
	configText := string(config)
	for _, want := range []string{
		`[project]`,
		`name = "My Project"`,
		`uuid = "`,
		`imports = []`,
		`[platform]`,
		`os = "linux"`,
		`arch = "amd64"`,
		`[build]`,
		`target = "x86_64-pc-linux-gnu"`,
		`profile = "server"`,
		`output = "bin/my-project"`,
	} {
		if !strings.Contains(configText, want) {
			t.Fatalf("sec.toml missing %q:\n%s", want, configText)
		}
	}
}

func TestInitProjectDoesNotOverwriteMainSource(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "cmd", "main", "main.sec")
	if err := os.MkdirAll(filepath.Dir(mainPath), 0755); err != nil {
		t.Fatal(err)
	}
	existing := []byte("module custom\n")
	if err := os.WriteFile(mainPath, existing, 0644); err != nil {
		t.Fatal(err)
	}

	options := initCommandOptions{
		ProjectDir:  dir,
		ProjectName: "custom",
		Target:      CompilerTarget{OS: "linux", Arch: "amd64"},
		Profile:     "server",
	}
	if err := initProject(options); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(existing) {
		t.Fatalf("main.sec was overwritten. got=%q want=%q", got, existing)
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
		"testdata/build_test.sec": "build_test",
		"program":                 "program",
		"archive.v1/program.sec":  "program",
	}

	for input, want := range tests {
		if got := defaultBuildOutputPath(input); got != want {
			t.Fatalf("defaultBuildOutputPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseMLIRBuildDefaultsToCurrentDirectoryArtifacts(t *testing.T) {
	options, ok := parseBuildCommandOptions(
		[]string{"testdata/ir/test_complex1.sec", "--pipeline", "mlir", "--keep-mlir", "--keep-llvm"},
		CompilerTarget{OS: "linux", Arch: "amd64"},
	)
	if !ok {
		t.Fatal("parseBuildCommandOptions returned ok=false")
	}
	if options.OutputFile != "test_complex1" {
		t.Fatalf("OutputFile = %q, want test_complex1", options.OutputFile)
	}
	if options.MLIROutputFile != "test_complex1.mlir" {
		t.Fatalf("MLIROutputFile = %q, want test_complex1.mlir", options.MLIROutputFile)
	}
	if options.LLVMOutputFile != "test_complex1.ll" {
		t.Fatalf("LLVMOutputFile = %q, want test_complex1.ll", options.LLVMOutputFile)
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
