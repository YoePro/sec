package semantic

import (
	"fmt"
	"strings"
)

// Derived destruction and remaining-subobject state (P17-30; package17
// §§ 11–15, 19–27, 59). Destruction plans form a closed, type-checked graph:
// a struct destroys its remaining owned fields in reverse declaration order
// (after its custom free), a fixed array one element plan an exact number of
// times (never one record per element), an active variant only its active
// payload. Partial moves of struct fields and active payloads are tracked by
// root Place identity and field/variant path; a destroy of the root lists the
// subobjects it skips. Arrays are never partially moved (§ 15).

// OwnedStep is one subobject step below a Place root: a stored field or the
// payload of a proven variant.
type OwnedStep struct {
	Payload bool
	Index   uint32
}

// OwnedPath is a subobject path below a Place root; empty is the whole value.
type OwnedPath []OwnedStep

func (path OwnedPath) String() string {
	parts := make([]string, len(path))
	for index, step := range path {
		if step.Payload {
			parts[index] = fmt.Sprintf("payload#%d", step.Index)
		} else {
			parts[index] = fmt.Sprintf("field#%d", step.Index)
		}
	}
	return strings.Join(parts, ".")
}

// overlaps reports whether one path is a prefix of the other.
func (path OwnedPath) overlaps(other OwnedPath) bool {
	shorter, longer := path, other
	if len(longer) < len(shorter) {
		shorter, longer = longer, shorter
	}
	for index := range shorter {
		if shorter[index] != longer[index] {
			return false
		}
	}
	return true
}

// trivialDestruction reports whether ending a value of the type needs no
// destruction action.
func trivialDestruction(module *Module, id TypeID) bool {
	typ, ok := module.Types.Lookup(id)
	if !ok {
		return false
	}
	switch typ.Kind {
	case TypeNamed:
		return trivialDestruction(module, typ.Base)
	case TypeArray:
		return typ.Length == "0" || trivialDestruction(module, typ.Element)
	case TypeStruct:
		definition, found := structDefinition(module, id)
		return found && definition.TriviallyDestructible
	case TypeUnion:
		for _, definition := range module.Unions {
			if definition.TypeID == id {
				return definition.TriviallyDestructible
			}
		}
		return false
	case TypeResult:
		return trivialDestruction(module, typ.Success) && trivialDestruction(module, typ.Error)
	}
	// Scalars and strings (trivially destructible, as Sema classifies them),
	// references and slices: a reference/slice ends its borrow and never
	// destroys the referent (§ 27).
	return true
}

