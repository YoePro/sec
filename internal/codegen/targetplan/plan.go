// Package targetplan resolves canonical scalar facts shared by legacy backends.
package targetplan

import (
	"fmt"
	"sec/internal/layout"
	platformtarget "sec/internal/platform/target"
)

// Plan resolves a triple by exact registry identity, without interpreting its
// architecture spelling. Empty triple selects the canonical host definition.
// Rules: rules/projects/projects.md — "Compilation plans and target lowering";
// rules/types/types.md — "int and uint"; correction5.md — target scalar facts.
func Plan(triple string) (layout.ResolvedScalarPlan, error) {
	if triple == "" {
		definition, ok := platformtarget.Find(platformtarget.Host())
		if ok {
			return definition.ScalarPlan()
		}
	} else {
		for _, definition := range platformtarget.Definitions() {
			if definition.LLVMTriple == triple {
				return definition.ScalarPlan()
			}
		}
	}
	return layout.ResolvedScalarPlan{}, fmt.Errorf("legacy lowering requires a canonical scalar plan for target %q", triple)
}
