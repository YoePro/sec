# Pitfall Analysis

## Status

Normative compiler-analysis rulebook for Sec 0.1.

This rulebook defines the semantic purpose, finding model, confidence model,
evidence and suppression rules, initial pitfall catalog, diagnostic integration,
corrective-action requirements, analysis-budget behavior, tooling integration,
incremental behavior, tests, and completion criteria for Sec pitfall analysis.

Mutable implementation status does not belong in this rulebook. It is governed
by the repository-level `implementation-status.yaml` ledger.

---

# Purpose

Pitfall analysis identifies semantically suspicious programming patterns that
commonly arise from human mistakes such as:

- off-by-one errors;
- incorrect range endpoints;
- incorrect collection bounds;
- confusion between length, capacity, extent, element count, and byte count;
- copy/paste mistakes where one value is checked but another is used;
- ineffective or tautological safety conditions;
- misuse of foreign pointer/extent relationships;
- other high-confidence patterns where canonical semantic facts strongly suggest
  that the written code does not match the programmer's likely intent.

Pitfall analysis is not a replacement for type checking, bounds analysis,
ownership analysis, lifetime analysis, escape analysis, deadlock analysis, FFI
validation, or any other analysis that owns a normative Sec rule.

Its role is to correlate facts from those analyses, recognize suspicious
relationships, improve diagnostics, and provide safe or explicitly heuristic
corrective guidance.

The analysis should help the programmer most where ordinary type correctness is
insufficient to reveal a likely mistake.

---

# Normative role

This rulebook is normative for:

- the distinction between proven invalid code and suspicious but valid code;
- the `PitfallFinding` analysis product;
- pitfall classification and confidence;
- evidence-for, evidence-against, and suppression semantics;
- semantic rather than purely syntactic pitfall recognition;
- interaction with the analysis that owns an underlying normative error;
- duplicate-diagnostic coalescing;
- the required Sec 0.1 pitfall catalog;
- FFI contract-backed pitfall recognition;
- safe corrective actions and heuristic suggested edits;
- preference for canonical Sec idioms in suggestions;
- Interactive, Standard, and Deep pitfall-analysis behavior;
- rule identities and registry requirements;
- incremental invalidation and LSP integration;
- required test classes;
- completion criteria for Sec 0.1 pitfall analysis.

This rulebook does not redefine:

- indexing legality;
- range semantics;
- collection length or capacity semantics;
- integer overflow or underflow semantics;
- ownership or move legality;
- borrow or lifetime validity;
- escape validity;
- task or thread safety;
- deadlock semantics;
- FFI declaration syntax;
- FFI import or binding-generation syntax;
- diagnostic suppression syntax;
- project configuration syntax;
- a new strict compilation mode.

Those concerns remain owned by their respective normative rulebooks and tooling
specifications.

---

# Core principle

Pitfall analysis recognizes suspicious semantic relationships.

It does not become the owner of every rule that it observes.

Conceptually:

```text
PitfallAnalysis
    detects suspicious relationships

OwningAnalysis
    defines whether the underlying behavior is valid Sec
```

Examples:

```text
out-of-bounds access
    owner = bounds/range semantics

invalid retained borrow
    owner = escape/lifetime analysis

deadlock
    owner = deadlock analysis

invalid FFI lifetime
    owner = FFI + lifetime semantics
```

Pitfall analysis may enrich such a diagnostic with intent-aware explanation and
a corrective action, but it must not replace the owning semantic rule.

---

# Proven invalidity is not a warning

When a pitfall implies a proven violation of a normative Sec rule, the resulting
program remains invalid according to that owning rule.

Pitfall analysis must not downgrade the result to a warning or advisory merely
because the mistake resembles a common human error.

For example:

```sec
for i in 0..values.len {
    Process(values[i])
}
```

The loop domain includes `values.len` while valid zero-based indices end before
`values.len`.

If the final indexed access is reachable, this is a proven bounds violation.

The primary diagnostic is therefore the ordinary normative bounds error.
Pitfall analysis may improve it with information such as:

```text
the inclusive range reaches `values.len`
`values.len` is the element count, not a valid index
consider `0..<values.len`
```

---

# Classification

Every reported pitfall finding has one of the following conceptual
classifications:

```text
ProvenInvalid
LikelyMistake
SuspiciousIntent
```

The exact compiler enum name is implementation-defined.

## ProvenInvalid

`ProvenInvalid` means canonical semantic analysis proves that a reachable
execution violates a normative Sec rule.

The owning semantic rule determines the actual compiler error.

Examples include:

```text
indexing a collection at exactly its length
an inclusive loop endpoint causing a proven final out-of-bounds access
accessing `i + 1` when the loop permits i == len - 1
accessing `i - 1` when the loop permits i == 0
```

## LikelyMistake

`LikelyMistake` means the program may remain valid Sec, but semantic evidence
strongly indicates a common accidental pattern.

Example:

```sec
for i in 1..<values.len {
    Process(values[i])
}
```

may be a likely mistake when:

```text
`values` supplies the loop bound
`values` is indexed by i
index 0 is not otherwise handled
no predecessor/neighbor access explains the starting index
no explicit contract establishes an intentional suffix traversal
```

## SuspiciousIntent

`SuspiciousIntent` means the pattern is noteworthy but the compiler lacks enough
evidence to claim high confidence that it is unintended.

These findings are optional analysis insight and normally belong to Deep
analysis rather than ordinary edit-time diagnostics.

---

# Confidence

