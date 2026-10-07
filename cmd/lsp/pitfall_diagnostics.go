package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sec/internal/diagnostics"
	"sec/internal/sema"
)

// pitfallDiagnosticSettings consumes the common diagnostic-rule policy. It
// controls optional presentation only, never the mandatory semantic producer.
// Rules: rules/tooling/diagnostics.md — §18(1)-(4), (7), (11);
// rules/analysis/pitfall_analysis.md — "Project configuration", "LSP presentation".
type pitfallDiagnosticSettings struct{ Rules map[string]string }

// pitfallSettingsFrom merges valid rule choices from initialization options or
// sec.diagnostics notifications. Absent or malformed fields preserve choices.
// Rules: rules/tooling/lsp.md — "Configuration";
// rules/tooling/diagnostics.md — §18 "Diagnostic configuration".
func pitfallSettingsFrom(current pitfallDiagnosticSettings, raw json.RawMessage) pitfallDiagnosticSettings {
	result := pitfallDiagnosticSettings{Rules: map[string]string{}}
	for key, value := range current.Rules {
		result.Rules[key] = value
	}
	var payload struct {
		Diagnostics *struct {
			Rules map[string]json.RawMessage `json:"rules"`
		} `json:"diagnostics"`
		Sec *struct {
			Diagnostics *struct {
				Rules map[string]json.RawMessage `json:"rules"`
			} `json:"diagnostics"`
		} `json:"sec"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return result
	}
	config := payload.Diagnostics
	if config == nil && payload.Sec != nil {
		config = payload.Sec.Diagnostics
	}
	if config != nil {
		for key, rawValue := range config.Rules {
			var value string
			if json.Unmarshal(rawValue, &value) == nil && validPitfallLevel(value) {
				result.Rules[key] = value
			}
		}
	}
	return result
}

// validPitfallLevel accepts the shared configurable diagnostic severity values.
// Rules: rules/tooling/diagnostics.md — §18(3).
func validPitfallLevel(value string) bool {
	switch value {
	case "off", "info", "warning", "error":
		return true
	}
	return false
}

// updatePitfallSettings invalidates every queued diagnostic generation at the
// same moment as the policy change, preventing old-policy publication.
// Rules: rules/analysis/pitfall_analysis.md — "LSP configuration reload";
// rules/tooling/lsp.md — "Snapshots", "Configuration".
func (s *server) updatePitfallSettings(raw json.RawMessage) {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	s.pitfallSettings = pitfallSettingsFrom(s.pitfallSettings, raw)
	for dir := range s.diagnosticGeneration {
		s.invalidateDiagnosticJob(dir)
	}
}

// pitfallSettingsSnapshot detaches client policy for one analysis request.
// Rules: rules/tooling/lsp.md — "Snapshots", "Responsiveness model".
func (s *server) pitfallSettingsSnapshot() pitfallDiagnosticSettings {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	return pitfallSettingsFrom(s.pitfallSettings, nil)
}

// pitfallLevel reads the already-defined project diagnostics.rules section on
// every request; project choices override user fallback, and unknown settings
// cannot alter mandatory definitions. No new pitfall manifest syntax is added.
// Rules: rules/tooling/diagnostics.md — §18(4), (10)-(12);
// rules/tooling/lsp.md — "Configuration";
// rules/projects/projects.md — §23 "Project analysis and tooling settings".
func pitfallLevel(uri string, settings pitfallDiagnosticSettings) int {
	level := "info"
	if value := settings.Rules["suspicious.pitfall"]; validPitfallLevel(value) {
		level = value
	}
	if value := settings.Rules[diagnostics.PitfallAdvisory]; validPitfallLevel(value) {
		level = value
	}
	path := pathFromURI(uri)
	file, err := os.Open(filepath.Join(findProjectRoot(path), ".sec", "sec.toml"))
	if err == nil {
		defer file.Close()
		rules := map[string]string{}
		section := ""
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
				section = strings.TrimSpace(line[1 : len(line)-1])
				continue
			}
			if section != "diagnostics.rules" {
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			if unquoted, err := strconv.Unquote(key); err == nil {
				key = unquoted
			}
			value = strings.TrimSpace(value)
			if comment := strings.Index(value, "#"); comment >= 0 {
				value = strings.TrimSpace(value[:comment])
			}
			if unquoted, err := strconv.Unquote(value); err == nil && validPitfallLevel(unquoted) {
				rules[key] = unquoted
			}
		}
		if scanner.Err() == nil {
			if value := rules["suspicious.pitfall"]; validPitfallLevel(value) {
				level = value
			}
			if value := rules[diagnostics.PitfallAdvisory]; validPitfallLevel(value) {
				level = value
			}
		}
	}
	return map[string]int{"off": 0, "info": 3, "warning": 2, "error": 1}[level]
}

// pitfallDiagnostics presents compiler findings and canonical evidence without
// duplicating a mandatory owner. Severity promotion affects presentation only.
// Rules: rules/analysis/pitfall_analysis.md — "LSP presentation", "Diagnostic ownership and coalescing";
// rules/tooling/diagnostics.md — §§9,18.
func pitfallDiagnostics(analyzer *sema.Analyzer, uri, text string, overlay sourceOverlay, settings pitfallDiagnosticSettings) []diagnostic {
	severity := pitfallLevel(uri, settings)
	result := []diagnostic{}
	if severity == 0 {
		return result
	}
	for _, finding := range analyzer.PitfallAnalysis().Findings() {
		if finding.DiagnosticID != "" || finding.Classification == sema.PitfallProvenInvalid || normalizedSourcePath(finding.Subject.Source.File) != normalizedSourcePath(pathFromURI(uri)) {
			continue
		}
		value := diagnostic{Range: diagnosticTokenRange(finding.Subject.Source, text), Severity: severity, Code: diagnostics.PitfallAdvisory, Source: "sec", Message: string(finding.Rule) + ": " + string(finding.Classification) + " (" + string(finding.Confidence) + ")"}
		for _, evidence := range finding.EvidenceFor {
			value.Message += "\n" + evidence.Fact
			if evidence.Source.File != "" && evidence.Source.Line > 0 {
				source := text
				if normalizedSourcePath(evidence.Source.File) != normalizedSourcePath(pathFromURI(uri)) {
					if data, ok := overlay[normalizedSourcePath(evidence.Source.File)]; ok {
						source = data
					} else if data, err := os.ReadFile(evidence.Source.File); err == nil {
						source = string(data)
					}
				}
				value.RelatedInformation = append(value.RelatedInformation, diagnosticRelatedInformation{Location: location{URI: uriFromPath(evidence.Source.File), Range: diagnosticTokenRange(evidence.Source, source)}, Message: evidence.Fact})
			}
		}
		for _, action := range finding.Actions {
			label := "Suggested edit: "
			if action.Kind == sema.PitfallProvenFix {
				label = "Proven fix: "
			}
			value.Message += "\n" + label + action.Title
			if action.Replacement != "" {
				value.Message += " → " + action.Replacement
			}
		}
		result = append(result, value)
	}
	return result
}
