package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// A checked conversion executes with the runtime order of checks: the
// intrinsic domain (ConversionError.OutOfRange) first, then declared
// contracts in source order (ContractError.Violation { Kind,
// DeclarationIndex }, the channel Sema types for numeric contracts); try and
// match handle the error like any other. A bound established by CTE is read in dependency order, even when
// its type is declared after the code that needs it.
// Rules: rules/types/contracts.md — §§ 13, 13.1; rules/compiler/compile_time_evaluation.md — § 38.
func TestSemanticCTECheckedConversions(t *testing.T) {
	source := `module main

fn ToPercent(n: int) Result[Percent, ContractError] {
    let p := try Percent(n)
    return Ok(p)
}

fn ToSmall(n: int) Result[int8, ConversionError] {
    let s := try int8(n)
    return Ok(s)
}

fn Probe(n: int) int {
    match ToPercent(n) {
        Ok(p) => return int(p)
        Err(_) => return -1
    }
}

fn Narrow(n: int) int {
    match ToSmall(n) {
        Ok(s) => return int(s)
        Err(ConversionError.OutOfRange) => return -2
        Err(_) => return -1
    }
}

fn Hundred() int {
    return 100
}

type Percent int range 0..Hundred() odd

let Fits: int := Probe(51)
let Above: int := Probe(150)
let Even: int := Probe(50)
let Wide: int := Narrow(300)
let Small: int := Narrow(-7)
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, source)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	want := map[string]string{"Fits": "51", "Above": "-1", "Even": "-1", "Wide": "-2", "Small": "-7"}
	for _, static := range analyzer.ResolvedStaticDeclarations() {
		expected, ok := want[static.Name]
		if !ok {
			continue
		}
		if static.Initializer.Integer == nil || static.Initializer.Integer.String() != expected {
			t.Errorf("%s = %v, want %s", static.Name, static.Initializer.Integer, expected)
		}
		delete(want, static.Name)
	}
	if len(want) != 0 {
		t.Errorf("missing statics %v", want)
	}
}

// A type fact whose establishment executes code that consults the same fact
// is a type/value dependency cycle (S1226), reported with the chain of
// facts, not a placeholder bound or an infinite recursion; terminating
// recursion stays legal.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 39–41.
func TestSemanticCTETypeFactCycles(t *testing.T) {
	source := `module main

type Limit int range 0..Bound()

fn CheckLimit(n: int) int {
    let checked := try Limit(n) {
        Err(_) => {
            return 0
        }
    }
    return 1
}

fn Bound() int {
    return CheckLimit(5) + 9
}

type Ping int range 0..PingBound()
type Pong int range 0..PongBound()

fn CheckPong(n: int) int {
    let p := try Pong(n) {
        Err(_) => {
            return 0
        }
    }
    return 1
}

fn CheckPing(n: int) int {
    let p := try Ping(n) {
        Err(_) => {
            return 0
        }
    }
    return 1
}

fn PingBound() int {
    return CheckPong(1) + 19
}

fn PongBound() int {
    return CheckPing(1) + 29
}
`
	errors := analyzeSourceRaw(t, source)
	cycles := []string{}
	for _, err := range errors {
		if err.ID == diagnostics.CompileTimeDependencyCycle {
			cycles = append(cycles, err.Message)
		}
	}
	if len(cycles) < 2 || len(errors) != len(cycles) {
		t.Fatalf("errors = %v", errors)
	}
	if !strings.Contains(cycles[0], "the range upper bound of Limit depends on itself") {
		t.Errorf("self cycle = %q", cycles[0])
	}
	if !strings.Contains(cycles[1], "the range upper bound of Ping -> the range upper bound of Pong -> the range upper bound of Ping") {
		t.Errorf("mutual cycle = %q", cycles[1])
	}
}

// A type default established by CTE is a type fact like a contract bound:
// executed code that default-initializes a field of the type reads it,
// establishing it on demand, and a default whose execution needs itself is
// a dependency cycle.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 38, 41; rules/types/default_values.md.
func TestSemanticCTETypeDefaultFacts(t *testing.T) {
	valid := `module main

type Level int range 0..100 default Start()

type Holder struct {
    level: Level,
}

fn Start() int {
    return 7
}

fn Read() int {
    let h := Holder {}
    return int(h.level)
}

let Value: int := Read()
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, valid)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	for _, static := range analyzer.ResolvedStaticDeclarations() {
		if static.Name == "Value" && (static.Initializer.Integer == nil || static.Initializer.Integer.String() != "7") {
			t.Errorf("Value = %v, want 7", static.Initializer.Integer)
		}
	}
	cyclic := strings.Replace(valid, "fn Start() int {\n    return 7", "fn Start() int {\n    let h := Holder {}\n    return 7", 1)
	errors = analyzeSourceRaw(t, cyclic)
	if len(errors) == 0 || errors[0].ID != diagnostics.CompileTimeDependencyCycle ||
		!strings.Contains(errors[0].Message, "the default of Level depends on itself") {
		t.Fatalf("errors = %v", errors)
	}
}
