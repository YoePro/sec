# Language Philosophy

## Purpose

Sec is designed to be simple to read, simple to write and predictable to reason about.

The language is intended to support both ordinary application development and low-level systems programming without requiring the programmer to adopt unnecessary complexity for either.

Complexity should exist where it provides value.

When complexity can be handled reliably by the compiler, it should normally remain in the compiler rather than being exposed as source-level ceremony.

The programmer should express intent.

The compiler should prove the consequences.


## Independent design

Sec is influenced by many existing programming languages and programming traditions.

These include but is not limited to:

* C
* C++
* C#
* F#
* Go
* Rust
* Zig
* Ada
* Vale
* Odin

Sec is not designed as a derivative or replacement syntax for any one of them.

A concept may be adopted when it serves Sec's goals.

A concept may be modified when Sec can make it simpler, safer or more consistent.

A concept may be rejected even when it is established practice in another language.

Similar syntax does not imply identical semantics.

Familiarity is useful, but consistency within Sec has higher priority than compatibility with the expectations created by another language.

Language decisions must therefore be justified by the needs and design principles of Sec itself, not by statements such as:

C does this
Rust requires this
Go does not allow this
C# calls this ...
F# represents this ...

Other languages are valuable sources of experience.

They are not normative sources for Sec.


## Practical simplicity

Sec should be understandable by competent programmers without requiring specialist academic knowledge.

A programmer should not need a PhD in programming-language theory to understand ordinary Sec code, compiler diagnostics or the language manual.

Technical concepts are not avoided merely because they are sophisticated.

Unnecessary intellectual or terminological complexity is avoided.

When a concept can be accurately described using ordinary programming terminology, Sec should prefer that terminology over specialist terminology that provides no practical benefit to the programmer.

The compiler implementation may use advanced algorithms, formal models and specialized terminology internally.

Those implementation details should not leak into the source language unless the programmer needs them to make a meaningful programming decision.

The goal is not to make the compiler simple.

The goal is to make correct programming comprehensible.


## Avoid semantic bureaucracy

Sec should not require syntax whose only purpose is to repeat information the compiler can already determine safely.

Explicit syntax is valuable when it expresses a real semantic choice.

Examples include:

* mutability
* fallible operations
* unsafe operations
* borrowing mode
* ownership-sensitive operations where intent cannot otherwise be determined
* external storage or hardware semantics
* units and contracts that express domain meaning

Explicit syntax is not valuable merely because it makes compiler implementation easier.

The language should avoid semantic bureaucracy: annotations, qualifiers, declarations or ceremony that force the programmer to maintain information already known or provable by the compiler.

The preferred rule is:

explicit intent
implicit bookkeeping

The programmer describes what is meant.

The compiler performs the bookkeeping needed to prove that it is valid.


## Compiler responsibility

The compiler is expected to perform substantial analysis.

This includes, but is not limited to:

* type analysis
* ownership analysis
* borrowing analysis
* lifetime and reference validation
* escape analysis
* effect analysis
* control-flow analysis
* definite-assignment analysis
* copy and move analysis
* destruction and cleanup planning
* stack analysis
* recursion analysis
* ISR analysis
* concurrency analysis
* compile-time evaluation
* contract validation
* unit analysis
* platform validation
* ABI and FFI validation
* optimization legality analysis

These analyses are compiler responsibilities.

They should influence source syntax only where the programmer must provide information that cannot be derived safely or where an explicit choice is semantically important.

The existence of sophisticated compiler analysis is not justification for sophisticated source syntax.


## Diagnostics are part of the language experience

Rejecting an incorrect program is not sufficient.

The compiler should explain why the program is incorrect whenever practical.

Diagnostics should help the programmer understand:

* what rule was violated
* where the relevant values or declarations originated
* why the compiler reached its conclusion
* what operation caused the conflict
* what change may resolve the problem when a useful suggestion exists

Compiler analysis should therefore be designed not only to answer:

is this program valid?

but also:

how can the compiler explain the result to the programmer?

Advanced analysis that cannot provide useful diagnostics should be treated with caution when a simpler and more understandable model can provide comparable safety.


## Static proof and checked semantics

Sec prefers static proof.

When the compiler can prove that an operation is valid, no runtime validation should be emitted merely for defensive purposes.

