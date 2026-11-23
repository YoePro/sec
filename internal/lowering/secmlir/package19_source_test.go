package secmlir

import (
	"bytes"
	"fmt"
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

func buildPackage19Source(t *testing.T, source string, arch string, width uint16) (*semantic.Module, error) {
	t.Helper()
	parsed := parser.New(lexer.NewWithFile(source, "source.sec")).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "linux", TargetArch: arch, Profile: "hosted", PointerWidthBits: width})
	analyzer.SetAllocationProfile("hosted")
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	return semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"source.sec"}, MaxPackage: 19})
}

// P19-20 (§§ 10, 36-42, 110, 115): Arena.FromBuffer creation and Reset of
// the borrowed Arena lower from their P19 plans end to end to verified
// schema 15 on 32- and 64-bit plans: a fresh domain retaining the slice
// borrow, state/epoch steps per Reset, and implicit destruction through the
// Arena resource cleanup plan. Operations without complete plans stop at
// named package-19 boundaries.
func TestPackage19ArenaSourceLowering(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := "module main\n\nfn F() void {\n    let mut buffer: byte[64]\n    let mut arena := Arena.FromBuffer(ref mut buffer[..])\n    arena.Reset()\n    arena.Reset()\n}\n\nfn main() int {\n    F()\n    return 0\n}\n"
	for _, target := range []struct {
		arch  string
		width uint16
	}{{"armv7", 32}, {"amd64", 64}} {
		t.Run(fmt.Sprint(target.width), func(t *testing.T) {
			module, err := buildPackage19Source(t, source, target.arch, target.width)
			if err != nil {
				t.Fatal(err)
			}
			output, err := EmitWithOptions(module, testPlan(target.width), EmitOptions{DialectVersion: 15})
			if err != nil {
				t.Fatal(err)
			}
			text := string(output)
			for _, want := range []string{
				`"sec.arena.create_borrowed"`, `state_after = 1 : i32, epoch_after = 1 : i32, backing_kind = "borrowed"`,
				`"sec.arena.reset"`, `state_before = 1 : i32, state_after = 2 : i32, epoch_before = 1 : i32, epoch_after = 2 : i32`,
				`state_before = 2 : i32, state_after = 3 : i32`, `dependency_proof = "sema-dependency-check"`,
				`kind = "compiler-known-resource"`,
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
	for name, test := range map[string]struct{ body, want string }{
		"WithCapacity": {"fn F() Result[void, AllocationError] {\n    let mut arena := try Arena.WithCapacity(64)\n    discard arena\n    return Ok()\n}", "Arena"},
		"Release":      {"fn F() void {\n    let mut buffer: byte[8]\n    let mut arena := Arena.FromBuffer(ref mut buffer[..])\n    arena.Release()\n}", "P17 ownership termination"},
		"New":          {"fn F() Result[int, AllocationError] {\n    let mut buffer: byte[64]\n    let mut arena := Arena.FromBuffer(ref mut buffer[..])\n    let value := try arena.New[int]()\n    *value = 4\n    return Ok(0)\n}", "Arena"},
	} {
		t.Run(name, func(t *testing.T) {
			module, err := buildPackage19Source(t, "module main\n\n"+test.body+"\n", "amd64", 64)
			if module != nil || err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want named boundary %q, got %v", test.want, err)
			}
		})
	}
}

// P19 § 130: schema 15 carries Sema's canonical allocation knowledge per
// callable as sec.may_allocate (compile-time metadata, never a runtime
// value) and the module verifies; schema 14 cannot express the hidden
// context of G.
func TestPackage19AllocationSummaryMetadata(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := "module main\n\nfn G() Result[void, CollectionError] {\n    let mut values: int[]\n    try values.Append(1)\n    return Ok()\n}\n\nfn H() int {\n    return 1\n}\n\nfn main() int {\n    return H()\n}\n"
	module, err := buildPackage19Source(t, source, "amd64", 64)
	if err != nil {
		t.Fatal(err)
	}
	output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 15})
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	for _, want := range []string{
		`sec.function_id = "main::G()", sec.source_name = "G", sec.extern = false, sec.unsafe = false, sec.may_allocate = "may-allocate"`,
		`sec.function_id = "main::H()", sec.source_name = "H", sec.extern = false, sec.unsafe = false, sec.may_allocate = "allocation-free"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	verify := exec.Command(tool, "--sec-verify-allocation-contexts", "--sec-verify-dynamic-arrays", "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if result, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, result)
	}
	// G requires the hidden context, which schema 14 cannot express.
	if _, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 14}); err == nil || !strings.Contains(err.Error(), "schema 15") {
		t.Fatalf("schema 14 accepted a hidden allocation context: %v", err)
	}
}

// P19-10 (§§ 51-64, 107-109): Sema's RequiresAllocationContext summary
// drives the hidden context: a function with an ambient allocation site and
// its synchronous callers take the marked first input, direct calls forward
// it, the program entry produces the target-provided context, and functions
// needing none (including explicit-Arena allocation) get none. Schema 15
// verifies, including the summary metadata.
func TestPackage19AllocationContextPropagation(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := "module main\n\nfn Grow() Result[void, CollectionError] {\n    let mut values: int[]\n    try values.Append(1)\n    return Ok()\n}\n\nfn Middle() Result[void, CollectionError] {\n    return Grow()\n}\n\nfn Pure() int {\n    return 1\n}\n\nfn main() int {\n    discard Middle()\n    return Pure()\n}\n"
	for _, target := range []struct {
		arch  string
		width uint16
	}{{"armv7", 32}, {"amd64", 64}} {
		module, err := buildPackage19Source(t, source, target.arch, target.width)
		if err != nil {
			t.Fatal(err)
		}
		output, err := EmitWithOptions(module, testPlan(target.width), EmitOptions{DialectVersion: 15})
		if err != nil {
			t.Fatal(err)
		}
		text := string(output)
		for _, want := range []string{
			`(%v0: !sec.alloc_context {sec.hidden_allocation_context = true}) -> (`,
			`"sec.call.direct"(%v0) <{callee = @sec_fn_0}> {sec.argument_actions = ["allocation-context"]`,
			`"sec.alloc_context.target"() <{origin_plan = "target-provided:hosted"}>`,
			`sec.requires_allocation_context = true`,
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("missing %q in:\n%s", want, text)
			}
		}
		if strings.Count(text, "sec.hidden_allocation_context = true") != 2 {
			t.Fatalf("exactly Grow and Middle take a hidden context:\n%s", text)
		}
		verify := exec.Command(tool, "--sec-verify-allocation-contexts", "--sec-verify-dynamic-arrays", "--sec-verify-storage-transitions", "--sec-verify-cleanups", "--sec-verify-ownership", "-", "-o", os.DevNull)
		verify.Stdin = bytes.NewReader(output)
		if result, err := verify.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s\n%s", err, result, text)
		}
	}
}
