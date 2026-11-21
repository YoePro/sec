package layout

import (
	"fmt"
	"math/big"
	"strings"
)

// Resolver is the canonical layout service for one CompilationPlan
// (P24-10/13/14). It resolves each requirement once (deterministic
// memoization by type identity and contract), assigns stable LayoutIDs in
// resolution order, detects by-value containment cycles through an
// in-progress stack with the concrete path, and publishes only detached
// copies. It never guesses a size or alignment: a missing fact makes the
// layout incomplete with the owner of the missing fact named.
//
// Rules:
//   - rules/memory/layout.md — §§ 3, 5, 7–9, 18, 25, 39
//   - rules/mlir/packages/sec-mlir-dialect_package24.md — §§ 7–11, 16
//   - rules/corrections/applied/MD-141-MD-142-scalar-and-aggregate-layout-normative-correction-2026-11-20-20261121.md — §§ 2–7
type Resolver struct {
	plan       ResolvedScalarPlan
	key        CompilationPlanLayoutKey
	alignments ScalarAlignments
	memo       map[string]LayoutID
	layouts    []ResolvedLayout
	inProgress []string
	arithmetic *SizeArithmetic
}

// MaximumExtent is the largest byte extent of a materialized value under a
// plan: 2^(W-1) - 1 for pointer width W (layout.md § 7(7)).
func MaximumExtent(plan ResolvedScalarPlan) *big.Int {
	limit := new(big.Int).Lsh(big.NewInt(1), uint(plan.PointerWidthBits-1))
	return limit.Sub(limit, big.NewInt(1))
}

// NewResolver binds a resolver to one validated plan and its typed scalar
// alignment facts (MD-141 § 3); nil leaves every scalar alignment unresolved.
func NewResolver(plan ResolvedScalarPlan, alignments ScalarAlignments) (*Resolver, error) {
	key, err := LayoutKeyOf(plan, alignments)
	if err != nil {
		return nil, err
	}
	copied := ScalarAlignments(nil)
	if alignments != nil {
		copied = ScalarAlignments{}
		for representation, alignment := range alignments {
			copied[representation] = alignment
		}
	}
	supported := []*big.Int{big.NewInt(1)}
	for _, alignment := range copied {
		supported = append(supported, new(big.Int).SetUint64(alignment))
	}
	arithmetic, err := NewSizeArithmetic(key.Triple+"/"+key.TargetArch, MaximumExtent(plan), supported)
	if err != nil {
		return nil, err
	}
	return &Resolver{plan: plan, key: key, alignments: copied, memo: map[string]LayoutID{}, arithmetic: arithmetic}, nil
}

// Key is the plan key every layout of this resolver carries.
func (r *Resolver) Key() CompilationPlanLayoutKey { return r.key }

// Resolve returns the layout of one requirement.
func (r *Resolver) Resolve(requirement LayoutRequirement) ResolvedLayout {
	return r.layouts[r.resolve(requirement)-1].clone()
}

// Layout returns a published layout by its identity.
func (r *Resolver) Layout(id LayoutID) (ResolvedLayout, bool) {
	if id == 0 || int(id) > len(r.layouts) {
		return ResolvedLayout{}, false
	}
	return r.layouts[id-1].clone(), true
}

// Layouts lists every published layout in identity order.
func (r *Resolver) Layouts() []ResolvedLayout {
	out := make([]ResolvedLayout, len(r.layouts))
	for index, layout := range r.layouts {
		out[index] = layout.clone()
	}
	return out
}

func memoKey(requirement LayoutRequirement) string {
	contract := requirement.Contract
	if contract == "" {
		contract = NativeLayoutContract
	}
	return requirement.TypeIdentity + "|" + string(contract)
}

func (r *Resolver) publish(layout ResolvedLayout, key string) LayoutID {
	layout.ID = LayoutID(len(r.layouts) + 1)
	layout.PlanKey = r.key
	r.layouts = append(r.layouts, layout)
	r.memo[key] = layout.ID
	return layout.ID
}

