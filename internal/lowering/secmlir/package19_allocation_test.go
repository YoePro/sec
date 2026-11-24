package secmlir

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/target"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// buildPackage19TargetSource analyzes with the registered target's complete
// plan, including its verified scalar alignments (MD-141), so layouts resolve.
func buildPackage19TargetSource(t *testing.T, source, os, arch string) (*semantic.Module, error) {
	t.Helper()
	definition, ok := target.Find(target.Target{OS: os, Arch: arch})
	if !ok {
		t.Fatalf("no registered target %s-%s", os, arch)
	}
	plan, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	if plan.Alignments == nil {
		t.Fatalf("%s-%s has no verified scalar alignments", os, arch)
	}
	parsed := parser.New(lexer.NewWithFile(source, "source.sec")).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	analyzer := sema.NewAnalyzerWithScalarPlan(plan)
	analyzer.SetAllocationProfile("hosted")
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	return semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"source.sec"}, MaxPackage: 19})
}

// P19-20 (arena.md §§ 16–31; P19 §§ 20–35, 113–114): New[T]() and
// Alloc[T](count) on a FromBuffer Arena lower to checked sec.arena.new /
// sec.arena.alloc with the canonical layout of T, the canonical default and
// the borrowed fixed capacity; the candidate is a dynamic-epoch reference of
// the current epoch published only as Ok, the owner state version advances
// per mutation and Reset continues the chain; the results are written and
// read through their references before Reset. Verified on 32- and 64-bit.
func TestPackage19ArenaAllocationLowering(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := `module main

type Point struct {
    x: int32,
    y: int64,
}

fn F(count: uint) Result[int64, AllocationError] {
    let mut buffer: byte[256]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    let point := try arena.New[Point]()
    point.y = 7
    let values := try arena.Alloc[int32](count)
    values[0] = 3
    let total := point.y + int64(values[0])
    arena.Reset()
    return Ok(total)
}

fn main() int {
    return 0
}
`
	for _, platform := range []struct{ os, arch, layout string }{
		{"linux", "amd64", `size=16:align=8`},
		{"linux", "armv7", `size=16:align=8`},
	} {
		t.Run(platform.arch, func(t *testing.T) {
			module, err := buildPackage19TargetSource(t, source, platform.os, platform.arch)
			if err != nil {
				t.Fatal(err)
			}
			definition, _ := target.Find(target.Target{OS: platform.os, Arch: platform.arch})
			plan, _ := definition.ScalarPlan()
			output, err := EmitWithOptions(module, plan, EmitOptions{DialectVersion: 15})
			if err != nil {
				t.Fatal(err)
			}
			text := string(output)
			for _, want := range []string{
				`"sec.arena.new"`, `state_before = 1 : i32, state_after = 2 : i32, epoch_before = 1 : i32, epoch_after = 1 : i32`,
				`capacity_plan = "borrowed-fixed-bytes"`, `default_plan = "canonical-default"`, platform.layout,
				`"sec.arena.alloc"`, `state_before = 2 : i32, state_after = 3 : i32`, `size=4:align=4`,
				`"sec.arena.reset"`, `state_before = 3 : i32, state_after = 4 : i32, epoch_before = 1 : i32, epoch_after = 2 : i32`,
				`failure_atomic`, `provenance = "arena"`,
				`sec.arena_demand_summary = [{bytes = `, `capacity_bytes = "256", domain = `, `kind = "constant"`, `range-upper-bound(`,
			} {
				if !strings.Contains(text, want) {
					t.Fatalf("schema-15 module lacks %q:\n%s", want, text)
				}
			}
			verify := exec.Command(tool, "--sec-verify-arenas", "--sec-verify-allocation-contexts", "--sec-verify-places", "--sec-verify-reference-guards",
				"--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s\n%s", err, result, text)
			}
		})
	}
}

// MD-136 (arena.md § 131, P19 § 166), T01/T02/T08/T10: `let mut a: Arena`
// and `Arena {}` lower to sec.arena.create_default (fresh domains, no
// backing, empty-default capacity, the frozen provider plan); Reset of an
// empty default advances its epoch without reclaiming anything, and implicit
// destruction runs the Arena resource cleanup once. Verified schema 15.
func TestPackage19ArenaDefaultLowering(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := `module main

fn F() void {
    let mut first: Arena
    let mut second := Arena {}
    first.Reset()
}

fn main() int {
    return 0
}
`
	module, err := buildPackage19TargetSource(t, source, "linux", "amd64")
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
	if strings.Count(text, `"sec.arena.create_default"`) != 2 {
		t.Fatalf("want two default creations:\n%s", text)
	}
	for _, want := range []string{
		`backing_kind = "none"`, `capacity_plan = "empty-default"`, `provider_plan = "interim-hosted-profile (pending MD-151)"`,
		`growth_kind = "growable-nonrelocating"`, `"sec.arena.reset"`, `state_before = 1 : i32, state_after = 2 : i32`,
		`relation = "Owned"`, `kind = "compiler-known-resource"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("schema-15 module lacks %q:\n%s", want, text)
		}
	}
	verify := exec.Command(tool, "--sec-verify-arenas", "--sec-verify-allocation-contexts", "--sec-verify-places", "--sec-verify-reference-guards",
		"--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if result, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s\n%s", err, result, text)
	}
}

// P19 §§ 43–48, 116; arena.md §§ 43–46, 131(7): an explicit Release of a
// FromBuffer Arena (ends the backing loan) and of the empty default Arena
// (reclaims nothing) moves the owner out of its storage, consumes it in
// sec.arena.release ending the domain, and cancels the owner's cleanup so
// implicit destruction never repeats it. Verified schema 15.
func TestPackage19ArenaReleaseLowering(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := `module main

fn Borrowed() void {
    let mut buffer: byte[64]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    arena.Reset()
    arena.Release()
}

fn Empty() void {
    let mut arena: Arena
    arena.Release()
}

fn main() int {
    return 0
}
`
	module, err := buildPackage19TargetSource(t, source, "linux", "amd64")
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
	if strings.Count(text, `"sec.arena.release"`) != 2 {
		t.Fatalf("want two releases:\n%s", text)
	}
	for _, want := range []string{
		`state_before = 2 : i32, state_after = 3 : i32, epoch_before = 2 : i32`, `transition = "end-domain"`,
		`dependency_proof = "sema-dependency-check"`, `backing_kind = "none"`, `"sec.own.move_from_place"`, `reason = "released"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("schema-15 module lacks %q:\n%s", want, text)
		}
	}
	verify := exec.Command(tool, "--sec-verify-arenas", "--sec-verify-allocation-contexts", "--sec-verify-places", "--sec-verify-reference-guards",
		"--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if result, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s\n%s", err, result, text)
	}
}
