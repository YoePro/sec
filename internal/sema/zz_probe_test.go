package sema

import (
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

func TestZZProbe(t *testing.T) {
	src := "module main\n\ntype Box struct {\n    value: int,\n}\n\nfn Use(b: ref Box) void {\n}\n\nfn Bump(b: ref mut Box) void {\n}\n\nfn F(b: ref Box, m: ref mut Box) void {\n    defer {\n        Use(b)\n        Bump(m)\n    }\n}\n\nfn main() int {\n    return 0\n}\n"
	p := parser.New(lexer.NewWithFile(src, "sample.sec"))
	parsed := p.Parse()
	a := NewAnalyzer()
	if errs := a.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatal(errs)
	}
	for _, plan := range a.resolvedDeferPlans {
		for _, c := range plan.Captures {
			t.Logf("capture kind=%s binding=%v place=%s use=%d:%d", c.Kind, c.Binding, c.Place.String(), c.Use.Line, c.Use.Column)
		}
	}
	for expr, use := range a.resolvedReferenceUses {
		t.Logf("use %s kind=%s borrow=%v", expr.String(), use.Kind, use.Facts.Borrow)
	}
}
