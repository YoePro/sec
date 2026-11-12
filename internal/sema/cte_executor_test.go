package sema

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Required contract and default positions execute ordinary user functions at
// compile time: nested calls, local mutation, while and for loops, recursion
// and path-sensitive legality establish the values the contracts then
// enforce; no const-fn modifier exists.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 2, 3, 6, 8(5)–(7), 19, 20, 40, 48.
func TestSemanticCTEExecutesOrdinaryFunctions(t *testing.T) {
	source := `module main

extern "C" fn ReadSensor() int

fn Limit() int {
    return 10 * 10
}

fn Square(x: int) int {
    return x * x
}

fn Bits(n: int) int {
    let mut value := 1
    let mut i := 0
    while i < n {
        value = value * 2
        i = i + 1
    }
    return value
}

fn SumTo(n: int) int {
    let mut total := 0
    for i in 1..n {
        total += i
    }
    return total
}

fn Fact(n: int) int {
    if n <= 1 {
        return 1
    }
    return n * Fact(n - 1)
}

fn Pick(useDefault: bool) int {
    if useDefault {
        return 10
    }
    return ReadSensor()
}

type Percent int range 0..Limit()
type Small int range 0..Square(4) default Square(2)
type Wide int range 0..Bits(10) + SumTo(4)
type Factorial int range 0..Fact(10)
type Picked int range 0..Pick(true)

fn Main() int {
    let ok: Percent := 100
    let tooBig: Percent := 101
    let wide: Wide := 1035
    let fact: Factorial := 3628801
    let picked: Picked := 11
    let mut small: Small
    return 0
}
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, source)
	want := []string{"value 101 violates range contract Percent 0..100", "value 1035 violates range contract Wide 0..1034",
		"value 3628801 violates range contract Factorial 0..3628800", "value 11 violates range contract Picked 0..10"}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v", errors)
	}
	for index, message := range want {
		if errors[index].ID != diagnostics.ValueViolatesContract || !strings.Contains(errors[index].Message, message) {
			t.Errorf("error %d = %s %q, want %q", index, errors[index].ID, errors[index].Message, message)
		}
	}
	small := analyzer.types["Small"]
	if small.ExplicitDefault == nil || small.ExplicitDefault.Integer == nil || small.ExplicitDefault.Integer.Int64() != 4 || small.InvalidExplicitDefault {
		t.Errorf("Small default = %+v", small.ExplicitDefault)
	}
}

// Required evaluation that executes but establishes no value is diagnosed by
// category with the root expression, the reason, the executed call chain and
// the location where evaluation stopped.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 19, 21, 22, 47–49, 54, 72.
func TestSemanticCTEFailureCategories(t *testing.T) {
	source := `module main

extern "C" fn ReadSensor() int

let mut Counter: int := 3

fn Checked(n: int) int {
    assert n > 0, "n must be positive"
    return n
}

fn Divide(n: int) int {
    return 100 / n
}

fn Grow(n: int8) int8 {
    return n * n
}

fn Pick(useDefault: bool) int {
    if useDefault {
        return 10
    }
    return ReadSensor()
}

fn Spin() int {
    let mut i := 0
    while i >= 0 {
        i = 1
    }
    return i
}

fn Deep(n: int) int {
    return Deep(n + 1)
}

fn ReadCounter() int {
    return Counter
}

type A int range 0..Checked(0)
type B int range 0..Divide(0)
type C int8 range 0..Grow(20)
type D int range 0..Pick(false)
type F int range 0..Spin()
type G int range 0..Deep(0)
type H int range 0..ReadCounter()
`
	errors := analyzeSourceRaw(t, source)
	want := []struct {
		id, text  string
		stoppedAt int
	}{
		{diagnostics.CompileTimeEvaluationFailed, "assertion (n > 0) failed: n must be positive (in Checked)", 8},
		{diagnostics.CompileTimeEvaluationFailed, "division by zero (in Divide)", 13},
		{diagnostics.CompileTimeEvaluationFailed, "arithmetic overflow: 400 does not fit int8 (in Grow)", 17},
		{diagnostics.CompileTimeForbiddenEffect, "ReadSensor is a foreign call", 24},
		{diagnostics.CompileTimeResourceLimit, "ExecutionSteps budget", 0},
		{diagnostics.CompileTimeResourceLimit, "CallDepth budget of 256 nested calls was reached (in Deep ×255)", 0},
		{diagnostics.CompileTimeEvaluationFailed, "Counter is a mutable static; semantic CTE never reads mutable static state", 40},
	}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v", errors)
	}
	for index, expected := range want {
		err := errors[index]
		if err.ID != expected.id || !strings.Contains(err.Message, expected.text) || err.Help == "" {
			t.Errorf("error %d = %s %q, want %s containing %q", index, err.ID, err.Message, expected.id, expected.text)
		}
		if expected.stoppedAt != 0 && err.PreviousLine != expected.stoppedAt {
			t.Errorf("error %d stopped at line %d, want %d", index, err.PreviousLine, expected.stoppedAt)
		}
	}
}

// Switch with explicit fallthrough, mutual recursion and type-associated
// static functions execute during semantic CTE.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 20, 40, 48(1);
// rules/control-flow/flowcontrol_switch.md — §§ 15, 16.
func TestSemanticCTEControlFlowAndStaticCalls(t *testing.T) {
	source := `module main

fn Weight(code: int) int {
    let mut total := 0
    switch code {
    case 1:
        total = 10
    case 2:
        total = 20
        fallthrough
    case 3:
        total += 5
    default:
        total = 99
    }
    return total
}

fn IsEven(n: int) bool {
    if n == 0 {
        return true
    }
    return IsOdd(n - 1)
}

fn IsOdd(n: int) bool {
    if n == 0 {
        return false
    }
    return IsEven(n - 1)
}

fn EvenLimit(n: int) int {
    if IsEven(n) {
        return n
    }
    return 0
}

type Sizes struct {}

impl Sizes {
    static fn Twice(value: int) int {
        return value * 2
    }
}

type One int range 0..Weight(1)
type Two int range 0..Weight(2)
type Other int range 0..Weight(7)
type Even int range 0..EvenLimit(12)
type Doubled int range 0..Sizes.Twice(21)

fn Main() int {
    let a: One := 11
    let b: Two := 26
    let c: Other := 100
    let d: Even := 13
    let e: Doubled := 43
    return 0
}
`
	errors := analyzeSourceRaw(t, source)
	want := []string{"One 0..10", "Two 0..25", "Other 0..99", "Even 0..12", "Doubled 0..42"}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v", errors)
	}
	for index, contract := range want {
		if errors[index].ID != diagnostics.ValueViolatesContract || !strings.Contains(errors[index].Message, contract) {
			t.Errorf("error %d = %q, want contract %s", index, errors[index].Message, contract)
		}
	}
}

// The compile-time evaluation probe analyzes the same AST before the real
// analysis; with legacy AST default materialization off it must leave the AST
// unchanged, so the real analysis of every valid fixture is identical.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 50, 61 (determinism).
func TestCompileTimeProbeLeavesASTUnchanged(t *testing.T) {
	files, _ := filepath.Glob("../../testdata/*valid*.sec")
	nested, _ := filepath.Glob("../../testdata/*/*valid*.sec")
	files = append(files, nested...)
	if len(files) == 0 {
		t.Fatal("no fixtures found")
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		analyze := func(probeFirst bool) []string {
			p := parser.New(lexer.New(string(data)))
			program := p.ParseProgram()
			if len(p.Errors()) > 0 {
				return nil
			}
			if probeFirst {
				probe := NewAnalyzer()
				probe.cteProbe = true
				probe.legacyDefaultAST = false
				probe.Analyze(program)
			}
			messages := []string{}
			for _, e := range NewAnalyzer().Analyze(program) {
				messages = append(messages, e.Error())
			}
			return messages
		}
		if plain, probed := analyze(false), analyze(true); strings.Join(plain, "\n") != strings.Join(probed, "\n") {
			t.Errorf("%s: probe changed the analysis:\n%v\nvs\n%v", file, plain, probed)
		}
	}
}

// Static initializers execute ordinary Sec functions at compile time and
// materialize the established scalar value, including chains of statics, a
// function-local static and a contract reading such a static; an executed
// read of a mutable static, an executed foreign call and a value that depends
// on itself are fail-closed diagnostics, never hidden runtime initializers.
// Rules: rules/declarations/static.md — §§ 15, 16; rules/compiler/compile_time_evaluation.md — §§ 9, 21, 39, 65.
func TestSemanticCTEStaticInitializers(t *testing.T) {
	valid := `module main

fn BufferSize() uint {
    let mut size: uint := 1
    for i in 0..11 {
        size = size * 2
    }
    return size
}

fn Half(n: uint) uint {
    return n / 2
}

fn Twice(n: int) int {
    return n * 2
}

let Size := BufferSize()
let HalfSize: uint := Half(Size)
let mut Retries: int := Twice(3)
let Ready: bool := Twice(2) == 4

type Bounded uint range 0..HalfSize

fn Main() int {
    static let local: int := Twice(21)
    let fits: Bounded := 2048
    return local
}
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, valid)
	if len(errors) != 0 {
		t.Fatalf("valid static CTE: %v", errors)
	}
	got := map[string]string{}
	for _, static := range analyzer.ResolvedStaticDeclarations() {
		switch {
		case static.Initializer.Integer != nil:
			got[static.Name] = static.Initializer.Integer.String()
		case static.Initializer.Bool != nil:
			got[static.Name] = map[bool]string{true: "true", false: "false"}[*static.Initializer.Bool]
		}
	}
	for name, want := range map[string]string{"Size": "4096", "HalfSize": "2048", "Retries": "6", "Ready": "true", "local": "42"} {
		if got[name] != want {
			t.Errorf("%s initializer = %q, want %q (all %v)", name, got[name], want, got)
		}
	}

	invalid := `module main

extern "C" fn ReadConfiguration() int

fn Load() int {
    return ReadConfiguration()
}

fn Twice(n: int) int {
    return n * 2
}

let mut Retries: int := Twice(3)

fn ReadRetries() int {
    return Retries
}

fn LoopA() int {
    return CycleA + 1
}

let Configuration: int := Load()
let FromMutable: int := ReadRetries()
let CycleA: int := LoopA()
let OutOfRange: uint8 := Twice(200)
`
	errors = analyzeSourceRaw(t, invalid)
	want := []struct{ id, text string }{
		{diagnostics.CompileTimeForbiddenEffect, "static initializer for Configuration Load() performs an effect forbidden"},
		{diagnostics.CompileTimeEvaluationFailed, "Retries is a mutable static"},
		{diagnostics.CompileTimeDependencyCycle, "the value of CycleA depends on itself: CycleA -> CycleA (in LoopA)"},
	}
	for _, expected := range want {
		found := false
		for _, err := range errors {
			found = found || err.ID == expected.id && strings.Contains(err.Message, expected.text)
		}
		if !found {
			t.Errorf("missing %s %q in %v", expected.id, expected.text, errors)
		}
	}
}

