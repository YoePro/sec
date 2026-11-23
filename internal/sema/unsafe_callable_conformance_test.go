package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// An unsafe method cannot witness a safe requirement, directly, by inherited
// interface or through a generic owner. Otherwise constraint calls erase the
// caller obligations even though direct calls correctly require unsafe.
// Rules: rules/memory/unsafe.md — §§6(3),39–41;
// rules/declarations/interfaces.md — §§6,9,12.
func TestUnsafeInterfaceImplementationPreservesSafety(t *testing.T) {
	for _, name := range []string{"contracts_invalid", "attributed_contracts_invalid"} {
		source, err := os.ReadFile("../../testdata/sema/unsafe_callable_conformance/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			p := parser.New(lexer.NewWithFile(string(source), "contracts_invalid.sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			errors := a.Analyze(program)
			if len(errors) != 3 {
				t.Fatalf("depth %v: expected three unsafe conformance failures, got %v", depth, errors)
			}
			for _, err := range errors {
				if err.ID != diagnostics.InterfaceMemberIncompatible || !strings.Contains(err.Message, "unsafe caller obligations") {
					t.Fatal(err)
				}
			}
		}
	}
}

// Supported ordinary/generic/overloaded functions, impl methods, attributed
// methods and extern declarations keep caller safety at each invocation.
// Direct authorization never covers nested arguments/receivers or the body
// of an unsafe declaration; converting its name into a safe value is invalid.
// Rules: rules/memory/unsafe.md — §§4(9–13),6–7,10–11,39–41.
func TestUnsafeCallableSafetyMatrix(t *testing.T) {
	for _, test := range []struct {
		name                    string
		callErrors, valueErrors int
	}{
		{"valid", 0, 0}, {"calls_invalid", 11, 1},
	} {
		source, err := os.ReadFile("../../testdata/sema/unsafe_callable_conformance/" + test.name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			p := parser.New(lexer.NewWithFile(string(source), test.name+".sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			errors := a.Analyze(program)
			counts := map[string]int{}
			for _, err := range errors {
				counts[err.ID]++
			}
			if len(errors) != test.callErrors+test.valueErrors || counts[diagnostics.UnsafeCallRequiresContext] != test.callErrors || counts[diagnostics.UnsafeFunctionValueMismatch] != test.valueErrors {
				t.Fatalf("%s/%v: %v", test.name, depth, errors)
			}
		}
	}
}

// A safe implementation may satisfy a requirement with stronger caller-side
// obligations; the reverse conversion is forbidden even before dispatch exists.
// Rules: rules/memory/unsafe.md — §40(1–3).
func TestInterfaceMethodSafetyCompatibilityDirection(t *testing.T) {
	safe := Function{ReturnType: Type{Kind: VoidType}}
	unsafe := safe
	unsafe.Unsafe = true
	if !compatibleInterfaceMethodSignature(safe, unsafe) || !compatibleInterfaceMethodSignature(unsafe, unsafe) || compatibleInterfaceMethodSignature(unsafe, safe) {
		t.Fatal("interface safety compatibility is reversed or erased")
	}
}
