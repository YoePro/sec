# MD-058 — Collection Capacity and Reservation; `Len` and `SizeOf` Synchronization

- **Status:** Applied 2026-10-12 to `rules/collections/collections.md` (§§ 3.1, 3.4, 13.4, 13.12, 14.3, 14.8, 15.2, 15.5, 16), `rules/compiler/compiler_known_members.md` (string `Len`/`SizeOf`, `ToByteArray`, "Collection capacity members"), `rules/memory/allocation.md` (§ 3(2a)), `sec/stdlib/str/stdlib_str.md` (§§ 8.3, 8.5) and the string-length prose of `types.md`, `contracts.md`, `lexical_structure.md` and `core-library.md`; the compiler-known registry, core and stdlib sources follow; runtime lowering of reservation remains tracked in governance
- **Created:** 2026-10-12
- **Last updated:** 2026-10-12
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path when accepted:** `rules/corrections/pending/md058-collection-capacity-reservation-correction-20261012.md`
- **Decision ID:** `MD-058`
- **Primary rulebook:** `rules/collections/collections.md`
- **Also amends:** `rules/compiler/compiler_known_members.md`, `rules/memory/allocation.md`, `sec/stdlib/str/stdlib_str.md`, and the canonical string/core documentation and declarations where inconsistent
- **Related, not resolved:** `MD-050` (validity of UTF-8 in every `string`); the existing allocation-profile and compiler-known-declaration work
- **Replaces:** No published numbered paragraph. Update existing sections in place; do not renumber them.
- **Governance:** Synchronize existing collection, core/compiler-known, string/stdlib, and allocation implementation fragments without duplicate entries. Remove MD-058 from `missing-decisions.yaml` **only after** the owning rulebooks and governance are synchronized.

## 1. Scope and authority

**1.1.** This correction records the language owner's decision to add consistent capacity reservation to Sec's **dynamic `list[T]`, `map[K, V]`, and `set[T]`**, completing the capability needed by `str.Builder`. It also records the accompanying decisions on the difference between logical length, reserved capacity, and payload size in bytes.

**1.2.** The new operations belong to compiler-known collection types, not to optional library extension methods. Their public declarations and LSP-visible documentation must be integrated into the owning compiler/core declaration registry. `sec/stdlib/str` consumes those operations; it does not obtain privilege to extend `list[T]` itself.

**1.3.** Statements in this correction are normative source-language behavior, **not assertions that the compiler already implements them**. The current collection rulebook documents `list.Capacity`, but omits `Reserve` and `WithCapacity`; it does not expose `Capacity` on map/set. The existing allocation rulebook contains a `list[int].WithCapacity(100)` example without the complete collection contract. The correction supplies the missing contract.

**1.4.** Every change to an existing rulebook must preserve that rulebook's published section/paragraph identities. Amend the relevant existing paragraph and add stable subparagraphs where necessary. Avoid duplicate alternative definitions.

## 2. Types receiving the new public API

**2.1.** Exactly the following **dynamic** compiler-known collection families receive both `Reserve` and `WithCapacity`:

| Dynamic type | Mutable instance member | Type-associated factory | Read-only properties |
|---|---|---|---|
| `list[T]` | `Reserve(additional: uint) Result[void, CollectionError]` | `static fn WithCapacity(capacity: uint) Result[list[T], CollectionError]` | `Len: uint`, `Capacity: uint` |
| `map[K, V]` | `Reserve(additional: uint) Result[void, CollectionError]` | `static fn WithCapacity(capacity: uint) Result[map[K, V], CollectionError]` | `Len: uint`, `Capacity: uint` |
| `set[T]` | `Reserve(additional: uint) Result[void, CollectionError]` | `static fn WithCapacity(capacity: uint) Result[set[T], CollectionError]` | `Len: uint`, `Capacity: uint` |

**2.2.** These are the **complete signatures of the newly required operations**. They are type-associated or instance methods as indicated; they are not constructors named `New`, free functions, aliases, property setters, or an implicit allocator selection API. `WithCapacity` is the specifically approved, fallible **associated factory**, not a replacement for the usual `init(...)` construction rules. Integrate real declarations and their real privileged implementations through the canonical core/compiler-known mechanism; do not publish an abbreviated dummy `impl` as an executable substitute.