func (r *Resolver) resolve(requirement LayoutRequirement) LayoutID {
	key := memoKey(requirement)
	if id, done := r.memo[key]; done {
		return id
	}
	contract := requirement.Contract
	if contract == "" {
		contract = NativeLayoutContract
	}
	base := ResolvedLayout{TypeIdentity: requirement.TypeIdentity, Contract: contract, Kind: requirement.Kind, Scalar: requirement.Scalar, Stability: LayoutPlanLocal}
	if requirement.TypeIdentity == "" {
		base.Completeness, base.Missing = LayoutInvalid, []string{"layout requirement without a semantic type identity"}
		return r.publish(base, key)
	}
	if contract != NativeLayoutContract {
		base.Completeness, base.Missing = LayoutUnsupported, []string{"explicit layout contract " + string(contract) + " (MD-019)"}
		return r.publish(base, key)
	}
	for index, active := range r.inProgress {
		if active == key {
			path := append(append([]string(nil), r.inProgress[index:]...), key)
			for position := range path {
				path[position] = strings.TrimSuffix(path[position], "|"+string(NativeLayoutContract))
			}
			base.Completeness = LayoutInvalid
			base.Missing = []string{"by-value layout cycle " + strings.Join(path, " -> ")}
			return r.publish(base, key)
		}
	}
	switch requirement.Kind {
	case LayoutScalar:
		if requirement.Scalar == ScalarDecimal {
			return r.resolveDecimal(base, key, requirement)
		}
		return r.publish(r.resolveScalar(base, requirement), key)
	case LayoutStruct:
		return r.resolveAggregate(base, key, requirement.Fields, "struct field placement and aggregate alignment (MD-142)")
	case LayoutNamed:
		return r.resolveNamed(base, key, requirement)
	case LayoutArray:
		return r.resolveArray(base, key, requirement)
	default:
		base.Completeness, base.Missing = LayoutUnsupported, []string{fmt.Sprintf("layout kind %q (later P24 stage)", requirement.Kind)}
		return r.publish(base, key)
	}
}

// resolveScalar selects the representation of one scalar (P24-16, MD-141):
// sizes from the semantic widths, the plan width for int/uint/float, bool as
// 1 byte and rune as 4 bytes, and the alignment from the plan's typed fact
// for the storage representation (bool/byte/char IntegerStorage(8), rune
// IntegerStorage(32)). A missing fact leaves the layout incomplete; the
// alignment is never derived from the byte width.
// Rules: rules/memory/layout.md — §§ 18, 18(4a), 18(5); rules/types/types.md — §§ 9.2–9.4;
// MD-141 correction §§ 2–3.
func (r *Resolver) resolveScalar(layout ResolvedLayout, requirement LayoutRequirement) ResolvedLayout {
	width := uint64(requirement.WidthBits)
	if requirement.TargetSized {
		width = uint64(r.plan.PointerWidthBits)
	}
	layout.Provenance = "scalar:" + string(requirement.Scalar)
	class := IntegerStorage
	switch requirement.Scalar {
	case ScalarByte, ScalarChar:
		width = 8
		layout.ValidBitPattern = "all 8-bit patterns"
	case ScalarBool:
		width = 8
		layout.ValidBitPattern = "0x00 (false) or 0x01 (true) only"
	case ScalarRune:
		width = 32
		layout.ValidBitPattern = "Unicode scalar values only (U+0000..U+10FFFF without surrogates)"
	case ScalarSigned, ScalarUnsigned:
		if width == 0 || width%8 != 0 {
			layout.Completeness, layout.Missing = LayoutInvalid, []string{fmt.Sprintf("integer width %d bits is not a whole number of bytes", width)}
			return layout
		}
		layout.ValidBitPattern = fmt.Sprintf("all %d-bit patterns", width)
	case ScalarFloat:
		if width != 32 && width != 64 {
			layout.Completeness, layout.Missing = LayoutInvalid, []string{fmt.Sprintf("float width %d is not binary32 or binary64", width)}
			return layout
		}
		class = BinaryFloatStorage
		layout.ValidBitPattern = fmt.Sprintf("IEEE binary%d", width)
	default:
		layout.Completeness, layout.Missing = LayoutInvalid, []string{fmt.Sprintf("unknown scalar kind %q", requirement.Scalar)}
		return layout
	}
	representation := StorageRepresentation{Class: class, WidthBits: uint16(width)}
	if !validStorageRepresentation(representation) {
		layout.Completeness, layout.Missing = LayoutUnsupported, []string{fmt.Sprintf("scalar storage representation %s (MD-141 § 3(2))", representation)}
		return layout
	}
	layout.SizeBytes = new(big.Int).SetUint64(width / 8)
	if _, err := r.arithmetic.CheckedAdd(layout.TypeIdentity, layout.SizeBytes, new(big.Int)); err != nil {
		layout.SizeBytes = nil
		layout.Completeness, layout.Missing = LayoutInvalid, []string{err.Error()}
		return layout
	}
	alignment, known := r.alignments[representation]
	if !known {
		layout.Completeness = LayoutIncomplete
		layout.Missing = []string{fmt.Sprintf("alignment of %s storage on %s (MD-141 § 3)", representation, r.key.Triple)}
		return layout
	}
	layout.AlignBytes = new(big.Int).SetUint64(alignment)
	layout.Completeness = LayoutComplete
	return layout
}

