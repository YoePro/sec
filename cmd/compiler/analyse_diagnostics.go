package main

import (
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