Classification and confidence are independent concepts.

The analysis must support at least:

```text
Proven
High
Medium
```

Low-confidence speculation should normally produce no diagnostic.

Typical combinations are:

```text
ProvenInvalid    + Proven
LikelyMistake    + High
SuspiciousIntent + Medium
```

The compiler may internally represent greater detail, but it must preserve the
semantic distinction between proof and intent inference.

---

# No new strict mode

This rulebook does not define `strict`, `use strict`, or any other new strict
compilation mode.

Meaningless but otherwise valid code may be surfaced by optional analysis,
including Deep analysis, when doing so is useful and sufficiently precise.

If a separate Sec rulebook later defines a general strict-policy mechanism,
pitfall analysis may consume that policy. It must not invent such a feature on
its own.

---

# PitfallFinding

Pitfall rules produce structured findings rather than directly formatting final
user-facing diagnostics.

Conceptually, a finding must be able to carry information equivalent to:

```text
PitfallFinding {
    RuleIdentity
    Kind
    Classification
    Confidence
    Subject
    EvidenceFor
    EvidenceAgainst
    Suppression
    OwningRule
    SuggestedActions
    AnalysisState
}
```

The exact data structure is implementation-defined.

The structured finding is the canonical pitfall-analysis product.

Human-readable diagnostics, LSP diagnostics, code actions, and `sec analyse`
output are presentations of that structured result.

---

# Evidence model

Pitfall analysis must represent evidence structurally before formatting it as
prose.

Evidence categories must be able to express at least the conceptual strengths:

```text
Proof
StrongEvidence
SupportingEvidence
ContradictingEvidence
SuppressingEvidence
```

A numeric scoring model is not required.

The compiler may use one internally, but semantic suppression and classification
must remain explainable through concrete facts.

---

# Evidence for a finding

Consider:

```sec
for i in 1..<values.len {
    Process(values[i])
}
```

Evidence supporting a skipped-first-element finding may include:

```text
iteration starts at 1
same collection provides upper bound
same collection is indexed by the induction variable
index 0 is not processed on a dominating path
no predecessor access exists
no explicit suffix-traversal contract exists
```

The analysis should use the strongest available semantic facts rather than mere
surface resemblance.

---

# Evidence against a finding

Consider:

```sec
for i in 1..<values.len {
    Compare(values[i - 1], values[i])
}
```

The predecessor access is strong evidence that starting at index 1 is
intentional because it makes both indexed expressions valid.

That evidence suppresses the skipped-first-element advisory.

This is a normative false-positive-avoidance requirement.

---

# Suppressing evidence

Heuristic findings must be suppressed when semantic evidence establishes a
common intentional pattern.

Examples of suppressing evidence include:

```text
predecessor traversal
explicit handling of the first element before the loop
proven equality or sufficient-size relationship between two collections
explicit non-empty guard
explicit fixed-extent or protocol contract
explicit FFI contract
explicit hardware/storage contract
canonical intent pattern recognized by the compiler
```

A suppressed finding may remain visible in optional Deep analysis debugging
output when the user asks for detailed analysis reasoning.

It must not be shown as an ordinary warning/advisory after it has been
suppressed.

---

# Proven errors cannot be heuristically suppressed

Suppression applies to heuristic intent findings.

A proven normative error cannot disappear merely because the pattern appears
intentional.

For example, if an access is proven out of bounds, a recognized coding idiom
cannot make the access valid.

The owning semantic analysis remains authoritative.

---

# Semantic recognition, not syntax matching

Pitfall analysis must prefer semantic relationships over syntax patterns when
canonical facts can distinguish intended code from suspicious code.

A rule such as:

```text
if loop starts at 1:
    warn
```

is not acceptable.

The compiler must instead consider facts such as:

```text
iteration domain
indexed collection
index relation
neighbor offsets
guards
prior handling
collection contracts
postconditions
proven reachability
```

This requirement exists specifically to keep false positives low.

---

# Normalized relations

Pitfall recognition should consume normalized semantic relations where
available.

Useful conceptual facts include:

```text
DerivedFrom(value, origin)
SameRoot(a, b)
DifferentRoot(a, b)
LengthOf(value, collection)
CapacityOf(value, collection)
PointerInto(pointer, storage)
ExtentOf(value, storage)
IndexOffset(base, offset)
RangeDomain(value, lower, upper, inclusivity)
```

The exact internal representation is implementation-defined.

Existing Place/provenance, range, control-flow, type, and FFI analyses own the
underlying facts.

Pitfall analysis must not build a second independent provenance system merely to
support its rules.

---

# Source mapping

Semantic normalization must not discard the source locations and source forms
required for clear diagnostics and code actions.

A finding should be able to explain the programmer's written expression even if
its reasoning used normalized Semantic IR or equivalent canonical facts.

---

# Reachability

Only reachable semantic behavior contributes to a pitfall finding.

Proven unreachable code does not create a new pitfall finding.

Where Sec already treats proven dead or unreachable code as a compiler error,
the owning reachability diagnostic remains primary.

Pitfall analysis may improve the explanation of why a branch or condition is
impossible.

---

# Guards participate in pitfall reasoning

Control-flow guards must refine the facts used by pitfall analysis.

For example:

```sec
if values.len == 0 {
    return
}

let last := values[values.len - 1]
```

establishes the non-empty condition required by the final-element access.

No empty-collection pitfall should be reported on the reachable path after the
guard.

Similarly:

