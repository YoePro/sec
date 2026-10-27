package semantic

import (
	"fmt"
	"math/big"
	"strings"

	"sec/internal/ast"
	"sec/internal/sema"
)

// returnOrigins converts Sema's returned-reference origin summary of a
// function into positional Semantic IR metadata: parameter positions in the
// Semantic IR signature (the receiver, when present, is parameter 0) and P13
// field identities instead of source names (package15.md §§ 61-62;
// semantic_ir.md § 45(12)). An unknown origin or one the IR cannot represent
// yet is a package-15 boundary.
func (fb *functionBuilder) returnOrigins(resolved sema.Function, decl *ast.FunctionDeclaration) ([]ReturnOrigin, error) {
	summary, ok := fb.owner.analyzer.ReturnReferenceSummaryOf(resolved)
	if !ok || summary.Unknown || len(summary.Origins) == 0 {
		return nil, fb.unsupported("returned reference whose origin is unknown", decl.Token)
	}
	offset := len(fb.fn.Parameters) - len(resolved.Parameters)
	var origins []ReturnOrigin
	for _, source := range summary.Origins {
		origin := ReturnOrigin{Kind: ReturnOriginParameter}
		switch source.Kind {
		case sema.ReturnOriginParameter:
			origin.Parameter = offset + source.Parameter
		case sema.ReturnOriginReceiver:
			if offset != 1 {
				return nil, fb.unsupported("returned reference into a receiver without a receiver parameter", decl.Token)
			}
			origin.Kind = ReturnOriginReceiver
		case sema.ReturnOriginStatic:
			static, found := fb.owner.moduleStaticNamed(source.Static)
			if !found {
				return nil, fb.unsupported("returned reference to static storage of another module", decl.Token)
			}
			origin.Kind, origin.Static = ReturnOriginStatic, static.ID
		default:
			// Addressed storage has no Semantic IR root yet (P15-42a).
			return nil, fb.unsupported("returned reference to "+string(source.Kind)+" storage", decl.Token)
		}
		var current TypeID
		if origin.Kind == ReturnOriginStatic {
			current = fb.owner.module.Statics[origin.Static-1].Type
		} else {
			if origin.Parameter < 0 || origin.Parameter >= len(fb.fn.Parameters) {
				return nil, fmt.Errorf("returned-reference origin names parameter %d of %d", origin.Parameter, len(fb.fn.Parameters))
			}
			referent, ok := fb.owner.module.Types.Lookup(fb.fn.Parameters[origin.Parameter].Value.Type)
			if !ok || referent.Kind != TypeReference && referent.Kind != TypeSharedSlice && referent.Kind != TypeMutableSlice {
				return nil, fb.unsupported("returned reference derived from a non-reference parameter", decl.Token)
			}
			current = referent.Element
			if referent.Kind == TypeSharedSlice || referent.Kind == TypeMutableSlice {
				// A slice parameter is itself the indexed sequence.
				current = fb.fn.Parameters[origin.Parameter].Value.Type
			}
		}
		for _, projection := range source.Projections {
			switch projection.Canonical() {
			case sema.CanonicalProjectionSlice:
				sequence, found := fb.owner.module.Types.Lookup(current)
				if !found || sequence.Kind != TypeArray && sequence.Kind != TypeSharedSlice && sequence.Kind != TypeMutableSlice {
					return nil, fb.unsupported("returned slice range of a non-sequence", decl.Token)
				}
				step := ReturnOriginStep{Kind: ReturnOriginSlice, UnknownRange: true}
				if projection.SliceStart != nil && projection.SliceEnd != nil {
					step.UnknownRange = false
					step.Start, step.EndExclusive = cloneBigInt(projection.SliceStart), cloneBigInt(projection.SliceEnd)
				}
				origin.Path = appendReturnOriginPath(origin.Path, []ReturnOriginStep{step})
				current = fb.owner.module.Types.Intern(Type{Kind: TypeSharedSlice, Element: sequence.Element})
			case sema.CanonicalProjectionField:
				definition, found := fb.owner.structDefinition(current)
				if !found {
					return nil, fb.unsupported("returned reference through a field of a non-struct", decl.Token)
				}
				matched := false
				for _, field := range definition.Fields {
					if field.Name == projection.Name {
						origin.Path = append(origin.Path, ReturnOriginStep{Kind: ReturnOriginField, Field: field.ID})
						current, matched = field.Type, true
						break
					}
				}
				if !matched {
					return nil, fmt.Errorf("returned-reference origin field %s is not a stored field", projection.Name)
				}
			case sema.CanonicalProjectionConstant, sema.CanonicalProjectionDynamic:
				array, found := fb.owner.module.Types.Lookup(current)
				slice := found && (array.Kind == TypeSharedSlice || array.Kind == TypeMutableSlice)
				if !found || array.Kind != TypeArray && !slice {
					return nil, fb.unsupported("returned reference through an element of a non-array", decl.Token)
				}
				step := ReturnOriginStep{Kind: ReturnOriginElement}
				// A slice element is bounds-checked at run time even for a
				// constant index, so its origin stays the conservative [*]
				// (P16 § 67).
				if !slice && projection.Canonical() == sema.CanonicalProjectionConstant && projection.ConstantIndex != nil {
					step.Index = new(big.Int).Set(projection.ConstantIndex)
				}
				origin.Path = append(origin.Path, step)
				current = array.Element
			default:
				return nil, fb.unsupported("returned reference through a "+string(projection.Canonical())+" projection", decl.Token)
			}
		}
		origins = append(origins, origin)
	}
	return origins, nil
}

