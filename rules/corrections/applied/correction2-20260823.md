# Correction 2 — direct LLVM operator lowering loses semantic signedness and string-comparison semantics

## Audit context

- Repository: `github.com/YoePro/sec`
- Repository baseline: `c515862`
- Audited: `2026-08-19`
- Primary rulebook: `rules/foundations/operators.md`
- Rulebook revision: **inferred revision 2.x**
  - The file has no explicit document-revision metadata.
  - Its history shows substantial updates during the revision-2 rulebook period, including changes on 2026-08-13, 2026-08-14, and 2026-08-18.
  - Later specialized rulebooks and the implementation governance ledger take precedence where they are newer or more specific.

## Classification

Implementation bug.

This is not a proposal to change Sec semantics.

The maintained direct LLVM path currently emits operator behavior that can differ from the semantic operation already accepted by Sema. A backend may reject unsupported lowering, but it must not silently lower an accepted Sec operation with different semantics.

## Affected code

`internal/codegen/llvm/expressions.go`

## Normative requirements

The operator rulebook requires:

- operator meaning to be resolved before backend lowering;
- backends to preserve Sec semantics rather than reinterpret operators;
- integer arithmetic and comparisons to retain signed/unsigned meaning;
- integer division to use the semantics of the resolved integer type;
- range membership to use the same resolved comparison semantics as the participating type;
- string equality to compare immutable string content;
- string ordering to be lexicographic by Unicode scalar sequence;
- unsupported backend lowering to fail explicitly rather than silently produce different semantics.

The newer Sec MLIR integer lowering already preserves signedness and therefore represents the later implementation authority for integer backend behavior.

## Bug 1 — unsigned integer division is lowered as signed division

`emitInfixExpression()` lowers every integer `/` as:

```go
return g.emitIntegerBinary("sdiv", left, right)
```

The legacy LLVM value representation does not retain semantic signedness. An unsigned integer such as `uint` is represented as an LLVM integer value, but division is still emitted with `sdiv`.

For values whose high bit is set, signed and unsigned division produce different results.

### Required correction

The direct LLVM lowering must consume resolved semantic integer signedness and select:

- `sdiv` for signed integer division;
- `udiv` for unsigned integer division.

If the direct LLVM backend cannot obtain the required semantic fact, it must reject the operation as unsupported rather than guess signedness.

## Bug 2 — unsigned ordered comparisons are lowered as signed comparisons

The same lowering emits:

```go
"<"  -> "slt"
"<=" -> "sle"
">"  -> "sgt"
">=" -> "sge"
```

for every integer-like operand.

This is incorrect for unsigned integer types.

### Required correction

Select predicates from the resolved semantic type:

- signed: `slt`, `sle`, `sgt`, `sge`;
- unsigned: `ult`, `ule`, `ugt`, `uge`.

Equality predicates `eq` and `ne` are independent of signedness.

## Bug 3 — range membership inherits the same signedness error

`emitMembershipExpression()` currently uses:

```go
sge
sle
slt
```

for range bounds regardless of the resolved type.

Consequently, membership on unsigned integer ranges can produce an incorrect result when values cross the signed high-bit boundary.

### Required correction

Range-membership comparisons must use the resolved signedness of the membership type exactly as ordinary ordered comparison does.

The left operand and range bounds must still be evaluated according to the canonical exact-once and source-order rules.

## Bug 4 — direct LLVM string comparison does not implement Sec string semantics

A string literal is represented internally by the legacy generator as:

```go
value{typ: "string", ref: pointer, lenRef: length}
```

but all `==`, `!=`, `<`, `<=`, `>`, and `>=` operations are sent to the generic integer-style:

```go
icmp <predicate> <left.typ> ...
```

This does not implement either:

- exact string-content equality; or
- Unicode-scalar lexicographic ordering.

With the current internal `"string"` pseudo-type it can also produce invalid LLVM textual IR rather than a valid comparison.

### Required correction

String comparison must have a dedicated lowering path.

At minimum:

- `==` / `!=` compare content, not pointer or descriptor identity;
- `<`, `<=`, `>`, `>=` implement the canonical lexicographic Unicode-scalar ordering;
- no allocation is introduced merely to compare;
- operands are evaluated exactly once and left-to-right.

If this functionality is intentionally unavailable in the direct LLVM backend, the backend must emit an explicit unsupported-lowering error rather than generic `icmp`.

## Architectural correction

The root problem is that the legacy `value` structure carries mostly physical LLVM type strings and loses semantic distinctions required by Sec.

Do not infer operator semantics solely from:

```go
value.typ
```

The backend should consume immutable Sema/Semantic-IR operator facts, including at least:

- semantic operand type;
- signedness;
- resolved operator kind;
- result type;
- failure behavior where applicable.

This is already the direction used by the Sec MLIR pipeline and should be preserved rather than reconstructing semantics in a backend.

## Required regression tests

Add direct-LLVM tests covering at least:

1. unsigned division with a value above the signed maximum;
2. unsigned `<`, `<=`, `>`, and `>=` across the signed high-bit boundary;
3. unsigned inclusive and exclusive range membership across that boundary;
4. signed comparison remains signed;
5. string equality for equal content stored at distinct locations;
6. string inequality for differing content;
7. string prefix ordering;
8. non-ASCII Unicode-scalar string ordering;
9. explicit unsupported-backend failure instead of wrong IR for any comparison still not implemented.

## Governance note

> **Applied 2026-08-23:** The direct LLVM value/local representation now retains
> represented unsigned integer semantics for division, compound division,
> ordered comparison, and range membership. String comparison fails explicitly
> until canonical Unicode-scalar content comparison is available.

The implementation ledger already records correct signed/unsigned integer handling in the newer Sec MLIR P7/P8/P9 path.

This correction therefore concerns the still-selectable direct LLVM backend and must not downgrade the implemented Sec MLIR integer status.
