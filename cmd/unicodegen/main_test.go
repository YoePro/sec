package main

import (
	"os"
	"path/filepath"
	"testing"
	"unicode"
)

func TestLetterRangesMatchGoUnicodeIsLetter(t *testing.T) {
	ranges := unicodeLetterRanges()
	for value := rune(-1); value <= unicode.MaxRune; value++ {
		if got, want := tableContains(ranges, value), unicode.IsLetter(value); got != want {
			t.Fatalf("IsLetter(%U) = %t, want %t", value, got, want)
		}
	}
	if tableContains(ranges, unicode.MaxRune+1) {
		t.Fatal("table accepted rune above Unicode maximum")
	}
}

func TestLetterRangesCoverRepresentativeRunes(t *testing.T) {
	tests := []struct {
		value rune
		want  bool
	}{
		{'A', true},
		{'z', true},
		{'\u00E5', true},
		{'\u00C4', true},
		{'\u03A9', true},
		{'\u03BB', true},
		{'\u0416', true},
		{'\u044F', true},
		{'\u4E2D', true},
		{'0', false},
		{'$', false},
		{'\u0378', false},
		{unicode.ReplacementChar, false},
		{-1, false},
	}

	ranges := unicodeLetterRanges()
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
	if want := renderLetterTables(); string(got) != string(want) {
		t.Fatalf("%s is stale; run go run ./cmd/unicodegen", path)
	}
}
