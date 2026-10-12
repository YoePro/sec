package main

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTryNestedMatchEscapeLSP verifies that exhaustive returning match handlers
// use compiler coverage and flow at the actual library source URI.
// Rules: rules/errors/errorhandling.md — §21; rules/control-flow/flowcontrol_match.md — coverage and arm flow.
func TestTryNestedMatchEscapeLSP(t *testing.T) {
	path, err := filepath.Abs("../../sec/stdlib/str/escape.sec")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	values := analyze((&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String(), string(data))
	for _, got := range values {
		if strings.Contains(got.Message, "try handler must return") {
			t.Fatal(got)
		}
	}
	t.Logf("diagnostics: %+v", values)
}

// TestTryNestedMatchRecoveryLSP uses the compiler flow result for valid and
// invalid nested matches without adding editor-local return analysis.
// Rules: rules/errors/errorhandling.md — §§21–21.1; rules/control-flow/flowcontrol_match.md — §23.
func TestTryNestedMatchRecoveryLSP(t *testing.T) {
	for _, test := range []struct {
		file  string
		count int
	}{
		{"try_handler_nested_match_valid.sec", 0},
		{"try_handler_nested_match_invalid.sec", 2},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		values := analyze("file:///tmp/sec-try-nested/"+test.file, string(data))
		if len(values) != test.count {
			t.Fatal(values)
		}
	}
}
