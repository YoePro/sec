package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// foreignExtentFixture supplies real selected foreign declarations and calls,
// keeping test metadata separate from source-language annotation syntax.
func foreignExtentFixture(t *testing.T, depth AnalysisDepth) (*Analyzer, *ast.Program, []*ast.CallExpression) {
	t.Helper()
	file := "../../testdata/sema/pitfall_foreign_extent_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzerWithDepth(depth)
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	var calls []*ast.CallExpression
	walkASTValue(reflect.ValueOf(program), func(node any) {
		if call, ok := node.(*ast.CallExpression); ok && call != nil {
			calls = append(calls, call)
		}
	})
	return a, program, calls
}

// TestForeignBufferExtentConsumption verifies actual argument binding from
// explicit metadata, all depths, immutable snapshots, multiple buffers/units,
// overload identity, absent metadata, indirect calls, reset and budget isolation.
// Rules: rules/analysis/pitfall_analysis.md — "FFI pitfall analysis",
// "Canonical foreign extent relationships", "Analysis states";
// rules/platform/ffi.md — §§2–3.
func TestForeignBufferExtentConsumption(t *testing.T) {
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		a, program, calls := foreignExtentFixture(t, depth)
		if len(a.PitfallAnalysis().ForeignExtentInputs()) != 0 {
			t.Fatal("names inferred foreign relations")
		}
		var store ForeignBufferExtentContractStore
		source := lexer.Token{File: "trusted-foreign-metadata", Line: 11, Column: 2}
		recorded := map[foreignExtentTarget]bool{}
		for _, call := range calls {
			resolved, ok := a.ResolvedCallTarget(call)
			if !ok || resolved.Kind != ResolvedForeignCall || recorded[foreignExtentTargetOf(resolved.Function)] {
				continue
			}
			fn := resolved.Function
			var relations []ForeignBufferExtentRelation
			switch fn.Name {
			case "Transfer":
				// Only the uint64 overload has metadata; a uint32 overload of the same
				// function name must not receive another declaration's relation.
				if fn.Parameters[1].Type.Name != "uint64" {
					continue
				}
				relations = []ForeignBufferExtentRelation{{PointerArgument: 0, ExtentArgument: 1, ExtentUnit: ForeignExtentBytes, AccessMode: ForeignBufferWrite, Source: source}}
			case "Copy":
				relations = []ForeignBufferExtentRelation{
					{PointerArgument: 0, ExtentArgument: 2, ExtentUnit: ForeignExtentBytes, AccessMode: ForeignBufferWrite, Source: source},
					{PointerArgument: 1, ExtentArgument: 2, ExtentUnit: ForeignExtentElements, AccessMode: ForeignBufferRead, Source: source},
				}
			case "Borrowed":
				relations = []ForeignBufferExtentRelation{{PointerArgument: 0, ExtentArgument: 1, ExtentUnit: ForeignExtentElements, Source: source}}
			default:
				continue
			}
			if err := store.Record(fn, relations); err != nil {
				t.Fatal(err)
			}
			recorded[foreignExtentTargetOf(fn)] = true
			// Record retains no alias to the caller's relation slice.
			relations[0].ExtentUnit = "corrupted"
		}
		a.SetForeignBufferExtentContracts(&store)
		for key := range store.contracts {
			delete(store.contracts, key)
		}
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		inputs := a.PitfallAnalysis().ForeignExtentInputs()
		if len(inputs) != 6 {
			t.Fatalf("depth %v: inputs = %+v, want six", depth, inputs)
		}
		count := 0
		for _, call := range calls {
			facts, known := a.ResolvedForeignBufferExtentsOf(call)
			if !known {
				continue
			}
			for _, fact := range facts {
				count++
				if fact.PointerSource != expressionToken(call.Arguments[fact.PointerArgument]) || fact.ExtentSource != expressionToken(call.Arguments[fact.ExtentArgument]) || fact.ContractSource != source || fact.ExtentUnit == "corrupted" {
					t.Fatal("wrong argument binding", fact)
				}
				if fact.PointerExpression != call.Arguments[fact.PointerArgument].String() || fact.ExtentExpression != call.Arguments[fact.ExtentArgument].String() {
					t.Fatal(fact)
				}
			}
			facts[0].ExtentUnit = "corrupted"
			again, _ := a.ResolvedForeignBufferExtentsOf(call)
			if again[0].ExtentUnit == "corrupted" {
				t.Fatal("mutable canonical facts")
			}
		}
		if count != 6 {
			t.Fatal(count)
		}
		inputs[0].AccessMode = "corrupted"
		if a.PitfallAnalysis().ForeignExtentInputs()[0].AccessMode == "corrupted" {
			t.Fatal("mutable pitfall input snapshot")
		}
		// Facts alone cannot assert provenance/quantity validity or emit findings.
		if len(a.PitfallAnalysis().Findings()) != 0 {
			t.Fatal(a.PitfallAnalysis().Findings())
		}
		if err := a.SetPitfallBudget(0, 0); err != nil {
			t.Fatal(err)
		}
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		if len(a.PitfallAnalysis().ForeignExtentInputs()) != 0 || len(a.resolvedForeignBufferExtents) != 5 {
			t.Fatal("advisory budget changed mandatory foreign facts")
		}
		a.SetForeignBufferExtentContracts(nil)
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		if len(a.resolvedForeignBufferExtents) != 0 {
			t.Fatal("stale call facts survived metadata removal")
		}
	}
}

