# Language Philosophy

- **Status:** Normative
- **Created:** 2026-08-12
- **Last updated:** 2026-10-02
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/foundations/language_philosophy.md`
- **Replaces:** Earlier unversioned revision at the same canonical path
- **Repository baseline reviewed:** `main-reviewed-2026-10-02`
- **Implementation governance:** Not applicable; this rulebook defines design principles rather than implementation-status work items.
- **Related rulebooks:** `rules/README.md`, `rules/foundations/grammar.md`, `rules/foundations/lexical_structure.md`, `rules/foundations/names_scopes_visibility.md`, `rules/foundations/operators.md`, `rules/foundations/attributes.md`, `rules/types/types.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/errors/errorhandling.md`, `rules/errors/runtime_checks.md`, `rules/errors/panic.md`, `rules/tooling/diagnostics.md`, `rules/compiler/compiler_pipeline.md`, `rules/compiler/semantic_ir.md`, `rules/platform/platform_model.md`

---

## § 1. Purpose and authority

§ 1(1) This rulebook defines the design principles used to evaluate Sec language, compiler, core-library, standard-library, tooling, and platform-model decisions.

§ 1(2) Sec is designed to be simple to read, simple to write, and predictable to reason about.

§ 1(3) Sec is intended to support both ordinary application development and low-level systems programming without forcing unnecessary complexity on either.

§ 1(4) Complexity should exist where it provides value.

§ 1(5) When complexity can be handled reliably by the compiler, it should normally remain in the compiler rather than being exposed as source-level ceremony.

§ 1(6) The programmer should express intent. The compiler should prove the consequences.

§ 1(7) This rulebook does not override a more specific normative rulebook. Where a concrete language rule has been deliberately specified, that concrete rule is authoritative for that subject.

§ 1(8) These principles guide new decisions, interpretation of genuinely unspecified areas, and review of proposed revisions. They must not be used to silently invent semantics that another owning rulebook has not defined.

---

## § 2. Independent design

§ 2(1) Sec is influenced by many existing programming languages and programming traditions.

§ 2(2) These include, but are not limited to:

```text
C
C++
C#
F#
Go
Rust
Zig
Ada
Vale
Odin
```

§ 2(3) Sec is not designed as a derivative or replacement syntax for any one of them.

§ 2(4) A concept may be adopted when it serves Sec's goals.

§ 2(5) A concept may be modified when Sec can make it simpler, safer, clearer, or more consistent.

§ 2(6) A concept may be rejected even when it is established practice in another language.

§ 2(7) Similar syntax does not imply identical semantics.

§ 2(8) Familiarity is useful, but consistency within Sec has higher priority than compatibility with expectations created by another language.

§ 2(9) Language decisions must be justified by Sec's own requirements and design principles, not merely because another language does or does not use a particular design.

§ 2(10) Other languages are valuable sources of experience. They are not normative sources for Sec.

---

## § 3. Practical simplicity

§ 3(1) Sec should be understandable by competent programmers without requiring specialist academic knowledge for ordinary programming.

§ 3(2) Technical concepts are not avoided merely because they are sophisticated.

§ 3(3) Unnecessary intellectual or terminological complexity should be avoided.

§ 3(4) When a concept can be described accurately using ordinary programming terminology, programmer-facing documentation and diagnostics should prefer that terminology over specialist terminology that adds no practical value.

§ 3(5) The compiler implementation may use advanced algorithms, formal models, and specialized terminology internally.

§ 3(6) Internal complexity should not leak into source syntax merely because the compiler uses it internally.

§ 3(7) The goal is not to make the compiler simple. The goal is to make correct programming comprehensible.

---

## § 4. Explicit intent, implicit bookkeeping

§ 4(1) Sec should not require syntax whose only purpose is to repeat information the compiler can already determine safely and unambiguously.

§ 4(2) Explicit syntax is valuable when it expresses a real semantic choice.

§ 4(3) Such choices include, where defined by their owning rulebooks:

```text
mutability
fallibility
unsafe operations
borrowing authority
ownership transfer
external storage
hardware semantics
units
contracts
target-sensitive operations
```

§ 4(4) Explicit syntax is not valuable merely because it makes compiler implementation easier.

§ 4(5) Sec should avoid semantic bureaucracy: annotations, qualifiers, declarations, or ceremony that force the programmer to maintain information already known or safely provable by the compiler.

§ 4(6) The preferred rule is:

```text
explicit intent
implicit bookkeeping
```

§ 4(7) The programmer describes what is meant. The compiler performs the bookkeeping needed to prove that it is valid.

§ 4(8) When an operation represents a semantic choice that cannot be derived safely, the language should require that choice to be explicit rather than guess.

---

## § 5. Compiler responsibility

§ 5(1) The Sec compiler is expected to perform substantial semantic analysis.

§ 5(2) This includes, where applicable:

```text
type analysis
ownership analysis
borrowing analysis
lifetime and reference validation
escape analysis
effect analysis
control-flow analysis
definite-assignment analysis
copy and move analysis
destruction and cleanup planning
stack analysis
recursion analysis
ISR analysis
concurrency analysis
compile-time evaluation
contract validation
unit analysis
platform validation
ABI and FFI validation
optimization-legality analysis
```

§ 5(3) These analyses are compiler responsibilities.

§ 5(4) They should influence source syntax only where the programmer must provide information that cannot be derived safely or where an explicit choice is semantically important.

§ 5(5) Sophisticated compiler analysis is not, by itself, justification for sophisticated source syntax.

§ 5(6) Compiler implementation difficulty is not sufficient reason to weaken a language guarantee.

---

## § 6. Compiler as mentor

§ 6(1) Rejecting an incorrect program is not sufficient when the compiler can reasonably explain the problem.

§ 6(2) Diagnostics should help the programmer understand:

```text
what rule was violated
where relevant values or declarations originated
why the compiler reached its conclusion
what operation caused the conflict
what change may resolve the problem when a valid suggestion exists
```

§ 6(3) Compiler analysis should therefore be designed to answer both whether the program is valid and, when practical, why and how the programmer can resolve the problem.

§ 6(4) Diagnostics must not claim certainty beyond the analysis that was actually performed.

§ 6(5) Advanced analysis that cannot support useful explanation should be treated with caution when a simpler model can provide comparable safety with clearer diagnostics.

§ 6(6) Exact diagnostic identity, structure, severity, localization, and tooling behavior are owned by `rules/tooling/diagnostics.md`.

---

## § 7. Prove, check, or reject

§ 7(1) Sec prefers static proof.

§ 7(2) When the compiler proves that an operation is valid, it should not emit runtime validation merely for defensive purposes unless another normative rule requires an observable check.

§ 7(3) When a language safety condition depends on runtime data and static proof is insufficient, the language may define runtime validation.

§ 7(4) Examples include, where defined by owning rulebooks:

```text
dynamic bounds validation
runtime contract validation
arithmetic checks
reference-generation validation
representation validation
```

§ 7(5) Runtime validation does not imply garbage collection or a mandatory general-purpose runtime.

§ 7(6) A runtime check may lower directly to ordinary machine instructions or target-specific support.

§ 7(7) The general principle is:

```text
prove when possible
check when required by defined semantics
reject when the required guarantee cannot otherwise be established
```

§ 7(8) The compiler must not silently weaken a language guarantee merely because proving it statically is difficult.

§ 7(9) Concrete runtime-check and panic-free semantics are owned by `rules/errors/runtime_checks.md` and `rules/errors/panic.md`.

---

## § 8. Safety without removing low-level control

§ 8(1) Sec is intended to support low-level programming.

§ 8(2) Relevant domains include operating-system interfaces, FFI, memory-mapped hardware, embedded systems, bare-metal systems, allocators, device drivers, platform runtimes, and systems software.

§ 8(3) Low-level capability must not require the entire language to adopt unsafe semantics.

§ 8(4) Safe code should retain the strongest guarantees the compiler can provide.

§ 8(5) Operations whose correctness cannot generally be verified must cross an explicit unsafe boundary or another explicitly defined low-level mechanism.

§ 8(6) Unsafe code does not disable the language.

§ 8(7) Inside unsafe code, ordinary rules remain in force except for the specific proof obligation explicitly transferred to the programmer by the unsafe operation.

§ 8(8) Unsafe therefore means that a specific operation contains assumptions the compiler cannot prove. It does not mean that the compiler stops checking the program.

---

## § 9. Deterministic semantics

§ 9(1) Sec favors deterministic language semantics.

§ 9(2) When behavior can affect program correctness, resource lifetime, ownership state, or observable execution, the language should define it rather than leave it accidentally dependent on implementation details.

§ 9(3) This includes ownership transfer, copy and move behavior, destruction, cleanup, defer execution, expression evaluation order, control flow, initialization, and error propagation.

§ 9(4) Optimization may remove, combine, or rearrange operations only when observable Sec semantics remain unchanged.

§ 9(5) Backend convenience must not define source-language behavior.

§ 9(6) Deliberately unspecified behavior, implementation-defined behavior, and target-defined behavior must be explicitly classified as such by the owning rulebook rather than arising accidentally.

---

## § 10. No hidden semantic surprises

§ 10(1) Source code should communicate operations that have important semantic or performance consequences.

§ 10(2) Sec should avoid hidden behavior such as unexpected heap allocation, unexpected ownership transfer, hidden garbage collection, hidden reference counting, implicit expensive copying, implicit resource acquisition, implicit exception mechanisms, unexpected dynamic dispatch, and backend-dependent safety behavior.

§ 10(3) This does not mean that every machine instruction must be visible in source.

§ 10(4) The compiler is expected to generate substantial implementation machinery.

§ 10(5) The distinction is between hidden implementation and hidden semantics.

§ 10(6) Compiler-generated implementation is desirable when it faithfully implements clear source semantics.

§ 10(7) Compiler-generated semantic surprises are not.

---

## § 11. Cost awareness

§ 11(1) Sec should make it possible for programmers to reason about important costs.

§ 11(2) The language need not expose every machine-level cost explicitly.

§ 11(3) Operations with substantially different ownership, allocation, copying, synchronization, or dispatch behavior should not be made indistinguishable when the distinction matters to program design.

§ 11(4) Zero-cost abstractions are desirable when practical.

§ 11(5) Correctness and comprehensibility take priority over slogans about zero cost.

§ 11(6) A predictable and explicit cost is preferable to an invisible and surprising one.

---

## § 12. Ownership and memory

§ 12(1) Sec uses deterministic ownership and compile-time analysis as central tools for memory and resource safety.

§ 12(2) The ownership model should remain understandable without requiring programmers to manually describe compiler-internal lifetime relationships.

§ 12(3) Programmers should not normally need explicit lifetime annotations.

§ 12(4) Borrowing and reference rules should prevent invalid programs while avoiding unnecessary source-level bookkeeping.

§ 12(5) When the compiler cannot prove that an ownership, borrowing, or lifetime relationship is safe, Sec should prefer a clear diagnostic over exposing increasingly complex compiler-internal annotations merely to make the program accepted.

§ 12(6) Compiler-internal concepts such as regions, data-flow states, and lifetime models are implementation techniques unless a separate normative rule deliberately exposes them.

§ 12(7) Internal analysis concepts are not automatically source-language concepts.

---

## § 13. Abstraction without loss of control

§ 13(1) High-level abstractions and low-level control are not opposing goals.

§ 13(2) A feature may provide a high-level source representation while lowering to direct and predictable machine behavior.

§ 13(3) Examples include register fields instead of repeated manual masks and shifts, units instead of unchecked numeric conventions, typed `Result` values instead of hidden error channels, properties instead of manually repeated access logic, safe references instead of ordinary raw pointers, and compiler-generated cleanup instead of manually repeated release logic.

§ 13(4) An abstraction is useful when it reduces programmer error without hiding semantically important behavior.

§ 13(5) Sec must not equate low-level programming with low-level syntax.

---

## § 14. One language across targets

§ 14(1) Sec targets both hosted and freestanding environments.

§ 14(2) These include desktop and server operating systems, embedded operating systems, microcontrollers, and bare-metal targets.

§ 14(3) Core language semantics remain the same across targets.

§ 14(4) Targets and target profiles may differ in available capabilities.

§ 14(5) A target may lack heap allocation, threads, operating-system services, particular ABI capabilities, or specific runtime facilities.

§ 14(6) Such differences may restrict which programs or library facilities are available.

§ 14(7) Target differences must not silently redefine the meaning of core Sec constructs.

§ 14(8) Capability resolution and `CompilationPlan` behavior are owned by the platform rulebooks.

§ 14(9) The language should avoid requiring a general runtime whenever practical.

---

## § 15. Language, core, stdlib, platform, and compiler-known behavior

§ 15(1) A feature should not become compiler magic merely because implementing it as source code is inconvenient.

§ 15(2) Conversely, behavior fundamental to the semantic model need not be forced into an ordinary library abstraction when that would weaken static analysis, type identity, target integration, or runtime independence.

§ 15(3) The boundary between language semantics, compiler-known identities and operations, core, standard library, and platform library must be chosen according to semantic responsibility.

§ 15(4) Compiler-known behavior may connect trusted core declarations to language semantics without making the corresponding intrinsic part of the public source API.

§ 15(5) The programmer-facing model should remain coherent regardless of whether a feature is implemented by source, compiler-known support, target lowering, or a combination of them.

§ 15(6) Public behavior and private implementation hooks must not be conflated.

---

## § 16. Infer, do not guess

§ 16(1) The compiler should infer information aggressively when the result is unambiguous and safe.

§ 16(2) Inference must not become guessing.

§ 16(3) When several interpretations are semantically possible and the compiler cannot establish which one the programmer intended, Sec should require enough information to make the choice explicit.

§ 16(4) This principle applies particularly to ownership, borrowing, overload resolution, generic inference, conversions, FFI ownership, unsafe operations, and target-sensitive behavior.

§ 16(5) The language must not silently select a convenient interpretation when another valid interpretation would change program meaning.

---

## § 17. Readability and locality

§ 17(1) Code should normally be understandable from the visible code together with the contracts of the named abstractions it uses.

§ 17(2) Important behavior should not depend unnecessarily on distant declarations or hidden global state.

§ 17(3) Sec should favor local reasoning, explicit type identity, clear ownership behavior, visible fallibility, predictable name resolution, and limited hidden global state.

§ 17(4) This principle does not prohibit abstraction.

§ 17(5) Abstraction should preserve the programmer's ability to understand relevant semantics without reconstructing the entire program mentally.

---

## § 18. Consistency and composition

§ 18(1) Language features should compose.

§ 18(2) A new feature should use existing language concepts when those concepts already express the required semantics correctly.

§ 18(3) Special syntax or special semantic exceptions require a clear benefit.

§ 18(4) When several designs are technically possible, the preferred solution normally reduces cognitive load, improves readability and maintainability, increases useful compile-time verification, avoids semantic bureaucracy and hidden runtime costs, keeps generated behavior predictable, preserves deterministic semantics, composes with existing rules, and produces understandable diagnostics.

§ 18(5) A locally convenient feature should be rejected when it creates disproportionate complexity elsewhere in the language.

§ 18(6) Consistency is not an absolute requirement to preserve a bad abstraction. A deliberate special rule may be preferable when the underlying semantics are genuinely different and the distinction is explicit.

---

## § 19. Evolution

§ 19(1) Sec is expected to evolve.

§ 19(2) Language evolution should prefer extending a coherent semantic model over accumulating unrelated special cases.

§ 19(3) Existing decisions may be revised when implementation experience, language use, or later design work demonstrates a better solution.

§ 19(4) Revisions should be judged by the same design principles as new features.

§ 19(5) Compatibility with an earlier design is valuable, but preserving a known design mistake is not.

§ 19(6) A later normative decision supersedes an earlier conflicting provisional decision when the later decision deliberately revises that subject.

§ 19(7) Rulebooks must be kept synchronized so that historical design remnants do not become accidental competing semantics.

§ 19(8) Stable public contracts, diagnostic identities, paragraph references, and other deliberately versioned interfaces should not be changed casually merely because implementation work is ongoing.

---

## § 20. Backend independence

§ 20(1) LLVM, MLIR, Semantic IR, and other compiler technologies are implementation mechanisms.

§ 20(2) They do not define Sec source semantics.

§ 20(3) The compiler may change its internal representation, analysis pipeline, or backend without changing the meaning of valid Sec programs.

§ 20(4) A language feature must not be specified merely in terms of what a particular backend happens to support.

§ 20(5) Backend limitations may temporarily limit implementation coverage.

§ 20(6) Temporary implementation limitations must be distinguished from language-level restrictions.

§ 20(7) Semantic IR may be normative as a compiler correctness boundary where an owning compiler rulebook explicitly defines obligations on it; that still does not make a particular internal data structure part of Sec source syntax.

---

## § 21. Decision discipline

§ 21(1) A language-design discussion should distinguish between a genuinely new semantic choice, a conflict between existing rules, an implementation consequence of an already decided rule, and a temporary implementation limitation.

§ 21(2) Consequences that follow unambiguously from existing normative decisions should be applied rather than repeatedly reopened as design decisions.

§ 21(3) A new decision is required when:
- a new semantic area has not previously been specified;
- two existing normative rules conflict;
- an earlier rule produces an evidently invalid or incoherent consequence;
- several materially different semantics remain possible.

§ 21(4) Implementation details that do not change programmer-visible semantics do not, by themselves, require a language decision.

§ 21(5) This discipline prevents both accidental redesign and accidental invention by implementation work.

---

## § 22. Design test

§ 22(1) When evaluating a language proposal, ask:

```text
Does the programmer need to express this information, or can the compiler derive it safely?

