# MD-110 — Detached register bit-pattern conversions and their Semantic IR / Sec MLIR representation

- **Status:** **APPLIED 2026-10-30** — normatively integrated 2026-10-26 (registers.md § 12/§ 12.3, types.md, semantic_ir.md §§ 21(6)/59(5), sec_mlir_dialect.md v11.41.2 and the v11 snapshot, P15 note); MLIR spelling completed 2026-10-30 (TypeAttr register_type, sec.int.bit_extract, basis "bit-extraction"; lowering v11.30). Sema, the three Semantic IR operations, C++ verifiers, Go emitter and native bit[N]/from_bits/to_bits execution are implemented and tested. The checked try T(x) path still stops fail-closed in Semantic IR until checked conversions get try failure lowering; it and the § 11(3)/§ 11(5) tests stay in governance/declarations.yaml frontend.registers.
- **Created:** 2026-10-25
- **Last updated:** 2026-10-25
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Decision ID:** `MD-110`
- **Classification:** Constraining (per `missing-decisions.yaml`, 2026-10-23)
- **Archived at:** `rules/corrections/applied/md110-register-bit-pattern-conversion-ir-20261025.md`
- **Primary normative owners:** `rules/declarations/registers.md`; `rules/compiler/semantic_ir.md`; `rules/mlir/sec_mlir_dialect.md`; `rules/mlir/dialect-versions/sec_mlir_dialect_v11.md`
- **Additional integration:** `rules/types/types.md` (cross-reference to the existing `bit[N](value)` contract); matching schema-12 inheritance; P15 detached register-value implementation/tests; applicable `governance/` fragments; `missing-decisions.yaml`
- **Related decisions:** MD-056 (`bit[N](value)`), MD-077 (register values / hardware instances / Semantic IR / MLIR), MD-104 (exact integer conversion proof transport)
- **Replaces:** the open MD-110 conversion-vocabulary question; **does not replace** MD-077 or change hardware-resource access semantics
- **Repository reference and synchronization limit:** Compared with publicly retrievable `registers.md`, `types.md`, `semantic_ir.md` and `sec_mlir_dialect.md`, and with the local complete MD-077 correction dated 2026-10-22. The current `main` commit SHA and the complete changes from 2026-10-23 could **not** be independently verified in this environment. The integrator **must first read current `main` HEAD**, reconcile any already-installed MD-077/v11.41.1 spellings, and record the actual integration commit. Do not overwrite newer canonical sections with stale retrieved text.

> **Authority and purpose.** The language decisions below were discussed and accepted by the project owner on 2026-10-25. The owner delegated internal Semantic IR / MLIR naming and representation choices. The normative contract distinguishes register bit patterns from numbers, exact transfers from deliberate low-bit extraction, and logical register values from ordered byte sequences. This correction is ready for controlled integration; implementation is not claimed complete.

---

## § 1. Scope and non-goals

**§ 1(1).** This correction defines conversions **to and from detached, nominal `register[N]` values**. Detached registers are normal Sec values usable for protocols, packed file formats, computation and snapshots. A register bit pattern has no inherent signedness or numeric interpretation.

**§ 1(2).** This correction does **not** convert or copy a `HardwareInstance[T]` into a detached register value, read hardware, change any physical access width, replace the required type-owned `impl`, or introduce ordinary `Place` operations for hardware endpoints. A hardware observation that returns a detached value remains subject to MD-077 and the hardware access plan; only after that observation may the result be converted as a detached value.

**§ 1(3).** There is no new public core function, method or implicit numeric coercion. Source spelling uses the existing explicit type-conversion form `Type(value)` and the existing explicit extraction `bit[N](value)`.

**§ 1(4).** Integer/register conversion and register/byte serialization are separate operations. This correction fixes conversion semantics and required IR vocabulary; it does **not** introduce a new public register-to-bytes API or choose a `stdlib/io` interface.

## § 2. Canonical logical register value

