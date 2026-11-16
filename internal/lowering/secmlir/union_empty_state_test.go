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

// P17-19 follow-up: a mutable binding of a NonDefaultable union begins in the
// compiler-known `empty` initialization state. The storage is initialized
// with storage.init-empty; `is empty`, a variant test and a match with a
// reachable `empty` state test it with storage.is-empty before any value
// read; the `empty` arm or an unguarded `_` receives the empty path, and the
// value path loads and dispatches as usual. The verifier rejects a load the
// state may still reach.
// Rules: rules/types/default_values.md — § 33(4)–(6); rules/control-flow/flowcontrol_match.md — § 11;
// rules/declarations/unions.md — §§ 7–8; rules/compiler/semantic_ir.md — § 23(4).
func TestSchema13EmptyUnionState(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := `module main

type Pending union {
    Idle
    Running(int)
}

fn Assigned() int {
    let mut p: Pending
    p = Pending.Running(3)
    match p {
        Pending.Idle => return 0
        Pending.Running(n) => return n
    }
}

fn IsEmpty(flag: bool) bool {
    let mut p: Pending
    if flag {
        p = Pending.Idle
    }
    return p is empty
}

fn IsIdle(flag: bool) bool {
    let mut p: Pending
    if flag {
        p = Pending.Idle
    }
    return p is Idle
}

fn EmptyArm(flag: bool) int {
    let mut p: Pending
    if flag {
        p = Pending.Running(2)
    }
    match p {
        empty => return -1
        Pending.Idle => return 0
        Pending.Running(n) => return n
    }
}

fn CatchAll(flag: bool) int {
    let mut p: Pending
    if flag {
        p = Pending.Running(2)
    }
    match p {
        Pending.Running(n) => return n
        _ => return 0
    }
}

fn main() int {
    return 0
}
`
	parsed := parser.New(lexer.NewWithFile(source, "empty.sec")).Parse()
	analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64})
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"empty.sec"}, MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	if err := semantic.Verify(module); err != nil {
		t.Fatal(err)
	}
	text := semantic.Format(module)
	body := func(name string) string {
		start := strings.Index(text, `func "main::`+name+`(`)
		end := strings.Index(text[start:], "\n  }\n")
		return text[start : start+end]
	}
	for name, want := range map[string]int{"Assigned": 0, "IsEmpty": 1, "IsIdle": 1, "EmptyArm": 1, "CatchAll": 1} {
		part := body(name)
		if !strings.Contains(part, "storage.init-empty") {
			t.Errorf("%s does not begin empty:\n%s", name, part)
		}
		if got := strings.Count(part, "storage.is-empty"); got != want {
			t.Errorf("%s tests the empty state %d times, want %d:\n%s", name, got, want, part)
		}
	}
	if part := body("EmptyArm"); strings.Index(part, "storage.is-empty") > strings.Index(part, "storage.load") {
		t.Errorf("EmptyArm loads the subject before testing the empty state:\n%s", part)
	}

	// A load the empty state may reach is rejected.
	broken, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"empty.sec"}, MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	for _, fn := range broken.Functions {
		if fn.Name != "IsIdle" {
			continue
		}
		for _, block := range fn.Blocks {
			for index, op := range block.Operations {
				if op.Kind == semantic.OpCondBranch && index > 0 && block.Operations[index-1].Kind == semantic.OpStorageIsEmpty {
					block.Operations[index].Successors[0], block.Operations[index].Successors[1] = block.Operations[index].Successors[1], block.Operations[index].Successors[0]
				}
			}
		}
	}
	if err := semantic.Verify(broken); err == nil || !strings.Contains(err.Error(), "may still be empty") {
		t.Errorf("a load reachable from the empty state verified: %v", err)
	}

	output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 13})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"sec.storage.init_empty", "sec.storage.is_empty", `sec.match_pattern_kind = "empty-state"`} {
		if !bytes.Contains(output, []byte(want)) {
			t.Errorf("schema 13 lacks %q:\n%s", want, output)
		}
	}
	verify := exec.Command(tool, "--sec-verify-places", "--sec-verify-cleanups", "--sec-verify-ownership", "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if out, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("native verify: %v\n%s", err, out)
	}
	if _, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 12}); err == nil {
		t.Error("schema 12 emitted the empty union state")
	}
}

// An owned union declared empty registers nothing at its declaration. Each
// assignment registers its value where no earlier incarnation is live and
// replaces one where it is (MD-125); the first assignment inherits the
// declaration's never-registered incarnation, so in a loop the previous
// iteration's value is destroyed by the replacement, never leaked.
// Rules: rules/types/default_values.md — § 33(5); rules/memory/destruction.md — § 13.3;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 31–33, 38.
func TestSchema13OwnedEmptyUnionState(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := `module main

type Box struct {
    value: int,
}

impl Box {
    free {
    }
}

fn Make() Box {
    return Box{value: 1}
}

type Slot union {
    Idle
    Held(Box)
}

fn Once() void {
    let mut s: Slot
    s = Slot.Held(Make())
}

fn Maybe(flag: bool) bool {
    let mut s: Slot
    if flag {
        s = Slot.Held(Make())
    }
    return s is empty
}

fn Twice(flag: bool) void {
    let mut s: Slot
    if flag {
        s = Slot.Held(Make())
    }
    s = Slot.Idle
}

fn Looped() void {
    let mut s: Slot
    for i in 0..3 {
        s = Slot.Held(Make())
    }
}

fn main() int {
    return 0
}
`
	parsed := parser.New(lexer.NewWithFile(source, "owned_empty.sec")).Parse()
	analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64})
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"owned_empty.sec"}, MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	if err := semantic.Verify(module); err != nil {
		t.Fatal(err)
	}
	text := semantic.Format(module)
	// Every union storage is owned: a value enters it only through Place
	// ownership operations, never a plain storage.store.
	for _, fn := range module.Functions {
		union := map[semantic.StorageID]bool{}
		for _, storage := range fn.Storages {
			if storage.Name == "s" {
				union[storage.ID] = true
			}
		}
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				if op.Kind == semantic.OpStorageStore && union[op.Storage] {
					t.Errorf("%s stores an owned union value without ownership", fn.Name)
				}
			}
		}
	}
	if got := strings.Count(text, "own.replace-place"); got != 5 {
		t.Errorf("want one conditional replacement per assignment (5), got %d:\n%s", got, text)
	}
	if got := strings.Count(text, "cleanup.track-owned"); got != 5 || !strings.Contains(text, "unless-active=") {
		t.Errorf("want one conditional registration per assignment, got %d:\n%s", got, text)
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
