# Stack Analysis

- **Status:** Normative
- **Created:** 2026-08-08
- **Last updated:** 2026-11-07 (stable chapter, subsection, and paragraph numbering)
- **Document revision:** 1
- **Sec language version:** 0.1
- **Canonical path:** `rules/analysis/stack_analysis.md`
- **Repository baseline reviewed:** `28780c01d63c3fc9eec2321ef67e3dd69962f043`
- **Implementation governance:** `governance/analysis.yaml` (`sema.stack-analysis`)
- **Related rulebooks:** `rules/analysis/call_graph.md`, `rules/analysis/closure_analysis.md`, `rules/memory/storage.md`, `rules/memory/lifetime_analysis.md`, `rules/memory/layout.md`, `rules/platform/ffi.md`, `rules/platform/interrupts.md`, `rules/compiler/incremental_compilation.md`, `rules/tooling/diagnostics.md`, `rules/tooling/lsp.md`

---

## § 1 Status

**§ 1(1)** Normative compiler-analysis rulebook for Sec 0.1.

**§ 1(2)** This rulebook defines the semantic and machine stack-analysis model, stack-bound
classification, per-function and per-root composition, recursion and SCC
handling, callable and foreign-call integration, frame construction, live-range
and stack-slot reuse requirements, stack-budget validation, tooling integration,
summary and incremental-analysis requirements, diagnostics, tests, and completion
criteria for Sec stack analysis.

**§ 1(3)** Mutable implementation status does not belong in this rulebook. It is governed
by the canonical `governance/analysis.yaml` ledger fragment.

**§ 1(4)** The canonical ledger integration identifier is:

```text
sema.stack-analysis
```

---

## § 2 Purpose

**§ 2(1)** Stack analysis determines how much stack resource an execution may require and
why that requirement arises.

**§ 2(2)** The analysis answers questions such as:

```text
How much automatic storage is simultaneously live in this function?
What is the maximum stack contribution of this call path?
Which recursive SCC determines the maximum stack depth?
Is a finite upper bound proven, unknown, or unbounded?
Which callable or foreign boundary prevents a finite proof?
How much stack is required from each execution root?
Does the verified requirement satisfy the active stack budget?
```

**§ 2(3)** Stack analysis is a resource analysis. It does not redefine storage placement,
object lifetime, borrowing, escape, ownership, layout, call reachability, task
semantics, ISR semantics, or backend ABI rules.

**§ 2(4)** Those semantics are consumed as canonical input facts.

---

## § 3 Normative role

**§ 3(1)** This rulebook is normative for:

- the distinction between semantic and machine stack requirements;
- stack-bound classification;
- per-function own-frame and transitive stack summaries;
- per-path stack composition;
- recursion and mutually recursive SCC handling;
- recursion-depth bounds;
- closed and open indirect-call target handling;
- callable, foreign, runtime, and platform stack contracts;
- execution roots and physical stack domains;
- frame construction from materialized automatic storage;
- live-range constrained stack-slot reuse;
- stack accounting for arrays, aggregates, descriptors, views, temporaries,
  return storage, cleanup, defer, and destruction;
- stack-budget validation;
- summary versioning and incremental invalidation;
- Interactive, Standard, and Deep stack-analysis behavior;
- LSP and `sec analyse` integration requirements;
- required test classes;
- completion criteria for Sec 0.1 stack analysis.

**§ 3(2)** This rulebook does not redefine:

- `StorageOrigin`, `BackingRelation`, or `ReclamationAuthority`;
- object or storage lifetime;
- ownership, copy, move, or borrow legality;
- escape validity;
- type or aggregate layout;
- function-value target discovery;
- call graph construction;
- source-level recursion policy;
- task runtime implementation;
- ISR restrictions;
- FFI declaration syntax or foreign metadata import;
- ABI calling convention rules;
- register allocation;
- backend optimization legality;
- project configuration syntax.

**§ 3(3)** Those concerns remain owned by their respective normative rulebooks and target
or tooling specifications.

---

## § 4 Core principle

**§ 4(1)** Stack analysis determines stack resource requirements without changing storage
semantics.

**§ 4(2)** In particular:

```text
Stack pressure
    does not authorize
implicit heap promotion
implicit arena promotion
implicit allocator-backed promotion
```

**§ 4(3)** If an automatic object is too large for an active stack budget, the compiler
must not silently repair the program by changing its storage origin.

**§ 4(4)** Any alternative storage strategy remains an explicit programmer or platform
choice governed by the storage and allocation rules.

---

## § 5 Stack analysis levels

**§ 5(1)** Sec distinguishes two stack-analysis levels.

### § 5.1 Semantic stack requirement

**§ 5.1(1)** The semantic stack requirement describes materialized stack storage required by
Sec semantics and canonical semantic lowering.

**§ 5.1(2)** It may include:

```text
automatic local storage
materialized semantic temporaries
semantically required return storage
cleanup or destruction state required by canonical lowering
alignment and padding introduced at the semantic frame level
```

**§ 5.1(3)** It does not attempt to predict backend register spills or other final
code-generation accidents.

### § 5.2 Machine stack requirement

**§ 5.2(1)** The machine stack requirement describes the final target-specific stack cost of
the compiled code.

**§ 5.2(2)** It may additionally include:

```text
ABI argument or return areas
saved registers
register spills
outgoing call area
target frame alignment
prologue or epilogue state
backend/runtime bookkeeping
```

**§ 5.2(3)** The relationship is conceptually:

```text
Semantic stack requirement
    + target/ABI/backend frame effects
    -> Machine stack requirement
```

**§ 5.2(4)** The two results are distinct compiler facts and must not overwrite one another.

**§ 5.2(5)** For example:

```text
Semantic upper bound: 8192 B
Machine exact maximum: 7344 B
```

**§ 5.2(6)** is a valid pair of results.

---

## § 6 Stack-bound classification

**§ 6(1)** A stack requirement must carry the quality of the result rather than only a
byte count.

**§ 6(2)** Conceptually:

```text
StackBound:
    Exact(bytes)
    UpperBound(bytes)
    Unknown
    Unbounded
```

### § 6.1 Exact

**§ 6.1(1)** `Exact(bytes)` means the analysis level knows the exact required amount for the
reported scope and CompilationPlan.

### § 6.2 UpperBound

**§ 6.2(1)** `UpperBound(bytes)` means the compiler has proven that the requirement cannot
exceed the reported value, although the real requirement may be smaller.

**§ 6.2(2)** A verified upper bound is sufficient for a stack safety or resource proof.

### § 6.3 Unknown

**§ 6.3(1)** `Unknown` means that a finite requirement may exist, but the compiler cannot
prove a finite upper bound from the currently available semantic facts,
contracts, summaries, or analysis precision.

**§ 6.3(2)** `Unknown` is not zero and is not evidence that the program will overflow the
stack.

### § 6.4 Unbounded

**§ 6.4(1)** `Unbounded` means the compiler has proven that the reachable execution structure
admits arbitrarily large stack depth or stack demand under the relevant model.

