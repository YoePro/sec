package mlir

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func parseTestProgram(t *testing.T, input string) *ast.Program {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	return program
}

func TestGenerateMinimalMain(t *testing.T) {
	input := `
module main

fn main() int {
    if true {
        return 0
    }

    return 1
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}

	for _, want := range []string{
		`module attributes {llvm.target_triple = "x86_64-pc-linux-gnu"}`,
		"llvm.func @main() -> i32",
		"llvm.cond_br",
		"llvm.return",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateLetBindings(t *testing.T) {
	input := `
module main

fn main() int {
    let a: int := 10
    let b: uint := 20u
    let f: float := 3.5
    let ok: bool := true
    let text := "hello"
    let c := a + 2

    if ok {
        return c
    }

    return 0
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}

	for _, want := range []string{
		"llvm.alloca",
		"llvm.store",
		"llvm.load",
		"llvm.add",
		"f64",
		"i64",
		"i1",
		"!llvm.ptr",
		"llvm.mlir.global",
		`@__sec_str_0("hello") : !llvm.array<5 x i8>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `\00`) {
		t.Fatalf("generated MLIR should not add implicit nul terminators:\n%s", got)
	}
}

func TestGenerateStructValues(t *testing.T) {
	input := `
module main

type Point struct {
    x: int,
    y: int,
    visible: bool,
}

type Segment struct {
    start: Point,
    end: Point,
}

fn makePoint(x: int, y: int) Point {
    return Point{ visible: true, y: y, x: x }
}

fn endpoint(segment: Segment) int {
    return segment.end.x
}

fn main() int {
    let first := makePoint(1, 2)
    let last := makePoint(42, 3)
    let segment := Segment{ end: last, start: first }
    return endpoint(segment)
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}

	for _, want := range []string{
		"llvm.func @makePoint(%x: i32, %y: i32) -> !llvm.struct<(i32, i32, i1)>",
		"llvm.func @endpoint(%segment: !llvm.struct<(!llvm.struct<(i32, i32, i1)>, !llvm.struct<(i32, i32, i1)>)>) -> i32",
		"llvm.insertvalue",
		"llvm.extractvalue",
		"[1] : !llvm.struct<(!llvm.struct<(i32, i32, i1)>, !llvm.struct<(i32, i32, i1)>)>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated struct MLIR missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateAssignments(t *testing.T) {
	input := `
module main

fn main() int {
    let mut value: int := 1
    value += 2
    value *= 4

    let mut ok: bool := false
    ok = true

    if ok {
        return value
    }

    return 0
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}

	for _, want := range []string{
		"llvm.store",
		"llvm.load",
		"llvm.add",
		"llvm.mul",
		"llvm.cond_br",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateIfElse(t *testing.T) {
	input := `
module main

fn main() int {
    let mut value := 0

    if false {
        value = 1
    } else {
        value = 7
    }

    return value
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}

	for _, want := range []string{
		"^if_then",
		"^if_else",
		"^if_end",
		"llvm.cond_br",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateReturningIfElse(t *testing.T) {
	input := `
module main

fn main() int {
    if false {
        return 1
    } else {
        return 9
    }
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	if strings.Contains(got, "^if_end") {
		t.Fatalf("returning if/else should not emit an unreachable end block:\n%s", got)
	}
}

func TestGenerateElseIf(t *testing.T) {
	input := `
module main

fn main() int {
    let value := 2

    if value == 1 {
        return 1
    } else if value == 2 {
        return 2
    } else {
        return 3
    }
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	if strings.Count(got, "llvm.cond_br") != 2 {
		t.Fatalf("else-if should emit two conditional branches:\n%s", got)
	}
}

func TestGenerateNegativeIntegerConstant(t *testing.T) {
	input := `
module main

fn main() int {
    return -1
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	if !strings.Contains(got, "llvm.mlir.constant(-1 : i32) : i32") {
		t.Fatalf("negative integer literal should lower directly:\n%s", got)
	}
	if strings.Contains(got, "llvm.sub") {
		t.Fatalf("negative integer literal should not lower through subtraction:\n%s", got)
	}
}

func TestInferWideIntegerLiteralType(t *testing.T) {
	input := `
module main

fn main() int {
    let mut max_val := 2147483647
    let mut i64_val := 2147483648

    if max_val == 0 {
        return -1
    }

    return 0
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	if !strings.Contains(got, "llvm.mlir.constant(2147483647 : i32) : i32") {
		t.Fatalf("i32 max literal should remain i32:\n%s", got)
	}
	if !strings.Contains(got, "llvm.mlir.constant(2147483648 : i64) : i64") {
		t.Fatalf("literal above i32 max should infer i64:\n%s", got)
	}
	if strings.Contains(got, "llvm.mlir.constant(2147483648 : i32) : i32") {
		t.Fatalf("literal above i32 max must not be emitted as i32:\n%s", got)
	}
}

func TestGenerateFunctionCall(t *testing.T) {
	input := `
module main

fn Add(a: int, b: int) int {
    return a + b
}

fn main() int {
    return Add(2, 3)
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	for _, want := range []string{
		"llvm.func @Add",
		"llvm.call @Add",
		") -> i32",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateShortCircuitLogicalExpressions(t *testing.T) {
	input := `
module main

fn Check(value: int) bool {
    return value > 0
}

fn main() int {
    let x := 5
    let y := 10

    if x > 2 && Check(y) {
        return 1
    }

    if x < 2 || Check(y) {
        return 2
    }

    return 3
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	for _, forbidden := range []string{"llvm.and", "llvm.or"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("logical operators should short-circuit, found %s:\n%s", forbidden, got)
		}
	}
	for _, want := range []string{
		"^logic_right",
		"^logic_end",
		"llvm.cond_br",
		"llvm.store",
		"llvm.load",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateShortCircuitInLet(t *testing.T) {
	input := `
module main

fn main() int {
    let a := false
    let b := true
    let ok := a && b

    if ok {
        return 1
    }

    return 2
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	if strings.Contains(got, "llvm.and") {
		t.Fatalf("let-bound && should short-circuit:\n%s", got)
	}
	if !strings.Contains(got, "^logic_end") {
		t.Fatalf("let-bound && should emit short-circuit control flow:\n%s", got)
	}
}

func TestGenerateRangeFor(t *testing.T) {
	input := `
module main

fn main() int {
    let mut sum := 0

    for i in 0..3 {
        sum += i
    }

    return sum
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	for _, want := range []string{
		"^for_condition",
		"^for_body",
		"^for_next",
		"^for_end",
		"llvm.select",
		"llvm.cond_br",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateLoopLocalsAllocaInFunctionPrologue(t *testing.T) {
	input := `
module main

fn main() int {
    let mut sum := 0

    for i in 0..<10 {
        let internal_val := 5
        sum = sum + i + internal_val
    }

    return sum
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	bodyIndex := strings.Index(got, "^for_body")
	if bodyIndex < 0 {
		t.Fatalf("generated MLIR missing for body:\n%s", got)
	}
	if strings.Contains(got[bodyIndex:], "llvm.alloca") {
		t.Fatalf("loop body must not contain alloca instructions:\n%s", got)
	}
	prologueEnd := strings.Index(got, "llvm.br ^for_condition")
	if prologueEnd < 0 {
		t.Fatalf("generated MLIR missing branch to loop condition:\n%s", got)
	}
	if count := strings.Count(got[:prologueEnd], "llvm.alloca"); count < 3 {
		t.Fatalf("expected local allocas in function prologue, got %d:\n%s", count, got)
	}
}

func TestGenerateDescendingRangeForWithNegativeStep(t *testing.T) {
	input := `
module main

fn main() int {
    let mut countdown := 0

    for i in 3..-1 step -1 {
        countdown += i
    }

    return countdown
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	for _, want := range []string{
		"llvm.mlir.constant(-1 : i32) : i32",
		"llvm.add",
		"^for_condition",
		"^for_next",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateFloatRangeForAndLiteralComparisonContext(t *testing.T) {
	input := `
module main

fn main() int {
    let mut x := 10i
    let mut y := 20u

    if (x != 5 && y > 15) || x == 0 {
        x = 100
    } else {
        x = 200
    }

    let mut float_sum := 0.0f
    for f in 0.0f..5.0f step 0.5f {
        float_sum = float_sum + f
    }

    return x
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	for _, want := range []string{
		"llvm.mlir.constant(15 : i64) : i64",
		`llvm.icmp "ugt"`,
		"llvm.fcmp",
		"llvm.fadd",
		"^for_condition",
		"^logic_right",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "llvm.mlir.constant(15 : i32) : i32") {
		t.Fatalf("literal in uint comparison should be emitted with comparison context:\n%s", got)
	}
	if strings.Contains(got, "llvm.mlir.constant(true) : i1") {
		t.Fatalf("short-circuit || should preserve the left value instead of initializing result to constant true:\n%s", got)
	}
}

func TestGenerateRangeForBreakContinue(t *testing.T) {
	input := `
module main

fn main() int {
    let mut sum := 0

    for i in 0..10 {
        if i == 2 {
            continue
        }
        if i == 4 {
            break
        }
        sum += i
    }

    return sum
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	if strings.Count(got, "llvm.br ^for_next") < 1 {
		t.Fatalf("continue should branch to for_next:\n%s", got)
	}
	if strings.Count(got, "llvm.br ^for_end") < 1 {
		t.Fatalf("break should branch to for_end:\n%s", got)
	}
}

func TestGenerateInfiniteFor(t *testing.T) {
	input := `
module main

fn main() int {
    let mut value := 0

    for {
        value = 9
        break
    }

    return value
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	for _, want := range []string{
		"^for_body",
		"^for_end",
		"llvm.br ^for_body",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateWhileBreakContinue(t *testing.T) {
	input := `
module main

fn main() int {
    let mut value := 0
    let mut total := 0

    while value < 10 {
        value += 1
        if value == 5 {
            continue
        }
        if value == 8 {
            break
        }
        total += value
    }

    return total
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	for _, want := range []string{
		"^while_condition",
		"^while_body",
		"^while_end",
		"llvm.cond_br",
		"llvm.br ^while_condition",
		"llvm.br ^while_end",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateWhileWithRangeInCondition(t *testing.T) {
	input := `
module main

fn main() int {
    let mut value := -10

    while !(value in 0..100) {
        value = value + 1
    }

    return value
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	for _, want := range []string{
		`llvm.icmp "sge"`,
		`llvm.icmp "sle"`,
		"llvm.and",
		"llvm.xor",
		"^while_condition",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateDecimalStorageLiteralsParametersAndReturns(t *testing.T) {
	input := `
module main

fn EchoDecimal(value: decimal) decimal {
    return value
}

fn EchoDecimal128(value: decimal128) decimal128 {
    return value
}

fn main() int {
    let ordinary: decimal := 123.45
    let exact: decimal128 := 123456789012345678901234.5678
    let copied := EchoDecimal(ordinary)
    let copied128 := EchoDecimal128(exact)
    return int(copied)
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}

	for _, want := range []string{
		"llvm.func @EchoDecimal(%value: !llvm.struct<(i64, i32)>) -> !llvm.struct<(i64, i32)>",
		"llvm.func @EchoDecimal128(%value: !llvm.struct<(i128, i32)>) -> !llvm.struct<(i128, i32)>",
		"llvm.mlir.constant(12345 : i64) : i64",
		"llvm.mlir.constant(2 : i32) : i32",
		"llvm.mlir.constant(1234567890123456789012345678 : i128) : i128",
		"llvm.mlir.constant(4 : i32) : i32",
		"llvm.insertvalue",
		"llvm.extractvalue",
		"^decimal_cast_condition",
		"llvm.sdiv",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateInlineIntegerToDecimalConversions(t *testing.T) {
	input := `
module main

fn main() int {
    let signed: int128 := 170141183460469231731687303715884105727
    let exact: decimal128 := decimal128(signed)
    let ordinary: decimal := decimal(42)
    let narrowed: int64 := int64(exact)
    return int(ordinary) + int(narrowed)
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}

	for _, want := range []string{
		"llvm.mlir.constant(170141183460469231731687303715884105727 : i128) : i128",
		"llvm.insertvalue",
		"llvm.extractvalue",
		"llvm.mlir.constant(10 : i128) : i128",
		"llvm.sdiv",
		"^decimal_cast_body",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "_sec_decimal") {
		t.Fatalf("decimal conversion must be emitted inline without runtime calls:\n%s", got)
	}
}

func TestGenerateWideIntegerOperations(t *testing.T) {
	input := `
module main

fn WideSigned(left: int128, right: int128) int128 {
    return ~(left & right) | (left ^ right)
}

fn WideUnsigned(left: uint256, right: uint256) uint256 {
    let shifted := left >> right
    return (shifted + right) / right
}

fn main() int {
    let signed: int128 := 170141183460469231731687303715884105727
    let unsigned: uint256 := 115792089237316195423570985008687907853269984665640564039457584007913129639935
    let signed_result := WideSigned(signed, int128(1))
    let unsigned_result := WideUnsigned(unsigned, uint256(1))
    return 0
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}

	for _, want := range []string{
		"llvm.func @WideSigned(%left: i128, %right: i128) -> i128",
		"llvm.func @WideUnsigned(%left: i256, %right: i256) -> i256",
		"llvm.and",
		"llvm.xor",
		"llvm.or",
		"llvm.lshr",
		"llvm.udiv",
		"llvm.mlir.constant(115792089237316195423570985008687907853269984665640564039457584007913129639935 : i256) : i256",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateIntegerAndBoolSwitch(t *testing.T) {
	input := `
module main

fn Classify(value: int) int {
    let mut result := 0
    switch value {
    case 1, 3:
        result = 10
    case 2:
        result = 20
    default:
        result = 30
    }
    return result
}

fn BoolValue(value: bool) int {
    switch value {
    case true:
        return 1
    case false:
        return 0
    }
}

fn main() int {
    return Classify(3) + BoolValue(false)
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}

	for _, want := range []string{
		"^switch_test",
		"^switch_case",
		"^switch_default",
		"^switch_end",
		`llvm.icmp "eq"`,
		"llvm.or",
		"llvm.cond_br",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "llvm.call @Classify") != 1 {
		t.Fatalf("switch integration call missing:\n%s", got)
	}
}

func TestGenerateSwitchRejectsDeferredForms(t *testing.T) {
	tests := map[string]string{
		"subjectless": `
module main
fn main() int {
    switch {
    case true:
        return 1
    default:
        return 0
    }
}
`,
		"range": `
module main
fn main() int {
    switch 5 {
    case 0..10:
        return 1
    default:
        return 0
    }
}
`,
		"fallthrough": `
module main
fn main() int {
    switch 1 {
    case 1:
        fallthrough
    case 2:
        return 2
    default:
        return 0
    }
}
`,
	}

	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			program := parseTestProgram(t, input)
			if _, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu"); err == nil {
				t.Fatal("GenerateWithTriple unexpectedly accepted deferred switch form")
			}
		})
	}
}