// TestForeignBufferExtentMetadataValidation rejects unsound or ambiguous
// producer metadata atomically while retaining the last usable contract.
// Rules: rules/analysis/pitfall_analysis.md — "Canonical foreign extent relationships",
// "FFI pitfall analysis"; rules/platform/ffi.md — §§2–3, 45.
func TestForeignBufferExtentMetadataValidation(t *testing.T) {
	a, _, calls := foreignExtentFixture(t, AnalysisStandard)
	var fn Function
	for _, call := range calls {
		if resolved, ok := a.ResolvedCallTarget(call); ok && resolved.Function.Name == "Transfer" {
			fn = resolved.Function
			break
		}
	}
	base := ForeignBufferExtentRelation{PointerArgument: 0, ExtentArgument: 1, ExtentUnit: ForeignExtentBytes, Source: lexer.Token{File: "metadata", Line: 1, Column: 1}}
	var store ForeignBufferExtentContractStore
	if err := store.Record(fn, []ForeignBufferExtentRelation{base}); err != nil {
		t.Fatal(err)
	}
	before := store.contracts[foreignExtentTargetOf(fn)]
	for _, mutate := range []func(*ForeignBufferExtentRelation){
		func(r *ForeignBufferExtentRelation) { r.PointerArgument = -1 },
		func(r *ForeignBufferExtentRelation) { r.ExtentArgument = len(fn.Parameters) },
		func(r *ForeignBufferExtentRelation) { r.ExtentArgument = 0 },
		func(r *ForeignBufferExtentRelation) { r.ExtentUnit = "" },
		func(r *ForeignBufferExtentRelation) { r.ExtentUnit = "words" },
		func(r *ForeignBufferExtentRelation) { r.AccessMode = "execute" },
		func(r *ForeignBufferExtentRelation) { r.Source = lexer.Token{} },
	} {
		bad := base
		mutate(&bad)
		if err := store.Record(fn, []ForeignBufferExtentRelation{bad}); err == nil {
			t.Fatal("accepted", bad)
		}
		if !reflect.DeepEqual(before, store.contracts[foreignExtentTargetOf(fn)]) {
			t.Fatal("failed record changed contract")
		}
	}
	if err := store.Record(fn, []ForeignBufferExtentRelation{base, base}); err == nil {
		t.Fatal("duplicate pair accepted")
	}
	for _, mutate := range []func(*Function){
		func(f *Function) { f.Extern = false },
		func(f *Function) { f.ABI = "invalid" },
		func(f *Function) { f.Token = lexer.Token{} },
		func(f *Function) { f.Parameters[0].Type = builtinTypes()["uint"] },
		func(f *Function) { f.Parameters[1].Type = builtinTypes()["string"] },
		func(f *Function) { f.Parameters[1].Variadic = true },
	} {
		bad := fn
		bad.Parameters = append([]FunctionParameter(nil), fn.Parameters...)
		mutate(&bad)
		if err := store.Record(bad, []ForeignBufferExtentRelation{base}); err == nil {
			t.Fatal("accepted invalid declaration", bad)
		}
	}
}