**§ 6.4(2)** `Unbounded` is stronger information than `Unknown` and must remain distinct.

---

## § 7 Partial information

**§ 7(1)** Loss of an overall upper bound must not erase independently known information.

**§ 7(2)** For example:

```text
known stack contribution before unknown call: 1536 B
overall stack maximum: Unknown
cause: callable target contract has no verified stack bound
```

**§ 7(3)** Deep analysis may additionally expose known minimum contributions.

**§ 7(4)** A known minimum contribution must never be used as an upper bound for a safety
or budget proof.

---

## § 8 CompilationPlan dependence

**§ 8(1)** Stack results that depend on size, alignment, ABI, runtime behavior, target
calling convention, or backend frame construction are resolved per active
`CompilationPlan`.

**§ 8(2)** The same Sec function may therefore have different stack requirements on
platforms such as:

```text
linux/amd64
bare-metal Cortex-M
riscv32
```

**§ 8(3)** without changing its source-language meaning.

**§ 8(4)** Machine stack summaries are necessarily CompilationPlan-specific.

**§ 8(5)** Semantic summaries may remain target-independent or symbolic only where their
inputs are target-independent.

---

## § 9 ResolvedLayout is authoritative

**§ 9(1)** Stack analysis must consume canonical resolved layout facts.

**§ 9(2)** It must not implement a parallel struct, array, union, pointer, descriptor, or
alignment model.

**§ 9(3)** Relevant inputs include:

```text
Size
Alignment
Stride where applicable
resolved descriptor representation
resolved aggregate representation
```

**§ 9(4)** When layout is not yet resolved, target-dependent byte results may remain
symbolic or unknown according to the analysis stage.

---

## § 10 Per-function stack summaries

**§ 10(1)** A function stack summary must distinguish at least:

```text
OwnFrame
TransitiveMaximum
BoundKind
MaximumCause
```

**§ 10(2)** The exact internal representation is implementation-defined.

### § 10.1 Own frame

**§ 10.1(1)** `OwnFrame` is the maximum stack storage attributable to the function's own
materialized frame at the relevant analysis level.

### § 10.2 Transitive maximum

**§ 10.2(1)** `TransitiveMaximum` includes the function's own frame together with the maximum
simultaneously live callee contribution on any reachable execution path.

### § 10.3 Maximum cause

**§ 10.3(1)** The summary should preserve enough information to explain the call path, SCC,
unknown boundary, or frame contributor responsible for the result.

---

## § 11 Call-path composition

**§ 11(1)** Sequential calls do not automatically sum their complete transitive maxima.

**§ 11(2)** For example:

```text
A own frame = 100 B
B maximum   = 500 B
C maximum   = 800 B
```

**§ 11(3)** for:

```sec
fn A() void {
    B()
    C()
}
```

**§ 11(4)** produces conceptually:

```text
100 + max(500, 800) = 900 B
```

**§ 11(5)** when the two calls cannot be simultaneously active.

**§ 11(6)** Nested calls do sum simultaneously live frames.

**§ 11(7)** For:

```text
A -> B -> C
```

**§ 11(8)** the active call chain may require:

```text
Own(A) + Own(B) + Own(C)
```

**§ 11(9)** plus applicable machine-level overhead at the machine analysis level.

---

## § 12 Control-flow composition

**§ 12(1)** Stack analysis operates on reachable execution paths.

**§ 12(2)** For mutually exclusive branches such as:

```sec
if condition {
    LargePathA()
} else {
    LargePathB()
}
```

**§ 12(3)** the call contribution is the maximum of the two reachable paths, not their sum.

**§ 12(4)** The same principle applies to:

```text
if
match
switch
select
error paths
cleanup paths
panic paths where applicable
```

**§ 12(5)** Proven unreachable paths do not contribute to a stack bound.

---

## § 13 Loops and stack depth

**§ 13(1)** Ordinary loop iteration does not multiply frame depth.

**§ 13(2)** For example:

```sec
for item in values {
    Process(item)
}
```

**§ 13(3)** does not imply:

```text
iteration count * Stack(Process)
```

**§ 13(4)** because each iteration normally reuses the same active call structure.

**§ 13(5)** Loop analysis may still affect frame live ranges and cleanup overlap.

**§ 13(6)** Recursion is different because each recursive call may create another
simultaneously live frame.

---

## § 14 Execution roots

**§ 14(1)** Whole-program stack analysis is measured from execution roots rather than only
from the program's ordinary main entry.

**§ 14(2)** Conceptual execution roots include:

```text
ProgramEntry
ThreadEntry
TaskExecutionContext
ISR
PlatformEntry
UnknownEntry
```

**§ 14(3)** The exact root model is determined by canonical call graph, runtime, platform,
and CompilationPlan semantics.

**§ 14(4)** Reachability and maximum stack requirement are root-specific.

**§ 14(5)** A function reachable from the ordinary program entry but not from an ISR root
does not contribute to that ISR root's stack requirement.

---

## § 15 Physical stack domains

**§ 15(1)** Logical execution roots and physical stacks are separate concepts.

**§ 15(2)** Two threads with separate stacks are analyzed as separate stack domains.

**§ 15(3)** Their per-stack maxima do not become one artificial combined depth.

**§ 15(4)** Whole-program resource reporting may separately report aggregate reserved stack
memory across several threads or tasks.

**§ 15(5)** If several logical roots may execute on the same physical stack, the active
runtime and CompilationPlan must define possible nesting or reentry, and stack
analysis must account for those relations.

**§ 15(6)** Task stack behavior follows the actual Sec runtime/platform model. Stack
analysis must not assume that every task has a dedicated native stack.

**§ 15(7)** Possible implementations may include dedicated stacks, shared execution stacks,
stackless state machines, or platform-defined execution contexts.

**§ 15(8)** This rulebook does not choose among them.

---

## § 16 Interrupt nesting

**§ 16(1)** Stack analysis must be capable of composing nested execution contributions when
the platform model permits them.

**§ 16(2)** Conceptually:

```text
thread frame chain
+ interrupt frame
+ nested interrupt frame
```

**§ 16(3)** may share one physical stack domain.

**§ 16(4)** `rules/platform/interrupts.md` determines the canonical nesting relationships;
`rules/analysis/isr_analysis.md` validates each root against its profile. Stack
analysis consumes those facts and computes their resource contribution.

---

## § 17 Call graph ownership

**§ 17(1)** Stack analysis consumes the canonical call graph defined by `call_graph.md`.

**§ 17(2)** It must not create a parallel call-reachability model.

**§ 17(3)** Relevant call-graph facts include:

```text
known targets
closed or open target sets
callable contracts
execution roots
SCCs
reentry relationships where represented
```

---

## § 18 Acyclic call graphs

**§ 18(1)** For an acyclic reachable call graph, stack maxima can be composed bottom-up.

**§ 18(2)** Conceptually:

```text
Maximum(F) = OwnFrame(F) + maximum reachable callee contribution at each call path
```

**§ 18(3)** A leaf function has no callee contribution, but it may still have a non-zero
own frame.

