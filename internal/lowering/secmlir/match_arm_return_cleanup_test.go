package secmlir

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// A `return` written as a match arm body leaves the invocation like a return
// statement, so Sema records its cleanup exit and the function's owned
// registrations (including the arm's payload binding) run before it: on a
// fresh temporary subject (whose payload forwards into the binding) and on a
// local one.
// Rules: rules/memory/destruction.md — § 6.2; rules/control-flow/flowcontrol_match.md — § 13;
// rules/compiler/semantic_ir.md — § 69(4).
func TestSchema13MatchArmReturnRunsCleanup(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := `module main

type Box struct {
    value: int,
}

impl Box {
    free {
    }
}

fn Make() Option[Box] {
    return Some(Box{value: 1})
}

fn Take(b: Box) int {
    return b.value
}

fn Temporary() int {
    match Make() {
        Some(b) => return Take(<-b)
        None => return 0
    }
}

fn Local() int {
    let o := Make()
    match o {
        Some(b) => return Take(<-b)
        None => return 0
    }
}

fn main() int {
    return 0
}
`
	parsed := parser.New(lexer.NewWithFile(source, "arms.sec")).Parse()
	analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64})
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"arms.sec"}, MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	if err := semantic.Verify(module); err != nil {
		t.Fatal(err)
	}
	output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 13})
	if err != nil {
		t.Fatal(err)
	}
	verify := exec.Command(tool, "--sec-verify-places", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if out, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("native verify: %v\n%s", err, out)
	}
}