// Static property getters execute during semantic CTE in contracts, defaults
// and static initializers; an instance getter needs a receiver value and is
// still an implementation gap (S1101).
// Rules: rules/compiler/compile_time_evaluation.md — §§ 6(2), 8(6); rules/declarations/properties.md — § 4.
func TestSemanticCTEStaticPropertyGetters(t *testing.T) {
	source := `module main

type Limits struct {
    count: int,
}

impl Limits {
    static property MaxPort: int {
        get {
            let base := 0xFF
            return base * 0x100 + 0xFF
        }
    }
    property Doubled: int {
        get {
            return self.count * 2
        }
    }
}

let Port := Limits.MaxPort

type PortNumber int range 1..Limits.MaxPort default Limits.MaxPort

fn Main() int {
    let p: PortNumber := 65536
    return 0
}
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, source)
	if len(errors) != 1 || errors[0].ID != diagnostics.ValueViolatesContract || !strings.Contains(errors[0].Message, "PortNumber 1..65535") {
		t.Fatalf("errors = %v", errors)
	}
	if typ := analyzer.types["PortNumber"]; typ.ExplicitDefault == nil || typ.ExplicitDefault.Integer.Int64() != 65535 {
		t.Errorf("PortNumber default = %+v", typ.ExplicitDefault)
	}
}

// Struct values execute during semantic CTE: literals with type defaults for
// omitted fields, field reads and writes, instance methods that mutate their
// receiver, instance getters and value semantics of copies.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 2(2), 20, 26, 32.
func TestSemanticCTEStructValues(t *testing.T) {
	source := `module main

type Height int default 3

type Config struct {
    width: int,
    height: Height,
    depth: int,
}

impl Config {
    fn Area() int {
        return width * int(height)
    }
    fn Grow(by: int) void {
        self.width = self.width + by
        height = Height(int(height) + by)
    }
    property Volume: int {
        get {
            return self.Area() * depth
        }
    }
}

fn Make(w: int) Config {
    return Config { width: w, depth: 2 }
}

fn Compute() int {
    let mut c := Make(4)
    let copy := c
    c.Grow(1)
    c.depth = 10
    return c.Volume + copy.Area()
}

type Limit int range 0..Compute()
type W int range 0..Make(7).width

fn Main() int {
    let x: Limit := 213
    let y: W := 8
    return 0
}
`
	errors := analyzeSourceRaw(t, source)
	want := []string{"Limit 0..212", "W 0..7"}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v", errors)
	}
	for index, contract := range want {
		if errors[index].ID != diagnostics.ValueViolatesContract || !strings.Contains(errors[index].Message, contract) {
			t.Errorf("error %d = %q, want contract %s", index, errors[index].Message, contract)
		}
	}
}

// Result and Option are ordinary data during semantic CTE: Ok/Err/Some/None
// construction, try unwrapping and propagating Err out of the function, match
// with payload bindings and guards in statement and expression position, and
// `if value is Some(binding)`.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 20, 46.
func TestSemanticCTEResultOptionTryAndMatch(t *testing.T) {
	source := `module main

type ParseError enum error {
    Empty,
    TooBig,
}

type Level enum {
    Low,
    High,
}

fn Parse(n: int) Result[int, ParseError] {
    if n < 0 {
        return Err(ParseError.Empty)
    }
    if n > 1000 {
        return Err(ParseError.TooBig)
    }
    return Ok(n * 2)
}

fn Twice(n: int) Result[int, ParseError] {
    let first := try Parse(n)
    return Ok(first * 2)
}

fn Find(n: int) Option[int] {
    if n == 0 {
        return None
    }
    return Some(n + 1)
}

fn Score(n: int) int {
    match Twice(n) {
        Ok(value) where value > 100 => return value
        Ok(value) => return value + 1000
        Err(error) => return 7
    }
}

fn Lookup(n: int) int {
    if Find(n) is Some(found) {
        return found
    }
    return 0
}

fn Weight(level: int) int {
    let found := Find(level)
    return match found {
        Some(value) => value * 10
        None => 0
    }
}

type A int range 0..Score(30)
type B int range 0..Score(2000)
type C int range 0..Lookup(41)
type D int range 0..Weight(1)

fn Main() int {
    let a: A := 121
    let b: B := 8
    let c: C := 43
    let d: D := 11
    return 0
}
`
	errors := analyzeSourceRaw(t, source)
	want := []string{"A 0..120", "B 0..7", "C 0..42"}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v", errors)
	}
	for index, contract := range want {
		if errors[index].ID != diagnostics.ValueViolatesContract || !strings.Contains(errors[index].Message, contract) {
			t.Errorf("error %d = %q, want contract %s", index, errors[index].Message, contract)
		}
	}
}

// Language-defined operation failures are SemanticOperationFailed, distinct
// from a reached assert or panic, and still the registered S1223.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 47(4)–(6), 54.
func TestSemanticCTESemanticOperationFailed(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

fn Divide(n: int) int {
    return 100 / n
}

fn Check(n: int) int {
    assert n > 0
    return n
}

type A int range 0..Divide(0)
type B int range 0..Check(0)
`)
	if len(errors) != 2 || errors[0].ID != diagnostics.CompileTimeEvaluationFailed || errors[1].ID != diagnostics.CompileTimeEvaluationFailed {
		t.Fatalf("errors = %v", errors)
	}
	if !strings.Contains(errors[0].Message, "failed a semantic operation during compile-time evaluation: division by zero") || !strings.Contains(errors[0].Help, "SemanticOperationFailed") {
		t.Errorf("division = %q / %q", errors[0].Message, errors[0].Help)
	}
	if !strings.Contains(errors[1].Message, "failed during compile-time evaluation: assertion") {
		t.Errorf("assert = %q", errors[1].Message)
	}
}

