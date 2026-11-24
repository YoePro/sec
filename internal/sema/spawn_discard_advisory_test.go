package sema

import (
	"os"
	"reflect"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// MD-133 T07–T09/T14–T15/T35–T36 reuse A3001, its policy and source identity;
// only the final ordinary non-void call in the child receives the explanation.
// Rules: rules/concurrency/spawn.md §3A(8); diagnostics.md §25(8).
func TestSpawnDiscardAdvisory(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/spawn_blocks/advice.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	lines := strings.Split(source, "\n")
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		for _, policy := range []string{"default", "off", "warning", "error"} {
			p := parser.New(lexer.New(source))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			if policy != "default" {
				a.SetDiagnosticRules(diagnostics.RulePolicy{"ownership.implicit-discarded-result": policy})
			}
			var previous []Error
			for attempt := 0; attempt < 2; attempt++ {
				errors := a.Analyze(program)
				facts := a.Warnings()
				if policy == "error" {
					facts = errors
				} else if len(errors) != 0 {
					t.Fatal(errors)
				}
				seen := map[int]bool{}
				spawn, ordinary := 0, 0
				for _, fact := range facts {
					if fact.ID != diagnostics.ImplicitDiscardedResult {
						continue
					}
					severity := diagnostics.SeverityInformation
					if policy == "warning" {
						severity = diagnostics.SeverityWarning
					}
					if policy == "error" {
						severity = diagnostics.SeverityError
					}
					if fact.Severity != severity {
						t.Fatal(fact)
					}
					if seen[fact.Line] {
						t.Fatalf("duplicate occurrence: %+v", fact)
					}
					seen[fact.Line] = true
					line := lines[fact.Line-1]
					switch {
					case strings.Contains(line, "Task advice"), strings.Contains(line, "Thread advice"):
						owner := "Task"
						if strings.Contains(line, "Thread advice") {
							owner = "Thread"
						}
						if !strings.Contains(fact.Message, "successful handle is "+owner+"[void]") || strings.Contains(fact.Message, "Result[") {
							t.Fatal(fact)
						}
						spawn++
					case strings.Contains(line, "ordinary advice"):
						if strings.Contains(fact.Message, "successful handle") {
							t.Fatal(fact)
						}
						ordinary++
					default:
						t.Fatalf("unexpected advice: %+v", fact)
					}
					if fact.PreviousLine != 2 || fact.RelatedLabel != "selected callable declaration" {
						t.Fatalf("lost provenance: %+v", fact)
					}
				}
				wantSpawn, wantOrdinary := 5, 2
				if policy == "off" {
					wantSpawn, wantOrdinary = 0, 0
				}
				if spawn != wantSpawn || ordinary != wantOrdinary {
					t.Fatalf("%s/%s spawn=%d ordinary=%d facts=%v", depth, policy, spawn, ordinary, facts)
				}
				if attempt > 0 && !reflect.DeepEqual(previous, facts) {
					t.Fatal("unstable repeated analysis")
				}
				previous = facts
			}
		}
	}
}

// Invalid/must-use/non-void and unsupported process children never get a
// successful-void-handle explanation; advisory suppression changes no errors.
// Rules: rules/concurrency/spawn.md §3A(8); diagnostics.md §25(8).
func TestSpawnDiscardAdvisoryPrecedence(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/spawn_blocks/advice_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range strings.Split(string(data), "// CASE ")[1:] {
		name, source, _ := strings.Cut(part, "\n")
		t.Run(name, func(t *testing.T) {
			for _, policy := range []string{"off", "default", "error"} {
				p := parser.New(lexer.New(source))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzer()
				if policy != "default" {
					a.SetDiagnosticRules(diagnostics.RulePolicy{"ownership.implicit-discarded-result": policy})
				}
				errors := a.Analyze(program)
				if len(errors) == 0 {
					t.Fatal("mandatory error suppressed")
				}
				for _, fact := range append(errors, a.Warnings()...) {
					if strings.Contains(fact.Message, "successful handle") {
						t.Fatalf("invalid child got explanation: %+v", fact)
					}
					if name == "must-use" && fact.ID == diagnostics.ImplicitDiscardedResult {
						t.Fatalf("must-use got advisory: %+v", fact)
					}
				}
			}
		})
	}
}

// The explanation is kind-specific and idempotent independently of which
// creation kinds have source support. Process creation remains separately gated.
// Rules: rules/concurrency/spawn.md §3A(8); diagnostics.md §25(8).
func TestSpawnDiscardAdvisoryKinds(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/spawn_blocks/advice.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	var source *ast.SpawnExpression
	ast.Visit(program, func(node any) bool {
		if expr, ok := node.(*ast.SpawnExpression); ok && source == nil {
			source = expr
		}
		return true
	})
	if source == nil {
		t.Fatal("missing spawn fixture")
	}
	last := source.Body.Statements[len(source.Body.Statements)-1].(*ast.ExpressionStatement)
	for kind, owner := range map[string]string{"": "Task", "task": "Task", "thread": "Thread", "process": "Process"} {
		expr := *source
		expr.Kind = kind
		original := Error{ID: diagnostics.ImplicitDiscardedResult, Severity: diagnostics.SeverityInformation, Message: "original advisory"}
		a := NewAnalyzer()
		a.implicitDiscards = map[ast.Expression]Error{last.Expression: original}
		a.explainVoidSpawnDiscard(&expr, Type{Kind: VoidType}, 0)
		first := a.implicitDiscards[last.Expression]
		a.explainVoidSpawnDiscard(&expr, Type{Kind: VoidType}, 0)
		if !reflect.DeepEqual(first, a.implicitDiscards[last.Expression]) || len(a.implicitDiscards) != 1 || strings.Count(first.Message, owner+"[void]") != 1 || first.ID != original.ID || first.Severity != original.Severity {
			t.Fatal(kind, first)
		}
	}
}
