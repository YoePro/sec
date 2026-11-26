package sema

import (
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestPackage19NoArenaSurface (P19-30, P19 §§ 143-147, 161): Sec 0.1 has no
// individual Arena Free, no uninitialized safe allocation, no public
// capacity or usage member, no trimming, and neither Arena nor the internal
// allocation context is a source or C ABI value. Each absent surface is a
// Sema error, never an inferred operation. Non-trivial New/Alloc element
// types (§ 145) are TestArenaTypeRestrictionMatrix.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 143-147, 161;
// rules/memory/arena.md — §§ 26, 52.
func TestPackage19NoArenaSurface(t *testing.T) {
	body := func(code string) string {
		return "fn F() void {\n    let mut buffer: byte[64]\n    let mut arena := Arena.FromBuffer(ref mut buffer[..])\n" + code + "\n}"
	}
	for _, check := range []struct{ name, source, want string }{
		{"individual Free", body("    let p := try arena.New[int32]() { Err(_) => { return } }\n    arena.Free(p)"), "arena.Free"},
		{"uninitialized allocation", body("    discard arena.AllocUninit[int32](4u)"), "arena.AllocUninit"},
		{"public capacity", body("    discard arena.Capacity"), "unknown member Capacity on Arena"},
		{"public usage", body("    discard arena.Used()"), "arena.Used"},
		{"arbitrary trimming", body("    arena.Trim()"), "arena.Trim"},
		{"Arena over the C ABI", "extern \"C\" fn native(a: Arena) void", "cannot cross the C ABI boundary"},
		{"Arena reference over the C ABI", "extern \"C\" fn native(a: ref mut Arena) void", "cannot cross the C ABI boundary"},
		{"allocation context is not a source type", "fn G(c: AllocationContext) void {}", "unknown type AllocationContext"},
	} {
		t.Run(check.name, func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile("module main\n\n"+check.source+"\n", "p19.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := NewAnalyzer()
			analyzer.SetAllocationProfile("hosted")
			errors := analyzer.Analyze(parsed.Program)
			if len(errors) != 1 || !strings.Contains(errors[0].Message, check.want) {
				t.Fatalf("want one error containing %q, got %v", check.want, errors)
			}
		})
	}
}
