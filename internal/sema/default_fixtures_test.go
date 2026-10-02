package sema

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// The default-value fixture suite named by the default-values rulebook: every
// *_valid.sec analyzes without errors and resolves the documented defaults,
// and every *_invalid.sec reports exactly its intended diagnostics.
//
// Rules:
//   - rules/types/default_values.md — "Primitive tests", "Range tests", "in [...] tests",
//     "Struct tests", "Collection and array tests", "List defaults",
//     "Floating and decimal ranges", "Ambiguous nearest-to-zero values", "Diagnostics"
func TestDefaultValueFixtureSuite(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		source, err := os.ReadFile("../../testdata/defaults/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(source)
	}
	wantDefaults := map[string]map[string]string{
		"default_values_valid.sec": {
			"DefaultInt8": "0", "DefaultUint64": "0", "DefaultFloat": "0.0", "DefaultDecimal": "0.0",
			"DefaultBool": "false", "DefaultString": `""`, "DefaultChar": "0t", "DefaultRune": "0r",
		},
		"default_ranges_valid.sec": {
			"ZeroValid": "0", "PositiveOnly": "3", "NegativeOnly": "-4", "PositiveEven": "4",
			"PositiveOdd": "5", "DecimalRange": "0.01", "ExplicitOverride": "42", "ExclusiveUpper": "-3",
			"BinaryLower": "1.5", "InexactLower32": "0.1", "ExclusiveNegative": "-2.5000000000000004",
			"NamedOverFloat": "1.5",
		},
		"default_in_contract_valid.sec": {
			"Role": `"admin"`, "Retry": "3", "Flag": "true", "Override": `"b"`, "EvenMembers": "4",
		},
		"default_structs_valid.sec": {
			"Empty": "Empty {  }",
			"Outer": `Outer { inner: Inner { count: 0, name: "" }, percent: 1, port: 8080, flag: false }`,
			"Inner": `Inner { count: 0, name: "" }`,
			"Port":  "8080",
		},
		"default_lists_valid.sec":  {},
		"default_arrays_valid.sec": {},
	}
	for file, defaults := range wantDefaults {
		analyzer, errors := analyzeSourceWithAnalyzer(t, read(file))
		if len(errors) != 0 {
			t.Fatalf("%s: errors = %v", file, errors)
		}
		for name, want := range defaults {
			got, _, ok := DefaultValueDisplay(analyzer.Types()[name])
			if !ok || got != want {
				t.Errorf("%s: default of %s = %q (%v), want %q", file, name, got, ok, want)
			}
		}
	}

	wantDiagnostics := map[string][]string{
		"default_values_invalid.sec": {"S1047 6:9", "S1009 7:13", "S1009 8:13"},
		"default_ranges_invalid.sec": {"S1059 5:47", "S1060 6:48", "S1061 10:13", "S1009 11:13"},
		"default_in_contract_invalid.sec": {
			"S1054 4:19", "S1053 5:32", "S1059 6:50", "S1011 7:39", "S1011 7:39",
		},
		"default_structs_invalid.sec": {"S1059 5:38", "S1061 20:19", "S1063 21:19", "S1010 22:18"},
		"default_lists_invalid.sec":   {" 5:17", " 6:32"},
		"default_arrays_invalid.sec":  {"S1009 8:13", "S1009 9:13"},
	}
	for file, want := range wantDiagnostics {
		errors := analyzeSourceRaw(t, read(file))
		got := make([]string, 0, len(errors))
		for _, diagnostic := range errors {
			got = append(got, fmt.Sprintf("%s %d:%d", diagnostic.ID, diagnostic.Line, diagnostic.Column))
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s: diagnostics = %v, want %v\n%v", file, got, want, errors)
		}
	}
}
