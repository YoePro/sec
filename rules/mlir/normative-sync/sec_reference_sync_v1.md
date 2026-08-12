# Package 15 Normative Synchronization - Direct Safe References

## Status

Normative synchronization for:

```text
rules/references.txt
rules/runtime_checks.md
rules/reference_model.md
rules/panic.md
```

Package:

```text
SEC-MLIR-P15
```

Repository baseline:

```text
152c772
```

`rules/reference_model.md` is canonical for Sec 0.1 reference semantics.

This synchronization removes older wording that can otherwise be read as
contradicting the canonical model.

---

# 1. Reference validity is not compile-time-only

Older `references.txt` wording states that reference validity is determined
entirely at compile time.

Canonical rule:

```text
safe reference validity is established by semantic analysis
and may be preserved through static proof or required runtime validation
```

The compiler should eliminate runtime validity checks when it proves them
unnecessary.

A target/profile may retain hardening checks where permitted.

---

# 2. Runtime metadata is not mandatory

The existence of an epoch dependency does not imply that a reference runtime
value contains an epoch field.

Possible safe representations include:

```text
address only
address + expected epoch
side-table checked address
hardware capability/tag
other semantics-preserving target representation
```

The compiler selects representation after semantic analysis.

---

# 3. Default logical epoch width

Canonical default:

```text
64 bits
```

including 32-bit general-purpose targets.

Pointer width and logical epoch width are independent.

Shorter widths are compile-time-selected only when stale-reference safety is
preserved by proof, retirement or explicit exhaustion behavior.

Epoch width is never dynamically selected at runtime.

---

# 4. Ordinary stale direct reference

A stale ordinary:

```text
ref T
ref mut T
```

indicates violation of a safe-reference guarantee.

Using it results in:

```text
deterministic panic
or target trap where the selected profile uses trap semantics
```

Canonical stable panic reason:

```text
panic.invalid-reference-generation
```

This is not normal Result-producing business logic.

---

# 5. `try` does not recover ordinary stale direct references

Remove/clarify any generic table entry implying:

```text
ordinary direct reference generation validation -> ReferenceError through try
```

for Sec 0.1 direct references.

Programmers do not write `try` around every ordinary safe dereference.

The non-panicking alternatives are:

```text
static proof
compile-time rejection of an unsafe reference pattern
or an explicitly fallible handle abstraction for a domain where stale
resolution is expected normal behavior
```

---

# 6. Stable and weak handles remain fallible

A stale stable/weak handle may be normal resolution failure.

Possible future API result forms include:

```text
Option[ref T]
Result[ref T, StaleReferenceError]
```

The exact source API remains deferred.

Do not apply ordinary direct-reference panic semantics to weak-handle resolution.

---

# 7. Runtime-checks synchronization

Keep `reference-generation validation` as a runtime-check category.

Clarify its three outcomes:

```text
proven safe:
    no runtime check

ordinary direct safe reference, dynamic validation:
    check + panic.invalid-reference-generation on violated safety guarantee

fallible handle resolution:
    explicit Option/Result according to handle API
```

These are different semantics.

---

# 8. Generation checks are partial safety mechanisms

Generation/epoch validity never replaces:

```text
ownership
borrow compatibility
bounds
initialization
type validity
relocation correctness
address-space correctness
concurrency synchronization
```

A matching generation alone does not make a reference safe.

---

# 9. Compile-time origin analysis remains authoritative

Existing Sema reference-origin and Place analysis remains compile-time.

Dynamic epoch metadata must not become a runtime borrow checker or runtime
provenance-disambiguation mechanism.

Known origin sets are analysis facts.

Unknown origin is handled conservatively.

---

# 10. No mandatory runtime

Synchronization must preserve:

```text
no GC
no reference counting requirement
no universal handle table
no global generation manager
no runtime borrow counters
```

Programs whose references are fully proven may lower without epoch metadata.