// AggregateValueSize and TransientMemory are enforced ResourceLimit budgets,
// and receiver-free calls are memoized within one analysis so a naive
// exponential recursion that would exhaust ExecutionSteps completes.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 48(4), 49, 60.
func TestSemanticCTEBudgetsAndMemoization(t *testing.T) {
	// AggregateValueSize: one value larger than the budget fails even when
	// TransientMemory still has room.
	large := &CompileTimeAggregate{Fields: map[string]DefaultConstant{}}
	for field := 0; field <= cteMaxAggregateValueSize; field++ {
		name := fmt.Sprintf("f%d", field)
		large.Names = append(large.Names, name)
		large.Fields[name] = DefaultConstant{Kind: IntType}
	}
	executor := newCTEExecutor(&Analyzer{}, nil)
	if executor.chargeAggregate(DefaultConstant{Kind: StructType, Aggregate: large}, lexer.Token{Line: 1}) || executor.failure == nil ||
		executor.failure.Kind != cteResourceLimit || !strings.Contains(executor.failure.Reason, "AggregateValueSize") {
		t.Fatalf("aggregate budget failure = %+v", executor.failure)
	}

	var wide strings.Builder
	wide.WriteString("module main\n\ntype Wide struct {\n")
	for field := 0; field < 40; field++ {
		fmt.Fprintf(&wide, "    f%d: int,\n", field)
	}
	wide.WriteString("}\n\nfn Churn() int {\n    let mut value := Wide {}\n    let mut i := 0\n    while i < 1000000 {\n        value.f0 = i\n        i = i + 1\n    }\n    return value.f0\n}\n\ntype C int range 0..Churn()\n")
	errors := analyzeSourceRaw(t, wide.String())
	if len(errors) != 1 || errors[0].ID != diagnostics.CompileTimeResourceLimit || !strings.Contains(errors[0].Message, "TransientMemory") {
		t.Fatalf("transient errors = %v", errors)
	}

	errors = analyzeSourceRaw(t, `module main

fn Fib(n: int) int {
    if n < 2 {
        return n
    }
    return Fib(n - 1) + Fib(n - 2)
}

type F int range 0..Fib(30)

fn Main() int {
    let f: F := 832040
    let g: F := 832041 + 1
    return 0
}
`)
	if len(errors) != 1 || errors[0].ID != diagnostics.ValueViolatesContract || !strings.Contains(errors[0].Message, "F 0..832040") {
		t.Fatalf("memoized Fib errors = %v", errors)
	}
}

