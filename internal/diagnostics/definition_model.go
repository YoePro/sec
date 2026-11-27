package diagnostics

import "fmt"

// Namespace is the permanent leading-letter ID namespace, independent of
// topic family, severity and whether policy may disable a diagnostic.
// Rules: rules/tooling/diagnostics.md — §§3(8-10),5(2-5),7(5).
type Namespace string

const (
	NamespaceLexical   Namespace = "L"
	NamespaceParser    Namespace = "P"
	NamespaceSemantic  Namespace = "S"
	NamespaceAdvisory  Namespace = "A"
	NamespaceBuild     Namespace = "B"
	NamespaceFormatter Namespace = "F"
)

// ArgumentKind is the closed logical vocabulary of diagnostic message inputs.
// Values describe source/compiler facts rather than their current wire encoding.
// Rules: rules/tooling/diagnostics.md — §7(10-11).
type ArgumentKind string

const (
	ArgumentText       ArgumentKind = "Text"
	ArgumentIdentifier ArgumentKind = "Identifier"
	ArgumentType       ArgumentKind = "Type"
	ArgumentValue      ArgumentKind = "Value"
	ArgumentInteger    ArgumentKind = "Integer"
	ArgumentBoolean    ArgumentKind = "Boolean"
	ArgumentPath       ArgumentKind = "Path"
	ArgumentSymbol     ArgumentKind = "Symbol"
	ArgumentToken      ArgumentKind = "Token"
	ArgumentTarget     ArgumentKind = "Target"
	ArgumentVariant    ArgumentKind = "Variant"
	ArgumentCapability ArgumentKind = "Capability"
)

// ArgumentDefinition preserves registry order and required/optional status.
// Rules: rules/tooling/diagnostics.md — §7(10).
type ArgumentDefinition struct {
	Name        string       `json:"name"`
	Kind        ArgumentKind `json:"kind"`
	Required    bool         `json:"required"`
	Description string       `json:"description"`
}

// definitionSnapshot exposes the logical fields without changing published
// identities. Existing short-message keys follow the emitted-occurrence naming
// contract; extended keys/relations exist only where registry documentation
// exists. Missing argument migrations remain empty, not inferred from text.
// Rules: rules/tooling/diagnostics.md — §§5,7,13-14,16(4).
func definitionSnapshot(d Definition) Definition {
	if d.Namespace == "" && len(d.ID) > 0 {
		d.Namespace = Namespace(d.ID[:1])
	}
	if d.ShortMessageKey == "" {
		d.ShortMessageKey = d.Name + ".message"
	}
	if explanation, ok := Explain(d.ID); ok {
		if d.ExplanationKey == "" {
			d.ExplanationKey = d.Name + ".explanation"
		}
		if d.RelatedIDs == nil {
			d.RelatedIDs = explanation.Related
		}
	}
	d.Arguments = append([]ArgumentDefinition{}, d.Arguments...)
	d.RelatedIDs = append([]string{}, d.RelatedIDs...)
	return d
}

// validateDefinition rejects contradictory namespace, argument and relation
// metadata while allowing the explicitly tracked catalog/argument migration.
// Rules: rules/tooling/diagnostics.md — §§5,7,12(1-4).
func validateDefinition(d Definition) error {
	if len(d.ID) != 5 {
		return fmt.Errorf("diagnostic %q has invalid ID syntax", d.ID)
	}
	switch d.Namespace {
	case NamespaceLexical, NamespaceParser, NamespaceSemantic, NamespaceAdvisory, NamespaceBuild, NamespaceFormatter:
	default:
		return fmt.Errorf("diagnostic %s has unknown namespace %q", d.ID, d.Namespace)
	}
	if string(d.Namespace) != d.ID[:1] {
		return fmt.Errorf("diagnostic %s has inconsistent namespace %q", d.ID, d.Namespace)
	}
	for _, digit := range d.ID[1:] {
		if digit < '0' || digit > '9' {
			return fmt.Errorf("diagnostic %q has invalid ID syntax", d.ID)
		}
	}
	names := map[string]bool{}
	for _, arg := range d.Arguments {
		if arg.Name == "" || names[arg.Name] || arg.Description == "" {
			return fmt.Errorf("diagnostic %s has empty/duplicate argument or missing description: %q", d.ID, arg.Name)
		}
		names[arg.Name] = true
		switch arg.Kind {
		case ArgumentText, ArgumentIdentifier, ArgumentType, ArgumentValue, ArgumentInteger, ArgumentBoolean, ArgumentPath, ArgumentSymbol, ArgumentToken, ArgumentTarget, ArgumentVariant, ArgumentCapability:
		default:
			return fmt.Errorf("diagnostic %s argument %s has unknown kind %q", d.ID, arg.Name, arg.Kind)
		}
	}
	related := map[string]bool{}
	for _, id := range d.RelatedIDs {
		if _, ok := registry[id]; !ok || related[id] {
			return fmt.Errorf("diagnostic %s has unknown/duplicate related ID %q", d.ID, id)
		}
		related[id] = true
	}
	return nil
}
