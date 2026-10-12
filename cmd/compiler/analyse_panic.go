package main

import (
	"fmt"
	"strings"

	"sec/internal/diagnostics"
	"sec/internal/sema"
)

// addPanicCauseReport renders every canonical introducing source and its
// call-site witness, including possible reasons and static diagnostic messages.
// Rules: rules/errors/panic.md — §§25(3),28(2),(6)–(7);
// rules/analysis/effect_analysis.md — "Effect causes".
func addPanicCauseReport(section *analyseSection, causes []sema.EffectDomainCause, root string, names map[sema.CallableID]string) {
	for _, cause := range causes {
		effect := cause.Effect
		reasons := []string{}
		for _, reason := range effect.PanicReasonIDs {
			if definition, ok := diagnostics.PanicReasonByID(reason); ok {
				reasons = append(reasons, definition.Name)
			}
		}
		line := fmt.Sprintf("%s: panic cause %s in %s at %s", root, effect.Kind, names[cause.Callable], analysePosition(cause.Source))
		if len(reasons) > 0 {
			line += " [" + strings.Join(reasons, ", ") + "]"
		}
		if effect.HasMessage {
			line += fmt.Sprintf("; message %q", effect.Message)
		}
		section.add("%s", line)
		for _, site := range cause.Path {
			section.add("  via %s at %s [%s, %s]", names[site.Caller], analysePosition(site.Source), site.Dispatch, site.Execution)
		}
	}
}
