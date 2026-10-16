package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// MD-059 (2026-10-16): Sec 0.1 has no unsafe function type. An `unsafe fn`
// type is rejected in every type position with a diagnostic that explains
// the rule; an unsafe function used as a function value is S1141, also
// inside `unsafe`; unsafe declarations and their direct calls inside an
// unsafe block keep working.
//
// Rules:
//   - rules/foundations/grammar.md — FunctionType note
//   - rules/memory/unsafe.md — §§ 39(5), 39(6)
//   - rules/corrections/applied/md059-ffi-unsafe-callables-correction-20261016.md — §§ 2, 8, 9(1)
func TestMD059NoUnsafeFunctionType(t *testing.T) {
	for name, source := range map[string]string{
		"let":       "module main\nfn Use() void {\n    let operation: unsafe fn() void\n}\n",
		"parameter": "module main\nfn Use(operation: unsafe fn() void) void {}\n",
		"return":    "module main\nfn Make() unsafe fn() void {\n    return Make\n}\n",
		"field":     "module main\ntype Holder struct {\n    operation: unsafe fn() void,\n}\n",
	} {
		t.Run(name, func(t *testing.T) {
			p := parser.New(lexer.New(source))
			p.ParseProgram()
			errors := p.Errors()
			if len(errors) == 0 || !strings.Contains(errors[0], "Sec 0.1 has no unsafe function type") {
				t.Fatalf("want the MD-059 unsafe function type diagnostic first, got %v", errors)
			}
		})
	}
	for name, test := range map[string]struct {
		body string
		want string
	}{
		"unsafe to safe value":         {"let f: fn() void := Danger", diagnostics.UnsafeFunctionValueMismatch},
		"unsafe to safe inside unsafe": {"unsafe {\n        let f: fn() void := Danger\n    }", diagnostics.UnsafeFunctionValueMismatch},
		"direct call inside unsafe":    {"unsafe {\n        Danger()\n    }", ""},
	} {
		t.Run(name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, "\nmodule main\n\nunsafe fn Danger() void {}\n\nfn Use() void {\n    "+test.body+"\n}\n")
			if test.want == "" {
				if len(errors) != 0 {
					t.Fatalf("unexpected errors: %v", errors)
				}
				return
			}
			if len(errors) != 1 || errors[0].ID != test.want || !strings.Contains(errors[0].Help, "c::fn") {
				t.Fatalf("want one %s explaining the foreign c::fn distinction, got %v", test.want, errors)
			}
		})
	}
}
