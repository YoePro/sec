# MD-070: As-property naming correction

- Status: Applied to canonical rulebooks on 2026-11-06.
- Authority: language-owner clarification, 2026-11-06.
- Supersedes: MD-070's method spelling for raw-to-safe reference conversion.
- Implementation governance: `governance/memory.yaml`.

## § 1 Naming authority

The established naming convention is binding: `As*` members are properties,
read without parentheses. An API that requires a function uses `Get*()`.
A previous correction's contradictory example does not override this convention.
This correction repairs the MD-070 inconsistency; it introduces no GetRef alias.

## § 2 Canonical raw-pointer properties

`RawPtr[T].AsRef: ref T` and `RawPtr[T].AsRefMut: ref mut T` are read-only,
unsafe compiler-known properties. There are no call parameters or method forms.
`RawPtr[void]` has neither usable property.

```sec
unsafe {
    let observer := pointer.AsRef
    let editor := otherPointer.AsRefMut
}
```

MD-079 call-form `unsafe Call()` authorizes a call only. Reading these properties,
like `.Ptr`, requires an enclosing unsafe block. `pointer.AsRef()` and
`pointer.AsRefMut()` are rejected as property calls, with a suggestion to read
the property in an unsafe block. The compiler does not automatically add unsafe
or change its boundary. Ordinary user properties and unrelated types do not
acquire a raw-pointer conversion through a name match.

## § 3 Unchanged safety and lowering contracts

All non-nullness, alignment, initialization, lifetime, address-space, ownership
and shared/exclusive-authority requirements of raw_pointers.md §§18–18A remain.
Known invalidity is rejected even inside unsafe. No ownership, pinning, storage
lifetime extension, allocation or general conversion trait is introduced.
Stable compiler-known identities remain CKM-RAWPTR-AS-REF and
CKM-RAWPTR-AS-REF-MUT. Their presentation kind/signature is property.
The distinct raw-to-safe Semantic IR operation still awaits MD-084; a valid
property read meets the same explicit unsupported-lowering boundary.

## § 4 Conformance

Sema, completion, hover, synthetic definitions and documentation agree on
property kind and concrete reference authority. Methods with the old spelling,
property reads outside unsafe, void-pointer conversions, known null or unavailable
origins and conflicting authority are negative cases. Formatting preserves
property reads, unsafe blocks, comments and tokens and invents no parentheses.
