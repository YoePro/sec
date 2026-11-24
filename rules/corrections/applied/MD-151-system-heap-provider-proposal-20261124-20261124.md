# MD-151 — Verified malloc/free allocation provider

- Status: **Proposal for owner decision; not accepted or normative.**
- Owner decision: **accepted as written on 2026-11-24** (revision 1); integrated 2026-11-24 into `target_profiles.md` § 41A, `allocation.md` §§ 7(6), 10(7), 17(11), 28A(4), `arena.md` §§ 12(10), 13(7), 32(4), 35(5) and `projects.md` § 19(13). The status line above records the proposal as written.
- Date: 2026-11-24
- Revision: 1
- Decision: MD-151; dependent work includes MD-039 and MD-139.
- Intended owning rules: `rules/platform/target_profiles.md` § 41,
  `rules/memory/allocation.md` §§ 7, 10, 17, 28A,
  `rules/memory/arena.md` §§ 12–13, 32, 35, 38,
  and `rules/projects/projects.md` allocation selection rules.

The owner authorized preparation of this proposal, not adoption of its choices.
Nothing here declares an allocator implemented or changes a current rulebook.
On acceptance, integrate the decisions into the owning sections, synchronize
their existing governance owners, and archive this correction under the normal
correction workflow. Do not count this proposal as completed implementation.

## 1. Decision requested

Adopt `sec.system-heap.v1`, a compiler-managed provider backed by a qualified
target C runtime's `malloc` and `free`, as the sole initial eligible default on
qualified Hosted targets. Support over-alignment with a checked allocation
adapter; use the same provider for ambient owned storage and owned Arena chunks.
Declare no default provider for RTOS or BareMetal. Replace the provisional
profile-string mapping with verified catalog selection when this is implemented.

This proposes all seven choices listed in MD-151. It does not define a string
descriptor ABI, a new source allocator API, or new manifest keys.

## 2. Identity, catalog and defaults

**2(1)** `sec.system-heap.v1` identifies the provider contract, not one heap
instance or one universal C ABI. Each catalog record binds it to exactly one
target identity, ABI, C-runtime identity/version compatibility range and
toolchain binding. The frozen ProviderIdentity includes that record's immutable
identity and digest. Different runtimes or incompatible bindings do not share
reclamation authority merely because their contract spelling agrees.

**2(2)** Initial eligible target families are Hosted Linux, Darwin and Windows
on amd64 and arm64. Eligibility is not evidence of support. A target/runtime
combination gets a positive catalog record only after qualification in § 8.
An absent or unqualified record stays Unknown; an explicitly incompatible
combination is Unsupported. There is no automatic libc choice from an OS name.

**2(3)** Catalog records are maintained, versioned compiler/toolchain target
metadata, alongside the target registry. They are not arbitrary project TOML
declarations. The initial implementation uses typed records with deterministic
canonical serialization for fingerprints; no external catalog file format is
introduced by this decision. Project `provider` selection references the catalog
identity through the existing MD-139 syntax and cannot create or override it.

**2(4)** A qualified Hosted record explicitly nominates this provider as its
default. Zero defaults remain unresolved; multiple eligible defaults are an
error. Family selection, allocation mode and activation precedence remain
target_profiles.md § 41. Selecting a provider never enables a disabled baseline
or overrides `noalloc`.

**2(5)** RTOS, QNX, BareMetal and `embedded-arena` have no new default under this
proposal. QNX is not qualified by being POSIX-like. No static-buffer provider,
FreeRTOS/Zephyr binding or manifest capacity key is introduced. Explicit
caller-supplied Arena backing retains its existing rules. Additional provider
contracts can be decided separately without changing this provider's identity.

## 3. Required frozen facts and implementation binding

Each positive record supplies the following facts in addition to target and
runtime compatibility, with provenance and a qualification-version digest:

| Fact | Proposed value or obligation |
| --- | --- |
| Context kind | CompilerManaged |
| Capabilities | Owned byte storage, individual reclamation, fresh nonrelocating Arena chunks |
| Reclamation | Exact matching-provider free of a live allocation; no bulk-only substitution |
| Alignment | Powers of two up to the explicit qualified maximum; § 4 |
| Capacity | Dynamic and unknown; no guaranteed free bytes or infallible reservation |
| Failure | Canonical AllocationError; § 5 |
| Context limits | Ordinary permitted execution contexts only; § 6 |
| Lifetime | Storage remains valid until matching release, subject to owning language lifetime |
| Binding | Exact target malloc/free call ABI, size representation, link requirements and adapter revision |