**2.3.** The new `map` and `set` `Capacity` observations are read-only `uint` properties. `list.Capacity` already exists and retains its established meaning. This correction **does not introduce a `Capacity()` method**.

**2.4.** The following do **not** gain `Reserve` or `WithCapacity`:

- `list[T, Capacity]`, `map[K, V, Capacity]`, and `set[T, Capacity]`: their maximum capacities are established by type-level constraints.
- `T[]`: any internal spare capacity remains private, as already established by `collections.md` §6.3.
- `T[N]`, `ref T[]`, `ref mut T[]`, and other non-growing views.

**2.5.** Bounded collection forms retain or gain a read-only `.Capacity` **observation** equal to their compile-time maximum entry count, consistent with the already specified `list[T, Capacity].Capacity`. For `map[K, V, Capacity]` and `set[T, Capacity]`, this introduces only an observation; it does **not** permit dynamic growth beyond the bound or a reservation method.

## 3. The precise reservation contract

**3.1.** On a dynamic collection `collection` with current logical length `L`, `collection.Reserve(n)` successfully guarantees:

```text
collection.Capacity >= L + n
```

The sum is checked for `uint` and target-representable capacity overflow **before** mutating collection state. `n` measures **additional future entries beyond the existing `Len`**, not an increase of `n` above the old `Capacity` and not a byte count.

**3.2.** For `list`, `Len` and `Capacity` count live elements and available element slots. For `map`, they count key/value **entries**. For `set`, they count **distinct elements**. Associative capacity does not expose bucket counts, hash-table load factors, control-byte allocations, or the physical representation.

**3.3.** An already sufficient capacity makes `Reserve(n)` a successful **no-op**: no allocator call, no relocation, no change to `Len`, no change to `Capacity`, and no invalidation of existing references. `Reserve(0)` is therefore always a successful no-op for a valid collection.

**3.4.** When growth is necessary, the implementation may reserve more than the requested minimum. Its growth factor, byte layout, hash-table bucket structure, and excess capacity are not source-language guarantees. After successful reservation, the collection is still logically unchanged: **the same live values/entries, same ownership, same `Len`**. No new `T`, `K`, or `V` is constructed merely because capacity increased.

**3.5.** A successful `Reserve(n)` promises that up to `n` subsequently added elements/entries can be accommodated **without additional collection-backing capacity growth**, provided the collection has not undergone intervening operations that change that guarantee. This does not promise that arbitrary `T`, `K`, or `V` construction, copying, callbacks, custom allocators, or other independent operations are allocation-free or infallible.

**3.6.** Reservation may relocate or reorganize backing storage. Normal ownership, borrowing, and generational/reference invalidation restrictions apply. It is illegal to perform a conflicting reservation while an active borrow forbids that structural mutation. Successful relocation must not leave live owners referring to a destroyed backing incarnation.

**3.7.** `Reserve` is **transactional**. On an allocation, overflow, or other supported resource failure, the original collection remains observably unchanged: same entries, values, keys, length, capacity, ownership, and valid backing identity. No partial rehash, partial move, leaked newly allocated backing, or accidental element destruction is permitted. This failure guarantee covers a map/set rehash as well as contiguous-list relocation.

## 4. `WithCapacity`: initial capacity, not initial length

**4.1.** The associated call `Type.WithCapacity(n)` creates a **new empty owning collection** whose capacity is at least `n` on success:

```text
newCollection.Len == 0
newCollection.Capacity >= n
```

No elements, keys, or values are constructed by capacity reservation. No prior mutable owner is involved.

**4.2.** This operation is fallible, using exactly `Result[CollectionType, CollectionError]`. On failure it returns no partially constructed owner. Any temporary storage is cleaned up in accordance with the allocation and destruction rules.

**4.3.** `WithCapacity(0)` produces a valid empty collection without requesting backing allocation. Any regular default empty construction remains available independently; no empty `init` must be added to type documentation solely because this factory exists.

