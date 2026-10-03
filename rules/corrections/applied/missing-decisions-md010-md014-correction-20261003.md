# Missing Decisions MD-010 through MD-014 — Correction

**Status:** Applied correction with explicit deferrals  
**Applied:** 2026-10-03  
**Created:** 2026-10-03  
**Updated:** 2026-10-03  
**Revision:** 1  
**Sec version:** 0.1  
**Canonical path:** `rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md`  
**Replaces:** None

## 1. Purpose and scope

1.1. This correction records the Sec 0.1 decisions and specification synchronizations reached while reviewing `MD-010` through `MD-014` from `missing-decisions.yaml`.

1.2. This correction deliberately distinguishes:

- accepted language decisions;
- already-existing decisions that were incompletely synchronized into rulebooks;
- accepted partial conclusions that do not yet close their missing decision;
- questions intentionally transferred to a dedicated future rulebook.

1.3. `MD-010` is not closed by this correction. The regular-expression language is large enough to require its own dedicated rulebook.

1.4. `MD-012` is not closed by this correction. The semantic layering of checked conversion failures is accepted, but the canonical public runtime error type or types, exact variant names, and payload remain undecided.

1.5. `MD-011`, `MD-013`, and `MD-014` may be treated as resolved once all owning rulebooks and governance fragments have been synchronized with this correction.

1.6. An entry must not be removed from `missing-decisions.yaml` merely because this correction exists. Remove an entry only when the entry's normative work is complete.

1.7. In particular:

```text
MD-010
    remains open until the dedicated regex rulebook is accepted and integrated.

MD-011
    may close after rulebook/governance synchronization.

MD-012
    remains open until the canonical runtime error type structure and payload are fixed.

MD-013
    may close after formatter rulebook/governance/test synchronization.

MD-014
    may close after type/default/layout/target rulebook synchronization.
```

1.8. This correction partially supersedes the earlier correction:

```text
rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md
```

only where § 3 below replaces the earlier `ConstantExpression` grammar shape for contract arguments with ordinary `Expression` syntax in a `SemanticCompileTimeRequiredContext`.

---

## 2. MD-010 — Regular expressions move to a dedicated rulebook

2.1. `MD-010` is intentionally not resolved inside `rules/types/contracts.md`.

2.2. Sec regular expressions are expected to be useful outside type contracts.

2.3. Sec therefore requires one coherent regular-expression rulebook rather than a contract-private regex dialect.

2.4. The future regex rulebook must own the regular-expression language itself, including at least:

- pattern syntax;
- escape interpretation;
- interaction with Sec string decoding;
- Unicode matching semantics;
- anchoring;
- whole-value validation versus search;
- matching unit;
- character classes and Unicode properties;
- flags;
- captures where supported;
- unsupported constructs;
- complexity guarantees;
- compiled-pattern semantics;
- invalid-pattern diagnostics;
- runtime regex APIs;
- search/find/split/replacement behavior where provided;
- contract integration.

2.5. The `regex` type contract must reuse that same regex language.

2.6. The type-contract rulebook must not independently define a second regex grammar.

2.7. The following design directions were recorded during the MD-010 review and must be preserved for the dedicated regex discussion, but they are not made normative by this correction:

```text
Sec owns the regex dialect rather than normatively depending on one
specific implementation library.

A regex supplied as a Sec string is interpreted from the already-decoded
Sec string value.

Regex contracts are intended as whole-value validation rather than
substring search.

Unicode scalar matching is preferred over UTF-8-byte matching for the
text regex language.

Implicit Unicode normalization is not intended.

The dialect should preserve a predictable non-exponential complexity
guarantee and avoid unrestricted backtracking constructs such as
backreferences.

Invalid compile-time-known patterns should be compile-time errors.

Compile-time-known values/defaults of a regex-constrained type should be
validated during compilation.

A regex contract should apply through a named-type chain whose underlying
text type is `string`, rather than silently coercing bytes, runes, chars,
or their sequence types into strings.
```

2.8. These points are design notes, not a substitute for the dedicated rulebook.

2.9. The current design-note artifact is:

```text
rules/design/regex-design-notes-20261003.md
```

2.10. That design-note artifact is non-normative and must be replaced or superseded by the accepted dedicated regex rulebook.

2.11. `MD-009` remains authoritative for the Sec 0.1 reservation and contextual contract spelling `regex`.

2.12. The compile-time classification of the regex pattern expression is resolved separately by MD-011 in § 3 of this correction.