```sec
for i in 0..values.len {
    if i == values.len {
        break
    }

    Process(values[i])
}
```

must not receive the simple inclusive-range out-of-bounds finding if control-flow
analysis proves that the indexed access cannot execute for `i == values.len`.

A separate simplification advisory may still be possible, but it is not the
same finding.

---

# Interprocedural evidence

Pitfall analysis may use interprocedural evidence when available within the
active analysis budget.

For example:

```sec
ValidateNonEmpty(values)
let last := values[values.len - 1]
```

may be proven safe when the imported or inferred summary for
`ValidateNonEmpty` establishes an applicable non-empty postcondition.

Pitfall analysis does not own that postcondition analysis. It consumes the
canonical summary.

Interactive analysis may omit an expensive interprocedural refinement and mark
the relevant rule `NotEvaluated` or `Pending` rather than inventing a conclusion.

---

# Analysis states

Pitfall rule evaluation must distinguish at least the conceptual states:

```text
Finding
NoFinding
Suppressed
NotEvaluated
Pending
```

`NoFinding` means the rule was evaluated with sufficient required facts and did
not produce a finding.

`NotEvaluated` means the rule was not run or its required facts were not
available under the active budget.

`Pending` is a tooling state indicating that required analysis work is not yet
complete.

`NotEvaluated` and `Pending` must never be cached or interpreted as proof that no
pitfall exists.

---

# Canonical pitfall families

The pitfall architecture must support multiple semantic families.

Conceptual families include:

```text
BoundsAndRanges
ControlFlow
Collections
OwnershipAndLifetime
OptionResultErrorFlow
Concurrency
Numeric
APIUsage
FFI
Resources
```

Sec 0.1 does not require every possible family to have a large catalog.

The required initial implementation prioritizes high-confidence bounds, range,
collection, control-flow, and contract-backed FFI mistakes.

---

# Required Sec 0.1 catalog

The following catalog defines the minimum semantic pitfall coverage required for
Sec 0.1.

A particular finding may be implemented by the owning bounds/range analysis and
enriched by pitfall analysis, or directly recognized by pitfall analysis from
canonical facts. The architectural ownership rule remains unchanged.

---

# Bounds and range pitfalls

## Inclusive upper bound against collection length

Example:

```sec
for i in 0..values.len {
    Process(values[i])
}
```

The loop establishes:

```text
0 <= i <= values.len
```

while indexing requires:

```text
0 <= i < values.len
```

If the indexed access is reachable at the inclusive endpoint, the result is:

```text
Classification = ProvenInvalid
Confidence     = Proven
OwningRule     = Bounds
```

The diagnostic should normally recommend the canonical half-open traversal:

```sec
for i in 0..<values.len {
    Process(values[i])
}
```

---

## Direct index at length

Example:

```sec
let value := values[values.len]
```

This is a proven bounds violation.

The diagnostic should explain that `len` is the element count and therefore one
past the last zero-based index.

---

## Upper neighbor access

Example:

```sec
for i in 0..<values.len {
    Compare(values[i], values[i + 1])
}
```

The analysis must compare the complete loop domain against every indexed
expression.

When the loop permits `i == values.len - 1`, the `i + 1` access is invalid.

The compiler must not limit this rule to literal syntax; semantically equivalent
normalized offset expressions must be handled when available.

---

## Lower neighbor access

Example:

```sec
for i in 0..<values.len {
    Compare(values[i - 1], values[i])
}
```

If the loop permits `i == 0`, the predecessor access is invalid.

Where the body itself demonstrates predecessor traversal intent, a safe
correction may be to begin at index 1, subject to the ordinary fix-safety rules.

---

## Ineffective upper bounds guard

Example:

```sec
if index <= values.len {
    Process(values[index])
}
```

If equality remains reachable, the guard does not establish a valid indexing
precondition.

The compiler must diagnose the proven boundary error through the owning bounds
rule.

---

## Ineffective rejection guard

Example:

```sec
if index > values.len {
    return
}

Process(values[index])
```

The guard permits `index == values.len`.

If that equality is reachable, the subsequent indexed access is invalid.

---

## Final-element access requires non-empty proof

Example:

```sec
let last := values[values.len - 1]
```

This expression requires a proof that the collection is non-empty, subject to
Sec's canonical arithmetic and bounds semantics.

A preceding guard or type/contract proof may satisfy that requirement.

Without such a proof, the owning arithmetic/bounds rules determine validity,
and pitfall analysis may provide the intent-aware explanation.

---

# Skipped-first-element advisory

Example:

```sec
for i in 1..<values.len {
    Process(values[i])
}
```

This is not inherently invalid.

A high-confidence `LikelyMistake` finding may be produced when evidence includes:

```text
the loop starts at index 1
`values` supplies the loop extent
`values` is indexed directly by i
index 0 is not handled on every dominating path
no predecessor/neighbor relation explains the starting index
no explicit contract establishes intentional suffix traversal
```

The finding must be suppressed for common intentional patterns such as:

```sec
for i in 1..<values.len {
    Compare(values[i - 1], values[i])
}
```

or:

```sec
Process(values[0])

for i in 1..<values.len {
    Process(values[i])
}
```

This family is a false-positive-sensitive advisory, not a validity rule.

---

# Omitted-last-element advisory

A loop that appears intended to traverse a complete collection but provably
stops before its final element may produce a high-confidence advisory when no
semantic evidence explains the omission.

The rule must not treat every shortened range as suspicious.

Intent evidence such as neighbor access, chunking, sentinel handling, explicit
range contracts, or a separately processed final element suppresses the
finding.

