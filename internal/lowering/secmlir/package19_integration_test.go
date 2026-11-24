package secmlir

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/target"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// P19-41 (P19 § 160): Arena lowering composes with the P15 reference, P16
// slice and P18 dynamic-array models through schema-15 Sec MLIR.
//   - P15: New publishes a ref mut T of the current epoch (dynamic-epoch,
//     provenance arena) that is written and read; a use after Reset is a
//     Sema error, never a lowered stale access.
//   - P16: Alloc publishes a mutable slice; element index and a shared
//     subslice range are checked and keep the Arena domain and epoch.
//   - P18: an owning T[] grows through the resolved allocation context with
//     Arena reclamation authority and the canonical storage transitions (no
//     second storage model and no individual reclaim of Arena bytes).
//
// The P17 rows (move, implicit destruction, explicit Release cleanup
// cancellation) are TestPackage19ReleaseMatrix. Defer ordering with Arena
// operations or Arena-backed references (P19-21) and a T[] backed by an
// explicit Arena (P19-10 §§ 55, 59; non-trivial elements MD-135) remain
// governance items.
func TestPackage19IntegrationMatrix(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := `module main

type Point struct {
    x: int32,
    y: int64,
}

fn References() Result[int64, AllocationError] {
    let mut buffer: byte[64]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    let point := try arena.New[Point]()
    point.y = 7
    let y := point.y
    arena.Reset()
    return Ok(y)
}

fn Slices() Result[int32, AllocationError] {
    let mut buffer: byte[64]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    let values := try arena.Alloc[int32](4u)
    values[1] = 5
    let part := ref values[1..<3]
    let first := part[0]
    return Ok(first)
}

fn Dynamic() Result[void, CollectionError] {
    let mut values: int[]
    try values.Append(1)
    return Ok()
}

fn main() int {
    discard References()
    discard Slices()
    discard Dynamic()
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
	arenaReference := `validity_policy = "dynamic-epoch", invalidation_domain = `
	references := package19FunctionText(t, text, "References")
	for _, want := range []string{`"sec.arena.new"`, arenaReference, `provenance = "arena"`, `"sec.arena.reset"`} {
		if !strings.Contains(references, want) {
			t.Fatalf("References lacks %q:\n%s", want, references)
		}
	}
	slices := package19FunctionText(t, text, "Slices")
	for _, want := range []string{`"sec.arena.alloc"`, `"sec.slice.index_in_bounds"`, `"sec.slice.element_place"`, `"sec.slice.range_check"`, `"sec.slice.reborrow_shared"`} {
		if !strings.Contains(slices, want) {
			t.Fatalf("Slices lacks %q:\n%s", want, slices)
		}
	}
	allocated := ""
	for _, line := range strings.Split(slices, "\n") {
		if strings.Contains(line, `"sec.arena.alloc"`) {
			start := strings.Index(line, arenaReference)
			end := strings.Index(line[start:], ", address_stability")
			if start < 0 || end < 0 {
				t.Fatalf("Alloc result lacks reference facts: %s", line)
			}
			allocated = line[start : start+end]
		}
	}
	for _, line := range strings.Split(slices, "\n") {
		if (strings.Contains(line, `"sec.slice.element_place"`) || strings.Contains(line, `"sec.slice.reborrow_shared"`)) && !strings.Contains(line, allocated) {
			t.Fatalf("slice projection lost the Arena domain or epoch: %s", line)
		}
	}
	dynamic := package19FunctionText(t, text, "Dynamic")
	if !strings.Contains(dynamic, `"sec.dynamic_array.grow_backing"`) || !strings.Contains(dynamic, `authority = "Arena"`) || !strings.Contains(dynamic, `allocation_context = `) ||
		!strings.Contains(dynamic, `"sec.storage.establish_domain"`) || strings.Contains(dynamic, `"sec.storage.reclaim"`) || strings.Contains(dynamic, `"sec.arena.`) {
		t.Fatalf("Dynamic: T[] must grow through the resolved context in the canonical storage model:\n%s", dynamic)
	}
	verify := exec.Command(tool, "--sec-verify-arenas", "--sec-verify-allocation-contexts", "--sec-verify-places", "--sec-verify-reference-guards",
		"--sec-verify-dynamic-arrays", "--sec-verify-storage-transitions", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if result, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s\n%s", err, result, text)
	}

	stale := strings.Replace(source, "    arena.Reset()\n    return Ok(y)", "    arena.Reset()\n    let late := point.y\n    return Ok(late)", 1)
	parsed := parser.New(lexer.NewWithFile(stale, "source.sec")).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	analyzer := sema.NewAnalyzerWithScalarPlan(plan)
	analyzer.SetAllocationProfile("hosted")
	errors := analyzer.Analyze(parsed.Program)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "reset") {
		t.Fatalf("a use after Reset must be a Sema error: %v", errors)
	}
}
