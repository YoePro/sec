package sema

import (
	"fmt"
	"os"
	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// TestConcurrencyUmbrellaExamples checks the exact published examples at every
// mandatory depth and scalar plan. Creation, execution outcome and application
// error layers remain distinct, including inferred bindings and explicit types.
// Rules: rules/concurrency/concurrency.md §7(1–13); rules/concurrency/tasks.md §§4,12;
// rules/errors/errorhandling.md §§7,12.
func TestConcurrencyUmbrellaExamples(t *testing.T) {
	for _, name := range []string{"fallible_task", "nested_result"} {
		source, err := os.ReadFile("../../testdata/concurrency/umbrella/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []uint16{32, 64} {
			for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
				t.Run(fmt.Sprintf("%s/%d/%s", name, width, depth), func(t *testing.T) {
					p := parser.New(lexer.NewWithFile(string(source), name+".sec"))
					program := p.ParseProgram()
					if len(p.Errors()) != 0 {
						t.Fatal(p.Errors())
					}
					a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
					if errs := a.Analyze(program); len(errs) != 0 {
						t.Fatal(errs)
					}
					child := "int"
					if name == "nested_result" {
						child = "Result[int, ReadError]"
					}
					spawn, await := 0, 0
					_ = walk.Inspect(program, func(node any) error {
						switch expr := node.(type) {
						case *ast.SpawnExpression:
							spawn++
							typ, ok := a.ResolvedTypeOf(expr)
							if !ok || typeDisplayName(typ) != "Result[Task["+child+"], TaskSpawnError]" {
								t.Fatalf("creation layer: %+v", typ)
							}
						case *ast.AwaitExpression:
							await++
							typ, ok := a.ResolvedTypeOf(expr)
							if !ok || typeDisplayName(typ) != "TaskOutcome["+child+"]" {
								t.Fatalf("outcome layer: %+v", typ)
							}
						}
						return nil
					})
					if spawn != 1 || await != 1 {
						t.Fatal("missing example operations", spawn, await)
					}
				})
			}
		}
	}
}

// TestConcurrencyUmbrellaRejectsLegacyShapes prevents the removed infallible,
// direct-T await and flattened application-result forms from returning.
// Rules: rules/concurrency/concurrency.md §7(2–13); rules/concurrency/tasks.md §§4,12.
func TestConcurrencyUmbrellaRejectsLegacyShapes(t *testing.T) {
	source, err := os.ReadFile("../../testdata/concurrency/umbrella/fallible_task_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errs := analyzeSourceRaw(t, string(source))
	if len(errs) != 4 {
		t.Fatal(errs)
	}
	for i, want := range []string{"Result[Task[int], TaskSpawnError]", "await requires an owning Task", "TaskOutcome[int]", "Result[Task[Result[int, ReadError]], TaskSpawnError]"} {
		if !strings.Contains(errs[i].Message, want) {
			t.Fatalf("error %d: %v; want %s", i, errs[i], want)
		}
	}
}

// TestConcurrencyUmbrellaPublishedSources keeps the exact accepted sources in
// the canonical book, rather than testing silently corrected lookalike samples.
// Rules: rules/concurrency/concurrency.md §7(4),§7(13).
func TestConcurrencyUmbrellaPublishedSources(t *testing.T) {
	book, err := os.ReadFile("../../rules/concurrency/concurrency.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"fallible_task", "nested_result"} {
		source, err := os.ReadFile("../../testdata/concurrency/umbrella/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		published := "```sec\n" + strings.TrimSuffix(strings.TrimPrefix(string(source), "module main\n\n"), "\n") + "\n```"
		if !strings.Contains(string(book), published) {
			t.Fatalf("%s differs from its canonical published example", name)
		}
	}
}