**§ 18(4)** A machine-level leaf function may have an exact zero frame when no stack storage,
ABI area, saved state, spill, or alignment requirement exists.

**§ 18(5)** Zero own-frame size does not imply zero transitive stack usage.

---

## § 19 Recursion and SCCs

**§ 19(1)** Recursion is identified from call-graph strongly connected components.

**§ 19(2)** A multi-node SCC may represent mutual recursion.

**§ 19(3)** A singleton SCC is recursive only when it contains a self-edge.

**§ 19(4)** Recursion can arise through:

```text
direct calls
mutual direct calls
function-value calls
callbacks
foreign reentry
runtime hooks
```

**§ 19(5)** Therefore stack analysis must operate on the resolved semantic call graph rather
than source syntax alone.

---

## § 20 Recursion-depth classification

**§ 20(1)** Recursion depth is a separate fact from byte stack requirement.

**§ 20(2)** Conceptually:

```text
RecursionDepthBound:
    ExactDepth(n)
    UpperBoundDepth(n)
    UnknownDepth
    UnboundedDepth
```

**§ 20(3)** A recursive call does not automatically imply `UnboundedDepth`.

---

## § 21 Finite recursion and finite global bounds

**§ 21(1)** A recursive function may terminate for every concrete invocation without having
a finite compile-time maximum over all permitted root inputs.

**§ 21(2)** For example:

```sec
fn Countdown(value: uint) void {
    if value == 0 {
        return
    }

    Countdown(value - 1)
}
```

**§ 21(3)** has finite depth for each concrete `value`.

**§ 21(4)** However, a global finite upper bound requires a finite proven upper bound on the
initial recursive measure.

**§ 21(5)** Therefore:

```text
decreasing measure
    + finite proven initial upper bound
    -> possible finite recursion-depth proof
```

**§ 21(6)** A decreasing argument alone does not prove a finite whole-program maximum.

---

## § 22 Sources of finite recursion bounds

**§ 22(1)** Stack analysis may consume canonical facts such as:

```text
constant call arguments
range-constrained parameters
constrained types
verified collection extents
verified finite state transitions
root contracts
other canonical range or control-flow facts
```

**§ 22(2)** to establish a finite recursion-depth bound.

**§ 22(3)** Stack analysis should reuse these facts rather than build an unrelated range or
termination analysis.

**§ 22(4)** The required Sec 0.1 baseline need not implement a general-purpose termination
prover.

**§ 22(5)** Simple bounded recursive measures are sufficient for the baseline.

**§ 22(6)** Deep analysis may later add more advanced techniques such as size-change
reasoning, multi-variable ranking functions, or path-sensitive recursive measure
tracking.

---

## § 23 Unknown recursion depth

**§ 23(1)** A recursive structure whose concrete depth may be finite but whose maximum
cannot be proven is `UnknownDepth`.

**§ 23(2)** For example, recursively walking a linked structure with no statically bounded
maximum chain length is normally unknown rather than proven unbounded.

**§ 23(3)** The distinction is normative.

---

## § 24 Proven unbounded recursion

**§ 24(1)** `UnboundedDepth` requires a proof that arbitrarily large recursive depth is
reachable under the relevant execution model.

**§ 24(2)** Simple examples include reachable unconditional recursive cycles with no finite
limiting state transition.

**§ 24(3)** For example:

```sec
fn Again() void {
    Again()
}
```

**§ 24(4)** may yield a proven unbounded stack requirement when the call is reachable and
no guaranteed constant-stack recursion rule applies.

**§ 24(5)** Mutual recursion may be proven unbounded by the same principle.

---

## § 25 Recursive SCC stack contribution

**§ 25(1)** A recursive SCC must account for every simultaneously live frame along a
recursive cycle.

**§ 25(2)** For example:

```text
A -> B -> C -> A
```

**§ 25(3)** with own frames:

```text
A = 100 B
B = 200 B
C = 300 B
```

**§ 25(4)** has a cycle contribution involving all three frames, not merely the largest
single frame.

**§ 25(5)** Different cycles within one SCC may have different stack contributions.

**§ 25(6)** A sound bound must account for the maximum relevant recursive cycle or use an
alternative sound abstraction that proves an equal or larger upper bound.

---

## § 26 Recursive progress must cover relevant transitions

**§ 26(1)** A recursion-depth proof must apply to all recursive transitions relevant to the
claimed bound.

**§ 26(2)** If one cycle is known to decrease a bounded measure but another possible cycle
in the same SCC has no corresponding progress proof, the bounded measure from
the first cycle must not be unsoundly applied to the second.

**§ 26(3)** The result is `UnknownDepth` or `UnboundedDepth` according to what can actually
be proven.

---

## § 27 Tail recursion

**§ 27(1)** Source-level tail position does not by itself prove constant stack usage.

**§ 27(2)** Optional tail-call elimination is a backend optimization and must not be used as
a source-level resource proof.

**§ 27(3)** If Sec or a specific CompilationPlan later guarantees constant-stack tail calls
for a class of calls, stack analysis may consume that guarantee.

**§ 27(4)** Final machine stack summaries may naturally reflect tail calls that were
actually and validly applied by the backend.

---

## § 28 Inlining

**§ 28(1)** Inlining may alter final machine frame requirements by changing call overhead,
materialization, slot reuse, register pressure, and spills.

**§ 28(2)** Semantic stack proofs must not depend on optional inlining.

**§ 28(3)** Final machine stack analysis reflects the actual compiled frame structure.

---

## § 29 Closed indirect-call target sets

**§ 29(1)** For a reachable indirect call whose canonical callable analysis provides a
closed target set:

```text
KnownTargets = {A, B, C}
IsClosed = true
```

**§ 29(2)** the stack contribution of the call is the maximum requirement among all
possible targets, accounting for the caller's own frame and path context.

**§ 29(3)** Closed target sets therefore permit finite stack composition when all targets
have finite compatible summaries.

---

## § 30 Open callable contracts

**§ 30(1)** Known targets do not close an otherwise open target set.

**§ 30(2)** For example:

```text
KnownTargets = {A, B}
OpenContract = X
```

**§ 30(3)** requires the open contract to contribute a sound bound for every additional
target permitted by that contract.

**§ 30(4)** An open callable target set can provide a finite stack bound only when its
callable contract supplies a verified stack bound sufficient for all permitted
targets.

**§ 30(5)** The exact source or metadata syntax of such a contract is outside this
rulebook.

---

## § 31 Open calls without stack contracts

**§ 31(1)** A reachable open indirect call whose contract provides no usable stack bound
contributes `Unknown` to the transitive maximum.

**§ 31(2)** Known target contributions should still be preserved for explanation where
useful.

**§ 31(3)** The compiler must not assume that an unknown callback consumes zero stack.

---

## § 32 Reentry through callable and foreign boundaries

**§ 32(1)** Possible reentry participates in recursion analysis.

**§ 32(2)** For example:

```text
A
 -> ForeignCall
 -> Callback
 -> A
```

**§ 32(3)** is stack-recursive/reentrant when the foreign or callable contracts permit this
path.

