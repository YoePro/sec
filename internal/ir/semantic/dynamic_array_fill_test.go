package semantic

import (
	"strings"
	"testing"
)

// dynamicArrayFillSource fills owning arrays from an unsigned runtime count
// and a non-negative literal count.
const dynamicArrayFillSource = "module main\n\nfn Make(n: uint) Result[uint, CollectionError] {\n    let values: int[] := try fill(7, n)\n    let five: int[] := try fill(1, 5)\n    return Ok(values.Len + five.Len)\n}\n\nfn main() int {\n    return 0\n}\n"

// `try fill(value, count)` into an owning dynamic array evaluates the value
// once, appends count copies to a temporary owner through the P18 growth
// protocol and moves it into Ok; a failed growth destroys the prefix and
// yields Err(CollectionError.AllocationFailed) without a panic endpoint. A
// signed runtime count stays a named boundary (MD-155).
// Rules: rules/compiler/compiler_known_members.md — § 50(2).
func TestDynamicArrayFillSemanticIR(t *testing.T) {
	module, err := hostedModule(t, dynamicArrayFillSource, 18)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		"fill.result", "dynamic-array.empty", "transition=establish-domain", "transition=advance-epoch",
		"dynamic-array.append-prepared", "int.compare lt", "[loop=", "own.move-from-place", "result.ok",
		`cause=temporary-end`, "enum.constant case=#0", "result.err",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Count(text, "dynamic-array.append-prepared") != 2 || strings.Contains(text, "fail.bounds") {
		t.Fatalf("want one append per fill and no bounds panic:\n%s", text)
	}
	_, err = hostedModule(t, "module main\n\nfn F(n: int) Result[uint, CollectionError] {\n    let values: int[] := try fill(7, n)\n    return Ok(values.Len)\n}\n\nfn main() int {\n    return 0\n}\n", 18)
	if err == nil || !strings.Contains(err.Error(), "MD-155") {
		t.Fatalf("signed runtime count: %v", err)
	}
}
