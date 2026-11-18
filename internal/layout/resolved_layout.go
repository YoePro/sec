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
	// The families below have record slots now; their resolution belongs to
	// later P24 stages and is reported as unsupported until then.
	LayoutArray      LayoutKind = "array"
	LayoutVariant    LayoutKind = "variant"
	LayoutDescriptor LayoutKind = "descriptor"
)

// LayoutStability states how far a layout may be relied on (layout.md
// § 39): native layouts are plan-local, explicit contracts are stable.
type LayoutStability string

const (
	LayoutPlanLocal LayoutStability = "plan-local"
	LayoutStable    LayoutStability = "stable"
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
	Element    LayoutID
	Count      *big.Int
	Variants   []ResolvedVariantLayout
	Descriptor string
	Stability  LayoutStability
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

// ScalarAlignments maps a scalar storage size in bytes to its alignment in
// bytes for one target. It is the "supported scalar alignments" plan fact of
// layout.md § 3(2); no target supplies it until MD-141 is decided, and a nil
// table leaves every scalar alignment unresolved.
type ScalarAlignments map[uint64]uint64

// canonical spells the table deterministically for the plan key.
func (a ScalarAlignments) canonical() string {
	if a == nil {
		return "unresolved"
	}
	sizes := make([]uint64, 0, len(a))
	for size := range a {
		sizes = append(sizes, size)
	}
	sort.Slice(sizes, func(i, j int) bool { return sizes[i] < sizes[j] })
	parts := make([]string, len(sizes))
	for index, size := range sizes {
		parts[index] = fmt.Sprintf("%d:%d", size, a[size])
	}
	return strings.Join(parts, ",")
}

// LayoutRevision is the compiler layout-rule revision folded into every plan
// key, so a rule change can never reuse an older cached result.
const LayoutRevision = "p24-resolver-1"

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
	Revision                                   string
}

// String is the canonical spelling used for cache keys and printing.
func (k CompilationPlanLayoutKey) String() string {
	return fmt.Sprintf("%s/%s/%s/%s/%s/p%d/%s/e%d/c{%s}/d{%s}/[%s]/%s", k.TargetOS, k.TargetArch, k.Triple, k.ABI, k.Profile,
		k.PointerWidthBits, k.Endianness, k.LogicalEpochBits, k.CABI, k.AddressDomains, k.ScalarAlignments, k.Revision)
}

// LayoutKeyOf builds the layout key of a validated scalar plan together with
// its scalar alignment facts.
func LayoutKeyOf(plan ResolvedScalarPlan, alignments ScalarAlignments) (CompilationPlanLayoutKey, error) {
	if err := plan.Validate(); err != nil {
		return CompilationPlanLayoutKey{}, fmt.Errorf("layout key requires a valid plan: %w", err)
	}
	for size, alignment := range alignments {
		if size == 0 || alignment == 0 || alignment&(alignment-1) != 0 {
			return CompilationPlanLayoutKey{}, fmt.Errorf("scalar alignment %d for size %d is not a positive power of two", alignment, size)
		}
	}
	return CompilationPlanLayoutKey{
		TargetOS: plan.TargetOS, TargetArch: plan.TargetArch, Triple: plan.LLVMTriple, ABI: plan.ABI, Profile: plan.Profile,
		PointerWidthBits: plan.PointerWidthBits, Endianness: plan.Endianness, LogicalEpochBits: plan.References.EpochBits(),
		CABI: fmt.Sprintf("%+v", plan.CABI), AddressDomains: fmt.Sprintf("%+v", plan.AddressDomains),
		ScalarAlignments: alignments.canonical(), Revision: LayoutRevision,
	}, nil
}
