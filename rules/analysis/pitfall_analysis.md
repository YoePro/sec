# Pitfall Analysis

- **Status:** Normative
- **Created:** 2026-08-08
- **Last updated:** 2026-11-07 (stable chapter, subsection, and paragraph numbering)
- **Document revision:** 2.1
- **Sec language version:** 0.1
- **Canonical path:** `rules/analysis/pitfall_analysis.md`
- **Replaces:** Earlier unversioned revision at the same canonical path
- **Repository baseline reviewed:** `28780c01d63c3fc9eec2321ef67e3dd69962f043`
- **Implementation governance:** `governance/analysis.yaml` (`sema.pitfall-analysis`)
- **Related rulebooks:** `rules/analysis/effect_analysis.md`, `rules/analysis/escape_analysis.md`, `rules/analysis/parameter_usage_analysis.md`, `rules/analysis/call_graph.md`, `rules/analysis/data_races.md`, `rules/analysis/deadlock_analysis.md`, `rules/foundations/operators.md`, `rules/types/contracts.md`, `rules/collections/collections.md`, `rules/collections/shaped-types.md`, `rules/platform/ffi.md`, `rules/tooling/diagnostics.md`, `rules/tooling/lsp.md`, `rules/projects/projects.md`, `rules/compiler/compiler_analysis.md`, `rules/compiler/incremental_compilation.md`

---

## § 1 Status and authority

**§ 1(1)** This is the normative compiler-analysis rulebook for Sec 0.1 pitfall analysis.

**§ 1(2)** This rulebook defines the semantic purpose, finding model, confidence model,
evidence and suppression rules, initial pitfall catalog, diagnostic integration,
corrective-action requirements, analysis-budget behavior, tooling integration,
incremental behavior, tests, and completion criteria for Sec pitfall analysis.

**§ 1(3)** Mutable implementation status does not belong in this rulebook. It is governed
by `sema.pitfall-analysis` in `governance/analysis.yaml`.

---

## § 2 Purpose

**§ 2(1)** Pitfall analysis identifies semantically suspicious programming patterns that
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

**§ 2(2)** Pitfall analysis is not a replacement for type checking, bounds analysis,
ownership analysis, lifetime analysis, escape analysis, deadlock analysis, FFI
validation, or any other analysis that owns a normative Sec rule.

**§ 2(3)** Its role is to correlate facts from those analyses, recognize suspicious
relationships, improve diagnostics, and provide safe or explicitly heuristic
corrective guidance.

**§ 2(4)** The analysis should help the programmer most where ordinary type correctness is
insufficient to reveal a likely mistake.

---

## § 3 Normative role

**§ 3(1)** This rulebook is normative for:

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

**§ 3(2)** This rulebook does not redefine:

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

**§ 3(3)** Those concerns remain owned by their respective normative rulebooks and tooling
specifications.

---

## § 4 Core principle

**§ 4(1)** Pitfall analysis recognizes suspicious semantic relationships.

**§ 4(2)** It does not become the owner of every rule that it observes.

**§ 4(3)** Conceptually:

```text
PitfallAnalysis
    detects suspicious relationships

OwningAnalysis
    defines whether the underlying behavior is valid Sec
```

**§ 4(4)** Examples:

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

**§ 4(5)** Pitfall analysis may enrich such a diagnostic with intent-aware explanation and
a corrective action, but it must not replace the owning semantic rule.

---

## § 5 Proven invalidity is not a warning

**§ 5(1)** When a pitfall implies a proven violation of a normative Sec rule, the resulting
program remains invalid according to that owning rule.

**§ 5(2)** Pitfall analysis must not downgrade the result to a warning or advisory merely
because the mistake resembles a common human error.

**§ 5(3)** For example:

```sec
for i in 0..values.len {
    Process(values[i])
}
```

**§ 5(4)** The loop domain includes `values.len` while valid zero-based indices end before
`values.len`.

**§ 5(5)** If the final indexed access is reachable, this is a proven bounds violation.

**§ 5(6)** The primary diagnostic is therefore the ordinary normative bounds error.
Pitfall analysis may improve it with information such as:

```text
the inclusive range reaches `values.len`
`values.len` is the element count, not a valid index
consider `0..<values.len`
```

---

## § 6 Classification

**§ 6(1)** Every reported pitfall finding has one of the following conceptual
classifications:

```text
ProvenInvalid
LikelyMistake
SuspiciousIntent
```

**§ 6(2)** The exact compiler enum name is implementation-defined.

### § 6.1 ProvenInvalid

**§ 6.1(1)** `ProvenInvalid` means canonical semantic analysis proves that a reachable
execution violates a normative Sec rule.

**§ 6.1(2)** The owning semantic rule determines the actual compiler error.

**§ 6.1(3)** Examples include:

```text
indexing a collection at exactly its length
an inclusive loop endpoint causing a proven final out-of-bounds access
accessing `i + 1` when the loop permits i == len - 1
accessing `i - 1` when the loop permits i == 0
```

### § 6.2 LikelyMistake

**§ 6.2(1)** `LikelyMistake` means the program may remain valid Sec, but semantic evidence
strongly indicates a common accidental pattern.

**§ 6.2(2)** Example:

```sec
for i in 1..<values.len {
    Process(values[i])
}
```

**§ 6.2(3)** may be a likely mistake when:

```text
`values` supplies the loop bound
`values` is indexed by i
index 0 is not otherwise handled
no predecessor/neighbor access explains the starting index
no explicit contract establishes an intentional suffix traversal
```

### § 6.3 SuspiciousIntent

**§ 6.3(1)** `SuspiciousIntent` means the pattern is noteworthy but the compiler lacks enough
evidence to claim high confidence that it is unintended.

**§ 6.3(2)** These findings are optional analysis insight and normally belong to Deep
analysis rather than ordinary edit-time diagnostics.

---

## § 7 Confidence

**§ 7(1)** Classification and confidence are independent concepts.

**§ 7(2)** The analysis must support at least:

```text
Proven
High
Medium
```

**§ 7(3)** Low-confidence speculation should normally produce no diagnostic.

**§ 7(4)** Typical combinations are:

```text
ProvenInvalid    + Proven
LikelyMistake    + High
SuspiciousIntent + Medium
```

**§ 7(5)** The compiler may internally represent greater detail, but it must preserve the
semantic distinction between proof and intent inference.

---

## § 8 No new strict mode

**§ 8(1)** This rulebook does not define `strict`, `use strict`, or any other new strict
compilation mode.

**§ 8(2)** Meaningless but otherwise valid code may be surfaced by optional analysis,
including Deep analysis, when doing so is useful and sufficiently precise.

**§ 8(3)** If a separate Sec rulebook later defines a general strict-policy mechanism,
pitfall analysis may consume that policy. It must not invent such a feature on
its own.

---

## § 9 PitfallFinding

