# Sec Language Principle — Do Not Report Unnecessary Errors

- **Status:** Applied 2026-10-15 to `rules/foundations/language_philosophy.md` (new §§ 10.1–10.5 under § 10, § 22(1) design-test question, § 23(13); revision 2.1) and cross-referenced from `rules/errors/errorhandling.md` § 1. No API behavior was changed; per § 6(3) each existing API contract is reviewed separately in its owning rulebook.
- **Created:** 2026-10-15
- **Last updated:** 2026-10-15
- **Document revision:** 1.0
- **Sec language version:** 0.1 and later
- **Canonical correction path:** `rules/corrections/applied/sec-no-unnecessary-errors-20261015.md`
- **Primary owning rulebook:** `rules/foundations/language_philosophy.md`
- **Related rulebooks:** `rules/errors/errorhandling.md`; relevant API-specific rulebooks as individual contracts are reviewed
- **Decision authority:** Language owner, 2026-10-15
- **Replaces:** No earlier numbered paragraph; amend the existing rulebooks in place without renumbering their established paragraphs
- **Governance:** Synchronize existing error-handling and API contract entries where applicable; do not invent a new missing-decision ID or duplicate existing governance IDs

---

## § 1. Purpose

**§ 1(1).** Sec must not impose error-handling work on callers for conditions that an operation can handle correctly, safely and deterministically within its documented contract.

**§ 1(2).** This is a general language and API design principle. It does not establish behavior for any particular standard-library, platform or compiler-known function.

**§ 1(3).** The principle complements Sec's existing philosophy of *explicit intent, implicit bookkeeping* and its explicit, statically typed `Result[T, E]` error model. It does not introduce exceptions, a new error mechanism or an automatic success fallback.

## § 2. Normative principle

**§ 2(1).** **An error represents a meaningful failure to fulfill an operation's contract, not merely an unusual input or a harmless boundary condition.**

**§ 2(2).** If an operation can satisfy its contract without performing work, it should normally complete successfully rather than report an error solely because no work was needed.

**§ 2(3).** An API must not add an error variant, a `Result` return type or mandatory caller-side error handling solely to distinguish a harmless no-op from ordinary success when the distinction has no contractual value.

**§ 2(4).** The definition of success and failure must be part of the API contract. The compiler must not guess that an error may be suppressed because a condition appears harmless in one calling context.

**§ 2(5).** Harmless boundary behavior must be predictable and may be an immediate return, a no-op or another explicitly specified successful outcome. It must not conceal a failed postcondition.

## § 3. Errors that remain necessary

**§ 3(1).** Errors remain necessary when the caller must know that an operation failed to provide a required result, acquire a resource, preserve data, satisfy a contract or meet another meaningful guarantee.

**§ 3(2).** In particular, this principle does not waive Sec's rules for memory and ownership safety, lifetime and generation validity, overflow, lossy conversions, unsafe boundaries, ABI validation or other mandated checks.

**§ 3(3).** An implementation must not silently convert an actual failure into success, discard owned resources, bypass required diagnostics or weaken existing safety guarantees in order to reduce the number of errors.

**§ 3(4).** A non-error condition that callers need to distinguish may instead be expressed through an appropriate ordinary return value, `bool`, `Option[T]`, a result enum or another explicit type. The correct return shape depends on the operation's contract; this paragraph does not require any specific shape.

## § 4. API design review

**§ 4(1).** Before introducing a new recoverable error, the owning API rulebook should answer:

1. Which documented promise failed?
2. Does the caller need to recover, retry or change control flow because of this condition?
3. Can the operation achieve its promised outcome safely without doing work?
4. Would treating this condition as success conceal information that matters to the caller?
5. Would a normal return value represent the outcome more accurately than an error?

**§ 4(2).** Where the answers show that the operation completes its contract and the distinction is immaterial, prefer ordinary success.

**§ 4(3).** A narrower error surface is desirable only when it remains semantically honest; fewer errors are not a justification for ambiguity or hidden failure.

## § 5. Non-normative examples

**§ 5(1).** Reserving capacity that is already sufficient may be successful without allocating additional storage. Whether a specific reservation API returns `void` or `Result[void, E]` for *other* possible failures remains defined by that API.

**§ 5(2).** A hypothetical operation that removes an absent item may choose to treat absence as a successful no-op if its contract promises only that the item is absent afterward. An operation whose contract promises to retrieve the removed item must instead expose the distinction.

**§ 5(3).** Zero or negative `duration` values are valid `duration` values. Whether a particular wait or scheduling API treats a non-positive value as an immediate successful return, a distinct outcome or a rejected argument is **outside this correction** and belongs to that API's own rulebook.

**§ 5(4).** A failed memory allocation is not a no-op when the caller requested new storage and the operation cannot fulfill that request. It remains an actual failure.

## § 6. Integration requirements

**§ 6(1).** Add this principle to `rules/foundations/language_philosophy.md`, preferably near its existing sections *Avoid semantic bureaucracy*, *Static proof and checked semantics*, and *No hidden semantic surprises*. Preserve existing text and established numbering where present.

**§ 6(2).** Cross-reference this principle from the error-handling design guidance without changing the established semantics of `Result`, `Option`, `try`, `panic` or compiler safety diagnostics.

**§ 6(3).** Do not retrofit a new success rule into an existing API by inference. Review each affected API contract independently and record any resulting behavioral change in its owning rulebook and tests.

**§ 6(4).** Do not specify the behavior of `Wait` in this correction. The discussion that motivated this principle is illustrative, not an API decision.

## § 7. Conformance checks for future API revisions

**§ 7(1).** For each new recoverable error variant, verify that it corresponds to a failure or outcome which callers genuinely need to distinguish.

**§ 7(2).** Include tests for successful boundary or no-op behavior where the specific API contract permits it.

**§ 7(3).** Include negative tests showing that actual contract violations and safety failures remain visible and cannot be silently reclassified as success.

**§ 7(4).** Ensure documentation, generated LSP comments and diagnostics distinguish a successful no-op from an actual failure.
