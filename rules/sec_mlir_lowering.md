# Sec MLIR Lowering

## Status

Normative lowering specification.

Current lowering specification version: `4`

This document is subordinate to:

```text
rules/operators.md
rules/runtime_checks.md
rules/types.md
rules/layout.md
rules/semantic_ir.txt
rules/sec_mlir.md
rules/sec_mlir_dialect.md
```

It defines when resolved Sec MLIR semantics may be discharged into lower MLIR.

---

# 1. Version history

## Version 1

Trivial-core lowering.

## Version 2

Scalar coverage, plan-aware scalar resolution, canonical one-byte bool storage.

## Version 3

Checked integer semantic-lowering boundary.

## Version 4

Checked integer conversion to signless standard Arith while preserving explicit
Sec failure CFG and deterministic checked behavior.

---

# 2. Fundamental rule

A schema-v4 checked integer operation may lower to standard Arith only when the
lower computation is total for every input accepted by the Sec operation.

The lower computation must produce:

```text
result
failed
```

without:

```text
undefined behavior
poison as failure semantics
target shift-count masking
silent checked wrap
host-width truncation
```

The existing Sec failure branch remains explicit.

---

# 3. Checked guard precondition

Before lowering:

```text
--sec-verify-checked-integer-guards
```

must succeed.

Package 8 lowering must not consume malformed high-level checked CFG.

The lowering implementation should share/call the guard verifier rather than
trust external pass ordering only.

---

# 4. Signless core integer representation

After Package 8, plain builtin integer representation in the lowered core uses:

```text
i8
i16
i32
i64
i128
i256
```

Conversion:

```text
siN -> iN
uiN -> iN
```

for active builtin widths.

Signed/unsigned semantics are encoded in already-selected lower operations and
predicates.

---

# 5. Semantic provenance before type erasure

Before signedness type information is erased, compiler-generated ABI-relevant
boundaries preserve source scalar origin using:

```text
sec.scalar_kind
```

on `func.func` arguments/results.

Sec-origin scalar storage preserves the same metadata on its declaration or
lowered allocation.

Examples that become representation-identical but retain distinct provenance:

```text
int32
uint32

int128
uint128

byte
char
uint8
```

This is lowering provenance, not a substitute for nominal type identity.

---

# 6. Nominal boundary

Do not normalize inside:

```text
!sec.named
!sec.distinct
```

in lowering specification version 4.

---

# 7. Foreign boundary

`sec.call.foreign` remains explicit.

Its plain integer operand/result representation may become signless.

The target extern function retains `sec.scalar_kind` on arguments/results.

Foreign ABI lowering must use preserved semantic provenance and ABI contracts,
not infer signedness from signless `iN`.

---

# 8. No poison-producing assumptions

Package 8-generated checked integer lowering does not use:

```text
overflow<nsw>
overflow<nuw>
nneg
exact
```

as correctness requirements.

Those attributes are reserved for later optimization after proof.

---

# 9. Checked negation

For signed `iN`:

```text
failed = value == signedMinimum(N)
result = 0 - value
```

The subtraction uses ordinary modulo bitvector arithmetic with no poison flag.

The result is semantically consumed only when `failed=false`.

---

# 10. Unsigned checked addition

Use:

```text
arith.addui_extended
```

Map:

```text
Sec result = sum
Sec failed = overflow
```

---

# 11. Unsigned checked subtraction

Use:

```text
arith.subui_extended
```

Map:

```text
Sec result = difference
Sec failed = borrow
```

---

# 12. Signed checked addition

For source width `N`, sign-extend both operands to:

```text
i(N+1)
```

Perform ordinary `arith.addi`.

The widened sum is exact.

Failure:

```text
wide < signedMinimum(N)
or
wide > signedMaximum(N)
```

Result:

```text
truncate wide to iN
```

No overflow flags.

---

# 13. Signed checked subtraction

Use exact `i(N+1)` signed widening and signed range comparison.

No overflow flags.

---

# 14. Unsigned checked multiplication

Use:

```text
arith.mului_extended
```

Failure:

```text
highHalf != 0
```

Result:

```text
lowHalf
```

---

# 15. Signed checked multiplication

Use:

```text
arith.mulsi_extended
```

Failure when the high half is not the sign extension of the low half.

Canonical check:

```text
signFill = arithmeticRightShift(low, N - 1)
failed = high != signFill
```

Equivalent exact `i(2N)` widening is permitted.

---

# 16. Unsigned division

Failure:

```text
rhs == 0
```

Safe execution:

```text
safeRhs = select failed, 1, rhs
result = arith.divui lhs, safeRhs
```

