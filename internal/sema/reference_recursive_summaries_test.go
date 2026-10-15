package sema

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Recursive summaries preserve parameter alternatives and field projections,
// independently of declaration order and analysis depth.
// Rules: rules/memory/lifetime_analysis.md — §28.2–28.3.
func TestRecursiveReferenceSummaries(t *testing.T) {
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		for order := 0; order < 3; order++ {
			program := recursiveReferenceFixture(t, "reference_recursive_summaries.sec")
			slots := []int{}
			functions := []ast.Statement{}
			for i, statement := range program.Statements {
				if _, ok := statement.(*ast.FunctionDeclaration); ok {
					slots = append(slots, i)
					functions = append(functions, statement)
				}
			}
			for i, slot := range slots {
				selected := i
				if order == 1 {
					selected = len(functions) - 1 - i
				}
				if order == 2 {
					selected = (i + 1) % len(functions)
				}
				program.Statements[slot] = functions[selected]
			}
			a := NewAnalyzerWithDepth(depth)
			for reuse := 0; reuse < 2; reuse++ {
				if errs := a.Analyze(program); len(errs) != 0 {
					t.Fatal(depth, order, errs)
				}
				for _, name := range []string{"Self", "Alpha", "Beta", "Swap", "Field", "Pair.Recur", "Pair.First"} {
					fn := a.functions[name][0]
					if !fn.HasReturnOrigin || fn.ReturnOrigin.Unknown {
						t.Fatal(depth, order, name, fn.ReturnOrigin)
					}
					roots := []string{}
					for _, place := range localOriginPlaces(fn.ReturnOrigin) {
						roots = append(roots, place.String())
					}
					sort.Strings(roots)
					want := []string{"$param:0"}
					if name == "Swap" {
						want = []string{"$param:0", "$param:1"}
					}
					if name == "Field" {
						want = []string{"$param:0.first"}
					}
					if name == "Pair.First" {
						want = []string{"$receiver.first"}
					}
					if !reflect.DeepEqual(roots, want) {
						t.Fatal(depth, order, name, roots, want)
					}
				}
				for _, record := range a.AnalysisSchedule() {
					if record.ID == "reference-summaries" && (!record.Converged || record.Widened || record.Iterations > len(functions)+1) {
						t.Fatal(record)
					}
				}
				if a.referenceSummaryRound != nil || a.referenceSummaryPending != nil {
					t.Fatal("inference state leaked")
				}
			}
		}
	}
}

// Missing base cases and references to local storage remain rejected; neither
// a recursive equation nor a base case can fabricate a safe returned lifetime.
// Rules: rules/memory/references.md — §9(10); rules/memory/lifetime_analysis.md — §28.3.
func TestRecursiveReferenceSummaryFailures(t *testing.T) {
	for _, test := range []struct {
		file, message string
		count         int
	}{
		{"reference_recursive_unseeded_invalid.sec", "unknown control-flow provenance", 3},
		{"reference_recursive_local_invalid.sec", "cannot return reference to local", 2},
	} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			a := NewAnalyzerWithDepth(depth)
			errs := a.Analyze(recursiveReferenceFixture(t, test.file))
			if len(errs) != test.count || !strings.Contains(errs[0].Message, test.message) {
				t.Fatal(depth, test.file, errs)
			}
			if strings.Contains(test.file, "unseeded") {
				fn := a.functions["Spin"][0]
				if !fn.HasReturnOrigin || !fn.ReturnOrigin.Unknown {
					t.Fatal("unseeded cycle became proof", fn.ReturnOrigin)
				}
			}
			if errs := a.Analyze(recursiveReferenceFixture(t, "reference_recursive_summaries.sec")); len(errs) != 0 {
				t.Fatal("reuse after failure", errs)
			}
		}
	}
}

// Exhausting refinement publishes Unknown before mandatory body consumers.
// Rules: rules/compiler/compiler_analysis.md — §14(1–3), §15(3–5).
func TestRecursiveReferenceSummaryBudget(t *testing.T) {
	a := NewAnalyzerWithDepth(AnalysisInteractive)
	a.analysisBudget.MaxSummaryIterations = 1
	if errs := a.Analyze(recursiveReferenceFixture(t, "reference_recursive_summaries.sec")); len(errs) == 0 {
		t.Fatal("budget exhaustion certified lifetimes")
	}
	for _, name := range []string{"Self", "Alpha", "Beta", "Swap", "Field", "Pair.Recur", "Pair.First"} {
		if !a.functions[name][0].ReturnOrigin.Unknown {
			t.Fatal("incomplete origin retained", name)
		}
	}
	for _, record := range a.AnalysisSchedule() {
		if record.ID == "reference-summaries" && (!record.Widened || record.Converged || record.Iterations != 1) {
			t.Fatal(record)
		}
	}
}

// Source fixtures keep the tests on the ordinary parser/Sema path.
// Rules: rules/memory/lifetime_analysis.md — §28.2–28.3.
func recursiveReferenceFixture(t *testing.T, name string) *ast.Program {
	t.Helper()
	path := "../../testdata/sema/" + name
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), path))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	return program
}

// Local pending equations must never be inferred for opaque declarations, and
// a frozen round must not read a callee's partially updated positive summary.
// Rules: rules/memory/references.md — §9(10); rules/memory/lifetime_analysis.md — §28.2–28.3.
func TestRecursiveReferenceRoundIsolation(t *testing.T) {
	program := recursiveReferenceFixture(t, "reference_recursive_summaries.sec")
	a := NewAnalyzerWithDepth(AnalysisDeep)
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	snapshot := copyFunctionReferenceSummaries(a.functions)
	a.referenceSummaryRound = a.referenceSummaryRoundInputs(program, snapshot)
	a.referenceSummaryPending = map[*ast.CallExpression]bool{}
	a.summaryPass = true
	fn := a.functions["Self"][0]
	fn.ReturnOrigin = localReferenceOrigin{Unknown: true, Ambiguous: true}
	call := &ast.CallExpression{}
	if a.prepareReferenceSummaryCall(call, &fn) || fn.ReturnOrigin.Unknown {
		t.Fatal("read partially updated callee")
	}
	opaque := a.functions["Self"][0]
	opaque.Extern = true
	snapshot["Self"] = []Function{opaque}
	a.referenceSummaryRound = a.referenceSummaryRoundInputs(program, snapshot)
	if a.prepareReferenceSummaryCall(call, &opaque) || a.referenceSummaryPending[call] {
		t.Fatal("opaque callee became a pending local equation")
	}
}