When a language safety rule cannot be proven statically, Sec may use compiler-defined runtime validation where the language semantics require the condition to be checked.

Examples may include:

* dynamic bounds validation
* runtime contract validation
* arithmetic checks
* reference generation validation
* other safety conditions whose truth depends on runtime values

Runtime validation does not imply garbage collection or a mandatory general-purpose runtime.

A check may lower directly to ordinary machine instructions or target-specific support.

The general principle is:

prove when possible
check when necessary
reject when neither can provide the required guarantee

The compiler must not silently weaken a language guarantee merely because proving it statically is difficult.


## Safety without removing low-level control

Sec is intended to support low-level programming.

This includes areas such as:

* operating-system interfaces
* FFI
* memory-mapped hardware
* embedded systems
* bare-metal systems
* allocators
* device drivers
* platform runtimes
* systems software

Low-level capability must not require the entire language to adopt unsafe semantics.

Safe code should retain the strongest guarantees the compiler can provide.

Operations whose correctness cannot generally be verified must cross an explicit unsafe boundary or use another explicitly defined low-level mechanism.

Unsafe code does not disable the language.

Inside unsafe code, ordinary rules for types, ownership, control flow, visibility, error handling and other validatable semantics continue to apply.

Unsafe means that a specific operation contains assumptions the compiler cannot prove.

It does not mean that the compiler stops checking the program.


## Determinism

Sec favors deterministic semantics.

When behavior can affect program correctness, resource lifetime or observable execution, the language should define it rather than leave it accidentally dependent on the compiler backend.

This includes areas such as:

* ownership transfer
* copy and move behavior
* destruction
* cleanup
* defer execution
* expression evaluation order
* control-flow behavior
* initialization
* error propagation

Optimization may remove, combine or rearrange operations only when the observable Sec semantics remain unchanged.

Backend convenience must not define source-language behavior.


## No hidden semantic surprises

Source code should communicate operations that have important semantic or performance consequences.

Sec should avoid hidden behavior such as:

* unexpected heap allocation
* unexpected ownership transfer
* hidden garbage collection
* hidden reference counting
* implicit expensive copying
* implicit resource acquisition
* implicit exception mechanisms
* unexpected dynamic dispatch
* backend-dependent safety behavior

This does not mean every machine instruction must be visible in source code.

The compiler is expected to generate substantial implementation machinery.

The distinction is between hidden implementation and hidden semantics.

Compiler-generated implementation is desirable when it faithfully implements clear source semantics.

Compiler-generated semantic surprises are not.


## Cost awareness

Sec should make it possible for programmers to reason about important costs.

The language should not require every low-level cost to be written explicitly, but operations with substantially different ownership, allocation or dispatch behavior should not be made indistinguishable when that difference matters.

Zero-cost abstractions are desirable when practical.

However, the language must not sacrifice correctness or comprehensibility merely to satisfy a slogan about zero cost.

A predictable and explicit cost is preferable to an invisible and surprising one.


## Ownership and memory

Sec uses deterministic ownership and compile-time analysis as central tools for memory and resource safety.

The ownership model should remain understandable without requiring programmers to manually describe compiler-internal lifetime relationships.

The programmer should not normally need explicit lifetime annotations.

Borrowing and reference rules should prevent invalid programs while avoiding unnecessary source-level bookkeeping.

When the compiler cannot prove that an ownership, borrowing or lifetime relationship is safe, Sec should prefer a clear diagnostic over introducing increasingly complex annotation systems merely to make the program accepted.

Compiler-internal concepts such as regions, data-flow states and lifetime models are implementation techniques.

They are not automatically language concepts.


## Abstraction without loss of control

High-level abstractions and low-level control are not opposing goals.

A language feature may provide a high-level source representation while lowering to direct and predictable machine behavior.

Examples include:

* register fields instead of manual masks and shifts
* units instead of unchecked numeric conventions
* typed Result values instead of hidden error channels
* properties instead of manually repeated validation logic
* safe references instead of ordinary raw pointers
* compiler-generated cleanup instead of manually repeated resource release

An abstraction is useful when it reduces programmer error without hiding semantically important behavior.

Sec should therefore avoid equating low-level programming with low-level syntax.