// child resolves a contained by-value requirement with key on the
// in-progress stack and reports whether that produced a by-value cycle.
func (r *Resolver) child(key string, requirement LayoutRequirement) ResolvedLayout {
	r.inProgress = append(r.inProgress, key)
	child := r.layouts[r.resolve(requirement)-1]
	r.inProgress = r.inProgress[:len(r.inProgress)-1]
	return child
}

func isCycle(layout ResolvedLayout) bool {
	return layout.Completeness == LayoutInvalid && len(layout.Missing) > 0 && strings.HasPrefix(layout.Missing[0], "by-value layout cycle")
}

// propagate folds a contained layout's state into its container: an invalid
// part makes the container invalid, an unsupported part unsupported, and an
// incomplete part contributes its missing facts.
func propagate(container *ResolvedLayout, part ResolvedLayout, path string) {
	switch part.Completeness {
	case LayoutInvalid:
		if container.Completeness != LayoutInvalid {
			container.Completeness, container.Missing = LayoutInvalid, nil
		}
		for _, missing := range part.Missing {
			if strings.HasPrefix(missing, "by-value layout cycle") {
				container.Missing = append(container.Missing, missing)
			} else {
				container.Missing = append(container.Missing, path+": "+missing)
			}
		}
	case LayoutUnsupported:
		if container.Completeness != LayoutInvalid {
			if container.Completeness != LayoutUnsupported {
				container.Completeness, container.Missing = LayoutUnsupported, nil
			}
			for _, missing := range part.Missing {
				container.Missing = append(container.Missing, path+": "+missing)
			}
		}
	case LayoutIncomplete:
		if container.Completeness == "" || container.Completeness == LayoutIncomplete {
			container.Completeness = LayoutIncomplete
			for _, missing := range part.Missing {
				container.Missing = appendOnce(container.Missing, missing)
			}
		}
	}
}

func appendOnce(list []string, value string) []string {
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}

