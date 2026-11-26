package layout

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
)

// The canonical resolved-layout records (P24-10). A ResolvedLayout is the one
// immutable answer to "how is this concrete semantic type represented under
// this CompilationPlan": its exact size and alignment, completeness, kind,
// scalar provenance, valid bit patterns, fields and the plan key it belongs
// to. Identity is the semantic type identity plus the plan key and layout
// contract, never a display name or host address.
//
// Rules:
//   - rules/memory/layout.md — §§ 3, 5–9, 18, 39
//   - rules/mlir/packages/sec-mlir-dialect_package24.md — §§ 7–11

// LayoutID identifies one resolved layout inside one resolver snapshot.
type LayoutID uint32

// LayoutCompleteness is the resolver state of a layout (P24-13): complete
// layouts may be materialized; incomplete, invalid and unsupported ones may
// not, and never receive a guessed size or alignment (layout.md § 5(5)).
type LayoutCompleteness string

const (
	LayoutComplete    LayoutCompleteness = "complete"
	LayoutIncomplete  LayoutCompleteness = "incomplete"
	LayoutInvalid     LayoutCompleteness = "invalid"
	LayoutUnsupported LayoutCompleteness = "unsupported"
)

// LayoutKind is the representation family of a layout.
type LayoutKind string

const (
	LayoutScalar LayoutKind = "scalar"
	LayoutStruct LayoutKind = "struct"
	LayoutEnum   LayoutKind = "enum"
	// The families below have record slots now; their resolution belongs to
	// later P24 stages and is reported as unsupported until then.
	// LayoutNamed is a named or constrained type over its carrier (P24-18).
	LayoutNamed      LayoutKind = "named"
	LayoutArray      LayoutKind = "array"
	LayoutVariant    LayoutKind = "variant"
	LayoutDescriptor LayoutKind = "descriptor"
	// LayoutAtomic is the storage of Atomic[T] over its contained carrier
	// (MD-095; layout.md § 47.6).
	LayoutAtomic LayoutKind = "atomic"
)

// LayoutStability states the scope of a proven representation guarantee.
// Completeness, equal byte layouts and a requested contract do not prove it.
// Rules: rules/memory/layout.md §§33-35,39.
type LayoutStability string

const (
	LayoutStabilityUnknown LayoutStability = "unknown"
	LayoutNativeUnstable   LayoutStability = "native-unstable"
	LayoutTargetStable     LayoutStability = "target-stable"
	LayoutContractStable   LayoutStability = "contract-stable"
	// Compatibility names; callers must use the recorded scope and reason.
	LayoutPlanLocal = LayoutNativeUnstable
	LayoutStable    = LayoutContractStable
)

// ResolvedVariantLayout is one variant of an enum/union layout: its tag
// value and payload layout, kept separate from the payload's offset.
type ResolvedVariantLayout struct {
	Index   int
	Name    string
	Tag     *big.Int
	Payload LayoutID
	Offset  *big.Int
}

// ScalarKind preserves the source scalar family, so semantic sign and type
// identity are not lowered to raw bits early (P24-16).
type ScalarKind string

const (
	ScalarBool     ScalarKind = "bool"
	ScalarByte     ScalarKind = "byte"
	ScalarChar     ScalarKind = "char"
	ScalarRune     ScalarKind = "rune"
	ScalarSigned   ScalarKind = "signed"
	ScalarUnsigned ScalarKind = "unsigned"
	ScalarFloat    ScalarKind = "float"
	ScalarDecimal  ScalarKind = "decimal"
)

// LayoutContractID names the layout contract a type is resolved under. The
// native contract is the default; explicit contracts belong to MD-019.
type LayoutContractID string

const NativeLayoutContract LayoutContractID = "native"

// LayoutRequirement is the plan-independent request for one concrete type:
// its semantic identity, family and the facts that select its
// representation. Width zero with TargetSized means the plan width.
type LayoutRequirement struct {
	TypeIdentity string
	Kind         LayoutKind
	Scalar       ScalarKind
	WidthBits    uint16
	TargetSized  bool
	Contract     LayoutContractID
	// Fields are the stored-field requirements of an aggregate, in
	// declaration order.
	Fields []FieldRequirement
	// Carrier is the representation a named or constrained type reuses, and
	// Validity its nominal/contract validity predicate (P24-18).
	Carrier  *LayoutRequirement
	Validity string
	// Element and Count describe a fixed array T[N] (P24-20).
	Element *LayoutRequirement
	Count   *big.Int
	// EnumMembers retains the closed declared value domain, including aliases.
	EnumMembers []EnumMemberRequirement
}