**§ 32(4)** A verified `NoReentry`-equivalent contract may exclude such a recursive edge.

**§ 32(5)** Unknown reentry behavior may force the affected stack result to `Unknown`.

**§ 32(6)** This rulebook defines consumption of such facts, not their source syntax.

---

## § 33 Foreign, runtime, and platform calls

**§ 33(1)** Foreign or runtime code executing on the same physical stack may contribute to
the current stack requirement.

**§ 33(2)** A verified foreign/runtime/platform stack contract may provide a finite
CompilationPlan-specific contribution.

**§ 33(3)** If no such contract or body-level summary is available, the reachable foreign
contribution is `Unknown` rather than zero.

**§ 33(4)** Compiler-known platform functions may provide canonical stack summaries when
that knowledge is explicit and trustworthy.

**§ 33(5)** FFI tooling and metadata import are outside this rulebook.

---

## § 34 Separate compilation

**§ 34(1)** Separate compilation must be able to preserve compatible stack summaries so
callers can compose stack requirements without the callee body.

**§ 34(2)** Conceptually, imported information may include:

```text
semantic own-frame information
semantic transitive bound
machine own-frame information for a CompilationPlan
machine transitive bound for a CompilationPlan
bound kind
unknown/unbounded cause metadata
```

**§ 34(3)** Machine summaries must not be reused for an incompatible CompilationPlan, ABI,
summary schema, or compiler semantic model.

**§ 34(4)** Missing compatible summary information produces localized `Unknown` where the
information is required.

---

## § 35 Generic and symbolic summaries

**§ 35(1)** Stack analysis need not duplicate a complete analysis for every generic
instantiation when a sound symbolic semantic summary can be preserved.

**§ 35(2)** Conceptual examples include:

```text
OwnFrame(T) <= T.SizeOf + K
Maximum(F<N>) <= N * K + C
```

**§ 35(3)** Such representations are internal analysis facts, not source syntax.

**§ 35(4)** When types, constants, specialization choices, and CompilationPlan are resolved,
the symbolic facts may be instantiated.

**§ 35(5)** Machine summaries may still differ per specialization when lowering produces
different final machine behavior.

---

## § 36 Frame construction

**§ 36(1)** A function's semantic own frame is based on maximum simultaneously live
materialized automatic storage rather than the sum of all declarations that
appear anywhere in the function.

**§ 36(2)** Conceptually:

```text
SemanticOwnFrame =
    maximum simultaneously live materialized automatic storage
    + semantic frame metadata
    + semantic alignment/padding
```

**§ 36(3)** at the semantic analysis level.

---

## § 37 Lexical scope and live range

**§ 37(1)** Source lexical scope is not itself the authoritative stack-storage live range.

**§ 37(2)** A storage object may cease to require materialized stack storage before the end
of a broad lexical block when all canonical lifetime, destruction, cleanup,
address, and escape dependencies are finished.

**§ 37(3)** Conversely, `defer`, destruction, borrowing, retained addresses, or cleanup paths
may keep storage live beyond its last ordinary source read.

**§ 37(4)** Stack analysis consumes canonical lifetime and storage facts to establish these
ranges.

---

## § 38 Stack-slot reuse

**§ 38(1)** Two materialized stack objects may share physical stack space only when the
compiler proves that their required live storage cannot overlap.

**§ 38(2)** Conceptually:

```text
CanReuse(A, B)
    only if
NoSimultaneousLiveStorage(A, B)
    and
size/alignment constraints are satisfied
```

**§ 38(3)** When overlap is unknown and an upper-bound proof is required, the conservative
result is no reuse.

---

## § 39 Mutually exclusive branches

**§ 39(1)** Storage required exclusively by mutually exclusive control-flow branches may
share stack space when the compiler proves the branch live ranges cannot
overlap.

**§ 39(2)** For example:

```sec
if condition {
    let first: LargeA
    UseA(first)
} else {
    let second: LargeB
    UseB(second)
}
```

**§ 39(3)** may require approximately the larger branch-local storage rather than the sum of
both branch-local objects, subject to all surrounding live storage, alignment,
and cleanup constraints.

---

## § 40 Sequential local reuse

**§ 40(1)** Sequential locals within one branch may reuse storage when the earlier object's
materialized lifetime has definitely ended before the later object's storage is
needed.

**§ 40(2)** Reuse must respect:

```text
borrows
references
raw or safe address validity
escape facts
destruction
cleanup
defer
partial move state
pinning
stable-address requirements
```

**§ 40(3)** No stack-specific shortcut may invalidate these semantics.

---

## § 41 Alignment and padding

**§ 41(1)** Required alignment and padding are real stack costs.

**§ 41(2)** Stack analysis consumes resolved object alignment and applies frame-level
alignment requirements at the level where they are introduced.

**§ 41(3)** Semantic layout padding belongs to semantic frame accounting when required by
canonical semantic frame layout.

**§ 41(4)** ABI/backend padding belongs to machine frame accounting.

---

## § 42 Parameters

**§ 42(1)** Source parameters do not automatically consume local semantic stack storage.

**§ 42(2)** Their physical placement may be in registers, incoming stack areas, or indirect
ABI locations.

**§ 42(3)** If Sec semantics require a parameter value to have addressable storage, semantic
analysis may record that requirement.

**§ 42(4)** The backend may then materialize an actual machine stack slot when required by
the target ABI and lowering.

**§ 42(5)** Source-level by-value semantics remain by-value even when the ABI passes the
physical representation indirectly.

**§ 42(6)** ABI indirection does not turn a source value parameter into a borrow.

---

## § 43 Address-taken values

**§ 43(1)** Taking the address of a local or parameter does not by itself require heap or
arena storage.

**§ 43(2)** A stack object remains valid when lifetime and escape analysis prove that every
reference or pointer dependency is valid for the stack object's storage
lifetime.

**§ 43(3)** Addressability may constrain register-only lowering and stack-slot reuse.

---

## § 44 Return storage

**§ 44(1)** Return-value semantics and physical return storage are distinct.

**§ 44(2)** Depending on semantic lowering and ABI, a result may use:

```text
register return
caller-provided return storage
hidden return pointer
callee temporary
direct construction into final destination
```

**§ 44(3)** Stack accounting must count the storage where it is actually materialized and
must not double-count one caller-owned result slot as both caller and callee
storage when no second buffer exists.

---

## § 45 Temporaries

**§ 45(1)** Only temporaries that require materialized stack storage contribute to stack
usage.

**§ 45(2)** Pure SSA values or values kept entirely in registers do not consume semantic
stack bytes merely because a conceptual temporary exists.

**§ 45(3)** Materialized aggregate temporaries may contribute substantially even when there
is no named local variable.

**§ 45(4)** Semantically guaranteed direct construction or copy elision may remove a
semantic temporary.

**§ 45(5)** Optional backend optimization may improve only the machine-level result unless
its behavior is part of canonical semantic lowering.

---

## § 46 Destruction and cleanup

**§ 46(1)** The last ordinary read of a value does not necessarily end its stack-storage
requirement.

**§ 46(2)** When destruction requires the materialized object representation, storage must
remain available until the relevant destruction action is complete.

