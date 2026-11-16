package secmlir

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// P16 carry-over closed in P17: a direct reference holder in a mutable
// binding lives in storage. `r = otherRef` (or `r <- other` for a `ref mut`
// holder) stores the new reference and writes nothing through the old one;
// every use reads the holder out of its Place with Sema's facts for that
// point, which name every origin a rebinding or join can bring there, and the
// verifier checks them against each reaching store. Slice and StringView
// holders work the same way, and so does a try recovery value that joins
// slice holders through a block parameter. A join of different provenance kinds keeps
// every alternative under the joined provenance (package15 § 20).
// Rules: rules/memory/references.md — § 3.4(5); rules/memory/borrowing.md — §§ 8.2, 35.2;
// rules/memory/lifetime_analysis.md — § 11.
func TestSchema13ReferenceHolderRebind(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := `module main

fn Straight(a: ref int, b: ref int) int {
    let mut r := a
    let first := *r
    r = b
    return first + *r
}

fn Joined(a: ref int, b: ref int, c: bool) int {
    let mut r := a
    if c {
        r = b
    }
    return *r
}

fn Locals(c: bool) int {
    let x := 1
    let y := 2
    let mut r := ref x
    if c {
        r = ref y
    }
    return *r
}

fn Exclusive(a: ref mut int, b: ref mut int) void {
    let mut r :<- a
    *r = 1
    r <- b
    *r = 3
}

fn Mixed(a: ref int, c: bool) int {
    let y := 2
    let mut r := a
    if c {
        r = ref y
    }
    return *r
}

fn SliceRebind(a: ref int[], b: ref int[]) int {
    let mut s := a
    s = b
    return s[0]
}

fn SliceJoin(c: bool) uint {
    let x: int[3] := [1, 2, 3]
    let y: int[2] := [4, 5]
    let mut s := ref x[..]
    if c {
        s = ref y[..]
    }
    return s.Len
}

fn SliceExclusive(a: ref mut int[], b: ref mut int[]) void {
    let mut s :<- a
    s[0] = 1
    s <- b
    s[0] = 2
}

fn View(a: StringView, b: StringView) uint {
    let mut v := a
    v = b
    return v.Len
}

fn Recovery(values: int[4], start: int) uint {
    let view := try ref values[start..] {
        Err(_) => ref values[..]
    }
    return view.Len
}

fn main() int {
    return 0
}
`
	parsed := parser.New(lexer.NewWithFile(source, "rebind.sec")).Parse()
	analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64})
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"rebind.sec"}, MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	if err := semantic.Verify(module); err != nil {
		t.Fatal(err)
	}
	text := semantic.Format(module)
	if !strings.Contains(text, "holder {") || !regexp.MustCompile(`holder \{borrow=#\d+ lifetime=#\d+ validity=proven proof=lexical-borrow space=ordinary stability=Stable provenance=local origins=\[#\d+,#\d+\]\}`).MatchString(text) {
		t.Errorf("holder reads or the joined origins are missing:\n%s", text)
	}
	// The join verifier checks each holder read against every reaching store.
	for name, mutate := range map[string]func(*semantic.ReferenceMetadata){
		"omitted origin":   func(facts *semantic.ReferenceMetadata) { facts.OriginStorages = facts.OriginStorages[:1] },
		"other provenance": func(facts *semantic.ReferenceMetadata) { facts.Provenance = "parameter" },
	} {
		broken, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"rebind.sec"}, MaxPackage: 17})
		if err != nil {
			t.Fatal(err)
		}
		for _, fn := range broken.Functions {
			if fn.Name != "Locals" {
				continue
			}
			for _, block := range fn.Blocks {
				for index, op := range block.Operations {
					if op.PlaceReadMode == semantic.PlaceReadHolder {
						facts := *op.Reference
						facts.OriginStorages = append([]uint32(nil), facts.OriginStorages...)
						mutate(&facts)
						block.Operations[index].Reference = &facts
					}
				}
			}
		}
		if err := semantic.Verify(broken); err == nil || !strings.Contains(err.Error(), "holder read of") {
			t.Errorf("%s: a holder read not covering its reaching references verified: %v", name, err)
		}
	}
	if !strings.Contains(text, "provenance=joined") {
		t.Errorf("the mixed join lost its alternatives:\n%s", text)
	}
	output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 13})
	if err != nil {
		t.Fatal(err)
	}
	verify := exec.Command(tool, "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "--sec-verify-cleanups", "--sec-verify-ownership", "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if out, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("native verify: %v\n%s", err, out)
	}
}
