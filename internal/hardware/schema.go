// Package hardware owns the shared compile-time hardware metadata schema. It
// does not execute device operations or turn declarations into reference Places.
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md §§2–8.
package hardware

// Category separates declaration identities even when addresses coincide (§2).
type Category string

const (
	RegionCategory   Category = "region"
	ResourceCategory Category = "resource"
	EndpointCategory Category = "endpoint"
)

// Member describes the closed member type and its conditional requiredness.
// Required is always, linear, memory-mapped, read, write, or enabled (MD-111 §§4–6).
type Member struct{ Name, Type, Required string }

// EnumCase separates source spelling from the exact manifest encoding (§7).
type EnumCase struct{ Name, Encoding string }

var schemas = map[Category][]Member{
	RegionCategory:   {{"domain", "string", "always"}, {"kind", "HardwareRegionKind", "always"}, {"base", "HardwareDomainAddress", "linear"}, {"size", "HardwareDomainExtent", "linear"}, {"readable", "bool", "always"}, {"writable", "bool", "always"}},
	ResourceCategory: {{"width", "uint", "always"}},
	EndpointCategory: {
		{"kind", "HardwareEndpointKind", "always"}, {"region", "HardwareRegionRef", "memory-mapped"}, {"resource", "HardwareResourceRef", "always"},
		{"offset", "HardwareDomainAddress", "memory-mapped"}, {"size", "HardwareDomainExtent", "memory-mapped"}, {"alignment", "uint", "memory-mapped"},
		{"read_widths", "list<uint>", "always"}, {"write_widths", "list<uint>", "always"},
		{"read_footprint", "HardwareFootprintKind", "read"}, {"write_footprint", "HardwareFootprintKind", "write"},
		{"read_effect", "HardwareReadEffect", "read"}, {"write_operation", "HardwareWriteOperation", "write"},
		{"ordering", "HardwareOrdering", "enabled"}, {"completion", "HardwareCompletion", "write"}, {"atomicity", "HardwareAtomicity", "enabled"},
		{"context", "HardwareAccessContext", "always"}, {"fault", "HardwareFaultContract", "always"}},
}
var enums = map[string][]EnumCase{
	"HardwareRegionKind":     {{"MMIO", "mmio"}, {"RAM", "ram"}, {"ROM", "rom"}, {"Reserved", "reserved"}},
	"HardwareEndpointKind":   {{"MemoryMapped", "memory-mapped"}, {"PortMapped", "port-mapped"}, {"InstructionEncoded", "instruction-encoded"}, {"RuntimeMapped", "runtime-mapped"}, {"TargetDefined", "target-defined"}},
	"HardwareFootprintKind":  {{"Whole", "whole"}},
	"HardwareReadEffect":     {{"NonDestructive", "non_destructive"}, {"ReadClear", "read_clear"}, {"ReadSet", "read_set"}},
	"HardwareWriteOperation": {{"Replace", "replace"}, {"SetBits", "set_bits"}, {"ClearBits", "clear_bits"}, {"ToggleBits", "toggle_bits"}},
	"HardwareOrdering":       {{"Volatile", "volatile"}}, "HardwareCompletion": {{"Issued", "issued"}},
	"HardwareAtomicity": {{"Unspecified", "unspecified"}}, "HardwareAccessContext": {{"Current", "current"}}, "HardwareFaultContract": {{"Platform", "platform"}},
}

// Schema returns an isolated copy of the authoritative member table (§§4–6).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func Schema(category Category) []Member { return append([]Member(nil), schemas[category]...) }

// Enum returns an isolated copy of the closed metadata enum (§§4A,7).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func Enum(name string) []EnumCase { return append([]EnumCase(nil), enums[name]...) }
