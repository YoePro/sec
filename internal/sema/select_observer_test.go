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

// TestSelectProcessObserver validates repeatable non-owning waits, exact branch
// binding type, copyability, ordinary blocking and nonblocking select preparation.
// Rules: rules/concurrency/select.md §§25–26,54; rules/concurrency/processes.md §5.14,§21.
func TestSelectProcessObserver(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/select_observer/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			t.Run(fmt.Sprintf("%d/%s", width, depth), func(t *testing.T) {
				p := parser.New(lexer.NewWithFile(string(source), "observer.sec"))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
				errs := a.Analyze(program)
				if len(errs) != 0 {
					t.Fatal(errs)
				}
				waits := 0
				_ = walk.Inspect(program, func(node any) error {
					call, ok := node.(*ast.CallExpression)
					if !ok {
						return nil
					}
					member, ok := call.Callee.(*ast.MemberExpression)
					if !ok || member.Property.Value != "Wait" {
						return nil
					}
					waits++
					typ, ok := a.ResolvedTypeOf(call)
					if !ok || typ.Name != "ProcessStatus" || typ.Kind != EnumType {
						t.Fatal("wait result", typ)
					}
					return nil
				})
				if waits != 6 {
					t.Fatal("waits", waits)
				}
			})
		}
	}
}

// TestSelectObserverRejectsHiddenWaits accepts only the exact intrinsic process
// operation, never a task/thread observer or a same-spelled user method.
// Rules: rules/concurrency/select.md §§25–26,51,54; rules/concurrency/processes.md §5.14.
func TestSelectObserverRejectsHiddenWaits(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/select_observer/invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errs := analyzeSourceRaw(t, string(source))
	if len(errs) != 9 {
		t.Fatalf("errors: %+v", errs)
	}
	for _, err := range errs {
		if err.Message == "" || err.Line == 0 {
			t.Fatal(err)
		}
	}
	combined := ""
	for _, err := range errs {
		combined += err.Message + "\n"
	}
	for _, want := range []string{"ThreadObserver has no wait", "operation is not selectable", "ProcessObserver.Wait expects 0 arguments", "join requires", "detach requires", "has no source struct constructor", "has no source constructor"} {
		if !strings.Contains(combined, want) {
			t.Fatalf("missing %s: %s", want, combined)
		}
	}
}

// TestSelectObserverBlockingPhases distinguishes ordinary blocking waits from
// readiness preparation. A default select cannot acquire a hidden Wait effect;
// a waiting select contributes its own boundary instead of every candidate.
// Rules: rules/concurrency/select.md §§4,25,40–41; rules/concurrency/processes.md §21.
func TestSelectObserverBlockingPhases(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/select_observer/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errs := analyzeSourceWithAnalyzerRaw(t, string(source))
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	graph := a.CallGraph()
	seen := map[string]bool{}
	for _, node := range graph.Nodes() {
		switch node.Name {
		case "Ordinary", "Selected", "Waiting", "NonBlockingSelect":
			seen[node.Name] = true
			summary := graph.BlockSummary(node.ID)
			expected := map[string]int{"Ordinary": 2, "Selected": 1, "Waiting": 1, "NonBlockingSelect": 0}[node.Name]
			if len(summary.DirectEffects) != expected || summary.MayBlock != (expected != 0) {
				t.Fatalf("%s: %+v", node.Name, summary)
			}
			for _, effect := range summary.DirectEffects {
				want := "ProcessObserver.Wait"
				if node.Name == "Waiting" {
					want = "select without default"
				}
				if effect.Operation != want {
					t.Fatalf("%s: %+v", node.Name, effect)
				}
			}
		}
	}
	if len(seen) != 4 {
		t.Fatal("missing phase checks", seen)
	}
}

// TestProcessObserverBoundaries keeps the implicit compiler-known observer
// opaque and nondefaultable while leaving target ProcessPlatform declarations
// with their owning implementation rather than fabricating a portable layout.
// Rules: rules/concurrency/processes.md §§5.14–5.15.
func TestProcessObserverBoundaries(t *testing.T) {
	typ := NewAnalyzer().Types()["ProcessObserver"]
	if !isProcessObserverType(typ) || typ.ExplicitlyNonCopyable || len(typ.Fields) != 0 || IsDefaultable(typ) {
		t.Fatal(typ)
	}
	source, err := os.ReadFile("../../testdata/sema/select_observer/platform_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errs := analyzeSourceRaw(t, string(source))
	if len(errs) != 1 || !errs[0].IsImplementationCapability() || !strings.Contains(errs[0].Message, "ProcessObserver.Platform") {
		t.Fatal(errs)
	}
}

// TestProcessObserverWaitCannotBeReplaced prevents an extension method from
// changing the exact selected operation's compiler-owned result contract.
// Rules: rules/concurrency/select.md §§25,54(5); rules/concurrency/processes.md §5.14.
func TestProcessObserverWaitCannotBeReplaced(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/select_observer/replacement_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errs := analyzeSourceRaw(t, string(source))
	if len(errs) == 0 {
		t.Fatal("source extension replaced the compiler-owned wait contract")
	}
}
