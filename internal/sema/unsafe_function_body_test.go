package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// rules/memory/unsafe.md §7(1–5): callable safety applies to calls, while
// every body operation still needs its own explicit lexical unsafe context.
func TestUnsafeFunctionBodyRemainsExplicit(t *testing.T) {
	for _, test := range []struct {
		file   string
		errors int
	}{
		{"raw_pointer_unsafe_body_invalid.sec", 1},
		{"unsafe_function_asm_invalid.sec", 1},
		{"unsafe_callable_body_invalid.sec", 4},
		{"unsafe_callable_body_valid.sec", 0},
	} {
		t.Run(test.file, func(t *testing.T) {
			path := "../../testdata/tooling/" + test.file
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(source), path))
			parsed := p.Parse()
			if parsed.HasErrors {
				t.Fatal(p.Errors())
			}
			errs := NewAnalyzer().Analyze(parsed.Program)
			if len(errs) != test.errors {
				t.Fatalf("errors = %+v, want %d", errs, test.errors)
			}
			for _, err := range errs {
				if !strings.Contains(err.Message, "requires unsafe") && !strings.Contains(err.Message, "only allowed inside unsafe") {
					t.Fatalf("unexpected diagnostic: %+v", err)
				}
			}
		})
	}
}

// The platform implementations must also meet unsafe.md §7(1–3) after
// removing implicit body permission; this checks body semantics under each
// scalar plan, independently of source selection and sibling-module loading.
func TestUnsafePlatformSyscallBodiesRemainValid(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64", "armv7"} {
		t.Run(arch, func(t *testing.T) {
			path := "../../sec/platform/linux/" + arch + "/raw_syscall.sec"
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(source), path))
			parsed := p.Parse()
			if parsed.HasErrors {
				t.Fatal(p.Errors())
			}
			width := uint16(64)
			if arch == "armv7" {
				width = 32
			}
			plan := layout.ResolvedScalarPlan{TargetOS: "linux", TargetArch: arch, PointerWidthBits: width, Endianness: layout.LittleEndian}
			if errs := NewAnalyzerWithScalarPlan(plan).Analyze(parsed.Program); len(errs) != 0 {
				t.Fatal(errs)
			}
		})
	}
}
