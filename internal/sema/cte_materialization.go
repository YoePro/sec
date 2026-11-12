package sema

// cteMaterializationGraph validates semantic values before their storage
// handoff, independently of physical target layout and lowering support.
// Pointer keys are compiler bookkeeping only, never Sec value identities.
// Rules: rules/compiler/compile_time_evaluation.md — §§11,30–37,48,56–57.
type cteMaterializationGraph struct {
	active map[*CompileTimeAggregate]string
	nodes  int
}

// prepareCTEMaterialization checks the complete reachable value graph and
// copies accepted aggregates without evaluator liveness identities. Unknown
// representations fail closed; symbolic static references/relocations retain
// their separate owning validation and physical-materialization boundaries.
// Rules: rules/compiler/compile_time_evaluation.md — §§28(5–6),29–37.
func prepareCTEMaterialization(value DefaultConstant, typ Type) (DefaultConstant, *cteFailure) {
	graph := cteMaterializationGraph{active: map[*CompileTimeAggregate]string{}}
	return graph.value(value, typ, "value", 0)
}

// reject records a deterministic stored-value path and a distinct failure
// category: invalid evaluator state is a compiler invariant, missing support
// is ImplementationGap, and a computed graph without a static representation
// is ResultNotMaterializable rather than an evaluation failure.
// Rules: rules/compiler/compile_time_evaluation.md — §§30,36,54,56–57,59.
func (g *cteMaterializationGraph) reject(kind cteFailureKind, path, reason string) (DefaultConstant, *cteFailure) {
	return DefaultConstant{}, &cteFailure{Kind: kind, Reason: path + ": " + reason}
}

// value validates one stored value in its canonical concrete type; only the
// active payload is stored in a tagged value. Traversal has deterministic
// limits and declaration order, independent of Go map iteration order.
// Rules: rules/compiler/compile_time_evaluation.md — §§11,31–36,48–49.
func (g *cteMaterializationGraph) value(value DefaultConstant, typ Type, path string, depth int) (DefaultConstant, *cteFailure) {
	g.nodes++
	if depth > cteMaxCallDepth {
		return g.reject(cteResourceLimit, path, "materialization graph traversal reached the CallDepth compiler budget")
	}
	if g.nodes > cteMaxAggregateValueSize {
		return g.reject(cteResourceLimit, path, "materialization graph traversal reached the AggregateValueSize compiler budget")
	}
	switch typ.Kind {
	case "", InvalidType, GenericType, GenericTypePackType, AnyType, InterfaceType, ErrorRootType:
		return g.reject(cteImplementationGap, path, "no concrete static value representation for "+typeDisplayName(typ))
	case ReferenceType, RawPtrType, SliceType, StringViewType:
		return g.reject(cteResultNotMaterializable, path, "reference, pointer or view has no validated program storage identity or target relocation; transient backing storage cannot escape")
	case FunctionType, VariadicPackType:
		return g.reject(cteImplementationGap, path, "static materialization of "+typeDisplayName(typ)+" is not represented by the evaluator")
	case ArrayType:
		return g.reject(cteImplementationGap, path, "fixed-array evaluator values have no canonical element representation yet")
	case NeverType:
		return g.reject(cteCompilerInvariantFailure, path, "never has no value to materialize")
	case VoidType:
		if value.Kind != VoidType || value.Aggregate != nil {
			return g.reject(cteCompilerInvariantFailure, path, "unexpected stored value for "+typeDisplayName(typ))
		}
		return value, nil
	}
	if dynamicCollection(typ) {
		return g.reject(cteResultNotMaterializable, path, "dynamic owning collection "+typeDisplayName(typ)+" has no defined static materialization")
	}
	if typ.Kind != StructType && typ.Kind != UnionType && typ.Kind != ResultType {
		if value.Aggregate != nil {
			return g.reject(cteCompilerInvariantFailure, path, "aggregate value supplied for scalar "+typeDisplayName(typ))
		}
		switch value.Kind {
		case "", InvalidType, ReferenceType, RawPtrType, SliceType, StringViewType, FunctionType, VariadicPackType, ArrayType, StructType, ResultType, UnionType:
			return g.reject(cteCompilerInvariantFailure, path, "non-scalar evaluator value supplied for scalar "+typeDisplayName(typ))
		}
		// Scalar validation/nominal legality belongs to typed Sema and the
		// executor's inDomain checks. Preserve the canonical value and type;
		// do not reinterpret bytes with the compiler host's ABI.
		switch typ.Kind {
		case BoolType, IntType, UintType, FloatType, DecimalType, EnumType, StringType, CharType, RuneType, UnitDimensionType:
			return value, nil
		default:
			return g.reject(cteImplementationGap, path, "no defined evaluator materialization for "+typeDisplayName(typ))
		}
	}
	aggregate := value.Aggregate
	if aggregate == nil {
		// Nominal payload-free union variants are stored directly by the
		// shared scalar/nominal constant evaluator.
		if typ.Kind == UnionType && value.Nominal.Owner == nominalConstantOwner(typ) && value.Nominal.Owner != "" {
			for _, variant := range typ.UnionVariants {
				if variant.Name == value.Nominal.Member && variant.Payload == nil && len(variant.PayloadFields) == 0 {
					return value, nil
				}
			}
		}
		return g.reject(cteCompilerInvariantFailure, path, "missing aggregate value for "+typeDisplayName(typ))
	}
	if first, cycle := g.active[aggregate]; cycle {
		return g.reject(cteResultNotMaterializable, path, "cyclic evaluator object graph reaches "+first+"; Sec defines no implicit static heap")
	}
	g.active[aggregate] = path
	defer delete(g.active, aggregate)
	if aggregate.Type.Kind != "" && canonicalTypeIdentity(aggregate.Type) != canonicalTypeIdentity(typ) {
		return g.reject(cteCompilerInvariantFailure, path, "aggregate type differs from its canonical stored type "+typeDisplayName(typ))
	}
	clean := aggregate.clone()
	clean.Owner = 0
	value.Aggregate = clean
	if typ.Kind == StructType {
		if aggregate.Payload != nil || aggregate.Variant != "" {
			return g.reject(cteCompilerInvariantFailure, path, "struct carries an unexpected tagged payload")
		}
		return g.fields(value, typ.Fields, path, depth)
	}
	if len(aggregate.Fields) != 0 || len(aggregate.Names) != 0 {
		return g.reject(cteCompilerInvariantFailure, path, "tagged value carries fields outside its active payload")
	}
	payloadType, fields, valid := cteMaterializedVariant(typ, aggregate.Variant)
	if !valid {
		return g.reject(cteCompilerInvariantFailure, path, "unknown active variant "+aggregate.Variant+" of "+typeDisplayName(typ))
	}
	payloadPath := path + "." + aggregate.Variant
	if fields != nil {
		if aggregate.Payload == nil || aggregate.Payload.Aggregate == nil {
			return g.reject(cteCompilerInvariantFailure, payloadPath, "missing field payload")
		}
		// Field-payload aggregates are structural carriers, not invented
		// source types. Their fields retain each declared Sec field type.
		carrier := *aggregate.Payload
		if first, cycle := g.active[carrier.Aggregate]; cycle {
			return g.reject(cteResultNotMaterializable, payloadPath, "cyclic evaluator object graph reaches "+first)
		}
		g.active[carrier.Aggregate] = payloadPath
		original := carrier.Aggregate
		carrier.Aggregate = original.clone()
		carrier.Aggregate.Owner = 0
		result, failure := g.fields(carrier, fields, payloadPath, depth+1)
		delete(g.active, original)
		if failure != nil {
			return DefaultConstant{}, failure
		}
		clean.Payload = &result
	} else if payloadType == nil || payloadType.Kind == VoidType {
		if aggregate.Payload != nil && (aggregate.Payload.Kind != VoidType || aggregate.Payload.Aggregate != nil) {
			return g.reject(cteCompilerInvariantFailure, payloadPath, "unexpected payload on a payload-free variant")
		}
		clean.Payload = nil
	} else {
		if aggregate.Payload == nil {
			return g.reject(cteCompilerInvariantFailure, payloadPath, "missing required payload")
		}
		payload, failure := g.value(*aggregate.Payload, *payloadType, payloadPath, depth+1)
		if failure != nil {
			return DefaultConstant{}, failure
		}
		clean.Payload = &payload
	}
	return value, nil
}