The raw division never sees zero.

---

# 17. Signed division

Failure:

```text
rhs == 0
or
lhs == signedMinimum(N) && rhs == -1
```

Safe execution:

```text
safeRhs = select failed, 1, rhs
result = arith.divsi lhs, safeRhs
```

The raw operation never receives an undefined input.

No `exact` attribute.

---

# 18. Unsigned remainder

Failure:

```text
rhs == 0
```

Safe execution:

```text
safeRhs = select failed, 1, rhs
result = arith.remui lhs, safeRhs
```

---

# 19. Signed remainder

Use the same Sec failure set as signed division:

```text
rhs == 0
or
lhs == signedMinimum(N) && rhs == -1
```

Then:

```text
safeRhs = select failed, 1, rhs
result = arith.remsi lhs, safeRhs
```

This keeps Sec checked semantics independent of lower-level remainder edge-case
choices.

---

# 20. Shift-count validation

For value width `N`, count width `M`:

```text
K = max(N + 1, M + 1)
```

Extend count to `iK` using original semantic count signedness.

Failure:

```text
signed count < 0
count >= N
```

Create:

```text
safeCountWide = select invalid, 0, countWide
safeCount = truncate safeCountWide to iN
```

Every raw Arith shift receives `safeCount`.

---

# 21. Unsigned left shift

```text
result = arith.shli value, safeCount
failed = invalidCount
```

No overflow flags.

Discarding high bits is Sec-defined behavior.

---

# 22. Signed left shift

Use exact widened signed computation in `i(2N)`:

```text
wideValue = signExtend(value)
wideCount = zeroExtend(safeCount)
wideResult = arith.shli wideValue, wideCount
```

Failure:

```text
invalidCount
or
wideResult < signedMinimum(N)
or
wideResult > signedMaximum(N)
```

Result:

```text
truncate wideResult to iN
```

---

# 23. Signed right shift

```text
result = arith.shrsi value, safeCount
failed = invalidCount
```

No `exact`.

---

# 24. Unsigned right shift

```text
result = arith.shrui value, safeCount
failed = invalidCount
```

---

# 25. Bitwise lowering

```text
sec.int.bit_not -> arith.xori with all-ones
and -> arith.andi
or  -> arith.ori
xor -> arith.xori
```

---

# 26. Comparison lowering

Equality:

```text
eq
ne
```

Ordered signed:

```text
slt
sle
sgt
sge
```

Ordered unsigned:

```text
ult
ule
ugt
uge
```

Signedness is selected from the schema-v4 semantic operation before type
normalization.

---

# 27. Unary plus

Lower to identity.

---

# 28. Existing failure CFG

Package 8 does not reconstruct failure control flow.

The schema-v4:

```text
cf.cond_br failed, failure, success
```

remains.

Value replacement connects generated `failed` to the same branch.

`sec.fail.arithmetic` remains unchanged.

---

# 29. Standard type conversion

Convert plain builtin integer types consistently in:

```text
func.func
block arguments
func.call
cf branch operands
func.return operands
Sec-origin rank-zero scalar memrefs
sec.call.direct
sec.call.foreign
plain integer arith.constant
schema-v4 integer op inputs/results
```

Do not recurse into nominal Sec types.

---

# 30. Wide integer rule

All arithmetic construction uses arbitrary-width MLIR/LLVM APInt facilities.

Required widths:

```text
8
16
32
64
128
256
```

Temporary widths include:

```text
N+1
2N
max(N+1, M+1)
```

Therefore implementations must support temporary widths such as:

```text
257
512
```

No fixed host-width arithmetic is valid.

---

# 31. Post-lowering state

After successful Package 8:

```text
schema-v4 sec.int.* ops are absent
plain lowered integer core values use signless iN
checked failure flags remain i1
checked failure branches remain
sec.fail.arithmetic remains
sec.call.foreign remains
nominal integer types remain when present
decimal/string remain high-level
```

---

# 32. Optimization independence

Correctness does not depend on:

```text
canonicalization
CSE
inlining
SCCP
DCE
```

Future optimization may simplify safe substitutions or overflow checks only
after proof.

---

# 33. Completion rule

Lowering specification version 4 is implemented when:

```text
all schema-v4 builtin integer ops lower to standard Arith
no raw div/rem can receive invalid divisor
no raw shift can receive invalid count
checked failure flags match Sec semantics
signed/unsigned comparison semantics are preserved
active 128/256-bit integers are fully covered
ABI-relevant scalar provenance survives signless normalization
sec.fail.arithmetic remains explicit
no LLVM dialect is generated
```