**4.4.** `WithCapacity` uses the active explicit or implicit allocation context **only if** it must allocate. It must not invent a global allocator or fall back to an unrelated allocator/provider. If allocation is required but no valid context can exist for a target, follow the target/allocation rules for compile-time rejection; if a valid context is present but allocation fails at runtime, return the designated `CollectionError`.

## 5. Error surface and source-level calls

**5.1.** `Reserve` and `WithCapacity` both return the already established public **`CollectionError`** family. In particular, these operations must distinguish the existing cases:

| Existing case | When it applies here |
|---|---|
| `CollectionError.SizeOverflow` | `Len + additional`, requested total capacity, or required storage-size computation is unrepresentable. |
| `CollectionError.AllocationFailed` | Required allocation/reallocation fails with an available legitimate allocation context. |
| `CollectionError.CapacityExceeded` | A hard collection/target capacity limit, where the existing capacity contract classifies the request this way, prevents a representable reservation. |

**5.2.** This correction does **not** create a second reservation-specific error type or silently rename existing `CollectionError` variants. Since the published `collections.md` currently says its error cases include **“at least”** these cases, integration must locate the **canonical complete** `CollectionError` declaration and reflect its exact members in the owning core/public documentation. Do not invent undocumented variants or claim that the three rows above are necessarily the complete enum.

**5.3.** The exact user-visible calls are:

```sec
let mut bytes: list[byte]
try bytes.Reserve(1024)

let mut numbers := try list[int].WithCapacity(100)
let mut users := try map[string, int].WithCapacity(100)
let mut names := try set[string].WithCapacity(100)

try numbers.Reserve(20)
try users.Reserve(20)
try names.Reserve(20)
```

`try` remains governed by the canonical error-handling rules. These snippets assume an allocation-capable environment. They introduce no new `try` behavior.

**5.4.** The quantity is always in logical element/entry units:

```text
Existing map: Len = 20, Capacity = 40
Reserve(100) succeeds -> Capacity >= 120
Reserve(10) succeeds -> no growth required if Capacity >= 30
```

The operation never means “reserve 100 bytes” or “increase current capacity by 100.”

## 6. `str.Builder` and the original MD-058 blocker

**6.1.** The existing `str.Builder.init(initialCapacity)` remains **infallible**. `initialCapacity` is explicitly a **best-effort preallocation hint**, not a guarantee. The builder must remain a valid empty builder if it cannot honor the hint, without hiding a failed initial reservation as successful guaranteed capacity.

**6.2.** The existing `str.Builder.Reserve(additionalBytes: uint) Result[void, CollectionError]` is the **guaranteed**, explicit fallible operation. On success, the builder can accept at least `additionalBytes` more encoded bytes beyond its current encoded payload length without another backing-capacity increase. On failure its prior contents, length, ownership, and capacity remain unchanged.

**6.3.** If `Builder` uses `list[byte]` as its backing, `Builder.Reserve(additionalBytes)` delegates the corresponding capacity guarantee to its backing list with the correct existing byte count. `Builder` need not introduce a second allocation mechanism. A `Builder` implementation must not simply ignore `Reserve` or report success without meeting its capacity promise.

**6.4.** The best-effort `init` hint must obey allocation-context rules. Absence of a valid allocation context is **not** permission to select a hidden global heap. Failure to satisfy the hint is not permission to discard an existing initialized value. Operations that subsequently require allocation remain fallible through their canonical contracts.

## 7. `Len`, `Capacity`, and `SizeOf` are different observations

**7.1.** `.Len: uint` is the number of **logical items**, never automatically a byte count. In particular:

| Category | `.Len` | `.Capacity` | Instance `.SizeOf` |
|---|---|---|---|
| `string` | Unicode scalar (`rune`) count | Not added | Number of encoded UTF-8 payload bytes |
| `byte[]` / `char[]` | Number of elements | Not public | Represented element payload byte extent |
| `rune[]` | Number of elements | Not public | Represented element payload byte extent |
| `list[T]` | Number of elements | Available element capacity | Initialized contiguous element payload bytes |
| `map[K, V]` | Number of entries | Available entry capacity | No general **payload** `SizeOf` contract added |
| `set[T]` | Number of distinct elements | Available entry capacity | No general **payload** `SizeOf` contract added |
| Bounded `list` / `map` / `set` | Number of live entries | Declared maximum element/entry count | Existing representation-specific rules remain unchanged |

