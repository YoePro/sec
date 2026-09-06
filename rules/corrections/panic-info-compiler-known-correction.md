# PanicInfo compiler-known type correction

- **Status:** Pending normative correction
- **Created:** 2026-09-06
- **Last updated:** 2026-09-06
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `0f5027d`
- **Intended path:** `rules/corrections/panic-info-compiler-known-correction.md`
- **Primary authority to amend:** `rules/errors/panic.md`
- **Dependent rulebook:** `rules/concurrency/processes.md`

---

## 1. Purpose

`rules/errors/panic.md` revision 2.0 currently describes `PanicInfo` only conceptually and explicitly leaves its exact public ABI/layout/name unfixed.

That is no longer sufficient for Sec 0.1.

`PanicInfo` is referenced by canonical concurrency outcomes, including the process surface:

```text
Process[T].Panic    PanicInfo
```

The compiler, core library, runtime/lowering layers, tests, LSP, and dependent rulebooks therefore require one exact source-visible compiler-known panic information type rather than a conceptual placeholder.

This correction makes `PanicID` and `PanicInfo` concrete Sec 0.1 compiler-known/core types while preserving the panic rulebook as their sole canonical semantic owner.

---

## 2. Canonical `PanicID`

### 2.1 Exact declaration

Sec 0.1 must provide the following compiler-known nominal type:

```sec
type PanicID uint32
```

`PanicID` is:

- immutable when stored in ordinary immutable bindings;
- copyable;
- equality-comparable;
- target-independent in public representation;
- not a native signal number, OS exception code, source address, or process/thread identifier;
- the stable identity of a canonical panic reason in the panic/diagnostic reason registry.

The fixed `uint32` representation is part of the Sec 0.1 public type contract. The diagnostics/panic registry remains the canonical owner of which numeric value is assigned to each stable panic reason.

Ordinary source code must not infer panic semantics from an unregistered numeric value. Compiler/runtime creation of `PanicID` values must use the canonical registry.

### 2.2 Registry ownership

The existing rule that exact reason registry spelling and numeric assignments belong to the diagnostics/panic registry remains in force.

This correction fixes the **storage/type representation** of the ID, not the registry's assignment policy.

The canonical registry must provide stable entries for at least the panic categories already required by `panic.md`:

```text
arithmetic overflow
division by zero
invalid shift
bounds failure
contract failure
assertion failure
checked unreachable reached
invalid reference generation
explicit panic
foreign abort/trusted-boundary failure
```

No compiler/runtime implementation may substitute transient source-location hashes, native signal numbers, or backend-specific trap IDs for the canonical `PanicID` registry value.

---

## 3. Canonical `PanicInfo`

### 3.1 Exact declaration

Sec 0.1 must provide this compiler-known/core type:

```sec
type PanicInfo struct {
    ID: PanicID,
    File: string,
    Line: uint,
    Column: uint,
    Function: string,
}
```

These five fields are the complete portable Sec 0.1 `PanicInfo` surface.

A conforming implementation must not omit, rename, dynamically retag, or replace these fields with an implementation-private shape while still exposing the value as `PanicInfo`.

### 3.2 Field meanings

`ID`:

```text
PanicID
```

is the stable canonical panic reason identifier.

`File`:

```text
string
```

is the compiler-known source file identity/name selected by the diagnostics/source-location policy for the panic point.

`Line`:

```text
uint
```

is the source line number associated with the panic point according to Sec source-location rules.

`Column`:

```text
uint
```

is the source column associated with the panic point according to Sec source-location rules.

`Function`:

```text
string
```

is the compiler-known canonical diagnostic name of the Sec function/method/execution entry associated with the panic point.

### 3.3 Immutability and copyability

`PanicInfo` is a bounded immutable information value once constructed for observation.

Its Sec 0.1 fields are copyable, so `PanicInfo` itself is copyable unless a later canonical language-wide rule changes the copyability of one of those field types.

Copying `PanicInfo` does not duplicate panic-domain ownership, task/thread/process lifecycle ownership, or a native exception object. It copies only structured panic metadata.

---

## 4. Allocation-free minimum path

The existing panic requirement that the minimum panic path require no dynamic allocation remains normative.

Therefore construction of the canonical `PanicInfo` value must not require:

```text
heap allocation
dynamic string concatenation
growing collections
filesystem lookup
network access
runtime symbolization
```

`File` and `Function` must be materializable from compiler-emitted/static metadata, constant tables, or another allocation-free representation that still presents the exact source-level `string` fields above.

The source-visible `string` type does not authorize the panic path to allocate a new dynamic string merely to populate these fields.

`Line`, `Column`, and `ID` are direct bounded values.

---

## 5. Optional richer panic reporting

