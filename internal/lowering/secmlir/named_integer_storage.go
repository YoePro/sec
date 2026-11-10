package secmlir

import "sec/internal/ir/semantic"

// storageScalarKind retains canonical integer carrier provenance on a named
// storage declaration without erasing its nominal element type. Physical
// lowering consumes this provenance at the same boundary as named values.
// Rules: rules/compiler/semantic_ir.md — §7(3–5);
// rules/types/types.md — §22.1(1–6);
// rules/corrections/applied/md087-p5-p6-p8-reference-boundaries-correction-20261021.md — §5(3–4).
func (e *emitter) storageScalarKind(id semantic.TypeID) (string, bool) {
	seen := map[semantic.TypeID]bool{}
	named := false
	for !seen[id] {
		seen[id] = true
		value, known := e.module.Types.Lookup(id)
		if !known {
			return "", false
		}
		if value.Kind != semantic.TypeNamed {
			if named {
				switch value.Kind {
				case semantic.TypeInt, semantic.TypeUint, semantic.TypeByte, semantic.TypeChar, semantic.TypeRune:
				default:
					return "", false
				}
			}
			return e.scalarKind(id)
		}
		named = true
		id = value.Base
	}
	return "", false
}