**§ 2(1).** A detached value of type `T register[N]` consists logically of an exact `N`-bit vector associated with nominal `RegisterTypeID(T)`. Bits are indexed `0..N-1`, with bit zero the least-significant bit. The bit vector includes reserved bits and bits belonging to nested fields.

**§ 2(2).** Two register types with identical widths and bit patterns remain distinct nominal types. Conversion to or from an integer does not erase or implicitly convert nominal register identity.

**§ 2(3).** `msb-first` / `lsb-first` govern assignment of declared fields to logical bit positions. They do not govern integer signedness. `big-endian` / `little-endian` govern ordered multi-byte encoding/decoding, not the meaning of the logical `N`-bit value or a numeric conversion.

**§ 2(4).** Neither host endianness, host integer layout, a native pointer, a hardware bus, nor an LLVM/MLIR physical integer type may be used to infer or redefine the logical register value.

## § 3. Integer source domain and exact construction

**§ 3(1).** The accepted source operands of an explicit detached register construction `T(x)` are primitive signed and unsigned integer values of resolved width, including target-sized `int` / `uint` after their `CompilationPlan` width is resolved, and the raw unsigned eight-bit `byte` scalar. Unresolved integer literals use the existing literal/contextual typing rules. Named numeric types, enums, `char`, `rune`, units, floats and other nominal values do not silently shed their identity; an applicable separate explicit conversion must precede `T(x)`. `bit[N](x)` already produces the ordinary unsigned carrier documented in `types.md` and may therefore be used directly.

**§ 3(2).** For register width `N`, the source *mathematical integer value* `x` is losslessly admissible as a **bit-pattern transfer** if and only if:

```text
unsigned source:                 0 <= x <= 2^N - 1
signed source, x >= 0:           x <= 2^N - 1
signed source, x < 0:           -2^(N-1) <= x
```

A source meeting these bounds produces exactly the low `N` bits of the source's standard two's-complement representation. The bounds ensure that the original source value can be recovered by the appropriate zero extension (nonnegative input) or sign extension (negative input) from those bits. This is *bit-pattern representability*, **not** a requirement that the source be nonnegative or that every source represent the same numeric interpretation.

**§ 3(3).** An accepted nonnegative input is encoded using its ordinary binary bits and zero extension where needed. An accepted negative signed input is encoded using two's-complement low bits and sign extension where needed. In particular, an `int8` value `-1` and a `uint8` value `255` both construct the `register[8]` pattern `0xFF`; they remain **different integer values** in their original types.

**§ 3(4).** `Packet8(-1)` is valid and yields `0xFF`. `Packet8(0xFF)` is valid and yields `0xFF`. `Packet8(0x01FF)` is rejected as a known out-of-range value. For register width eight, signed `-128` is representable, signed `-129` is not, unsigned `255` is representable, and unsigned `256` is not.

**§ 3(5).** This rule is independent of the *source integer width* as long as the source type is legal and its value is representable. Thus an `int32` containing `-1` can construct a `register[8]` with bits `0xFF`, without pretending that negative values are invalid register inputs. A 16-bit signed `-1` used to construct `register[16]` yields `0xFFFF`; a positive `uint8` `255` used to construct `register[16]` yields `0x00FF`.

**§ 3(6).** A statically known source that violates § 3(2) is a compile-time error. Known-invalid construction is not rescued by adding `try`; constant diagnostics remain mandatory. For a runtime value that may violate § 3(2), construction uses Sec's existing fallible explicit conversion and `try`/checked-handling contract. If Sema has already established that the operand is in range, no runtime check is required. A runtime check must test the *bit-pattern representability* bounds of § 3(2), not merely the unsigned interval `0..2^N-1`.

**§ 3(7).** An ordinary source-to-register conversion never silently discards high bits. In particular, it must not implement `T(x)` as an unconditional modulo `2^N` operation on arbitrary values. Such modulo extraction belongs exclusively to the explicit `bit[N](x)` construct or another separately specified intentional lossy operation.

## § 4. Deliberate low-bit extraction

