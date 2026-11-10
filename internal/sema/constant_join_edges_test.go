package sema

import (
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Constant tracking precision at joins: a try handler that returns or
// terminates does not invalidate constants after the try, while a
// value-producing handler that changes a binding still does; a switch clause
// entered by fallthrough starts from the constants its direct and fallthrough
// entries agree on, and fallthrough no longer disables the post-switch merge.
//
// Rules:
//   - rules/foundations/operators.md — "Compile-time overflow"
//   - rules/errors/errorhandling.md — §§ 21–22 handler flow
//   - rules/control-flow/flowcontrol_switch.md — fallthrough entry merge
func TestConstantJoinThroughTryHandlersAndFallthrough(t *testing.T) {
	const source = `module main

type Problem union error {
    Bad,
}

fn Load() Result[int, Problem] {
    return Ok(1)
}

fn ReturningHandler() uint {
    let mut n: uint := 0
    let value := try Load() {
        Err(_) => {
            n = 7
            return n
        }
    }
    discard value
    n -= 1
    return n
}

fn ValueHandler() uint {
    let mut n: uint := 0
    let value := try Load() {
        Err(_) => {
            n = 7
            0
        }
    }
    discard value
    n -= 1
    return n
}

fn FallthroughAgrees(k: int) uint {
    let mut n: uint := 0
    switch k {
    case 1:
        n = 0
        fallthrough
    case 2:
        n -= 1
    default:
        n = 2
    }
    return n
}

fn FallthroughSame(k: int) uint {
    let mut n: uint := 0
    switch k {
    case 1:
        n = 0
        fallthrough
    case 2:
        n += 0
    default:
        n = 0
    }
    n -= 1
    return n
}

fn FallthroughDiffers(k: int) uint {
    let mut n: uint := 1
    switch k {
    case 1:
        n = 0
        fallthrough
    case 2:
        n -= 1
    default:
        n = 2
    }
    return n
}
`
	parsed := parser.New(lexer.New(source))
	program := parsed.ParseProgram()
	if len(parsed.Errors()) > 0 {
		t.Fatal(parsed.Errors())
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("unexpected errors: %v", errors)
	}
	got := map[int]bool{}
	for _, warning := range analyzer.Warnings() {
		if warning.ID == diagnostics.OperatorIntegerOverflow {
			got[warning.Line] = true
		}
	}
	// 20: the returning handler keeps n == 0; 44: both entries of the
	// fallthrough-entered clause agree on n == 0; 62: every clause leaves
	// n == 0 despite the fallthrough. ValueHandler (33) and FallthroughDiffers
	// (73) prove nothing.
	want := map[int]bool{20: true, 44: true, 62: true}
	for line := range want {
		if !got[line] {
			t.Errorf("missing S1023 at line %d; got %v (warnings %v)", line, got, analyzer.Warnings())
		}
	}
	for line := range got {
		if !want[line] {
			t.Errorf("unexpected S1023 at line %d; warnings %v", line, analyzer.Warnings())
		}
	}
}
