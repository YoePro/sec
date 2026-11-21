package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

type cAliasSource struct {
	file, text string
	layer      CBindingLayer // zero: not a binding file
}

func analyzeCAliasSources(t *testing.T, sources ...cAliasSource) (*Analyzer, []Error) {
	t.Helper()
	program := &ast.Program{}
	surface := CBindingSurface{Identity: "sec.test@1.0.0/linux-amd64-glibc", Files: map[string]CBindingLayer{}}
	for _, source := range sources {
		p := parser.New(lexer.NewWithFile(source.text, source.file))
		result := p.Parse()
		if result.HasErrors {
			t.Fatalf("parse %s: %v", source.file, p.Errors())
		}
		program.Statements = append(program.Statements, result.Program.Statements...)
		if source.layer != 0 {
			surface.Files[source.file] = source.layer
		}
	}
	model := layout.CModelSysVX8664
	analyzer := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: model.PointerWidthBits, CABI: model})
	analyzer.SetCBindingSurface(surface)
	return analyzer, analyzer.Analyze(program)
}

const cAliasUser = `module main

extern "C" fn Allocate(size: c::stddef::size_t) RawPtr[c::char]

fn Use(n: uint64) void {
    let size: c::stddef::size_t := 16
    let text: c::example::pointer := unsafe Allocate(size)
    discard text
    discard n
}
`

// MD-021 §§ 3, 8.6: a selected binding module's aliases are transparent:
// a use resolves to the target type itself, in declarations and extern
// signatures, and no nominal identity is introduced.
func TestCAliasResolvesTransparently(t *testing.T) {
	analyzer, errs := analyzeCAliasSources(t,
		cAliasSource{"bindings/common/stddef.sec", "module common\n\n@cAlias(name: \"c::stddef::size_t\", target: c::ulong)\n@cAlias(name: \"c::example::pointer\", target: RawPtr[c::char])\n", CBindingCommon},
		cAliasSource{"main.sec", cAliasUser, 0},
	)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	facts := analyzer.CAliasFacts()
	if len(facts) != 2 || facts[0].Name != "c::example::pointer" || facts[0].Target.Kind != RawPtrType ||
		facts[1].Name != "c::stddef::size_t" || facts[1].Target.Name != "c::ulong" || facts[1].Layer != CBindingCommon {
		t.Fatalf("facts = %+v", facts)
	}
	// Transparency: the alias is c::ulong, so a Sec integer still needs an
	// explicit conversion exactly as for c::ulong.
	_, errs = analyzeCAliasSources(t,
		cAliasSource{"bindings/common/stddef.sec", "module common\n\n@cAlias(name: \"c::stddef::size_t\", target: c::ulong)\n", CBindingCommon},
		cAliasSource{"main.sec", "module main\n\nfn Use(n: uint64) void {\n    let size: c::stddef::size_t := n\n    discard size\n}\n", 0},
	)
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "cannot initialize c::ulong with uint64") {
		t.Fatalf("transparent identity: %v", errs)
	}
}

// MD-021 § 8.6(2): one environment definition specializes one common
// definition; chains resolve after specialization.
func TestCAliasEnvironmentSpecializesCommon(t *testing.T) {
	analyzer, errs := analyzeCAliasSources(t,
		cAliasSource{"bindings/common/types.sec", "module common\n\n@cAlias(name: \"c::stddef::size_t\", target: c::uint)\n@cAlias(name: \"c::stddef::ssize_t\", target: c::stddef::ptrdiff_t)\n", CBindingCommon},
		cAliasSource{"bindings/amd64/types.sec", "module amd64\n\n@cAlias(name: \"c::stddef::size_t\", target: c::ulong)\n@cAlias(name: \"c::stddef::ptrdiff_t\", target: c::long)\n", CBindingEnvironment},
	)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	got := map[string]string{}
	for _, fact := range analyzer.CAliasFacts() {
		got[fact.Name] = fact.Target.Name + "/" + fact.Layer.String()
	}
	want := map[string]string{"c::stddef::size_t": "c::ulong/environment", "c::stddef::ptrdiff_t": "c::long/environment", "c::stddef::ssize_t": "c::long/common"}
	for name, target := range want {
		if got[name] != target {
			t.Fatalf("%s = %q, want %q (all %v)", name, got[name], target, got)
		}
	}
}

