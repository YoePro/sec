# Parser Recovery revision 2.0 — Post-Merge Conformance Review and Focused Correction

- **Status:** Applied 2026-11-17 as `rules/compiler/parser_recovery.md` revision 2.2 (§ 27 reconciled with revision 2.1: existing § 27(4) kept, this correction's § 27(4)–(6) added as § 27(5)–(7)); governance `frontend.parser-long-term` synchronized
- **Created:** 2026-11-02
- **Last updated:** 2026-11-02
- **Document revision:** 1.0
- **Sec language version:** 0.1 (including already approved 0.1.x additions)
- **Canonical owner:** `rules/compiler/parser_recovery.md`, revision 2.0 (2026-10-28)
- **Repository:** `YoePro/sec`
- **Reviewed `main` HEAD:** `cbd88bdd32b9d4b4ff58d3a27927ced4e2822600`
- **Reviewed parser book blob:** `1ec45e6ed4502ad6f08d89fd21e74446e104b806`
- **Governance owners:** `governance/parser.yaml` — existing IDs `frontend.parser-recovery-result`, `frontend.grammar-conformance`, and `frontend.parser-long-term`
- **Normative cross-references:** `rules/foundations/grammar.md`, `rules/foundations/attributes.md`, `rules/tooling/diagnostics.md`, `rules/tooling/formatter.md`, `rules/errors/panic.md`, `rules/errors/errorhandling.md`, `rules/control-flow/flowcontrol_match.md`, `rules/types/types.md`
- **Decision status:** No newly discovered language-design decision is required for the corrections below. Do not create an MD ID unless a new unresolved fork is subsequently demonstrated.

---

## § 1. Scope and authority

**§ 1(1).** This review addresses the **already modernized revision 2.0**, not the superseded 2026-08-07 text. Revision 2.0 already has a metadata header, numbered sections and paragraphs, canonical-syntax ownership boundaries, and synchronization with MD-003, MD-029, MD-042 and MD-102. Preserve that work.

**§ 1(2).** The review covers four questions: alignment with current Sec grammar, stable normative style, separation of normative obligations from implementation work, and genuinely unresolved MD questions. A specification may state an externally observable recovery requirement; it must not prescribe a particular developer's implementation steps or track their completion.

**§ 1(3).** **Never renumber existing § IDs** in revision 2.0. In-place paragraph wording corrections keep the exact IDs; new clauses require fresh IDs or stable suffixes. Cross-references to § 55.4, § 55.5, § 67, § 68, § 69 and § 70 are already aligned with recent accepted decisions and must be preserved.

## § 2. Findings and dispositions

**§ 2(1).** **Confirmed: `§ 14.1(1)` uses task language.** The line `Add:` above the list `InvalidDeclaration`, `InvalidStatement`, `InvalidExpression`, `InvalidTypeReference`, `InvalidPattern`, `InvalidMember` reads as an implementation step in a canonical rulebook. Replace it with a normative requirement. **Severity:** editorial/normative separation. No new governance entry is needed; existing `frontend.parser-recovery-result` already tracks those node families.

**§ 2(2).** **Confirmed: `§ 27` centers the legacy `#target(...)` and omits the corresponding canonical attribute-recovery contract.** Under `rules/foundations/attributes.md`, the canonical `@target(...)` and `@when(...)` declarations are recognized source attributes, whereas `#target` is a compatibility form. The book already mentions canonical attributes in §§ 22 and 69; § 27 should cover them without duplicating target-selection semantics. **Severity:** local omission, no grammar redesign.

**§ 2(3).** **Confirmed: `§ 74(2)` lists possible internal token-representation choices.** The externally relevant rule is complete retention of real tokens, skipped tokens, virtual tokens and trivia. No particular token-buffer/CST/AST implementation option belongs in the normative book. Keep `§ 74(3)` (a lossless CST is not required) and `§ 74(4)` (tooling obligations) if desired; remove the internal implementation menu from `§ 74(2)`.

**§ 2(4).** **Confirmed: `§ 81(2)` contains a seven-step procedural algorithm introduced as a recommendation.** It is useful as engineering guidance but is not a language contract. The normative book already states all important observable invariants in §§ 7, 12–13, 16–17, 20–24, 80 and 84. Recast § 81 as implementation-independent constraints; put any chosen algorithmic sequence only into existing governance or package-specific working instructions.

**§ 2(5).** **Confirmed: `§ 82` still reads partly as a test-work plan.** Imperatives such as `Truncate valid files after every token`, `Feed repeated malformed tokens`, and `Verify ...` describe test execution, not source-language semantics. The book itself says actual tests, fixtures, fuzz campaigns and status belong in governance. Preserve its observed *requirements* (termination, bounded diagnostics, deterministic recovery, sibling retention, complete source ranges and tooling continuity), but remove test-task prescriptions. Reconcile with `frontend.parser-long-term` and the existing recovery harness **without duplicating** items or resetting verified status.

**§ 2(6).** **Reviewed and retained:** § 18's 100-error recommended default is an observable diagnostic budget and already tracked as implemented by `frontend.parser-recovery-result`. The numeric default is not an implementation *task*. Do not silently change this policy during this cleanup; revisit separately only if the owner wants to change the diagnostic contract.

**§ 2(7).** **Reviewed and retained:** §§ 55.4–55.5 (MD-029/042), 67 (contextual matrix `x`), 68 (MD-102), 69 (`panic`, `assert`, `free`, `require` and reserved `?`), and 70 (legacy syntax) are current with the owning October rulebooks. The newer revisions to the canonical source rules do not justify reinstating outdated reserved-syntax language.

## § 3. Exact normative paragraph revisions

The following proposals apply **in place** to existing paragraphs. Source syntax and normative wording are in English, as required for Sec.

### § 3.1. Replace parser book § 14.1(1)

> **§ 14.1(1)** The recovery model must distinguish at least these invalid-node categories when malformed source is committed to the corresponding syntactic construct:
>
> ```text
> InvalidDeclaration
> InvalidStatement
> InvalidExpression
> InvalidTypeReference
> InvalidPattern
> InvalidMember
> ```
>
> These categories describe the retained source structure, not successful semantic analysis. A parser may use any internal representation that preserves the corresponding observable recovery information.

### § 3.2. Supplement parser book § 27 without changing its identity

Replace existing **§ 27(1)–(3)** with the following complete normative contract, retaining those paragraph IDs; add **§ 27(4)–(6)**:

> **§ 27(1)** Recovery of malformed canonical compiler-known attributes uses the attribute grammar in `rules/foundations/attributes.md`. In an `@target(...)` or `@when(...)` argument list, recovery respects nested delimiters and synchronizes at a reliable comma, the attribute's own closing `)`, the next attribute of the same declaration, an unambiguous following declaration, or EOF. A foreign delimiter is not consumed to complete the malformed attribute.
>
> **§ 27(2)** The parser retains the attribute's name, source range, successfully parsed arguments, malformed argument positions and recovery events. An unknown or malformed argument name does not cause loss of later arguments or the declaration to which the attribute is attached.
>
> **§ 27(3)** Parsing a malformed attribute must not reinterpret its remaining tokens as ordinary expressions or as declarations inside the attribute. Recovery must preserve independent adjacent attributes, the owning declaration and later sibling declarations whenever a reliable boundary exists.
>
> **§ 27(4)** Legacy `#target(...)` is recognized only under the compatibility contract of `rules/compiler/compiler.md` § 15. For this legacy form the parser recovers at its own comma, closing `)`, a reliable next top-level declaration or EOF, and it retains the focused legacy/compatibility diagnostic required by the canonical owner.
>
> **§ 27(5)** Target or configuration inclusion is not decided by parser recovery. Malformed source remains invalid even when its declaration would otherwise be excluded by a compilation-plan selector. The selected attribute rulebook owns whether, when and how target selection is evaluated.
>
> **§ 27(6)** The parser must not duplicate the canonical attribute inventory or target-selector semantics. New valid compiler-known attributes automatically use the same recovery invariants through the recognized attribute grammar.

**Review note:** § 27(4) does **not** require changing how existing `#target` files are accepted; it preserves the owner-specific migration status. Recheck exact legacy diagnostic spelling against the authoritative registry rather than inventing one in this book.

### § 3.3. Replace parser book § 74(2)

> **§ 74(2)** The retained source and recovery facts must be sufficient to reconstruct the original spelling and relative positions of real tokens and trivia, to distinguish skipped tokens from consumed tokens and virtual tokens, and to apply source-range-safe tooling edits. The contract does not require a particular token-buffer, CST or AST storage representation.

Existing **§ 74(1), § 74(3), § 74(4)** remain unchanged.

### § 3.4. Replace parser book § 81(1)–(3)

> **§ 81(1)** Parser recovery must guarantee progress or an explicit exit from the current parse context after a failed syntax expectation. It must not repeatedly reconsider the same source token in the same context without advancing, creating one permitted virtual repair, or terminating that recovery episode.
>
> **§ 81(2)** Recovery preserves any enclosing delimiter it does not own, selects only grammar-supported safe insertion or skipping behavior, and retains all source and diagnostic facts required by §§ 11–17 and 73–79. The handling of malformed input remains deterministic under the same language version and compilation context.
>
> **§ 81(3)** No specific parser architecture, repair search algorithm or ordering of internal recovery helpers is mandated. Compilers must satisfy the observable invariants in this rulebook regardless of their chosen algorithm.

### § 3.5. Make parser book § 82 purely normative

Preserve **§ 82** and all existing **§ 82.1–82.8** identities. Replace test-execution phrasing with observable conformance claims. Recommended clauses:

> **§ 82(1)** A conforming Sec parser satisfies the recovery, preservation, diagnostic and progress contracts of this rulebook across valid, incomplete and malformed source. Specific test files, fixtures, fuzz campaigns, verification commands and implementation status belong in the existing parser governance entries.
>
> **§ 82.1(1)** After a recoverable error, the parse result provides bounded structured diagnostics, the primary diagnostic location and recovery action, a valid partial representation where possible, preserved later siblings and source ranges, and deterministic termination without a parser panic.
>
> **§ 82.2(1)** One local malformed token or missing delimiter should normally result in one causal primary parser diagnostic when one safe recovery explains the source; dependent parser errors from the same cause are suppressed as defined in §§ 16–17.
>
> **§ 82.2(2)** This obligation applies equally to malformed field, parameter, case, match-arm and delimiter boundaries; these are examples, not exhaustive special cases.
>
> **§ 82.3(1)** A local recovery must not emit unrelated secondary diagnostics solely because the following field, parameter, statement, case, arm or declaration was incorrectly swallowed or reinterpreted.
>
> **§ 82.4(1)** Finite malformed input, including long runs of repeated delimiters, operators and reserved tokens, must not cause nonterminating parser recovery.
>
> **§ 82.4(2)** When parsing cannot continue safely, recovery must terminate through the explicit fatal-result contract rather than loop indefinitely or return an apparently valid tree.
>
> **§ 82.5(1)** An arbitrary prefix of otherwise valid source, ending at any token boundary, is an admissible malformed input to the parser.
>
> **§ 82.5(2)** Such inputs must terminate without parser panic, obey the diagnostic budget and return a semantically nonpublishable but internally coherent partial result where recovery is possible.
>
> **§ 82.6(1)** A deletion, duplication, substitution or displacement of source tokens must not invalidate progress, source-range, recovery-episode or enclosing-boundary guarantees.
>
> **§ 82.6(2)** A parse result must not silently become a different well-formed program as a consequence of such malformed source.
>
> **§ 82.7(1)** The same requirements apply to malformed lexer output, nested delimiters, long malformed lists or expressions, Unicode identifiers, and unterminated text or comments, subject to the fatal conditions in § 19.
>
> **§ 82.8(1)** During incomplete source editing, tooling may recover valid local semantic and syntactic facts without asserting that the incomplete source is compilable.
>
> **§ 82.8(2)** Completion, document symbols, hovers and code actions must preserve invalid-source boundaries and may suggest only corrections whose safety can be justified by §§ 76–79.

Retitle the § 82.1–82.8 headings to describe their *invariant category* rather than `... tests` without altering any section number. Suggested names: **Recovery invariants**, **Causal diagnostic isolation**, **Cascade suppression**, **Progress and termination**, **Truncated-source behavior**, **Malformed-source robustness**, **Lexer and nesting robustness**, **Tooling continuity**.

## § 4. Governance synchronization

**§ 4(1).** Before adding an implementation task, reconcile all proposed effects against the existing `governance/parser.yaml` entries. **Do not create a new generic integration.** The review found the existing IDs:

| ID | Owner subject | What to reconcile |
|---|---|---|
| `frontend.parser-recovery-result` | Already implemented parse result, event stream, invalid nodes, focused diagnostics and malformed construct retention | § 14, § 18 and observable behavior; retain completed work. |
| `frontend.grammar-conformance` | Canonical source grammar and migration rejections | § 27 / canonical attributes and § 70 / `#target` compatibility. |
| `frontend.parser-long-term` | Remaining specialized recovery gaps, parser contexts and robustness work | § 27 attribute sibling retention if actually missing; § 81/82 implementation work and previously recorded fuzz campaigns. |

**§ 4(2).** Governance may retain concrete test recipes that are removed from § 82. In particular, repeated malformed tokens, token-prefix truncation, mutation sweeps, deterministic fuzz seeds, source-range checks and tooling fixture continuations remain appropriate under the existing testing and recovery records. Their presence in the earlier rulebook is **not evidence** that any work is presently incomplete.

**§ 4(3).** For § 27, check current code and tests before recording a new `remaining` or `identified_bugs` item. If canonical `@target`/`@when` malformed-argument recovery already satisfies the rule, only update the rulebook cross-reference. If the implementation fails a reproducible case, record the discrepancy under the appropriate existing integration ID, with source fixture, location, expected actual behavior and severity. Avoid a duplicate by first searching that integration's existing entries.

**§ 4(4).** Update the existing parser governance record's reviewed normative revision reference when this correction is integrated. Do not declare changes implemented or tests passed without actual repository verification.

## § 5. Acceptance conditions for applying this correction

**§ 5(1).** Work in this order: check repository sync identifier; confirm the canonical file and governance entries remain unchanged relative to this review; apply only the specified paragraph changes; inspect the resulting diff; reconcile governance; run focused parser/grammar/diagnostic/formatter/LSP validation; and review references to the edited sections. Complete one step and verify it before proceeding to the next.

**§ 5(2).** The canonical file must keep all specialized recovery contracts in §§ 22–80, the complete error/Result pattern exception in § 55.4, the MD-042 distinction in § 55.5, contextual `x` in § 67, contextual `Some` in § 68, reserved syntax in § 69, and migration rules in § 70. **No unrelated semantic change** is authorized by this review.

**§ 5(3).** This correction is **proposed and not merged**. The current repository is authoritative until these edits are actually integrated. No source code, governance record, or validation result has been modified by the preparation of this document.
