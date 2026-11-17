package sema

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestMutexGuardForwarding verifies ordinary fields, properties, overloads,
// method/owner generics and returned references on all analysis budgets/widths.
// Invalid cases retain original visibility, guard authority and lifetime.
// Rules: rules/concurrency/mutex.md — §§29–35.
func TestMutexGuardForwarding(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files []string
		want  int
	}{
		{"valid", []string{"mutex_forwarding_valid.sec"}, 0},
		{"invalid", []string{"mutex_forwarding_invalid.sec"}, 15},
		{"modules", []string{"mutex_forwarding_owner.sec", "mutex_forwarding_client_invalid.sec"}, 4},
	} {
		for _, width := range []uint16{32, 64} {
			for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
				t.Run(fmt.Sprintf("%s/%d/%s", tc.name, width, depth), func(t *testing.T) {
					program := &ast.Program{}
					for _, name := range tc.files {
						source, err := os.ReadFile("../../testdata/sema/" + name)
						if err != nil {
							t.Fatal(err)
						}
						p := parser.New(lexer.NewWithFile(string(source), name))
						part := p.ParseProgram()
						if len(p.Errors()) != 0 {
							t.Fatal(p.Errors())
						}
						program.Statements = append(program.Statements, part.Statements...)
					}
					a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
					errors := a.Analyze(program)
					if len(errors) != tc.want {
						t.Fatalf("got %d errors, want %d: %+v", len(errors), tc.want, errors)
					}
					if tc.name == "valid" {
						assertMutexForwardingFacts(t, a)
					}
					if tc.name == "invalid" {
						assertMutexForwardingDiagnostics(t, program, errors)
					}
					if tc.name == "modules" {
						for _, e := range errors {
							if !strings.Contains(e.Message, "accessib") && !strings.Contains(e.Message, "visible") {
								t.Fatal(e)
							}
						}
					}
				})
			}
		}
	}
}

// assertMutexForwardingFacts checks the published protected owner while the
// source receiver keeps its nominal guard identity and reference provenance.
// Rules: rules/concurrency/mutex.md — §§30(3–5),31(4),35.
func assertMutexForwardingFacts(t *testing.T, a *Analyzer) {
	t.Helper()
	fields, properties, methods, references := 0, 0, 0, 0
	for expr, typ := range a.expressionTypes {
		member, ok := expr.(*ast.MemberExpression)
		if !ok || !isMutexGuardType(dereferenceType(a.expressionTypes[member.Object])) {
			continue
		}
		owner := dereferenceType(a.expressionTypes[member.Object]).TypeArgs[0]
		if plan, ok := a.ResolvedStructMemberOf(member); ok {
			if typeDisplayName(plan.OwnerType) != typeDisplayName(owner) {
				t.Fatalf("wrong declaring owner: %+v", plan)
			}
			if plan.Kind == MemberStoredField {
				fields++
			}
			if plan.Kind == MemberProperty {
				properties++
			}
		}
		if typ.Kind == ReferenceType {
			origin, ok := a.knownReturnedReferenceOrigin(member)
			if !ok || len(localOriginPlaces(origin)) == 0 {
				t.Fatalf("missing guard provenance for %s", member)
			}
			references++
		}
	}
	for call, resolved := range a.resolvedCalls {
		member, ok := call.Callee.(*ast.MemberExpression)
		if !ok || !isMutexGuardType(dereferenceType(a.expressionTypes[member.Object])) {
			continue
		}
		if resolved.Function.ImplTarget == "MutexGuard" {
			t.Fatal("forwarding erased declaring method owner")
		}
		methods++
	}
	if fields == 0 || properties == 0 || methods == 0 || references == 0 {
		t.Fatalf("missing evidence: fields=%d properties=%d methods=%d references=%d", fields, properties, methods, references)
	}
}

// assertMutexForwardingDiagnostics requires one expected diagnostic in every
// invalid function and acquisition provenance for each escaping reference.
// Rules: rules/concurrency/mutex.md — §§32–35, especially §35(4).
func assertMutexForwardingDiagnostics(t *testing.T, program *ast.Program, errors []Error) {
	t.Helper()
	expected := map[string]string{
		"MissingTry":     "try",
		"ImmutableField": "immutable place", "ImmutableMethod": "mutable receiver",
		"ImmutableProperty": "immutable place", "ImmutableGetter": "mutable receiver",
		"SharedField": "immutable place", "SharedMethod": "mutable receiver", "SharedGetter": "mutable receiver",
		"PrivateProperty": "not visible", "EscapeField": "reference to local variable guard",
		"EscapeMethod": "reference to local variable guard", "EscapeProperty": "reference to local variable guard",
		"BorrowThenRelease": "while it is borrowed", "MethodThenRelease": "while it is borrowed", "PropertyThenRelease": "while it is borrowed",
	}
	seen := map[string]bool{}
	for _, e := range errors {
		var owner *ast.FunctionDeclaration
		for _, statement := range program.Statements {
			if fn, ok := statement.(*ast.FunctionDeclaration); ok && fn.Name.Token.Line <= e.Line {
				owner = fn
			}
		}
		if owner == nil {
			t.Fatal(e)
		}
		name := owner.Name.Value
		want, exists := expected[name]
		if !exists || seen[name] || !strings.Contains(e.Message, want) {
			t.Fatalf("%s: unexpected diagnostic %+v", name, e)
		}
		seen[name] = true
		if strings.HasPrefix(name, "Escape") {
			guard := owner.Body.Statements[0].(*ast.LetStatement)
			if e.PreviousLine != guard.Name.Token.Line {
				t.Fatalf("missing guard acquisition location: %+v", e)
			}
		}
	}
	if len(seen) != len(expected) {
		t.Fatal("missing invalid paths", seen)
	}
}