**§ 9(1)** Pitfall rules produce structured findings rather than directly formatting final
user-facing diagnostics.

**§ 9(2)** Conceptually, a finding must be able to carry information equivalent to:

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

**§ 9(3)** The exact data structure is implementation-defined.

**§ 9(4)** The structured finding is the canonical pitfall-analysis product.

**§ 9(5)** Human-readable diagnostics, LSP diagnostics, code actions, and `sec analyse`
output are presentations of that structured result.

---

## § 10 Evidence model

**§ 10(1)** Pitfall analysis must represent evidence structurally before formatting it as
prose.

**§ 10(2)** Evidence categories must be able to express at least the conceptual strengths:

```text
Proof
StrongEvidence
SupportingEvidence
ContradictingEvidence
SuppressingEvidence
```

**§ 10(3)** A numeric scoring model is not required.

**§ 10(4)** The compiler may use one internally, but semantic suppression and classification
must remain explainable through concrete facts.

---

## § 11 Evidence for a finding

**§ 11(1)** Consider:

```sec
for i in 1..<values.len {
    Process(values[i])
}
```

**§ 11(2)** Evidence supporting a skipped-first-element finding may include:

```text
iteration starts at 1
same collection provides upper bound
same collection is indexed by the induction variable
index 0 is not processed on a dominating path
no predecessor access exists
no explicit suffix-traversal contract exists
```

**§ 11(3)** The analysis should use the strongest available semantic facts rather than mere
surface resemblance.

---

## § 12 Evidence against a finding

**§ 12(1)** Consider:

```sec
for i in 1..<values.len {
    Compare(values[i - 1], values[i])
}
```

**§ 12(2)** The predecessor access is strong evidence that starting at index 1 is
intentional because it makes both indexed expressions valid.

**§ 12(3)** That evidence suppresses the skipped-first-element advisory.

**§ 12(4)** This is a normative false-positive-avoidance requirement.

---

## § 13 Suppressing evidence

**§ 13(1)** Heuristic findings must be suppressed when semantic evidence establishes a
common intentional pattern.

**§ 13(2)** Examples of suppressing evidence include:

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

**§ 13(3)** A suppressed finding may remain visible in optional Deep analysis debugging
output when the user asks for detailed analysis reasoning.

**§ 13(4)** It must not be shown as an ordinary warning/advisory after it has been
suppressed.

---

## § 14 Proven errors cannot be heuristically suppressed

**§ 14(1)** Suppression applies to heuristic intent findings.

**§ 14(2)** A proven normative error cannot disappear merely because the pattern appears
intentional.

**§ 14(3)** For example, if an access is proven out of bounds, a recognized coding idiom
cannot make the access valid.

**§ 14(4)** The owning semantic analysis remains authoritative.

---

## § 15 Semantic recognition, not syntax matching

**§ 15(1)** Pitfall analysis must prefer semantic relationships over syntax patterns when
canonical facts can distinguish intended code from suspicious code.

**§ 15(2)** A rule such as:

```text
if loop starts at 1:
    warn
```

**§ 15(3)** is not acceptable.

**§ 15(4)** The compiler must instead consider facts such as:

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

**§ 15(5)** This requirement exists specifically to keep false positives low.

---

## § 16 Normalized relations

**§ 16(1)** Pitfall recognition should consume normalized semantic relations where
available.

**§ 16(2)** Useful conceptual facts include:

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

**§ 16(3)** The exact internal representation is implementation-defined.

**§ 16(4)** Existing Place/provenance, range, control-flow, type, and FFI analyses own the
underlying facts.

**§ 16(5)** Pitfall analysis must not build a second independent provenance system merely to
support its rules.

---

## § 17 Source mapping

**§ 17(1)** Semantic normalization must not discard the source locations and source forms
required for clear diagnostics and code actions.

**§ 17(2)** A finding should be able to explain the programmer's written expression even if
its reasoning used normalized Semantic IR or equivalent canonical facts.

---

## § 18 Reachability

**§ 18(1)** Only reachable semantic behavior contributes to a pitfall finding.

**§ 18(2)** Proven unreachable code does not create a new pitfall finding.

**§ 18(3)** Where Sec already treats proven dead or unreachable code as a compiler error,
the owning reachability diagnostic remains primary.

**§ 18(4)** Pitfall analysis may improve the explanation of why a branch or condition is
impossible.

---

## § 19 Guards participate in pitfall reasoning

**§ 19(1)** Control-flow guards must refine the facts used by pitfall analysis.

**§ 19(2)** For example:

```sec
if values.len == 0 {
    return
}

let last := values[values.len - 1]
```

**§ 19(3)** establishes the non-empty condition required by the final-element access.

**§ 19(4)** No empty-collection pitfall should be reported on the reachable path after the
guard.

**§ 19(5)** Similarly:

```sec
for i in 0..values.len {
    if i == values.len {
        break
    }

    Process(values[i])
}
```

**§ 19(6)** must not receive the simple inclusive-range out-of-bounds finding if control-flow
analysis proves that the indexed access cannot execute for `i == values.len`.

**§ 19(7)** A separate simplification advisory may still be possible, but it is not the
same finding.

---

## § 20 Interprocedural evidence

**§ 20(1)** Pitfall analysis may use interprocedural evidence when available within the
active analysis budget.

**§ 20(2)** For example:

```sec
ValidateNonEmpty(values)
let last := values[values.len - 1]
```

**§ 20(3)** may be proven safe when the imported or inferred summary for
`ValidateNonEmpty` establishes an applicable non-empty postcondition.

**§ 20(4)** Pitfall analysis does not own that postcondition analysis. It consumes the
canonical summary.

**§ 20(5)** Interactive analysis may omit an expensive interprocedural refinement and mark
the relevant rule `NotEvaluated` or `Pending` rather than inventing a conclusion.

---

## § 21 Analysis states

**§ 21(1)** Pitfall rule evaluation must distinguish at least the conceptual states:

```text
Finding
NoFinding
Suppressed
NotEvaluated
Pending
```

**§ 21(2)** `NoFinding` means the rule was evaluated with sufficient required facts and did
not produce a finding.

**§ 21(3)** `NotEvaluated` means the rule was not run or its required facts were not
available under the active budget.

**§ 21(4)** `Pending` is a tooling state indicating that required analysis work is not yet
complete.

**§ 21(5)** `NotEvaluated` and `Pending` must never be cached or interpreted as proof that no
pitfall exists.

---

## § 22 Canonical pitfall families

**§ 22(1)** The pitfall architecture must support multiple semantic families.

**§ 22(2)** Conceptual families include:

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

**§ 22(3)** Sec 0.1 does not require every possible family to have a large catalog.

**§ 22(4)** The required initial implementation prioritizes high-confidence bounds, range,
collection, control-flow, boolean-intent, and contract-backed FFI mistakes.

---

## § 23 Required Sec 0.1 catalog

