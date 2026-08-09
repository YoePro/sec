package sema

import "testing"

func TestEscapeAnalysisSummarizesReturnedParameterBorrow(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Identity(value: ref int) ref int {
    return value
}
`)
	assertSemaErrors(t, errors, nil)

	summary := escapeSummaryNamed(t, analyzer.EscapeAnalysis(), "Identity")
	if summary.Unknown {
		t.Fatal("returned parameter borrow must have known provenance")
	}
	if len(summary.Parameters) != 1 || summary.Parameters[0].Index != 0 || summary.Parameters[0].Name != "value" || !hasEscapeDisposition(summary.Parameters[0].Dispositions, EscapeParameterReturned) {
		t.Fatalf("parameter summary = %#v", summary.Parameters)
	}
	if len(summary.ReturnFacts) != 1 {
		t.Fatalf("return facts = %#v", summary.ReturnFacts)
	}
	fact := summary.ReturnFacts[0]
	if fact.Subject != EscapeSubjectSafeReference || fact.Mode != EscapeModeBorrowEscape || fact.Destination != EscapeDestinationCaller || fact.Unknown {
		t.Fatalf("return fact = %#v", fact)
	}
	if len(fact.Sources) != 1 || fact.Sources[0].Kind != EscapeSourceParameter || fact.Sources[0].ParameterIndex != 0 {
		t.Fatalf("return sources = %#v", fact.Sources)
	}
}

func TestEscapeAnalysisPreservesAggregateCarrierPaths(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Views struct {
    first: ref int,
    second: ref int,
}

fn MakeViews(first: ref int, second: ref int) Views {
    return Views { first: first, second: second }
}
`)
	assertSemaErrors(t, errors, nil)

	summary := escapeSummaryNamed(t, analyzer.EscapeAnalysis(), "MakeViews")
	if len(summary.Parameters) != 2 {
		t.Fatalf("parameter summaries = %#v", summary.Parameters)
	}
	if len(summary.ReturnFacts) != 1 || summary.ReturnFacts[0].Subject != EscapeSubjectStorageDependency {
		t.Fatalf("return facts = %#v", summary.ReturnFacts)
	}
	sources := summary.ReturnFacts[0].Sources
	if !hasEscapeSource(sources, EscapeSourceParameter, 0, ".first") || !hasEscapeSource(sources, EscapeSourceParameter, 1, ".second") {
		t.Fatalf("aggregate sources = %#v", sources)
	}
}

func TestEscapeAnalysisRecordsCaptureWithoutLambdaReturnLeakage(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Apply(factor: int) int {
    let multiply := capture(factor) fn(value: int) int {
        return value * factor
    }
    return multiply(2)
}
`)
	assertSemaErrors(t, errors, nil)

	summary := escapeSummaryNamed(t, analyzer.EscapeAnalysis(), "Apply")
	if len(summary.CaptureFacts) != 1 {
		t.Fatalf("capture facts = %#v", summary.CaptureFacts)
	}
	capture := summary.CaptureFacts[0]
	if capture.Mode != EscapeModeCapture || capture.Destination != EscapeDestinationClosureEnvironment || len(capture.Sources) != 1 || capture.Sources[0].Kind != EscapeSourceParameter {
		t.Fatalf("capture fact = %#v", capture)
	}
	if len(summary.ReturnFacts) != 1 {
		t.Fatalf("lambda return leaked into enclosing summary: %#v", summary.ReturnFacts)
	}
	if len(summary.Parameters) != 1 || !hasEscapeDisposition(summary.Parameters[0].Dispositions, EscapeParameterCaptured) {
		t.Fatalf("parameter summary = %#v", summary.Parameters)
	}
}

func TestEscapeAnalysisRetainsRejectedLocalBorrowEscape(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Invalid() ref int {
    let value := 10
    return ref value
}
`)
	if len(errors) != 1 {
		t.Fatalf("errors = %#v", errors)
	}

	summary := escapeSummaryNamed(t, analyzer.EscapeAnalysis(), "Invalid")
	if !summary.Unknown || len(summary.ReturnFacts) != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	fact := summary.ReturnFacts[0]
	if fact.Unknown || len(fact.Sources) != 1 || fact.Sources[0].Kind != EscapeSourceLocal || fact.Sources[0].Name != "value" {
		t.Fatalf("local escape fact = %#v", fact)
	}
}