**7.2.** A contiguous-sequence instance's `.SizeOf: uint` denotes its **represented initialized payload byte extent**, not allocated capacity, owner descriptor size, allocator metadata, or reserved but uninitialized slots. For `list[T]`, the existing rule remains conceptually `Len * element layout stride`.

**7.3.** For `string`, the approved semantic change is:

```sec
let text := "Hej π"

// text.Len    == 5 (Unicode scalar values)
// text.SizeOf == 6 (UTF-8 payload bytes)
```

No terminating NUL, unused capacity, descriptor, or allocation header is included in `text.SizeOf`. This rule describes the **logical encoded payload length**, not the total heap memory committed by an allocator. The latter is a separate diagnostic/allocator concern.

**7.4.** Distinguish **value-form** and **type-form** observations:

```sec
let payloadBytes := text.SizeOf   // Encoded bytes of this string value.
let descriptorBytes := string.SizeOf // Physical layout size of a string value.
```

The associated type-form `Type.SizeOf` remains a compiler-known physical-layout query. The value-form `string` behavior above **supersedes** the older descriptor-size rule in `compiler_known_members.md`. `SizeOf` itself does not need to be a separately implemented function in `sec/core`; compiler-known declaration, Semantic IR, backend, and LSP contracts remain authoritative.

**7.5.** Do **not** add redundant public aliases `ByteLen`, `RuneLen`, or `CharLen` for these two observations. Where older public core string declarations expose `ByteLen` and `RuneLen`, migrate references to `.SizeOf` and `.Len`, respectively, and update the owning declarations, call sites, tests, and generated documentation. An internal helper needed for efficient scalar counting is not automatically a new public member.

**7.6.** The previously decided default `Len(string) = Unicode scalar count` does **not** settle `MD-050`: whether all `string` values are guaranteed valid UTF-8, how constructors validate, and what happens at invalid byte/slice boundaries remain open there. Do not silently invent a replacement-character rule, invalid-UTF-8 acceptance rule, or separate `CharLen` interpretation while implementing this correction. Compiler-known `len(text)` must continue to agree with `text.Len` for logical count, subject to its existing signed-result representability contract.

**7.7.** `map` and `set` are not contiguous logical payloads, so the addition of `.Capacity` does **not** confer `.Ptr` or a universal `Len * entry stride` byte-size property. A physical type-layout query, when valid under the general `Type.SizeOf` rule, is conceptually distinct from associative storage allocation and must not be reported as payload size.

## 8. Documentation and compiler integration requirements

**8.1.** Amend `rules/collections/collections.md` as follows:

1. Extend §13 (`list`) with the exact `Reserve`, `WithCapacity`, and `Capacity` guarantees in §§2–5 of this correction.
2. Extend §14 (`map`) and §15 (`set`) with the corresponding full contracts and read-only `Capacity` observations for both dynamic and bounded variants.
3. Clarify §3 (shared properties) and §16 (errors) where appropriate. Preserve `T[]` §6.3's explicit non-exposure of capacity and reservation.
4. Explain that `SizeOf` is **not** a general map/set payload size and that `Reserve` never causes construction of new logical elements.

**8.2.** Amend `rules/compiler/compiler_known_members.md` and its actual declaration registry:

1. Register/describe the full newly required collection public methods and properties, their static/instance nature, `uint` parameter/result widths, `Result` error types, mutability, allocation effects, and ownership effects.
2. Update `string.Len` from byte count to Unicode scalar count and `string` **value-form** `SizeOf` from descriptor size to encoded UTF-8 payload bytes.
3. Preserve `string.SizeOf` **type-form** as descriptor size, other type-form layout queries, and the existing contiguous-sequence semantics.
4. Update contradictory text such as the old test requirements `string byte length`, `string descriptor`, and `ToByteArray`'s obsolete equation `array.Len == text.Len`: the byte-array length is the encoded payload byte count (`text.SizeOf`).
5. Preserve compiler-known member identity and LSP declarations; a prose-only API surface is insufficient.

**8.3.** Synchronize `rules/memory/allocation.md`'s already-present `list[int].WithCapacity(100)` example to the now-complete contract. Include map/set equivalents where useful. No implicit global allocation provider is added.

