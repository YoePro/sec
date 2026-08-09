package main

import (
	"strconv"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	secmlirlowering "sec/internal/lowering/secmlir"
	"sec/internal/parser"
	"sec/internal/sema"
)

func TestParseEmitSecMLIRCommandArgs(t *testing.T) {
	defaultTarget := CompilerTarget{OS: "linux", Arch: "amd64"}
	options, ok := parseEmitSecMLIRCommandArgs([]string{
		"sample.sec", "-o", "sample.mlir", "--target", "linux-amd64", "--mlir-bin", "/tmp/mlir/bin",
	}, defaultTarget)
	if !ok {
		t.Fatal("valid emit-sec-mlir arguments rejected")
	}
	if options.InputFile != "sample.sec" || options.OutputFile != "sample.mlir" || options.MLIRBin != "/tmp/mlir/bin" || options.Target != defaultTarget {
		t.Fatalf("options = %#v", options)
	}

	defaults, ok := parseEmitSecMLIRCommandArgs([]string{"sample.sec"}, defaultTarget)
	if !ok || defaults.OutputFile != "-" || defaults.Target != defaultTarget {
		t.Fatalf("default options = %#v, ok = %t", defaults, ok)
	}
}

func TestParseEmitSecMLIRCommandArgsRejectsMalformedInput(t *testing.T) {
	target := CompilerTarget{OS: "linux", Arch: "amd64"}
	for _, arguments := range [][]string{
		{},
		{"one.sec", "two.sec"},
		{"sample.sec", "-o"},
		{"sample.sec", "--mlir-bin"},
		{"sample.sec", "--target", "invalid"},
		{"sample.sec", "--unknown"},
		{"sample.sec", "-o", "one.mlir", "-o", "two.mlir"},
	} {
		if _, ok := parseEmitSecMLIRCommandArgs(arguments, target); ok {
			t.Fatalf("accepted malformed arguments %#v", arguments)
		}
	}
}

func TestPackage7SourceEmitsSchema4CheckedIntegerMLIR(t *testing.T) {
	source := `module main
fn Arithmetic(a: int, b: int, count: int) bool {
    let sum := a + b
    let negated := -sum
    let bits := (~negated & a) | (b ^ a)
    let shifted := bits << count
    return shifted >= b
}
fn Wide(a: int128, b: int128) int128 { return a * b }
fn Huge(a: uint256, b: uint256) uint256 { return a + b }
fn SignedAdd(a: int32, b: int32) int32 { return a + b }
fn UnsignedSubtract(a: uint32, b: uint32) uint32 { return a - b }
fn SignedDivide(a: int64, b: int64) int64 { return a / b }
fn UnsignedRemainder(a: uint64, b: uint64) uint64 { return a % b }
fn SignedShift(a: int256, count: int32) int256 { return a >> count }
fn UnsignedShift(a: uint128, count: uint16) uint128 { return a << count }
fn WideCompare(a: uint256, b: uint256) bool { return a < b }
fn First() int64 { return 1 }
fn Second() int64 { return 2 }
fn Third() int64 { return 3 }
fn NestedCalls() int64 { return First() + Second() * Third() }
fn CompareInIf(a: int32, b: int32) int32 {
    if a >= b { return a }
    return b
}
`
	p := parser.New(lexer.NewWithFile(source, "checked.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := sema.NewAnalyzer()
	if errors := a.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}
	module, err := semantic.Build(parsed.Program, a, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"checked.sec"}, MaxPackage: 7})
	if err != nil {
		t.Fatal(err)
	}
	for _, targetName := range []CompilerTarget{{OS: "linux", Arch: "armv7"}, {OS: "linux", Arch: "amd64"}} {
		t.Run(targetName.Arch, func(t *testing.T) {
			target, ok := findTargetDefinition(targetName)
			if !ok {
				t.Fatalf("%s target missing", targetName.String())
			}
			plan, err := target.scalarPlan()
			if err != nil {
				t.Fatal(err)
			}
			output, err := secmlirlowering.Emit(module, plan)
			if err != nil {
				t.Fatal(err)
			}
			text := string(output)
			for _, expected := range []string{
				"sec.dialect_version = 4 : i32", "sec.int.binary_checked",
				"sec.int.neg_checked", "sec.int.bit_not", "sec.int.bitwise",
				"sec.int.shift_checked", "sec.int.cmp", "sec.fail.arithmetic",
				`kind = "subtract"`, `kind = "divide"`, `kind = "remainder"`,
				`kind = "right_signed"`, `kind = "left_unsigned"`, `predicate = "lt"`,
				"si32", "ui32", "si64", "ui64", "si128", "si256", "ui128", "ui256",
				"sec.call.direct", "cf.cond_br",
				"#dlti.dl_entry<index, " + strconv.Itoa(int(plan.PointerWidthBits)) + ">",
			} {
				if !strings.Contains(text, expected) {
					t.Errorf("missing %q in:\n%s", expected, text)
				}
			}
		})
	}
}
