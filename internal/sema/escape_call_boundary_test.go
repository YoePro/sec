package sema

import (
	"reflect"
	"testing"
)

const escapeCallBoundarySource = `
module main

@noCopy
type Resource struct {
    id: int,
}

fn Inspect(value: ref Resource) int {
    return value.id
}

fn Modify(value: ref mut Resource) void {
    value.id = 1
}

fn Consume(-> value: Resource) int {
    return value.id
}

fn Twice(-> value: int) int {
    return value * 2
}

fn Add(left: int, right: int) int {
    return left + right
}

fn Forward(-> owned: Resource, number: int) int {
    let mut local := Resource { id: number }
    discard Inspect(local)
    Modify(local)
    discard Add(number, 3)
    discard Twice(<-number)
    discard Consume(Resource { id: 2 })
    discard Consume(<-local)
    return Consume(<-owned)
}
`

// Calls to resolved callees record the resolved semantic relation per
// argument: borrow, mutable borrow, copy, or ownership transfer. Consuming
// transfers are ValueTransfer escapes and mark transferred caller parameters
// OwnershipTransferred; borrows of Sec callees record no retention.
func TestEscapeAnalysisRecordsCallBoundaryRelations(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, escapeCallBoundarySource)
	assertSemaErrors(t, errors, nil)
	escape := analyzer.EscapeAnalysis()

	type observed struct {
		callee   string
		index    int
		relation EscapeCallRelation
		source   EscapeSourceKind
		name     string
	}
	var got []observed
	for _, argument := range escape.CallArguments() {
		if len(argument.Sources) != 1 {
			t.Fatalf("argument sources = %#v", argument.Sources)
		}
		got = append(got, observed{argument.CalleeName, argument.ParameterIndex, argument.Relation, argument.Sources[0].Kind, argument.Sources[0].Name})
	}
	want := []observed{
		{"Inspect", 0, EscapeCallBorrow, EscapeSourceLocal, "local"},
		{"Modify", 0, EscapeCallMutableBorrow, EscapeSourceLocal, "local"},
		{"Add", 0, EscapeCallCopy, EscapeSourceParameter, "number"},
		{"Add", 1, EscapeCallCopy, EscapeSourceTemporary, ""},
		{"Twice", 0, EscapeCallOwnershipTransfer, EscapeSourceParameter, "number"},
		{"Consume", 0, EscapeCallOwnershipTransfer, EscapeSourceTemporary, ""},
		{"Consume", 0, EscapeCallOwnershipTransfer, EscapeSourceLocal, "local"},
		{"Consume", 0, EscapeCallOwnershipTransfer, EscapeSourceParameter, "owned"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("call arguments =\n%#v\nwant\n%#v", got, want)
	}

	transfers := 0
	for _, fact := range escape.Facts() {
		if fact.Mode == EscapeModeRetention || fact.Mode == EscapeModeBorrowEscape && fact.Destination == EscapeDestinationRetainingCall {
			t.Fatalf("borrowed Sec parameter recorded retention: %#v", fact)
		}
		if fact.Mode != EscapeModeValueTransfer || fact.Destination != EscapeDestinationRetainingCall {
			continue
		}
		transfers++
	}
	if transfers != 4 {
		t.Fatalf("ownership-transfer facts = %d, want 4", transfers)
	}

	summary := escapeSummaryNamed(t, escape, "Forward")
	dispositions := map[string][]EscapeParameterDisposition{}
	for _, parameter := range summary.Parameters {
		dispositions[parameter.Name] = parameter.Dispositions
	}
	if !hasEscapeDisposition(dispositions["owned"], EscapeParameterOwnershipTransferred) || !hasEscapeDisposition(dispositions["number"], EscapeParameterOwnershipTransferred) {
		t.Fatalf("Forward parameter dispositions = %#v", summary.Parameters)
	}
	for _, name := range []string{"Inspect", "Modify", "Add"} {
		if callee, ok := escapeSummaryLookup(escape, name); ok && len(callee.Parameters) != 0 {
			t.Fatalf("%s parameters gained dispositions from its callers: %#v", name, callee.Parameters)
		}
	}
}

