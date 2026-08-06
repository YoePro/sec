package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

func TestDiagnosticCountLabel(t *testing.T) {
	tests := []struct {
		count    int
		singular string
		want     string
	}{
		{count: 0, singular: "error", want: "0 errors"},
		{count: 1, singular: "error", want: "1 error"},
		{count: 2, singular: "warning", want: "2 warnings"},
	}

	for _, tt := range tests {
		if got := diagnosticCountLabel(tt.count, tt.singular); got != tt.want {
			t.Fatalf("diagnosticCountLabel(%d, %q) = %q, want %q", tt.count, tt.singular, got, tt.want)
		}
	}
}

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

	got = stdlibModulePath("unicode", CompilerTarget{})
	want = "sec/stdlib/unicode/unicode.sec"
	if got != want {
		t.Fatalf("stdlibModulePath(%q) = %q, want %q", "unicode", got, want)
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

func TestSourceIncludePathsLoadsPlatformPackageDirectory(t *testing.T) {
	paths, ok := sourceIncludePaths("platform/linux/amd64", CompilerTarget{OS: "linux", Arch: "amd64"})
	if !ok {
		t.Fatal("sourceIncludePaths did not accept platform package")
	}
	wants := map[string]bool{
		"raw_syscall.sec":     false,
		"syscall_numbers.sec": false,
	}
	for _, path := range paths {
		if _, exists := wants[filepath.Base(path)]; exists {
			wants[filepath.Base(path)] = true
		}
	}
	for path, found := range wants {
		if !found {
			t.Fatalf("platform package missing %s: %#v", path, paths)
		}
	}
}

func TestSourceIncludePathsLoadsIOPackageFiles(t *testing.T) {
	paths, ok := sourceIncludePaths("io", CompilerTarget{OS: "linux", Arch: "amd64"})
	if !ok {
		t.Fatal("sourceIncludePaths did not accept io package")
	}
	wants := map[string]bool{
		"write.linux.amd64.sec": false,
		"file.linux.amd64.sec":  false,
	}
	for _, path := range paths {
		if _, exists := wants[filepath.Base(path)]; exists {
			wants[filepath.Base(path)] = true
		}
	}
	for path, found := range wants {
		if !found {
			t.Fatalf("io package missing %s: %#v", path, paths)
		}
	}
}

func TestCompilerLoadsIOReadFileIntoAPI(t *testing.T) {
	source := `module main

import "io"

fn Load(path: string) Result[uint, io.IOError] {
	let mut buffer: byte[16] := [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
	let count := try io.ReadFileInto(path, ref mut buffer[..])

	return Ok(count)
}
`

	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	target := CompilerTarget{OS: "linux", Arch: "amd64"}
	resolveCoreLibrary(program)
	resolveStdlibImports(program, target)
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) > 0 {
		t.Fatalf("Analyze(user with io file API) errors: %v", errors)
	}
}

func TestCompilerLoadsIOWriteSeekAndDirectoryAPIs(t *testing.T) {
	source := `module main

import "io"

fn Update(path: string, text: string) Result[uint, io.IOError] {
	let mut file := try io.OpenReadWrite(path, true)
	let written := try file.WriteString(text)
	let position := try file.Seek(0, io.SeekOrigin.Start)
	try file.Flush()
	try file.Close()

	return Ok(written + position)
}

fn Save(path: string, text: string) Result[void, io.IOError] {
	try io.WriteStringFile(path, text)
	return Ok()
}

fn SaveBytes(path: string, data: ref byte[]) Result[void, io.IOError] {
	try io.WriteFile(path, ref data[..])
	return Ok()
}

fn Append(path: string, text: string) Result[uint, io.IOError] {
	let mut file := try io.OpenAppend(path)
	let written := try file.WriteString(text)
	try file.Close()
	return Ok(written)
}

fn FirstEntry(path: string) Result[Option[io.DirectoryEntry], io.IOError] {
	let mut directory := try io.OpenDirectory(path)
	let entry := try directory.Next()
	try directory.Close()

	return Ok(entry)
}

fn ListEntries(path: string, entries: ref mut io.DirectoryEntry[]) Result[uint, io.IOError] {
	let count := try io.ReadDirectoryInto(path, ref mut entries[..])
	return Ok(count)
}

fn EntryName(entry: io.DirectoryEntry) string {
	return entry.Name
}

fn EntryType(entry: io.DirectoryEntry) io.DirectoryEntryType {
	return entry.Type
}

fn CheckAndRename(oldPath: string, newPath: string) Result[bool, io.IOError] {
	let exists := try io.Exists(oldPath)
	try io.Access(oldPath, io.AccessMode.Read)
	try io.Rename(oldPath, newPath)
	return Ok(exists)
}

fn DirectoryLifecycle(path: string) Result[void, io.IOError] {
	try io.CreateDirectory(path)
	try io.RemoveDirectory(path)
	return Ok()
}

fn Remove(path: string) Result[void, io.IOError] {
	try io.RemoveFile(path)
	return Ok()
}

fn CopyHandles(
	source: ref mut io.File,
	destination: ref mut io.File,
	buffer: ref mut byte[],
) Result[uint, io.IOError] {
	let copied := try io.Copy(source, destination, ref mut buffer[..])
	return Ok(copied)
}

fn ReadFixed(file: ref mut io.File, buffer: ref mut byte[]) Result[uint, io.IOError] {
	let count := try file.ReadExact(ref mut buffer[..])
	try file.Truncate(count)
	return Ok(count)
}
`

	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	target := CompilerTarget{OS: "linux", Arch: "amd64"}
	resolveCoreLibrary(program)
	resolveStdlibImports(program, target)
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) > 0 {
		t.Fatalf("Analyze(user with complete io API) errors: %v", errors)
	}
}