---

# Count versus inclusive endpoint

An expression such as:

```sec
let part := values[start..start + count]
```

contains `count + 1` positions when the range is inclusive.

Pitfall analysis may report a `LikelyMistake` only when surrounding semantic
facts establish that `count` represents an element count rather than an
inclusive endpoint.

Where the intended semantics are proven to be start-plus-count, the canonical
half-open form is:

```sec
let part := values[start..<start + count]
```

No warning may be based solely on the variable name `count` if no semantic
contract or trusted metadata establishes its meaning.

---

# Avoiding fragile `0..len - 1` workarounds

Code such as:

```sec
for i in 0..values.len - 1 {
    Process(values[i])
}
```

may be valid under a non-empty proof but unnecessarily encodes the half-open
traversal through arithmetic on `len`.

When the compiler proves semantic equivalence and there is no relevant
observable difference, optional analysis may recommend the canonical Sec form:

```sec
for i in 0..<values.len {
    Process(values[i])
}
```

This is normally optional analysis information rather than a language error.

---

# Collection relationship pitfalls

## Bound source differs from indexed collection

Example:

```sec
for i in 0..<left.len {
    Process(right[i])
}
```

The analysis must correlate the source of the bound with the collection being
indexed.

If it is proven that `right` can be shorter than `left`, the owning bounds rule
may produce a proven invalidity.

If the relationship is unknown but:

```text
`left` supplies the bound
`right` is the only collection indexed by i
`left` is otherwise unused in the loop body
no sufficient relation between their lengths is proven
```

then a high-confidence copy/paste `LikelyMistake` finding may be appropriate.

A proven equality or sufficient lower bound on `right.len` suppresses the
pitfall finding.

---

## Parallel collection traversal

Example:

```sec
for i in 0..<left.len {
    Compare(left[i], right[i])
}
```

No pitfall is reported when analysis proves that `right.len >= left.len`,
including through a dominating equality check such as:

```sec
if left.len != right.len {
    return
}
```

Without a sufficient relationship, the normal bounds result and applicable
pitfall classification follow from the available proof strength.

---

# Length, capacity, and extent

Length, capacity, and extent are semantically distinct concepts.

Pitfall analysis must preserve that distinction.

## Capacity used as live length

Example:

```sec
for i in 0..<values.capacity {
    Process(values[i])
}
```

For a collection type where capacity may exceed live length, this is suspicious.

If analysis proves that non-live elements can be reached, the owning collection
or bounds semantics determine the resulting error.

If the use remains valid only under an unproven equality assumption, a
high-confidence `LikelyMistake` may be produced.

A proven `capacity == len` relationship on the relevant path suppresses the
heuristic finding.

---

# Structural mutation during indexed iteration

Example:

```sec
for i in 0..<values.len {
    if ShouldRemove(values[i]) {
        values.RemoveAt(i)
    }
}
```

Pitfall analysis must correlate:

```text
iteration depends on collection shape
body structurally mutates the same collection
subsequent iteration/index assumptions may become stale
```

Structural mutation is not automatically an error.

The result depends on the collection operation, iteration semantics, control
flow, and whether subsequent indexed accesses remain valid.

The required rule must therefore distinguish:

```text
proven invalid traversal
high-confidence suspicious traversal
proven safe structural pattern
```

rather than warning on every mutation inside a loop.

---

# Control-flow pitfalls

## Guard checks the wrong value

Example:

```sec
if leftIndex < left.len {
    Process(right[rightIndex])
}
```

If the checked relation does not establish the precondition required by the
following operation, the compiler should correlate the guard and use.

Where the mismatch strongly resembles a copy/paste error, a `LikelyMistake`
finding may be emitted in addition to any owning bounds error.

---

## Safety check without control transfer

Example:

```sec
if index >= values.len {
    Log("bad index")
}

Process(values[index])
```

If the error branch neither terminates nor establishes a new valid index, the
check does not protect the following access.

The owning bounds analysis determines whether the subsequent access is proven
invalid.

Pitfall analysis may explain that the apparent safety check does not prevent the
unsafe path and may suggest the likely missing `return`, `continue`, or other
appropriate control transfer only when the intended action can be inferred with
sufficient confidence.

---

# Meaningless comparisons from proven ranges

When a type or path contract already proves a value's range, comparisons that
cannot affect control flow may be surfaced as optional analysis information.

Examples include conceptually:

```sec
if values.len < 0 {
    ...
}
```

or a comparison outside a constrained named type's permitted range.

If the unreachable branch is already invalid under Sec's dead-code rules, that
owning error remains primary.

If the code is merely meaningless but otherwise valid, this rulebook does not
invent a strict mode to reject it. Such findings may instead be shown by Deep
analysis or by an existing project diagnostic policy if one applies.

---

# Tautological interval conditions

Example:

```sec
if value >= 0 or value <= 10 {
    ...
}
```

For an ordinary totally ordered numeric value, the condition covers every value
and is therefore tautological.

The compiler should explain the proven boolean result.

When the surrounding form strongly indicates that the programmer intended an
inclusive interval membership test, the diagnostic should prefer Sec's
canonical range-membership syntax:

```sec
if value in 0..10 {
    ...
}
```

rather than merely suggesting replacement of `or` with `and`.

The canonical suggestion is appropriate only when the intended interval and
comparison semantics are compatible with `in`.

---

# Canonical idiom guidance