Does the feature make correct code easier to write and understand?

Does it introduce semantic bureaucracy?

Does it require specialist terminology without corresponding value?

Is important behavior visible and predictable?

Can the compiler explain failures clearly?

Does it compose with ownership, error handling, generics, concurrency,
platform capabilities, and other existing rules?

Does it preserve deterministic semantics?

Does it work without assuming garbage collection or a mandatory general runtime?

Does it remain meaningful across hosted and freestanding targets?

Does it preserve the boundary between public semantics and private implementation?

Is the rule part of Sec because it serves Sec, rather than merely because
another language uses it?
```

§ 22(2) The preferred design is not necessarily the design with the fewest compiler rules.

§ 22(3) The preferred design gives the programmer the clearest useful model while allowing the compiler to carry as much mechanical complexity as practical.

---

## § 23. Normative summary

§ 23(1) The programmer expresses intent; the compiler proves and implements the consequences.

§ 23(2) Sec prefers practical simplicity over source-level ceremony and compiler convenience.

§ 23(3) Static proof is preferred; defined runtime validation is used when necessary; guarantees are not silently weakened.

§ 23(4) Unsafe transfers specific proof obligations. It does not disable ordinary language validation.

§ 23(5) Language semantics are deterministic except where another rulebook explicitly classifies behavior otherwise.

§ 23(6) Important semantic and performance consequences should not be hidden from the programmer.

§ 23(7) Compiler-internal complexity, regions, analyses, IR forms, and backend machinery are not automatically source-language concepts.

§ 23(8) The same core language semantics apply across targets; target capabilities restrict availability rather than silently redefining meaning.

§ 23(9) Inference is encouraged where safe and unambiguous. Guessing is not.

§ 23(10) New features should compose with existing concepts and should not create disproportionate special-case complexity.

§ 23(11) Later deliberate normative revisions may supersede earlier provisional designs, but stable public contracts should not be changed casually.

§ 23(12) Specific owning rulebooks remain authoritative for concrete language semantics. This philosophy rulebook guides design; it does not invent missing rules.