**§ 23(1)** The following catalog defines the minimum semantic pitfall coverage required for
Sec 0.1.

**§ 23(2)** A particular finding may be implemented by the owning bounds/range analysis and
enriched by pitfall analysis, or directly recognized by pitfall analysis from
canonical facts. The architectural ownership rule remains unchanged.

---

## § 24 API-usage pitfalls

### § 24.1 Large by-value argument at its final use

**§ 24.1(1)** The stable rule identity is `pitfall.ownership.last-use-argument-copy`.
A successfully resolved ordinary owning value argument may receive a
high-confidence `LikelyMistake` advisory recommending `<-name` when:

- canonical transfer facts establish a copy of an available, whole, addressable
  local or parameter binding, with the exact selected parameter type;
- aggregate copy-cost evidence reaches the implementation's large-value threshold;
- complete binding-use and ownership/dependency evidence supports no use after
  that argument, including later argument evaluation and failure handlers;
- moving is compatible with the source's storage and borrowing obligations.

**§ 24.1(2)** The same rule applies to arguments of `new` through its selected initializer.
The default policy reuses the parameter-usage large-value estimate (currently
64 bytes); this is advisory cost policy, not a source-language layout or
liveness rule. The cost of copying the value matters: the size of text behind
an inexpensive string view alone is not large-copy evidence. Unknown costs
cannot establish a recommendation.

**§ 24.1(3)** The analysis must not infer move from the last occurrence of an identifier's
spelling. Binding identity, subsequent branches, loop backedges, receiver or
alias dependencies, deferred uses, captures and fallible control flow matter.
An implementation may conservatively restrict candidates to straight-line
invocations, whole trivially copyable/destructible aggregates and fully known
sources. Unknown borrows/captures or incomplete analysis suppress the advice.
Already moved arguments, borrowing parameters, mandatory consuming arguments,
small values, projected sources and invalid implicit copies receive no duplicate
advisory from this rule. Normative ownership errors retain their own diagnostic.

**§ 24.1(4)** Example:

```sec
let files := ReadFileRecords()
UseRecords(files)
// If files is a large copied value with no remaining use/dependency:
// consider UseRecords(<-files).
```

**§ 24.1(5)** This is a `SuggestedEdit`, not a `ProvenFix`: ownership, identity, cleanup and
failure behavior can change. Never insert a move automatically or silently
change a parameter contract. `copy_move.md` §§ 6.3, 7 and 8.1–8.3 own legality.
The rule participates in Interactive/Standard/Deep optional budgets, structured
finding presentation and dependency-qualified incremental invalidation. Its
prerequisites include selected call/construction and transfer facts, resolved
binding uses, borrowing dependencies and copy-cost policy.


### § 24.2 Explicit `self` passed to its own method

**§ 24.2(1)** An instance-method receiver already supplies `self` implicitly. A call of the
form:

```sec
self.Relate(self)
```

**§ 24.2(2)** is therefore a high-confidence pitfall when semantic call resolution confirms
that `Relate` is an instance method and one explicit argument is the direct
`self` binding.

**§ 24.2(3)** The pitfall result is:

```text
Classification = LikelyMistake
Confidence     = High
OwningRule     = MethodReceiverAndArgumentSemantics
```

**§ 24.2(4)** The compiler must explain that `self` is already the implicit receiver and
must not infer this finding for ordinary functions, static methods, calls on a
different receiver, unresolved calls, or expressions that merely contain
`self` indirectly. No automatic edit is required: an explicit parameter may
still be semantically required by the selected overload, so the programmer
must choose the intended API shape.

---

## § 25 Bounds and range pitfalls

### § 25.1 Inclusive upper bound against collection length

**§ 25.1(1)** Example:

```sec
for i in 0..values.len {
    Process(values[i])
}
```

**§ 25.1(2)** The loop establishes:

```text
0 <= i <= values.len
```

**§ 25.1(3)** while indexing requires:

```text
0 <= i < values.len
```

**§ 25.1(4)** If the indexed access is reachable at the inclusive endpoint, the result is:

```text
Classification = ProvenInvalid
Confidence     = Proven
OwningRule     = Bounds
```

**§ 25.1(5)** The diagnostic should normally recommend the canonical half-open traversal:

```sec
for i in 0..<values.len {
    Process(values[i])
}
```

---

### § 25.2 Direct index at length

**§ 25.2(1)** Example:

```sec
let value := values[values.len]
```

**§ 25.2(2)** This is a proven bounds violation.

**§ 25.2(3)** The diagnostic should explain that `len` is the element count and therefore one
past the last zero-based index.

---

### § 25.3 Upper neighbor access

**§ 25.3(1)** Example:

```sec
for i in 0..<values.len {
    Compare(values[i], values[i + 1])
}
```

**§ 25.3(2)** The analysis must compare the complete loop domain against every indexed
expression.

**§ 25.3(3)** When the loop permits `i == values.len - 1`, the `i + 1` access is invalid.

**§ 25.3(4)** The compiler must not limit this rule to literal syntax; semantically equivalent
normalized offset expressions must be handled when available.

---

### § 25.4 Lower neighbor access

**§ 25.4(1)** Example:

```sec
for i in 0..<values.len {
    Compare(values[i - 1], values[i])
}
```

**§ 25.4(2)** If the loop permits `i == 0`, the predecessor access is invalid.

**§ 25.4(3)** Where the body itself demonstrates predecessor traversal intent, a safe
correction may be to begin at index 1, subject to the ordinary fix-safety rules.

---

### § 25.5 Ineffective upper bounds guard

**§ 25.5(1)** Example:

```sec
if index <= values.len {
    Process(values[index])
}
```

**§ 25.5(2)** If equality remains reachable, the guard does not establish a valid indexing
precondition.

**§ 25.5(3)** The compiler must diagnose the proven boundary error through the owning bounds
rule.

---

### § 25.6 Ineffective rejection guard

**§ 25.6(1)** Example:

```sec
if index > values.len {
    return
}

Process(values[index])
```

**§ 25.6(2)** The guard permits `index == values.len`.

**§ 25.6(3)** If that equality is reachable, the subsequent indexed access is invalid.

---

### § 25.7 Final-element access requires non-empty proof

**§ 25.7(1)** Example:

```sec
// Potential checked unsigned underflow if values is empty.
let last := values[len(values) - 1]
```

**§ 25.7(2)** This expression requires a proof that the collection is non-empty, subject to
Sec's canonical arithmetic and bounds semantics.

**§ 25.7(3)** A preceding guard or type/contract proof may satisfy that requirement. A safe,
non-allocating form is:

```sec
if values.Len > 0 {
    let last := values[values.Len - 1]
    Process(last)
}
```

**§ 25.7(4)** Without such a proof, the owning arithmetic/bounds rules determine validity,
and pitfall analysis may provide the intent-aware explanation.

