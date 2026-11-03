package main

import (
	"fmt"
	"sec/internal/diagnostics"
	"sec/internal/sema"
)

// analyseDiagnosticClass presents the severity and proof supplied by the
// diagnostic owner. Informational advice is not counted as a warning, and an
// error requiring an unavailable proof is distinguished from proven invalidity.
// No class is inferred from message text or diagnostic-name spelling.
// Rules: rules/compiler/compiler_analysis.md — §7(4–8), §61(3);
// rules/tooling/diagnostics.md — §4(6–9).
func analyseDiagnosticClass(value sema.Error) analyseReportClass {
	severity := value.Severity
	if definition, known := diagnostics.Lookup(value.ID); known && (definition.Mandatory || severity == "") {
		severity = definition.DefaultSeverity
	}
	switch severity {
	case diagnostics.SeverityInformation:
		return analyseClassAdvisory
	case diagnostics.SeverityError:
		if value.ProofState == diagnostics.ProofUnproven {
			return analyseClassUnproven
		}
		return analyseClassError
	default:
		return analyseClassWarning
	}
}

// emitSelectedDiagnostics sends already family/focus-filtered owner diagnostics
// through the canonical reporter in every presentation mode. JSON must retain
// the same occurrences as human output, including advisory IDs and repairs;
// summary counts alone cannot replace the diagnostics being counted.
// Rules: rules/tooling/cli.md — §§5.4,6(2–4);
// rules/tooling/diagnostics.md — §14;
// MD-017 correction — §§6,10(4).
func (r analyseReport) emitSelectedDiagnostics() {
	for _, warning := range r.warnings {
		label := "Warning"
		if warning.Severity == diagnostics.SeverityInformation {
			label = "Info"
		}
		if warning.Severity == "" {
			warning.Severity = diagnostics.SeverityWarning
		}
		cliDiagnostics.semaDiagnostic(warning, fmt.Sprintf("%s: %s\n", label, warning))
	}
}