When Sec has a canonical language construct that directly expresses the likely
intended operation, pitfall diagnostics should prefer that construct.

Examples include range membership with `in` and half-open traversal with `..<`.

This preference is a diagnostic/tooling rule, not a change to the semantics of
other equivalent expressions.

---

# Option, Result, and state-correlation pitfalls

Pitfall analysis may correlate a state check with the value subsequently used.

For example, conceptually:

```sec
if first.IsSome {
    Use(second.Value)
}
```

checking `first` does not establish the state of `second`.

The owning Option/state rules determine whether the access is valid.

Pitfall analysis may add a high-confidence copy/paste explanation when the
checked object and used object differ and no other proof establishes the used
object's state.

Equivalent reasoning applies to Result-like state when a branch proves a state
for one value but the body uses a different value as though it had that state.

No particular Option/Result member spelling is defined by this rulebook.

---

# Ownership, lifetime, concurrency, and resource families

The pitfall architecture must allow findings in ownership, lifetime,
concurrency, and resource-management domains.

However, Sec 0.1 pitfall analysis must not duplicate the semantic work of the
analyses that already own those areas.

Examples:

```text
retained local borrow
    owner = escape/lifetime

holding a synchronization guard across a blocking operation
    owner = deadlock/concurrency analysis

resource lifetime guaranteed by ordinary Sec destruction
    must not be diagnosed as a C-style leak
```

Lower-confidence style heuristics in these domains are not required for the
initial Sec 0.1 pitfall catalog.

---

# FFI pitfall analysis

FFI-related pitfall detection must be contract-backed.

Pitfall analysis must not infer pointer/length or pointer/size relationships
merely from parameter names, common C naming conventions, or guesses about a
foreign function's purpose.

The analysis consumes canonical FFI facts supplied by the FFI/tooling system.

Those canonical contracts may originate from:

```text
explicit Sec FFI declarations/contracts
trusted imported foreign metadata
generated foreign-library bindings
compiler-known platform/library metadata
```

How foreign metadata is imported or bindings are generated is outside the scope
of this rulebook.

---

# Canonical foreign extent relationships

Pitfall analysis must be able to consume a canonical relationship equivalent to:

```text
BufferExtentRelation {
    PointerArgument
    ExtentArgument
    ExtentUnit
    AccessMode
}
```

The exact data structure and spelling are implementation-defined.

The relationship must be able to distinguish at least:

```text
ExtentUnit:
    Elements
    Bytes
```

and access direction when supplied by the FFI contract.

Additional FFI contract dimensions such as retention, nullability, alignment,
ownership transfer, or memory space remain owned by the FFI rulebooks but may
also be consumed where relevant.

---

# FFI pointer/extent provenance mismatch

Consider a foreign contract stating that argument 1 describes the accessible
extent of the storage addressed by argument 0.

Example call:

```sec
ForeignWrite(first.Ptr, second.len)
```

If canonical provenance establishes:

```text
argument 0 pointer derives from first
argument 1 extent derives from second
```

and no contract proves that `second.len` is intentionally the extent of
`first`, pitfall analysis may report a high-confidence pointer/extent mismatch.

This is a relational finding based on:

```text
foreign contract
+
Place/provenance
```

not on argument names.

---

# FFI element count versus byte count

Consider:

```sec
ForeignWrite(values.Ptr, values.len)
```

When the canonical foreign contract defines the extent argument in elements,
`values.len` may be exactly the required quantity.

When the canonical foreign contract defines the extent argument in bytes, the
analysis must compare the passed quantity against the element width and the
foreign extent semantics.

For a multi-byte element type, passing element count where byte count is
required may be a proven contract violation when the foreign contract and type
layout make the mismatch unambiguous.

For byte-sized elements, element count and byte count may coincide and must not
produce a false finding.

Unknown or absent extent-unit metadata produces no contract-backed byte-count
pitfall finding.

---

# FFI pointer/size origin mismatch

The same provenance rule applies to size-bearing objects or structures.

Conceptually:

```sec
ForeignSend(header.Ptr, payload.SizeOf)
```

may be suspicious if the canonical foreign contract requires the size argument
to describe the object addressed by the pointer and provenance proves that the
arguments describe unrelated objects.

The rule must remain silent when the contract does not establish that relation.

---

# Foreign lifetime mistakes remain owned elsewhere

If a foreign function may retain a pointer and a local or temporary object is
supplied, escape/lifetime/FFI analysis owns the validity result.

Pitfall analysis may improve the diagnostic with wording such as:

```text
the foreign operation may retain this pointer
the supplied storage is local to the current call context
```

but it must not implement a parallel foreign-retention analysis.

---

# Corrective actions

Pitfall corrective actions are divided conceptually into:

```text
ProvenFix
SuggestedEdit
```

## ProvenFix

A `ProvenFix` is an edit whose relevant semantic correctness is established by
the compiler for the finding being repaired.

Examples may include:

```sec
if i <= values.len
```

becoming:

```sec
if i < values.len
```

when the condition is proven to be the bounds guard for the following access.

Another example is replacing a semantically equivalent pair of inclusive bound
comparisons with canonical range membership when evaluation semantics are
preserved:

```sec
if value >= 0 and value <= 10 {
    ...
}
```

may be represented as:

```sec
if value in 0..10 {
    ...
}
```

when the compiler proves equivalence.

## SuggestedEdit

A `SuggestedEdit` expresses likely programmer intent but is not proven to be
semantically equivalent to the original program.

For example:

```sec
for i in 1..<values.len {
```