**§ 25.7(5)** `len(values)` and `values.Len` are `uint` (MD-074/MD-088, 2026-10-17), so for an
empty `values` the subtraction is checked **unsigned underflow** that fails
before any indexing; it is not "indexing at -1". A signed index expression that
yields `-1` is a separate bounds violation. Findings explain which of the two
hazards applies.

**§ 25.7(6)** The same family covers predecessor access (`values[i - 1]` when `i` may be
`0`) and reverse loops:

1. The analysis considers dominating guards and control-flow proofs and never
   warns on the textual presence of `- 1` alone.
2. When invalid arithmetic or bounds are statically proven, the owning
   arithmetic or bounds diagnostic takes priority; pitfall analysis enriches
   the explanation and fix instead of reporting an overlapping error.
3. When safety can be neither proven nor disproven, a configurable pitfall
   finding may explain the missing non-empty or positive-index proof under the
   existing confidence and severity model; no new mandatory diagnostic ID is
   introduced without a diagnostic registry update.
4. A valid dominating non-empty or index-positive proof suppresses the
   finding.

---

## § 26 Skipped-first-element advisory

**§ 26(1)** Example:

```sec
for i in 1..<values.len {
    Process(values[i])
}
```

**§ 26(2)** This is not inherently invalid.

**§ 26(3)** A high-confidence `LikelyMistake` finding may be produced when evidence includes:

```text
the loop starts at index 1
`values` supplies the loop extent
`values` is indexed directly by i
index 0 is not handled on every dominating path
no predecessor/neighbor relation explains the starting index
no explicit contract establishes intentional suffix traversal
```

**§ 26(4)** The finding must be suppressed for common intentional patterns such as:

```sec
for i in 1..<values.len {
    Compare(values[i - 1], values[i])
}
```

**§ 26(5)** or:

```sec
Process(values[0])

for i in 1..<values.len {
    Process(values[i])
}
```

**§ 26(6)** This family is a false-positive-sensitive advisory, not a validity rule.

---

## § 27 Omitted-last-element advisory

**§ 27(1)** A loop that appears intended to traverse a complete collection but provably
stops before its final element may produce a high-confidence advisory when no
semantic evidence explains the omission.

**§ 27(2)** The rule must not treat every shortened range as suspicious.

**§ 27(3)** Intent evidence such as neighbor access, chunking, sentinel handling, explicit
range contracts, or a separately processed final element suppresses the
finding.

---

## § 28 Count versus inclusive endpoint

**§ 28(1)** An expression such as:

```sec
let part := values[start..start + count]
```

**§ 28(2)** contains `count + 1` positions when the range is inclusive.

**§ 28(3)** Pitfall analysis may report a `LikelyMistake` only when surrounding semantic
facts establish that `count` represents an element count rather than an
inclusive endpoint.

**§ 28(4)** Where the intended semantics are proven to be start-plus-count, the canonical
half-open form is:

```sec
let part := values[start..<start + count]
```

**§ 28(5)** No warning may be based solely on the variable name `count` if no semantic
contract or trusted metadata establishes its meaning.

---

## § 29 Avoiding fragile `0..len - 1` workarounds

**§ 29(1)** Code such as:

```sec
for i in 0..values.len - 1 {
    Process(values[i])
}
```

**§ 29(2)** may be valid under a non-empty proof but unnecessarily encodes the half-open
traversal through arithmetic on `len`.

**§ 29(3)** When the compiler proves semantic equivalence and there is no relevant
observable difference, optional analysis may recommend the canonical Sec form:

```sec
for i in 0..<values.len {
    Process(values[i])
}
```

**§ 29(4)** This is normally optional analysis information rather than a language error.

---

## § 30 Collection relationship pitfalls

### § 30.1 Bound source differs from indexed collection

**§ 30.1(1)** Example:

```sec
for i in 0..<left.len {
    Process(right[i])
}
```

**§ 30.1(2)** The analysis must correlate the source of the bound with the collection being
indexed.

**§ 30.1(3)** If it is proven that `right` can be shorter than `left`, the owning bounds rule
may produce a proven invalidity.

**§ 30.1(4)** If the relationship is unknown but:

```text
`left` supplies the bound
`right` is the only collection indexed by i
`left` is otherwise unused in the loop body
no sufficient relation between their lengths is proven
```

**§ 30.1(5)** then a high-confidence copy/paste `LikelyMistake` finding may be appropriate.

**§ 30.1(6)** A proven equality or sufficient lower bound on `right.len` suppresses the
pitfall finding.

---

### § 30.2 Parallel collection traversal

**§ 30.2(1)** Example:

```sec
for i in 0..<left.len {
    Compare(left[i], right[i])
}
```

**§ 30.2(2)** No pitfall is reported when analysis proves that `right.len >= left.len`,
including through a dominating equality check such as:

```sec
if left.len != right.len {
    return
}
```

**§ 30.2(3)** Without a sufficient relationship, the normal bounds result and applicable
pitfall classification follow from the available proof strength.

---

## § 31 Length, capacity, and extent

**§ 31(1)** Length, capacity, and extent are semantically distinct concepts.

**§ 31(2)** Pitfall analysis must preserve that distinction.

### § 31.1 Capacity used as live length

**§ 31.1(1)** Example:

```sec
for i in 0..<values.capacity {
    Process(values[i])
}
```

**§ 31.1(2)** For a collection type where capacity may exceed live length, this is suspicious.

**§ 31.1(3)** If analysis proves that non-live elements can be reached, the owning collection
or bounds semantics determine the resulting error.

**§ 31.1(4)** If the use remains valid only under an unproven equality assumption, a
high-confidence `LikelyMistake` may be produced.

**§ 31.1(5)** A proven `capacity == len` relationship on the relevant path suppresses the
heuristic finding.

---

## § 32 Structural mutation during indexed iteration

**§ 32(1)** Example:

```sec
for i in 0..<values.len {
    if ShouldRemove(values[i]) {
        values.RemoveAt(i)
    }
}
```

**§ 32(2)** Pitfall analysis must correlate:

```text
iteration depends on collection shape
body structurally mutates the same collection
subsequent iteration/index assumptions may become stale
```

**§ 32(3)** Structural mutation is not automatically an error.

**§ 32(4)** The result depends on the collection operation, iteration semantics, control
flow, and whether subsequent indexed accesses remain valid.

**§ 32(5)** The required rule must therefore distinguish:

```text
proven invalid traversal
high-confidence suspicious traversal
proven safe structural pattern
```

**§ 32(6)** rather than warning on every mutation inside a loop.

```text
proven invalid      Clear() followed on the same straight-line path by an
                    index of the collection with the loop binding
likely mistake      RemoveAt, Remove, Clear, or Insert on the collection while
                    the loop indexes it by its binding or passes the binding
                    to the mutation
proven safe         the mutation is directly followed by return or by break
(suppressed)        of this loop; an Append beyond the once-evaluated range;
                    a loop that never addresses the collection by its binding
                    (a draining loop such as RemoveAt(0) per iteration)
```

