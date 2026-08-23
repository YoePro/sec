# Correction 5 — target-sized `int` and `uint` are not target-aware in Sema and legacy AST backends

## Audit context

- Repository: `github.com/YoePro/sec`
- Repository baseline: `c515862`
- Audited: `2026-08-19`
- Primary rulebook: `rules/types/types.md`
- Rulebook revision: **inferred revision 2.x**
  - The file has no explicit document-revision metadata.
  - It is the canonical replacement for the retired `types.txt`.
  - Its structure and synchronized Sec 0.1 decisions belong to the revision-2-era rulebook set.
  - Later specialized rulebooks and newer MLIR package rulebooks take precedence where they are more specific.

## Classification

Implementation bug.

This is not a language-rule change.

The canonical type rule states that `int` and `uint` are target-sized integer types whose physical width is selected by the target profile. The selected target must therefore participate in every semantic representability decision that depends on their width, and every maintained backend must emit the selected width or explicitly reject unsupported lowering.

The newer Semantic-IR/Sec-MLIR scalar-layout path already implements this correctly. The defect is in frontend Sema and the still-selectable legacy AST backends.

---

## Normative requirement

`rules/types/types.md` defines:

```text
int and uint are target-sized integer types.
Their physical width is selected by the target profile.
```

This implies:

1. `int` and `uint` use the target profile's integer width.
2. They use the same selected target width unless a later target rule explicitly defines otherwise.
3. Literal representability and compile-time overflow checks for these types must use that selected width.
4. Backend lowering must preserve that selected width.
5. Backend or host defaults must not redefine source-language type semantics.

---

# Bug 1 — Sema hard-codes 64-bit representability for `int` and `uint`

## Affected code

`internal/sema/types.go`

`builtinTypes()` currently initializes:

```go
"int":  signedType("int", -1<<63, 1<<63-1),
"uint": unsignedType("uint", ^uint64(0)),
```

Therefore the semantic type objects initially describe:

```text
int  = signed 64-bit representability
uint = unsigned 64-bit representability
```

independent of selected target.

`sema.Analyzer` has no target/profile/scalar-plan field, and:

```go
func NewAnalyzer() *Analyzer
```

constructs the builtin type table without target information.

## Compiler-driver confirmation

`cmd/compiler/main.go` receives a `CompilerTarget`, validates it, and resolves target-specific source selection, but then does:

```go
analyzer := sema.NewAnalyzer()
errors := analyzer.Analyze(program)
```

The selected target is not supplied to Sema.

Therefore frontend checks cannot distinguish, for example:

```text
linux-amd64
linux-armv7
```

when validating target-sized integer representability.

## Consequence

On a 32-bit target, source such as:

```sec
let value: int := 4_000_000_000
```

must not be accepted as an ordinary valid `int` literal merely because it fits signed 64-bit range.

Likewise, compile-time arithmetic, conversions, enum-underlying checks, named-type contracts, defaults, and any other semantic operation that depends on target-sized integer bounds must not use host-independent 64-bit bounds for a 32-bit target.

The current Sema model can accept or reason about such values using the wrong representability domain before later lowering resolves `!sec.int` to the real target width.

---

# Bug 2 — legacy direct LLVM lowering hard-codes inconsistent widths

## Affected code

`internal/codegen/llvm/types.go`

The direct LLVM type mapping currently contains:

```go
case "int":
    return "i32"
case "uint":
    return "i64"
```

This is incorrect in two ways:

1. neither mapping is selected from the target profile;
2. `int` and `uint` are assigned different widths.

`GenerateWithTriple()` stores the LLVM target triple, but `llvmReturnType()` does not consume it when resolving `int` or `uint`.

Consequently, merely changing `--target` does not select the canonical source width of either target-sized integer type in this backend.

---

# Bug 3 — legacy AST-to-LLVM-dialect MLIR lowering has the same width defect

## Affected code

`internal/codegen/mlir/generator.go`

Both:

```go
mlirType(...)
```

and:

```go
mlirBuiltinNumericType(...)
```

map:

```go
int  -> i32
uint -> i64
```

The generator stores `targetTriple`, but these type-resolution paths do not derive the integer width from it or from a canonical target scalar plan.

This backend therefore repeats the same incorrect source-type assumption as the direct LLVM backend.

---

# Correct later implementation already exists

The newer Semantic IR / Sec MLIR scalar-layout path is the authority for the intended architecture.

The governance entry:

```text
lowering.sec-mlir-scalar-layout
```

records an implemented `ResolvedScalarPlan` with explicit target/ABI/profile/pointer-width facts and target-specific resolution of `!sec.int` and `!sec.uint`.

P7/P8 verification also demonstrates:

```text
linux-armv7  -> target-sized integer resolves to 32 bit
linux-amd64  -> target-sized integer resolves to 64 bit
```

That implementation must remain the reference behavior.

---

# Required correction

## 1. Give Sema target-aware scalar facts

Sema must receive an immutable target semantic plan, or a smaller frontend-safe target scalar view derived from the canonical CompilationPlan/ResolvedScalarPlan.

At minimum it needs the semantic facts required for source validation:

```text
target int width
target uint width
signed minimum
signed maximum
unsigned maximum
```

Do not parse architecture strings independently inside Sema.

Do not duplicate target tables already owned by the compiler target/layout system.

## 2. Resolve builtin semantic bounds from that plan

`int` and `uint` semantic bounds must be instantiated from the selected target facts.

Fixed-width types remain target-independent:

```text
int32   always 32-bit signed
uint32  always 32-bit unsigned
...
```

Target-sized:

```text
int
uint
```

must use the selected target plan.

## 3. Audit every frontend use of integer bounds

After target-aware builtin types exist, verify at least:

- contextual literal shaping;
- explicit conversion;
- constant arithmetic overflow;
- shifts;
- enum underlying values;
- named types derived from `int`/`uint`;
- contracts and contract satisfiability;
- default resolution;
- compile-time constants;
- generic substitutions whose concrete type is `int`/`uint`.

No such code may fall back to a hard-coded host or 64-bit assumption.

## 4. Remove independent legacy backend width guesses

The direct LLVM and legacy MLIR generators must either:

### Preferred

consume the same resolved target scalar facts used by the maintained pipeline;

or:

### Temporary safe behavior

reject target-sized `int`/`uint` lowering where they cannot preserve the selected target semantics.

They must not continue mapping:

```text
int  -> i32
uint -> i64
```

independent of target.

## 5. Preserve source/backend separation

The physical core representation may become signless after semantic signedness has been preserved in facts/provenance, as the newer Sec MLIR pipeline already does.

The fix must not infer source semantics back from LLVM integer widths.

---

# Required regression tests

Add tests covering at least:

## Sema

1. maximum valid `int` on a 32-bit target is accepted;
2. one above maximum `int` on a 32-bit target is rejected;
3. minimum valid `int` on a 32-bit target is accepted;
4. one below minimum `int` on a 32-bit target is rejected;
5. maximum valid `uint` on a 32-bit target is accepted;
6. one above maximum `uint` on a 32-bit target is rejected;
7. the corresponding larger values are accepted on a 64-bit target where representable;
8. fixed-width `int64`/`uint64` behavior is unchanged between 32-bit and 64-bit targets;
9. target-sized named types inherit the selected bounds;
10. compile-time checked arithmetic uses selected target width;
11. conversion into target-sized integer types uses selected target width;
12. enum values with `int`/`uint` underlying type use selected target bounds.

## Backends

13. direct LLVM `int` width matches selected target if that backend remains supported;
14. direct LLVM `uint` width matches the same selected target width;
15. legacy MLIR `int` and `uint` match selected target if that backend remains supported;
16. no backend silently uses different widths for `int` and `uint`;
17. unsupported legacy target-sized lowering fails explicitly rather than emitting semantically wrong IR.

## Cross-layer

18. the same source rejected by 32-bit Sema but accepted by 64-bit Sema behaves consistently through Sec MLIR lowering;
19. no value accepted by target-aware Sema becomes unrepresentable merely because scalar layout later resolves the selected target width.

---

# Governance note

> **Applied 2026-08-23:** Compiler target selection now constructs Sema from
> `ResolvedScalarPlan`; `int` and `uint` receive exact 32-bit or 64-bit bounds.
> Remaining legacy AST backend width integration stays explicit in governance.

Do **not** downgrade:

- `lowering.sec-mlir-scalar-layout`;
- `lowering.sec-mlir-checked-integers`;
- `lowering.sec-mlir-checked-integer-arith`.

Those newer P6–P8 paths already model target-sized integer layout correctly.

This correction concerns:

- target-unaware frontend semantic bounds;
- target-insensitive legacy direct LLVM width mapping;
- target-insensitive legacy AST-to-LLVM-dialect MLIR width mapping.
