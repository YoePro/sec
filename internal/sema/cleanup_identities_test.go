package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// P17-18: semantic identity follows binding incarnation and lexical site,
// never a source name or runtime registration instance.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§36,38,42–44.
func TestCleanupIdentityAllocation(t *testing.T) {
	site := func(line int) lexer.Token { return lexer.Token{File: "example.sec", Line: line, Column: 1} }
	build := func() cleanupIdentityAllocator {
		var ids cleanupIdentityAllocator
		if ids.owned(0, site(1)) != 0 || ids.owned(1, lexer.Token{}) != 0 || ids.scope(0, lexer.Token{}) != 0 || ids.deferred(0, site(1)) != (ResolvedDeferIdentity{}) {
			t.Fatal("unknown inputs acquired identities")
		}
		if len(ids.actions)+len(ids.scopes)+len(ids.defers) != 0 {
			t.Fatal("unknown inputs changed identity tables")
		}
		outer := ids.scope(0, site(1))
		inner := ids.scope(outer, site(2))
		first := ids.owned(1, site(3))
		deferred := ids.deferred(inner, site(4))
		reinitialized := ids.owned(1, site(5))
		otherBinding := ids.owned(2, site(3))
		if first != 1 || deferred.Action != 2 || reinitialized != 3 || otherBinding != 4 {
			t.Fatal("actions alias binding incarnations or defer sites")
		}
		if ids.owned(1, site(3)) != first || ids.deferred(inner, site(4)) != deferred || ids.scope(0, site(1)) != outer {
			t.Fatal("revisiting a semantic site created another identity")
		}
		if outer == inner || ids.scope(inner, site(2)) == inner || ids.deferred(outer, site(4)).Defer == deferred.Defer {
			t.Fatal("lexical scope identity was discarded")
		}
		otherFile := site(3)
		otherFile.File = "other.sec"
		if ids.owned(1, otherFile) == first {
			t.Fatal("different initialization sites were merged")
		}
		return ids
	}
	if !reflect.DeepEqual(build(), build()) {
		t.Fatal("allocation is not reproducible")
	}
}

// Source producers publish deterministic, read-only scope/defer identities;
// loop sites remain sites, with dynamic registration deferred to P17-19..24.
// Rules: rules/control-flow/defer.md — §§12–14;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§36,42–44.
func TestCleanupIdentitySourceQueriesAndReset(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/cleanup_identities.sec")
	if err != nil {
		t.Fatal(err)
	}
	parse := func() *ast.Program {
		p := parser.New(lexer.New(string(source)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		return program
	}
	a := NewAnalyzer()
	program := parse()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	if len(a.cleanupDefers) != 3 || len(a.cleanupScopes) < 6 || a.currentCleanupScope != 0 {
		t.Fatalf("unexpected identity inventory: %d defers, %d scopes, current %d", len(a.cleanupDefers), len(a.cleanupScopes), a.currentCleanupScope)
	}
	before := a.cleanupIdentities
	// Copy the maps so a query that allocates an identity is observable.
	wantActions := len(before.actions)
	wantScopes := len(before.scopes)
	wantDefers := len(before.defers)
	seen := map[CleanupActionID]bool{}
	var oldDefer *ast.DeferStatement
	var oldBlock *ast.BlockStatement
	for stmt, want := range a.cleanupDefers {
		got, ok := a.ResolvedDeferIdentityOf(stmt)
		if !ok || got != want || got.Defer == 0 || got.Action == 0 || got.Scope == 0 || seen[got.Action] {
			t.Fatalf("invalid defer identity: %+v", got)
		}
		seen[got.Action] = true
		oldDefer = stmt
		got.Action = 0
		if again, _ := a.ResolvedDeferIdentityOf(stmt); again != want {
			t.Fatal("query mutated the published fact")
		}
	}
	for block, want := range a.cleanupScopes {
		if got, ok := a.ResolvedCleanupScopeOf(block); !ok || got != want || got == 0 {
			t.Fatal("missing block identity")
		}
		for _, statement := range block.Statements {
			if deferred, ok := statement.(*ast.DeferStatement); ok {
				identity, known := a.ResolvedDeferIdentityOf(deferred)
				bodyScope, bodyKnown := a.ResolvedCleanupScopeOf(deferred.Body)
				if !known || identity.Scope != want || !bodyKnown || bodyScope == want {
					t.Fatal("defer lost its enclosing lexical scope or merged it with its body")
				}
			}
		}
		oldBlock = block
	}
	if _, ok := a.ResolvedDeferIdentityOf(&ast.DeferStatement{}); ok {
		t.Fatal("unseen defer acquired an identity")
	}
	if _, ok := a.ResolvedCleanupScopeOf(&ast.BlockStatement{}); ok {
		t.Fatal("unseen block acquired an identity")
	}
	if wantActions != len(before.actions) || wantScopes != len(before.scopes) || wantDefers != len(before.defers) {
		t.Fatal("queries allocated identities")
	}
	if errors := a.Analyze(parse()); len(errors) != 0 {
		t.Fatal(errors)
	}
	if !reflect.DeepEqual(before, a.cleanupIdentities) {
		t.Fatal("reanalysis changed identities or retained an old snapshot")
	}
	if _, ok := a.ResolvedDeferIdentityOf(oldDefer); ok {
		t.Fatal("old defer survived reanalysis")
	}
	if _, ok := a.ResolvedCleanupScopeOf(oldBlock); ok {
		t.Fatal("old block survived reanalysis")
	}
}

// Invalid defer bodies cannot publish identities for later cleanup producers.
// Rules: rules/control-flow/defer.md — §16;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §45.
func TestCleanupIdentityRejectedDefer(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/cleanup_identities_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
	if len(errors) == 0 || len(a.cleanupDefers) != 0 || len(a.cleanupIdentities.actions) != 0 || len(a.cleanupIdentities.defers) != 0 {
		t.Fatalf("rejected defer published identities: errors=%v, facts=%v", errors, a.cleanupDefers)
	}
}
