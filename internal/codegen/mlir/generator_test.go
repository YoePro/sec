package mlir

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
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

func TestGenerateLinkedExternDeclarationAndCall(t *testing.T) {
	input := `
module main

@link_name("c-add")
extern "C" fn add(left: int32, right: int32) int

fn main() int {
	unsafe {
		return add(1, 2)
	}
}
`

	program := parseTestProgram(t, input)
	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	for _, want := range []string{
		`llvm.func @"c-add"(i32, i32) -> i32`,
		`llvm.call @"c-add"(`,
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

func TestGenerateTopLevelIntegerConstant(t *testing.T) {
	input := `
module main

let STDOUT := 1i

fn descriptor() int {
    return STDOUT
}

fn main() int {
    return descriptor()
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	if !strings.Contains(got, "llvm.mlir.constant(1 : i32)") {
		t.Fatalf("generated MLIR is missing the top-level integer constant:\n%s", got)
	}
}

func TestGenerateLinuxAMD64SyscallInlineAsm(t *testing.T) {
	input := `
module main

unsafe extern "system" fn rawWrite(number: uint, fd: uint, ptr: uint, len: uint) int {
    asm {
        "syscall"
        inputs:
            rax(uint(number))
            rdi(fd)
            rsi(ptr)
            rdx(len)
        outputs:
            rax(result)
        clobbers:
            rcx
            r11
            memory
    }
    return result
}

unsafe fn write(fd: int, ref ptr: byte, len: int64) int {
    return rawWrite(1, uint(fd), uint(ptr), uint(len))
}

fn main() int {
    let text := "Hello"
    write(1, text.ptr, text.len)
    return 0
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}

	for _, want := range []string{
		`llvm.func @write(%fd: i32, %ptr: !llvm.ptr, %len: i64) -> i32`,
		`llvm.ptrtoint`,
		`llvm.inline_asm has_side_effects "syscall"`,
		`"={rax},{rax},{rdi},{rsi},{rdx},~{rcx},~{r11},~{memory}"`,
		` %number, %fd, %ptr, %len :`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `%number, %number`) {
		t.Fatalf("generated MLIR must not duplicate the syscall number operand:\n%s", got)
	}
}

func TestGenerateRawPointerParameterAndConversion(t *testing.T) {
	input := `
module main

unsafe fn address(ptr: RawPtr[byte]) uint {
    return uint(ptr)
}

fn main() int {
    return 0
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	for _, want := range []string{
		"llvm.func @address(%ptr: !llvm.ptr) -> i64",
		"llvm.ptrtoint %ptr : !llvm.ptr to i64",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated raw pointer MLIR missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateMaterializedStructAndMutableDefaults(t *testing.T) {
	input := `
module main

type State struct {
    count: int,
    ready: bool,
    name: string,
}

fn main() int {
    let mut state: State
    return state.count
}
`
	program := parseTestProgram(t, input)
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("semantic errors: %v", errors)
	}
	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	if count := strings.Count(got, "llvm.insertvalue"); count < 3 {
		t.Fatalf("defaulted struct fields were not fully initialized; insertvalue count=%d:\n%s", count, got)
	}
	if !strings.Contains(got, "llvm.mlir.constant(false) : i1") {
		t.Fatalf("generated MLIR is missing bool default:\n%s", got)
	}
	if strings.Contains(got, "llvm.call") || strings.Contains(got, "func.call") {
		t.Fatalf("default construction must not call a runtime function:\n%s", got)
	}
}

func TestGenerateConstrainedScalarDefaultsWithoutRuntime(t *testing.T) {
	input := `
module main

type Port int range 1..65535
type RetryCount int in [3, 5]
type ExitCode int default 9
type PositiveAmount decimal range 0.01..100.00

fn main() int {
    let mut port: Port
    let mut retry: RetryCount
    let mut code: ExitCode
    let mut amount: PositiveAmount
    return 0
}
`
	program := parseTestProgram(t, input)
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("semantic errors: %v", errors)
	}
	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	for _, want := range []string{
		"llvm.mlir.constant(1 : i32)",
		"llvm.mlir.constant(3 : i32)",
		"llvm.mlir.constant(9 : i32)",
		"llvm.mlir.constant(1 : i64)",
		"llvm.mlir.constant(2 : i32)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated constrained default MLIR missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "llvm.call") || strings.Contains(got, "func.call") {
		t.Fatalf("constrained defaults must not call a runtime function:\n%s", got)
	}
}

func TestGenerateUnaryPlus(t *testing.T) {
	program := parseTestProgram(t, `module main

fn main() int {
    return +9
}
`)
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("semantic errors: %v", errors)
	}
	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	if !strings.Contains(got, "llvm.mlir.constant(9 : i32)") {
		t.Fatalf("generated unary-plus MLIR is missing its operand:\n%s", got)
	}
}

func TestGenerateFunctionOverloadsByArity(t *testing.T) {
	input := `
module main

fn pick(value: int) int {
    return value
}

fn pick(left: int, right: int) int {
    return left + right
}

fn main() int {
    return pick(2, 3)
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}

	for _, want := range []string{
		`llvm.func @pick__sec_arity_1`,
		`llvm.func @pick__sec_arity_2`,
		`llvm.call @pick__sec_arity_2`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
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

func TestGenerateStructWithFixedArrayField(t *testing.T) {
	input := `
module main

type Buffer struct {
    bytes: byte[512],
    len: int,
}

fn main() int {
    let buffer := Buffer{ len: 0 }
    return buffer.len
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	if !strings.Contains(got, "!llvm.struct<(!llvm.array<512 x i8>, i32)>") {
		t.Fatalf("generated MLIR is missing the fixed array struct field:\n%s", got)
	}
}

func TestGenerateNestedStructFieldAssignment(t *testing.T) {
	input := `
module main

type Vector struct {
    x: int,
    y: int,
}

type Player struct {
    pos: Vector,
}

fn move(player: Player, dx: int) Player {
    let mut updated := player
    updated.pos.x = updated.pos.x + dx
    updated.pos.y += 2
    return updated
}

fn main() int {
    let player := Player{ pos: Vector{ x: 10, y: 20 } }
    let moved := move(player, 5)
    return moved.pos.x + moved.pos.y
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}

	for _, want := range []string{
		"llvm.extractvalue",
		"llvm.insertvalue",
		"llvm.store",
		"llvm.add",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated struct assignment MLIR missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateEnumValuesConversionsAndSwitch(t *testing.T) {
	input := `
module main

enum Status int {
    unknown = 0,
    active = 10,
    paused,
    disabled = 99,
}

enum Permission uint {
    none = 0,
    read = 1 << iota,
    write = 1 << iota,
    execute = 1 << iota,
}

enum Wide uint128 {
    huge = 18446744073709551616,
}

type Snapshot struct {
    status: Status,
}

type Vehicle struct {
    id: int,
}

impl Vehicle {
    enum FuelType {
        petrol,
        diesel,
        electric,
    }
}

fn statusCode(value: Status) int {
    switch value {
    case Status.paused:
        return 200
    default:
        return 0
    }
}

fn fuelCode(value: Vehicle.FuelType) int {
    switch value {
    case Vehicle.FuelType.diesel:
        return 20
    default:
        return 0
    }
}

fn wideValue(value: Wide) uint128 {
    return uint128(value)
}

fn main() int {
    let status: Status := Status(11)
    let snapshot := Snapshot{ status: status }
    let numeric: int := int(snapshot.status)
    let restored: Status := Status(numeric)
    let permission: uint := uint(Permission.execute)

    if permission != 8u {
        return 1
    }
    if wideValue(Wide.huge) != 18446744073709551616 {
        return 2
    }

    return statusCode(restored) + fuelCode(Vehicle.FuelType.diesel)
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}

	for _, want := range []string{
		"llvm.func @statusCode(%value: i32) -> i32",
		"llvm.func @fuelCode(%value: i32) -> i32",
		"llvm.func @wideValue(%value: i128) -> i128",
		"llvm.mlir.constant(11 : i32) : i32",
		"llvm.mlir.constant(8 : i64) : i64",
		"llvm.mlir.constant(18446744073709551616 : i128) : i128",
		"llvm.mlir.constant(1 : i32) : i32",
		"llvm.extractvalue",
		"llvm.icmp \"eq\"",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated enum MLIR missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateBitBackedEnums(t *testing.T) {
	input := `
module main

enum ClockSource: bit[2] {
    Internal = 0b00,
    External = 0b01,
    Bypass = 0b10,
}

enum BinaryState: bit {
    Off = 0,
    On = 1,
}

fn clockCode(value: ClockSource) int {
    switch value {
    case ClockSource.Bypass:
        return int(value)
    default:
        return 0
    }
}

fn main() int {
    return clockCode(ClockSource.Bypass) + int(BinaryState.On)
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}

	for _, want := range []string{
		"llvm.func @clockCode(%value: i2) -> i32",
		"llvm.mlir.constant(2 : i2) : i2",
		"llvm.mlir.constant(1 : i1) : i1",
		"llvm.zext",
		"i2 to i32",
		"i1 to i32",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated bit enum MLIR missing %q:\n%s", want, got)
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

func TestGenerateEnumMatchExpressionAndStatement(t *testing.T) {
	input := `
module main

enum Direction {
    North,
    South,
}

fn Record(value: int) void {
}

fn Score(direction: Direction) int {
    match direction {
        Direction.North => {
            Record(1)
        }
        _ => {
            Record(2)
        }
    }

    let code: int := match direction {
        Direction.North => 10
        other => 20
    }

    return match direction {
        Direction.North => code
        Direction.South => 30
    }
}

fn main() int {
    return Score(Direction.North)
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}

	for _, want := range []string{
		"^match_test",
		"^match_arm",
		"^match_end",
		`llvm.icmp "eq"`,
		"llvm.cond_br",
		"llvm.call @Record",
		"llvm.store",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "llvm.call @Record") != 2 {
		t.Fatalf("match statement should emit both arm bodies:\n%s", got)
	}
}

func TestGenerateMatchRejectsResultPatternsUntilResultABIExists(t *testing.T) {
	input := `
module main

fn Use(value: int) int {
    return match value {
        Ok(value) => value
        Err(error) => 0
    }
}

fn main() int {
    return Use(1)
}
`
	program := parseTestProgram(t, input)

	if _, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu"); err == nil || !strings.Contains(err.Error(), "Result match patterns") {
		t.Fatalf("GenerateWithTriple error = %v, want Result match unsupported", err)
	}
}

func TestGenerateDeferCleanupBeforeReturn(t *testing.T) {
	input := `
module main

fn Record(value: int) void {
}

fn main() int {
    let mut value := 1
    defer {
        Record(value)
    }
    value = 2
    return value
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}

	for _, want := range []string{
		"^defer_body",
		"^defer_next",
		"llvm.cond_br",
		"llvm.call @Record",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated MLIR missing %q:\n%s", want, got)
		}
	}

	callIndex := strings.Index(got, "llvm.call @Record")
	returnIndex := strings.LastIndex(got, "llvm.return")
	if callIndex < 0 || returnIndex < 0 || callIndex > returnIndex {
		t.Fatalf("defer cleanup must be emitted before function return:\n%s", got)
	}
}

func TestGenerateMultipleDefersUseLIFOOrder(t *testing.T) {
	input := `
module main

fn Record(value: int) void {
}

fn main() int {
    defer {
        Record(1)
    }
    defer {
        Record(2)
    }
    return 0
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}

	first := strings.Index(got, "llvm.mlir.constant(1 : i32)")
	second := strings.Index(got, "llvm.mlir.constant(2 : i32)")
	if first < 0 || second < 0 {
		t.Fatalf("generated MLIR missing defer payload constants:\n%s", got)
	}
	if second > first {
		t.Fatalf("defer cleanup should emit second defer before first defer:\n%s", got)
	}
}

func TestGenerateReturnBeforeLaterDeferSkipsLaterCleanup(t *testing.T) {
	input := `
module main

fn Record(value: int) void {
}

fn main() int {
    return 0

    defer {
        Record(1)
    }
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	if strings.Contains(got, "llvm.call @Record") {
		t.Fatalf("unreached later defer must not be emitted before earlier return:\n%s", got)
	}
}

func TestGenerateDeferInsideLoopRequiresGeneratedCleanupState(t *testing.T) {
	input := `
module main

fn Record(value: int) void {
}

fn main() int {
    for {
        defer {
            Record(1)
        }
        break
    }
    return 0
}
`
	program := parseTestProgram(t, input)

	if _, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu"); err == nil || !strings.Contains(err.Error(), "defer inside loops") {
		t.Fatalf("GenerateWithTriple error = %v, want defer inside loops unsupported", err)
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

func TestGenerateFixedArrays(t *testing.T) {
	input := `
module main

fn BuildPair(left: int, right: int) int[2] {
    return [left, right]
}

fn ReadAt(values: uint[3], index: uint) uint {
    return values[index]
}

fn Sum(values: int[3]) int {
    let mut total := 0
    for index, value in values {
        total += index + value
    }
    return total
}

fn main() int {
    let mut matrix: int[2][2] := [
        [1, 2],
        [3, 4],
    ]
    matrix[1][0] = 30
    let inferred := [5, 6]
    let pair := BuildPair(matrix[0][1], inferred[0])
    return pair[0] + pair[1] + int(matrix.len)
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	for _, want := range []string{
		"!llvm.array<2 x !llvm.array<2 x i32>>",
		"llvm.func @BuildPair(%left: i32, %right: i32) -> !llvm.array<2 x i32>",
		"llvm.func @ReadAt(%values: !llvm.array<3 x i64>, %index: i64) -> i64",
		"llvm.insertvalue",
		"llvm.getelementptr",
		`llvm.icmp "ult"`,
		"llvm.intr.trap",
		"for_array_condition",
		"for_array_next",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated array MLIR missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateNamedStringTypesAndStructFields(t *testing.T) {
	input := `
module main

type TokenType string

TokenType (
    IDENT := "IDENT",
)

type Token struct {
    kind: TokenType,
    lexeme: string,
}

fn Identity(value: string) string {
    let mut result := "x"
    result = value
    return result
}

fn Make() Token {
    let mut token := Token {
        kind: IDENT,
        lexeme: "x",
    }
    token.lexeme = Identity("name")
    return token
}

fn main() int {
    let token := Make()
    return int(token.kind.len + token.lexeme.len)
}
`
	program := parseTestProgram(t, input)

	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("GenerateWithTriple returned error: %v", err)
	}
	for _, want := range []string{
		"!llvm.struct<(!llvm.ptr, i64)>",
		"!llvm.struct<(!llvm.struct<(!llvm.ptr, i64)>, !llvm.struct<(!llvm.ptr, i64)>)>",
		"llvm.func @Identity(%value.ptr: !llvm.ptr, %value.len: i64) -> !llvm.struct<(!llvm.ptr, i64)>",
		"llvm.insertvalue",
		"llvm.extractvalue",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated named string MLIR missing %q:\n%s", want, got)
		}
	}
}