// Re-inferring the same call does not duplicate its call-boundary facts, and
// snapshots are detached from the analyzer state.
func TestEscapeCallBoundaryFactsAreDeduplicatedAndDetached(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, escapeCallBoundarySource)
	assertSemaErrors(t, errors, nil)
	first := analyzer.EscapeAnalysis().CallArguments()
	first[0].Sources[0].Name = "mutated"
	second := analyzer.EscapeAnalysis().CallArguments()
	if len(first) != len(second) || second[0].Sources[0].Name != "local" {
		t.Fatalf("call-argument snapshot is not detached: %#v", second[0])
	}
	seen := map[escapeCallArgumentKey]bool{}
	for _, argument := range second {
		key := escapeCallArgumentKey{argument.Caller, argument.Sink.File, argument.Sink.Line, argument.Sink.Column, argument.ParameterIndex}
		if seen[key] {
			t.Fatalf("duplicate call-boundary fact %#v", argument)
		}
		seen[key] = true
	}
}

func escapeSummaryLookup(escape *EscapeAnalysis, name string) (EscapeCallableSummary, bool) {
	for _, summary := range escape.Summaries() {
		if summary.Name == name {
			return summary, true
		}
	}
	return EscapeCallableSummary{}, false
}

const escapeFunctionValueCallSource = `
module main

@noCopy
type Resource struct {
    id: int,
}

fn Inspect(value: ref Resource) int {
    return value.id
}

fn Apply(callback: fn(ref Resource) int, value: ref Resource) int {
    return callback(value)
}

fn Run(consume: fn(Resource) int, -> owned: Resource) int {
    return consume(<-owned)
}

fn Known(number: int) int {
    let local := Resource { id: number }
    let inspect := Inspect
    let double := fn(input: int) int {
        return input * 2
    }
    return inspect(local) + double(number)
}
`

// Calls through function values compose call-boundary facts from the
// function type and the proven target set: closed sets of Sec bodies keep the
// NoRetention contract, while open or unresolved targets are conservative
// UnknownRetention for borrowed and reference-like arguments.
func TestEscapeAnalysisComposesFunctionValueCalls(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, escapeFunctionValueCallSource)
	assertSemaErrors(t, errors, nil)
	escape := analyzer.EscapeAnalysis()

	byCallee := map[string]EscapeCallArgument{}
	for _, argument := range escape.CallArguments() {
		if argument.Callee == "" {
			byCallee[argument.CalleeName] = argument
		}
	}
	if got := byCallee["callback"]; got.Relation != EscapeCallBorrow || !got.UnknownTargets || len(got.Targets) != 0 || got.Sources[0].Kind != EscapeSourceParameter {
		t.Fatalf("unknown-target borrow = %#v", got)
	}
	if got := byCallee["consume"]; got.Relation != EscapeCallOwnershipTransfer || !got.UnknownTargets || got.Sources[0].Name != "owned" {
		t.Fatalf("unknown-target transfer = %#v", got)
	}
	if got := byCallee["inspect"]; got.Relation != EscapeCallBorrow || got.UnknownTargets || len(got.Targets) != 1 || got.Sources[0].Name != "local" {
		t.Fatalf("named-target borrow = %#v", got)
	}
	if got := byCallee["double"]; got.Relation != EscapeCallCopy || got.UnknownTargets || len(got.Targets) != 1 {
		t.Fatalf("lambda-target copy = %#v", got)
	}

	retentions := 0
	for _, fact := range escape.Facts() {
		if fact.Mode != EscapeModeRetention {
			continue
		}
		retentions++
		if fact.Destination != EscapeDestinationUnknown || !fact.Unknown || fact.Subject != EscapeSubjectSafeReference || len(fact.Sources) != 1 || fact.Sources[0].Name != "value" {
			t.Fatalf("unknown retention fact = %#v", fact)
		}
	}
	if retentions != 1 {
		t.Fatalf("retention facts = %d, want only the borrow passed to the unknown callback", retentions)
	}

	apply := escapeSummaryNamed(t, escape, "Apply")
	if len(apply.Parameters) != 1 || apply.Parameters[0].Name != "value" || !hasEscapeDisposition(apply.Parameters[0].Dispositions, EscapeParameterUnknownRetention) {
		t.Fatalf("Apply parameters = %#v", apply.Parameters)
	}
	run := escapeSummaryNamed(t, escape, "Run")
	if len(run.Parameters) != 1 || run.Parameters[0].Name != "owned" || !hasEscapeDisposition(run.Parameters[0].Dispositions, EscapeParameterOwnershipTransferred) || hasEscapeDisposition(run.Parameters[0].Dispositions, EscapeParameterUnknownRetention) {
		t.Fatalf("Run parameters = %#v", run.Parameters)
	}
	if known, ok := escapeSummaryLookup(escape, "Known"); ok && len(known.Parameters) != 0 {
		t.Fatalf("known Sec targets produced dispositions: %#v", known.Parameters)
	}
}

