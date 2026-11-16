package secmlir

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// The schema-13 ownership verifier reads non-triviality from the module's
// destruction plan table, so every type that needs destruction and appears
// in a value carries its plan even when no cleanup registration covers it
// (a returned call result, a forwarded value). A value of such a type that
// loses its terminal action is then rejected.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 53, 112–116;
// rules/mlir/dialect-versions/sec_mlir_dialect_v13.md — §§ 25–27.
func TestSchema13DestructionPlansCoverEveryOwnedValue(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := `module main

type Box struct {
    value: int,
}

impl Box {
    free {
    }
}

type Pair struct {
    left: Box,
    count: int,
}

fn Make() Box {
    return Box{value: 1}
}

fn Forward() Box {
    return Make()
}

fn Wrap() Pair {
    return Pair{left: Make(), count: 2}
}

fn main() int {
    return 0
}
`
	parsed := parser.New(lexer.NewWithFile(source, "plans.sec")).Parse()
	analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64})
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"plans.sec"}, MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	if err := semantic.Verify(module); err != nil {
		t.Fatal(err)
	}
	planned := map[string]bool{}
	for _, plan := range module.DestructionPlans {
		if typ, ok := module.Types.Lookup(plan.Type); ok {
			planned[typ.Identity] = true
		}
	}
	for _, identity := range []string{"main::Box", "main::Pair"} {
		if !planned[identity] {
			t.Errorf("no destruction plan for %s, which a value of the module holds", identity)
		}
	}
	output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 13})
	if err != nil {
		t.Fatal(err)
	}
	run := func(text []byte) ([]byte, error) {
		verify := exec.Command(tool, "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
		verify.Stdin = bytes.NewReader(text)
		return verify.CombinedOutput()
	}
	if out, err := run(output); err != nil {
		t.Fatalf("native verify: %v\n%s", err, out)
	}
	// Without its return action, Forward's returned call result has no
	// written-down move; only a plan for Box lets the verifier see that.
	text := string(output)
	name := strings.Index(text, `sec.source_name = "Forward"`)
	if name < 0 {
		t.Fatalf("no Forward in:\n%s", text)
	}
	forward := strings.LastIndex(text[:name], "func.func @")
	end := name + strings.Index(text[name:], "} loc(")
	part := strings.Replace(text[forward:end], `{sec.return_action = "move"}`, "", 1)
	if part == text[forward:end] {
		t.Fatalf("Forward returns without an action:\n%s", text[forward:end])
	}
	if out, err := run([]byte(text[:forward] + part + text[end:])); err == nil || !strings.Contains(string(out), "needs destruction") {
		t.Errorf("a returned owned value without its move action verified: %v\n%s", err, out)
	}
}
