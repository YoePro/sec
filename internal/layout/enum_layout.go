package layout

import (
	"fmt"
	"math/big"
	"strings"
)

// resolveEnum reuses an ordinary closed enum's selected integer storage while
// preserving nominal identity, aliases and the exact declared validity domain.
// Payload, bit and string enums require their own representation contracts.
// Rules: rules/memory/layout.md — §§14(1)–(6),31(2),47.1(3);
// rules/declarations/enums.md — §§14.1,19 "Lowering requirements".
func (r *Resolver) resolveEnum(base ResolvedLayout, key string, requirement LayoutRequirement) LayoutID {
	carrier := requirement.Carrier
	if carrier == nil || carrier.Kind != LayoutScalar || (carrier.Scalar != ScalarSigned && carrier.Scalar != ScalarUnsigned && carrier.Scalar != ScalarByte) {
		base.Completeness, base.Missing = LayoutUnsupported, []string{"enum without a selected primitive integer carrier (P24 enum representation)"}
		return r.publish(base, key)
	}
	storage := r.child(key, *carrier)
	if id, done := r.memo[key]; done {
		return id
	}
	base.Carrier = storage.ID
	if !storage.Complete() {
		base.Completeness, base.Missing = storage.Completeness, append([]string(nil), storage.Missing...)
		return r.publish(base, key)
	}
	width := carrier.WidthBits
	if carrier.TargetSized {
		width = r.plan.PointerWidthBits
	}
	if carrier.Scalar == ScalarByte {
		width = 8
	}
	minimum, maximum := new(big.Int), new(big.Int).Lsh(big.NewInt(1), uint(width))
	if carrier.Scalar == ScalarSigned {
		maximum.Rsh(maximum, 1)
		minimum.Neg(maximum)
	}
	maximum.Sub(maximum, big.NewInt(1))
	values, seen, names := []string{}, map[string]bool{}, map[string]bool{}
	for index, member := range requirement.EnumMembers {
		if member.Name == "" || names[member.Name] || member.Value == nil || member.Value.Cmp(minimum) < 0 || member.Value.Cmp(maximum) > 0 {
			base.Completeness, base.Missing = LayoutInvalid, []string{fmt.Sprintf("enum member %q is missing, duplicated or outside selected %d-bit carrier", member.Name, width)}
			return r.publish(base, key)
		}
		names[member.Name] = true
		value := member.Value.String()
		if !seen[value] {
			seen[value] = true
			values = append(values, value)
		}
		base.Variants = append(base.Variants, ResolvedVariantLayout{Index: index, Name: member.Name, Tag: new(big.Int).Set(member.Value)})
	}
	base.SizeBytes, base.AlignBytes = new(big.Int).Set(storage.SizeBytes), new(big.Int).Set(storage.AlignBytes)
	base.Completeness = LayoutComplete
	base.ValidBitPattern = "closed enum values {" + strings.Join(values, ",") + "}"
	base.Provenance = "enum over " + storage.TypeIdentity + " (layout.md §14)"
	return r.publish(base, key)
}
