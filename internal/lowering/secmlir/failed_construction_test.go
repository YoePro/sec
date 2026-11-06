package secmlir

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// P17-16 failed construction reaches verified schema 13: struct fields, union
// fields, array elements and fresh call arguments already evaluated are
// destroyed in reverse order on a failing later operand's exit (propagation
// or a handler arm's return), before function cleanup, and the outer value is
// never built there. Fields below a non-trivial field are read in place.
// Rules: rules/memory/destruction.md — §§ 7, 8, 14.2; package17 §§ 12, 14, 65.
func TestSchema13CleansFailedConstruction(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/failed_construction.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "construction.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"construction.sec"}, MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			text := semantic.Format(module)
			body := func(name string) string {
				start := strings.Index(text, `func "main::`+name+`(`)
				if start < 0 {
					t.Fatalf("missing function %s", name)
				}
				end := strings.Index(text[start:], "\n  }\n")
				return text[start : start+end]
			}
			for name, want := range map[string]int{"StructField": 1, "ReverseOrder": 2, "ArrayElement": 1, "Argument": 1, "UnionFields": 1, "HandlerReturn": 1, "HandlerBreak": 1} {
				if got := strings.Count(body(name), "cause=temporary-end"); got != want {
					t.Errorf("%s destroys %d prepared operands on its failure exit, want %d:\n%s", name, got, want, body(name))
				}
			}
			// Reverse evaluation order: the second prepared field ends first.
			reverse := body("ReverseOrder")
			first, second := strings.Index(reverse, "own.destroy-value %3"), strings.Index(reverse, "own.destroy-value %1")
			if first < 0 || second < 0 || first > second {
				t.Errorf("ReverseOrder does not destroy in reverse order:\n%s", reverse)
			}
			// P17-23: the moved argument is moved out only on the call's
			// success path, after the failing argument's branch.
			for _, name := range []string{"MovedArgument", "MovedStorageArgument"} {
				part := body(name)
				branch, move := strings.Index(part, "conditional-branch"), strings.Index(part, "own.move")
				if branch < 0 || move < branch || strings.Contains(part, "cause=temporary-end") {
					t.Errorf("%s moves its argument before the call commits:\n%s", name, part)
				}
			}
			if part := body("NestedRead"); strings.Count(part, "place.read") < 4 || strings.Contains(part, "non-trivial") {
				t.Errorf("NestedRead does not read nested fields in place:\n%s", part)
			}
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
			if err != nil {
				t.Fatal(err)
			}
			verify := exec.Command(tool, "--sec-verify-slice-guards", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, result)
			}
		})
	}
}