**§ 46(3)** Stack analysis therefore distinguishes:

```text
last ordinary use
object lifetime end
destruction
storage reuse/reclamation point
```

**§ 46(4)** according to the canonical lifetime, destruction, and storage rules.

---

## § 47 Defer

**§ 47(1)** A deferred action may extend the stack-storage dependency of values it uses.

**§ 47(2)** For example:

```sec
fn Work() void {
    let resource := Open()

    defer {
        Close(resource)
    }

    HeavyWork()
}
```

**§ 47(3)** may require `resource` to remain materially available through cleanup.

**§ 47(4)** Stack analysis consumes canonical defer-use and capture/dependency facts rather
than implementing an independent defer-capture model.

---

## § 48 Error and panic paths

**§ 48(1)** Reachable error, cleanup, destruction, and panic paths contribute to maximum
stack demand when their semantics require additional active frames or
simultaneously live storage.

**§ 48(2)** A cleanup path may therefore determine the maximum even when the ordinary
success path is cheaper.

**§ 48(3)** Panic strategy may be CompilationPlan-dependent where runtime behavior differs.

**§ 48(4)** Stack analysis must not assume that panic cleanup is free.

---

## § 49 Fixed arrays

**§ 49(1)** A fixed array embedded in automatic storage contributes through its resolved
inline size and alignment.

**§ 49(2)** For example:

```sec
let buffer: byte[4096]
```

**§ 49(3)** is not a dynamic descriptor and must not be silently modeled as separate
allocator-backed storage.

---

## § 50 Embedded aggregates

**§ 50(1)** Embedded fields contribute through the resolved size of the containing object.

**§ 50(2)** For example, if an automatic struct contains a fixed array, stack analysis
counts the containing struct's resolved size rather than double-counting the
field as a separate independent stack object.

**§ 50(3)** Tagged unions contribute according to their resolved union representation, not
the sum of all variants.

---

## § 51 Dynamic owning collections and views

**§ 51(1)** For a dynamic owning collection whose backing storage is separate from its
stack-resident value representation, stack analysis counts the materialized
owning descriptor or handle but not the separately allocated backing storage.

**§ 51(2)** Likewise, a borrowed view contributes its stack-resident reference/descriptor
representation while its backing storage is accounted for at the storage origin
where that backing actually resides.

**§ 51(3)** Therefore:

```text
runtime collection length
    !=
runtime stack frame size
```

**§ 51(4)** when the backing storage is separate.

---

## § 52 Arena and allocator-backed storage

**§ 52(1)** Arena-backed and allocator-backed payload bytes are not stack bytes merely
because a handle, descriptor, or reference to them is stored in an automatic
local.

**§ 52(2)** Stack analysis counts only the stack-resident representation.

**§ 52(3)** Arena and allocator resource accounting remains owned by the corresponding
storage/allocation analysis and runtime model.

---

## § 53 Runtime-sized automatic storage

**§ 53(1)** Sec 0.1 stack analysis must not invent runtime-sized automatic stack allocation
where no such source/storage feature exists.

**§ 53(2)** If a future Sec feature explicitly permits runtime-sized automatic storage, its
storage semantics must represent that choice explicitly.

**§ 53(3)** A finite stack proof for such storage requires a finite proven upper bound on
its runtime size expression.

---

## § 54 Partial moves

**§ 54(1)** A partial move does not automatically make the containing stack storage reusable.

**§ 54(2)** Remaining live fields, destruction responsibility, cleanup state, and ownership
metadata may still require the containing representation or parts of it to
remain live.

**§ 54(3)** Stack analysis consumes the canonical ownership and destruction state rather
than inventing a stack-specific partial-move model.

---

## § 55 Pinning and stable addresses

**§ 55(1)** Pinning temporarily constrains storage movement and reuse.

**§ 55(2)** Pinning does not change storage origin or automatically extend storage to the
end of the function.

**§ 55(3)** When the pin and all other dependencies end, reuse may again become possible.

**§ 55(4)** Likewise, a stable-address requirement constrains slot reuse while the address
may still be semantically observed.

**§ 55(5)** Address stability, reference lifetime, and generation/invalidation facts are
canonical inputs to stack reuse decisions where applicable.

---

## § 56 Generational-pointer representation

**§ 56(1)** Any stack bytes required by the eventual canonical generational-pointer
representation are determined by resolved layout and lowering.

**§ 56(2)** Stack analysis counts that representation where it is materialized but does not
define the representation itself.

---

## § 57 Compiler bookkeeping

**§ 57(1)** Compiler bookkeeping may contribute to a frame.

**§ 57(2)** Conceptual examples include:

```text
cleanup flags
destruction state
panic/unwind state
generation or validation tokens
```

**§ 57(3)** Bookkeeping mandated by canonical semantic lowering belongs to semantic frame
analysis.

**§ 57(4)** Bookkeeping introduced only by a particular backend, ABI, or runtime belongs to
machine frame analysis.

---

## § 58 Semantic and machine frame authority

**§ 58(1)** Semantic stack analysis provides the authoritative semantic frame proof.

**§ 58(2)** After backend lowering and frame layout are known, machine stack analysis
provides the authoritative machine-level requirement.

**§ 58(3)** For example:

```text
SemanticUpperBound = 4608 B
MachineExactFrame  = 4384 B
```

**§ 58(4)** is not a contradiction.

**§ 58(5)** The values answer different questions.

---

## § 59 Stack budgets

**§ 59(1)** Required stack and available stack are separate concepts.

**§ 59(2)** Stack analysis first determines a requirement.

**§ 59(3)** An independent target, project, thread, task, ISR, platform, or build contract
may provide an available stack budget.

**§ 59(4)** Validation then compares the appropriate verified requirement with that budget.

**§ 59(5)** Conceptually:

```text
StackBudget {
    Domain
    MeasurementLevel
    MaximumBytes
}
```

**§ 59(6)** where the measurement level distinguishes at least semantic and machine stack
budgets.

**§ 59(7)** This representation is conceptual and does not define configuration syntax.

---

## § 60 Budget validation

**§ 60(1)** A finite verified bound satisfies a finite stack budget only when the compiler
can prove:

```text
RequiredStack <= AvailableStack
```

**§ 60(2)** at the budget's specified measurement level and physical stack domain.

**§ 60(3)** If the budget requires a finite proof and the stack result is `Unknown`, the
build fails because the required proof could not be established.

**§ 60(4)** This is distinct from proving that runtime overflow necessarily occurs.

**§ 60(5)** If the result is `Unbounded` and the budget is finite, the budget cannot be
satisfied.

---

## § 61 Machine-level revalidation

**§ 61(1)** A hard machine-stack budget must be revalidated after final backend frame
construction.

**§ 61(2)** A semantic estimate below the machine budget is not sufficient when later ABI,
spill, or frame-layout effects increase the final machine requirement above the
budget.

**§ 61(3)** Conversely, if the applicable contract explicitly concerns final machine stack,
a conservative semantic upper bound above the budget does not by itself reject
the build when the final verified machine requirement satisfies the budget.

