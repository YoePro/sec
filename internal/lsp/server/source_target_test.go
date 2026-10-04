package server

import (
	"os"
	"path/filepath"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	platformtarget "sec/internal/platform/target"
	"sec/internal/sema"
)

const (
	sharedPlatformSource  = "module os\n\nfn Open() int { return Native() }\n"
	linuxPlatformSource   = "#target(os: \"linux\", arch: \"amd64\")\nmodule os\n\ntype NativeFileHandle struct {\n}\n\nfn Native() int { return 1 }\n"
	windowsPlatformSource = "#target(os: \"windows\", arch: \"amd64\")\nmodule os\n\ntype NativeFileHandle struct {\n}\n\nfn Native() int { return 2 }\n"
)

func writePlatformModule(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	shared := filepath.Join(dir, "file.sec")
	linux := filepath.Join(dir, "file.linux.amd64.sec")
	windows := filepath.Join(dir, "file.windows.amd64.sec")
	for path, text := range map[string]string{shared: sharedPlatformSource, linux: linuxPlatformSource, windows: windowsPlatformSource} {
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return shared, linux, windows
}

func assembledFiles(program *ast.Program) map[string]bool {
	files := map[string]bool{}
	for _, statement := range program.Statements {
		if function, ok := statement.(*ast.FunctionDeclaration); ok && function.Name != nil {
			files[filepath.Base(function.Name.Token.File)] = true
		}
	}
	return files
}

// Platform-specific files of one module declare the same names for different
// targets; one analysis sees only the files of its target, so they never
// collide.
//
// Rules:
//   - rules/platform/platform_model.md — source selection by target
//   - rules/projects/modules.md — "Source directory and module membership"
func TestAssembleModuleSelectsSiblingsForTheActiveTarget(t *testing.T) {
	shared, linux, windows := writePlatformModule(t)
	tests := []struct {
		name   string
		active string
		text   string
		target platformtarget.Target
		want   string
		absent string
	}{
		{"windows document", windows, windowsPlatformSource, platformtarget.Target{OS: "windows", Arch: "amd64"}, "file.sec", "file.linux.amd64.sec"},
		{"linux document", linux, linuxPlatformSource, platformtarget.Target{OS: "linux", Arch: "amd64"}, "file.sec", "file.windows.amd64.sec"},
		{"shared document for windows", shared, sharedPlatformSource, platformtarget.Target{OS: "windows", Arch: "amd64"}, "file.windows.amd64.sec", "file.linux.amd64.sec"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			program := parser.New(lexer.NewWithFile(test.text, test.active)).Parse().Program
			AssembleModuleForTarget(program, test.active, SourceOverlay{}, test.target)
			files := assembledFiles(program)
			if !files[test.want] || files[test.absent] {
				t.Fatalf("assembled files = %v", files)
			}
			if errors := sema.NewAnalyzer().Analyze(program); len(errors) != 0 {
				t.Fatalf("platform files collided: %+v", errors)
			}
		})
	}
}

// Without an explicit target, a directed document selects its own platform
// so it can be edited on any host.
func TestAssembleModuleUsesTheDocumentsOwnTarget(t *testing.T) {
	_, _, windows := writePlatformModule(t)
	program := parser.New(lexer.NewWithFile(windowsPlatformSource, windows)).Parse().Program
	AssembleModule(program, windows, SourceOverlay{})
	if files := assembledFiles(program); files["file.linux.amd64.sec"] {
		t.Fatalf("windows document assembled the linux file: %v", files)
	}
}

func TestProgramMatchesTarget(t *testing.T) {
	parse := func(text string) *ast.Program { return parser.New(lexer.New(text)).Parse().Program }
	linux := platformtarget.Target{OS: "linux", Arch: "amd64"}
	if !ProgramMatchesTarget(parse("module os\n"), linux) {
		t.Fatal("an undirected file applies to every target")
	}
	if !ProgramMatchesTarget(parse("#target(os: \"linux\", arch: \"any\")\nmodule os\n"), linux) {
		t.Fatal("arch any matches")
	}
	if ProgramMatchesTarget(parse("#target(os: \"linux\", arch: \"arm64\")\nmodule os\n"), linux) {
		t.Fatal("another architecture does not match")
	}
}
