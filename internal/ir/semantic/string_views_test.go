package semantic

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// buildStringViewFixture builds the MD-126 fixture through the ordinary
// parser, Sema and Semantic IR builder at the current package boundary.
func buildStringViewFixture(t *testing.T) *Module {
	t.Helper()
	source, err := os.ReadFile("../../../testdata/semantic_ir/string_views.sec")
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(source), "string_views.sec")).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	module, err := Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", SourceFiles: []string{"string_views.sec"}, MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	return module
}

// String ranges build distinct StringView operations: construction from a
// string Place, observed rune extents, shared bounded-view reuse for length,
// copy and reborrow, returned origins and rune-by-value decoding.
// Rules: rules/collections/collections.md — §7.10; rules/compiler/semantic_ir.md — §17(9)–(10);
// rules/control-flow/flowcontrol_for.md — §19.
func TestStringViewsBuildVerifiedOperations(t *testing.T) {
	module := buildStringViewFixture(t)
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		"= string-view", "string.view-borrow %", "string.rune-len %", "slice.range-check %", "slice.range-normalize %",
		"slice.copy-shared %", "slice.reborrow-shared %", "string-view.byte-len %", "string-view.decode-next %",
		`fail.bounds operation="slice-range"`, "returns-from=[param#0[?]]", "returns-from=[param#0[0..<1]]", "range=[1..<3]",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	kinds := map[LoopKind]int{}
	for _, fn := range module.Functions {
		for _, loop := range fn.Loops {
			kinds[loop.Kind]++
		}
	}
	if kinds[LoopForStringView] != 2 {
		t.Fatalf("StringView loops = %d, want 2", kinds[LoopForStringView])
	}
}

// The verifier rejects a view that is not a shared StringView, a view whose
// start/length is not its normalized range tuple, and malformed decoding.
// Rules: rules/collections/collections.md — §7.10; rules/compiler/semantic_ir.md — §17(9).
func TestStringViewsVerifierRejectsMalformedOperations(t *testing.T) {
	cases := map[string]func(*Module) bool{
		"decode result type": func(m *Module) bool {
			return mutateFirst(m, OpStringViewDecodeNext, func(op *Operation) { op.Results[1].Type = op.Results[2].Type })
		},
		"view result type": func(m *Module) bool {
			return mutateFirst(m, OpStringViewBorrow, func(op *Operation) { op.Results[0].Ownership = OwnershipMutableReference })
		},
		"swapped range tuple": func(m *Module) bool {
			return mutateFirst(m, OpStringViewBorrow, func(op *Operation) { op.Operands[1], op.Operands[2] = op.Operands[2], op.Operands[1] })
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			module := buildStringViewFixture(t)
			if !mutate(module) {
				t.Fatal("fixture has no operation to mutate")
			}
			err := Verify(module)
			if err == nil {
				t.Fatal("malformed StringView operation verified")
			}
			t.Log(err)
		})
	}
}

// mutateFirst applies change to the first operation of kind in the module.
func mutateFirst(module *Module, kind OpKind, change func(*Operation)) bool {
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for i := range block.Operations {
				if block.Operations[i].Kind == kind {
					change(&block.Operations[i])
					return true
				}
			}
		}
	}
	return false
}
