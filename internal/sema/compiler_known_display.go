package sema

import "strings"

// CompilerKnownDisplayFacts is the shared user-facing registry projection.
// Label contains the public name qualified by a concrete resolved receiver
// when present. Receiver never substitutes a prose receiver pattern for a
// concrete type. An empty Name or Signature means that fact is unavailable;
// Label and ResultType still provide readable fallbacks. Stable registry IDs
// remain on the original symbol and are never used as display-name fallbacks.
// Consumers retain effects, authority, visibility and availability from that
// original symbol: these display facts do not grant access to an operation.
// Rules: rules/compiler/compiler_known_members.md — "Stable member identity",
// "LSP", "Synthetic definitions", "Named and related types";
// governance/tooling_lsp.yaml — tooling.lsp-compiler-known-friendly-names.
type CompilerKnownDisplayFacts struct {
	Name       string
	Label      string
	Kind       string
	Receiver   string
	Signature  string
	ResultType string
}

// DisplayFacts derives labels from resolved registry names, concrete receivers
// and the existing instantiated signature metadata, without an ID/name table.
// Rules: rules/compiler/compiler_known_members.md — "LSP", "Synthetic definitions".
func (member CompilerKnownMember) DisplayFacts() CompilerKnownDisplayFacts {
	member = withSyntheticMetadata(member)
	kind := "symbol"
	switch member.Kind {
	case CompilerKnownProperty:
		kind = "property"
	case CompilerKnownMethod:
		kind = "method"
	case CompilerKnownAssociatedFunction:
		kind = "associated function"
	case CompilerKnownValueExpression:
		kind = "value"
	}
	facts := compilerKnownDisplayFacts(member.Name, kind, member.Result)
	facts.Receiver = compilerKnownDisplayText(member.BoundReceiver)
	if facts.Name != "" && facts.Receiver != "" {
		facts.Label = facts.Receiver + "." + facts.Name
	}
	facts.Signature = compilerKnownDisplayText(member.Signature)
	if facts.Signature == "" && facts.Name != "" && facts.ResultType != "context-dependent" {
		switch member.Kind {
		case CompilerKnownProperty:
			facts.Signature = "property " + facts.Name + ": " + facts.ResultType
		case CompilerKnownValueExpression:
			facts.Signature = facts.Name + ": " + facts.ResultType
		}
		if facts.Signature != "" && member.Unsafe {
			facts.Signature = "unsafe " + facts.Signature
		}
	}
	return facts
}

// DisplayFacts renders a global function from its canonical parameter/result
// facts. A nil parameter list is contextual/unknown arity (len/fill), not proof
// of a zero-argument signature. A known empty list can represent zero arity.
// Rules: rules/compiler/compiler_known_members.md — "Global `len`",
// "Contextual `fill`", "LSP"; rules/declarations/functions.md — §§7,9.
func (function CompilerKnownFunction) DisplayFacts() CompilerKnownDisplayFacts {
	facts := compilerKnownDisplayFacts(function.Name, "function", function.Result)
	if facts.Name == "" || function.Parameters == nil || facts.ResultType == "context-dependent" {
		return facts
	}
	parameters := make([]string, 0, len(function.Parameters))
	for _, parameter := range function.Parameters {
		name := compilerKnownDisplayText(parameter.Name)
		parameterType := compilerKnownDisplayType(parameter.Type)
		if name == "" || parameterType == "context-dependent" {
			return facts
		}
		if parameter.Type.Kind != ReferenceType {
			if parameter.MutableRef {
				parameterType = "ref mut " + parameterType
			} else if parameter.Ref {
				parameterType = "ref " + parameterType
			}
		}
		if parameter.Variadic {
			parameterType = "..." + parameterType
		}
		if parameter.Consuming {
			name = "->" + name
		}
		parameters = append(parameters, name+": "+parameterType)
	}
	facts.Signature = "fn " + facts.Name + "(" + strings.Join(parameters, ", ") + ") " + facts.ResultType
	return facts
}

// DisplayFacts presents a value intrinsic using its source spelling and type;
// querying metadata does not bypass its internal/core visibility requirements.
// Rules: rules/compiler/compiler_known_members.md — "LSP",
// "Private core UTC wall-clock intrinsic".
func (value CompilerKnownValue) DisplayFacts() CompilerKnownDisplayFacts {
	facts := compilerKnownDisplayFacts(value.Name, "value", value.Result)
	if facts.Name != "" && facts.ResultType != "context-dependent" {
		facts.Signature = facts.Name + ": " + facts.ResultType
	}
	return facts
}

// compilerKnownDisplayFacts supplies consistent missing-name/type fallbacks
// for every registry family, without synthesizing a source declaration.
// Rules: rules/compiler/compiler_known_members.md — "Stable member identity", "LSP".
func compilerKnownDisplayFacts(name, kind string, result Type) CompilerKnownDisplayFacts {
	facts := CompilerKnownDisplayFacts{Name: compilerKnownDisplayText(name), Kind: kind, ResultType: compilerKnownDisplayType(result)}
	facts.Label = facts.Name
	if facts.Label == "" {
		facts.Label = "compiler-known " + kind
	}
	return facts
}

// compilerKnownDisplayType retains the canonical concrete type spelling and
// distinguishes unresolved/contextual result types from a real void result.
// Rules: rules/compiler/compiler_known_members.md — "LSP", "Named and related types".
func compilerKnownDisplayType(typ Type) string {
	if typ.Kind != "" && typ.Kind != InvalidType {
		if name := compilerKnownDisplayText(typeDisplayName(typ)); name != "" {
			return name
		}
	}
	return "context-dependent"
}

// compilerKnownDisplayText prevents transport placeholders from becoming
// friendly-name fallbacks. It does not resolve a name or decode an ID.
// Rules: rules/compiler/compiler_known_members.md — "Stable member identity", "LSP".
func compilerKnownDisplayText(text string) string {
	text = strings.TrimSpace(text)
	for _, prefix := range []string{"CKM-", "CKF-", "CKV-"} {
		if strings.Contains(text, prefix) {
			return ""
		}
	}
	return text
}
