package mlir

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Rules: rules/foundations/operators.md — Floating equality, ordering and remainder;
// rules/mlir/packages/sec-mlir-dialect_package20.md — §§23–27.
func TestLegacyFloatOperatorMatrix(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/codegen/float_operators/matrix.sec")
	if err != nil {
		t.Fatal(err)
	}
	program := parseTestProgram(t, string(data))
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	text, err := GenerateAnalyzed(program, analyzer, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{"f32", "f64"} {
		for _, predicate := range []string{"oeq", "une", "olt", "ole", "ogt", "oge"} {
			found := false
			for _, line := range strings.Split(text, "\n") {
				if strings.Contains(line, `llvm.fcmp "`+predicate+`"`) && strings.HasSuffix(line, ": "+typ) {
					found = true
				}
			}
			if !found {
				t.Errorf("missing %s comparison in %s", predicate, typ)
			}
		}
	}
	if strings.Contains(text, `llvm.fcmp "one"`) || strings.Contains(text, "fastmath") {
		t.Fatal("unordered inequality or strict floating semantics lost")
	}
	if count := strings.Count(text, " = llvm.frem "); count != 8 {
		t.Fatalf("remainder operations = %d, want eight (binary, local, field and indexed remainder at both widths)", count)
	}
}

// Rules: rules/compiler/compiler_pipeline.md — §§32–34 (lowering prerequisites).
func TestLegacyImportedLayoutsResolveOnConsumption(t *testing.T) {
	for _, name := range []string{"matrix", "use_imported_layout", "use_unsupported_layout"} {
		t.Run(name, func(t *testing.T) {
			path := "../../../testdata/codegen/float_operators/" + name + ".sec"
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			primary := parser.New(lexer.NewWithFile(string(data), name+".sec"))
			program := primary.ParseProgram()
			if len(primary.Errors()) != 0 {
				t.Fatal(primary.Errors())
			}
			data, err = os.ReadFile("../../../testdata/codegen/float_operators/imported_layouts.sec")
			if err != nil {
				t.Fatal(err)
			}
			imported := parser.New(lexer.NewWithFile(string(data), "imported.sec"))
			program.Statements = append(program.Statements, imported.ParseProgram().Statements...)
			if len(imported.Errors()) != 0 {
				t.Fatal(imported.Errors())
			}
			text, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
			if name == "use_unsupported_layout" {
				if err == nil || !strings.Contains(err.Error(), "does not support field type rune") {
					t.Fatalf("consumed unsupported layout: %v", err)
				}
			} else if err != nil || text == "" {
				t.Fatalf("unused unsupported layout blocked supported source: %v", err)
			}
		})
	}
}

func TestLegacyFloatCompareRejectsInvalidOperands(t *testing.T) {
	for _, test := range []struct{ operator, left, right string }{
		{"==", "f32", "f64"}, {"==", "i64", "i64"}, {"invalid", "f64", "f64"},
	} {
		generator := &Generator{}
		if _, err := generator.emitFloatCompare(test.operator, value{typ: test.left}, value{typ: test.right}); err == nil {
			t.Errorf("accepted invalid float comparison %+v", test)
		}
		if generator.out.Len() != 0 {
			t.Fatal("invalid comparison emitted partial output")
		}
	}
}
