package sema

import (
	"sort"
	"strings"
	"testing"

	"sec/internal/ast"
)

// Lambda captures v2: owned captures are mutable environment fields, the
// callable capability is inferred from how the body uses its environment
// (shared, `mut fn` on mutation or `ref mut`, `-> fn` on moving state out),
// `ref`/`ref mut` captures are reference holders borrowed at construction,
// and closure copyability follows the environment.
//
// Rules:
//   - rules/declarations/lambda-functions.md — §§9–10, 15–20, 23–24, 27–28
//   - rules/analysis/closure_analysis.md — § 8.3 "CaptureMode"
func TestLambdaCaptureModesInferCallableCapability(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `module main

type Res struct {
    id: int,
}

impl Res {
    free {
    }
}

fn Shared() int {
    let factor := 2
    let multiply := capture(factor) fn(value: int) int {
        return value * factor
    }
    return multiply(3)
}

fn Mutable() int {
    let count := 0
    let mut next := capture(count) fn() int {
        count += 1
        return count
    }
    let first := next()
    return first + count
}

fn Consuming() int {
    let r := Res { id: 1 }
    let take := capture(<-r) fn() Res {
        return r
    }
    let owned := take()
    return owned.id
}

fn Borrowing() void {
    let mut total := 1
    let base := 2
    let mut add := capture(ref mut total, ref base) fn() void {
        *total += *base
    }
    add()
}

fn Peeking() int {
    let base := 2
    let peek := capture(ref base) fn() int {
        return *base
    }
    return base + peek()
}

fn Strengthen() int {
    let factor := 2
    let pure := capture(factor) fn() int {
        return factor
    }
    let mut widened: mut fn() int := pure
    return widened()
}
`)
	if len(errors) != 0 {
		t.Fatalf("unexpected errors: %v", errors)
	}
	lambdas := []*ast.LambdaExpression{}
	for expr := range analyzer.expressionTypes {
		if lambda, ok := expr.(*ast.LambdaExpression); ok {
			lambdas = append(lambdas, lambda)
		}
	}
	sort.Slice(lambdas, func(i, j int) bool { return lambdas[i].Token.Line < lambdas[j].Token.Line })
	want := []struct {
		capability CallableCapability
		moveOnly   bool
		name       string
	}{
		{CallableShared, false, "fn(int) int"},
		{CallableMutable, false, "mut fn() int"},
		{CallableConsuming, true, "-> fn() Res"},
		{CallableMutable, true, "mut fn() void"},
		{CallableShared, false, "fn() int"},
		{CallableShared, false, "fn() int"},
	}
	if len(lambdas) != len(want) {
		t.Fatalf("found %d lambdas, want %d", len(lambdas), len(want))
	}
	for i, lambda := range lambdas {
		typ := analyzer.expressionTypes[lambda]
		if normalizedCallableCapability(typ.FunctionCapability) != want[i].capability || typ.FunctionEnvironmentMoveOnly != want[i].moveOnly || typ.Name != want[i].name {
			t.Errorf("lambda at line %d: %s capability %s moveOnly %v, want %s %s %v", lambda.Token.Line, typ.Name, typ.FunctionCapability, typ.FunctionEnvironmentMoveOnly, want[i].name, want[i].capability, want[i].moveOnly)
		}
	}
	if !MoveOnly(analyzer.expressionTypes[lambdas[2]]) || !MoveOnly(analyzer.expressionTypes[lambdas[3]]) || MoveOnly(analyzer.expressionTypes[lambdas[1]]) {
		t.Error("a consuming callable and a ref mut closure are move-only; a mutable closure over copyable state is not")
	}

	records, ok := analyzer.ResolvedLambdaCapturesOf(lambdas[3])
	if !ok || len(records) != 2 {
		t.Fatalf("borrow capture records: %+v", records)
	}
	if records[0].Mode != CaptureMutableBorrow || records[0].Transfer != CaptureTransferBorrow || records[0].SourceBecomesUnavailable ||
		records[1].Mode != CaptureSharedBorrow || records[1].Transfer != CaptureTransferBorrow {
		t.Errorf("borrow capture records: %+v", records)
	}
}

func TestLambdaCaptureModeDiagnostics(t *testing.T) {
	prelude := `module main

type Res struct {
    id: int,
}

impl Res {
    free {
    }
}

`
	cases := []struct {
		name string
		body string
		want string
	}{
		{"mut fn into fn", `fn F() int {
    let count := 0
    let mut next := capture(count) fn() int {
        count += 1
        return count
    }
    let shared: fn() int := next
    return 0
}`, "cannot initialize fn() int with mut fn() int"},
		{"mut fn call needs mutable binding", `fn F() int {
    let count := 0
    let next := capture(count) fn() int {
        count += 1
        return count
    }
    return next()
}`, "calling `mut fn` value next requires mutable access to it"},
		{"consumed callable reuse", `fn F() int {
    let r := Res { id: 1 }
    let take := capture(<-r) fn() Res {
        return r
    }
    let first := take()
    let second := take()
    return 0
}`, "use of moved value take"},
		{"ref mut of immutable", `fn F() int {
    let base := 2
    let bump := capture(ref mut base) fn() void {
    }
    return base
}`, "cannot create mutable reference to immutable variable base"},
		{"conflicting borrow captures", `fn F() int {
    let mut total := 1
    let first := capture(ref mut total) fn() void {
        *total += 1
    }
    let second := capture(ref total) fn() int {
        return *total
    }
    return 0
}`, "cannot create shared reference to total while it is mutably borrowed"},
		{"shared capture is read-only", `fn F() int {
    let base := 2
    let bump := capture(ref base) fn() void {
        *base = 3
    }
    return 0
}`, "cannot assign through shared reference base"},
		{"borrow capture is a reference holder", `fn F() int {
    let base := 2
    let read := capture(ref base) fn() int {
        return base
    }
    return 0
}`, "lambda must return int, got ref int"},
		{"returned closure borrows a local", `fn F(value: int) fn() int {
    return capture(ref value) fn() int {
        return *value
    }
}`, "function F cannot return lambda borrowing local variable value"},
		{"plain capture of move-only stays rejected", `fn F() int {
    let r := Res { id: 1 }
    let take := capture(r) fn() int {
        return 0
    }
    return 0
}`, "cannot copy-capture"},
		{"ref mut closure is move-only", `fn F() int {
    let mut total := 1
    let mut add := capture(ref mut total) fn() void {
        *total += 1
    }
    let copy := add
    add()
    return 0
}`, "cannot copy move-only value add"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, prelude+test.body+"\n")
			for _, err := range errors {
				if strings.Contains(err.Message, test.want) || strings.Contains(err.Help, test.want) {
					return
				}
			}
			t.Fatalf("no diagnostic containing %q; got %v", test.want, errors)
		})
	}
}