func TestCompilerLoadsUnicodeIsLetter(t *testing.T) {
	source := `module main

import "unicode"

fn IsLetter(ch: rune) bool {
	return unicode.IsLetter(ch)
}
`

	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	resolveCoreLibrary(program)
	resolveStdlibImports(program, CompilerTarget{})
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) > 0 {
		t.Fatalf("Analyze(user with unicode IsLetter) errors: %v", errors)
	}
}

func TestSourceIncludePathsLoadsProjectImportsFromSecProjectRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".sec", "sec.toml"), []byte("[project]\nname = \"sample\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	sourceFile := filepath.Join(dir, "cmd", "sec", "main.sec")
	if err := os.MkdirAll(filepath.Dir(sourceFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourceFile, []byte("module main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	imported := filepath.Join(dir, "lexer", "token.sec")
	if err := os.MkdirAll(filepath.Dir(imported), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(imported, []byte("module token\n"), 0644); err != nil {
		t.Fatal(err)
	}

	paths, ok := sourceIncludePathsWithSources("lexer/token", CompilerTarget{}, []string{sourceFile})
	if !ok {
		t.Fatal("sourceIncludePathsWithSources did not accept project import")
	}
	if len(paths) != 1 || paths[0] != imported {
		t.Fatalf("project import paths = %#v, want %#v", paths, []string{imported})
	}
}

func TestCompilerAllowsCoreStringImpl(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "sec", "core", "string.sec")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0755); err != nil {
		t.Fatal(err)
	}
	source := `module string

impl string {
	fn Len() uint {
		return self.len
	}
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}

	program := parseSourceFile(sourcePath)
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) > 0 {
		t.Fatalf("Analyze(core string) errors: %v", errors)
	}
}

func TestCompilerLoadsCoreLibraryBeforeUserAnalysis(t *testing.T) {
	source := `module main

fn IsBlank(value: string) bool {
	return value.IsEmpty()
}
`

	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	resolveCoreLibrary(program)
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) > 0 {
		t.Fatalf("Analyze(user with core method) errors: %v", errors)
	}
}

func TestCompilerLoadsCoreRuneAndStringConversionAPI(t *testing.T) {
	source := `module main

fn Runes(value: string) rune[] {
	return value.ToRuneArray()
}

fn Display(value: rune) string {
	return value.ToString()
}

fn IsIdentifierStart(value: rune) bool {
	return value.IsLetter()
}
`

	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	resolveCoreLibrary(program)
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) > 0 {
		t.Fatalf("Analyze(user with core rune and string conversion API) errors: %v", errors)
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
		filepath.Join(dir, "cmd", "my-project"),
		filepath.Join(dir, "bin"),
		filepath.Join(dir, ".sec"),
		filepath.Join(dir, "internal"),
	} {
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			t.Fatalf("%s was not created as directory", path)
		}
	}

	mainSource, err := os.ReadFile(filepath.Join(dir, "cmd", "my-project", "main.sec"))
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
		`[build]`,
		`backend = "mlir"`,
		`profile = "server"`,
		`diagnostics = "default"`,
		`[profile.server]`,
		`[variant.linux-amd64]`,
		`os = "linux"`,
		`arch = "amd64"`,
		`llvm_triple = "x86_64-pc-linux-gnu"`,
		`[target.my-project]`,
		`kind = "command"`,
		`source = "cmd/my-project"`,
		`artifact = "my-project"`,
		`variants = ["linux-amd64"]`,
		`output = "bin/my-project/linux-amd64/server/my-project"`,
	} {
		if !strings.Contains(configText, want) {
			t.Fatalf("sec.toml missing %q:\n%s", want, configText)
		}
	}
}

func TestInitProjectDoesNotOverwriteMainSource(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "cmd", "custom", "main.sec")
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

func TestInitProjectIsIdempotentWhenManifestExists(t *testing.T) {
	dir := t.TempDir()
	options := initCommandOptions{
		ProjectDir:  dir,
		ProjectName: "demo",
		Target:      CompilerTarget{OS: "linux", Arch: "amd64"},
		Profile:     "server",
	}

	if err := initProject(options); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(dir, ".sec", "sec.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := initProject(options); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(dir, ".sec", "sec.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != string(first) {
		t.Fatalf("sec.toml changed on idempotent init\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestInitProjectRejectsNestedManifest(t *testing.T) {
	root := t.TempDir()
	options := initCommandOptions{
		ProjectDir:  root,
		ProjectName: "root",
		Target:      CompilerTarget{OS: "linux", Arch: "amd64"},
		Profile:     "server",
	}
	if err := initProject(options); err != nil {
		t.Fatal(err)
	}

	nested := options
	nested.ProjectDir = filepath.Join(root, "nested")
	nested.ProjectName = "nested"
	if err := initProject(nested); err == nil || !strings.Contains(err.Error(), "nested Sec project manifest is not allowed") {
		t.Fatalf("initProject nested error = %v, want nested manifest error", err)
	}
	if fileExists(filepath.Join(nested.ProjectDir, ".sec", "sec.toml")) {
		t.Fatal("nested sec.toml was created")
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
