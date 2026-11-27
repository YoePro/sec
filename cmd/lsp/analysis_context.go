package main

import (
	"context"
	"sec/internal/ast"
	"sec/internal/sema"
)

// analysisEntryMutex permits a reader to abandon a cache lease while the
// producer or another reader still owns it. Cancellation never steals a lease.
// Rules: rules/tooling/lsp.md — Snapshots, Responsiveness model, §6.7(2–3).
type analysisEntryMutex struct{ token chan struct{} }

func newAnalysisEntryMutex() analysisEntryMutex {
	return analysisEntryMutex{token: make(chan struct{}, 1)}
}

func (m *analysisEntryMutex) Lock()   { _ = m.LockContext(context.Background()) }
func (m *analysisEntryMutex) Unlock() { <-m.token }

func (m *analysisEntryMutex) LockContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case m.token <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// analysisContext carries request withdrawal without fingerprinting it as a
// semantic dependency. Direct/context-free clients use the same compiler path.
// Rules: rules/tooling/lsp.md — Snapshots, §6.7(2–3).
func (overlay sourceOverlay) analysisContext() context.Context {
	if overlay.workContext != nil {
		return overlay.workContext
	}
	return context.Background()
}

// analyzeCompilerSnapshot uses request cancellation for private proof analyses
// too (rename and verified code actions), never returning partial proofs.
// Rules: rules/tooling/lsp.md — Safe fixes, Snapshots, §6.7(2–3).
func analyzeCompilerSnapshot(analyzer *sema.Analyzer, program *ast.Program, overlay sourceOverlay) []sema.Error {
	diagnostics, err := analyzer.AnalyzeContext(overlay.analysisContext(), program)
	if err != nil {
		panic(cancelledLSPAnalysis{err})
	}
	return diagnostics
}