// formatReturnOrigins prints canonical root/path metadata without losing ranges.
// Rules: rules/compiler/semantic_ir.md — §45(12a); rules/mlir/sec_mlir_dialect.md — v12.34.
func formatReturnOrigins(origins []ReturnOrigin) string {
	parts := make([]string, 0, len(origins))
	for _, origin := range origins {
		var text strings.Builder
		if origin.Kind == ReturnOriginReceiver {
			text.WriteString("receiver")
		} else if origin.Kind == ReturnOriginStatic {
			fmt.Fprintf(&text, "static@%d", origin.Static)
		} else {
			fmt.Fprintf(&text, "param#%d", origin.Parameter)
		}
		for _, step := range origin.Path {
			switch {
			case step.Kind == ReturnOriginSlice:
				if step.UnknownRange {
					text.WriteString("[?]")
				} else {
					fmt.Fprintf(&text, "[%s..<%s]", step.Start, step.EndExclusive)
				}
			case step.Kind == ReturnOriginField:
				fmt.Fprintf(&text, ".field#%d", step.Field)
			case step.Index != nil:
				fmt.Fprintf(&text, "[%s]", step.Index.String())
			default:
				text.WriteString("[*]")
			}
		}
		parts = append(parts, text.String())
	}
	return " returns-from=[" + strings.Join(parts, ", ") + "]"
}

// verifyReturnOrigins requires a function returning a direct reference to
// carry its origin summary, and every returned reference to be traceable
// through copies, moves and reborrows to a parameter's reference and a
// projection path that the summary lists: a reference to local storage or of
// unknown derivation is rejected (semantic_ir.md § 96(5)).
func verifyReturnOrigins(module *Module, fn *Function) error {
	returned, ok := module.Types.Lookup(fn.ReturnType)
	if !ok || returned.Kind != TypeReference && returned.Kind != TypeSharedSlice && returned.Kind != TypeMutableSlice {
		if len(fn.ReturnOrigins) != 0 {
			return fmt.Errorf("returned-reference origins on a function that returns no reference")
		}
		return nil
	}
	if fn.Extern {
		return nil
	}
	if len(fn.ReturnOrigins) == 0 {
		return fmt.Errorf("function returning a reference has no returned-reference origin summary")
	}
	for _, origin := range fn.ReturnOrigins {
		for _, step := range origin.Path {
			if step.Kind == ReturnOriginSlice && (step.UnknownRange && (step.Start != nil || step.EndExclusive != nil) || !step.UnknownRange && (step.Start == nil || step.EndExclusive == nil || step.Start.Sign() < 0 || step.Start.Cmp(step.EndExclusive) > 0)) {
				return fmt.Errorf("invalid returned slice range")
			}
		}
		if err := verifyReturnOriginRangeType(module, fn, origin, returned); err != nil {
			return err
		}
	}
	definitions := map[ValueID]Operation{}
	parameters := map[ValueID]int{}
	for index, parameter := range fn.Parameters {
		parameters[parameter.Value.ID] = index
	}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			for _, result := range op.Results {
				definitions[result.ID] = op
			}
		}
	}
	functions := map[FunctionID]*Function{}
	for _, other := range module.Functions {
		functions[other.ID] = other
	}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind != OpReturn || len(op.Operands) != 1 {
				continue
			}
			traced, ok := traceReturnedReference(op.Operands[0], definitions, parameters, functions, 0)
			if !ok {
				return fmt.Errorf("returned reference %%%d does not derive from a reference parameter", op.Operands[0])
			}
			for _, origin := range traced {
				if !returnOriginListed(fn.ReturnOrigins, origin.Kind, origin.Static, origin.Parameter, origin.Path) {
					return fmt.Errorf("returned reference %%%d derives from %s, which the origin summary does not list", op.Operands[0], strings.TrimSuffix(strings.TrimPrefix(formatReturnOrigins([]ReturnOrigin{origin}), " returns-from=["), "]"))
				}
			}
		}
	}
	return nil
}

