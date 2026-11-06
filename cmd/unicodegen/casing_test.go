package main

import (
	"bytes"
	"testing"
)

// The generated simple case runs reproduce every UnicodeData.txt simple
// mapping, including Cyrillic (+32), alternating Latin Extended pairs,
// titlecase digraphs and scalars whose full mapping differs (ß stays ß).
// Rules: rules/library/core-library.md — §11.3.
func TestSimpleCaseMappingRuns(t *testing.T) {
	db, err := loadUCD(repositoryUCD, unicodeVersion)
	if err != nil {
		t.Fatal(err)
	}
	upper, lower := caseRuns(db.simpleUpper), caseRuns(db.simpleLower)
	for _, test := range []struct {
		name     string
		runs     []caseRun
		from, to rune
	}{
		{"Cyrillic lower", lower, 'А', 'а'},
		{"Cyrillic upper", upper, 'я', 'Я'},
		{"Latin Extended-A pair", lower, 'Ā', 'ā'},
		{"dotted capital I", lower, 'İ', 'i'},
		{"dotless i", upper, 'ı', 'I'},
		{"long s", upper, 'ſ', 'S'},
		{"sharp s has no simple upper", upper, 'ß', 'ß'},
		{"Greek final sigma", upper, 'ς', 'Σ'},
		{"Armenian", lower, 'Ա', 'ա'},
		{"Deseret", lower, 0x10400, 0x10428},
		{"unmapped", upper, '1', '1'},
	} {
		if got := mapThroughRuns(test.runs, test.from); got != test.to {
			t.Errorf("%s: U+%04X maps to U+%04X, want U+%04X", test.name, test.from, got, test.to)
		}
	}
	if got := db.simpleTitlecase(0x01C6); got != 0x01C5 {
		t.Errorf("titlecase of U+01C6 is U+%04X, want U+01C5", got)
	}
	rendered, err := renderCoreCaseMappings(db)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"fn _UnicodeSimpleUpper(ch: rune) rune", "fn _UnicodeSimpleLower(ch: rune) rune", "fn _UnicodeSimpleTitle(ch: rune) rune"} {
		if !bytes.Contains(rendered, []byte(want)) {
			t.Errorf("missing %s", want)
		}
	}
}

// Full mappings keep the unconditional SpecialCasing entries and C+F folds.
// Rules: rules/library/core-library.md — §11.3; stdlib/str/stdlib_str.md — §§ 9.3–9.6.
func TestFullCaseMappings(t *testing.T) {
	db, err := loadUCD(repositoryUCD, unicodeVersion)
	if err != nil {
		t.Fatal(err)
	}
	find := func(mappings []fullMapping, point rune) []rune {
		for _, mapping := range mappings {
			if mapping.point == point {
				return mapping.runes
			}
		}
		return nil
	}
	upper := db.fullMappings(2, db.simpleUppercase)
	if got := string(find(upper, 'ß')); got != "SS" {
		t.Errorf("ß uppercases to %q, want SS", got)
	}
	if got := string(find(upper, 'ﬁ')); got != "FI" {
		t.Errorf("ﬁ uppercases to %q, want FI", got)
	}
	if got := string(find(db.fullMappings(1, db.simpleTitlecase), 'ß')); got != "Ss" {
		t.Errorf("ß titlecases to %q, want Ss", got)
	}
	if got := string(find(db.fullMappings(0, db.simpleLowercase), 'İ')); got != "i̇" {
		t.Errorf("İ lowercases to %q, want i + U+0307", got)
	}
	if find(db.fullMappings(0, db.simpleLowercase), 'Σ') != nil {
		t.Error("Final_Sigma is conditional and must not be an unconditional lowercase entry")
	}
	if got := string(find(db.foldMappings(), 'ß')); got != "ss" {
		t.Errorf("ß folds to %q, want ss", got)
	}
	if got := string(find(db.foldMappings(), 'A')); got != "a" {
		t.Errorf("A folds to %q, want a", got)
	}
	rendered, err := renderCoreFullCaseMappings(db)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"type RuneCaseMapping struct", "fn FullLowercase() RuneCaseMapping", "fn FullUppercase() RuneCaseMapping", "fn FullTitlecase() RuneCaseMapping", "fn FullCaseFold() RuneCaseMapping"} {
		if !bytes.Contains(rendered, []byte(want)) {
			t.Errorf("missing %s", want)
		}
	}
}
