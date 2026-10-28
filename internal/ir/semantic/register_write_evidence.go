package semantic

import (
	"fmt"
	"math/big"
)

// checkRegisterWriteEvidence binds every numeric field proof to represented
// SSA values, their type/constant/mask/extraction domain and dominating guards.
// It checks transported evidence, never source names or independent Sema facts.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a (MD-114 §7).
func checkRegisterWriteEvidence(module *Module, fn *Function, dom map[BlockID]map[BlockID]bool) error {
	defs := map[ValueID]Operation{}
	types := map[ValueID]TypeID{}
	for _, p := range fn.Parameters {
		types[p.Value.ID] = p.Value.Type
	}
	for _, b := range fn.Blocks {
		for _, p := range b.Parameters {
			types[p.ID] = p.Type
		}
		for _, op := range b.Operations {
			for _, r := range op.Results {
				defs[r.ID] = op
				types[r.ID] = r.Type
			}
		}
	}
	original := func(id ValueID) ValueID {
		for depth := 0; depth < 64; depth++ {
			op := defs[id]
			if op.Kind != OpNumericConvertExact || len(op.Operands) != 1 {
				break
			}
			id = op.Operands[0]
		}
		return id
	}
	constant := func(id ValueID) *big.Int {
		op := defs[original(id)]
		if op.Kind == OpConstInt {
			return cloneBigInt(op.Integer)
		}
		if op.Kind == OpConstBool && op.Bool != nil {
			if *op.Bool {
				return big.NewInt(1)
			}
			return new(big.Int)
		}
		return nil
	}
	type interval struct{ low, high *big.Int }
	rangeOf := func(id ValueID) *interval {
		id = original(id)
		if c := constant(id); c != nil {
			return &interval{cloneBigInt(c), c}
		}
		op := defs[id]
		if op.Kind == OpRegisterValueExtract {
			reg, ok := registerDefinition(module, types[op.Operands[0]])
			if !ok || int(op.RegisterField) >= len(reg.Fields) {
				return nil
			}
			width := reg.Fields[op.RegisterField].Width
			return &interval{new(big.Int), new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(width)), big.NewInt(1))}
		}
		if op.Kind == OpIntBitExtract && op.Integer != nil && op.Integer.IsUint64() && op.Integer.Uint64() <= 256 {
			return &interval{new(big.Int), new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(op.Integer.Uint64())), big.NewInt(1))}
		}
		if op.Kind == OpIntBitwise && op.IntegerBitwise == IntegerBitwiseAnd {
			for _, operand := range op.Operands {
				if c := constant(operand); c != nil && c.Sign() >= 0 {
					return &interval{new(big.Int), c}
				}
			}
		}
		signed, width, ok := integerShape(module, types[id])
		if !ok || signed || width == 0 {
			return nil
		}
		return &interval{new(big.Int), new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(width)), big.NewInt(1))}
	}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind != OpRegisterValueUpdate || op.RegisterWrite.Basis == "typed" {
				continue
			}
			proof := op.RegisterWrite
			value := original(op.Operands[1])
			r := rangeOf(value)
			if r != nil && proof.Basis == "interval" {
				for _, guardBlock := range fn.Blocks {
					if guardBlock.ID == block.ID || !dom[block.ID][guardBlock.ID] {
						continue
					}
					for _, branch := range guardBlock.Operations {
						if branch.Kind != OpCondBranch || len(branch.Successors) != 2 || branch.Successors[0].Block == branch.Successors[1].Block {
							continue
						}
						yes, no := dom[block.ID][branch.Successors[0].Block], dom[block.ID][branch.Successors[1].Block]
						if yes == no {
							continue
						}
						compare := defs[branch.Operands[0]]
						if compare.Kind != OpIntCompare {
							continue
						}
						variable := original(compare.Operands[0])
						c := constant(compare.Operands[1])
						predicate := string(compare.IntegerCompare)
						if variable != value {
							variable = original(compare.Operands[1])
							c = constant(compare.Operands[0])
							predicate = map[string]string{"lt": "gt", "le": "ge", "gt": "lt", "ge": "le", "eq": "eq", "ne": "ne"}[predicate]
						}
						if variable != value || c == nil {
							continue
						}
						if no {
							predicate = map[string]string{"lt": "ge", "le": "gt", "gt": "le", "ge": "lt", "eq": "ne", "ne": "eq"}[predicate]
						}
						if predicate == "eq" || predicate == "ge" || predicate == "gt" {
							lower := cloneBigInt(c)
							if predicate == "gt" {
								lower.Add(lower, big.NewInt(1))
							}
							if r.low.Cmp(lower) < 0 {
								r.low = lower
							}
						}
						if predicate == "eq" || predicate == "le" || predicate == "lt" {
							upper := cloneBigInt(c)
							if predicate == "lt" {
								upper.Sub(upper, big.NewInt(1))
							}
							if r.high.Cmp(upper) > 0 {
								r.high = upper
							}
						}
					}
				}
			}
			if r == nil || r.low.Cmp(r.high) > 0 || r.low.Cmp(proof.Low) < 0 || r.high.Cmp(proof.High) > 0 {
				return fmt.Errorf("register.value.update field #%d write proof %s[%v..%v] is not established by replacement %%%d SSA evidence", op.RegisterField, proof.Basis, proof.Low, proof.High, value)
			}
			if proof.Basis == "constant" && (r.low.Cmp(r.high) != 0 || r.low.Cmp(proof.Low) != 0) {
				return fmt.Errorf("register.value.update constant proof contradicts replacement SSA")
			}
		}
	}
	return nil
}

// VerifyRegisterEmissionEvidence checks the currently representable SSA proof
// transport before native emission. Sema-approved IR remains inspectable when
// a future operation/CFG proof has no installed native encoding; this query
// reports that capability boundary instead of changing source-language legality.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a (MD-114 §7(8),§9(6)).
func VerifyRegisterEmissionEvidence(module *Module) error {
	for _, fn := range module.Functions {
		if fn.Extern || len(fn.Blocks) == 0 {
			continue
		}
		blocks := map[BlockID]*Block{}
		preds := map[BlockID][]BlockID{}
		for _, block := range fn.Blocks {
			blocks[block.ID] = block
			for _, op := range block.Operations {
				for _, edge := range op.Successors {
					preds[edge.Block] = append(preds[edge.Block], block.ID)
				}
			}
		}
		if err := checkRegisterWriteEvidence(module, fn, dominators(fn.Blocks[0].ID, blocks, preds)); err != nil {
			return fmt.Errorf("function %s: %w", fn.ID, err)
		}
	}
	return nil
}
