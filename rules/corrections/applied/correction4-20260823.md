# Correction 4 — inherited membership contracts are not revalidated against derived contracts

## Audit context

- Repository: `github.com/YoePro/sec`
- Repository baseline: `c515862`
- Audited: `2026-08-19`
- Primary rulebook: `rules/types/default_values.md`
- Related rulebook: `rules/types/contracts.md`
- Rulebook revision: **inferred revision 2.0**
  - `default_values.md` has no explicit document-revision metadata.
  - Repository history places its current canonical rewrite/update on 2026-08-12 to 2026-08-13, inside the revision-2 rulebook period.
  - Later specialized rulebooks take precedence on conflict.

## Classification

Implementation bug.

This is not a language-rule change.

The resulting contract set of a derived named type is a conjunction of inherited and newly declared contracts. Every value in an `in [...]` membership contract must satisfy every other contract on the resulting type. Invalid membership entries are declaration errors and must never be silently filtered when resolving a default.

## Affected code

Primarily:

- `internal/sema/contracts.go`
- `internal/sema/defaults.go`

## Normative requirement

For an `in [...]` contract:

- source order is semantic;
- every listed value must satisfy every other contract on the type;
- the first listed value is the implicit default;
- an invalid listed value makes the declaration invalid;
- invalid entries are not removed or skipped to manufacture another default.

Contracts inherited from a base named type remain part of the resulting semantic contract conjunction.

## Bug

When a named type derives from another constrained named type, Sema copies inherited contracts:

```go
typ.Contracts = append([]Contract(nil), baseType.Contracts...)
```

and then appends the new contracts.

However, membership cross-validation is driven only from the current declaration's AST contracts:

```go
a.checkMembershipContractValues(typ, contractNode)
```

`checkMembershipContractValues()` iterates over membership contracts found in `contractNode`. It therefore revalidates only `in [...]` lists written directly on the current declaration.

Inherited `MembershipContract` values already stored in `typ.Contracts` are not revalidated against contracts newly introduced by the derived type.

Default resolution later scans semantic membership values and chooses the first value that satisfies the complete contract set:

```go
for _, value := range membership.Values {
    if defaultConstantCompatible(typ, value) &&
       defaultConstantSatisfies(typ, value) {
        return MembershipDefault(value)
    }
}
```

That behavior can silently skip an inherited membership member that became invalid and select a later member instead.

## Example

```sec
type Base int in [1, 2]
type Derived Base even
```

Canonical result:

- `Derived` inherits `in [1, 2]`;
- `Derived` adds `even`;
- inherited membership value `1` violates `even`;
- therefore the declaration of `Derived` is invalid.

The compiler must reject the declaration.

Current behavior can instead:

1. omit membership revalidation because `Derived` has no newly written `in [...]` AST node;
2. reach default resolution with inherited `[1, 2]`;
3. skip `1` because it fails `even`;
4. select `2` as the derived default.

This changes the semantic first member of the inherited membership list into a filtered list, which the rulebook explicitly forbids.

## Required correction

After inheritance and application of all new contracts, validate **every semantic membership contract in the resulting type** against the complete resulting contract conjunction.

The validation must include membership contracts inherited through any number of named-type derivations.

A valid implementation should:

1. construct the complete resulting semantic contract set;
2. identify every `MembershipContract` in that set;
3. validate every member against all sibling/inherited/new contracts;
4. reject the current derived declaration if a newly introduced contract makes any inherited membership value invalid;
5. preserve source order exactly;
6. never repair the declaration by filtering members;
7. preserve the rule that the first membership value remains the implicit default when the declaration is valid.

## Diagnostic provenance

Ideally, semantic membership entries should retain source provenance sufficient to report:

- the derived contract that introduced the incompatibility; and
- the inherited membership value that is invalid under the derived contract.

If the current semantic representation cannot retain both locations, extend the membership-contract representation rather than losing the relationship permanently.

A diagnostic at the derived declaration is preferable to silently accepting the type, but the final implementation should provide related locations where practical.

## Required regression tests

Add tests covering at least:

1. inherited `in [1, 2]` plus derived `even` is rejected because `1` is invalid;
2. inherited membership plus a derived range rejects any inherited member outside the new range;
3. inherited membership plus a derived `multipleOf` rejects incompatible inherited members;
4. inherited membership plus a compatible derived contract is accepted;
5. when all inherited members remain valid, the first inherited member remains the default;
6. no later inherited member is selected merely because an earlier member violates a derived contract;
7. multi-level inheritance revalidates the complete inherited membership set;
8. diagnostics identify the derived declaration and, when provenance is available, the offending inherited membership value.

## Governance note

> **Applied 2026-08-23:** Cross-contract validation iterates the complete
> semantic membership set after inheritance and local contract application.
> Invalid inherited members reject the derivation without filtering or changing
> source-ordered default selection.

The governance ledger already records substantial in-list default and cross-contract support. This correction is narrower:

> cross-contract validation does not currently cover inherited membership entries when a derived named type adds new constraints.

Do not downgrade unrelated default-resolution support.
