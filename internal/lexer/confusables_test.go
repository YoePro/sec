package lexer

import (
	"testing"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 2.2–2.4
func TestConfusableDataMatchesCompilerUnicodeVersion(t *testing.T) {
	if ConfusableDataVersion != unicode.Version {
		t.Fatalf("confusable data Unicode %s differs from identifier Unicode %s", ConfusableDataVersion, unicode.Version)
	}
	if ConfusableDataVersion != norm.Version {
		t.Fatalf("confusable data Unicode %s differs from normalization Unicode %s", ConfusableDataVersion, norm.Version)
	}
}

func TestConfusableSkeletons(t *testing.T) {
	cases := []struct {
		left, right string
		want        bool
	}{
		{"admin", "аdmin", true},  // Cyrillic а
		{"count", "count", false}, // identical spellings are one identifier
		{"Value", "value", false},
		{"total", "totaI", true}, // Latin capital I versus small l
		{"μs", "µs", true},       // Greek mu versus micro sign
		{"left", "right", false},
	}
	for _, test := range cases {
		if got := IdentifiersConfusable(test.left, test.right); got != test.want {
			t.Errorf("IdentifiersConfusable(%q, %q) = %t, want %t (skeletons %q, %q)", test.left, test.right, got, test.want, ConfusableSkeleton(test.left), ConfusableSkeleton(test.right))
		}
	}
}