Optional profile-specific panic reporting may still include information such as:

```text
message
operation
type identity
task/thread/process identity
source expression
call stack
related panic
native trap information
```

Such information is **not** part of the portable Sec 0.1 `PanicInfo` struct unless a later normative revision explicitly adds fields.

A richer panic sink, diagnostic record, platform structure, debugger interface, or logging payload must not silently mutate the canonical `PanicInfo` declaration.

---

## 6. Concurrency and process use

### 6.1 One canonical panic information type

Tasks, Threads, Processes, supervisors, test harnesses, and other managed containment boundaries must use this canonical `PanicInfo` when they expose structured panic observation.

The process rulebook must not introduce `ProcessPanicInfo` or another competing process-only panic payload.

### 6.2 Process panic observation

For a Sec callable subprocess, `ProcessStatus.Panicked` is valid only when a complete canonical `PanicInfo` has been established for the owner process.

After successful join of a panicked `Process[T]`:

```text
Process[T].Panic    PanicInfo
```

is available according to `processes.md`.

If the child panic occurs but the process completion/transport mechanism cannot establish the complete canonical `PanicInfo` at the owner boundary, the owner must not receive a fabricated or partially initialized `PanicInfo`.

The process is then classified according to the abnormal-termination rules in `processes.md` rather than reporting `ProcessStatus.Panicked` with invented metadata.

### 6.3 Panic metadata transport is runtime control metadata

Transport of `PanicInfo` across a managed process containment boundary is part of Sec's compiler/runtime process-control protocol.

It is not ordinary user-requested serialization of an arbitrary `PanicInfo` value and does not weaken the general `ProcessTransferable` rules for user values.

The transport must preserve the canonical `PanicID` and source metadata meanings defined here.

---

## 7. Required changes to `rules/errors/panic.md`

When applying this correction:

1. increment the panic rulebook from document revision `2.0` to `2.1`;
2. update `Last updated` to `2026-09-06` or the actual later application date;
3. replace the conceptual `PanicInfo` example in §13 with the exact declaration in §3.1 of this correction;
4. remove the statement that the exact public ABI/layout/name of `PanicInfo` is unfixed;
5. add the exact `PanicID` declaration from §2.1;
6. retain diagnostics/panic-registry ownership of stable reason spelling and numeric assignments;
7. retain the allocation-free minimum-path requirements and explicitly bind them to construction of the exact `PanicInfo` fields;
8. state that all compiler-known panic-related types named by the rulebook must have a complete canonical declaration rather than conceptual-only names;
9. add `rules/concurrency/processes.md` to the concurrency containment references where process panic observation is described.

---

## 8. Compiler/core implementation obligations

Sec 0.1 must provide actual compiler-known/core implementations for:

```sec
type PanicID uint32

type PanicInfo struct {
    ID: PanicID,
    File: string,
    Line: uint,
    Column: uint,
    Function: string,
}
```

The compiler/runtime must also provide:

- canonical registry lookup/creation of `PanicID` values;
- allocation-free creation of minimum `PanicInfo` values;
- source-file, line, column, and canonical function metadata generation;
- preservation of the same `PanicInfo` type through task/thread/process containment boundaries;
- process panic transport sufficient for the `Process[T].Panic` contract;
- diagnostics when a compiler-known panic reason lacks a registered `PanicID`;
- tests that prevent target-specific or backend-specific reason IDs from leaking into the portable type.

---

## 9. Implementation-status reconciliation

The mutable implementation ledger must not mark the panic-information correction implemented until both exact core types and the required registry/source-metadata plumbing exist.

The process integration may depend on canonical `PanicInfo`, but it must keep process panic observation incomplete until the panic correction is implemented.

No process implementation may work around the dependency by adding a private process-only panic information type.

---

## 10. Required tests

At minimum:

```text
PanicID has the exact public type PanicID over uint32
PanicInfo has exactly ID, File, Line, Column, and Function with the declared types
minimum PanicInfo creation requires no heap allocation
stable panic reason produces the same canonical PanicID independent of target backend
PanicInfo is copyable metadata and does not carry lifecycle ownership
managed task/thread containment uses the same PanicInfo type
Process[T].Panic is available only after joined Panicked status
process panic transport preserves PanicID and source metadata
failed process panic-metadata establishment never fabricates partial PanicInfo
optional richer reporting does not alter the portable PanicInfo declaration
```

---

## 11. Completion condition

This correction is complete when `rules/errors/panic.md` itself contains the exact compiler-known/core declarations above, the diagnostics/panic registry supplies stable `PanicID` assignments, the allocation-free implementation path exists, dependent concurrency books refer to the canonical type, and no normative text still describes `PanicInfo` as merely conceptual or ABI-unfixed.