// verifyDestructionPlanGraph checks that every plan is complete for its type
// and that every step names a plan of the stepped type.
func verifyDestructionPlanGraph(module *Module) error {
	plans := map[string]DestructionPlanRecord{}
	for _, plan := range module.DestructionPlans {
		plans[plan.ID] = plan
	}
	step := func(owner DestructionPlanRecord, id string, typ TypeID) error {
		target, ok := plans[id]
		if !ok {
			return fmt.Errorf("destruction plan %q steps to unknown plan %q", owner.ID, id)
		}
		if target.Type != typ {
			return fmt.Errorf("destruction plan %q steps to %q, which destroys !%d, not !%d", owner.ID, id, target.Type, typ)
		}
		return nil
	}
	for _, plan := range module.DestructionPlans {
		typ, _ := module.Types.Lookup(plan.Type)
		if plan.CustomFree && plan.Kind != "struct" {
			return fmt.Errorf("destruction plan %q: custom free belongs to a struct plan", plan.ID)
		}
		if plan.Kind != "struct" && len(plan.Fields) != 0 || plan.Kind != "fixed-array" && (plan.Element != "" || plan.ElementCount != "") ||
			plan.Kind != "active-variant" && len(plan.Variants) != 0 || plan.Kind != "compiler-known-resource" && plan.Resource != "" {
			return fmt.Errorf("destruction plan %q carries steps of another kind", plan.ID)
		}
		switch plan.Kind {
		case "trivial":
			if !trivialDestruction(module, plan.Type) {
				return fmt.Errorf("destruction plan %q is trivial for a type that needs destruction", plan.ID)
			}
		case "struct":
			definition, found := structDefinition(module, plan.Type)
			if !found {
				return fmt.Errorf("struct destruction plan %q destroys a non-struct", plan.ID)
			}
			listed := map[StructFieldID]bool{}
			for index, field := range plan.Fields {
				if int(field.Field) >= len(definition.Fields) || listed[field.Field] || index > 0 && field.Field >= plan.Fields[index-1].Field {
					return fmt.Errorf("struct destruction plan %q must list fields once in reverse declaration order", plan.ID)
				}
				listed[field.Field] = true
				if err := step(plan, field.Plan, definition.Fields[field.Field].Type); err != nil {
					return err
				}
			}
			for _, field := range definition.Fields {
				if !listed[field.ID] && !trivialDestruction(module, field.Type) {
					return fmt.Errorf("struct destruction plan %q omits non-trivial field #%d", plan.ID, field.ID)
				}
			}
		case "fixed-array":
			if typ.Kind != TypeArray || plan.Element == "" || plan.ElementCount != typ.Length {
				return fmt.Errorf("fixed-array destruction plan %q needs one element plan and the exact count", plan.ID)
			}
			if err := step(plan, plan.Element, typ.Element); err != nil {
				return err
			}
		case "active-variant":
			var variants []UnionVariantDefinition
			for _, definition := range module.Unions {
				if definition.TypeID == plan.Type {
					variants = definition.Variants
				}
			}
			if variants == nil {
				return fmt.Errorf("active-variant destruction plan %q destroys a non-union", plan.ID)
			}
			listed := map[UnionVariantIndex]bool{}
			for _, variant := range plan.Variants {
				if int(variant.Variant) >= len(variants) || listed[variant.Variant] || variants[variant.Variant].Payload == 0 {
					return fmt.Errorf("active-variant destruction plan %q names an invalid or payload-free variant", plan.ID)
				}
				listed[variant.Variant] = true
				if err := step(plan, variant.Plan, variants[variant.Variant].Payload); err != nil {
					return err
				}
			}
			for _, variant := range variants {
				if variant.Payload != 0 && !listed[variant.Index] && !trivialDestruction(module, variant.Payload) {
					return fmt.Errorf("active-variant destruction plan %q omits non-trivial variant #%d", plan.ID, variant.Index)
				}
			}
		case "compiler-known-resource":
			if plan.Resource == "" {
				return fmt.Errorf("resource destruction plan %q names no release contract", plan.ID)
			}
		default:
			return fmt.Errorf("destruction plan %q has unknown kind %q", plan.ID, plan.Kind)
		}
	}
	return nil
}

// ownedPlace resolves a Place SSA value to its root identity and subobject
// path. ok is false for a Place reached through an element step, which is
// never partially moved.
func ownedPlace(id ValueID, definitions map[ValueID]Operation) (string, OwnedPath, bool) {
	var reversed OwnedPath
	for {
		op, found := definitions[id]
		if !found {
			return fmt.Sprintf("value:%%%d", id), reverseOwnedPath(reversed), true
		}
		switch op.Kind {
		case OpPlaceField:
			reversed = append(reversed, OwnedStep{Index: uint32(op.StructField)})
			id = op.Operands[0]
		case OpPlaceUnionPayload:
			reversed = append(reversed, OwnedStep{Payload: true, Index: uint32(op.UnionVariant)})
			id = op.Operands[0]
		case OpPlaceStorageRoot:
			return fmt.Sprintf("storage:$%d", op.Storage), reverseOwnedPath(reversed), true
		case OpPlaceStaticRoot:
			return fmt.Sprintf("static:@%d", op.Static), reverseOwnedPath(reversed), true
		case OpPlaceArrayElement, OpSliceElementPlace:
			return "", nil, false
		default:
			return fmt.Sprintf("place:%%%d", id), reverseOwnedPath(reversed), true
		}
	}
}

func reverseOwnedPath(reversed OwnedPath) OwnedPath {
	path := make(OwnedPath, len(reversed))
	for index := range reversed {
		path[index] = reversed[len(reversed)-1-index]
	}
	return path
}

