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
type Resolver struct {
	plan       ResolvedScalarPlan
	key        CompilationPlanLayoutKey
	alignments ScalarAlignments
	memo       map[string]LayoutID
	layouts    []ResolvedLayout
	inProgress []string
}

// NewResolver binds a resolver to one validated plan and its scalar
// alignment facts (nil until MD-141 is decided).
func NewResolver(plan ResolvedScalarPlan, alignments ScalarAlignments) (*Resolver, error) {
	key, err := LayoutKeyOf(plan, alignments)
	if err != nil {
		return nil, err
	}
	copied := ScalarAlignments(nil)
	if alignments != nil {
		copied = ScalarAlignments{}
		for size, alignment := range alignments {
			copied[size] = alignment
		}
	}
	return &Resolver{plan: plan, key: key, alignments: copied, memo: map[string]LayoutID{}}, nil
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
		return r.publish(r.resolveScalar(base, requirement), key)
	case LayoutStruct:
		r.inProgress = append(r.inProgress, key)
		cycle := ""
		for _, field := range requirement.Fields {
			child := r.layouts[r.resolve(field.Requirement)-1]
			if child.Completeness == LayoutInvalid && strings.HasPrefix(strings.Join(child.Missing, ""), "by-value layout cycle") {
				cycle = child.Missing[0]
			}
		}
		r.inProgress = r.inProgress[:len(r.inProgress)-1]
		if id, done := r.memo[key]; done {
			// A cycle through this struct already published it as invalid.
			return id
		}
		if cycle != "" {
			base.Completeness, base.Missing = LayoutInvalid, []string{cycle}
			return r.publish(base, key)
		}
		base.Completeness, base.Missing = LayoutUnsupported, []string{"struct field placement (P24-19)"}
		return r.publish(base, key)
	default:
		base.Completeness, base.Missing = LayoutUnsupported, []string{fmt.Sprintf("layout kind %q (later P24 stage)", requirement.Kind)}
		return r.publish(base, key)
	}
}

// resolveScalar selects the representation of one scalar (P24-16): sizes
// from the semantic widths, the plan width for int/uint/float, and the
// alignment from the plan's scalar alignment facts. bool and rune storage
// sizes and every scalar alignment are undecided (MD-141), so those layouts
// stay incomplete instead of guessed; decimals are P24-17.
// Rules: rules/memory/layout.md — §§ 18, 18(4a), 18(5); rules/types/types.md — §§ 9.2–9.4.
func (r *Resolver) resolveScalar(layout ResolvedLayout, requirement LayoutRequirement) ResolvedLayout {
	width := uint64(requirement.WidthBits)
	if requirement.TargetSized {
		width = uint64(r.plan.PointerWidthBits)
	}
	layout.Provenance = "scalar:" + string(requirement.Scalar)
	switch requirement.Scalar {
	case ScalarByte, ScalarChar:
		width = 8
		layout.ValidBitPattern = "all 8-bit patterns"
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
		layout.ValidBitPattern = fmt.Sprintf("IEEE binary%d", width)
	case ScalarBool:
		layout.ValidBitPattern = "false or true only"
		layout.Missing = append(layout.Missing, "storage size of bool (MD-141)")
	case ScalarRune:
		layout.ValidBitPattern = "Unicode scalar values only (U+0000..U+10FFFF without surrogates)"
		layout.Missing = append(layout.Missing, "storage size of rune (MD-141)")
	case ScalarDecimal:
		layout.Completeness, layout.Missing = LayoutUnsupported, []string{"decimal carrier layout (P24-17)"}
		return layout
	default:
		layout.Completeness, layout.Missing = LayoutInvalid, []string{fmt.Sprintf("unknown scalar kind %q", requirement.Scalar)}
		return layout
	}
	if len(layout.Missing) == 0 {
		layout.SizeBytes = new(big.Int).SetUint64(width / 8)
		alignment, known := r.alignments[width/8]
		if !known {
			layout.Missing = append(layout.Missing, fmt.Sprintf("alignment of a %d-byte scalar on %s (MD-141)", width/8, r.key.Triple))
		} else {
			layout.AlignBytes = new(big.Int).SetUint64(alignment)
		}
	}
	if len(layout.Missing) != 0 {
		layout.Completeness = LayoutIncomplete
		return layout
	}
	layout.Completeness = LayoutComplete
	return layout
}
