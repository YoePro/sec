package main

import (
	"fmt"
	"strings"

	"sec/internal/lexer"
	"sec/internal/sema"
)

// borrowHover presents the borrow Sema resolved at a reference creation, or
// held by the binding declared at the cursor: borrow kind, referent Place and
// its alternative origins, holder, reborrow parent, mutability authority, the
// resolved end of the holder's live range and the temporal validity
// dependencies of the reference. It reads ResolvedBorrowsAt only and offers
// no edit (borrowing tooling must not change ownership semantics).
// Rules: rules/memory/borrowing.md — §§ 11, 15; rules/tooling/lsp.md — "Hover".
func borrowHover(analyzer *sema.Analyzer, token lexer.Token) string {
	if analyzer == nil {
		return ""
	}
	sections := []string{}
	for _, fact := range analyzer.ResolvedBorrowsAt(token.File, token.Line, token.Column) {
		sections = append(sections, borrowFactLines(fact))
	}
	if len(sections) == 0 {
		return ""
	}
	return "\n\n**Borrow**\n\n" + strings.Join(sections, "\n\n")
}

func borrowFactLines(fact sema.ResolvedBorrowFact) string {
	plan := fact.Plan
	kind := "shared borrow (`ref`)"
	if plan.Facts.MutableAuthority {
		kind = "mutable borrow (`ref mut`)"
	}
	switch {
	case plan.Temporary:
		kind += ", of a compiler temporary owned until the end of the full expression"
	case plan.Implicit:
		kind += ", implicit call-site borrow"
	}
	lines := []string{"- kind: " + kind}
	referent := "`" + plan.Place.String() + "`"
	alternatives := []string{}
	for _, alternative := range plan.Place.AlternativeOrigins {
		alternatives = append(alternatives, "`"+alternative.String()+"`")
	}
	if len(alternatives) > 0 {
		referent += " or, on other paths, " + strings.Join(alternatives, ", ")
	}
	lines = append(lines, "- referent: "+referent)
	if plan.Facts.Provenance != "" {
		lines = append(lines, fmt.Sprintf("- provenance: %s", plan.Facts.Provenance))
	}
	if domain := borrowStorageDomain(plan.Place); domain != "" {
		lines = append(lines, "- storage domain: "+domain)
	}
	if plan.Reborrow {
		parent := "an existing reference"
		if plan.Place.ReferenceHolder != "" {
			parent = "`" + plan.Place.ReferenceHolder + "`"
		}
		lines = append(lines, "- reborrow of "+parent+"; it cannot outlive that parent borrow")
	}
	if plan.Facts.MutableAuthority {
		lines = append(lines, "- authority: exclusive; no other access to the referent while the borrow is live")
	} else {
		lines = append(lines, "- authority: read-only; the referent cannot be mutated or moved while the borrow is live")
	}
	if fact.Holder != "" {
		lines = append(lines, fmt.Sprintf("- holder: `%s` (declared at %d:%d)", fact.Holder, fact.HolderToken.Line, fact.HolderToken.Column))
		lines = append(lines, "- ends: "+borrowEndSpelling(fact.Ends))
	} else if plan.Implicit || plan.Temporary {
		lines = append(lines, "- ends: after the call")
	}
	lines = append(lines, "- validity: "+borrowValiditySpelling(plan.Facts))
	return strings.Join(lines, "\n")
}

func borrowEndSpelling(ends []sema.ReferenceLifetimeEnd) string {
	if len(ends) == 0 {
		return "when the holder goes out of scope"
	}
	// A last-required-use end drops the borrow (MD-065), so a later
	// scope-exit end of the same holder is not a second live-range end.
	parts := []string{}
	seen := map[string]bool{}
	for _, end := range ends {
		if end.Before.Line > 0 {
			part := fmt.Sprintf("after its last required use, before %d:%d", end.Before.Line, end.Before.Column)
			if !seen[part] {
				seen[part] = true
				parts = append(parts, part)
			}
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "; ")
	}
	for _, end := range ends {
		part := "at scope exit"
		if end.Source.Line > 0 {
			part = fmt.Sprintf("at scope exit or rebinding, %d:%d", end.Source.Line, end.Source.Column)
		}
		if !seen[part] {
			seen[part] = true
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "; ")
}

func borrowValiditySpelling(facts sema.ReferenceFacts) string {
	parts := []string{}
	switch facts.Validity {
	case sema.ReferenceValidityProven:
		text := "proven at compile time"
		if facts.ProofProvenance != "" {
			text += " (" + facts.ProofProvenance + ")"
		}
		parts = append(parts, text)
	case sema.ReferenceValidityDynamicEpoch:
		parts = append(parts, fmt.Sprintf("checked at run time against epoch dependency %d of invalidation domain %d", facts.EpochDependency, facts.InvalidationDomain))
	case "":
		parts = append(parts, "unknown")
	default:
		parts = append(parts, string(facts.Validity))
	}
	if facts.AddressStability != "" && facts.AddressStability != sema.AddressStabilityUnknown {
		parts = append(parts, "referent address "+strings.ToLower(string(facts.AddressStability)))
	}
	if facts.Pinned {
		parts = append(parts, "depends on a pin")
	}
	return strings.Join(parts, "; ")
}

// borrowStorageDomain names the storage domain of the referent root in
// source terms (storage.md § 3), never a numeric identity.
func borrowStorageDomain(place sema.Place) string {
	root := "`" + place.Root + "`"
	switch place.StorageOrigin {
	case sema.StorageOriginAutomatic:
		return "automatic storage of " + root + " in this invocation"
	case sema.StorageOriginStatic:
		return "static storage of " + root + " for the whole program"
	case sema.StorageOriginThreadLocal:
		return "thread-local storage of " + root
	case sema.StorageOriginArena:
		return "Arena storage; valid within its allocation epoch"
	case sema.StorageOriginAllocatorBacked:
		return "allocator-backed storage owned through " + root
	}
	return ""
}