// EnumMemberRequirement is a resolved integer member, never a new tag assignment.
type EnumMemberRequirement struct {
	Name  string
	Value *big.Int
}

// FieldRequirement is one stored field of an aggregate requirement.
type FieldRequirement struct {
	Name        string
	Requirement LayoutRequirement
}

// ResolvedFieldLayout keeps field identity distinct from its offset.
type ResolvedFieldLayout struct {
	Index  int
	Name   string
	Offset *big.Int
	Layout LayoutID
}

// ResolvedLayout is one immutable published layout.
type ResolvedLayout struct {
	ID           LayoutID
	TypeIdentity string
	PlanKey      CompilationPlanLayoutKey
	Contract     LayoutContractID
	Completeness LayoutCompleteness
	Kind         LayoutKind
	Scalar       ScalarKind
	SizeBytes    *big.Int
	AlignBytes   *big.Int
	// ValidBitPattern states the representation-validity predicate, which
	// is distinct from the physical width (layout.md § 18(5)).
	ValidBitPattern string
	Fields          []ResolvedFieldLayout
	// Element and Count describe a fixed array; Variants an enum/union;
	// Descriptor names the descriptor family (slice, string, reference).
	Element LayoutID
	Count   *big.Int
	// Carrier is the layout a named or constrained type reuses.
	Carrier    LayoutID
	Variants   []ResolvedVariantLayout
	Descriptor string
	// Atomic is the access extent and guaranteed storage alignment of an
	// Atomic[T] layout, published separately from the object's own size
	// and alignment (layout.md § 47.6(4)); zero for every other kind.
	Atomic    ResolvedAtomicAccess
	Stability LayoutStability
	// StabilityReason records the owning guarantee or missing proof. PlanKey
	// and Contract retain the exact scope; no cross-plan compatibility follows.
	StabilityReason string
	// Missing names every fact that keeps the layout from being complete,
	// with the decision or package that owns it.
	Missing    []string
	Provenance string
}

// clone returns a detached copy, so published layouts stay immutable.
func (l ResolvedLayout) clone() ResolvedLayout {
	if l.SizeBytes != nil {
		l.SizeBytes = new(big.Int).Set(l.SizeBytes)
	}
	if l.AlignBytes != nil {
		l.AlignBytes = new(big.Int).Set(l.AlignBytes)
	}
	l.Fields = append([]ResolvedFieldLayout(nil), l.Fields...)
	for index := range l.Fields {
		if l.Fields[index].Offset != nil {
			l.Fields[index].Offset = new(big.Int).Set(l.Fields[index].Offset)
		}
	}
	if l.Count != nil {
		l.Count = new(big.Int).Set(l.Count)
	}
	l.Variants = append([]ResolvedVariantLayout(nil), l.Variants...)
	for index := range l.Variants {
		if l.Variants[index].Tag != nil {
			l.Variants[index].Tag = new(big.Int).Set(l.Variants[index].Tag)
		}
		if l.Variants[index].Offset != nil {
			l.Variants[index].Offset = new(big.Int).Set(l.Variants[index].Offset)
		}
	}
	l.Missing = append([]string(nil), l.Missing...)
	return l
}

// Complete reports whether the layout may be materialized.
func (l ResolvedLayout) Complete() bool { return l.Completeness == LayoutComplete }

// StorageClass is the physical representation class of a scalar storage
// fact: integer storage or IEEE binary floating-point storage. Integer and
// float storage of equal size are distinct facts (MD-141 § 3(2), § 3(9)).
type StorageClass string

const (
	IntegerStorage     StorageClass = "int"
	BinaryFloatStorage StorageClass = "float"
)

// StorageRepresentation identifies one scalar storage representation:
// IntegerStorage(8..256) or BinaryFloatStorage(32, 64). bool, byte and char
// use IntegerStorage(8); rune uses IntegerStorage(32) (MD-141 § 3(2)).
type StorageRepresentation struct {
	Class     StorageClass
	WidthBits uint16
}

// String spells the representation, for example "int32" or "float64", as
// the canonical plan-key spelling (MD-141 § 3(8)).
func (r StorageRepresentation) String() string { return fmt.Sprintf("%s%d", r.Class, r.WidthBits) }