---

## 3. MD-011 — Contract and default expressions use semantic CTE

3.1. Sec 0.1 does not introduce a separate restricted source language for contract constants.

3.2. Contract/default expressions use ordinary Sec `Expression` syntax.

3.3. The owning semantic rule classifies the required expression as a:

```text
SemanticCompileTimeRequiredContext
```

3.4. This applies to every contract/default position that requires a value before runtime lowering, including:

```text
range lower bound
range upper bound
each in [...] member
multipleOf divisor
minLen value
maxLen value
exactLen value
regex pattern
explicit default expression
```

3.5. Argumentless contracts do not introduce a separate contract-argument expression.

3.6. Argumentless contracts nevertheless participate fully in compile-time validation whenever the constrained value is known during compilation.

3.7. This includes at least:

```text
odd
even
finite
notEmpty
unique
```

3.8. Therefore a compile-time-known value or explicit default that violates `odd`, `even`, or another applicable argumentless contract is a compile-time contract error.

3.9. Example:

```sec
type EvenValue int even default 4
```

is valid when all other contracts also accept `4`.

3.10. Example:

```sec
type EvenValue int even default 3
```

is invalid at compile time.

### 3.1 Range bounds are not literal-only

3.11. A range bound is no longer restricted to `SignedNumericConstant` grammar.

3.12. A range bound is an ordinary `Expression` evaluated in a `SemanticCompileTimeRequiredContext`.

3.13. Therefore these forms are valid when their evaluation succeeds under semantic CTE:

```sec
let MinimumPort: int := 1
let MaximumPort: int := 0xFFFF

type Port int range MinimumPort..MaximumPort
```

3.14. Ordinary runtime-dependent bounds remain invalid because the required semantic CTE cannot complete.

### 3.2 Immutable compile-time-established values

3.15. An immutable value already established by semantic CTE may be referenced from a contract/default expression when ordinary visibility and dependency rules permit it.

3.16. Example:

```sec
let MaximumPort: int := 0xFFFF

type Port int range 1..MaximumPort
```

3.17. Ordinary mutable runtime static state is not CTE-readable merely because its initial value was once known at compile time.

### 3.3 Ordinary functions and methods

3.18. Sec does not require a source-level `const fn` or `comptime fn` category for contract/default evaluation.

3.19. Ordinary Sec functions may execute during semantic CTE when the concrete invocation is CTE-legal.

3.20. Static functions may likewise execute during semantic CTE when their arguments and executed path are CTE-legal.

3.21. Ordinary instance methods may participate where ordinary semantic CTE already permits the call and the receiver is available to CTE.

3.22. Example:

```sec
fn MaxPort() int {
    return 0xFFFF
}

fn DefaultPort() int {
    return 8080
}

type Port int
    range 1..MaxPort()
    default DefaultPort()
```

### 3.4 Property getters

3.23. Property syntax does not make an otherwise CTE-legal computation runtime-only.

3.24. Reading a property during semantic CTE evaluates the property's getter.

3.25. A static property may therefore be used in a contract/default expression when its getter execution is CTE-legal.

3.26. Example:

```sec
impl Standards {
    static property MaxPort: int {
        get {
            return 0xFFFF
        }
    }
}

type Port int range 1..Standards.MaxPort
```

3.27. An instance property may also be used when:

- its receiver is available to semantic CTE;
- the getter is legal for that receiver;
- the concrete getter execution performs only CTE-permitted operations.

3.28. Property access is not rejected merely because a property represents behavior rather than stored data.

3.29. Runtime-only receiver state remains unavailable to required CTE.

### 3.5 Path-sensitive legality

3.30. CTE legality is determined by the concrete operations actually executed.

3.31. A function or getter may contain a runtime-only operation on an unexecuted branch without making every compile-time invocation invalid.

3.32. Therefore Sec does not require declaration-wide purity merely to permit a call during semantic CTE.

3.33. Local mutation, control flow, ordinary calls, temporary values, and other operations already permitted by the semantic CTE rulebook remain permitted.

3.34. Forbidden ambient/runtime effects remain forbidden when actually executed.

### 3.6 Explicit defaults and transient evaluator allocation

3.35. Evaluation of an explicit default expression may use transient evaluator-local allocation when semantic CTE permits it.

3.36. This does not mean that the canonical runtime/static default may depend on hidden runtime allocation.

3.37. CTE evaluability and static materializability remain separate properties.