**§ 4(1).** `bit[N](x)` retains its already-normative MD-056 meaning: extract the `N` least-significant bits from the integer operand with `x mod 2^N`, including negative values. The extraction is explicit, pure, infallible, allocation-free, independent of endianness and distinct from exact conversion. `N` and the result type remain governed by `rules/types/types.md`; MD-110 does not re-define them.

**§ 4(2).** Where `bit[N](x)` produces an unsigned carrier already proven to lie in `0..2^N-1`, constructing a `register[N]` from that carrier is an exact, infallible conversion. No duplicate range check or manufactured failure edge is permitted.

**§ 4(3).** The following illustrates the difference:

```sec
// Detached register values contain bits, not signed or unsigned numbers.
type Packet8 register[8] {
    Data: bit[8],
}

let signed: int8 := -1
let unsigned: uint8 := 255
let a := Packet8(signed)              // Valid: 0xFF.
let b := Packet8(unsigned)            // Valid: 0xFF.
let invalid := Packet8(0x01FF)        // Compile-time error: 9-bit positive value.
let explicit := Packet8(bit[8](0x01FF)) // Valid: deliberate low-eight-bit extraction.
```

The `invalid` line is an intentionally rejected diagnostic test; it must not be included in a positive compilation test.

## § 5. Explicit register-to-integer interpretation

**§ 5(1).** An explicit `IntegerType(registerValue)` is admitted for a *detached* register value when the integer result type has a resolved representation width `W >= N`. It neither changes the register nor performs physical I/O. The target integer type chooses the numeric interpretation; the register type itself has no signedness.

**§ 5(2).** For an unsigned integer target, interpret the register's `N` bits as an unsigned magnitude and zero-extend to `W` if `W > N`. For a signed integer target, interpret the same `N` bits as a two's-complement signed pattern and sign-extend from bit `N-1` to `W` if `W > N`. These conversions are explicit, pure, total over the `register[N]` bit domain and do not require `try`.

**§ 5(3).** No implicit register-to-integer conversion exists. No integer target narrower than the complete register width is introduced by MD-110. To select fewer bits intentionally, first convert to an unsigned carrier wide enough for the complete register, then use the already-defined `bit[M](value)` extraction; no implicit lossy narrowing may be synthesized.

**§ 5(4).** Example:

```sec
type Packet8 register[8] {
    Data: bit[8],
}

let packet := Packet8(-1)
let bitsUnsigned := uint8(packet)  // 255.
let bitsSigned := int8(packet)     // -1.
let widerUnsigned := uint16(packet) // 255.
let widerSigned := int16(packet)   // -1.
```

**§ 5(5).** Plain numeric conversions between primitive integers remain governed by their own rules. The signed reinterpretation in § 5(2) applies to the specifically declared explicit **register-to-integer** conversion; it is not a general permission to reinterpret arbitrary primitive integers as signed without a checked conversion.

## § 6. Byte order, serialization and cross-platform equivalence

**§ 6(1).** Neither `T(integer)` nor `IntegerType(detachedRegister)` performs byte swapping, byte decoding, memory reinterpretation, or native-endian conversion. Both use the canonical logical `N`-bit value.

**§ 6(2).** When the register is encoded as or decoded from an **ordered byte sequence**, its declared byte order applies, independently of the sender's or receiver's CPU endianness. The established rule for absent explicit byte order remains unchanged: use the applicable contextual/native policy, which may be unsuitable for a portable wire protocol. A portable cross-platform protocol should declare the required byte order explicitly.

**§ 6(3).** For a 16-bit `msb-first big-endian` register with top-nibble `Code=1` and remaining 12-bit `Value=0x234`, the logical register is `0x1234` and the ordered encoded bytes are `[0x12, 0x34]` on both big-endian system A and little-endian system B. Both decode those bytes to the same logical bit pattern and field values. Both must emit `[0x12, 0x34]` when encoding the same value in the opposite direction.