**§ 61(4)** The budget must identify which measurement level is authoritative.

---

## § 62 No-budget behavior

**§ 62(1)** When no active contract requires a finite stack proof, `Unknown` may remain a
valid analysis result rather than automatically invalidating the program.

**§ 62(2)** It remains useful for:

```text
sec analyse
LSP optional insight
resource reports
ISR analysis
platform tooling
```

**§ 62(3)** A target or project may impose stronger requirements.

**§ 62(4)** Stack analysis reports facts; resource policy determines whether those facts
are acceptable.

---

## § 63 Large-frame advisories

**§ 63(1)** A large automatic frame may be worth reporting even when no hard budget is
violated.

**§ 63(2)** Such a diagnostic is advisory or optional insight, not a language validity rule.

**§ 63(3)** No normative byte threshold is defined here.

**§ 63(4)** Recommendation policy may consider the active CompilationPlan and configured
resource guidance.

**§ 63(5)** The compiler should explain the major automatic-storage contributor rather than
prescribe hidden heap or arena promotion.

**§ 63(6)** A suitable advisory concept is:

```text
`buffer` contributes 1 MiB of automatic storage to this frame.
Consider whether this storage should use an explicitly selected non-stack
storage strategy.
```

**§ 63(7)** The storage strategy remains explicit.

---

## § 64 Stack cause paths

**§ 64(1)** Stack analysis should preserve a cause path sufficient to explain a maximum,
unknown result, unbounded result, or budget failure.

**§ 64(2)** For example:

```text
main
  -> Parse
  -> Decode
  -> Inflate
  -> BuildTable
```

**§ 64(3)** with representative frame contributions.

**§ 64(4)** Recursive reports should identify the responsible SCC, relevant depth proof,
and recursive contribution.

**§ 64(5)** Unknown reports should identify the boundary that prevented a finite proof.

**§ 64(6)** Representative paths must be selected deterministically when several equivalent
maximum paths exist.

---

## § 65 Reusable stack proofs

**§ 65(1)** A verified per-root stack proof should be reusable by downstream compiler
consumers.

**§ 65(2)** Conceptually:

```text
StackProof {
    Root
    StackDomain
    Bound
    BoundKind
    MeasurementLevel
    CompilationPlan
    Dependencies
}
```

**§ 65(3)** The exact internal structure is implementation-defined.

**§ 65(4)** Consumers must preserve the meaning of `Exact`, `UpperBound`, `Unknown`, and
`Unbounded`.

**§ 65(5)** `Unknown` must never be treated as zero.

---

## § 66 Consumers

**§ 66(1)** Canonical stack facts may be consumed by:

```text
build validation
ISR analysis
LSP
sec analyse
resource reports
embedded/platform tooling
optimization and cost diagnostics
```

**§ 66(2)** The ISR analysis must reuse stack-analysis facts rather than implement a second
stack calculator.

---

## § 67 Summary model

**§ 67(1)** Stable function summaries should expose semantic stack information independently
from final machine stack information.

**§ 67(2)** Conceptually:

```text
FunctionStackSummary {
    SemanticOwnFrame
    SemanticMaximum
    BoundKind
    RecursiveState
    Dependencies
    MaximumCause
}
```

**§ 67(3)** and, when backend information is available:

```text
MachineStackSummary {
    MachineOwnFrame
    MachineMaximum
    BoundKind
    CompilationPlan
    Dependencies
    MaximumCause
}
```

**§ 67(4)** These names are explanatory rather than mandatory implementation identifiers.

---

## § 68 Summary versioning

**§ 68(1)** Persisted stack summaries must be versioned and validated before reuse.

**§ 68(2)** Compatibility may depend on:

```text
function identity
function signature
summary schema version
compiler semantic-analysis version
CompilationPlan
ABI
generic specialization where relevant
```

**§ 68(3)** An incompatible or stale summary must not be used as a verified stack proof.

---

## § 69 Summary invalidation

**§ 69(1)** Stack summaries must be invalidated when relevant inputs change.

**§ 69(2)** Relevant dependencies include:

```text
function body
callee stack summary
call graph target set
recursion/SCC structure
ResolvedLayout
lifetime/storage facts affecting live ranges or reuse
defer/destruction behavior
generic specialization
foreign/runtime/platform stack contract
CompilationPlan
ABI/backend frame information
analysis schema or semantic version
```

**§ 69(3)** Semantic and machine cache dependencies may differ and may therefore be
invalidated independently where safe.

---

## § 70 Incremental propagation

**§ 70(1)** Incremental analysis should propagate changed stack summaries through dependent
callers and execution roots.

**§ 70(2)** If a changed function recomputes to an equivalent stack summary, further
propagation may stop.

**§ 70(3)** An unrelated function edit should not force complete project-wide stack
reanalysis when dependency information proves that cached results remain valid.

**§ 70(4)** Changes to callable target sets or reentry contracts may invalidate stack
results even when the source text of the call site did not change.

---

## § 71 Analysis state

**§ 71(1)** Interactive tooling must distinguish incomplete analysis from a proven stack
result.

**§ 71(2)** Conceptually, tooling state may distinguish:

```text
Pending
Partial
Complete
```

**§ 71(3)** or an equivalent model.

**§ 71(4)** A pending recursive, indirect, or foreign contribution must not be rendered as
zero or as a verified finite maximum.

---

## § 72 Analysis budgets

**§ 72(1)** Stack analysis follows the common Sec analysis-budget model.

### § 72.1 Interactive

**§ 72.1(1)** Interactive analysis prioritizes low-latency, incremental results such as:

```text
local frame facts
cached direct-call summaries
cheap closed-target composition
available recursion summaries
```

**§ 72.1(2)** Interactive analysis may report `Pending` while stronger proof is still being
computed.

**§ 72.1(3)** It must remain sound.

### § 72.2 Standard

**§ 72.2(1)** Standard analysis performs every stack proof required for source validity or the
active `CompilationPlan` and resource contracts.

**§ 72.2(2)** If a build requires a finite root stack bound, Standard analysis must perform
the necessary SCC, indirect-target, imported-summary, and budget proof.

**§ 72.2(3)** Required proof must not be deferred to Deep.

**§ 72.2(4)** Standard results are deterministic.

### § 72.3 Deep

**§ 72.3(1)** Deep analysis may add greater precision and richer explanation, including:

```text
all execution-root reports
alternative maximum paths
frame-contributor breakdowns
stack-slot reuse explanations
more expensive recursion reasoning
symbolic or context-sensitive refinement
cross-module resource maps
suppressed or blocked precision explanations
```

**§ 72.3(2)** Deep analysis improves precision and information without changing source
semantics.

**§ 72.3(3)** If a specific build contract requires a refinement normally associated with
Deep, that refinement becomes required for that build rather than remaining
optional.

---

## § 73 LSP presentation

**§ 73(1)** LSP stack presentation distinguishes information the programmer needs from
optional resource insight.

### § 73.1 NeedToKnow

**§ 73.1(1)** Examples include:

