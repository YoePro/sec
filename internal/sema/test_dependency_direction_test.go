package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

const testDependencyTestFile = `module calc

type Fixture struct {
    value: int,
}

let limit: int := 3

fn Helper(value: int) int {
    return Double(value)
}

test "double" {
    testing.ExpectEqual(4, Helper(2))
    let fixture := Fixture { value: limit }
    discard fixture.value
}
`

// Production source must not resolve test-only declarations even when a
// test compilation analyzes both together; each offending use reports S1145
// once, members of a test-only type are covered by the type, and test source
// may depend on production and test source (testing.md §§ 25, 26.2, 40.1).
func TestProductionCannotDependOnTestOnlyDeclarations(t *testing.T) {
	production := `module calc

fn Double(value: int) int {
    return Helper(value) * 2
}

fn Build() Fixture {
    return Fixture { value: limit }
}
`
	_, errors := analyzeSourceFilesRaw(t, sourceFile{"calc/calc.sec", production}, sourceFile{"calc/calc_test.sec", testDependencyTestFile})
	got := []string{}
	for _, err := range errors {
		if err.ID != diagnostics.ProductionDependsOnTest {
			t.Fatalf("unexpected error %s: %s", err.ID, err.Message)
		}
		if !strings.HasSuffix(err.File, "calc.sec") || strings.HasSuffix(err.File, "_test.sec") || !strings.HasSuffix(err.PreviousFile, "calc_test.sec") {
			t.Fatalf("S1145 locations = %s / %s", err.File, err.PreviousFile)
		}
		got = append(got, strings.TrimPrefix(err.Message, "production source cannot depend on test-only declaration "))
	}
	if want := []string{"Fixture", "Helper", "Fixture", "limit"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("S1145 names = %v, want %v", got, want)
	}
}

func TestTestSourceMayDependOnProduction(t *testing.T) {
	production := `module calc

fn Double(value: int) int {
    return value * 2
}
`
	_, errors := analyzeSourceFilesRaw(t, sourceFile{"calc/calc.sec", production}, sourceFile{"calc/calc_test.sec", testDependencyTestFile})
	assertSemaErrors(t, errors, nil)
}
