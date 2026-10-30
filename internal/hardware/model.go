package hardware

import (
	"math/big"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/modules"
)

// Identity uses resolver-owned modules, never display names, files or addresses.
// Rules: MD-111 §2; rules/projects/modules.md §2.4.
type Identity struct {
	Module   modules.ModuleIdentity
	Category Category
	Name     string
}
type AddressDomainIdentity string
type MemoryRegionIdentity Identity
type HardwareResourceIdentity Identity
type HardwareEndpointIdentity Identity

// Later binding/planning producers must supply their own canonical keys; a
// declaration does not manufacture these identities (MD-111 §§2,11–12).
type HardwareBindingIdentity struct {
	Owner modules.CrossModuleDeclarationIdentity
}
type PhysicalOperationIdentity struct {
	Endpoint  HardwareEndpointIdentity
	Operation string
}
type VerifiedOperationPlanIdentity struct {
	Operation                  PhysicalOperationIdentity
	CompilationPlanID, ProofID string
}

// Origin retains source-stage provenance per property; it is not an identity.
// Rules: MD-111 §§2(4),3,13(2).
type Origin struct{ Token lexer.Token }
type Value struct {
	Kind     string // integer, string, bool, path, list
	Text     string
	Bool     bool
	Path     []string
	Elements []Value
	Origin   Origin
}
type Property struct {
	Name   string
	Value  Value
	Origin Origin
}

// Scope contains only resolver-selected imports. The adapter must obtain aliases
// from actual import statements, not a display-name search (MD-111 §8(3)).
type Scope struct {
	Module  modules.ModuleIdentity
	File    string
	Imports map[string][]modules.ModuleIdentity
}
type Declaration struct {
	Identity   Identity
	Scope      Scope
	Origin     Origin
	Properties []Property
}

// TargetFacts are positive selected inputs, not inferred device capabilities.
// EstablishedRegions restrict declarations to the target/Device's memory map.
// Rules: MD-111 §§1(6),4(2),6(1),12.
type TargetFacts struct {
	UintBits           uint16
	Domains            []layout.AddressDomain
	EstablishedRegions []Region
}

// ResolvedValue retains exact integers, set-normalized widths, symbolic identity,
// qualified enum spelling and the property's original source for tools (§§2–7).
type ResolvedValue struct {
	Source     Value // original list order and value spans, independent of normalized set order
	Type, Text string
	Bool       bool
	Integer    *big.Int
	Widths     []*big.Int
	Reference  Identity
	Origin     Origin
}
type Record struct {
	Identity   Identity
	Origin     Origin
	Properties map[string]ResolvedValue
}
type Region struct {
	Record
	Domain             layout.AddressDomain
	Kind               string
	Base, Size         *big.Int
	Readable, Writable bool
}
type Resource struct {
	Record
	Width *big.Int
}
type Endpoint struct {
	Record
	Region        MemoryRegionIdentity
	Resource      HardwareResourceIdentity
	Domain        layout.AddressDomain
	Address, Size *big.Int
}

// Facts are declaration facts only: no Place eligibility, atomicity proof,
// physical operation, issued I/O or verified operation plan is implied (§§4,6,11).
type Facts struct {
	UintBits  uint16
	Domains   []layout.AddressDomain
	Regions   []Region
	Resources []Resource
	Endpoints []Endpoint
}

// Issue categories are shared by all metadata consumers (§13(2)); adapter-owned
// diagnostic registries give each category its stable language diagnostic ID.
type Issue struct {
	Code, Message, Help string
	Origin, Related     Origin
	Warning             bool
}

// Clone snapshots all mutable fact payloads, including arbitrary-precision
// integers and per-property provenance. Rules: MD-111 §§2,12; incremental_compilation.md §15.
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func (f Facts) Clone() Facts {
	out := Facts{UintBits: f.UintBits, Domains: append([]layout.AddressDomain(nil), f.Domains...)}
	for _, r := range f.Regions {
		r.Record = cloneRecord(r.Record)
		r.Base = copyInt(r.Base)
		r.Size = copyInt(r.Size)
		out.Regions = append(out.Regions, r)
	}
	for _, r := range f.Resources {
		r.Record = cloneRecord(r.Record)
		r.Width = copyInt(r.Width)
		out.Resources = append(out.Resources, r)
	}
	for _, e := range f.Endpoints {
		e.Record = cloneRecord(e.Record)
		e.Address = copyInt(e.Address)
		e.Size = copyInt(e.Size)
		out.Endpoints = append(out.Endpoints, e)
	}
	return out
}

// cloneRecord isolates normalized per-property values (MD-111 §§2–3).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func cloneRecord(r Record) Record {
	props := make(map[string]ResolvedValue, len(r.Properties))
	for k, v := range r.Properties {
		v.Source = cloneValue(v.Source)
		v.Integer = copyInt(v.Integer)
		widths := make([]*big.Int, len(v.Widths))
		for i, w := range v.Widths {
			widths[i] = copyInt(w)
		}
		v.Widths = widths
		props[k] = v
	}
	r.Properties = props
	return r
}

// copyInt preserves exact domain values without sharing mutable integers (§3).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func copyInt(v *big.Int) *big.Int {
	if v == nil {
		return nil
	}
	return new(big.Int).Set(v)
}

// cloneValue retains original metadata spelling and element provenance (§3(6)).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func cloneValue(v Value) Value {
	v.Path = append([]string(nil), v.Path...)
	elements := make([]Value, len(v.Elements))
	for i, e := range v.Elements {
		elements[i] = cloneValue(e)
	}
	v.Elements = elements
	return v
}