may be suggested as:

```sec
for i in 0..<values.len {
```

when a skipped-first-element finding has high confidence.

Such an edit must not be presented as a guaranteed safe automatic repair.

---

# Fix safety

An automatic proven fix must preserve all relevant observable semantics beyond
the specific mistake being repaired.

The compiler must account for at least:

```text
evaluation order
number of evaluations
side effects
panic/error behavior
ownership and move behavior
borrow behavior
control-flow behavior relevant to the edit
```

For example:

```sec
GetValue() >= 0 and GetValue() <= 10
```

must not automatically become:

```sec
GetValue() in 0..10
```

if the original evaluates `GetValue()` twice and that difference is observable.

Effect analysis or equivalent canonical facts may prove that a transformation
is safe in cases where repeated evaluation is pure and observationally
irrelevant according to Sec semantics.

---

# Diagnostic ownership and coalescing

Pitfall analysis must not generate duplicate diagnostics for the same root
cause.

For example, the compiler should not emit all of:

```text
error: index may be out of bounds
error: inclusive range includes len
warning: likely off-by-one
```

for a single proven endpoint mistake.

Instead, one primary diagnostic is produced by the owning normative rule and the
pitfall finding supplies supporting explanation and applicable fixes.

Conceptual diagnostic priority is:

```text
NormativeError
    >
HighConfidenceWarning
    >
Advisory
    >
OptionalInsight
```

The repository-wide diagnostic governance determines concrete diagnostic IDs
and configurable severities.

Pitfall rule identities do not encode the current severity.

---

# Stable pitfall rule identities

Each pitfall pattern has a stable rule identity for configuration, testing,
analysis dumps, profiling, and tooling.

Conceptual examples include:

```text
pitfall.bounds.inclusive-length-index
pitfall.bounds.skipped-zero
pitfall.collection.wrong-bound-source
pitfall.collection.capacity-as-length
pitfall.control-flow.wrong-guard-subject
pitfall.ffi.pointer-extent-origin-mismatch
pitfall.ffi.extent-unit-mismatch
```

These are conceptual identities, not a requirement to use these exact strings
as user-facing diagnostic IDs.

When an occurrence is a proven bounds error, the primary user-facing diagnostic
still belongs to bounds analysis.

---

# Rule registry

The compiler must maintain a canonical registry of pitfall rules.

A rule definition must be able to describe at least:

```text
RuleIdentity
Family
RequiredFacts
MinimumAnalysisBudget
DefaultConfidence
```

and the rule's relation/evidence/suppression/fix behavior.

A rule implementation may become more precise without changing its stable rule
identity so long as it continues to describe the same conceptual programming
mistake.

---

# Rule dependencies

Pitfall rules declare the canonical semantic facts they require.

Examples:

```text
inclusive-length indexing:
    RangeFacts
    BoundsFacts
    ControlFlow

wrong collection bound source:
    PlaceProvenance
    RangeFacts
    CollectionShape

FFI pointer/extent mismatch:
    FFIContract
    PlaceProvenance

byte-count mismatch:
    FFIContract
    TypeLayout
    PlaceProvenance
```

This dependency model allows the analysis scheduler and LSP to avoid expensive
work for rules that are disabled or outside the current analysis budget.

---

# Interactive, Standard, and Deep analysis

Pitfall analysis follows the project-wide analysis budget model.

## Interactive

Interactive analysis prioritizes:

```text
proven normative violations
cheap Proven findings
cheap High-confidence LikelyMistake findings
cheap proven fixes
```

Interactive analysis must remain sound.

It may omit expensive optional rules.

A missing expensive prerequisite produces `NotEvaluated` or `Pending`, not an
unsound `NoFinding` conclusion.

## Standard

Standard analysis includes the required Sec 0.1 high-confidence pitfall catalog
where its prerequisite facts are part of normal compilation analysis.

Project diagnostic policy controls the presentation of optional warnings and
information.

Normative errors are unaffected by pitfall presentation settings.

## Deep

Deep analysis may additionally perform:

```text
more expensive range reasoning
interprocedural intent evidence
cross-function correlation
medium-confidence SuspiciousIntent rules
suppressed-finding explanation
blocked/not-evaluated rule explanation
cross-module correlation where summaries permit it
```

Deep analysis refines information and presentation. It does not change source
semantics.

---

# sec analyse

Pitfall analysis participates in the project-wide analysis CLI model.

Conceptually:

```text
sec analyse
```

runs all available analyses by default, including pitfall analysis.

The tooling must also permit pitfall analysis to be selected individually.

The exact command-line spelling is owned by CLI/tooling rules.

Deep pitfall output may include:

```text
reported findings
suppressed findings
not-evaluated rules
evidence for and against findings
suppression reasons
proven fixes
heuristic suggested edits
missing foreign contracts that block a contract-backed rule
```

---

# LSP presentation

LSP presentation distinguishes:

```text
NeedToKnow
OptionalInsight
```

A proven normative error is `NeedToKnow`.

Examples of `OptionalInsight` include:

```text
likely skipped first element
canonical `value in range` suggestion
suppressed pitfall reasoning
medium-confidence Deep findings
```

Standard hover should not become a dump of every pitfall fact known by the
compiler.

Pitfall information is primarily exposed through:

```text
diagnostics
code actions
explicit analysis results
```

Detailed hover information may be configurable when Deep LSP analysis is
enabled.

---

# Project configuration

Project configuration may control optional pitfall behavior, including
conceptually:

