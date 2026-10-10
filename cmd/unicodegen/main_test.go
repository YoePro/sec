package main

import (
	"os"
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

func TestGeneratedUnicodeSourceIsCurrent(t *testing.T) {
	path := filepath.Join("..", "..", "sec", "stdlib", "unicode", "unicode.sec")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := renderLetterTables(loadRepositoryUCD(t)); string(got) != string(want) {
		t.Fatalf("%s is stale; run go run ./cmd/unicodegen", path)
	}
}