// MD-021 §§ 3(4), 8.6(2)-(4) and attributes.md § 95(1): every rejection
// carries its own registered diagnostic.
func TestCAliasRejections(t *testing.T) {
	cases := []struct {
		name    string
		sources []cAliasSource
		id      string
		message string
	}{
		{"outside binding module", []cAliasSource{{"main.sec", "module main\n\n@cAlias(name: \"c::stddef::size_t\", target: c::ulong)\n", 0}},
			diagnostics.CAliasOutsideBindingModule, "not in a module of the selected C binding environment"},
		{"fundamental name", []cAliasSource{{"b/common.sec", "module common\n\n@cAlias(name: \"c::int\", target: c::long)\n", CBindingCommon}},
			diagnostics.CAliasInvalidName, "compiler-owned fundamental C namespace"},
		{"not c::", []cAliasSource{{"b/common.sec", "module common\n\n@cAlias(name: \"stddef::size_t\", target: c::long)\n", CBindingCommon}},
			diagnostics.CAliasInvalidName, "must start with c::"},
		{"bad segment", []cAliasSource{{"b/common.sec", "module common\n\n@cAlias(name: \"c::std def::x\", target: c::long)\n", CBindingCommon}},
			diagnostics.CAliasInvalidName, "non-identifier segment"},
		{"same-layer duplicate", []cAliasSource{
			{"b/one.sec", "module common\n\n@cAlias(name: \"c::stddef::size_t\", target: c::ulong)\n", CBindingCommon},
			{"b/two.sec", "module common\n\n@cAlias(name: \"c::stddef::size_t\", target: c::ulong)\n", CBindingCommon}},
			diagnostics.CAliasDuplicate, "declared twice on the common layer"},
		{"environment duplicate", []cAliasSource{
			{"b/one.sec", "module amd64\n\n@cAlias(name: \"c::stddef::size_t\", target: c::ulong)\n@cAlias(name: \"c::stddef::size_t\", target: c::uint)\n", CBindingEnvironment}},
			diagnostics.CAliasDuplicate, "declared twice on the environment layer"},
		{"cycle", []cAliasSource{{"b/common.sec", "module common\n\n@cAlias(name: \"c::a::x\", target: c::a::y)\n@cAlias(name: \"c::a::y\", target: c::a::x)\n", CBindingCommon}},
			diagnostics.CAliasCycle, "circular alias chain"},
		{"illegal target", []cAliasSource{{"b/common.sec", "module common\n\n@cAlias(name: \"c::a::text\", target: string)\n", CBindingCommon}},
			diagnostics.CAliasIllegalTarget, "cannot cross a C ABI boundary"},
		{"unknown target", []cAliasSource{{"b/common.sec", "module common\n\n@cAlias(name: \"c::a::x\", target: c::a::missing)\n", CBindingCommon}},
			diagnostics.ForeignUnresolvedCBindingType, "no C binding environment defines it"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, errs := analyzeCAliasSources(t, test.sources...)
			for _, err := range errs {
				if err.ID == test.id && strings.Contains(err.Message, test.message) {
					return
				}
			}
			t.Fatalf("want %s %q, got %v", test.id, test.message, errs)
		})
	}
}

// ffi.md § 56(2): without a selected environment every binding name stays
// S1078, and a use of an alias that failed reports no dependent error.
func TestCAliasWithoutSurfaceKeepsS1078(t *testing.T) {
	_, errs := analyzeCAliasSources(t, cAliasSource{"main.sec", "module main\n\nfn Use() void {\n    let size: c::stddef::size_t := 1\n    discard size\n}\n", 0})
	if len(errs) != 1 || errs[0].ID != diagnostics.ForeignUnresolvedCBindingType {
		t.Fatalf("errors = %v", errs)
	}
	_, errs = analyzeCAliasSources(t,
		cAliasSource{"b/common.sec", "module common\n\n@cAlias(name: \"c::a::text\", target: string)\n", CBindingCommon},
		cAliasSource{"main.sec", "module main\n\nfn Use() void {\n    let t: c::a::text := \"x\"\n    discard t\n}\n", 0})
	if len(errs) != 1 || errs[0].ID != diagnostics.CAliasIllegalTarget {
		t.Fatalf("dependent errors after a failed alias: %v", errs)
	}
}
