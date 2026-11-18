package sema

import (
	"context"

	"sec/internal/ast"
)

// Analyze preserves the context-free analysis API. Cancellation-aware compiler
// work uses AnalyzeContext and must discard the analyzer on a cancelled request.
// Rules: rules/compiler/compiler_analysis.md §§9–10,14;
// rules/compiler/compile_time_evaluation.md §§53(6),57(1–3).
func (a *Analyzer) Analyze(program *ast.Program) []Error {
	errors, _ := a.AnalyzeContext(context.Background(), program)
	return errors
}

// AnalyzeContext runs semantic analysis with cooperative CTE cancellation.
// Cancellation is returned as a context error, never a source diagnostic or
// successful analysis. The caller must discard all other facts of an abandoned
// analysis. No cancelled request's CTE values or memoized calls are published.
// An Analyzer is owned by one request and must not be used concurrently.
// Rules: rules/compiler/compile_time_evaluation.md §§53(1–6),57(1–3),60.
func (a *Analyzer) AnalyzeContext(ctx context.Context, program *ast.Program) ([]Error, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	previous := a.cteContext
	a.cteContext = ctx
	defer func() { a.cteContext = previous }()
	if err := ctx.Err(); err != nil {
		a.discardCancelledCTE()
		return nil, err
	}
	errors := a.analyzeProgram(program)
	if err := ctx.Err(); err != nil {
		a.discardCancelledCTE()
		return nil, err
	}
	return errors, nil
}

// compileTimeCancellation polls compiler work without interpreting it as a Sec
// effect, an evaluation budget, or evidence that the program cannot evaluate.
// Rules: rules/compiler/compile_time_evaluation.md §§48–49,53(6),57(1–3).
func (a *Analyzer) compileTimeCancellation() error {
	if a != nil && a.cteContext != nil {
		return a.cteContext.Err()
	}
	return nil
}

// discardCancelledCTE drops request-local outcomes and successes accumulated
// before cancellation; none may become a negative semantic cache entry.
// Rules: rules/compiler/compile_time_evaluation.md §§57(2–3),60.
func (a *Analyzer) discardCancelledCTE() {
	a.cteResults = nil
	a.cteMemo = nil
	a.cteMemoDependencies = nil
	a.ctePending = nil
	a.errors = nil
	a.warnings = nil
}

// OptionalCompileTimeValueContext preserves compiler-work cancellation even
// though ordinary optional-fold failure merely selects runtime evaluation.
// It publishes neither partial values nor request-local memo entries on cancel.
// Rules: rules/compiler/compile_time_evaluation.md §§7(5–7),53(6),57(1–3).
func (a *Analyzer) OptionalCompileTimeValueContext(ctx context.Context, program *ast.Program, expr ast.Expression) (DefaultConstant, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return DefaultConstant{}, false, err
	}
	if a == nil {
		return DefaultConstant{}, false, nil
	}
	previous := a.cteContext
	a.cteContext = ctx
	defer func() { a.cteContext = previous }()
	value, ok, err := a.OptionalCompileTimeValue(program, expr)
	if cancelled := ctx.Err(); cancelled != nil {
		a.cteMemo, a.cteMemoDependencies = nil, nil
		return DefaultConstant{}, false, cancelled
	}
	return value, ok, err
}