## One language across targets

Sec targets both hosted and freestanding environments.

These include:

* desktop and server operating systems
* embedded operating systems
* microcontrollers
* bare-metal targets

The core language semantics should remain the same across targets.

Target profiles may differ in available facilities.

For example, a target may lack:

* heap allocation
* threads
* operating-system services
* particular ABI capabilities
* specific runtime support

Such differences may restrict which programs or library facilities are available.

They must not silently redefine the meaning of core Sec language constructs.

The language should avoid requiring a runtime whenever practical.


## Libraries and language

A feature should not become compiler magic merely because implementing it as ordinary source code is inconvenient.

Conversely, behavior that is fundamental to the semantic model does not need to be forced into an ordinary library abstraction when doing so would weaken analysis or require artificial runtime machinery.

The boundary between:

* language semantics
* compiler-known operations
* core library
* standard library
* platform library

should be chosen according to semantic responsibility.

The programmer-facing model should remain coherent regardless of where a feature is implemented.


## Reject ambiguity rather than disguise it

The compiler should infer information aggressively when the result is unambiguous and safe.

Inference must not become guessing.

When several interpretations are semantically possible and the compiler cannot establish which one the programmer intended, Sec should require enough information to make the choice explicit.

This principle applies particularly to areas such as:

* ownership
* borrowing
* overload resolution
* generic inference
* conversions
* FFI ownership
* unsafe operations

The language should not silently choose a convenient interpretation that may change program meaning.


## Readability and locality

Code should normally be understandable from the code that is visible.

Important behavior should not depend unnecessarily on declarations far away from the operation being read.

Sec should favor:

* local reasoning
* explicit type identity
* clear ownership behavior
* visible fallibility
* predictable name resolution
* limited hidden global state

This principle does not prohibit abstraction.

It means abstraction should preserve the programmer's ability to understand the relevant semantics without reconstructing the entire program mentally.


## Consistency over special cases

Language features should compose.

A new feature should use existing language concepts when those concepts already express the required semantics correctly.

Special syntax or special semantic exceptions should require a clear benefit.

When several designs are technically possible, the preferred solution normally:

* reduces cognitive load
* improves readability
* improves maintainability
* increases useful compile-time verification
* avoids semantic bureaucracy
* avoids hidden runtime costs
* keeps generated behavior predictable
* preserves deterministic semantics
* composes with existing language rules
* produces understandable diagnostics

A locally convenient feature should be rejected when it creates disproportionate complexity elsewhere in the language.


## Evolution

Sec is expected to evolve.

Language evolution should prefer extending a coherent semantic model over accumulating unrelated special cases.

Existing decisions may be revised when experience demonstrates a better solution.

Such changes should be judged by the same design principles as new features.

Compatibility with an earlier design is valuable, but preserving a known design mistake is not.

Newer design decisions normally supersede older provisional decisions when they represent a deliberate refinement of the language model.

The rulebooks must be kept synchronized so that historical design remnants do not become accidental competing semantics.


## Backend independence

LLVM, MLIR, Semantic IR and other compiler technologies are implementation mechanisms.

They do not define Sec source semantics.

The compiler may change its internal representation, analysis pipeline or backend without changing the meaning of valid Sec programs.

A language feature must not be specified merely in terms of what a particular backend happens to support.

Backend limitations may temporarily limit implementation coverage.

They must be distinguished from language-level restrictions.


## Design test

When evaluating a language proposal, the following questions should be asked:

* Does the programmer need to express this information, or can the compiler derive it safely?
* Does the feature make correct code easier to write and understand?
* Does it introduce semantic bureaucracy?
* Does it require specialist terminology without providing corresponding value?
* Is important behavior visible and predictable?
* Can the compiler explain failures clearly?
* Does it compose with ownership, error handling, generics and other existing rules?
* Does it preserve deterministic semantics?
* Does it work without assuming a garbage collector or mandatory runtime?
* Does it remain meaningful on both hosted and freestanding targets?
* Is the rule part of Sec because it serves Sec, or merely because another language does it?

The preferred design is not necessarily the design with the fewest compiler rules.

It is the design that gives the programmer the clearest useful model while allowing the compiler to carry as much mechanical complexity as practical.
