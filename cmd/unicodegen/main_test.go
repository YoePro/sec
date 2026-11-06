package main

import (
	"path/filepath"
	"testing"
)

var repositoryUCD = filepath.Join("..", "..", "third_party", "unicode", unicodeVersion, "ucd")

func loadRepositoryUCD(t *testing.T) *ucdDatabase {
	t.Helper()
	db, err := loadUCD(repositoryUCD, unicodeVersion)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestLetterRangesCoverRepresentativeRunes(t *testing.T) {
	tests := []struct {
		value rune
		want  bool
	}{
		{'A', true},
		{'z', true},
		{'å', true},
		{'Ä', true},
		{'Ω', true},
		{'λ', true},
		{'Ж', true},
		{'я', true},
		{'中', true},
		{'0', false},
		{'$', false},
		{'͸', false},
		{'�', false},
		{-1, false},
		{maxCodePoint + 1, false},
	}

	ranges := unicodeLetterRanges(loadRepositoryUCD(t))
	for _, tt := range tests {
		if got := tableContains(ranges, tt.value); got != tt.want {
			t.Errorf("IsLetter(%U) = %t, want %t", tt.value, got, tt.want)
		}
	}
}

// All generated artifacts must be current, including the core tables.
// Rules: rules/library/core-library.md — §11.3.
func TestGeneratedUnicodeSourceIsCurrent(t *testing.T) {
	if err := generateUnicode(repositoryUCD, "../../sec/stdlib/unicode/unicode.sec", "../../sec/core", true); err != nil {
		t.Fatal(err)
	}
}
