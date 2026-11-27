package diagnostics

import (
	"reflect"
	"testing"
)

// Definition v2 exposes independent identity/policy fields, the exact ordered
// A2012 argument contract, and detached metadata for Lookup and All readers.
// Rules: rules/tooling/diagnostics.md — §§5-7,13.
func TestDefinitionModelV2(t *testing.T) {
	if err := Validate(); err != nil {
		t.Fatal(err)
	}
	d, ok := Lookup(InferredDecimalScale)
	if !ok || d.Namespace != NamespaceAdvisory || d.Family != "types" || d.DefaultSeverity != SeverityInformation || d.Mandatory || d.ShortMessageKey != d.Name+".message" {
		t.Fatal(d)
	}
	names := []string{"type", "carrier_width", "preferred_scale", "selected_scale", "capacity_reduced", "limiting_bound"}
	kinds := []ArgumentKind{ArgumentType, ArgumentInteger, ArgumentInteger, ArgumentInteger, ArgumentBoolean, ArgumentValue}
	if len(d.Arguments) != len(names) {
		t.Fatal(d.Arguments)
	}
	for i, arg := range d.Arguments {
		if arg.Name != names[i] || arg.Kind != kinds[i] || arg.Required != (i < 5) || arg.Description == "" {
			t.Fatal(arg)
		}
	}
	d.Arguments[0].Name = "corrupt"
	again, _ := Lookup(d.ID)
	if again.Arguments[0].Name != "type" {
		t.Fatal("Lookup aliases registry arguments")
	}
	all := All()
	for i := range all {
		if all[i].ID == d.ID {
			all[i].Arguments[0].Name = "changed"
		}
	}
	again, _ = Lookup(d.ID)
	if again.Arguments[0].Name != "type" {
		t.Fatal("All aliases registry arguments")
	}
	for _, d := range All() {
		if string(d.Namespace) != d.ID[:1] || d.ShortMessageKey != d.Name+".message" {
			t.Fatal(d)
		}
		explanation, documented := Explain(d.ID)
		if documented {
			if d.ExplanationKey == "" || !reflect.DeepEqual(d.RelatedIDs, append([]string{}, explanation.Related...)) {
				t.Fatal(d)
			}
			if len(d.RelatedIDs) > 0 {
				d.RelatedIDs[0] = "changed"
				fresh, _ := Lookup(d.ID)
				if fresh.RelatedIDs[0] == "changed" {
					t.Fatal("related IDs alias registry")
				}
			}
		} else if d.ExplanationKey != "" {
			t.Fatal("invented explanation catalog", d)
		}
	}
	// Published configurable S identities stay in S; advisory policy does not rename them.
	if d, _ := Lookup("S1014"); d.Namespace != NamespaceSemantic || d.Mandatory {
		t.Fatal(d)
	}
}

// Invalid registry metadata is rejected without changing active definitions;
// absent optional catalog/category data is allowed during documented migration.
// Rules: rules/tooling/diagnostics.md — §§5,7,12.
func TestDefinitionModelValidation(t *testing.T) {
	base, _ := Lookup(InferredDecimalScale)
	for name, mutate := range map[string]func(*Definition){
		"short ID":             func(d *Definition) { d.ID = "A12" },
		"nondecimal ID":        func(d *Definition) { d.ID = "A12a4" },
		"new namespace":        func(d *Definition) { d.ID = "X2012"; d.Namespace = "X" },
		"mismatched namespace": func(d *Definition) { d.Namespace = NamespaceSemantic },
		"argument kind":        func(d *Definition) { d.Arguments[0].Kind = "RuntimeObject" },
		"empty argument":       func(d *Definition) { d.Arguments[0].Name = "" },
		"duplicate argument":   func(d *Definition) { d.Arguments[1].Name = d.Arguments[0].Name },
		"missing description":  func(d *Definition) { d.Arguments[0].Description = "" },
		"unknown relation":     func(d *Definition) { d.RelatedIDs = []string{"S9999"} },
		"duplicate relation":   func(d *Definition) { d.RelatedIDs = []string{InferredDecimalScale, InferredDecimalScale} },
	} {
		t.Run(name, func(t *testing.T) {
			d := definitionSnapshot(base)
			mutate(&d)
			if err := validateDefinition(d); err == nil {
				t.Fatal("invalid metadata accepted", d)
			}
		})
	}
	base.Category = "types.decimal-scale"
	base.ShortMessageKey, base.ExplanationKey = "", ""
	base.RelatedIDs = []string{DefaultViolatesContract, DefaultNotRepresentable}
	if err := validateDefinition(base); err != nil {
		t.Fatal(err)
	}
	if err := Validate(); err != nil {
		t.Fatal("negative tests changed registry", err)
	}
}