// verifySubobjectState tracks, per block, which subobjects of each Place root
// were moved out: a moved subobject cannot be moved, copied or replaced again
// until it is reinitialized; the whole value cannot be moved or copied while
// any part is moved; a partial move from a struct with custom free is
// rejected (§ 12); and destroying the root must skip at least every moved
// subobject, through a plan that permits skipping. Moves on other paths are
// the path-sensitive verifier's concern (P17-52).
func verifySubobjectState(module *Module, fn *Function) error {
	definitions := map[ValueID]Operation{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			for _, result := range op.Results {
				definitions[result.ID] = op
			}
		}
	}
	for _, block := range fn.Blocks {
		moved := map[string][]OwnedPath{}
		for _, op := range block.Operations {
			if !isOwnershipOperation(op.Kind) || len(op.Operands) == 0 {
				continue
			}
			placeTyp, ok := module.Types.Lookup(definitionsType(op.Operands[0], definitions, fn))
			if !ok || placeTyp.Kind != TypePlace {
				continue
			}
			root, path, tracked := ownedPlace(op.Operands[0], definitions)
			if !tracked {
				if op.Kind == OpOwnMoveFromPlace {
					return fmt.Errorf("own.move-from-place of an array element: fixed-array partial move is deferred (package17 § 15)")
				}
				continue
			}
			overlapping := func() (OwnedPath, bool) {
				for _, previous := range moved[root] {
					if previous.overlaps(path) {
						return previous, true
					}
				}
				return nil, false
			}
			switch op.Kind {
			case OpOwnMoveFromPlace, OpOwnCopyFromPlace, OpOwnReplacePlace:
				if previous, conflict := overlapping(); conflict {
					return fmt.Errorf("%s of %s %s overlaps moved-out subobject %q", op.Kind, root, path, previous.String())
				}
				if op.Kind == OpOwnMoveFromPlace {
					if len(path) > 0 {
						if err := partialMoveAllowed(module, op.Operands[0], definitions); err != nil {
							return err
						}
					}
					moved[root] = append(moved[root], path)
				}
			case OpOwnInitializePlace:
				remaining := moved[root][:0]
				for _, previous := range moved[root] {
					if len(previous) < len(path) || !previous.overlaps(path) {
						remaining = append(remaining, previous)
					}
				}
				moved[root] = remaining
			case OpOwnDestroyPlace:
				for _, previous := range moved[root] {
					if len(previous) == 0 {
						return fmt.Errorf("own.destroy-place of moved value %s", root)
					}
					if !containsOwnedPath(op.Ownership.Skipped, previous) {
						return fmt.Errorf("own.destroy-place of %s does not skip moved-out subobject %q", root, previous.String())
					}
				}
				if len(op.Ownership.Skipped) != 0 {
					plan, err := destructionPlan(module, op.Ownership.DestructionPlan, placeTyp.Element)
					if err != nil {
						return err
					}
					if plan.CustomFree || plan.Kind != "struct" && plan.Kind != "active-variant" {
						return fmt.Errorf("own.destroy-place skips subobjects through plan %q, which cannot skip", plan.ID)
					}
				}
				delete(moved, root)
			}
		}
	}
	return nil
}

func definitionsType(id ValueID, definitions map[ValueID]Operation, fn *Function) TypeID {
	if op, found := definitions[id]; found {
		for _, result := range op.Results {
			if result.ID == id {
				return result.Type
			}
		}
	}
	for _, parameter := range fn.Parameters {
		if parameter.Value.ID == id {
			return parameter.Value.Type
		}
	}
	return 0
}

// partialMoveAllowed rejects moving a field out of a struct whose plan runs
// custom free (§ 12; destruction.md § 15.4).
func partialMoveAllowed(module *Module, place ValueID, definitions map[ValueID]Operation) error {
	op := definitions[place]
	if op.Kind != OpPlaceField || len(op.Operands) != 1 {
		return nil
	}
	parent := definitions[op.Operands[0]]
	for _, result := range parent.Results {
		if result.ID != op.Operands[0] {
			continue
		}
		typ, _ := module.Types.Lookup(result.Type)
		for _, plan := range module.DestructionPlans {
			if plan.Type == typ.Element && plan.CustomFree {
				return fmt.Errorf("own.move-from-place of a field of %q: a struct with custom free cannot be partially moved", plan.ID)
			}
		}
	}
	return nil
}

func containsOwnedPath(paths []OwnedPath, path OwnedPath) bool {
	for _, candidate := range paths {
		if len(candidate) == len(path) && candidate.overlaps(path) {
			return true
		}
	}
	return false
}
