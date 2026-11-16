package semantic

import (
	"fmt"
	"sort"

	"sec/internal/sema"
)

// verifyHolderStorageJoins checks every read of a reference holder kept in
// mutable storage against each reference that can reach it, over all paths
// (rebinding and control-flow joins): the read must keep the stored
// references' validity policy, address space and provenance, name every
// origin storage they depend on, and, for a dynamic-epoch reference, the same
// invalidation domain. A reaching reference without facts fails closed.
// Rules: rules/memory/references.md — § 3.4(5); rules/memory/lifetime_analysis.md — § 11;
// rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 25–31.
func verifyHolderStorageJoins(fn *Function) error {
	facts := map[ValueID]*ReferenceMetadata{}
	for _, parameter := range fn.Parameters {
		if parameter.Reference != nil {
			facts[parameter.Value.ID] = parameter.Reference
		}
	}
	roots := map[ValueID]StorageID{}
	holderStorage := map[StorageID]bool{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Reference != nil && len(op.Results) == 1 {
				facts[op.Results[0].ID] = op.Reference
			}
			if op.Kind == OpPlaceStorageRoot && len(op.Results) == 1 {
				roots[op.Results[0].ID] = op.Storage
			}
		}
	}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpPlaceRead && op.PlaceReadMode == PlaceReadHolder && len(op.Operands) == 1 {
				if storage, stored := roots[op.Operands[0]]; stored {
					holderStorage[storage] = true
				}
			}
		}
	}
	if err := verifyHolderParameterJoins(fn, facts); err != nil {
		return err
	}
	if len(holderStorage) == 0 {
		return nil
	}
	// Reaching stores: per storage, the set of stored values on some path.
	type state map[StorageID]map[ValueID]bool
	clone := func(s state) state {
		out := state{}
		for storage, set := range s {
			copied := map[ValueID]bool{}
			for value := range set {
				copied[value] = true
			}
			out[storage] = copied
		}
		return out
	}
	transfer := func(block *Block, s state, visit func(Operation, state) error) error {
		for _, op := range block.Operations {
			if visit != nil {
				if err := visit(op, s); err != nil {
					return err
				}
			}
			if (op.Kind == OpStorageInit || op.Kind == OpStorageStore) && holderStorage[op.Storage] && len(op.Operands) == 1 {
				s[op.Storage] = map[ValueID]bool{op.Operands[0]: true}
			}
		}
		return nil
	}
	entry := map[BlockID]state{}
	if len(fn.Blocks) == 0 {
		return nil
	}
	start := fn.Blocks[0].ID
	if fn.Entry != 0 {
		start = fn.Entry
	}
	entry[start] = state{}
	for changed := true; changed; {
		changed = false
		for _, block := range fn.Blocks {
			in, reached := entry[block.ID]
			if !reached {
				continue
			}
			out := clone(in)
			_ = transfer(block, out, nil)
			if len(block.Operations) == 0 {
				continue
			}
			for _, successor := range block.Operations[len(block.Operations)-1].Successors {
				target, seen := entry[successor.Block]
				if !seen {
					entry[successor.Block] = clone(out)
					changed = true
					continue
				}
				for storage, set := range out {
					if target[storage] == nil {
						target[storage] = map[ValueID]bool{}
					}
					for value := range set {
						if !target[storage][value] {
							target[storage][value] = true
							changed = true
						}
					}
				}
			}
		}
	}
	check := func(op Operation, s state) error {
		if op.Kind != OpPlaceRead || op.PlaceReadMode != PlaceReadHolder || len(op.Operands) != 1 {
			return nil
		}
		storage, stored := roots[op.Operands[0]]
		if !stored {
			return nil
		}
		read := op.Reference
		reaching := make([]ValueID, 0, len(s[storage]))
		for value := range s[storage] {
			reaching = append(reaching, value)
		}
		sort.Slice(reaching, func(i, j int) bool { return reaching[i] < reaching[j] })
		if len(reaching) == 0 {
			return fmt.Errorf("holder read of $%d that no reference reaches", storage)
		}
		for _, value := range reaching {
			source := facts[value]
			if source == nil {
				return fmt.Errorf("holder read of $%d is reached by %%%d, which has no reference facts", storage, value)
			}
			// A joined read covers proven references of any known provenance.
			provenance := source.Provenance == read.Provenance || read.Provenance == string(sema.ReferenceProvenanceJoined) &&
				source.Validity == ReferenceProven && source.Provenance != string(sema.ReferenceProvenanceUnknown) && source.Provenance != string(sema.ReferenceProvenanceArena)
			if source.Validity != read.Validity || source.AddressSpace != read.AddressSpace || !provenance {
				return fmt.Errorf("holder read of $%d does not cover %%%d: validity, address space or provenance differ", storage, value)
			}
			if source.Validity == ReferenceDynamicEpoch && source.InvalidationDomain != read.InvalidationDomain {
				return fmt.Errorf("holder read of $%d joins dynamic-epoch references of different domains", storage)
			}
			for _, origin := range source.OriginStorages {
				covered := false
				for _, named := range read.OriginStorages {
					covered = covered || named == origin
				}
				if !covered {
					return fmt.Errorf("holder read of $%d omits origin storage #%d of %%%d", storage, origin, value)
				}
			}
		}
		return nil
	}
	for _, block := range fn.Blocks {
		if in, reached := entry[block.ID]; reached {
			if err := transfer(block, clone(in), check); err != nil {
				return err
			}
		}
	}
	return nil
}

