package layout

import (
	"fmt"
	"strings"
)

// DescribeLayoutFailure presents a failed canonical layout query in the existing
// consumer diagnostic, including the plan, storage requirements and repair owner.
// It selects no layout and never treats missing target facts as host facts.
// Rules: rules/corrections/applied/MD-141-MD-142-scalar-and-aggregate-layout-normative-correction-2026-11-20-20261121.md — §10(1);
// rules/memory/layout.md — §§5(5),42.
func DescribeLayoutFailure(plan ResolvedScalarPlan, requirement LayoutRequirement, typeName string, state LayoutCompleteness, missing []string, operation string) string {
	known := func(value string) string {
		if value == "" {
			return "unresolved"
		}
		return value
	}
	repair := "supply verified target data-layout and representation alignment facts in the CompilationPlan"
	switch state {
	case LayoutUnsupported:
		repair = "implement the required P24 representation or explicit layout contract in the canonical layout resolver"
	case LayoutInvalid:
		repair = "correct the contradictory type/layout contract or target configuration before materialization"
	}
	if len(missing) == 0 {
		missing = []string{"complete size and alignment facts are unavailable"}
	}
	if typeName == "" {
		typeName = known(requirement.TypeIdentity)
	}
	return fmt.Sprintf("layout of %s (type identity %s) on target %s/%s (triple %s, profile %s, ABI %s): required %s; %s: %s; %s needs a complete layout for byte extent, alignment and placement; %s",
		typeName, known(requirement.TypeIdentity), known(plan.TargetOS), known(plan.TargetArch), known(plan.LLVMTriple), known(plan.Profile), known(plan.ABI),
		strings.Join(layoutStorageDescriptions(plan, requirement, map[string]bool{}), ", "), state, strings.Join(missing, "; "), operation, repair)
}

// layoutStorageDescriptions describes required representation classes and widths
// from the semantic query. It performs no size, placement or alignment inference.
// Rules: rules/memory/layout.md — §§47.1–47.4; MD-141 correction §10(1).
func layoutStorageDescriptions(plan ResolvedScalarPlan, requirement LayoutRequirement, visiting map[string]bool) []string {
	if visiting[requirement.TypeIdentity] {
		return []string{"recursive " + requirement.TypeIdentity}
	}
	visiting[requirement.TypeIdentity] = true
	defer delete(visiting, requirement.TypeIdentity)
	if requirement.Carrier != nil {
		return layoutStorageDescriptions(plan, *requirement.Carrier, visiting)
	}
	if requirement.Element != nil {
		return layoutStorageDescriptions(plan, *requirement.Element, visiting)
	}
	if len(requirement.Fields) > 0 {
		var fields []string
		for _, field := range requirement.Fields {
			for _, representation := range layoutStorageDescriptions(plan, field.Requirement, visiting) {
				fields = append(fields, field.Name+": "+representation)
			}
		}
		return fields
	}
	if requirement.Kind != LayoutScalar {
		return []string{string(requirement.Kind) + " representation"}
	}
	width := requirement.WidthBits
	if requirement.TargetSized {
		width = plan.PointerWidthBits
	}
	class := IntegerStorage
	switch requirement.Scalar {
	case ScalarBool, ScalarByte, ScalarChar:
		width = 8
	case ScalarRune:
		width = 32
	case ScalarFloat:
		class = BinaryFloatStorage
	case ScalarDecimal:
		return []string{fmt.Sprintf("decimal coefficient IntegerStorage(%d), scale IntegerStorage(32)", width)}
	}
	if width == 0 {
		return []string{fmt.Sprintf("%s(unresolved width)", class)}
	}
	return []string{(StorageRepresentation{Class: class, WidthBits: width}).String()}
}
