package semantic

import (
	"strings"
	"testing"
)

// A fixed-array spread into an owned heterogeneous pack contributes one
// position per element, in source order and mixed with ordinary arguments
// and other spreads: the ordered sequence joins the specialization identity,
// each source is evaluated exactly once and its elements are copied by
// constant, proven-safe indexed reads.
// Rules: rules/declarations/spread.md — §§ 4, 4.1-4.3; rules/declarations/functions.md — § 38A(4)(2), (6)(1), (9);
// rules/compiler/semantic_ir.md — § 27A.
func TestTypePackFixedArraySpreadSpecializes(t *testing.T) {
	module, err := typePackFixtureModule(t, "fixed_array_spread.sec")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	got := specializationIDs(module)
	for _, id := range []FunctionID{
		"main::Take[...Types<int,int,int>](int,int,int)",
		"main::Take[...Types<bool,int,int,int,uint,bool,bool>](bool,int,int,int,uint,bool,bool)",
		"main::Tagged[...Types<int,int,int>](uint,int,int,int)",
		"main::Take[...Types<int,int,bool>](int,int,bool)",
		"main::Take[...Types<>]()",
	} {
		if got[id] == nil {
			t.Fatalf("missing %s in %v", id, got)
		}
	}
	var use *Function
	for _, fn := range module.Functions {
		if fn.ID == "main::Use()" {
			use = fn
		}
	}
	if use == nil {
		t.Fatal("missing main::Use()")
	}
	body := Format(&Module{Types: module.Types, Functions: []*Function{use}})
	if count := strings.Count(body, `call.direct "main::Make()"`); count != 1 {
		t.Fatalf("Make() evaluated %d times:\n%s", count, body)
	}
	if count := strings.Count(body, "array.extract"); count != 13 {
		t.Fatalf("array.extract count = %d, want 13 (3+3+2+3+2):\n%s", count, body)
	}
	if strings.Contains(body, "proof=runtime") {
		t.Fatalf("a fixed-array spread read needs a runtime bounds check:\n%s", body)
	}
}