**§ 6(4).** A little-endian host may store a native 16-bit scalar with bytes `[0x34, 0x12]`. Its native memory layout is **not** the declared external representation of a `big-endian` register. The compiler and libraries must not substitute a raw native memory copy for register serialization unless a separate representation proof demonstrates equivalent ordered bytes.

**§ 6(5).** Field allocation order and byte order remain orthogonal. Converting to or from an integer never reverses field order or changes logical bit positions. The byte-order rule applies to the complete declared register representation as specified in `registers.md`; nested layout does not license a second, undocumented automatic swap.

**§ 6(6).** Any byte-oriented conversion operation or API, whether compiler-known or implemented through `stdlib/io`, must honor §§ 6(1)–6(5) and the existing register byte-order contract. MD-110 does not specify a source-level register↔bytes function, a byte-stream API, or padding policy for non-byte-multiple register widths; these are **not prerequisites** for the present detached integer-conversion IR operations.

## § 7. Canonical Semantic IR operations

**§ 7(1).** Extend the MD-077 `register.value.*` operation family with the following stable compiler-internal identifiers:

| Semantic IR ID | Inputs | Result | Effects |
|---|---|---|---|
| `register.value.fits-bits` | Resolved primitive integer operand; destination `RegisterTypeID` and bit width | Boolean predicate satisfying § 3(2) | Pure |
| `register.value.from-bits` | Resolved integer operand and established bit-fit proof; destination `RegisterTypeID` | Detached value of the exact nominal register type | Pure on its verified success path |
| `register.value.to-bits` | Detached nominal register value; resolved signed or unsigned integer result type with width `W >= N` | Integer interpreted under § 5 | Pure, total |

**§ 7(2).** `register.value.fits-bits` is the *runtime* predicate required when source-domain or static proof is insufficient. Sema emits an explicit conditional branch on its result. The false edge follows Sec's already resolved fallible-conversion failure handling; the true edge may contain `register.value.from-bits`. This uses the existing checked-conversion control-flow structure, not a new source-level `try`, `Result` variant, exception class or implicit trap.

**§ 7(3).** `register.value.from-bits` is an **exact operation**, never an unchecked truncation operation. Sema must establish one of: (a) a compile-time constant fitting § 3; (b) a source value domain wholly fitting § 3; or (c) a dominating successful runtime `fits-bits` predicate for the exact same operand, nominal destination and `CompilationPlan` width. A narrower Sema-proven range may be used to remove a redundant runtime check only if the IR verifier receives an independently checkable canonical proof; otherwise preserve the runtime guard. No pass may invent a proof merely because a machine operation discards high bits.

**§ 7(4).** `register.value.to-bits` requires exactly the source register identity, width, target integer type, selected signed/unsigned interpretation and provenance. It cannot require a value-range check because § 5 guarantees total interpretation for `W >= N`.

**§ 7(5).** All three operations preserve canonical `RegisterTypeID`, operand and result type, exact source and destination bit widths, the resolved `CompilationPlan` where widths are target-dependent, source provenance and the chosen validation/interpretation contract. None consumes, creates or substitutes a hardware binding, resource ID, endpoint, software `impl` instance, or physical access plan.

**§ 7(6).** The existing `bit[N](x)` is **not** lowered through `register.value.from-bits` as a lossy shortcut. Its explicit low-bit extraction is evaluated first by the existing integer/bit-extraction semantics and yields its documented unsigned carrier, which may subsequently undergo an exact register conversion.

## § 8. Canonical Sec MLIR encoding

**§ 8(1).** Extend MD-077's early admitted schema-11 register family, inherited by schema 12, with these mappings:

| Semantic IR | Sec MLIR |
|---|---|
| `register.value.fits-bits` | `sec.register.fits_bits` |
| `register.value.from-bits` | `sec.register.from_bits` |
| `register.value.to-bits` | `sec.register.to_bits` |

The registered type remains MD-077's `!sec.register<"module::TypeName">`. No new register type, raw pointer, hardware-instance conversion, or physical register representation is introduced.