**§ 32(7)** Structural operations are the compiler-known members marked as structural
mutations. The suggested edit is to traverse in reverse or to use a `while`
loop that advances the index only when no element is removed.

---

## § 33 Boolean representation pitfalls

### § 33.1 Comparing a boolean with `0` or `1`

**§ 33.1(1)** A one-bit register field is read as `bool`. The register write boundary may
permit the hardware spellings `0` and `1`, but that convenience does not make
the value an integer when it is read. The ordinary equality rule therefore
continues to reject a comparison between `bool` and an integer.

**§ 33.1(2)** When the integer operand is the exact compile-time value `0` or `1`, however,
the likely boolean intent is unambiguous enough for a corrective suggestion.
Pitfall analysis must use the following equivalences:

```text
value == 1    -> value
value != 0    -> value
value == 0    -> !value
value != 1    -> !value
```

**§ 33.1(3)** The same proven simplifications apply to explicit boolean literals:

```text
value == true     -> value
value != false    -> value
value == false    -> !value
value != true     -> !value
```

**§ 33.1(4)** Operand order does not affect the result. An outer conversion such as
`bool(value == 1)` is redundant because an equality expression already
produces `bool`; the suggested replacement must therefore cover the whole
expression rather than producing a second, overlapping finding.

**§ 33.1(5)** For an integer `0` or `1`, the owning equality diagnostic remains the primary
type error and the pitfall result supplies a `SuggestedEdit`. For a valid
comparison with `true` or `false`, semantic equivalence is proven and the
replacement may be a `ProvenFix`.

**§ 33.1(6)** No boolean intent may be inferred from another integer, a non-constant numeric
value, or an expression whose boolean operand type has not been resolved.

---

## § 34 Control-flow pitfalls

### § 34.1 Guard checks the wrong value

**§ 34.1(1)** Example:

```sec
if leftIndex < left.len {
    Process(right[rightIndex])
}
```

**§ 34.1(2)** If the checked relation does not establish the precondition required by the
following operation, the compiler should correlate the guard and use.

**§ 34.1(3)** Where the mismatch strongly resembles a copy/paste error, a `LikelyMistake`
finding may be emitted in addition to any owning bounds error.

---

### § 34.2 Safety check without control transfer

**§ 34.2(1)** Example:

```sec
if index >= values.len {
    Log("bad index")
}

Process(values[index])
```

**§ 34.2(2)** If the error branch neither terminates nor establishes a new valid index, the
check does not protect the following access.

**§ 34.2(3)** The owning bounds analysis determines whether the subsequent access is proven
invalid.

**§ 34.2(4)** Pitfall analysis may explain that the apparent safety check does not prevent the
unsafe path and may suggest the likely missing `return`, `continue`, or other
appropriate control transfer only when the intended action can be inferred with
sufficient confidence.

---

## § 35 Meaningless comparisons from proven ranges

**§ 35(1)** When a type or path contract already proves a value's range, comparisons that
cannot affect control flow may be surfaced as optional analysis information.

**§ 35(2)** Examples include conceptually:

```sec
if values.len < 0 {
    ...
}
```

**§ 35(3)** or a comparison outside a constrained named type's permitted range.

**§ 35(4)** If the unreachable branch is already invalid under Sec's dead-code rules, that
owning error remains primary.

**§ 35(5)** If the code is merely meaningless but otherwise valid, this rulebook does not
invent a strict mode to reject it. Such findings may instead be shown by Deep
analysis or by an existing project diagnostic policy if one applies.

---

## § 36 Tautological interval conditions

**§ 36(1)** Example:

```sec
if value >= 0 || value <= 10 {
    ...
}
```

**§ 36(2)** For an ordinary totally ordered numeric value, the condition covers every value
and is therefore tautological.

**§ 36(3)** The compiler should explain the proven boolean result.

**§ 36(4)** When the surrounding form strongly indicates that the programmer intended an
inclusive interval membership test, the diagnostic should prefer Sec's
canonical range-membership syntax:

```sec
if value in 0..10 {
    ...
}
```

**§ 36(5)** rather than merely suggesting replacement of `||` with `&&`.

**§ 36(6)** The canonical suggestion is appropriate only when the intended interval and
comparison semantics are compatible with `in`.

---

## § 37 Canonical idiom guidance

**§ 37(1)** When Sec has a canonical language construct that directly expresses the likely
intended operation, pitfall diagnostics should prefer that construct.

**§ 37(2)** Examples include range membership with `in` and half-open traversal with `..<`.

**§ 37(3)** This preference is a diagnostic/tooling rule, not a change to the semantics of
other equivalent expressions.

---

## § 38 Option, Result, and state-correlation pitfalls

**§ 38(1)** Pitfall analysis may correlate a state check with the value subsequently used.

**§ 38(2)** For example, conceptually:

```sec
if first.IsSome {
    Use(second.Value)
}
```

**§ 38(3)** checking `first` does not establish the state of `second`.

**§ 38(4)** The owning Option/state rules determine whether the access is valid.

**§ 38(5)** Pitfall analysis may add a high-confidence copy/paste explanation when the
checked object and used object differ and no other proof establishes the used
object's state.

**§ 38(6)** Equivalent reasoning applies to Result-like state when a branch proves a state
for one value but the body uses a different value as though it had that state.

**§ 38(7)** No particular Option/Result member spelling is defined by this rulebook.

---

## § 39 Ownership, lifetime, concurrency, and resource families

**§ 39(1)** The pitfall architecture must allow findings in ownership, lifetime,
concurrency, and resource-management domains.

**§ 39(2)** However, Sec 0.1 pitfall analysis must not duplicate the semantic work of the
analyses that already own those areas.

**§ 39(3)** Examples:

```text
retained local borrow
    owner = escape/lifetime

holding a synchronization guard across a blocking operation
    owner = deadlock/concurrency analysis

resource lifetime guaranteed by ordinary Sec destruction
    must not be diagnosed as a C-style leak
```

**§ 39(4)** Lower-confidence style heuristics in these domains are not required for the
initial Sec 0.1 pitfall catalog.

---

## § 40 FFI pitfall analysis

**§ 40(1)** FFI-related pitfall detection must be contract-backed.

**§ 40(2)** Pitfall analysis must not infer pointer/length or pointer/size relationships
merely from parameter names, common C naming conventions, or guesses about a
foreign function's purpose.

**§ 40(3)** The analysis consumes canonical FFI facts supplied by the FFI/tooling system.

**§ 40(4)** Those canonical contracts may originate from:

```text
explicit Sec FFI declarations/contracts
trusted imported foreign metadata
generated foreign-library bindings
compiler-known platform/library metadata
```

**§ 40(5)** How foreign metadata is imported or bindings are generated is outside the scope
of this rulebook.

---

## § 41 Canonical foreign extent relationships

