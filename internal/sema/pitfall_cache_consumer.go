package sema

import "sec/internal/ast"

// walkPitfallBody consumes a complete cached optional body search only after
// fresh canonical Sema and whole-body budget admission. Mandatory owners and
// incomplete prerequisite closures always take the ordinary walk. Cached
// findings pass the current safety/publication gate and current counters.
// Rules: rules/analysis/pitfall_analysis.md — "Incremental analysis", "Analysis states",
// "Diagnostic ownership and coalescing", "Fix safety";
// rules/compiler/incremental_compilation.md — §§2,7,29-34.
func (b *pitfallBuilder) walkPitfallBody(body *ast.BlockStatement) {
	run := b.analyzer.pitfallCacheRun
	if run == nil {
		b.walkBlock(body)
		return
	}
	revision, known := run.Bodies[PitfallBodyIdentity(body)]
	if !known || len(b.analyzer.errors) != 0 {
		run.Usage.Unavailable++
		b.walkBlock(body)
		return
	}
	fingerprint, compatible := b.bodyFingerprint(body, revision)
	if !compatible {
		run.Usage.Unavailable++
		b.walkBlock(body)
		return
	}
	key := pitfallCacheKey{run.Scope, revision.Body}
	payload, hit, lease := run.Cache.lookupOrBegin(key, fingerprint, revision.Dependencies)
	if hit {
		run.Usage.Hits++
		for _, finding := range payload.Findings {
			b.add(finding)
		}
		b.result.foreignExtentInputs = append(b.result.foreignExtentInputs, payload.Foreign...)
		return
	}
	run.Usage.Misses++
	findingsStart, foreignStart := len(b.result.results), len(b.result.foreignExtentInputs)
	b.walkBlock(body)
	payload = pitfallBodyPayload{Findings: b.result.results[findingsStart:], Foreign: b.result.foreignExtentInputs[foreignStart:]}
	// Owner-enrichment occurrences may carry a distinct canonical diagnostic
	// root. Recompute those explanations instead of persisting an owner link.
	for _, finding := range payload.Findings {
		if finding.DiagnosticID != "" || finding.Classification == PitfallProvenInvalid {
			run.Cache.discard(lease)
			run.Usage.Unavailable++
			return
		}
	}
	lease.Entry.Payload = clonePitfallPayload(payload)
	run.Leases = append(run.Leases, lease)
}
