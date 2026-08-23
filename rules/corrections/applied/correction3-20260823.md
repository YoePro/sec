# Correction 3 — contract-set consistency ignores earlier range and multipleOf constraints

## Audit context

- Repository: `github.com/YoePro/sec`
- Repository baseline: `c515862`
- Audited: `2026-08-19`
- Primary rulebook: `rules/types/contracts.md`
- Rulebook revision: **inferred revision 2.0**
  - The file has no explicit document-revision metadata.
  - Repository history places its canonical rewrite on 2026-08-12 to 2026-08-13, inside the revision-2 rulebook period.
  - Later specialized rulebooks take precedence if a conflict is found.

## Classification

Implementation bug.

This is not a language-rule change.

The canonical contract rule states that sequential contracts are an implicit conjunction and that the compiler rejects contract sets that are provably unsatisfiable. The current Sema consistency checker does not evaluate the complete conjunction when more than one `range` or more than one `multipleOf` contract is present.

## Affected code

`internal/sema/contracts.go`

Primary function:

```go
func (a *Analyzer) checkContractSetConsistency(typ Type, contractNode ast.Contract)
```

## Bug

The checker currently stores only one range and one divisor:

```go
var rangeContract *RangeContract
var multiple *big.Int
```

and then overwrites them while iterating through `typ.Contracts`:

```go
case RangeContract:
    c := contract
    rangeContract = &c

case MultipleOfContract:
    if contract.Value != nil && contract.Value.Sign() != 0 {
        multiple = new(big.Int).Abs(contract.Value)
    }
```

Therefore earlier constraints of the same family disappear from the consistency proof.

This contradicts the canonical rule that all contracts participate in one conjunction.

## Example 1 — disjoint ranges

This declaration is provably impossible:

```sec
type Impossible int range 1..10 range 20..30
```

No integer can satisfy both ranges.

The current consistency checker retains only the second range and observes that `20..30` contains valid integers, so the declaration can escape the required unsatisfiable-contract diagnostic.

## Example 2 — combined divisibility

This declaration is also provably impossible:

```sec
type Impossible int range 1..10 multipleOf 4 multipleOf 6
```

A value must be divisible by both 4 and 6, therefore by `lcm(4, 6) = 12`.

There is no such value in `1..10`.

The current checker retains only the final divisor, 6, sees that 6 lies in the range, and can incorrectly accept the contract set as satisfiable.

## Example 3 — parity combined with several divisors

The same defect can hide contradictions involving parity:

```sec
type Impossible int odd multipleOf 2 multipleOf 3
```

`multipleOf 2` makes every permitted value even, so the conjunction with `odd` is impossible.

If the final stored divisor is 3, the existing special-case check sees an odd divisor and misses the earlier `multipleOf 2`.

## Required correction

Consistency analysis must combine all contracts of each relevant family before deciding satisfiability.

For integer contracts, at minimum:

1. Intersect every range constraint.
   - Effective lower bound is the strongest lower bound.
   - Effective upper bound is the strongest upper bound.
   - Preserve inclusive/exclusive semantics.
   - Reject an empty intersection.

2. Combine every nonzero `multipleOf` divisor.
   - Divisor sign is irrelevant.
   - The effective divisibility step is the least common multiple of all absolute divisors.
   - Reject zero divisors through the existing declaration error path.

3. Combine parity constraints with the effective divisibility requirement.
   - `odd` plus any effective even divisor is unsatisfiable.
   - `odd` plus `even` remains unsatisfiable.

4. Test the resulting combined range/divisibility/parity constraint for at least one representable value.

5. Include inherited contracts from the named base type as well as contracts declared on the new named type, because `typ.Contracts` is the semantic conjunction.

The implementation should not rely on declaration order to choose which same-family contract survives.

## Algorithmic note

This proof does not require enumerating the whole range.

A compact exact integer implementation can use:

- bound intersection for ranges;
- `LCM` for `multipleOf`;
- parity folded into the effective modular requirement;
- arithmetic computation of the first satisfying value at or above the effective lower bound;
- comparison with the effective upper bound.

This avoids range-size-dependent iteration.

## Required regression tests

Add tests covering at least:

1. two overlapping ranges are accepted;
2. two disjoint ranges are rejected;
3. inclusive/exclusive range intersection at one boundary;
4. `range 1..10 multipleOf 4 multipleOf 6` is rejected;
5. `range 1..20 multipleOf 4 multipleOf 6` is accepted;
6. `odd multipleOf 2 multipleOf 3` is rejected;
7. `even multipleOf 3 multipleOf 5` remains satisfiable where the range permits a value;
8. inherited range plus newly declared range is intersected;
9. inherited `multipleOf` plus newly declared `multipleOf` is combined;
10. contract order does not change the result.

## Governance note

> **Applied 2026-08-23:** Contract consistency now intersects every inherited
> and local integer range, combines divisors by exact LCM, folds parity, and
> checks the resulting conjunction without range-size-dependent enumeration.

The current governance ledger does not explicitly record this defect.

Do not downgrade the already implemented primitive/default-resolution contract work broadly. This correction is specifically about declaration-time proof of consistency across multiple same-family integer contracts.
