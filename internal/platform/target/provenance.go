package target

import (
	"fmt"

	"sec/internal/layout"
)

// FactProvenance explains why one resolved plan fact has its value
// (platform_model.md §§ 19, 20): the fact, its canonical value and the
// configuration source or registry entry that supplied it.
type FactProvenance struct {
	Fact   string
	Value  string
	Source string
}

// Resolution is a resolved plan with its fingerprints and provenance.
// Provenance is explanatory and never part of a fingerprint.
type Resolution struct {
	Plan         layout.ResolvedScalarPlan
	Fingerprints layout.PlanFingerprints
	Provenance   []FactProvenance
}

// Resolve resolves the plan of a requested target and records, in a fixed
// fact order, where each fact came from: the target identity from the
// request's source (for example a command-line --target, a manifest Target
// or the documented host default) and every derived fact from the compiler's
// target registry definition.
// Rules: rules/platform/platform_model.md — §§ 20, 23, 26, 27.
func Resolve(requested Target, source string) (Resolution, error) {
	definition, found := Find(requested)
	if !found {
		return Resolution{}, fmt.Errorf("unsupported target %s-%s (from %s)", requested.OS, requested.Arch, source)
	}
	plan, err := definition.ScalarPlan()
	if err != nil {
		return Resolution{}, err
	}
	registry := "target registry " + definition.OS + "-" + definition.Arch
	resolution := Resolution{Plan: plan, Fingerprints: plan.Fingerprints()}
	add := func(fact, value, origin string) {
		resolution.Provenance = append(resolution.Provenance, FactProvenance{Fact: fact, Value: value, Source: origin})
	}
	add("os", plan.TargetOS, source)
	add("arch", plan.TargetArch, source)
	add("profile", plan.Profile, registry)
	add("abi", plan.ABI, registry)
	add("pointer-width", fmt.Sprint(plan.PointerWidthBits), registry)
	add("endianness", string(plan.Endianness), registry)
	add("llvm-triple", plan.LLVMTriple, registry)
	return resolution, nil
}
