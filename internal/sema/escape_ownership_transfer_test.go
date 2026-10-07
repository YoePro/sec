package sema

import "testing"

// Revision 2.0 return and borrow-call regressions: escape facts follow the
// resolved return and call contract, never the presence of a `<-` token.
//
// Rules:
//   - rules/analysis/escape_analysis.md — "Core rule", "Returned owned values", "Call-boundary semantic classification"
//   - rules/analysis/escape_analysis.md — "Revision 2.0 ownership and borrow-call test requirements"
func TestEscapeAnalysisReturnTransferFollowsResolvedSemantics(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

@noCopy
type Buffer struct {
    data: int[8],
}

fn Plain(-> value: Buffer) Buffer {
    return value
}

fn Marked(-> value: Buffer) Buffer {
    return <-value
}

fn MakeLocal() Buffer {
    let local := Buffer{}
    return local
}

fn CopyThroughBorrow(value: ref Buffer) int {
    return value.data[0]
}

fn Inspect(value: ref Buffer) int {
    return 0
}

fn Modify(value: ref mut Buffer) void {
}

fn Caller(-> owned: Buffer) void {
    discard Inspect(owned)
    let mut local := Buffer{}
    Modify(local)
}
`)
	assertSemaErrors(t, errors, nil)
	analysis := analyzer.EscapeAnalysis()

	// `return value` and the optional `return <-value` are the same
	// ValueTransfer of the parameter.
	plain, marked := escapeSummaryNamed(t, analysis, "Plain"), escapeSummaryNamed(t, analysis, "Marked")
	for _, summary := range []EscapeCallableSummary{plain, marked} {
		if len(summary.Parameters) != 1 || !hasEscapeDisposition(summary.Parameters[0].Dispositions, EscapeParameterReturned) {
			t.Fatalf("%s parameter summary = %#v", summary.Name, summary.Parameters)
		}
		if len(summary.ReturnFacts) != 1 || summary.ReturnFacts[0].Mode != plain.ReturnFacts[0].Mode ||
			summary.ReturnFacts[0].Subject != EscapeSubjectOwnedValue {
			t.Fatalf("%s return facts = %#v", summary.Name, summary.ReturnFacts)
		}
	}

	// An owned local return transfers the value without a parameter or
	// storage dependency.
	if local := escapeSummaryNamed(t, analysis, "MakeLocal"); len(local.Parameters) != 0 || local.Unknown {
		t.Fatalf("local return summary = %#v", local)
	}

	// A value read through a borrow is copied out; the parameter does not
	// escape.
	if copied := escapeSummaryNamed(t, analysis, "CopyThroughBorrow"); len(copied.Parameters) != 0 ||
		len(copied.ReturnFacts) != 1 || copied.ReturnFacts[0].Sources[0].Kind != EscapeSourceTemporary {
		t.Fatalf("copy through borrow summary = %#v", copied)
	}

	// Borrowed parameters called without call-site markers stay call-local.
	for _, summary := range analysis.Summaries() {
		if summary.Name == "Caller" && len(summary.Parameters) != 0 {
			t.Fatalf("non-retaining borrow calls produced escape dispositions: %#v", summary.Parameters)
		}
	}
}