// traceReturnedReference follows a reference value back to the parameters
// whose references it may derive from, with the projection path below each:
// through copies, moves, reborrows, dereferences and field/element Places,
// and through a call whose callee summary maps its result to arguments.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.37,v12.34 (MD-085/115).
func traceReturnedReference(value ValueID, definitions map[ValueID]Operation, parameters map[ValueID]int, functions map[FunctionID]*Function, depth int) ([]ReturnOrigin, bool) {
	var reversed []ReturnOriginStep
	finish := func(origins []ReturnOrigin) []ReturnOrigin {
		suffix := make([]ReturnOriginStep, len(reversed))
		for i := range reversed {
			suffix[i] = reversed[len(reversed)-1-i]
		}
		for index := range origins {
			origins[index].Path = appendReturnOriginPath(origins[index].Path, suffix)
		}
		return origins
	}
	for {
		if index, ok := parameters[value]; ok {
			return finish([]ReturnOrigin{{Kind: ReturnOriginParameter, Parameter: index}}), true
		}
		op, ok := definitions[value]
		if !ok || len(op.Operands) == 0 && op.Kind != OpPlaceStaticRoot && op.Kind != OpDirectCall {
			return nil, false
		}
		switch op.Kind {
		case OpReferenceCopyShared, OpReferenceMove, OpReferenceReborrowShared, OpReferenceReborrowMutable, OpReferenceDerefPlace,
			// A fresh borrow is a valid returned origin only of static storage;
			// local value and storage roots end the trace unaccepted.
			OpReferenceBorrowShared, OpReferenceBorrowMutable:
			value = op.Operands[0]
		case OpSliceCopyShared, OpSliceMoveMutable:
			value = op.Operands[0]
		case OpSliceElementPlace:
			reversed = append(reversed, ReturnOriginStep{Kind: ReturnOriginElement})
			value = op.Operands[0]
		case OpSliceBorrowShared, OpSliceBorrowMutable, OpSliceReborrowShared, OpSliceReborrowMutable:
			step, unchanged, ok := returnedSliceRange(op, definitions, functions)
			if !ok {
				return nil, false
			}
			if !unchanged {
				reversed = append(reversed, step)
			}
			value = op.Operands[0]
		case OpPlaceField:
			reversed = append(reversed, ReturnOriginStep{Kind: ReturnOriginField, Field: op.StructField})
			value = op.Operands[0]
		case OpPlaceArrayElement:
			step := ReturnOriginStep{Kind: ReturnOriginElement}
			if op.ArrayCheckKind == "proven-safe" && op.Integer != nil {
				step.Index = op.Integer
			}
			reversed = append(reversed, step)
			value = op.Operands[0]
		case OpPlaceStaticRoot:
			return finish([]ReturnOrigin{{Kind: ReturnOriginStatic, Static: op.Static}}), true
		case OpDirectCall:
			callee := functions[op.Callee]
			if callee == nil || len(callee.ReturnOrigins) == 0 || depth > 64 {
				return nil, false
			}
			var origins []ReturnOrigin
			for _, calleeOrigin := range callee.ReturnOrigins {
				// A static origin names the same module's static at every
				// call site (MD-116 § 6(4)); it maps to no argument.
				if calleeOrigin.Kind == ReturnOriginStatic {
					origins = append(origins, ReturnOrigin{Kind: ReturnOriginStatic, Static: calleeOrigin.Static,
						Path: appendReturnOriginPath(nil, calleeOrigin.Path)})
					continue
				}
				if calleeOrigin.Parameter >= len(op.Operands) {
					return nil, false
				}
				argument, ok := traceReturnedReference(op.Operands[calleeOrigin.Parameter], definitions, parameters, functions, depth+1)
				if !ok {
					return nil, false
				}
				for _, origin := range argument {
					origin.Path = appendReturnOriginPath(origin.Path, calleeOrigin.Path)
					origins = append(origins, origin)
				}
			}
			return finish(origins), true
		default:
			return nil, false
		}
	}
}

// returnOriginListed compares detached canonical paths by exact numeric value.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.37,v12.34 (MD-085/115).
func returnOriginListed(origins []ReturnOrigin, kind ReturnOriginKind, static StaticID, parameter int, path []ReturnOriginStep) bool {
	for _, origin := range origins {
		if origin.Parameter != parameter || len(origin.Path) != len(path) {
			continue
		}
		if (origin.Kind == ReturnOriginStatic) != (kind == ReturnOriginStatic) || origin.Static != static {
			continue
		}
		same := true
		for index, step := range origin.Path {
			other := path[index]
			if step.Kind != other.Kind || step.Field != other.Field || step.UnknownRange != other.UnknownRange || !equalOriginInteger(step.Start, other.Start) || !equalOriginInteger(step.EndExclusive, other.EndExclusive) || (step.Index == nil) != (other.Index == nil) || step.Index != nil && step.Index.Cmp(other.Index) != 0 {
				same = false
				break
			}
		}
		if same {
			return true
		}
	}
	return false
}