**§ 41(1)** Pitfall analysis must be able to consume a canonical relationship equivalent to:

```text
BufferExtentRelation {
    PointerArgument
    ExtentArgument
    ExtentUnit
    AccessMode
}
```

**§ 41(2)** The exact data structure and spelling are implementation-defined.

**§ 41(3)** The relationship must be able to distinguish at least:

```text
ExtentUnit:
    Elements
    Bytes
```

**§ 41(4)** and access direction when supplied by the FFI contract.

**§ 41(5)** Additional FFI contract dimensions such as retention, nullability, alignment,
ownership transfer, or memory space remain owned by the FFI rulebooks but may
also be consumed where relevant.

---

## § 42 FFI pointer/extent provenance mismatch

**§ 42(1)** Consider a foreign contract stating that argument 1 describes the accessible
extent of the storage addressed by argument 0.

**§ 42(2)** Example call:

```sec
ForeignWrite(first.Ptr, second.len)
```

**§ 42(3)** If canonical provenance establishes:

```text
argument 0 pointer derives from first
argument 1 extent derives from second
```

**§ 42(4)** and no contract proves that `second.len` is intentionally the extent of
`first`, pitfall analysis may report a high-confidence pointer/extent mismatch.

**§ 42(5)** This is a relational finding based on:

```text
foreign contract
+
Place/provenance
```

**§ 42(6)** not on argument names.

---

## § 43 FFI element count versus byte count

**§ 43(1)** Consider:

```sec
ForeignWrite(values.Ptr, values.len)
```

**§ 43(2)** When the canonical foreign contract defines the extent argument in elements,
`values.len` may be exactly the required quantity.

**§ 43(3)** When the canonical foreign contract defines the extent argument in bytes, the
analysis must compare the passed quantity against the element width and the
foreign extent semantics.

**§ 43(4)** For a multi-byte element type, passing element count where byte count is
required may be a proven contract violation when the foreign contract and type
layout make the mismatch unambiguous.

**§ 43(5)** For byte-sized elements, element count and byte count may coincide and must not
produce a false finding.

**§ 43(6)** Unknown or absent extent-unit metadata produces no contract-backed byte-count
pitfall finding.

---

## § 44 FFI pointer/size origin mismatch

**§ 44(1)** The same provenance rule applies to size-bearing objects or structures.

**§ 44(2)** Conceptually:

```sec
ForeignSend(header.Ptr, payload.SizeOf)
```

**§ 44(3)** may be suspicious if the canonical foreign contract requires the size argument
to describe the object addressed by the pointer and provenance proves that the
arguments describe unrelated objects.

**§ 44(4)** The rule must remain silent when the contract does not establish that relation.

---

## § 45 Foreign lifetime mistakes remain owned elsewhere

**§ 45(1)** If a foreign function may retain a pointer and a local or temporary object is
supplied, escape/lifetime/FFI analysis owns the validity result.

**§ 45(2)** Pitfall analysis may improve the diagnostic with wording such as:

```text
the foreign operation may retain this pointer
the supplied storage is local to the current call context
```

**§ 45(3)** but it must not implement a parallel foreign-retention analysis.

---

## § 46 Corrective actions

**§ 46(1)** Pitfall corrective actions are divided conceptually into:

```text
ProvenFix
SuggestedEdit
```

### § 46.1 ProvenFix

**§ 46.1(1)** A `ProvenFix` is an edit whose relevant semantic correctness is established by
the compiler for the finding being repaired.

**§ 46.1(2)** Examples may include:

```sec
if i <= values.len
```

**§ 46.1(3)** becoming:

```sec
if i < values.len
```

**§ 46.1(4)** when the condition is proven to be the bounds guard for the following access.

**§ 46.1(5)** Another example is replacing a semantically equivalent pair of inclusive bound
comparisons with canonical range membership when evaluation semantics are
preserved:

```sec
if value >= 0 && value <= 10 {
    ...
}
```

**§ 46.1(6)** may be represented as:

```sec
if value in 0..10 {
    ...
}
```

**§ 46.1(7)** when the compiler proves equivalence.

### § 46.2 SuggestedEdit

**§ 46.2(1)** A `SuggestedEdit` expresses likely programmer intent but is not proven to be
semantically equivalent to the original program.

**§ 46.2(2)** For example:

```sec
for i in 1..<values.len {
```

**§ 46.2(3)** may be suggested as:

```sec
for i in 0..<values.len {
```

**§ 46.2(4)** when a skipped-first-element finding has high confidence.

**§ 46.2(5)** Such an edit must not be presented as a guaranteed safe automatic repair.

---

## § 47 Fix safety

**§ 47(1)** An automatic proven fix must preserve all relevant observable semantics beyond
the specific mistake being repaired.

**§ 47(2)** The compiler must account for at least:

```text
evaluation order
number of evaluations
side effects
panic/error behavior
ownership and move behavior
borrow behavior
control-flow behavior relevant to the edit
```

**§ 47(3)** For example:

```sec
GetValue() >= 0 && GetValue() <= 10
```

**§ 47(4)** must not automatically become:

```sec
GetValue() in 0..10
```

**§ 47(5)** if the original evaluates `GetValue()` twice and that difference is observable.

**§ 47(6)** Effect analysis or equivalent canonical facts may prove that a transformation
is safe in cases where repeated evaluation is pure and observationally
irrelevant according to Sec semantics.

---

## § 48 Diagnostic ownership and coalescing

**§ 48(1)** Pitfall analysis must not generate duplicate diagnostics for the same root
cause.

**§ 48(2)** For example, the compiler should not emit all of:

```text
error: index may be out of bounds
error: inclusive range includes len
warning: likely off-by-one
```

**§ 48(3)** for a single proven endpoint mistake.

**§ 48(4)** Instead, one primary diagnostic is produced by the owning normative rule and the
pitfall finding supplies supporting explanation and applicable fixes.

**§ 48(5)** Conceptual diagnostic priority is:

```text
NormativeError
    >
HighConfidenceWarning
    >
Advisory
    >
OptionalInsight
```

**§ 48(6)** `rules/tooling/diagnostics.md` defines diagnostic identity, severity,
configurability, structured occurrences, fixes, and CLI/LSP transport.

**§ 48(7)** Pitfall rule identity is analysis identity. It must not be substituted for a
registered compiler diagnostic ID.

**§ 48(8)** Pitfall rule identities do not encode the current severity.

---

## § 49 Stable pitfall rule identities

**§ 49(1)** Each pitfall pattern has a stable rule identity for configuration, testing,
analysis dumps, profiling, and tooling.

**§ 49(2)** Conceptual examples include:

```text
pitfall.bounds.inclusive-length-index
pitfall.bounds.skipped-zero
pitfall.collection.wrong-bound-source
pitfall.collection.capacity-as-length
pitfall.control-flow.wrong-guard-subject
pitfall.ffi.pointer-extent-origin-mismatch
pitfall.ffi.extent-unit-mismatch
```