const escapeForeignCallSource = `
module main

extern "C" fn Inspect(value: ref int32) int32

extern "C" fn Modify(value: ref mut int32) int32

unsafe extern "C" fn Keep(value: RawPtr[int32]) void

extern "C" fn Abs(value: int32) int32

fn Use(raw: RawPtr[int32], number: int32, shared: ref int32) int32 {
    let mut local := number
    discard Inspect(shared)
    discard Modify(local)
    unsafe {
        Keep(raw)
    }
    return Abs(number)
}
`

// Extern calls consume the FFI parameter contracts: ref T and ref mut T are
// call-bounded borrows foreign code must not retain, while raw pointers and
// values carrying one establish no retention and are conservative unknown
// foreign retention; address-free values retain nothing.
func TestEscapeAnalysisConsumesForeignRetentionContracts(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, escapeForeignCallSource)
	assertSemaErrors(t, errors, nil)
	escape := analyzer.EscapeAnalysis()

	relations := map[string]EscapeCallRelation{}
	for _, argument := range escape.CallArguments() {
		if !argument.Foreign {
			t.Fatalf("extern call argument not marked foreign: %#v", argument)
		}
		relations[argument.CalleeName] = argument.Relation
	}
	want := map[string]EscapeCallRelation{"Inspect": EscapeCallBorrow, "Modify": EscapeCallMutableBorrow, "Keep": EscapeCallCopy, "Abs": EscapeCallCopy}
	if !reflect.DeepEqual(relations, want) {
		t.Fatalf("foreign relations = %v, want %v", relations, want)
	}

	retained := map[string]EscapeSubjectKind{}
	for _, fact := range escape.Facts() {
		if fact.Mode != EscapeModeForeignRetention {
			continue
		}
		if fact.Destination != EscapeDestinationForeignCode || !fact.Unknown || len(fact.Sources) != 1 {
			t.Fatalf("foreign retention fact = %#v", fact)
		}
		retained[fact.Sources[0].Name] = fact.Subject
	}
	if want := map[string]EscapeSubjectKind{"raw": EscapeSubjectRawPointer}; !reflect.DeepEqual(retained, want) {
		t.Fatalf("foreign retention sources = %v, want %v", retained, want)
	}

	dispositions := map[string][]EscapeParameterDisposition{}
	for _, parameter := range escapeSummaryNamed(t, escape, "Use").Parameters {
		dispositions[parameter.Name] = parameter.Dispositions
	}
	if !hasEscapeDisposition(dispositions["raw"], EscapeParameterPassedToForeign) || !hasEscapeDisposition(dispositions["raw"], EscapeParameterUnknownRetention) {
		t.Fatalf("raw dispositions = %v", dispositions["raw"])
	}
	for _, name := range []string{"number", "shared"} {
		if len(dispositions[name]) != 0 {
			t.Fatalf("%s gained dispositions %v", name, dispositions[name])
		}
	}
}

// Raw pointers and callables stored in aggregates, arrays and generic
// arguments carry an address into foreign code (ffi.md § 14); address-free
// aggregates and recursive declarations terminate.
func TestTypeContainsAddress(t *testing.T) {
	raw := Type{Kind: RawPtrType, Name: "RawPtr[int32]"}
	scalar := Type{Kind: IntType, Name: "int32"}
	withPointer := Type{Kind: StructType, Name: "Node", Fields: []StructField{{Name: "value", Type: scalar}, {Name: "next", Type: raw}}}
	plain := Type{Kind: StructType, Name: "Plain", Fields: []StructField{{Name: "value", Type: scalar}}}
	callbacks := Type{Kind: ArrayType, Name: "fn()[]", Element: &Type{Kind: FunctionType, Name: "fn() void"}}
	for _, test := range []struct {
		typ  Type
		want bool
	}{{raw, true}, {withPointer, true}, {callbacks, true}, {plain, false}, {scalar, false}} {
		if got := typeContainsAddress(test.typ, map[string]bool{}); got != test.want {
			t.Errorf("typeContainsAddress(%s) = %t, want %t", test.typ.Name, got, test.want)
		}
	}
	if !foreignArgumentMayBeRetained(EscapeCallCopy, withPointer) || foreignArgumentMayBeRetained(EscapeCallBorrow, raw) || foreignArgumentMayBeRetained(EscapeCallCopy, plain) {
		t.Fatal("foreign retention classification ignores the FFI parameter contract")
	}
}