The binding records the target's C `size_t`, pointer/address-space facts and
maximum native allocation extent. A source `uint` is never assumed to be
`size_t`. The runtime adapter and its allocation tokens belong to the compiler
implementation; these are not new Sec FFI declarations or user-visible types.

The allocation and deallocation endpoints must use the same runtime heap
instance and compatible CRT linkage. On Windows this includes CRT instance and
linkage mode. A library cannot free storage through its own unrelated CRT.
Cross-module ownership requires a verified common binding or a retained call
to the originating deallocator. If neither exists, reject the boundary.

## 4. Size, alignment and allocation adapter

**4(1)** Requests carry a byte extent and a required alignment from canonical
layout facts. Alignment must be a nonzero power of two within the record's
qualified maximum. Do not infer that maximum from malloc's fundamental
alignment. The maximum must cover every admitted native layout; an unsupported
statically known layout is diagnosed before emission.

**4(2)** A target adapter provides stronger alignment when necessary by
requesting extra bytes from malloc and selecting an aligned interior byte range.
It retains the original malloc pointer, provider identity and release metadata
in an implementation-owned allocation token. The language value's public
representation is not fixed here. Tokens must be retainable without a second
fallible allocation, using reserved bytes or compiler-managed owner metadata.

**4(3)** All extent conversions, metadata overhead, alignment padding and offset
computations are checked against the selected target's size and object-extent
limits before the malloc call. The returned byte range and its metadata must
remain inside that one allocation. The adapter uses the qualified target's
pointer/provenance rules; a universal integer-pointer trick is not assumed.
Qualification must demonstrate the selected adapter's validity for that target.

**4(4)** The original base is passed to the matching free exactly once. An
interior pointer is never passed to free. A public reference/view cannot acquire
the allocation token's authority to release storage. The adapter never calls
realloc on a live allocation and never relocates already published storage.

**4(5)** A zero-byte request needs no malloc/free and consumes no provider
capacity; its internal empty token has no releasable base. This does not relax
the source operation's static context, ownership or failure-contract rules.
No semantics depend on a C runtime's malloc(0) behavior or empty pointer value.

## 5. Failure contract and MD-039

**5(1)** The adapter is fallible and returns success or the existing canonical
`AllocationError`. An invalid extent or unrepresentable target/overhead size is
`InvalidSize`; an invalid/unsupported runtime alignment is `InvalidAlignment`;
a valid nonzero request for which malloc returns null is `OutOfMemory`.
Missing or incompatible provider evidence is a compile-time plan error, not a
runtime `Unsupported` fallback. Existing API-specific errors take precedence
when their owning rulebook defines a more specific failure before this request.

**5(2)** Allocation has no language-visible side effects on failure. Temporary
provider storage acquired before a failed operation is released through the
same binding. The adapter does not consult errno to classify null, invoke Sec
handlers, retry indefinitely, or switch allocators. A qualified runtime must
provide the null-return failure behavior; extensions which invoke an allocator
failure callback must be disabled or separately qualified for this binding.

**5(3)** MD-039 string materialization maps provider failures to
`StringError.Allocation(cause)`. Under the protecting try it follows the resolved
StringError route; unprotected `+`/`+=` uses the registered
StringMaterializationFailure panic reason. Cleanup and the panic endpoint must
not allocate. Interpolation retains its own requirement for try.

**5(4)** Concat evaluates the canonical ordered segments and checks total byte
extent, allocates final character storage once, fills it and publishes only
success. Provider bookkeeping is part of that allocation, not another text
materialization. `+=` retains its destination until the complete candidate is
ready and commits exactly once. Operand failure, overflow or null allocation
leaves the destination unchanged and releases any candidate storage. String
sharing and final reclamation must follow the separately resolved string
lifetime/representation contract; choosing malloc does not authorize early
free, leaks as an ownership strategy, or a new implicit reference-counting ABI.

## 6. Execution contexts and lifetime

**6(1)** This provider offers no bounded-time, nonblocking, lock-free,
interrupt-safe or signal-handler-safe guarantee. It cannot satisfy ISR,
interruptSafe, noAlloc or equivalent prohibited-context requirements. A safe
ordinary task may use it only where that task's existing allocation/effect
rules permit. This decision defines no scheduler, thread or event behavior.

