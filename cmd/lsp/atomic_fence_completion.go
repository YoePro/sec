package main

import (
	"strings"

	"sec/internal/sema"
)

// atomicNamespaceCompletionItems completes the core `atomic` namespace after
// `atomic.` with its one public function, Fence (atomics.md §§ 3(3), 26).
// A qualifier preceded by another selector or identifier byte is not the
// namespace.
//
// Rules:
//   - rules/concurrency/atomics.md — §§ 3(5), 26
//   - rules/tooling/lsp.md — "Completion"
func atomicNamespaceCompletionItems(text string, dotOffset int, prefix string) ([]completionItem, bool) {
	const qualifier = "atomic"
	if dotOffset < len(qualifier) || text[dotOffset-len(qualifier):dotOffset] != qualifier {
		return nil, false
	}
	if start := dotOffset - len(qualifier); start > 0 && (isIdentifierByte(text[start-1]) || text[start-1] == '.') {
		return nil, false
	}
	member := sema.AtomicFenceMember()
	items := []completionItem{}
	if item, ok := compilerKnownCompletionItem(member); ok && completionLabelMatches(item.Label, prefix) {
		item.Kind = 3
		item.Detail = "atomic." + compilerKnownSignatureLabel(member)
		items = append(items, item)
	}
	return items, true
}

// fenceOrderCompletionItems keeps only the valid fence orders when
// `MemoryOrder.` is completed as the argument of atomic.Fence, so Relaxed —
// a compile-time error there — is never offered (atomics.md § 26(5)–(6)).
func fenceOrderCompletionItems(text string, receiverStart int, receiver string, items []completionItem) []completionItem {
	if receiver != "MemoryOrder" || !strings.HasSuffix(strings.TrimRight(text[:receiverStart], " \t"), "atomic.Fence(") {
		return items
	}
	filtered := items[:0]
	for _, item := range items {
		if item.Label != "Relaxed" {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