// ScalarAlignments maps each scalar storage representation to its required
// ABI alignment in bytes for one target: the typed scalar alignment facts of
// layout.md § 3(2) and MD-141 § 3. A nil table, or a missing entry, leaves
// the alignment unresolved; nothing is derived from the byte width.
type ScalarAlignments map[StorageRepresentation]uint64

// canonical spells the table deterministically for the plan key.
func (a ScalarAlignments) canonical() string {
	if a == nil {
		return "unresolved"
	}
	keys := make([]StorageRepresentation, 0, len(a))
	for key := range a {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Class != keys[j].Class {
			return keys[i].Class < keys[j].Class
		}
		return keys[i].WidthBits < keys[j].WidthBits
	})
	parts := make([]string, len(keys))
	for index, key := range keys {
		parts[index] = fmt.Sprintf("%s:%d", key, a[key])
	}
	return strings.Join(parts, ",")
}

// LayoutRevision is the compiler layout-rule revision folded into every plan
// key, so a rule change can never reuse an older cached result.
const LayoutRevision = "p24-resolver-3-integer-enums"

// CompilationPlanLayoutKey converges every plan fact that can change a layout
// result (P24-11): target identity, pointer width, endianness, ABI and C ABI
// data model, profile, reference representation policy, address domains,
// scalar alignment facts and the compiler layout revision. Explicit layout
// contracts are part of each requirement's identity, not of the plan key. Two keys are compatible only when equal.
type CompilationPlanLayoutKey struct {
	TargetOS, TargetArch, Triple, ABI, Profile string
	PointerWidthBits                           uint16
	Endianness                                 Endianness
	LogicalEpochBits                           uint16
	CABI                                       string
	AddressDomains                             string
	ScalarAlignments                           string
	AtomicAlignments                           string
	Revision                                   string
}

// String is the canonical spelling used for cache keys and printing.
func (k CompilationPlanLayoutKey) String() string {
	return fmt.Sprintf("%s/%s/%s/%s/%s/p%d/%s/e%d/c{%s}/d{%s}/[%s]/a[%s]/%s", k.TargetOS, k.TargetArch, k.Triple, k.ABI, k.Profile,
		k.PointerWidthBits, k.Endianness, k.LogicalEpochBits, k.CABI, k.AddressDomains, k.ScalarAlignments, k.AtomicAlignments, k.Revision)
}

// LayoutKeyOf builds the layout key of a validated scalar plan together with
// its scalar alignment facts.
func LayoutKeyOf(plan ResolvedScalarPlan, alignments ScalarAlignments) (CompilationPlanLayoutKey, error) {
	if err := plan.Validate(); err != nil {
		return CompilationPlanLayoutKey{}, fmt.Errorf("layout key requires a valid plan: %w", err)
	}
	for representation, alignment := range alignments {
		if !validStorageRepresentation(representation) {
			return CompilationPlanLayoutKey{}, fmt.Errorf("scalar alignment fact for unknown storage representation %s", representation)
		}
		if alignment == 0 || alignment&(alignment-1) != 0 {
			return CompilationPlanLayoutKey{}, fmt.Errorf("scalar alignment %d for %s is not a positive power of two", alignment, representation)
		}
	}
	return CompilationPlanLayoutKey{
		TargetOS: plan.TargetOS, TargetArch: plan.TargetArch, Triple: plan.LLVMTriple, ABI: plan.ABI, Profile: plan.Profile,
		PointerWidthBits: plan.PointerWidthBits, Endianness: plan.Endianness, LogicalEpochBits: plan.References.EpochBits(),
		CABI: fmt.Sprintf("%+v", plan.CABI), AddressDomains: fmt.Sprintf("%+v", plan.AddressDomains),
		ScalarAlignments: alignments.canonical(), AtomicAlignments: plan.AtomicAlignments.canonical(), Revision: LayoutRevision,
	}, nil
}

// validStorageRepresentation accepts exactly IntegerStorage(8, 16, 32, 64,
// 128, 256) and BinaryFloatStorage(32, 64) (MD-141 § 3(2)).
func validStorageRepresentation(r StorageRepresentation) bool {
	switch r.Class {
	case IntegerStorage:
		switch r.WidthBits {
		case 8, 16, 32, 64, 128, 256:
			return true
		}
	case BinaryFloatStorage:
		return r.WidthBits == 32 || r.WidthBits == 64
	}
	return false
}
