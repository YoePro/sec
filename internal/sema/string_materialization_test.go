package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 5.1–5.15
//   - rules/foundations/operators.md — "String concatenation"
func TestRuntimeStringMaterializationRequiresTry(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

fn Missing(name: string) string {
    return "Hello " + name
}

fn MissingInterpolation(count: int) string {
    return $"count={count}"
}

fn Folded() string {
    return "Hello " + "world" + '!'
}

fn Propagated(name: string) Result[string, AllocationError] {
    return Ok(try "Hello " + name)
}

fn Handled(name: string) string {
    return try "Hello " + name {
        Err(_) => "Hello"
    }
}
`)
	if len(errors) != 2 {
		t.Fatalf("errors = %+v, want two missing-try diagnostics", errors)
	}
	for index, line := range []int{4, 8} {
		if errors[index].ID != "S1097" || errors[index].Line != line || !strings.Contains(errors[index].Message, "requires try") {
			t.Fatalf("error %d = %+v, want S1097 at line %d", index, errors[index], line)
		}
	}
}

func TestStringConcatPlanRecordsResolvedAllocationContext(t *testing.T) {
	source := `module main

fn Render(name: string) Result[string, AllocationError] {
    return Ok(try "Hello " + name)
}

fn Folded() string {
    return "Hello " + "world"
}
`
	result := parser.New(lexer.NewWithFile(source, "allocation.sec")).Parse()
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}
	render := unwrapOkTry(result.Program.Statements[1].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.ReturnStatement).Value)
	plan, ok := analyzer.StringConcatPlanOf(render)
	if !ok || !plan.Runtime {
		t.Fatalf("runtime plan = %#v, found=%t", plan, ok)
	}
	context := plan.Allocation.Context
	if !context.Available || context.Origin != StorageOriginArena || context.Profile != "hosted" || plan.Allocation.FailureType.Name != "AllocationError" {
		t.Fatalf("allocation = %#v, want hosted Arena context with AllocationError", plan.Allocation)
	}
	folded := result.Program.Statements[2].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.ReturnStatement).Value
	if plan, ok := analyzer.StringConcatPlanOf(folded); !ok || plan.Runtime {
		t.Fatalf("folded plan = %#v, found=%t, want compile-time plan", plan, ok)
	}
	summary := analyzer.CallGraph().ArenaSummary(callGraphNodeIDByName(t, analyzer.CallGraph(), "Render"))
	if !summary.MayAllocate {
		t.Fatal("runtime string materialization did not publish an allocation effect")
	}
}

// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 5.18–5.19
//   - rules/memory/allocation.md — § 22(4)
func TestRuntimeStringMaterializationWithoutAllocationContext(t *testing.T) {
	source := `module main

fn Render(name: string) Result[string, AllocationError] {
    return Ok(try "Hello " + name)
}

fn Folded() string {
    return "Hello " + "world"
}
`
	result := parser.New(lexer.NewWithFile(source, "noalloc.sec")).Parse()
	analyzer := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "baremetal", TargetArch: "cortex-m3", Profile: "noalloc", PointerWidthBits: 32})
	errors := analyzer.Analyze(result.Program)
	if len(errors) != 1 || errors[0].ID != "S1098" || errors[0].Line != 4 || !strings.Contains(errors[0].Message, `"noalloc" provides none`) {
		t.Fatalf("errors = %+v, want one S1098 for the runtime plan only", errors)
	}
}
