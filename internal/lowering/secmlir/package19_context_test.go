package secmlir

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/target"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

const package19ContextSource = `module main

fn Grow() Result[void, CollectionError] {
    let mut values: int[]
    try values.Append(1)
    return Ok()
}

fn Middle() Result[void, CollectionError] {
    return Grow()
}

fn Outer() Result[void, CollectionError] {
    return Middle()
}

fn Lexical() Result[void, CollectionError] {
    let mut buffer: byte[64]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    arena.Reset()
    return Grow()
}

fn Explicit() Result[int32, AllocationError] {
    let mut buffer: byte[64]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    let values := try arena.Alloc[int32](4u)
    values[0] = 3
    return Ok(values[0])
}

fn Pure() int {
    return 1
}

fn main() int {
    discard Outer()
    discard Lexical()
    discard Explicit()
    return Pure()
}
`

// P19-36 (P19 § 154; P19 §§ 51-64, 77, 107-109, 119): the allocation-context
// matrix through schema-15 Sec MLIR. Allocation through an explicit Arena
// MayAllocate without requiring context; an ambient allocation (T[] Append)
// requires it and synchronous propagation carries it A -> B -> C as the
// hidden first input and first call operand; a function without the
// requirement gets none; a lexical Arena in scope is never taken as the
// context (the caller's hidden input is forwarded, no context is produced
// from the Arena); only the entry root produces the target-provided context.
// Missing context is a compile-time error under the known no-context profile
// (noalloc, S1108 with the causal path) and fails closed before physical
// lowering under an unresolved profile. The spawn boundary is covered by
// TestAllocationContextRequirements and the foreign-ABI rejection by the
// Semantic IR verifier matrix; see package19-todo.md P19-36 for the rows that
// remain.
func TestPackage19AllocationContextMatrix(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	module, err := buildPackage19TargetSource(t, package19ContextSource, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	definition, _ := target.Find(target.Target{OS: "linux", Arch: "amd64"})
	plan, _ := definition.ScalarPlan()
	output, err := EmitWithOptions(module, plan, EmitOptions{DialectVersion: 15})
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	const hidden = `(%v0: !sec.alloc_context {sec.hidden_allocation_context = true})`
	const forwarded = `"sec.call.direct"(%v0) <{callee = @`
	for _, name := range []string{"Grow", "Middle", "Outer", "Lexical"} {
		body := package19FunctionText(t, text, name)
		if !strings.Contains(body, hidden) || !strings.Contains(body, `sec.requires_allocation_context = true`) {
			t.Fatalf("%s: want the hidden context input:\n%s", name, body)
		}
		if strings.Contains(body, `"sec.alloc_context.`) {
			t.Fatalf("%s: a propagated context must not be produced locally:\n%s", name, body)
		}
		if name != "Grow" && (!strings.Contains(body, forwarded) || !strings.Contains(body, `sec.argument_actions = ["allocation-context"`)) {
			t.Fatalf("%s: the hidden input is not forwarded as the first call operand:\n%s", name, body)
		}
	}
	for _, name := range []string{"Explicit", "Pure"} {
		body := package19FunctionText(t, text, name)
		if strings.Contains(body, "!sec.alloc_context") || strings.Contains(body, "sec.requires_allocation_context") {
			t.Fatalf("%s: must not receive a context:\n%s", name, body)
		}
	}
	if body := package19FunctionText(t, text, "Explicit"); !strings.Contains(body, `sec.may_allocate = "may-allocate"`) || !strings.Contains(body, `"sec.arena.alloc"`) {
		t.Fatalf("Explicit: MayAllocate through the explicit Arena:\n%s", body)
	}
	if body := package19FunctionText(t, text, "Pure"); strings.Contains(body, `sec.may_allocate = "may-allocate"`) {
		t.Fatalf("Pure: must not allocate:\n%s", body)
	}
	entry := package19FunctionText(t, text, "main")
	if strings.Count(text, `"sec.alloc_context.target"`) != 1 || !strings.Contains(entry, `"sec.alloc_context.target"() <{origin_plan = "target-provided:hosted"}>`) || strings.Contains(entry, hidden) {
		t.Fatalf("only the entry root produces the target-provided context:\n%s", text)
	}
	if strings.Count(text, "sec.hidden_allocation_context = true") != 4 {
		t.Fatalf("exactly Grow, Middle, Outer and Lexical take a hidden context:\n%s", text)
	}
	verify := exec.Command(tool, "--sec-verify-arenas", "--sec-verify-allocation-contexts", "--sec-verify-dynamic-arrays", "--sec-verify-storage-transitions",
		"--sec-verify-places", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if result, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s\n%s", err, result, text)
	}

	for _, profile := range []string{"noalloc", "freestanding"} {
		t.Run("missing-"+profile, func(t *testing.T) {
			source := strings.Split(package19ContextSource, "fn Lexical()")[0] + "fn main() int {\n    discard Outer()\n    return 0\n}\n"
			parsed := parser.New(lexer.NewWithFile(source, "source.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(plan)
			analyzer.SetAllocationProfile(profile)
			errors := analyzer.Analyze(parsed.Program)
			if profile == "noalloc" {
				if len(errors) == 0 || errors[0].ID != diagnostics.NoAllocViolation || !strings.Contains(errors[0].Message, "main -> Outer -> Middle -> Grow") {
					t.Fatalf("want a compile-time error with the causal path: %v", errors)
				}
				return
			}
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			built, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"source.sec"}, MaxPackage: 19})
			if built != nil || err == nil || !strings.Contains(err.Error(), "implicit allocation context: unavailable; profile freestanding") {
				t.Fatalf("an unresolved profile must fail closed before physical lowering: %v", err)
			}
		})
	}
}