3.38. The resulting default must satisfy the canonical materialization/default-value rules of its type.

### 3.7 Grammar synchronization

3.39. Contract/default grammar should describe ordinary `Expression` syntax rather than a separate `ConstantExpression` sublanguage where the only semantic distinction is that the owning rule requires semantic CTE.

3.40. Conceptually:

```text
RangeContract
    ::= "range" [ Expression ] RangeOperator [ Expression ]

MembershipContract
    ::= "in" "[" Expression { "," Expression } [ "," ] "]"

MultipleOfContract
    ::= Contextual("multipleOf") Expression

MinLenContract
    ::= Contextual("minLen") Expression

MaxLenContract
    ::= Contextual("maxLen") Expression

ExactLenContract
    ::= Contextual("exactLen") Expression

RegexContract
    ::= Contextual("regex") Expression

DefaultClause
    ::= "default" Expression
```

3.41. The owning semantic rule supplies the required result type/domain for each expression.

3.42. In particular, the regex expression must produce the compile-time string value required by the regex contract.

3.43. Paragraphs 3.39–3.42 supersede the earlier correction's `RegexContract ::= Contextual("regex") ConstantExpression` shape.

3.44. This grammar change does not alter the MD-009 decision that `regex` is the compiler-known reserved contract spelling.

---

## 4. MD-012 — Checked conversion failure layers

4.1. The MD-012 review establishes three distinct semantic layers.

4.2. These layers must not be collapsed into one undifferentiated "conversion error" concept.

### 4.1 No defined conversion relation

4.3. First, the compiler determines whether the source and target types have a defined explicit conversion relation at all.

4.4. If no such conversion exists, the source program is invalid.

4.5. This is a compile-time type/conversion diagnostic.

4.6. It is not a runtime conversion failure.

4.7. Example:

```sec
let value: int := int("hello")
```

is invalid if Sec defines no `string -> int` conversion.

4.8. No runtime `Result` error type is created merely to represent an illegal relationship between orthogonal source and target types.

### 4.2 Intrinsic target-domain failure

4.9. A defined conversion may still fail because the runtime source value cannot form a valid value of the target primitive/scalar type.

4.10. This failure belongs to the target type's intrinsic value/representation domain.

4.11. It is not a user-declared contract failure.

4.12. Example:

```sec
let source: int := ReadValue()
let value := try int8(source)
```

may fail when the runtime integer cannot be represented as `int8`.

4.13. The fact that `uint8` has an intrinsic value range such as `0..255` is a property of the primitive type, not an implicit `range` contract.

4.14. Likewise, a conversion to `rune` may fail because a numerically representable integer is not a valid Unicode scalar value.

4.15. Intrinsic failure classes include concepts such as:

```text
overflow / non-representability
precision loss where the conversion contract forbids it
invalid Unicode scalar value
other target-scalar-specific validity failures defined by the owning type
```

4.16. § 4.15 describes semantic categories only. This correction does not fix their final public Sec error type names or variant spellings.

### 4.3 Declared contract failure

4.17. Contract validation happens only after the value is valid for the named type's underlying target domain.

4.18. Example:

```sec
type I8 int8 range 1..100
```

with the value `110` does not overflow `int8`.

4.19. It fails the declared `range 1..100` contract.

4.20. Similarly, an otherwise valid integer may fail `odd`, `even`, `multipleOf`, membership, or another declared contract.

4.21. Contract failure therefore belongs to named-type validation, not to primitive representability.

### 4.4 Evaluation order

4.22. For a conversion into a constrained named type, the semantic order is:

```text
1. Verify that the source-to-target conversion relation exists.
2. Evaluate the source expression according to ordinary Sec rules.
3. Perform the checked conversion into the underlying target value domain.
4. If intrinsic target-domain conversion fails, stop.
5. Evaluate declared contracts in their canonical source order.
6. If a contract fails, stop with a contract-validation failure.
7. Only after all checks succeed does the constrained named target value exist.
```

4.23. Example:

```sec
type SmallOdd int8
    range 1..100
    odd
```

For a runtime source integer:

```text
1000
    intrinsic int8 representability failure

110
    range contract failure

50
    odd contract failure

51
    success
```

4.24. A compiler-known invalid value is diagnosed at compile time rather than deliberately lowered into a runtime failure.

### 4.5 MD-012 remains open

4.25. The following questions remain unresolved and continue to belong to MD-012:

- whether intrinsic checked-conversion failures and contract-validation failures use separate public error types or a common typed wrapper;
- the canonical public type name or names;
- exact variant names;
- exact payload structures;
- whether contract failure exposes contract kind, source identity/index, or another stable identifier;
- which exact primitive/scalar failure variants belong to the public API.

4.26. Implementation must preserve the semantic distinction in §§ 4.3–4.24 even before the final runtime error type structure is accepted.

4.27. `MD-012` must therefore remain in `missing-decisions.yaml`.

---

## 5. MD-013 — Formatter alignment synchronization

5.1. Most of the questions described by `MD-013` were already decided when the formatter rulebook was designed.

5.2. They must not be reopened merely because an implementation or an older paragraph failed to preserve them.

### 5.1 Colon placement is already decided

5.3. In declaration alignment, the colon remains directly attached to the identifier.

5.4. Canonical alignment never introduces whitespace between an identifier and its colon.

5.5. Alignment padding follows the complete `identifier:` cell.

5.6. This is existing formatter semantics, not a new MD-013 design decision.

### 5.2 Alignment groups and trailing comments are already decided

5.7. A trailing comment belongs to its source item.

5.8. A trailing comment does not terminate an alignment group.

5.9. Blank lines and standalone comments do terminate the local structural alignment group.

5.10. Documentation comments and other already-defined structural group boundaries retain their existing formatter semantics.

5.11. Any formatter implementation or test that treats an ordinary trailing comment as terminating the field alignment group is inconsistent with the accepted formatter model and must be corrected.

### 5.3 Trailing comments occupy an aligned column

5.12. Trailing comments participate in the local declaration-table column layout.

5.13. The formatter first determines the local compatible alignment group.

5.14. Within that group, declaration cells align structurally.

5.15. The next structural column begins after the standard spacing in effect from the widest preceding cell.

5.16. Conceptually:

```text
widest identifier: [standard spacing] type [standard spacing] trailing comment
```

5.17. Shorter cells receive the additional padding necessary to reach the same following column.

5.18. The spacing before a trailing comment must therefore not be hard-coded independently of the formatter's canonical standard-spacing rule.

5.19. When the standard spacing in effect changes, the aligned comment column changes consistently with it.

### 5.4 Register fields

5.20. Register fields participate in the same generic local structural alignment engine as compatible struct/declaration fields.

5.21. This includes named register fields and reserved `_` fields.

5.22. Example shape:

```sec
type Status register[16] {
    Ready:     bit
    ErrorCode: bit[4]
    _:         bit[11]
}
```

5.23. Register-field alignment uses the same existing principles:

- `:` remains attached to the field identifier;
- the type/bit-field column aligns locally;
- trailing comments may occupy the aligned trailing-comment column;
- blank lines and standalone comments terminate the group;
- trailing comments do not terminate the group.

5.24. The fact that the current formatter or existing tests may leave register fields unaligned does not define language semantics.

5.25. Such behavior is an implementation gap relative to the accepted formatter model.

### 5.5 Nature of MD-013

5.26. MD-013 is therefore primarily a formatter specification/implementation synchronization item.

5.27. The only missing application-specific clarification was that register fields are clients of the same local structural alignment model.

5.28. After the formatter rulebook, implementation tests, and governance reflect §§ 5.3–5.27, MD-013 may be removed.

---

## 6. MD-014 — Plain `float` follows platform width

6.1. `MD-014` does not introduce a new Sec design choice.

6.2. Sec already uses platform-native width for the unsized scalar families.

6.3. Plain `float` follows the platform width on exactly the same principle as plain `int` and `uint`.

6.4. For a 32-bit platform:

```text
int
    native 32-bit integer width

uint
    native 32-bit unsigned integer width

float
    float32 width/semantics
```

6.5. For a 64-bit platform:

```text
int
    native 64-bit integer width

uint
    native 64-bit unsigned integer width

float
    float64 width/semantics
```

6.6. Plain `float` does not have a separately selectable default width independent of platform bitness.

6.7. The compiler must not introduce a second float-width policy such as an independently configured `DefaultFloatWidth`.

6.8. The canonical platform/target width fact already used to resolve native scalar width must determine plain `float` width as part of the same platform model.

6.9. Host compiler width, backend preference, LLVM defaults, or foreign ABI convenience must not override the selected Sec platform.

6.10. `float32` remains the explicit 32-bit binary floating-point type.

6.11. `float64` remains the explicit 64-bit binary floating-point type.

