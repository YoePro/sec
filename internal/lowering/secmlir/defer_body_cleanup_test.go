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

// P17-41: a defer body is its own invocation (defer.md § 10). Owned locals
// it declares register in the body's scopes inside the region and are
// destroyed when the body completes, before cleanup.defer-yield: in nested
// scopes, once per loop iteration, and not at all once moved. break,
// continue and return stay invalid in a defer body (defer.md § 18).
// The schema-13 verifiers check the body as its own invocation.
// Rules: rules/control-flow/defer.md — §§ 10–12, 18; rules/memory/destruction.md — §§ 6, 8;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 41–45.
func TestSchema13DeferBodyOwnedLocals(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := `module main

type Box struct {
    value: int,
}

impl Box {
    free {
    }
}

fn Use(b: ref Box) void {
}

fn Take(b: Box) void {
}

fn Make() Box {
    return Box{value: 1}
}

fn Local() void {
    let outer := Make()
    defer {
        let inner := Make()
        Use(ref inner)
        Use(ref outer)
    }
}

fn Nested(flag: bool) void {
    defer {
        let first := Make()
        if flag {
            let second := Make()
            Use(ref second)
        }
        Use(ref first)
    }
}

fn Moved() void {
    defer {
        let inner := Make()
        Take(<-inner)
    }
}

fn Looping() void {
    defer {
        for i in 0..3 {
            let each := Make()
            Use(ref each)
        }
    }
}

fn main() int {
    return 0
}
`
	parsed := parser.New(lexer.NewWithFile(source, "probe.sec")).Parse()
	analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64})
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"probe.sec"}, MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 13})
	if err != nil {
		t.Fatal(err)
	}
	text := semantic.Format(module)
	body := func(name string) string {
		start := strings.Index(text, `func "main::`+name+`(`)
		if start < 0 {
			t.Fatalf("no %s in:\n%s", name, text)
		}
		end := strings.Index(text[start:], "\n  }\n")
		return text[start : start+end]
	}
	region := func(name string) string {
		part := body(name)
		start := strings.Index(part, "cleanup.defer-register")
		// The region closes before the function's own exit cleanup.
		end := strings.LastIndex(part, "cleanup.run-function")
		if start < 0 || end < start || !strings.Contains(part[start:end], "cleanup.defer-yield") {
			t.Fatalf("%s has no defer region:\n%s", name, part)
		}
		return part[start:end]
	}
	for name, want := range map[string][]string{
		"Local":   {"cleanup.track-owned", "cleanup.run-scope scopes=["},
		"Nested":  {"cleanup.track-owned", "cleanup.run-scope"},
		"Moved":   {"cleanup.track-owned", "own.move", "cleanup.cancel"},
		"Looping": {"cleanup.track-owned", "cleanup.run-scope"},
	} {
		part := region(name)
		for _, item := range want {
			if !strings.Contains(part, item) {
				t.Errorf("%s region lacks %q:\n%s", name, item, part)
			}
		}
		if strings.Contains(part, "cleanup.run-function") {
			t.Errorf("%s region runs function cleanup:\n%s", name, part)
		}
		if strings.Count(part, "cleanup.track-owned") > 0 && !strings.Contains(part, "cleanup.run-scope") && !strings.Contains(part, "cleanup.cancel") {
			t.Errorf("%s region leaves a body local registered:\n%s", name, part)
		}
	}
	if got := strings.Count(region("Nested"), "cleanup.track-owned"); got != 2 {
		t.Errorf("Nested registers %d body locals, want 2", got)
	}
	verify := exec.Command(tool, "--sec-verify-slice-guards", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if out, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("native verify: %v\n%s", err, out)
	}
}