// resolveAggregate resolves every stored field in declaration order and
// places them with the canonical native algorithm (MD-142 § 6): cursor C=0,
// alignment A=1; for each field Offset = AlignUp(C, Ai), C = Offset + Size,
// A = max(A, Ai); then Size = AlignUp(C, A). Field identity stays separate
// from its offset, a zero-sized field occupies no bytes, and an empty struct
// has size 0 and alignment 1 (layout.md § 24(6)). Every step is checked
// against the plan's maximum byte extent. Explicit pack/minAlign/byteOffset
// contracts (MD-019) and target aggregate constraints are not modeled by the
// native contract and never reach this path.
// Rules: rules/memory/layout.md — §§ 7(7), 11, 13, 24(6), 25; MD-142 correction §§ 5–6.
func (r *Resolver) resolveAggregate(base ResolvedLayout, key string, fields []FieldRequirement, placement string) LayoutID {
	if len(fields) == 0 {
		base.Completeness, base.SizeBytes, base.AlignBytes = LayoutComplete, new(big.Int), big.NewInt(1)
		base.Provenance = "empty struct"
		return r.publish(base, key)
	}
	state := ResolvedLayout{}
	children := make([]ResolvedLayout, len(fields))
	for index, field := range fields {
		child := r.child(key, field.Requirement)
		children[index] = child
		base.Fields = append(base.Fields, ResolvedFieldLayout{Index: index, Name: field.Name, Layout: child.ID})
		propagate(&state, child, field.Name)
	}
	if id, done := r.memo[key]; done {
		// A cycle through this aggregate already published it as invalid.
		return id
	}
	if state.Completeness != "" {
		base.Completeness, base.Missing = state.Completeness, state.Missing
		if state.Completeness == LayoutIncomplete {
			base.Missing = append(base.Missing, placement)
		}
		return r.publish(base, key)
	}
	cursor, alignment := new(big.Int), big.NewInt(1)
	for index, child := range children {
		path := base.TypeIdentity + "." + fields[index].Name
		offset, err := r.arithmetic.AlignUp(path, cursor, child.AlignBytes)
		if err == nil {
			cursor, err = r.arithmetic.CheckedAdd(path, offset, child.SizeBytes)
		}
		if err != nil {
			base.Completeness, base.Missing = LayoutInvalid, []string{err.Error()}
			return r.publish(base, key)
		}
		base.Fields[index].Offset = offset
		if child.AlignBytes.Cmp(alignment) > 0 {
			alignment = new(big.Int).Set(child.AlignBytes)
		}
	}
	size, err := r.arithmetic.AlignUp(base.TypeIdentity, cursor, alignment)
	if err != nil {
		base.Completeness, base.Missing = LayoutInvalid, []string{err.Error()}
		return r.publish(base, key)
	}
	base.SizeBytes, base.AlignBytes, base.Completeness = size, alignment, LayoutComplete
	if base.Provenance == "" {
		base.Provenance = "native struct placement (MD-142 § 6)"
	}
	return r.publish(base, key)
}

// resolveDecimal resolves the canonical decimal carriers {i64,i32} and
// {i128,i32} (layout.md § 19) as a coefficient and a physical scale field.
// The public decimal scale is a language value, the i32 field only its
// storage. Any other width is an obsolete or truncating shape and invalid.
// Placement uses the ordinary native field algorithm with the fields in the
// fixed order coefficient, scale (MD-142 § 7(4)); no packed 12/20-byte size.
func (r *Resolver) resolveDecimal(base ResolvedLayout, key string, requirement LayoutRequirement) LayoutID {
	base.Provenance = "scalar:decimal"
	base.ValidBitPattern = "any coefficient; scale within the decimal scale range"
	if requirement.WidthBits != 64 && requirement.WidthBits != 128 {
		base.Completeness = LayoutInvalid
		base.Missing = []string{fmt.Sprintf("decimal carrier width %d is not the canonical i64 or i128 coefficient", requirement.WidthBits)}
		return r.publish(base, key)
	}
	coefficient := LayoutRequirement{TypeIdentity: fmt.Sprintf("int%d", requirement.WidthBits), Kind: LayoutScalar, Scalar: ScalarSigned, WidthBits: requirement.WidthBits}
	scale := LayoutRequirement{TypeIdentity: "int32", Kind: LayoutScalar, Scalar: ScalarSigned, WidthBits: 32}
	return r.resolveAggregate(base, key, []FieldRequirement{{Name: "coefficient", Requirement: coefficient}, {Name: "scale-storage", Requirement: scale}},
		fmt.Sprintf("decimal carrier {i%d,i32} placement (MD-142)", requirement.WidthBits))
}

