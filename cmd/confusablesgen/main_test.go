package main

import (
	"bytes"
	"go/format"
	"os"
	"strings"
	"testing"
)

// The checked-in table must be exactly what the generator produces from the
// pinned Unicode data, so the confusable data cannot drift independently.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 2.2–2.3
func TestGeneratedConfusableTableIsCurrent(t *testing.T) {
	source, err := os.Open("../../third_party/unicode/15.0.0/confusables.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	version, mappings, err := parseConfusables(source)
	if err != nil {
		t.Fatal(err)
	}
	if version != "15.0.0" || len(mappings) < 6000 {
		t.Fatalf("version %q with %d mappings", version, len(mappings))
	}
	want, err := format.Source(renderTable(version, mappings))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("../../internal/lexer/confusables_table.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("internal/lexer/confusables_table.go is stale; run go generate ./internal/lexer")
	}
	if _, _, err := parseConfusables(strings.NewReader("0041 ; 0061 ; MA\n")); err == nil {
		t.Fatal("data without a version header was accepted")
	}
}
