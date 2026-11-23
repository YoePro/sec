package semantic

import (
	"strings"
	"testing"
)

// core-library.md § 5.3, package18 §§ 33, 72-73: ToRuneArray borrows one
// shared view of the receiver, appends each decoded rune to a temporary
// owner through the P18 growth protocol, moves the owner into Ok when
// decoding ends, and on a failed growth ends the view, destroys the
// initialized prefix (partial construction) and yields
// Err(CollectionError.AllocationFailed). No panic endpoint exists.
func TestRuneArrayMaterializationSemanticIR(t *testing.T) {
	module, err := hostedModule(t, `module main

fn Runes() Result[uint, CollectionError] {
    let text := "héj"
    let runes := try text.ToRuneArray()
    return Ok(runes.Len)
}

fn main() int {
    return 0
}
`, 18)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		"$1 ToRuneArray.result: !6 mutable=true", "string.rune-len", "string.view-borrow",
		"dynamic-array.empty", "string-view.decode-next", "dynamic-array.grow-backing", "transition=establish-domain",
		"dynamic-array.append-prepared", "[loop=@1 edge=back]", "own.move-from-place", "result.ok %23",
		`own.destroy-place %25 plan="destroy:array:dynamic::rune:rune" cause=temporary-end`, "enum.constant case=#0",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Count(text, "ref.end-borrow %5") != 2 || strings.Contains(text, "fail.bounds") || strings.Contains(text, "panic") {
		t.Fatalf("the view must end on both exits and no panic path may exist:\n%s", text)
	}
	if _, err := hostedModule(t, "module main\n\nfn Runes() Result[uint, CollectionError] {\n    let text := \"a\"\n    let runes := try text.ToRuneArray()\n    return Ok(runes.Len)\n}\n\nfn main() int {\n    return 0\n}\n", 17); err == nil {
		t.Fatal("package 17 accepted a dynamic-array materialization")
	}
	if _, err := hostedModule(t, "module main\n\nfn Bytes() Result[uint, CollectionError] {\n    let text := \"a\"\n    let bytes := try text.ToByteArray()\n    return Ok(bytes.Len)\n}\n\nfn main() int {\n    return 0\n}\n", 18); err == nil || !strings.Contains(err.Error(), "string.ToByteArray materialization") {
		t.Fatalf("ToByteArray must stay a named boundary: %v", err)
	}
}