```text
required finite stack proof failed
hard stack budget exceeded
proven unbounded stack requirement violates active policy
```

### § 73.2 OptionalInsight

**§ 73.2(1)** Examples include:

```text
local frame size
transitive maximum
largest contributor
maximum call path
recursion depth source
stack-slot reuse explanation
```

**§ 73.2(2)** Optional stack insight is configurable.

**§ 73.2(3)** A normal hover should remain compact.

**§ 73.2(4)** A possible presentation is:

```text
Stack: <= 624 B local, <= 4.8 KiB transitive
```

**§ 73.2(5)** when such insight is enabled and proven.

**§ 73.2(6)** Detailed or Deep views may expose the full cause path and contributor breakdown.

---

## § 74 LSP configuration changes

**§ 74(1)** Persistent LSP analysis must respond to relevant project configuration changes
without requiring an LSP restart.

**§ 74(2)** When stack-analysis depth, optional insight, stack budgets, or related analysis
settings change, the LSP must:

```text
reload the affected configuration
invalidate affected stack summaries or presentation caches
recompute required stack analysis
refresh dependent diagnostics, hovers, code lenses, or analysis views
```

**§ 74(3)** Unchanged semantic summaries may be reused when only presentation policy changes.

---

## § 75 sec analyse

**§ 75(1)** `sec analyse` includes stack analysis in the default all-analysis behavior.

**§ 75(2)** The tooling must also permit stack analysis to be selected explicitly:
`sec analyse stack` (`rules/tooling/cli.md` § 5.2, MD-017).

**§ 75(3)** Deep stack analysis should be able to report:

```text
per-function semantic frames
per-function machine frames when available
per-root maximum stack
physical stack domains
recursive SCCs and depth bounds
unknown and unbounded causes
indirect/callable stack contracts
foreign/runtime boundaries
largest frame contributors
maximum call paths
stack-budget comparisons
semantic versus machine results
```

---

## § 76 Diagnostics

**§ 76(1)** Stack diagnostics must distinguish different proof outcomes.

### § 76.1 Proven budget excess

**§ 76.1(1)** When a verified requirement exceeds the applicable budget, the diagnostic should
state both values and the measurement level.

**§ 76.1(2)** Conceptually:

```text
error: machine stack budget exceeded for `Worker`

required: 9216 B
available: 8192 B
```

### § 76.2 Failure to prove a required bound

**§ 76.2(1)** When the active contract requires a finite proof but the result is `Unknown`, the
diagnostic must state that proof failed rather than claim that overflow is
certain.

**§ 76.2(2)** Conceptually:

```text
error: finite stack bound could not be proven for `Worker`

required by target:
    stack <= 8192 B

cause:
    callback target has no verified stack bound
```

### § 76.3 Proven unbounded stack

**§ 76.3(1)** A proven unbounded recursive or reentrant structure should be reported as
unbounded rather than merely unknown.

**§ 76.3(2)** Conceptually:

```text
error: stack requirement is unbounded

recursive cycle:
    Parse -> Visit -> Parse

no finite recursion-depth bound exists for this reachable cycle
```

---

## § 77 Diagnostic ownership and severity

**§ 77(1)** Stack analysis owns stack-resource facts and stack-budget proof diagnostics.

**§ 77(2)** A stack finding must not duplicate an existing lifetime, escape, ownership,
layout, or call-graph error when that upstream error already makes the program
invalid.

**§ 77(3)** Optional large-frame or resource-design findings remain advisories unless an
active resource contract promotes them through the normal diagnostic policy.

**§ 77(4)** No separate stack-specific strict mode is introduced by this rulebook.

---

## § 78 Determinism

**§ 78(1)** Stack analysis must be deterministic with respect to semantic inputs.

**§ 78(2)** Results must not depend on:

```text
source declaration order
hash-map iteration order
worklist order
parallel scheduling
cache population order
```

**§ 78(3)** When several cause paths produce the same bound, the summary bound remains the
same and any representative diagnostic path is chosen deterministically.

---

## § 79 Fixed-point and widening requirements

**§ 79(1)** Recursive and symbolic stack analysis must terminate.

**§ 79(2)** When precision must be reduced, widening must remain sound.

**§ 79(3)** Conceptually, analysis may lose precision in the direction:

```text
Exact -> UpperBound -> Unknown
```

**§ 79(4)** It must never replace unresolved or unknown stack demand with a guessed finite
bound.

**§ 79(5)** `Unbounded` is used only when unboundedness is proven.

---

## § 80 Required Sec 0.1 baseline

**§ 80(1)** The Sec 0.1 stack-analysis implementation must support at least:

```text
automatic local frame accounting
fixed arrays and embedded aggregates
descriptors and separate backing storage
alignment and padding
materialized temporaries
return storage
destruction and defer live-range extension
error and cleanup paths
sound stack-slot reuse
direct acyclic call composition
direct recursion detection
mutual recursion via SCCs
simple finite recursion bounds from canonical range/contracts
Unknown versus Unbounded recursion
closed indirect target sets
open callable contracts
reentry uncertainty
foreign/runtime stack summaries
execution roots and physical stack domains
semantic versus machine stack distinction
hard stack-budget validation
localized unknown causes
incremental summaries
Interactive, Standard, and Deep integration
```

**§ 80(2)** Advanced general termination proving, sophisticated symbolic optimization, and
prediction of backend register allocation are not required for Sec 0.1.

---

## § 81 Required frame tests

**§ 81(1)** Tests must cover at least:

```text
zero-frame function
single automatic local
multiple simultaneously live locals
sequential non-overlapping locals
mutually exclusive branch locals
fixed arrays
embedded aggregates
tagged-union storage
descriptor versus backing storage
alignment and padding
address-taken local
address-taken parameter
materialized aggregate temporary
return storage
defer-extended storage lifetime
destruction and cleanup overlap
```

---

## § 82 Required call-path tests

**§ 82(1)** Tests must cover at least:

```text
leaf function
single direct call
nested direct calls
sequential calls use maximum rather than sum
branch calls use maximum
proven unreachable call is ignored
error-path maximum
cleanup-path maximum
```

---

## § 83 Required recursion tests

**§ 83(1)** Tests must cover at least:

```text
direct unconditional recursion -> Unbounded
mutual unconditional recursion -> Unbounded
bounded recursive integer parameter
decreasing parameter without finite root upper bound -> Unknown
bounded mutual recursion
recursive SCC with several cycles
recursive SCC with insufficient progress proof -> Unknown
unreachable recursive SCC does not affect the root
```

**§ 83(2)** The distinction between `Unknown` and `Unbounded` must be explicitly tested.

---

## § 84 Required indirect-call tests

**§ 84(1)** Tests must cover at least:

```text
closed single target
closed multiple targets
largest target selected
known targets plus open contract
open contract with finite stack bound
open contract without stack bound -> Unknown
indirect recursion
verified no-reentry contract excludes recursive path
unknown reentry produces conservative Unknown where required
```

---

## § 85 Required foreign/runtime tests

**§ 85(1)** Tests must cover at least:

