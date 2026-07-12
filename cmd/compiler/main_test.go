package main

import "testing"

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