```text
enabled pitfall families
minimum reported confidence
Interactive analysis budget
Deep analysis in LSP
optional advisory presentation
```

This rulebook does not define the concrete configuration syntax.

Optional pitfall settings cannot disable underlying normative language errors.

If a general repository-wide diagnostic suppression mechanism exists, pitfall
advisories use it rather than inventing pitfall-specific source syntax.

---

# LSP configuration reload

Pitfall analysis participates in the project-wide LSP configuration lifecycle.

When analysis-related project configuration changes, the LSP must be able to:

```text
reload the project configuration
invalidate affected pitfall evaluation/filtering state
invalidate affected prerequisite analysis state when required
recompute affected findings
refresh diagnostics
refresh code actions
refresh dependent optional insight
```

An LSP restart must not be required merely to apply changed pitfall-analysis
configuration.

---

# Incremental analysis

Pitfall analysis must support dependency-driven incremental recomputation.

Changing one function must not require an unconditional project-wide rescan of
all local pitfall rules.

A cached finding depends conceptually on:

```text
semantic body identity
relevant CompilationPlan identity
active pitfall configuration
versions/revisions of required semantic facts
pitfall rule identity/version
```

The exact cache key is implementation-defined.

If a change modifies an interprocedural summary consumed by a pitfall rule,
only dependent findings need to be invalidated.

---

# Separate compilation

Pitfall analysis normally consumes canonical summaries owned by other analyses
rather than persisting a separate interprocedural pitfall-summary format.

Useful imported facts may include:

```text
function postconditions
parameter retention summaries
callable contracts
effect summaries
range/shape contracts
canonical FFI buffer relationships
```

Pitfall-specific persisted metadata should be introduced only when a future rule
requires information that cannot reasonably be derived from canonical semantic
summaries.

---

# Determinism

For the same:

```text
source program
active CompilationPlan
compiler version
relevant analysis configuration
```

Standard pitfall results must be deterministic.

Worklist order, thread scheduling, cache hit order, or similar incidental
implementation details must not change whether a finding is produced,
suppressed, or classified.

---

# Required Sec 0.1 implementation scope

The required baseline must support at least the following semantic classes:

```text
inclusive len upper-bound errors
direct index-at-len errors
upper neighbor i + 1 boundary errors
lower neighbor i - 1 boundary errors
ineffective <= len guards
ineffective > len rejection guards
len - 1 without non-empty proof
wrong collection bound source
length/capacity confusion
structural mutation during indexed iteration
wrong guarded value
proven tautological/impossible range conditions
canonical range-membership suggestion where safe
FFI pointer/extent provenance mismatch when canonical contract exists
FFI element-count/byte-count mismatch when canonical contract proves units
```

The required catalog also includes false-positive suppression for common
intentional traversal idioms.

---

# Strong advisory scope

The following classes are expected high-value advisories where evidence reaches
High confidence:

```text
iteration starts at 1 and likely skips element 0
iteration likely omits final element
count used as inclusive endpoint
fragile inclusive `0..len - 1` traversal where canonical half-open traversal is equivalent
cross-value guard/use mismatch
copy/paste collection-bound mismatch
```

They remain advisory unless a separate normative rule proves invalidity.

---

# Deep/future scope

Lower-confidence or more expensive patterns may be implemented in Deep analysis
without being required for the initial Sec 0.1 completion criterion.

Examples include:

```text
copy/paste boolean operands
likely reversed min/max intent
higher-order API misuse
complex resource-intent patterns
complex concurrency-intent patterns
cross-function suspicious invariants
```

Deep/future rules remain subject to the same evidence and false-positive
requirements.

---

# Required test structure

Each required pitfall rule should, where applicable, have four complementary
test classes:

```text
positive detection
boundary variant
intentional/suppressed case
near-miss case
```

This requirement exists to prevent the catalog from growing through detection
examples alone while false positives remain untested.

---

# Required bounds and range tests

The test suite must include at least:

```text
0..len indexed access
direct [len]
i + 1 upper violation
i - 1 lower violation
<= len guard
> len rejection guard
len - 1 on possibly empty collection
correct 0..<len traversal
correct predecessor traversal
correct non-empty proof
guard inside inclusive loop that removes the invalid endpoint before access
start-at-one likely skipped-zero advisory
first element handled before start-at-one loop suppresses advisory
```

---

# Required collection-relation tests

The test suite must include at least:

```text
wrong collection supplies the loop bound
sufficient/equal-length proof suppresses the mismatch finding
capacity used as live length
proven capacity == length suppresses heuristic finding
structural mutation causes stale indexed traversal
structural mutation pattern proven safe does not warn
```

---

# Required control-flow tests

The test suite must include at least:

```text
wrong variable guarded
guard misses equality boundary
guard correctly protects access
condition proven tautological
condition proven impossible
canonical range-membership suggestion
side-effecting expression prevents unsafe automatic canonical rewrite
```

---

# Required FFI tests

When canonical FFI contracts are available, the test suite must include at
least:

```text
matching pointer/extent provenance
mismatched pointer/extent provenance
pointer plus element count
pointer plus byte count
multi-byte element count incorrectly passed to byte-count contract
byte-sized element where len equals byte count
unknown foreign extent relationship produces no contract-backed false advisory
explicit Sec contract and imported/generated canonical contract produce equivalent pitfall facts
```

The FFI tooling/import implementation itself is outside this rulebook.

---

# Required confidence and suppression tests

The test suite must verify at least:

```text
High finding appears in a mode that enables it
Medium finding is omitted from ordinary Interactive output
Medium finding may appear in Deep output
SuppressingEvidence removes a heuristic finding
suppression never removes a normative error
duplicate findings with one root cause coalesce
NotEvaluated is distinguishable from NoFinding
Pending is distinguishable from NoFinding
```

---

# Required corrective-action tests

Every `ProvenFix` class must verify that the resulting program:

```text
parses
type-checks
removes the proven violation it targets
does not introduce additional observable evaluations
does not remove observable evaluations unless equivalence is proven
preserves ownership/move behavior
preserves relevant failure behavior
```

Heuristic `SuggestedEdit` tests must verify that the tool labels the edit as a
suggestion rather than a guaranteed semantic repair.

---

# Required incremental and LSP tests

The test suite must include at least:

```text
edit loop bound -> affected finding invalidated/recomputed
unrelated function edit -> unrelated cached local findings remain reusable
add valid guard -> old finding disappears
remove valid guard -> finding appears
change pitfall project settings -> affected diagnostics refresh
enable deeper LSP analysis -> additional optional findings may appear
disable optional pitfall family -> underlying normative error remains visible
```

---

# Performance tests

Pitfall analysis must be tested on representative large functions/projects with:

```text
many loops
many indexed accesses
large CFGs
many collections
many enabled pitfall rules
```

The tests should verify:

```text
bounded analysis work
incremental invalidation rather than unconditional whole-project rescanning
deterministic results
```

This rulebook does not define a normative wall-clock threshold.

---

# False-positive regression corpus

The compiler should maintain regression cases for intentionally suspicious but
correct code.

Representative cases include:

```text
neighbor traversal
sentinel first element
manual prefix processing
separately processed final element
equal-length parallel collections
ring-buffer indexing
intentional capacity-level storage manipulation
byte-buffer FFI APIs
protocol-defined ranges
explicit fixed-size contracts
```

When a real false positive is found, a regression test should normally be added
before or together with the rule refinement.

For an intent-oriented analysis, false-positive regressions are a core quality
requirement rather than optional polish.

---

# Governance

This rulebook contains normative pitfall-analysis behavior only.

Mutable implementation progress belongs in:

```text
implementation-status.yaml
```

A suitable ledger integration ID is conceptually:

```text
sema.pitfall-analysis
```

The ledger should track granular capabilities such as:

```text
finding model
range/bounds catalog
collection relationships
control-flow relationships
FFI relationships
evidence and suppression
corrective actions
incremental LSP integration
Deep analysis
```

The rulebook must not contain rapidly aging claims about which of these are
currently implemented.

---

# Completion criteria for Sec 0.1

Pitfall analysis is complete for Sec 0.1 when all of the following hold:

1. structured `PitfallFinding` results exist;
2. classification and confidence remain distinct;
3. evidence-for, evidence-against, and suppression are represented
   structurally;
4. proven normative errors remain owned by their canonical analyses;
5. duplicate root-cause diagnostics are coalesced;
6. the required bounds/range catalog is implemented;
7. collection-bound and length/capacity relationships are implemented;
8. relevant control-flow guards participate in pitfall reasoning;
9. canonical Sec range suggestions can be produced where semantically safe;
10. safe proven fixes and heuristic suggested edits remain distinct;
11. fix safety preserves relevant observable semantics;
12. FFI relationship rules operate when canonical FFI metadata is available;
13. missing FFI relationship metadata does not cause name-based guessing;
14. Interactive, Standard, and Deep modes produce the required subsets and
    precision behavior;
15. `NotEvaluated`, `Pending`, `NoFinding`, `Suppressed`, and `Finding` remain
    distinguishable;
16. findings support dependency-driven incremental invalidation;
17. project configuration can control optional findings without disabling
    normative errors;
18. required detection, suppression, near-miss, corrective-action,
    incremental, FFI, and false-positive regression tests pass;
19. mutable implementation status remains governed by
    `implementation-status.yaml`.

---

# Final normative summary

Pitfall analysis helps the programmer by recognizing high-confidence semantic
relationships that commonly indicate human mistakes.

It is not a second type checker, bounds checker, lifetime checker, deadlock
checker, or FFI implementation.

The analyses that own Sec semantics remain authoritative.

Pitfall analysis consumes their canonical facts, correlates those facts, and
produces structured findings with explicit classification, confidence,
evidence, suppression, diagnostic ownership, and corrective actions.

Proven invalidity remains a compiler error under the owning rule.

Likely mistakes and suspicious intent remain configurable analysis results and
do not create new source-language invalidity.

Recognition is semantic rather than merely syntactic, and suppression of common
intentional idioms is a normative requirement.

The initial Sec 0.1 catalog prioritizes bounds, ranges, collection relations,
control-flow relationships, and FFI pointer/extent relationships where explicit
canonical foreign contracts provide enough information.

Pitfall analysis never guesses foreign pointer/length semantics from parameter
names. FFI import and binding generation are separate tooling concerns.

Corrective actions distinguish proven-safe repairs from heuristic suggestions
and prefer canonical Sec idioms when semantic equivalence is established.

Interactive analysis emphasizes proven and cheap high-confidence findings.
Standard analysis supports the required high-confidence catalog. Deep analysis
may provide more expensive intent reasoning, suppressed findings, and optional
insight.

The LSP and `sec analyse` consume the same structured model with different
budgets and presentation policies.

Mutable implementation progress is governed by `implementation-status.yaml`.
