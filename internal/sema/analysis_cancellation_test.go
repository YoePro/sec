package sema

import (
	"context"
	"errors"
	"os"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// bodyCancellation delivers cancellation only during ordinary statement
// analysis. No user CTE loop, timeout sleep or source instrumentation is needed.
type bodyCancellation struct {
	context.Context
	cancel   context.CancelFunc
	analyzer *Analyzer
	reached  bool
}

func (c *bodyCancellation) Err() error {
	if c.analyzer.currentStatementToken.Line >= 35 {
		c.reached = true
		c.cancel()
	}
	return c.Context.Err()
}

// TestAnalysisCancellationInsideOrdinaryBody proves cancellation stops non-CTE
// compiler work, restores its request boundary and returns no source diagnostics.
// A fresh request still produces the same complete semantic result.
// Rules: rules/tooling/lsp.md §§6.7(2–3),7; compiler_analysis.md §§9–10,14.
func TestAnalysisCancellationInsideOrdinaryBody(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lsp/cancellation/main.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "cancellation.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		a := NewAnalyzerWithDepth(depth)
		ctx, cancel := context.WithCancel(context.Background())
		request := &bodyCancellation{Context: ctx, cancel: cancel, analyzer: a}
		diagnostics, err := a.AnalyzeContext(request, program)
		cancel()
		if !request.reached || !errors.Is(err, context.Canceled) || diagnostics != nil || len(a.errors) != 0 || len(a.Warnings()) != 0 || len(a.AnalysisSchedule()) != 0 {
			t.Fatal(depth, request.reached, diagnostics, err)
		}
		if a.analysisCancellable || a.cteContext != nil || len(a.cteMemo) != 0 {
			t.Fatal("abandoned request leaked cancellation state")
		}
		fresh := NewAnalyzerWithDepth(depth)
		if diagnostics, err := fresh.AnalyzeContext(context.Background(), program); err != nil || len(diagnostics) != 0 || len(fresh.AnalysisSchedule()) == 0 {
			t.Fatal(diagnostics, err)
		}
	}
}

type brokenCompilerContext struct{ context.Context }

func (brokenCompilerContext) Err() error { panic("compiler defect") }

// TestAnalysisCancellationDoesNotSwallowCompilerPanics keeps compiler defects
// distinct from request cancellation and restores context even while rethrowing.
// Rules: rules/tooling/lsp.md §6.7; compiler_analysis.md §§9–10.
func TestAnalysisCancellationDoesNotSwallowCompilerPanics(t *testing.T) {
	a := NewAnalyzer()
	defer func() {
		if value := recover(); value != "compiler defect" {
			t.Fatalf("lost compiler panic: %v", value)
		}
		if a.analysisCancellable || a.cteContext != nil {
			t.Fatal("request context leaked while rethrowing")
		}
	}()
	a.AnalyzeContext(brokenCompilerContext{context.Background()}, nil)
}
