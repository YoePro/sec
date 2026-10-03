package lexer

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

//go:generate go run ../../cmd/confusablesgen -input ../../third_party/unicode/15.0.0/confusables.txt -output confusables_table.go

// ConfusableSkeleton returns the UTS #39 skeleton of an identifier: the NFD
// form with every code point replaced by its confusable prototype, normalized
// to NFD again. The skeleton is used only to detect confusable spellings and
// never changes identifier identity, which remains the exact NFC spelling.
//
// Rules:
//   - rules/foundations/lexical_structure.md — "Visually confusable identifiers"
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 2.1–2.4
func ConfusableSkeleton(identifier string) string {
	var out strings.Builder
	for _, value := range norm.NFD.String(identifier) {
		if prototype, ok := confusablePrototypes[value]; ok {
			out.WriteString(prototype)
			continue
		}
		out.WriteRune(value)
	}
	return norm.NFD.String(out.String())
}

// IdentifiersConfusable reports two distinct identifier spellings whose UTS #39
// skeletons collide.
func IdentifiersConfusable(left, right string) bool {
	return left != right && ConfusableSkeleton(left) == ConfusableSkeleton(right)
}