func TestEscapeAnalysisRecordsOuterPlaceEscape(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Holder struct {
    view: ref int,
}

fn Invalid(target: ref mut Holder) void {
    let value := 10
    target.view = ref value
}
`)
	if len(errors) != 1 {
		t.Fatalf("errors = %#v", errors)
	}

	summary := escapeSummaryNamed(t, analyzer.EscapeAnalysis(), "Invalid")
	facts := analyzer.EscapeAnalysis().Facts()
	if len(facts) != 1 || facts[0].Destination != EscapeDestinationOuterPlace || facts[0].Mode != EscapeModeBorrowEscape {
		t.Fatalf("escape facts = %#v", facts)
	}
	if len(facts[0].Sources) != 1 || facts[0].Sources[0].Kind != EscapeSourceLocal || facts[0].Sources[0].Name != "value" {
		t.Fatalf("outer-place sources = %#v", facts[0].Sources)
	}
	if summary.Unknown {
		t.Fatalf("known local source unexpectedly widened summary: %#v", summary)
	}
}

func TestEscapeAnalysisRecordsResultPayloadReturns(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn ReturnValue(value: ref int) Result[ref int, string] {
    return Ok(value)
}

fn ReturnError(message: string) Result[int, string] {
    return Err(message)
}
`)
	assertSemaErrors(t, errors, nil)

	valueSummary := escapeSummaryNamed(t, analyzer.EscapeAnalysis(), "ReturnValue")
	if len(valueSummary.ReturnFacts) != 1 || valueSummary.ReturnFacts[0].Subject != EscapeSubjectSafeReference || len(valueSummary.Parameters) != 1 {
		t.Fatalf("value summary = %#v", valueSummary)
	}
	errorSummary := escapeSummaryNamed(t, analyzer.EscapeAnalysis(), "ReturnError")
	if len(errorSummary.ReturnFacts) != 1 || errorSummary.ReturnFacts[0].Subject != EscapeSubjectOwnedValue || len(errorSummary.Parameters) != 1 {
		t.Fatalf("error summary = %#v", errorSummary)
	}
}

func TestEscapeAnalysisSummarizesReturnedReceiverBorrow(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Holder struct {
    value: int,
}

impl Holder {
    fn Value() ref int {
        return ref self.value
    }
}
`)
	assertSemaErrors(t, errors, nil)

	summary := escapeSummaryNamed(t, analyzer.EscapeAnalysis(), "Holder.Value")
	if !hasEscapeDisposition(summary.Receiver, EscapeParameterReturned) {
		t.Fatalf("receiver summary = %#v", summary.Receiver)
	}
	if len(summary.ReturnFacts) != 1 || len(summary.ReturnFacts[0].Sources) != 1 || summary.ReturnFacts[0].Sources[0].Kind != EscapeSourceReceiver {
		t.Fatalf("receiver return facts = %#v", summary.ReturnFacts)
	}
}

func TestEscapeAnalysisSnapshotIsDefensive(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Identity(value: ref int) ref int {
    return value
}
`)
	assertSemaErrors(t, errors, nil)

	snapshot := analyzer.EscapeAnalysis()
	facts := snapshot.Facts()
	if len(facts) != 1 || len(facts[0].Sources) != 1 {
		t.Fatalf("facts = %#v", facts)
	}
	facts[0].Sources[0].Name = "changed"
	facts[0].Sources[0].Place.Root = "changed"

	again := snapshot.Facts()
	if again[0].Sources[0].Name != "value" || again[0].Sources[0].Place.Root == "changed" {
		t.Fatalf("snapshot was mutated through query result: %#v", again[0].Sources[0])
	}
}

func escapeSummaryNamed(t *testing.T, analysis *EscapeAnalysis, name string) EscapeCallableSummary {
	t.Helper()
	for _, summary := range analysis.Summaries() {
		if summary.Name == name {
			return summary
		}
	}
	t.Fatalf("missing escape summary for %s; summaries=%#v", name, analysis.Summaries())
	return EscapeCallableSummary{}
}

func hasEscapeDisposition(dispositions []EscapeParameterDisposition, want EscapeParameterDisposition) bool {
	for _, disposition := range dispositions {
		if disposition == want {
			return true
		}
	}
	return false
}

func hasEscapeSource(sources []EscapeSource, kind EscapeSourceKind, parameterIndex int, carrierPath string) bool {
	for _, source := range sources {
		if source.Kind == kind && source.ParameterIndex == parameterIndex && source.CarrierPath == carrierPath {
			return true
		}
	}
	return false
}