// A fault of the evaluator itself is a CompilerInvariantFailure outcome, not
// a source failure category (§ 57).
func TestSemanticCTECompilerInvariantFailure(t *testing.T) {
	executor := newCTEExecutor(nil, nil)
	result := executor.evaluate(&ast.Identifier{Value: "x"})
	if result.Failure == nil || result.Failure.Kind != cteCompilerInvariantFailure || !strings.Contains(result.Failure.Reason, "internal compile-time evaluator failure") {
		t.Fatalf("result = %+v", result.Failure)
	}
}

// A successfully evaluated value outside the owning construct's domain is
// diagnosed by that construct, not classified as an evaluator failure: a
// range bound beyond the base type, a non-integer range bound and an
// unrepresentable default.
// Rules: rules/compiler/compile_time_evaluation.md — § 62.
func TestSemanticCTEOwningContextValidation(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

fn Big() int {
    return 300
}

fn Flag() bool {
    return true
}

type Small int8 range 0..Big()
type Odd int range 0..Flag()
type D int8 default Big()
`)
	want := []struct{ id, text string }{
		{diagnostics.InvalidContractArgument, "range upper bound Big() evaluates to 300, which does not fit the base type of Small"},
		{diagnostics.InvalidContractArgument, "range upper bound Flag() of Odd must be an integer, got true"},
		{diagnostics.DefaultNotRepresentable, "default value Big() is not representable by D"},
	}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v", errors)
	}
	for index, expected := range want {
		if errors[index].ID != expected.id || !strings.Contains(errors[index].Message, expected.text) {
			t.Errorf("error %d = %s %q, want %s %q", index, errors[index].ID, errors[index].Message, expected.id, expected.text)
		}
	}
}

// Union variants with tuple and field payloads are constructed and matched
// (payload, field and payload-free patterns), errors are narrowed with
// `Err(Type.Variant)` and `Err(.Variant)`, and `try` with local handlers
// selects the first accepting handler, binding or comparing the error.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 20, 46;
// rules/control-flow/flowcontrol_match.md; rules/errors/errorhandling.md — try handlers.
func TestSemanticCTEUnionsPatternsAndHandlers(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type State union {
    idle
    running
}

type Number union {
    Integer(int)
    Pair {
        a: int,
        b: int,
    }
}

type Fail enum error {
    Bad,
    Worse,
}

fn Value(n: Number) int {
    return match n {
        Integer(value) => value
        Pair { a, b: second } => a * 10 + second
    }
}

fn StateCode(s: State) int {
    return match s {
        idle => 1
        running => 2
    }
}

fn Parse(n: int) Result[int, Fail] {
    if n < 0 {
        return Err(Fail.Bad)
    }
    if n > 1000 {
        return Err(Fail.Worse)
    }
    return Ok(n)
}

fn Recover(n: int) int {
    let v := try Parse(n) {
        Err(Fail.Bad) => 7
        Err(error) => 8
    }
    return v
}

fn Narrow(n: int) int {
    return match Parse(n) {
        Ok(value) => value
        Err(Fail.Bad) => 100
        Err(.Worse) => 200
    }
}

type A int range 0..Value(Number.Integer(5))
type B int range 0..Value(Number.Pair { a: 2, b: 3 })
type C int range 0..StateCode(State.running)
type D int range 0..Recover(-1)
type E int range 0..Recover(5000)
type F int range 0..Narrow(-5)
type G int range 0..Narrow(2000)

fn Main() int {
    let a: A := 6
    let b: B := 24
    let c: C := 3
    let d: D := 8
    let e: E := 9
    let f: F := 101
    let g: G := 201
    return 0
}
`)
	want := []string{"A 0..5", "B 0..23", "C 0..2", "D 0..7", "E 0..8", "F 0..100", "G 0..200"}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v", errors)
	}
	for index, contract := range want {
		if errors[index].ID != diagnostics.ValueViolatesContract || !strings.Contains(errors[index].Message, contract) {
			t.Errorf("error %d = %q, want contract %s", index, errors[index].Message, contract)
		}
	}
}