// resolveNamed resolves a named or constrained type as its carrier layout
// plus its own nominal identity and validity provenance (P24-18). Reusing
// the carrier representation neither erases the nominal identity nor
// replaces the carrier's validity: the predicates are combined.
// Rules: rules/memory/layout.md — §§ 4, 31; rules/types/types.md — Named types.
func (r *Resolver) resolveNamed(base ResolvedLayout, key string, requirement LayoutRequirement) LayoutID {
	if requirement.Carrier == nil {
		base.Completeness, base.Missing = LayoutInvalid, []string{"named type without a resolved carrier"}
		return r.publish(base, key)
	}
	carrier := r.child(key, *requirement.Carrier)
	if id, done := r.memo[key]; done {
		return id
	}
	base.Carrier = carrier.ID
	base.Scalar = carrier.Scalar
	base.Completeness, base.Missing = carrier.Completeness, append([]string(nil), carrier.Missing...)
	if carrier.SizeBytes != nil {
		base.SizeBytes = new(big.Int).Set(carrier.SizeBytes)
	}
	if carrier.AlignBytes != nil {
		base.AlignBytes = new(big.Int).Set(carrier.AlignBytes)
	}
	base.Fields = carrier.clone().Fields
	base.ValidBitPattern = carrier.ValidBitPattern
	if requirement.Validity != "" {
		base.ValidBitPattern = strings.TrimPrefix(base.ValidBitPattern+"; ", "; ") + requirement.Validity
	}
	base.Provenance = "named over " + carrier.TypeIdentity
	return r.publish(base, key)
}

// resolveArray resolves T[N] as exactly N inline element slots (P24-20,
// MD-142 § 7(1)). The count is kept exactly and never expanded. The array
// alignment is the element alignment, the stride AlignUp(Size(T), Align(T))
// (zero for a zero-sized element) and the size N × stride; N == 0 gives
// size zero. Every step is checked against the maximum byte extent.
// Rules: rules/memory/layout.md — §§ 7(7), 9, 12, 24(4); MD-142 correction §§ 5(4), 7(1–3).
func (r *Resolver) resolveArray(base ResolvedLayout, key string, requirement LayoutRequirement) LayoutID {
	if requirement.Element == nil || requirement.Count == nil || requirement.Count.Sign() < 0 {
		base.Completeness, base.Missing = LayoutInvalid, []string{"fixed array without an element type or a nonnegative count"}
		return r.publish(base, key)
	}
	element := r.child(key, *requirement.Element)
	if id, done := r.memo[key]; done {
		return id
	}
	base.Element, base.Count = element.ID, new(big.Int).Set(requirement.Count)
	state := ResolvedLayout{}
	propagate(&state, element, "element")
	if state.Completeness != LayoutInvalid && element.SizeBytes != nil {
		if _, err := r.arithmetic.CheckedMul(base.TypeIdentity, element.SizeBytes, requirement.Count); err != nil {
			state.Completeness, state.Missing = LayoutInvalid, []string{err.Error()}
		}
	}
	if state.Completeness != "" {
		base.Completeness, base.Missing = state.Completeness, state.Missing
		if state.Completeness == LayoutIncomplete {
			base.Missing = append(base.Missing, "fixed-array stride and alignment (MD-142)")
		}
		return r.publish(base, key)
	}
	stride, err := r.arithmetic.StrideOf(base.TypeIdentity, element.SizeBytes, element.AlignBytes, new(big.Int))
	var size *big.Int
	if err == nil {
		size, err = r.arithmetic.ArraySize(base.TypeIdentity, stride, requirement.Count)
	}
	if err != nil {
		base.Completeness, base.Missing = LayoutInvalid, []string{err.Error()}
		return r.publish(base, key)
	}
	base.SizeBytes, base.AlignBytes, base.Completeness = size, new(big.Int).Set(element.AlignBytes), LayoutComplete
	base.Provenance = "native fixed array (MD-142 § 7(1))"
	return r.publish(base, key)
}