**§ 8(2).** `sec.register.fits_bits` has one resolved scalar-integer operand, a canonical destination register type-identity attribute, and one boolean result. Its predicate is **exactly** § 3(2). It carries no side effects. Its result must be used as the verified success condition before runtime-checked conversion; it must not be replaced by a generic unsigned range test for signed operands.

**§ 8(3).** `sec.register.from_bits` has one resolved primitive-integer operand and one result of `!sec.register<T>`. It is pure only under the bit-fit verification below. It retains no byte-order reinterpretation or implicit endian transform. It must resolve `T` and its `N` in the canonical module register-definition table already required by MD-077.

**§ 8(4).** The mandatory **discardable transport** attribute on `sec.register.from_bits` is `sec.register_fit_proof`, a dictionary with the **exactly one** string field `basis`. Allowed values are `"constant"`, `"source-domain"`, or `"guarded"`:

```text
sec.register_fit_proof = {basis = "constant"}
sec.register_fit_proof = {basis = "source-domain"}
sec.register_fit_proof = {basis = "guarded"}
```

`constant` means that the operand is a verifiable integer constant whose actual value satisfies § 3(2), not merely a user-provided assertion. `source-domain` means that **every** value representable by the operand's resolved primitive source type satisfies § 3(2), including signed values when applicable. `guarded` means that a `sec.register.fits_bits` for the **same SSA value and register type** has its true edge dominate this conversion on **every** path, and the false edge reaches the already resolved checked-conversion failure continuation. A verifier rejects missing/unknown basis, mismatched operands, absent evidence, incorrect control flow or an incompatible unresolved target integer width. The attribute transports proof information, but cannot create proof.

**§ 8(5).** `sec.register.to_bits` consumes one `!sec.register<T>` and produces one signed or unsigned integer scalar of width `W >= N`; it uses § 5(2) to interpret and extend the bit pattern. It is pure, total, and has no range proof or error successor. Its register type ID must match the canonical module table entry. A target type narrower than `N`, or any non-integer destination, is rejected.

**§ 8(6).** A module's `sec.dialect_version` remains 11 or 12 according to its independently selected schema. These operations must be recorded as explicit MD-110 **schema-11 early admissions** in `sec_mlir_dialect.md` v11.41 (after MD-077's v11.41.1), with the matching versioned v11 snapshot updated in the same controlled change. Schema 12 inherits the operations; schemas 1–10 reject them. Adding the operations does **not** change default schema selection, complete unfinished packages or imply backend support.

**§ 8(7).** C++ operation registration, parse/print/round-trip tests, the Go emitter, operation/type verifiers, dialect version gate and lowering support manifest must agree on the exact spelling and meaning. Use the existing public integer scalar representations (`siW` / `uiW`) at the Sec dialect boundary according to the established signedness rules; later signless lowering must preserve the bits and carry the signed interpretation contract, not infer it from a signless `iW` alone.

**§ 8(8).** All three operations are detached, effect-free value computations. The integer/bit extensions in their final lowering may use target-supported instructions or verified scalar sequences. They must not be implemented as register byte serialization, raw `memcpy` of native storage, implicit hardware loads/stores, or an illegal signed-to-signless `arith.bitcast` in MLIR.

## § 9. Lowering and proof safety

**§ 9(1).** Before emitting Semantic IR, Sema resolves nominal register identity, integer source/target type, all widths, the signedness selected by the **integer** endpoint, constant/range proof (where applicable), and whether the operation is exact or requires a checked failure edge.

**§ 9(2).** `register.value.fits-bits` may be omitted if the exact fit is verifiable from a constant or the entire primitive source domain. In all other runtime cases, a check is retained unless an equivalent independently verifiable proof is materialized. Range analysis may optimize redundant checks only without changing observed success/failure behavior.

**§ 9(3).** Failure of a checked integer-to-register conversion occurs **before** producing a register value. An invalid source cannot produce a partially initialized nominal register, a fallback truncated value, a default zero value or a partially published successful IR module.