// verifyHolderParameterJoins checks a holder read over a block parameter (a
// recovery or branch join of reference holders) against every incoming
// argument, like a read of a holder in storage.
func verifyHolderParameterJoins(fn *Function, facts map[ValueID]*ReferenceMetadata) error {
	parameters := map[ValueID]BlockID{}
	position := map[ValueID]int{}
	for _, block := range fn.Blocks {
		for index, parameter := range block.Parameters {
			parameters[parameter.ID] = block.ID
			position[parameter.ID] = index
		}
	}
	valueRoots := map[ValueID]ValueID{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpPlaceValueRoot && len(op.Operands) == 1 && len(op.Results) == 1 {
				valueRoots[op.Results[0].ID] = op.Operands[0]
			}
		}
	}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind != OpPlaceRead || op.PlaceReadMode != PlaceReadHolder || len(op.Operands) != 1 || op.Reference == nil {
				continue
			}
			joined, ok := valueRoots[op.Operands[0]]
			target, isParameter := parameters[joined]
			if !ok || !isParameter {
				continue
			}
			for _, predecessor := range fn.Blocks {
				for _, terminator := range predecessor.Operations {
					for _, successor := range terminator.Successors {
						if successor.Block != target || position[joined] >= len(successor.Arguments) {
							continue
						}
						if err := holderCovers(op.Reference, facts[successor.Arguments[position[joined]]], successor.Arguments[position[joined]]); err != nil {
							return fmt.Errorf("holder read of joined %%%d: %w", joined, err)
						}
					}
				}
			}
		}
	}
	return nil
}

// holderCovers reports whether read facts cover one reaching reference.
func holderCovers(read, source *ReferenceMetadata, value ValueID) error {
	if source == nil {
		return fmt.Errorf("reached by %%%d, which has no reference facts", value)
	}
	provenance := source.Provenance == read.Provenance || read.Provenance == string(sema.ReferenceProvenanceJoined) &&
		source.Validity == ReferenceProven && source.Provenance != string(sema.ReferenceProvenanceUnknown) && source.Provenance != string(sema.ReferenceProvenanceArena)
	if source.Validity != read.Validity || source.AddressSpace != read.AddressSpace || !provenance {
		return fmt.Errorf("does not cover %%%d: validity, address space or provenance differ (%s/%s vs %s/%s)", value, source.Validity, source.Provenance, read.Validity, read.Provenance)
	}
	if source.Validity == ReferenceDynamicEpoch && source.InvalidationDomain != read.InvalidationDomain {
		return fmt.Errorf("joins dynamic-epoch references of different domains")
	}
	for _, origin := range source.OriginStorages {
		covered := false
		for _, named := range read.OriginStorages {
			covered = covered || named == origin
		}
		if !covered {
			return fmt.Errorf("omits origin storage #%d of %%%d", origin, value)
		}
	}
	return nil
}
