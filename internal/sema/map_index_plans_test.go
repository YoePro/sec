package sema

import (
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Indexed map access (collections.md §§ 14.4–14.5; owner decision
// 2026-11-08): a lookup checks the key against K and yields the V entry
// Place; a missing key panics without try and is IndexError.OutOfBounds under
// try. An indexed assignment with `=` or `<-` is an upsert: it never fails on
// a missing key, needs try, and fails with CollectionError.
func TestMapIndexLookupAndUpsert(t *testing.T) {
	const source = `module main

fn Use(users: ref mut map[string, int]) Result[int, error] {
    let plain := users["a"]
    let checked := try users["b"]
    try users["c"] = 3
    let moved := plain
    try users["d"] <- moved
    return Ok(plain + checked)
}
`
	parsed := parser.New(lexer.New(source))
	program := parsed.ParseProgram()
	if len(parsed.Errors()) > 0 {
		t.Fatal(parsed.Errors())
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("unexpected errors: %v", errors)
	}
	var indexes []*ast.IndexExpression
	walkASTValue(reflect.ValueOf(program), func(node any) {
		if index, ok := node.(*ast.IndexExpression); ok {
			indexes = append(indexes, index)
		}
	})
	if len(indexes) != 4 {
		t.Fatalf("found %d index expressions", len(indexes))
	}
	want := []struct {
		upsert  bool
		failure ArrayIndexFailureMode
		use     ArrayIndexUseKind
	}{
		{false, ArrayIndexFailureOrdinary, ArrayIndexRead},
		{false, ArrayIndexFailureFallible, ArrayIndexRead},
		{true, ArrayIndexFailureNone, ArrayIndexWrite},
		{true, ArrayIndexFailureNone, ArrayIndexWrite},
	}
	for i, expected := range want {
		plan, ok := analyzer.ResolvedMapIndexPlanOf(indexes[i])
		if !ok {
			t.Fatalf("index %d has no map plan", i)
		}
		if plan.Upsert != expected.upsert || plan.FailureMode != expected.failure || plan.UseKind != expected.use ||
			plan.KeyType.Kind != StringType || plan.ValueType.Kind != IntType {
			t.Errorf("index %d plan %+v", i, plan)
		}
		if expected.upsert && (plan.UpsertErrorType.Name != "CollectionError" || !plan.UpsertProtected || plan.ErrorType.Name != "") {
			t.Errorf("upsert %d failure facts %+v", i, plan)
		}
		if !expected.upsert && plan.ErrorType.Name != "IndexError" {
			t.Errorf("lookup %d error type %s", i, plan.ErrorType.Name)
		}
	}

	cases := map[string]struct{ body, want string }{
		"key type": {`fn F(users: map[string, int]) int {
    return users[1]
}`, "map[string, int] key must be string, got int"},
		"upsert needs try": {`fn F(users: ref mut map[string, int]) void {
    users["a"] = 1
}`, "map upsert users[\"a\"] = ... can fail with CollectionError and requires try"},
		"lookup panics under noPanic": {`@noPanic
fn F(users: map[string, int]) int {
    return users["a"]
}`, "does not satisfy @noPanic: reachable may-panic-bounds effect"},
		"lookup error channel": {`fn F(users: map[string, int]) Result[int, CollectionError] {
    let v := try users["a"]
    return Ok(v)
}`, "bodyless try propagates IndexError"},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, "module main\n\n"+test.body+"\n")
			for _, err := range errors {
				if strings.Contains(err.Message, test.want) {
					return
				}
			}
			t.Fatalf("no diagnostic containing %q; got %v", test.want, errors)
		})
	}

	valid := analyzeSourceRaw(t, `module main

@noPanic
fn Checked(users: ref mut map[string, int]) Result[int, error] {
    let v := try users["a"]
    try users["b"] = v
    return Ok(v)
}
`)
	if len(valid) != 0 {
		t.Fatalf("try lookup and try upsert satisfy @noPanic: %v", valid)
	}
}
