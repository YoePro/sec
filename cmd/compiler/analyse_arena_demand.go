package main

import (
	"fmt"

	"sec/internal/lexer"
	"sec/internal/sema"
)

// analysisArenaDemand is the JSON form of one Arena domain's demand: the
// callable that creates it, the canonical kind (constant, unknown or
// unbounded), the exact byte bound when constant, the symbolic derivation
// and the fixed backing capacity when known.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79-81, 130.
type analysisArenaDemand struct {
	Callable      string         `json:"callable"`
	Location      occurrenceSpan `json:"location"`
	Domain        uint32         `json:"domain"`
	Kind          string         `json:"kind"`
	Bytes         string         `json:"bytes,omitempty"`
	CapacityBytes string         `json:"capacity_bytes,omitempty"`
	Expression    string         `json:"expression"`
}

// addArenaDemand presents Sema's byte-accurate Arena demand: every created
// domain's peak bound over its Reset windows, compared with the fixed
// backing when both are known. A constant peak above the backing is an
// advisory finding (an allocation can then return AllocationError); unknown
// and unbounded demand are reported as such, never as a number. Strict
// bounded-memory severity is not decided here (MD-152).
// Rules: rules/memory/arena.md — §§ 114-115; rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79-81, 130.
func (r *analyseReport) addArenaDemand(analyzer *sema.Analyzer, inSelection func(lexer.Token) bool) {
	section := r.section("arena demand")
	for _, report := range analyzer.ArenaDemandReports() {
		if !inSelection(report.Source) {
			continue
		}
		for _, summary := range report.Summaries {
			entry := analysisArenaDemand{Callable: report.Name, Domain: uint32(summary.Domain), Kind: summary.Kind, Expression: summary.Expression,
				Location: occurrenceSpan{File: report.Source.File, Start: occurrencePosition{Line: report.Source.Line, Column: report.Source.Column}, End: occurrencePosition{Line: report.Source.Line, Column: report.Source.Column}}}
			line := fmt.Sprintf("%s: Arena domain #%d: %s demand", report.Name, summary.Domain, summary.Kind)
			if summary.Bytes != nil {
				entry.Bytes = summary.Bytes.String()
				line = fmt.Sprintf("%s: Arena domain #%d: peak demand %s bytes", report.Name, summary.Domain, summary.Bytes)
			}
			if summary.CapacityBytes != nil {
				entry.CapacityBytes = summary.CapacityBytes.String()
				line += fmt.Sprintf(" of a %s-byte backing", summary.CapacityBytes)
			}
			section.add("%s (%s)", line, summary.Expression)
			r.arenaDemand = append(r.arenaDemand, entry)
			if summary.Bytes != nil && summary.CapacityBytes != nil && summary.Bytes.Cmp(summary.CapacityBytes) > 0 {
				r.finding(analyseFinding{Family: "arena-demand", Class: analyseClassAdvisory, Rule: "rules/memory/arena.md § 114", Token: report.Source,
					Message: fmt.Sprintf("Arena domain #%d in %s can need %s bytes but its fixed backing has %s; an allocation may then return AllocationError (%s)", summary.Domain, report.Name, summary.Bytes, summary.CapacityBytes, summary.Expression),
					Actions: []string{"enlarge the backing buffer", "Reset the Arena between allocation windows"}})
				r.counts[analyseClassAdvisory]++
			}
		}
	}
}