**§ 49(3)** These are stable pitfall-analysis rule identities used for configuration,
testing, analysis dumps, profiling, dependency tracking, and tooling.

**§ 49(4)** They are not compiler diagnostic IDs.

**§ 49(5)** When a pitfall occurrence owns a user-facing advisory diagnostic, that
diagnostic must use a separately registered stable diagnostic ID from
`rules/tooling/diagnostics.md`.

**§ 49(6)** When the underlying condition is a proven violation owned by another semantic
analysis, that owning analysis supplies the primary diagnostic identity.

---

## § 50 Rule registry

**§ 50(1)** The compiler must maintain a canonical registry of pitfall rules.

**§ 50(2)** A rule definition must be able to describe at least:

```text
RuleIdentity
Family
RequiredFacts
MinimumAnalysisBudget
DefaultConfidence
```

**§ 50(3)** and the rule's relation/evidence/suppression/fix behavior.

**§ 50(4)** A rule implementation may become more precise without changing its stable rule
identity so long as it continues to describe the same conceptual programming
mistake.

---

## § 51 Rule dependencies

**§ 51(1)** Pitfall rules declare the canonical semantic facts they require.

**§ 51(2)** Examples:

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

**§ 51(3)** This dependency model allows the analysis scheduler and LSP to avoid expensive
work for rules that are disabled or outside the current analysis budget.

---

## § 52 Interactive, Standard, and Deep analysis

**§ 52(1)** Pitfall analysis follows the project-wide analysis budget model.

### § 52.1 Interactive

**§ 52.1(1)** Interactive analysis prioritizes:

```text
proven normative violations
cheap Proven findings
cheap High-confidence LikelyMistake findings
cheap proven fixes
```

**§ 52.1(2)** Interactive analysis must remain sound.

**§ 52.1(3)** It may omit expensive optional rules.

**§ 52.1(4)** A missing expensive prerequisite produces `NotEvaluated` or `Pending`, not an
unsound `NoFinding` conclusion.

### § 52.2 Standard

**§ 52.2(1)** Standard analysis includes the required Sec 0.1 high-confidence pitfall catalog
where its prerequisite facts are part of normal compilation analysis.

**§ 52.2(2)** Project diagnostic policy controls the presentation of optional warnings and
information.

**§ 52.2(3)** Normative errors are unaffected by pitfall presentation settings.

### § 52.3 Deep

**§ 52.3(1)** Deep analysis may additionally perform:

```text
more expensive range reasoning
interprocedural intent evidence
cross-function correlation
medium-confidence SuspiciousIntent rules
suppressed-finding explanation
blocked/not-evaluated rule explanation
cross-module correlation where summaries permit it
```

**§ 52.3(2)** Deep analysis refines information and presentation. It does not change source
semantics.

---

## § 53 sec analyse

**§ 53(1)** Pitfall analysis participates in the project-wide analysis CLI model.

**§ 53(2)** Conceptually:

```text
sec analyse
```

**§ 53(3)** runs all available analyses by default, including pitfall analysis.

**§ 53(4)** The tooling must also permit pitfall analysis to be selected individually:
`sec analyse pitfalls` (`rules/tooling/cli.md` § 5.2, MD-017).

**§ 53(5)** Deep pitfall output may include:

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

## § 54 LSP presentation

**§ 54(1)** LSP presentation distinguishes:

```text
NeedToKnow
OptionalInsight
```

**§ 54(2)** A proven normative error is `NeedToKnow`.

**§ 54(3)** Examples of `OptionalInsight` include:

```text
likely skipped first element
canonical `value in range` suggestion
suppressed pitfall reasoning
medium-confidence Deep findings
```

**§ 54(4)** Standard hover should not become a dump of every pitfall fact known by the
compiler.

**§ 54(5)** Pitfall information is primarily exposed through:

```text
diagnostics
code actions
explicit analysis results
```

**§ 54(6)** Detailed hover information may be configurable when Deep LSP analysis is
enabled.

---

## § 55 Project configuration

**§ 55(1)** Project configuration may control optional pitfall behavior, including
conceptually:

```text
enabled pitfall families
minimum reported confidence
Interactive analysis budget
Deep analysis in LSP
optional advisory presentation
```

**§ 55(2)** This rulebook does not define the concrete configuration syntax.

**§ 55(3)** Project-file syntax and configuration precedence are owned by
`rules/projects/projects.md`.

**§ 55(4)** Pitfall analysis consumes the resolved diagnostic policy. It does not define a
second project-configuration syntax.

**§ 55(5)** Optional pitfall settings cannot disable underlying normative language errors.

**§ 55(6)** If a general repository-wide diagnostic suppression mechanism exists, pitfall
advisories use it rather than inventing pitfall-specific source syntax.

---

## § 56 LSP configuration reload

**§ 56(1)** Pitfall analysis participates in the project-wide LSP configuration lifecycle.

**§ 56(2)** When analysis-related project configuration changes, the LSP must be able to:

```text
reload the project configuration
invalidate affected pitfall evaluation/filtering state
invalidate affected prerequisite analysis state when required
recompute affected findings
refresh diagnostics
refresh code actions
refresh dependent optional insight
```

**§ 56(3)** An LSP restart must not be required merely to apply changed pitfall-analysis
configuration.

---

## § 57 Incremental analysis

**§ 57(1)** Pitfall analysis must support dependency-driven incremental recomputation.

**§ 57(2)** Changing one function must not require an unconditional project-wide rescan of
all local pitfall rules.

**§ 57(3)** A cached finding depends conceptually on:

```text
semantic body identity
relevant CompilationPlan identity
active pitfall configuration
versions/revisions of required semantic facts
pitfall rule identity/version
```

**§ 57(4)** The exact cache key is implementation-defined.

**§ 57(5)** If a change modifies an interprocedural summary consumed by a pitfall rule,
only dependent findings need to be invalidated.

---

## § 58 Separate compilation

**§ 58(1)** Pitfall analysis normally consumes canonical summaries owned by other analyses
rather than persisting a separate interprocedural pitfall-summary format.

**§ 58(2)** Useful imported facts may include:

```text
function postconditions
parameter retention summaries
callable contracts
effect summaries
range/shape contracts
canonical FFI buffer relationships
```

**§ 58(3)** Pitfall-specific persisted metadata should be introduced only when a future rule
requires information that cannot reasonably be derived from canonical semantic
summaries.

---

## § 59 Determinism

**§ 59(1)** For the same:

```text
source program
active CompilationPlan
compiler version
relevant analysis configuration
```

**§ 59(2)** Standard pitfall results must be deterministic.

**§ 59(3)** Worklist order, thread scheduling, cache hit order, or similar incidental
implementation details must not change whether a finding is produced,
suppressed, or classified.

---