// TestForeignBufferExtentStaleMetadata keeps metadata bound to the exact
// declaration/signature and rejects shared-write contract contradictions.
// Rules: rules/analysis/pitfall_analysis.md — "FFI pitfall analysis",
// "Canonical foreign extent relationships"; rules/platform/ffi.md — §§2–3.
func TestForeignBufferExtentStaleMetadata(t *testing.T) {
	a, program, calls := foreignExtentFixture(t, AnalysisStandard)
	var original, borrowed Function
	for _, call := range calls {
		if resolved, ok := a.ResolvedCallTarget(call); ok {
			if resolved.Function.Name == "Transfer" && resolved.Function.Parameters[1].Type.Name == "uint64" {
				original = resolved.Function
			}
			if resolved.Function.Name == "Borrowed" {
				borrowed = resolved.Function
			}
		}
	}
	base := ForeignBufferExtentRelation{PointerArgument: 0, ExtentArgument: 1, ExtentUnit: ForeignExtentBytes, AccessMode: ForeignBufferRead, Source: lexer.Token{File: "metadata", Line: 4, Column: 1}}
	for _, mutate := range []func(*Function){
		func(f *Function) { f.Token.File = "different-module.sec" },
		func(f *Function) { f.Token.Column++ },
		func(f *Function) { f.Module += "_other" },
		func(f *Function) { f.Name += "_other" },
		func(f *Function) { f.ABI = "Sec" },
		func(f *Function) { f.LinkName += "_other" },
		func(f *Function) { f.Parameters[1].Type = builtinTypes()["int64"] },
		func(f *Function) { f.ReturnType = builtinTypes()["uint64"] },
	} {
		changed := original
		changed.Parameters = append([]FunctionParameter(nil), original.Parameters...)
		mutate(&changed)
		var store ForeignBufferExtentContractStore
		if err := store.Record(changed, []ForeignBufferExtentRelation{base}); err != nil {
			t.Fatal(err)
		}
		a.SetForeignBufferExtentContracts(&store)
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		if len(a.resolvedForeignBufferExtents) != 0 || len(a.PitfallAnalysis().ForeignExtentInputs()) != 0 {
			t.Fatal("stale metadata rebound to another signature")
		}
	}
	var store ForeignBufferExtentContractStore
	if err := store.Record(borrowed, []ForeignBufferExtentRelation{base}); err != nil {
		t.Fatal(err)
	}
	bad := base
	bad.AccessMode = ForeignBufferWrite
	if err := store.Record(borrowed, []ForeignBufferExtentRelation{bad}); err == nil {
		t.Fatal("shared borrow received write authority")
	}
	if err := store.Record(original, []ForeignBufferExtentRelation{base}); err != nil {
		t.Fatal(err)
	}
	a.SetForeignBufferExtentContracts(&store)
	// Changing a pointee type inside the producer's store cannot mutate the
	// analyzer's detached metadata signature.
	key := foreignExtentTargetOf(original)
	contract := store.contracts[key]
	contract.Signature.Parameters[0].TypeArgs[0].Name = "corrupted"
	store.contracts[key] = contract
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	if len(a.PitfallAnalysis().ForeignExtentInputs()) != 4 {
		t.Fatal("metadata type snapshot leaked", a.PitfallAnalysis().ForeignExtentInputs())
	}
	// A new AST snapshot at the same canonical source locations uses metadata;
	// old call pointers cannot query facts in that new analysis generation.
	file := "../../testdata/sema/pitfall_foreign_extent_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	fresh := parser.New(lexer.NewWithFile(string(data), file)).ParseProgram()
	if errors := a.Analyze(fresh); len(errors) != 0 {
		t.Fatal(errors)
	}
	for _, oldCall := range calls {
		if _, ok := a.ResolvedForeignBufferExtentsOf(oldCall); ok {
			t.Fatal("old call pointer survived reset")
		}
	}
	if len(a.PitfallAnalysis().ForeignExtentInputs()) != 4 {
		t.Fatal("equivalent fresh declaration lost metadata")
	}
}

// TestForeignBufferExtentReachability keeps dead calls out of contract-backed
// pitfall input without suppressing the owning unreachable-code error.
// Rules: rules/analysis/pitfall_analysis.md — "Reachability", "FFI pitfall analysis".
func TestForeignBufferExtentReachability(t *testing.T) {
	file := "../../testdata/sema/pitfall_foreign_extent_unreachable_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) == 0 {
		t.Fatal("invalid fixture lacks owning reachability error")
	}
	var target Function
	walkASTValue(reflect.ValueOf(program), func(node any) {
		if call, ok := node.(*ast.CallExpression); ok && call != nil {
			if resolved, known := a.ResolvedCallTarget(call); known {
				target = resolved.Function
			}
		}
	})
	var store ForeignBufferExtentContractStore
	if err := store.Record(target, []ForeignBufferExtentRelation{{PointerArgument: 0, ExtentArgument: 1, ExtentUnit: ForeignExtentBytes, Source: lexer.Token{File: "metadata", Line: 1, Column: 1}}}); err != nil {
		t.Fatal(err)
	}
	a.SetForeignBufferExtentContracts(&store)
	if errors := a.Analyze(program); len(errors) == 0 {
		t.Fatal("metadata erased owning reachability error")
	}
	if len(a.resolvedForeignBufferExtents) != 0 || len(a.PitfallAnalysis().ForeignExtentInputs()) != 0 {
		t.Fatal("unreachable calls published extent evidence", a.resolvedForeignBufferExtents, a.PitfallAnalysis().ForeignExtentInputs())
	}
}
