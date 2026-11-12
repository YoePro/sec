package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Every unsafe requirement is classified by the one unsafe-operation
// registry: sites record their kind, authorization and obligations; a
// missing context names the obligations and the safe alternative and says
// that unsafe does not repair a proven-invalid operation; an unsafe block or
// call-form prefix that authorizes nothing is an informational advisory, as
// is one nested in another unsafe context.
// Rules: rules/memory/unsafe.md — §§ 5, 6, 8, 47, 48, 50.
func TestUnsafeOperationRegistry(t *testing.T) {
	source := `module main

unsafe fn Danger(value: int) int {
    return value
}

fn Safe(value: int) int {
    return value
}

fn Missing() int {
    return Danger(1)
}

fn Authorized(pointer: RawPtr[int]) int {
    let first := unsafe Danger(2)
    let second := unsafe pointer.Read()
    unsafe {
        let third := Danger(3)
    }
    return first + second
}

fn Advisories() int {
    unsafe {
        let plain := Safe(1)
    }
    let prefixed := unsafe Safe(2)
    unsafe {
        unsafe {
            let inner := Danger(4)
        }
    }
    return plain + prefixed
}
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, source)
	if len(errors) != 2 || errors[0].ID != diagnostics.UnsafeCallRequiresContext ||
		!strings.Contains(errors[0].Help, "every caller obligation the callee documents") ||
		!strings.Contains(errors[0].Help, "it does not make a compiler-proven invalid operation valid") ||
		!strings.Contains(errors[0].Help, "Safe alternative: call a safe wrapper") {
		t.Fatalf("errors = %v", errors)
	}
	authorized := map[string]bool{}
	for _, site := range analyzer.UnsafeOperationSites() {
		authorized[string(site.Kind)+" "+site.Operation] = authorized[string(site.Kind)+" "+site.Operation] || site.Authorized
	}
	if !authorized["unsafe-call Danger"] || !authorized["raw-pointer-access RawPtr.Read"] {
		t.Errorf("sites = %+v", analyzer.UnsafeOperationSites())
	}
	advisories := []string{}
	for _, advisory := range analyzer.UnsafeAdvisories() {
		if advisory.Severity != diagnostics.SeverityInformation {
			t.Errorf("advisory %v is not informational", advisory)
		}
		advisories = append(advisories, advisory.ID+" "+advisory.Message)
	}
	joined := strings.Join(advisories, "\n")
	for _, want := range []string{
		"S1242 unsafe block contains no unsafe operation",
		"S1242 call-form unsafe authorizes no unsafe operation",
		"S1243 unsafe block is redundant inside another unsafe context",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("advisories lack %q:\n%s", want, joined)
		}
	}
	if len(advisories) != 3 {
		t.Errorf("advisories = %v", advisories)
	}
	if site, operation, ok := analyzer.UnsafeOperationAt("", 17, 34); !ok || operation.Kind != UnsafeRawPointerAccess || !site.Authorized || len(site.Obligations) == 0 {
		t.Errorf("site at Read = %+v %+v %v", site, operation, ok)
	}
}