## § 60 Required Sec 0.1 implementation scope

**§ 60(1)** The required baseline must support at least the following semantic classes:

```text
inclusive len upper-bound errors
direct index-at-len errors
redundant bool comparison with true/false
bool comparison with exact integer 0/1, linked to the owning type error
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

**§ 60(2)** The required catalog also includes false-positive suppression for common
intentional traversal idioms.

---

## § 61 Strong advisory scope

**§ 61(1)** The following classes are expected high-value advisories where evidence reaches
High confidence:

```text
iteration starts at 1 and likely skips element 0
iteration likely omits final element
count used as inclusive endpoint
fragile inclusive `0..len - 1` traversal where canonical half-open traversal is equivalent
cross-value guard/use mismatch
copy/paste collection-bound mismatch
```

**§ 61(2)** They remain advisory unless a separate normative rule proves invalidity.

---

## § 62 Deep/future scope

**§ 62(1)** Lower-confidence or more expensive patterns may be implemented in Deep analysis
without being required for the initial Sec 0.1 completion criterion.

**§ 62(2)** Examples include:

```text
copy/paste boolean operands
likely reversed min/max intent
higher-order API misuse
complex resource-intent patterns
complex concurrency-intent patterns
cross-function suspicious invariants
```

**§ 62(3)** Deep/future rules remain subject to the same evidence and false-positive
requirements.

---

## § 63 Required test structure

**§ 63(1)** Each required pitfall rule should, where applicable, have four complementary
test classes:

```text
positive detection
boundary variant
intentional/suppressed case
near-miss case
```

**§ 63(2)** This requirement exists to prevent the catalog from growing through detection
examples alone while false positives remain untested.

---

## § 64 Required bounds and range tests

**§ 64(1)** The test suite must include at least:

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

## § 65 Required collection-relation tests

**§ 65(1)** The test suite must include at least:

```text
wrong collection supplies the loop bound
sufficient/equal-length proof suppresses the mismatch finding
capacity used as live length
proven capacity == length suppresses heuristic finding
structural mutation causes stale indexed traversal
structural mutation pattern proven safe does not warn
```

---

## § 66 Required control-flow tests

**§ 66(1)** The test suite must include at least:

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

## § 67 Required FFI tests

**§ 67(1)** When canonical FFI contracts are available, the test suite must include at
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

**§ 67(2)** The FFI tooling/import implementation itself is outside this rulebook.

---

## § 68 Required confidence and suppression tests

**§ 68(1)** The test suite must verify at least:

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

## § 69 Required corrective-action tests

**§ 69(1)** Every `ProvenFix` class must verify that the resulting program:

```text
parses
type-checks
removes the proven violation it targets
does not introduce additional observable evaluations
does not remove observable evaluations unless equivalence is proven
preserves ownership/move behavior
preserves relevant failure behavior
```

**§ 69(2)** Heuristic `SuggestedEdit` tests must verify that the tool labels the edit as a
suggestion rather than a guaranteed semantic repair.

---

## § 70 Required incremental and LSP tests

**§ 70(1)** The test suite must include at least:

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

## § 71 Performance tests

**§ 71(1)** Pitfall analysis must be tested on representative large functions/projects with:

```text
many loops
many indexed accesses
large CFGs
many collections
many enabled pitfall rules
```

**§ 71(2)** The tests should verify:

```text
bounded analysis work
incremental invalidation rather than unconditional whole-project rescanning
deterministic results
```

**§ 71(3)** This rulebook does not define a normative wall-clock threshold.

---

## § 72 False-positive regression corpus

**§ 72(1)** The compiler should maintain regression cases for intentionally suspicious but
correct code.

**§ 72(2)** Representative cases include:

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

**§ 72(3)** When a real false positive is found, a regression test should normally be added
before or together with the rule refinement.

**§ 72(4)** For an intent-oriented analysis, false-positive regressions are a core quality
requirement rather than optional polish.

---

## § 73 Governance

**§ 73(1)** This rulebook contains normative pitfall-analysis behavior only.

**§ 73(2)** Mutable implementation progress belongs to:

```text
governance/analysis.yaml
```

**§ 73(3)** under the canonical integration ID:

```text
sema.pitfall-analysis
```

**§ 73(4)** That governance integration owns implementation tracking for at least:

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

**§ 73(5)** Implementation claims, current bug lists, implementation percentages, and
temporary compiler limitations must not be copied into this rulebook.

**§ 73(6)** Cross-area implementation work remains owned by its existing governance
integration. In particular:

```text
diagnostic transport
    governance/errors_diagnostics.yaml

LSP presentation
    governance/tooling_lsp.yaml

FFI semantic support
    governance/ffi.yaml

compiler analysis coordination
    owning compiler/analysis governance integration
```

**§ 73(7)** The pitfall governance entry may depend on those integrations but must not
duplicate their implementation state.

---

## § 74 Completion criteria for Sec 0.1

**§ 74(1)** Pitfall analysis is complete for Sec 0.1 when all of the following hold:

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
    `sema.pitfall-analysis` in `governance/analysis.yaml`.

---

## § 75 Final normative summary

**§ 75(1)** Pitfall analysis helps the programmer by recognizing high-confidence semantic
relationships that commonly indicate human mistakes.

**§ 75(2)** It is not a second type checker, bounds checker, lifetime checker, deadlock
checker, or FFI implementation.

**§ 75(3)** The analyses that own Sec semantics remain authoritative.

**§ 75(4)** Pitfall analysis consumes their canonical facts, correlates those facts, and
produces structured findings with explicit classification, confidence,
evidence, suppression, diagnostic ownership, and corrective actions.

**§ 75(5)** Proven invalidity remains a compiler error under the owning rule.

**§ 75(6)** Likely mistakes and suspicious intent remain configurable analysis results and
do not create new source-language invalidity.

**§ 75(7)** Recognition is semantic rather than merely syntactic, and suppression of common
intentional idioms is a normative requirement.

**§ 75(8)** The initial Sec 0.1 catalog prioritizes bounds, ranges, collection relations,
control-flow relationships, and FFI pointer/extent relationships where explicit
canonical foreign contracts provide enough information.

**§ 75(9)** Pitfall analysis never guesses foreign pointer/length semantics from parameter
names. FFI import and binding generation are separate tooling concerns.

**§ 75(10)** Corrective actions distinguish proven-safe repairs from heuristic suggestions
and prefer canonical Sec idioms when semantic equivalence is established.

**§ 75(11)** Interactive analysis emphasizes proven and cheap high-confidence findings.
Standard analysis supports the required high-confidence catalog. Deep analysis
may provide more expensive intent reasoning, suppressed findings, and optional
insight.

**§ 75(12)** The LSP and `sec analyse` consume the same structured model with different
budgets and presentation policies.

**§ 75(13)** Mutable implementation progress is governed by `sema.pitfall-analysis` in
`governance/analysis.yaml`.
