package semantic

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// buildPackage16Source analyzes source for a target pointer width and builds
// package-16 Semantic IR.
func buildPackage16Source(t *testing.T, source string, width uint16) (*Module, error) {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, "sample.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
	if errs := analyzer.Analyze(parsed.Program); len(errs) != 0 {
		return nil, fmt.Errorf("sema: %v", errs)
	}
	return Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", SourceFiles: []string{"sample.sec"}, MaxPackage: 16})
}

// P16-36 (package16 §§ 39–41, 64–65): slice parameters are bounded
// references with the caller's borrow; a shared holder is passed with
// slice.copy-shared under its own borrow, a mutable holder with an implicit
// full-range slice.reborrow-mut under a new borrow derived from it, so the
// caller keeps its holder, and a fresh `ref data[..]` is built in place.
func TestPackage16LowersSliceParameters(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		source, err := os.ReadFile("../../../testdata/semantic_ir/slice_source_parameters.sec")
		if err != nil {
			t.Fatal(err)
		}
		m, err := buildPackage16Source(t, string(source), width)
		if err != nil {
			t.Fatal(err)
		}
		text := Format(m)
		body := func(name string) string {
			start := strings.Index(text, `func "main::`+name)
			if start < 0 {
				t.Fatalf("missing %s in:\n%s", name, text)
			}
			rest := text[start:]
			return rest[:strings.Index(rest, "\n  }")]
		}
		for name, want := range map[string]map[string]int{
			"Fill(ref mut int[])":    {"[mutable-reference] {borrow=": 1, "slice.element-place": 1},
			"Forward(ref mut int[])": {"slice.reborrow-mut": 2, "proof=source-open-bound": 2, "[mutable-reference])": 2},
			"Peek(ref int[])":        {"slice.copy-shared": 1, "[shared-reference])": 1},
			"main()":                 {"slice.reborrow-mut": 2, "slice.copy-shared": 1, "slice.borrow-shared": 2, "slice.borrow-mut": 1},
		} {
			got := body(name)
			for operation, count := range want {
				if strings.Count(got, operation) != count {
					t.Fatalf("%s: %q x%d, want %d:\n%s", name, operation, strings.Count(got, operation), count, got)
				}
			}
		}
		// Each implicit reborrow takes a fresh borrow derived from the holder.
		forward := body("Forward(ref mut int[])")
		if strings.Count(forward, "from=#") != 2 || strings.Contains(forward, "slice.move-mut") {
			t.Fatalf("mutable slice arguments must be reborrowed, never moved or copied:\n%s", forward)
		}
	}
}

// P16-36 (package16 §§ 39–41): copying a shared slice holder is one
// slice.copy-shared under the same borrow; moving a mutable holder is one
// slice.move-mut; neither copies elements nor creates a new borrow.
func TestPackage16LowersSliceHolderTransfers(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/slice_source_transfers.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		m, err := buildPackage16Source(t, string(source), width)
		if err != nil {
			t.Fatal(err)
		}
		text := Format(m)
		copyFn := text[strings.Index(text, `func "main::Copy(`):]
		copyFn = copyFn[:strings.Index(copyFn, "\n  }")]
		moveFn := text[strings.Index(text, `func "main::Move()"`):]
		moveFn = moveFn[:strings.Index(moveFn, "\n  }")]
		if strings.Count(copyFn, "slice.copy-shared") != 1 || strings.Contains(copyFn, "slice.move-mut") || strings.Contains(copyFn, "array.extract") {
			t.Fatalf("shared holder copy:\n%s", copyFn)
		}
		if strings.Count(moveFn, "slice.move-mut") != 1 || strings.Contains(moveFn, "slice.copy-shared") || strings.Count(moveFn, "slice.borrow-mut") != 1 {
			t.Fatalf("mutable holder move:\n%s", moveFn)
		}
		for name, kind := range map[string]string{"Reborrow(": "slice.reborrow-shared", "ReborrowMutable()": "slice.reborrow-mut"} {
			body := text[strings.Index(text, `func "main::`+name):]
			body = body[:strings.Index(body, "\n  }")]
			if strings.Count(body, kind) != 1 || !strings.Contains(body, "proof=source-open-bound") || !strings.Contains(body, "from=#") {
				t.Fatalf("%s must reborrow the whole holder with %s:\n%s", name, kind, body)
			}
		}
		for _, line := range strings.Split(copyFn+"\n"+moveFn, "\n") {
			if strings.Contains(line, "slice.copy-shared") || strings.Contains(line, "slice.move-mut") {
				// The transferred holder keeps its source borrow.
				var borrow, from string
				fmt.Sscanf(line[strings.Index(line, "{borrow=#"):], "{borrow=#%s", &borrow)
				if index := strings.Index(line, "from=#"); index >= 0 {
					fmt.Sscanf(line[index:], "from=#%s", &from)
				}
				if borrow == "" || from != borrow {
					t.Fatalf("transfer changed its borrow: %s", line)
				}
			}
		}
	}
}

// P16-36 (package16 § 66): a slice derived from a slice parameter may be
// returned — the holder, a copy, a reborrow of its complete extent or a
// call result — with a return-origin summary naming the parameter; callers
// receive the facts instantiated from it.
func TestPackage16LowersReturnedSlices(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/slice_source_returns.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		m, err := buildPackage16Source(t, string(source), width)
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(m); err != nil {
			t.Fatal(err)
		}
		text := Format(m)
		for _, name := range []string{"Same(ref int[])", "Whole(ref mut int[])", "Copied(ref int[])", "Chain(ref int[])"} {
			header := text[strings.Index(text, `func "main::`+name):]
			header = header[:strings.Index(header, "\n")]
			if !strings.Contains(header, "returns-from=[param#0]") {
				t.Fatalf("%s lacks its return-origin summary: %s", name, header)
			}
		}
		main := text[strings.Index(text, `func "main::main()"`):]
		for _, call := range []string{`call.direct "main::Chain(ref int[])"`, `call.direct "main::Whole(ref mut int[])"`} {
			line := main[strings.Index(main, call):]
			line = line[:strings.Index(line, "\n")]
			if !strings.Contains(line, "{borrow=#") || !strings.Contains(line, "-reference]") {
				t.Fatalf("call result lacks instantiated slice facts: %s", line)
			}
		}
	}
	// MD-115 supplies the schema-12 range projection.
	_, err = buildPackage16Source(t, `
module main

fn Window(values: ref int[]) ref int[] {
    return ref values[1..<2]
}
`, 64)
	if err != nil {
		t.Fatalf("sub-slice return = %v", err)
	}
}