**8.4.** Synchronize `sec/stdlib/str/stdlib_str.md` and `Builder` implementations/tests with the best-effort constructor hint and guaranteed fallible `Reserve`. Keep `Builder` public method names and parameter spelling already defined there; do not invent extra builder aliases.

**8.5.** Update actual core `string` declarations, its exposed documentation and LSP hover to remove duplicate length observations only after the corresponding compiler semantics are implemented. Byte-oriented internals in `Slice`, `Split`, `IndexOf`, encoding/FFI helpers, and other core string operations must not accidentally start using the new `Len` (rune count) as a byte offset. This correction does not independently decide byte-versus-rune indexing or UTF-8 validity rules assigned to MD-050 and their existing owners.

**8.6.** Update any existing governance fragment that tracks the affected collection/core/string/allocation features; do not create parallel items when existing IDs suffice. A rulebook decision is distinct from verified compiler/runtime implementation.

## 9. Required conformance and regression tests

**9.1. Dynamic reservation:** For each of `list`, `map`, and `set`, test empty construction, `WithCapacity(0)`, `WithCapacity(n)`, `Reserve(0)`, `Reserve(n)` with insufficient and sufficient existing capacity, and the exact guarantee `Capacity >= Len + n`. Check both `Len` and payload ownership stay unchanged after reservation.

**9.2. Error paths:** Fault-inject allocation failure; trigger arithmetic/representation overflow; test legitimate target capacity limits. Confirm the designated `CollectionError` case and **transactional rollback** of entries, keys/values, length, capacity, storage identity, and destructor counts. For map/set, test rehash interruption and no partly visible table.

**9.3. Borrowing and lifetime:** Test rejected reservations during conflicting active borrows; legal growth after borrow end; relocation/generation invalidation rules; no invalidation for the no-op sufficiently-reserved path.

**9.4. API exposure:** Require `Reserve` and `WithCapacity` for the three dynamic collection families and `Capacity` for dynamic/bounded collection families. Reject these growth members on bounded variants, `T[]`, fixed arrays, and slices. Reject access to `T[].Capacity` and map/set `.Ptr`; ensure no `Capacity()` alias.

**9.5. Map/set semantics:** Check capacity and length in **entries**, not buckets or bytes, including insertion of an existing key/value, removal, and re-addition. Do not make iteration order a test invariant. Confirm capacity promises do not imply infallible user-defined payload construction.

**9.6. Builder:** Test successful `Builder.Reserve(additionalBytes)` and that its capacity guarantee is real; fault-inject reservation failure and preserve original contents. Test `init(initialCapacity)` when the hint can and cannot be met, including zero and allocation-context restrictions. Infallible `init` must not incorrectly promise successful preallocation.

**9.7. `Len` / `SizeOf`:** Test ASCII, multibyte scalars, empty strings, fixed/owning/borrowed arrays, `list[byte]`, `list[int32]`, map, set and bounded families; distinguish logical counts, encoded bytes, type-form descriptor size, physical payload strides, and capacities. In particular, assert `"Hej π".Len == 5` and `"Hej π".SizeOf == 6`, and ensure `"Hej π".ToByteArray().Len == 6`, not 5. Also test no redundant public `ByteLen`, `RuneLen`, or `CharLen` alias.

**9.8. Toolchain:** Verify the lexer/parser, type checker, compiler-known member registry, Semantic IR, code generation, formatter, diagnostics, LSP hover/completion, and incremental-vs-cold conformance for the new collection members and changed string observations. Do not label pending pieces implemented merely because the declarations are present.

## 10. Completion condition

**10.1.** MD-058 is **normatively decided** by this correction. It becomes eligible for removal from the authoritative `missing-decisions.yaml` after the owning rulebooks contain the actual semantics and public declarations, and all affected governance fragments are synchronized according to the backlog's removal contract.

**10.2.** Implementation work that remains, including any unimplemented compiler-known method bodies, allocation faults, LSP declaration registry work, and `MD-050` string-validity policy, must stay visible in their owning implementation-status and/or separate decision tracking. Do not misclassify this correction as completion of unrelated work.
