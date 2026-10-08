package readiness_test

import (
	"errors"
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/codegen/llvm"
	"sec/internal/codegen/mlir"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Rules: rules/concurrency/mutex.md §13; types/temporal.md §4.
func TestLegacyBackendsRejectInstant(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/sema/temporal/instant_identity.sec")
	if err != nil {
		t.Fatal(err)
	}
	for name, generate := range map[string]func(*ast.Program) (string, error){"llvm": llvm.Generate, "mlir": (&mlir.Generator{}).Generate} {
		t.Run(name, func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "instant.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			output, err := generate(parsed.Program)
			var unsupported *semantic.UnsupportedFeatureError
			if output != "" || !errors.As(err, &unsupported) || unsupported.Location.File != "instant.sec" {
				t.Fatal(output, err)
			}
		})
	}
}

// Rules: rules/concurrency/mutex.md §13; compiler/semantic_ir.md — lowering prerequisites.
func TestSemanticIRRejectsInstantRepresentation(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/sema/temporal/instant_identity.sec")
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(source), "instant.sec")).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(parsed.Program); len(errors) > 0 {
		t.Fatal(errors)
	}
	_, err = semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "application", SourceFiles: []string{"instant.sec"}, MaxPackage: 14})
	var unsupported *semantic.UnsupportedFeatureError
	if !errors.As(err, &unsupported) {
		t.Fatalf("opaque representation silently lowered: %v", err)
	}
}
