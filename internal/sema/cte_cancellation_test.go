package sema

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// cancelAtCheckpoint uses a real cancellation context, with deterministic
// delivery during executor polling rather than a timing-dependent sleep.
type cancelAtCheckpoint struct {
	context.Context
	cancel    context.CancelFunc
	remaining int
}

func (c *cancelAtCheckpoint) Err() error {
	c.remaining--
	if c.remaining == 0 {
		c.cancel()
	}
	return c.Context.Err()
}

func cancellationProgram(t *testing.T) *ast.Program {
	t.Helper()
	path := "../../testdata/sema/cte_cancellation.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), path))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	return program
}

// Cancellation is compiler work failure, not a source diagnostic, failed
// program proof, budget exhaustion or cached success; retry evaluates afresh.
// Rules: rules/compiler/compile_time_evaluation.md §§53(1–6),57(1–3),60.
func TestCTECancellationAnalysis(t *testing.T) {
	for _, when := range []int{1, 12, 120, 600} {
		t.Run(strconv.Itoa(when), func(t *testing.T) {
			program := cancellationProgram(t)
			a := NewAnalyzer()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			request := &cancelAtCheckpoint{ctx, cancel, when}
			diagnostics, err := a.AnalyzeContext(request, program)
			if !errors.Is(err, context.Canceled) || len(diagnostics) != 0 || len(a.errors) != 0 {
				t.Fatalf("cancel = %v, diagnostics = %v / %v", err, diagnostics, a.errors)
			}
			if len(a.cteResults) != 0 || len(a.cteMemo) != 0 || len(a.cteMemoDependencies) != 0 || a.cteContext != nil {
				t.Fatal("cancelled request retained CTE state")
			}
			// The same parsed program remains valid; cancellation never means
			// that its required compile-time values are semantically impossible.
			fresh := a.freshConfiguredAnalyzer()
			if ds, err := fresh.AnalyzeContext(context.Background(), program); err != nil || len(ds) != 0 {
				t.Fatalf("fresh retry = %v, %v", err, ds)
			}
			for _, static := range fresh.ResolvedStaticDeclarations() {
				v, ok := fresh.CompileTimeValueOf(static.Declaration.Value)
				if !ok || v.Value.Integer == nil || v.Value.Integer.String() != "100" {
					t.Fatalf("retry value = %v, %v", v, ok)
				}
			}
		})
	}
}

// Deadline expiry uses the same Cancelled outcome and returns the original
// context error. It cannot become a source error or an optional-fold miss.
// Rules: rules/compiler/compile_time_evaluation.md §§7(5–7),53(6),57(1–3).
func TestCTECancellationDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()
	a := NewAnalyzer()
	if ds, err := a.AnalyzeContext(ctx, cancellationProgram(t)); !errors.Is(err, context.DeadlineExceeded) || len(ds) != 0 {
		t.Fatalf("deadline = %v, %v", err, ds)
	}
	if _, ok, err := a.OptionalCompileTimeValueContext(ctx, nil, nil); ok || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("optional deadline = %v, %v", ok, err)
	}
}

// A reached cancellation yields the compiler-owned Cancelled outcome, no value
// or dependencies, cannot be caught by source try, and outranks budget checks.
// Optional requests and required requests use the same executor.
// Rules: rules/compiler/compile_time_evaluation.md §§7,48–49,53(6),57.
func TestCTECancellationExecutorAndOptional(t *testing.T) {
	program := cancellationProgram(t)
	a := NewAnalyzer()
	if ds := a.Analyze(program); len(ds) != 0 {
		t.Fatal(ds)
	}
	var root ast.Expression
	for _, static := range a.ResolvedStaticDeclarations() {
		if static.Name == "Handled" {
			root = static.Declaration.Value
		}
	}
	if root == nil {
		t.Fatal("missing static")
	}
	for _, optional := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		request := &cancelAtCheckpoint{ctx, cancel, 15}
		if optional {
			value, ok, err := a.OptionalCompileTimeValueContext(request, program, root)
			if ok || value.Integer != nil || !errors.Is(err, context.Canceled) {
				t.Fatalf("optional = %v, %v, %v", value, ok, err)
			}
			if a.cteContext != nil {
				t.Fatal("context leaked")
			}
		} else {
			a.cteContext = request
			executor := newCTEExecutor(a, program)
			result := executor.evaluate(root)
			a.cteContext = nil
			if result.Failure == nil || result.Failure.Kind != cteCancelled || result.Value.Integer != nil || len(result.Dependencies) != 0 {
				t.Fatalf("cancelled outcome = %+v", result)
			}
			a.cteResults = map[ast.Expression]cteResult{root: result}
			before := len(a.errors)
			if !a.reportCompileTimeExecutionFailure(root, "static initializer") || len(a.errors) != before {
				t.Fatal("cancellation emitted a source diagnostic")
			}
			delete(a.cteResults, root)
		}
		cancel()
		a.cteMemo, a.cteMemoDependencies = nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a.cteContext = ctx
	executor := newCTEExecutor(a, program)
	executor.steps = cteMaxExecutionSteps
	if executor.step(expressionToken(root)) || executor.failure.Kind != cteCancelled {
		t.Fatal("budget masked cancellation")
	}
	a.cteContext = nil
}