6.12. Code requiring an explicit fixed floating-point width uses `float32` or `float64`.

6.13. This correction does not introduce or alter any separate rule concerning source-level conversion or type identity between `float` and the explicit-width floating-point spellings. MD-014 resolves their platform width behavior.

### 6.1 Defaults and representability

6.14. Every rule that depends on the representable values of plain `float` must use the width selected by the target platform.

6.15. Therefore a range-constrained plain-`float` default is resolved using:

```text
float32 representability on a 32-bit platform
float64 representability on a 64-bit platform
```

6.16. The nearest-valid exactly-representable default rule is therefore deterministic once the target platform is resolved.

6.17. The current fallback that refuses to derive a nonzero implicit default for constrained plain `float` merely because its width was unspecified must be removed after the target-width rule is synchronized.

### 6.2 Nature of MD-014

6.18. MD-014 is a specification synchronization defect, not a new language-design fork.

6.19. `rules/types/types.md` must state explicitly that plain `float` is platform-sized in the same way as `int` and `uint`.

6.20. `rules/types/default_values.md` must use the resolved platform width when evaluating plain-`float` representability.

6.21. `rules/memory/layout.md` and the canonical target/platform rule must expose enough resolved platform information for layout and value-domain analysis to agree.

6.22. No independent plain-float width selector is introduced.

---

## 7. Required rulebook synchronization

7.1. MD-010 requires:

- creation and acceptance of the dedicated regex rulebook;
- later synchronization of `rules/types/contracts.md` to reference that rulebook;
- regex diagnostics/tooling synchronization;
- affected governance synchronization.

7.2. MD-011 requires synchronization of at least:

- `rules/types/contracts.md`;
- `rules/types/default_values.md`;
- `rules/compiler/compile_time_evaluation.md` where cross-reference clarification is needed;
- `rules/foundations/grammar.md`;
- affected parser/sema/CTE governance fragments.

7.3. MD-011 synchronization must explicitly update the earlier MD-009 grammar consequence from `ConstantExpression` to ordinary `Expression` plus semantic CTE classification.

7.4. MD-012 currently requires no invented runtime error declaration.

7.5. Before MD-012 closes, synchronize at least:

- `rules/types/contracts.md`;
- `rules/types/types.md`;
- `rules/errors/errorhandling.md`;
- the canonical core declarations if public conversion/contract error types are introduced;
- affected frontend/runtime governance.

7.6. MD-013 requires synchronization of at least:

- `rules/tooling/formatter.md`;
- formatter tests;
- formatter implementation/governance.

7.7. MD-013 synchronization must not reopen already accepted colon placement, group-boundary, or trailing-comment-column semantics.

7.8. MD-014 requires synchronization of at least:

- `rules/types/types.md`;
- `rules/types/default_values.md`;
- `rules/memory/layout.md`;
- the canonical platform/target width rule;
- affected scalar/default/layout governance.

---

## 8. Missing-decision status after this correction

8.1. After this correction is accepted:

```text
MD-010
    OPEN
    transferred to dedicated regex rulebook work

MD-011
    RESOLVED
    pending rulebook/governance synchronization

MD-012
    OPEN
    semantic failure layering resolved
    public runtime error type/payload still unresolved

MD-013
    RESOLVED
    primarily specification/implementation synchronization

MD-014
    RESOLVED
    longstanding platform-width rule, pending synchronization
```

8.2. `missing-decisions.yaml` must preserve MD-010 and MD-012 until their remaining normative questions are closed.

8.3. MD-011, MD-013, and MD-014 may be removed only after their owning rulebooks and governance have been synchronized.

---

## 9. Implementation discipline

9.1. Compiler and formatter implementation must not treat stale implementation behavior as language authority when it contradicts an accepted rule recorded here.

9.2. Implementation of MD-011 must use the canonical semantic CTE evaluator rather than creating a separate contract-only constant evaluator.

9.3. Implementation of MD-012 must preserve the semantic distinction between:

```text
illegal conversion relation
intrinsic target-domain conversion failure
declared contract failure
```

even while the final public runtime error type remains undecided.

9.4. Implementation of MD-013 must use the shared structural alignment model for register fields rather than a register-specific formatter policy.

9.5. Implementation of MD-014 must derive plain `float` width from the selected Sec platform exactly as the platform-native scalar model requires.

9.6. Lower compiler stages must consume the semantic decisions already resolved by the frontend and CompilationPlan and must not independently reinterpret these source-language rules.