**6(2)** Concurrent ordinary calls are permitted only by a record whose runtime
qualification establishes allocator safety in that execution environment.
This gives no concurrent-access permission to allocated Sec values or Arenas.
Unverified concurrency does not become Supported through the Hosted family.

**6(3)** The provider lifetime encloses every live allocation and deallocator
call. A runtime/DSO containing the binding cannot unload while its tokens remain
live. Allocation identities stay distinct across address reuse. References and
slices retain their canonical owner/domain and may not outlive their storage.
The compiler does not repair an invalid lifetime by moving storage to this heap.

## 7. Owned Arena backing

**7(1)** Arena.WithCapacity and Arena.Growable obtain owned backing from the
same resolved provider as their canonical active context; there is no hidden
global heap. A supplied fixed Arena cannot become a growable heap merely because
this catalog entry exists. FromBuffer remains caller-owned backing.

**7(2)** Growable acquires fresh independently allocated chunks. Growth never
reallocates or moves an existing chunk, preserves ArenaDomain and follows the
existing epoch rules. Requested object layout and chunk overhead are checked
before acquisition. A failed acquisition publishes no object, dependency or
capacity change. Initial/growth sizing follows existing capacity guarantees;
the geometric growth factor is an implementation policy, not a language promise.

**7(3)** Reset advances the epoch and makes retained chunk storage reusable
under arena.md; this provider does not require eager trimming. Release and
destruction release all owned chunks and their metadata exactly once through
their originating bindings. Reset/Release legality and live-reference rejection
are unchanged. This proposal does not give per-object free to an Arena client.

## 8. Qualification, plan fingerprints and migration

**8(1)** Each positive catalog record is backed by maintained ABI/runtime
evidence, a successful target link check and target-run qualification of
alignment, release, zero extent, boundary sizes and controlled failure behavior.
Tests cover both architectures claimed by that record; emulator execution is
identified as such. Cross-compilation consumes qualified target evidence, never
a successful probe of the compiler host. Every build need not rerun qualification.

**8(2)** Freeze and fingerprint include catalog identity/revision/digest,
target/ABI/runtime compatibility, adapter revision, provider capabilities and
limits, linkage mode and binding digest. Cache/import compatibility compares
those facts. A missing binding fails before executable emission; no unused
positive heap capability may be invented to make a plan freeze.

**8(3)** Production failure tests use an explicit test-toolchain binding with
deterministic allocation failure, the same adapter semantics and a distinct
fingerprint. It is not a user manifest option, an ambient environment-variable
override, or permission for ordinary projects to claim verified provenance.
Test allocation N, every earlier successful allocation and every cleanup edge.

**8(4)** On implementation, remove the provisional Hosted-string-to-Supported
projection. Qualified catalog defaults preserve ordinary Hosted behavior;
unqualified configurations become Unknown with a targeted diagnostic. Never
silently select the compiler host's malloc. Allocation-free/noalloc plans need
no provider. This may reject formerly provisionally accepted allocating programs;
diagnostics explain the missing target/runtime qualification.

## 9. Acceptance evidence and integration boundary

Acceptance requires catalog selection/ambiguity/ABI-negative tests, frozen-plan
and cache tests, target adapter tests, source-to-binary concat/append success,
provider failure and rollback, owned Arena failure/release, noAlloc/noPanic and
disabled/unqualified profile tests. Over-alignment and CRT-mismatch controls are
required. Tests must prove cleanup/release counts and destination preservation,
not merely observe a diagnostic or high-level operation.

MD-151 can leave the missing-decision ledger after owner acceptance and
normative integration, with all implementation obligations in the existing
governance owners. MD-039 stays open until its original full conformance point
passes through the one native compilation track. This proposal alone closes no
implementation point and does not require splitting that point into microtasks.

## 10. External implementation references

These explain adapter constraints; they do not define Sec semantics or qualify
any target by themselves:

- WG14 C working draft, memory management contracts:
  [N3220](https://www.open-std.org/jtc1/sc22/wg14/www/docs/n3220.pdf).
- Microsoft, matching CRT instances across library boundaries:
  [Potential errors passing CRT objects across DLL boundaries](https://learn.microsoft.com/cpp/c-runtime-library/potential-errors-passing-crt-objects-across-dll-boundaries).
- Microsoft, malloc null failure and optional failure-handler extension:
  [malloc](https://learn.microsoft.com/en-us/cpp/c-runtime-library/reference/malloc?view=msvc-170).

No part of the proposal is intended as a claim that a C runtime can guarantee
successful allocation whenever virtual address space appears available.