// Generic functions execute through their concrete instance, and the call
// chain of a failure names the instantiation (GenericInstantiationContext).
// Rules: rules/compiler/compile_time_evaluation.md — §§ 45, 59.
func TestSemanticCTEGenericInstances(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

fn Pick[T](a: T, b: T, first: bool) T {
    if first {
        return a
    }
    return b
}

fn Checked[T](value: T, ok: bool) T {
    assert ok, "rejected"
    return value
}

type A int range 0..Pick(3, 9, false)
type B int range 0..Checked(10, false)

fn Main() int {
    let a: A := 10
    return 0
}
`)
	if len(errors) != 2 {
		t.Fatalf("errors = %v", errors)
	}
	if errors[0].ID != diagnostics.CompileTimeEvaluationFailed || !strings.Contains(errors[0].Message, "rejected (in Checked[T = int])") {
		t.Errorf("generic failure = %q", errors[0].Message)
	}
	if errors[1].ID != diagnostics.ValueViolatesContract || !strings.Contains(errors[1].Message, "A 0..9") {
		t.Errorf("generic result = %q", errors[1].Message)
	}
}

// Optional early evaluation reuses the executor without source diagnostics:
// a failure only means the fold is unavailable.
// Rules: rules/compiler/compile_time_evaluation.md — § 7(5)–(7).
func TestOptionalCompileTimeValue(t *testing.T) {
	source := `module main

fn Six() int {
    return 2 * 3
}

fn Broken() int {
    return 1 / 0
}

fn Main() int {
    return Six() + Broken()
}
`
	p := parser.New(lexer.NewWithFile(source, "main.sec"))
	program := p.ParseProgram()
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	var six, broken ast.Expression
	for _, statement := range program.Statements {
		if function, ok := statement.(*ast.FunctionDeclaration); ok && function.Name.Value == "Main" {
			sum := function.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.InfixExpression)
			six, broken = sum.Left, sum.Right
		}
	}
	value, ok, err := analyzer.OptionalCompileTimeValue(program, six)
	if err != nil || !ok || value.Integer == nil || value.Integer.Int64() != 6 {
		t.Fatalf("Six() = %+v %v %v", value, ok, err)
	}
	before := len(analyzer.errors)
	if _, ok, err := analyzer.OptionalCompileTimeValue(program, broken); ok || err != nil || len(analyzer.errors) != before {
		t.Fatalf("Broken() optional evaluation = %v %v, errors %d -> %d", ok, err, before, len(analyzer.errors))
	}
}

// Error-variant payload patterns bind the error payload, and a block handler
// yields its final expression value.
// Rules: rules/control-flow/flowcontrol_match.md — open error narrowing;
// rules/errors/errorhandling.md — § 21.1.
func TestSemanticCTEErrorPayloadPatternsAndBlockHandlers(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type ParseError union error {
    TooLong(int)
    Empty
}

fn Parse(n: int) Result[int, ParseError] {
    if n == 0 {
        return Err(ParseError.Empty)
    }
    if n > 9 {
        return Err(ParseError.TooLong(n))
    }
    return Ok(n)
}

fn Length(n: int) int {
    return match Parse(n) {
        Ok(value) => value
        Err(ParseError.TooLong(length)) => length * 2
        Err(ParseError.Empty) => 1
    }
}

fn Fallback(n: int) int {
    let value := try Parse(n) {
        Err(error) => {
            let base := 40
            base + 2
        }
    }
    return value
}

type A int range 0..Length(12)
type B int range 0..Fallback(0)

fn Main() int {
    let a: A := 25
    let b: B := 43
    return 0
}
`)
	want := []string{"A 0..24", "B 0..42"}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v", errors)
	}
	for index, contract := range want {
		if errors[index].ID != diagnostics.ValueViolatesContract || !strings.Contains(errors[index].Message, contract) {
			t.Errorf("error %d = %q, want contract %s", index, errors[index].Message, contract)
		}
	}
}
