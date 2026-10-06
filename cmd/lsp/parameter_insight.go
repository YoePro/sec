package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// parameterInsightSettings controls optional presentation, never validity or
// semantic demand. The zero value disables both optional channels.
// Rules: rules/analysis/parameter_usage_analysis.md — "LSP presentation";
// rules/tooling/lsp.md — "Safety and advisory diagnostics", "Configuration".
type parameterInsightSettings struct {
	Hover      string `json:"hover"`
	Advisories string `json:"advisories"`
}

// parameterInsightSettingsFrom consumes initialization options and the common
// sec configuration payload. Missing/invalid fields preserve previous choices.
// Rules: rules/analysis/parameter_usage_analysis.md — "Project-configured LSP depth";
// rules/tooling/lsp.md — "Configuration".
func parameterInsightSettingsFrom(current parameterInsightSettings, raw json.RawMessage) parameterInsightSettings {
	var payload struct {
		Analysis *struct {
			Parameters *parameterInsightSettings `json:"parameters"`
		} `json:"analysis"`
		Sec *struct {
			Analysis *struct {
				Parameters *parameterInsightSettings `json:"parameters"`
			} `json:"analysis"`
		} `json:"sec"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return current
	}
	analysis := payload.Analysis
	if analysis == nil && payload.Sec != nil {
		analysis = payload.Sec.Analysis
	}
	if analysis == nil || analysis.Parameters == nil {
		return current
	}
	next := analysis.Parameters
	switch next.Hover {
	case "off", "concise", "detailed":
		current.Hover = next.Hover
	}
	switch next.Advisories {
	case "off", "info", "warning", "error":
		current.Advisories = next.Advisories
	}
	return current
}

// updateParameterInsight synchronizes presentation with diagnostic publication.
// Rules: rules/tooling/lsp.md — "Configuration", "Document synchronization".
func (s *server) updateParameterInsight(raw json.RawMessage) {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	s.parameterInsight = parameterInsightSettingsFrom(s.parameterInsight, raw)
}

// parameterInsightSettings snapshots settings while diagnostics may publish.
// Rules: rules/tooling/lsp.md — "Responsiveness model".
func (s *server) parameterInsightSettings() parameterInsightSettings {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	return s.parameterInsight
}

// parameterInsightDiagnostics filters only A2001 optional advice and applies
// the chosen advisory severity. Normative diagnostics are always retained.
// Rules: rules/tooling/lsp.md — "Safety and advisory diagnostics";
// rules/analysis/parameter_usage_analysis.md — "Large-value advisory".
func parameterInsightDiagnostics(values []diagnostic, settings parameterInsightSettings) []diagnostic {
	result := make([]diagnostic, 0, len(values))
	severity := map[string]int{"info": 3, "warning": 2, "error": 1}[settings.Advisories]
	for _, value := range values {
		if value.Code == diagnostics.LargeValueParameter {
			if severity == 0 {
				continue
			}
			value.Severity = severity
		}
		result = append(result, value)
	}
	return result
}

// parameterInsightHover resolves the hovered declaration/use through Sema
// definition tokens, avoiding name matching between unrelated parameters.
// It renders existing demand and candidate policy without a tooling analysis.
// Rules: rules/analysis/parameter_usage_analysis.md — "LSP presentation",
// "Recommendation confidence", "Candidate blockers";
// rules/tooling/lsp.md — "Hover".
func parameterInsightHover(analyzer *sema.Analyzer, token lexer.Token, mode string) string {
	analysis := analyzer.ParameterUsageAnalysis()
	if analysis == nil {
		return ""
	}
	definitions := append([]lexer.Token{token}, analyzer.DefinitionsAt(token.File, token.Line, token.Column)...)
	matches := func(declaration lexer.Token) bool {
		for _, definition := range definitions {
			if sameSourceToken(declaration, definition) {
				return true
			}
		}
		return false
	}
	var lines []string
	recommendations := analysis.Recommendations()
	for _, summary := range analysis.Summaries() {
		wholeCallable := matches(summary.Declaration)
		parameters := append([]sema.ParameterUsageParameterSummary(nil), summary.Parameters...)
		if summary.Receiver != nil && wholeCallable {
			parameters = append([]sema.ParameterUsageParameterSummary{*summary.Receiver}, parameters...)
		}
		for _, parameter := range parameters {
			if !wholeCallable && !matches(parameter.Declaration) {
				continue
			}
			demand := parameter.Demand
			if mode == "detailed" {
				lines = append(lines, fmt.Sprintf("Parameter `%s`: access=%s; mutation=%s; ownership=%s; lifetime=%s; identity=%s; shape=%v; minimum extent=%d; storage=%v; representation=%s; precision=%s (callable=%s).", parameter.Name, demand.Access, demand.Mutation, demand.Ownership, demand.Lifetime, demand.Identity, demand.Shapes, demand.MinimumExtent, demand.Storage, demand.Representation, demand.Precision, summary.Precision))
				for _, candidate := range recommendations {
					if candidate.Callable != summary.Callable || candidate.Binding != parameter.Binding || candidate.Parameter != parameter.Name {
						continue
					}
					lines = append(lines, fmt.Sprintf("Candidate `%s`: %s. Confidence: %s. Reasons: %s. Blockers: %s.", candidate.Candidate, candidate.Status, candidate.Confidence, strings.Join(candidate.Reasons, "; "), strings.Join(candidate.Blockers, "; ")))
				}
			} else if demand.Precision != sema.ParameterDemandExact || summary.Precision != sema.ParameterDemandExact {
				lines = append(lines, fmt.Sprintf("Parameter `%s`: demand analysis is incomplete (parameter=%s, callable=%s).", parameter.Name, demand.Precision, summary.Precision))
			} else if demand.Access == sema.ParameterAccessUnused {
				lines = append(lines, fmt.Sprintf("Parameter `%s`: unused by this function.", parameter.Name))
			} else {
				lines = append(lines, fmt.Sprintf("Parameter `%s`: %s access, %s lifetime, %s.", parameter.Name, demand.Access, demand.Lifetime, demand.Ownership))
			}
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(lines, "\n\n")
}