// fields validates every stored component in declaration order and rejects
// missing, duplicate or extra entries instead of silently dropping them.
// Rules: rules/compiler/compile_time_evaluation.md — §§32,36,57.
func (g *cteMaterializationGraph) fields(value DefaultConstant, fields []StructField, path string, depth int) (DefaultConstant, *cteFailure) {
	aggregate := value.Aggregate
	if aggregate.Payload != nil || aggregate.Variant != "" {
		return g.reject(cteCompilerInvariantFailure, path, "struct field carrier has an unexpected tagged payload")
	}
	if len(aggregate.Fields) != len(fields) || len(aggregate.Names) != len(fields) {
		return g.reject(cteCompilerInvariantFailure, path, "stored field set differs from the canonical declaration")
	}
	declared := map[string]bool{}
	for i, field := range fields {
		if aggregate.Names[i] != field.Name || declared[field.Name] {
			return g.reject(cteCompilerInvariantFailure, path, "stored field order differs from the canonical declaration")
		}
		declared[field.Name] = true
		fieldValue, ok := aggregate.Fields[field.Name]
		if !ok {
			return g.reject(cteCompilerInvariantFailure, path+"."+field.Name, "missing stored field")
		}
		clean, failure := g.value(fieldValue, field.Type, path+"."+field.Name, depth+1)
		if failure != nil {
			return DefaultConstant{}, failure
		}
		aggregate.Fields[field.Name] = clean
	}

	return value, nil
}

// cteMaterializedVariant reads the active payload type from the canonical
// Result, Option or user union declaration, preserving nested application E.
// Rules: rules/compiler/compile_time_evaluation.md — §§32(3),36,46.
func cteMaterializedVariant(typ Type, name string) (*Type, []StructField, bool) {
	if typ.Kind == ResultType && len(typ.TypeArgs) == 2 {
		if name == "Ok" {
			return &typ.TypeArgs[0], nil, true
		}
		if name == "Err" {
			return &typ.TypeArgs[1], nil, true
		}
		return nil, nil, false
	}
	if isOptionType(typ) && len(typ.TypeArgs) == 1 {
		if name == "Some" {
			return &typ.TypeArgs[0], nil, true
		}
		if name == "None" {
			return nil, nil, true
		}
		return nil, nil, false
	}
	for _, variant := range typ.UnionVariants {
		if variant.Name == name {
			return variant.Payload, variant.PayloadFields, true
		}
	}
	return nil, nil, false
}

// finishCTEMaterialization applies the graph boundary to an established
// result and retains its canonical type and dependencies on failure.
// Rules: rules/compiler/compile_time_evaluation.md — §§30(4–5),36,53–59.
func finishCTEMaterialization(result cteResult) cteResult {
	if result.Failure != nil {
		return result
	}
	value, failure := prepareCTEMaterialization(result.Value, result.Type)
	if failure != nil {
		result.Failure = failure
		result.Value = DefaultConstant{}
		return result
	}
	result.Value = value
	return result
}