**§ 9(4).** Compiler optimizations may fold `from_bits`, `to_bits` and `fits_bits` only when they preserve the exact bit vector, the selected signed or unsigned interpretation, the complete nominal type identity and all checked-control-flow obligations. In particular, an intermediate register may not be treated as interchangeable with a primitive integer for typechecking or member access.

**§ 9(5).** Register-to-integer conversion that requires resolving `int` or `uint` width must use the active `CompilationPlan`. Unknown or unsupported target width results in a focused diagnostic or unsupported-lowering result; host-platform guesses are prohibited.

## § 10. Diagnostics

**§ 10(1).** Diagnostics for a rejected constant must identify the declared register width, the source value and why its pattern cannot be represented without losing meaningful high bits, and point to the explicit `bit[N](...)` alternative when intentional truncation fits the programmer's goal.

**§ 10(2).** Diagnostics for an unchecked runtime conversion must explain that the operand may not fit the destination register pattern and indicate the normal checked/`try` conversion path, not propose implicit narrowing.

**§ 10(3).** Diagnostics for an invalid register-to-integer interpretation must report the register width, requested integer result type/width and the rule requiring `W >= N`. When appropriate, suggest conversion through a sufficiently wide unsigned carrier followed by explicit `bit[M](...)` extraction.

**§ 10(4).** Diagnostics shall distinguish detached register conversion from an attempted conversion of `HardwareInstance[T]`, which is invalid without a separately authorized hardware observation through `impl T`.

## § 11. Required conformance and regression tests

**§ 11(1).** Add source/Sema positive tests for exact `register[8]` construction from `int8(-1)`, `int32(-1)`, `uint8(255)` and `bit[8](0x01FF)`, and for different-source-width extensions into `register[16]`: signed `-1 -> 0xFFFF` and unsigned `255 -> 0x00FF`.

**§ 11(2).** Add negative constant tests: `Packet8(0x01FF)`; `Packet8(256)`; `Packet8(-129)`; and ordinary type/implicit-conversion mismatches. These are compile-time errors. `try` does not legalize a known invalid constant.

**§ 11(3).** Add runtime checked tests for `int16` or `uint16` sources whose values may or may not fit a register width, including success `-128`, `-1`, `0`, `128`, `255` and failure `-129`, `256`. Verify success/failure paths and absence of partial values or implicit truncation.

**§ 11(4).** Add complete-width numeric interpretation tests for the same register pattern `0xFF`: `uint8` gives `255`, `int8` gives `-1`, `uint16` gives `255`, and `int16` gives `-1`. Add register widths not equal to an ordinary storage carrier (for example `register[12]`) and test sign extension from **bit 11**, not bit 15 of its carrier.

**§ 11(5).** Add endian-independent protocol tests with the same `register[16] msb-first big-endian` bit pattern `0x1234`, serialized bytes `12 34` and equal decoded logical values on both big-endian and little-endian `CompilationPlan`s, in both communication directions. Treat this as a contract test of the **existing** byte encoder/decoder when one is available; it does not require introducing a new bytes API in MD-110. Explicit integer/register casts themselves must never insert a byte swap.

**§ 11(6).** Add Semantic IR tests for the exact three operation IDs, nominal type identity, source/result types, constant/source-domain/guarded proof paths, target-sized width resolution and pure effects. Missing proof, invalid proof, reversed checked branches and mismatched guarded SSA source must fail verification.

**§ 11(7).** Add schema-11 and schema-12 MLIR positive parse/print/verify round trips and negative version gate tests for schemas 1–10. Verify that `sec.register.from_bits` cannot accept a forged `source-domain` proof for an incompatible source type or a `guarded` proof without a dominating successful matching predicate. Verify `sec.register.to_bits` rejects result widths smaller than the register width.

**§ 11(8).** Verify `!sec.register<T>` conversions cannot take `!sec.hw_instance<T>` operands, cannot become ordinary `sec.place.read` or `sec.hw.observe`, do not acquire hardware effect metadata, and cannot be optimized into a native-endian memory copy.