```text
known foreign stack contract
unknown foreign stack contribution
compiler-known runtime stack summary
foreign reentry
callback into the caller SCC
CompilationPlan-specific foreign stack summary
```

**§ 85(2)** Foreign metadata import itself is tested by the FFI/tooling subsystem rather
than this rulebook.

---

## § 86 Required storage-accounting tests

**§ 86(1)** Tests must verify the distinction between inline automatic storage and separate
backing storage.

**§ 86(2)** At minimum:

```text
fixed byte[4096] automatic local -> inline stack bytes
dynamic byte[] descriptor with arena backing -> descriptor only
borrowed view -> view representation only
allocator-backed object handle -> handle only
embedded fixed array -> counted through enclosing object
```

---

## § 87 Required stack-slot reuse regression tests

**§ 87(1)** Tests must ensure reuse is rejected while any relevant dependency remains live.

**§ 87(2)** At minimum:

```text
borrow remains live
defer uses earlier local
destructor requires earlier object
safe or raw address remains valid
stable-address requirement remains active
partial move leaves aggregate state live
cleanup path references earlier local
unknown overlap under required upper-bound proof
```

---

## § 88 Required CompilationPlan tests

**§ 88(1)** Tests must verify that different representative target layouts and ABIs may
produce different machine stack summaries without changing source semantics.

**§ 88(2)** Relevant differences include:

```text
pointer width
alignment
calling convention
saved-register requirements
outgoing-call area
```

**§ 88(3)** The exact CI targets are implementation policy rather than normative language
syntax.

---

## § 89 Required budget tests

**§ 89(1)** Tests must cover at least:

```text
Exact below budget -> accepted
UpperBound below budget -> accepted
Exact above budget -> rejected
finite verified upper bound proving excess -> rejected
Unknown with mandatory proof -> failure-to-prove rejection
Unbounded with finite budget -> rejected
Unknown with no applicable finite-proof requirement -> retained analysis result
```

**§ 89(2)** Tests must also verify that semantic and machine budgets are compared against the
correct measurement level.

---

## § 90 Required incremental and LSP tests

**§ 90(1)** Tests must cover at least:

```text
change local size -> own-frame refresh
change callee frame -> dependent caller maximum refresh
change unrelated function -> unaffected cache reuse
add or remove call -> call-path refresh
change callable target set -> stack result refresh
add recursion -> SCC recomputed
remove recursion -> finite result restored
change stack budget -> diagnostics refreshed
change analysis depth -> optional insight refreshed
project configuration reload requires no LSP restart
```

---

## § 91 Required separate-compilation tests

**§ 91(1)** Tests must cover at least:

```text
compatible imported semantic summary
compatible imported machine summary
wrong CompilationPlan rejected
incompatible schema rejected
missing summary produces localized Unknown
body-derived and imported compatible summaries produce equivalent composition
```

---

## § 92 Required performance and termination tests

**§ 92(1)** Stack analysis must be tested on representative large inputs including:

```text
large acyclic call graphs
large SCCs
many execution roots
many closed indirect target sets
large functions with many live ranges
incremental leaf edits
```

**§ 92(2)** The tests must establish bounded analysis termination and deterministic results.

**§ 92(3)** This rulebook does not define a normative millisecond threshold.

---

## § 93 Completion criteria

**§ 93(1)** Stack analysis is complete for Sec 0.1 when all of the following are true:

1. semantic own frames can be modeled from materialized automatic storage;
2. stack-slot reuse is constrained by canonical lifetime, escape, destruction,
   cleanup, address, pinning, and storage facts;
3. `ResolvedLayout` is consumed as the authoritative size/alignment source;
4. inline fixed storage is distinguished from descriptors and separate backing;
5. direct call-path stack composition is sound;
6. recursive and mutually recursive SCCs are detected;
7. `Unknown` and `Unbounded` recursion are distinct;
8. simple finite recursion-depth bounds can be proven from canonical range or
   contract facts;
9. closed indirect target sets and open callable stack contracts are consumed;
10. foreign/runtime/platform stack contracts are consumed without assuming
    unknown calls cost zero;
11. possible reentry participates in recursion analysis;
12. execution roots and physical stack domains are represented;
13. semantic and machine stack requirements remain distinct;
14. final machine stack can be revalidated against machine-level budgets;
15. hard stack budgets correctly distinguish proven excess from failure to
    prove;
16. stack summaries are versioned and suitable for separate compilation;
17. dependency-driven incremental invalidation is implemented;
18. LSP and `sec analyse` consume the same canonical stack facts;
19. Interactive, Standard, and Deep analysis follow the common budget model;
20. the required frame, call-path, recursion, indirect, FFI/runtime, storage,
    budget, separate-compilation, incremental, and performance tests pass.

---

## § 94 Normative summary

**§ 94(1)** Stack analysis determines stack resource requirements without owning storage,
lifetime, escape, layout, or call-reachability semantics.

**§ 94(2)** Stack pressure never authorizes implicit promotion to heap, arena, or other
non-stack storage.

**§ 94(3)** Semantic and machine stack requirements are distinct compiler facts.

**§ 94(4)** Stack bounds distinguish exact, finite upper-bound, unknown, and proven
unbounded results.

**§ 94(5)** Per-function stack composition uses maximum simultaneously live storage and
maximum reachable call-path demand rather than naïvely summing all declarations
or sequential calls.

**§ 94(6)** Recursive and mutually recursive calls are analyzed through canonical call-graph
SCCs. Finite recursion depth requires a sound bound on every relevant recursive
transition and a finite bound on the initial recursive measure.

**§ 94(7)** Closed indirect-call target sets compose the maximum of all permitted targets.
Open target sets require a verified callable stack contract for a finite proof.
Unknown callable, foreign, runtime, or reentry behavior remains explicitly
unknown rather than being treated as zero.

**§ 94(8)** Stack-frame construction consumes resolved layout and canonical live-range,
ownership, lifetime, destruction, defer, escape, pinning, address, and storage
facts. Stack-slot reuse requires proof that materialized storage cannot overlap.

**§ 94(9)** Fixed arrays and embedded aggregates contribute inline storage. Dynamic owning
collections, views, arena-backed objects, and allocator-backed objects contribute
only their stack-resident descriptors or handles when their backing storage is
separate.

**§ 94(10)** Stack requirements are computed per execution root and physical stack domain
according to the active runtime and CompilationPlan.

**§ 94(11)** Required stack and available stack budget are separate concepts. A finite budget
is satisfied only by an appropriate verified finite bound at the budget's
specified semantic or machine measurement level.

**§ 94(12)** A hard budget failure caused by `Unknown` is a failure to prove the required
resource contract, not proof that overflow must occur. `Unbounded` remains a
separate proven result.

**§ 94(13)** Stack summaries are reusable, versioned, incrementally invalidated compiler
facts consumed by build validation, ISR analysis, LSP, `sec analyse`, resource
reporting, and platform tooling.

**§ 94(14)** Interactive analysis may expose pending or partial results but must remain sound.
Standard performs every stack proof required by the active build. Deep may add
more expensive precision and explanation without changing source semantics.
