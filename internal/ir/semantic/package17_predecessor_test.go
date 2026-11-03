package semantic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Every valid source fixture that builds and verifies at package 16 still does
// at package 17: P17 adds ownership lowering without taking away a predecessor
// form (P17-02, P17-56, P17-61). A moved reference holder and a forwarded
// trivial pack were such regressions.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 2, 132, 136.
func TestPackage17KeepsPackage16Programs(t *testing.T) {
	var files []string
	for _, pattern := range []string{"../../../testdata/semantic_ir/*.sec", "../../../testdata/semantic_ir/*/*.sec", "../../../testdata/generics/*/*.sec"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}
	build := func(source []byte, path string, maxPackage uint8) error {
		parsed := parser.New(lexer.NewWithFile(string(source), path)).Parse()
		analyzer := sema.NewAnalyzer()
		if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
			return errors[0]
		}
		module, err := Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: maxPackage})
		if err != nil {
			return err
		}
		return Verify(module)
	}
	checked := 0
	for _, path := range files {
		if strings.Contains(path, "invalid") {
			continue
		}
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		parsed := parser.New(lexer.NewWithFile(string(source), path)).Parse()
		if parsed.HasErrors || build(source, path, 16) != nil {
			continue
		}
		checked++
		if err := build(source, path, 17); err != nil {
			t.Errorf("%s builds at package 16 but not 17: %v", path, err)
		}
	}
	if checked < 100 {
		t.Fatalf("only %d predecessor fixtures checked", checked)
	}
}