**§ 11(9).** Test that existing `bit[N](value)` semantics, nested register field extraction/update, MD-077 hardware operation semantics and previous schema-11/12 regression inputs are unchanged. Emitters may enable only the operation subset fully supported by parsing, verification and lowering-capability checks; unsupported operations fail closed without publishing partial output.

## § 12. Controlled rulebook integration and governance

**§ 12(1).** Update `rules/declarations/registers.md` **within its existing § 12** to replace the misleading universal *numeric fit* wording with the exact bit-pattern representability rule in § 3, register-to-integer interpretation in § 5, and the endian separation in § 6. Add subordinate numbered paragraphs or an explicitly dated amendment without renumbering existing cited headings/paragraphs; preserve the existing statements about compile-time rejection, checked runtime conversion, nominal identity and absence of implicit endian conversion.

**§ 12(2).** Cross-reference `rules/types/types.md`'s normative MD-056 `bit[N](value)` rule, including its unsigned-carrier return type and source-width limits. Do **not** rewrite its unrelated extraction, units, enum or scalar conversion rules.

**§ 12(3).** Extend `rules/compiler/semantic_ir.md` § 21 and § 59 (and relevant verified conversion/failure sections) with § 7's exact operations, source and destination widths, signedness facts, proof and failure paths. Do not replace MD-077 `register.value.extract` / `register.value.update` or the hardware family.

**§ 12(4).** Add the § 8 representation as a clearly labelled, dated MD-110 extension to `rules/mlir/sec_mlir_dialect.md` v11.41.x and the matching `rules/mlir/dialect-versions/sec_mlir_dialect_v11.md` snapshot. Confirm schema-12 inheritance in the current main dialect and its versioned specification. Do not silently update unrelated historical schemas.

**§ 12(5).** Update relevant MLIR lowerings, operation registries, Go emitter, C++ verifiers, P15 package references and executable tests one narrowly scoped, tested step at a time; maintain the **single canonical spelling** of each operation. Any already existing operation with equivalent semantics in the *current* repository must be reused or reconciled by documented alias/migration, not duplicated under a second name.

**§ 12(6).** Reconcile only the **existing stable entries** in `governance/` implementation fragments. Preserve schema/version/area/purpose/integrations conventions and avoid duplicate implementation-status records. Legacy `implementation-status.yaml` is read-only. Update MD-110's `missing-decisions.yaml` entry only after its normative owners are synchronized and the appropriate acceptance checks pass; decision-finalized is not the same as fully implemented.

**§ 12(7).** Record the actual `main` HEAD used for integration and review it for semantic/name collisions **before changing files**. The previously inaccessible 2026-10-23 commit cannot be assumed unchanged. If a collision is discovered, the integrating agent must report the precise conflict instead of silently substituting an invented dialect convention or reopening a settled source-language question.

---

## § 13. Compact normative summary

**§ 13(1).** A `register[N]` is a nominal `N`-bit pattern **without inherent signedness**. Source integers supply explicitly interpreted bits; signed negative integers can supply legal all-one patterns. Conversion checks losslessness in the sense of § 3, not whether every source is a nonnegative number.

**§ 13(2).** Exact register construction does not silently truncate. Explicit `bit[N](x)` performs deliberate low-bit extraction. Register-to-integer conversion explicitly chooses signed or unsigned interpretation. Endian rules apply only at byte-sequence boundaries; A and B exchange the same externally declared `big-endian` bytes regardless of CPU-native order.

**§ 13(3).** Semantic IR uses `register.value.fits-bits`, `register.value.from-bits`, `register.value.to-bits`; Sec MLIR uses `sec.register.fits_bits`, `sec.register.from_bits`, `sec.register.to_bits`, under schema-11 early admission with schema-12 inheritance and strict proof/failure verification. These are detached pure value operations, never hardware-access operations.

**§ 13(4).** This closes the **design** portion of MD-110. It does not claim completion of source integration, compiler implementation, MLIR verification, endian-aware byte IO API design, or governance status.
