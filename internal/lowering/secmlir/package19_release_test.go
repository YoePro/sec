package secmlir

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/platform/target"
	"sec/internal/testsupport"
)

// package19FunctionText returns the emitted func.func of the source function
// name, up to the next function.
func package19FunctionText(t *testing.T, text, name string) string {
	t.Helper()
	start := strings.Index(text, `sec.source_name = "`+name+`"`)
	if start < 0 {
		t.Fatalf("no function %s:\n%s", name, text)
	}
	if header := strings.LastIndex(text[:start], "func.func @"); header >= 0 {
		start = header
	}
	rest := text[start:]
	if end := strings.Index(rest[1:], "func.func @"); end >= 0 {
		rest = rest[:end+1]
	}
	return rest
}

// P19-35 (P19 § 153; arena.md §§ 43–48; P19 §§ 43–48, 95–96; P17 §§ 38, 73,
// 80): the Release/destruction matrix through schema-15 Sec MLIR. Borrowed
// Release of a local or static backing ends the loan and the domain without
// any reclamation (ExternalOwner, no provider plan, no deallocation op); an
// Arena without explicit Release keeps its tracked cleanup, so implicit
// destruction performs the Release at scope exit; explicit Release cancels
// that cleanup; a moved-from source is not released (its cleanup is
// cancelled as moved, only the destination is released); returning an Arena
// transfers its cleanup to the caller. Owned and target-provided Release
// need verified provider rows (MD-151) and stay a governance remaining item.
func TestPackage19ReleaseMatrix(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := `module main

static let mut backing: byte[4] := [0, 0, 0, 0]

fn Borrowed() void {
    let mut buffer: byte[64]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    arena.Release()
}

fn Static() void {
    let mut arena := Arena.FromBuffer(ref mut backing[..])
    arena.Release()
}

fn Implicit() void {
    let mut buffer: byte[64]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    arena.Reset()
}

fn Moved() void {
    let mut buffer: byte[64]
    let mut first := Arena.FromBuffer(ref mut buffer[..])
    let mut second :<- first
    second.Release()
}

fn Make() Arena {
    let mut arena: Arena
    return arena
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
	tracked := `destruction_plan = "destroy::struct:Arena"`
	for _, name := range []string{"Borrowed", "Static"} {
		body := package19FunctionText(t, text, name)
		if strings.Count(body, `"sec.arena.release"`) != 1 || strings.Contains(body, "provider_plan") || strings.Contains(body, `"sec.arena.destroy"`) {
			t.Fatalf("%s: want one borrowed release without provider or destroy:\n%s", name, body)
		}
		for _, want := range []string{`transition = "end-domain"`, `relation = "Borrowed"`, `authority = "ExternalOwner"`, `reason = "released"`, tracked} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s lacks %q:\n%s", name, want, body)
			}
		}
	}
	if body := package19FunctionText(t, text, "Static"); !strings.Contains(body, "sec.place.static") {
		t.Fatalf("Static: backing does not come from the static root:\n%s", body)
	}
	implicit := package19FunctionText(t, text, "Implicit")
	if !strings.Contains(implicit, tracked) || strings.Contains(implicit, `"sec.cleanup.cancel"`) || strings.Contains(implicit, `"sec.arena.release"`) || !strings.Contains(implicit, `"sec.cleanup.run_function"`) {
		t.Fatalf("Implicit: the tracked Arena cleanup must stay active to scope exit:\n%s", implicit)
	}
	moved := package19FunctionText(t, text, "Moved")
	if strings.Count(moved, `"sec.arena.release"`) != 1 || strings.Count(moved, tracked) != 2 ||
		strings.Count(moved, `reason = "moved"`) != 1 || strings.Count(moved, `reason = "released"`) != 1 ||
		!(strings.Index(moved, `reason = "moved"`) < strings.Index(moved, `"sec.arena.release"`) && strings.Index(moved, `"sec.arena.release"`) < strings.Index(moved, `reason = "released"`)) {
		t.Fatalf("Moved: only the destination may be released:\n%s", moved)
	}
	make := package19FunctionText(t, text, "Make")
	if strings.Contains(make, `"sec.arena.release"`) || !strings.Contains(make, `reason = "transferred"`) || !strings.Contains(make, `sec.return_action = "move"`) {
		t.Fatalf("Make: returning the Arena must transfer its cleanup:\n%s", make)
	}
	verify := exec.Command(tool, "--sec-verify-arenas", "--sec-verify-allocation-contexts", "--sec-verify-places", "--sec-verify-reference-guards",
		"--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if result, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s\n%s", err, result, text)
	}
}
