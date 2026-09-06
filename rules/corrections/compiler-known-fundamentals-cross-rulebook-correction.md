# Compiler-Known Fundamentals Cross-Rulebook Correction

- Status: Normative correction and synchronization package
- Created: 2026-09-06
- Last updated: 2026-09-06
- Sec language version: 0.1
- Repository baseline reviewed: `0f5027d`
- Primary durable authority after synchronization: `rules/compiler/compiler_known_members.md`
- Related rulebooks: `rules/library/core-library.md`, `rules/declarations/properties.md`, `rules/collections/shaped-types.md`, `rules/memory/layout.md`, `rules/compiler/compile_time_evaluation.md`
- Intended correction area: `rules/corrections/`
- Classification: Repository-wide governance and semantic correction for compiler-known fundamentals.

---

## § 1 Purpose

§ 1(1) This correction clarifies the normative boundary between:

```text
source-visible compiler-known Sec surface;
privileged core Sec declarations;
compiler-internal semantic structures;
ordinary user-defined members;
source visibility.
```

§ 1(2) It also corrects the canonical `SizeOf` surface and the replacement policy for compiler-provided `ToString()`.

§ 1(3) After synchronization, `rules/compiler/compiler_known_members.md` remains the durable registry/member authority.

---

## § 2 Compiler-space is not source visibility

§ 2(1) The concepts:

```text
compiler-facing
user-facing
```

are not encoded by `_` or `__`.

§ 2(2) `_` and `__` remain ordinary Sec source-language visibility mechanisms.

§ 2(3) Compiler-space versus userspace is a separate semantic authority boundary and must be stated explicitly in rulebooks and compiler metadata.

§ 2(4) A compiler-internal semantic structure that is not a Sec declaration does not participate in source visibility at all.

---

## § 3 Legitimate use of `_` and `__`

§ 3(1) A real Sec source declaration may use `_` or `__` according to ordinary visibility semantics.

§ 3(2) For example, a privileged core helper may be sourcefile-only because it is a real Sec helper declaration.

§ 3(3) Such a helper is not sourcefile-only merely because the compiler knows about it.

§ 3(4) Compiler-facing semantic metadata may reference a private or sourcefile-only declaration where the origin source legally resolved it, without granting new source visibility.

---

## § 4 Complete declaration governance

§ 4(1) Every compiler-known Sec declaration introduced or materially required by a normative rulebook must be specified in complete canonical Sec syntax.

§ 4(2) A complete declaration includes as applicable:

```text
canonical name;
owning module;
generic parameters;
variants or members;
fields;
parameter types;
return types;
error types;
modifiers;
property accessors;
source visibility;
compile-time/runtime availability;
target dependence.
```

§ 4(3) Names alone are not a complete normative declaration.

§ 4(4) A rulebook may satisfy § 4(1) by giving the complete declaration itself or by an explicit canonical cross-reference to the owning rulebook containing the complete declaration.

§ 4(5) Every compiler-known/core/library type transitively used by such a declaration must likewise be completely defined or canonically cross-referenced.

---

## § 5 Compiler-semantic structures are different

§ 5(1) A compiler algorithm, diagnostic record, evaluator request, dependency record, or similar compiler-only structure need not be a Sec declaration.

§ 5(2) Such a structure must not be documented using invented Sec syntax merely to make it look like a source type.

§ 5(3) When normative, its complete compiler-semantic structure and category set must instead be stated explicitly and marked:

```text
not a Sec declaration
```

§ 5(4) `compile_time_evaluation.md` follows this model for its evaluator request/outcome/failure/dependency/provenance structures.

---

## § 6 Compiler-known fundamental categories

§ 6(1) Compiler-known source members must be classified by semantic authority.

§ 6(2) At minimum the registry distinguishes:

```text
CompilerProvidedFallbackMember
AuthoritativeCompilerSemanticProperty
CompilerKnownOperation
PrivilegedCoreMember
OrdinaryUserMember
```

§ 6(3) These labels are compiler-semantic registry categories, not Sec enum declarations.

§ 6(4) The category determines replacement and conflict policy.

---

## § 7 Compiler-provided fallback members

§ 7(1) A compiler-provided fallback member supplies canonical behavior when no valid user-owned exact replacement exists.

§ 7(2) A fallback member is not representation truth that user code can corrupt.

§ 7(3) `ToString()` is the canonical Sec 0.1 example.

§ 7(4) Ordinary member lookup resolves a valid user-owned exact replacement before the compiler-provided fallback when the owning type rules permit replacement.

§ 7(5) The fallback and user replacement must not become ambiguous overload candidates merely because both semantic sources exist.

---

## § 8 Canonical universal `ToString()` member shape

§ 8(1) The canonical fundamental callable shape is:

```sec
fn ToString() string
```

§ 8(2) The declaration shape means:

```text
name:
    ToString

parameters:
    none

receiver:
    ordinary instance receiver according to the owning type/member rules

result:
    string

fallibility:
    no source-level Result/error channel in this canonical no-argument surface

semantic category:
    CompilerProvidedFallbackMember where the compiler provides the implementation
```

§ 8(3) `string` is the canonical Sec built-in string type defined by the core/type/string rulebooks.

§ 8(4) The compiler-provided universal member surface is synthesized/registered compiler-known Sec member semantics, not a source declaration that must physically reside in one core source module. Any actual core helper or type used to implement the fallback remains a real Sec declaration and must have a complete canonical declaration and owning module.

§ 8(5) The actual default textual representation for each concrete type family is owned by the compiler-known/core/formatting rules and must be fully specified before claiming support for that family.

---

## § 9 User-defined `ToString()` replacement

§ 9(1) A user-owned nominal type may define the exact canonical replacement:

```sec
type Customer struct {
    Name: string
}

impl Customer {
    fn ToString() string {
        return self.Name
    }
}
```

§ 9(2) For `Customer`, the explicit user-defined `ToString() string` replaces the compiler-provided fallback for ordinary member resolution and formatting semantics.

§ 9(3) The replacement remains an ordinary Sec function body subject to ordinary:

```text
typing;
ownership;
borrowing;
effects;
CTE eligibility;
lowering;
diagnostics.
```

§ 9(4) A differently shaped overload does not replace the no-argument canonical fallback merely because it is named `ToString`.

§ 9(5) Ordinary user code may not attach implementations to compiler-owned built-in types where core/member rules forbid such extension.

---

## § 10 `ToString()` availability audit

§ 10(1) Synchronize `compiler_known_members.md` with the already locked Sec design that every Sec type which can produce an ordinary value has the compiler-known fundamental `ToString() string` member surface rather than limiting that surface to the currently enumerated primitive printable subset.

§ 10(2) This universal member-surface rule does not imply one universal formatting algorithm. For each concrete type family, the owning rulebook/core surface must define or cross-reference the canonical fallback semantics.

§ 10(3) A type family must not be marked implementation-complete merely because the registry recognizes the name if its actual fallback formatting semantics are undefined.

§ 10(4) User-defined exact replacement remains governed by § 9.

---

## § 11 Authoritative compiler semantic properties

§ 11(1) An authoritative compiler semantic property exposes a canonical semantic/compiler fact.

§ 11(2) User code must not replace or redefine such a property on a receiver category where the canonical property applies.

§ 11(3) The property must have one stable compiler/member identity per semantic surface.

§ 11(4) `SizeOf` is the canonical representation/layout example.

§ 11(5) Shaped properties such as rank/shape/layout facts remain authoritative where their owning rulebooks define them.

---

## § 12 Canonical `SizeOf` source surface

§ 12(1) Sec 0.1 uses two property source forms:

```sec
let valueSize: uint := value.SizeOf
let typeSize: uint := TypeName.SizeOf
```

§ 12(2) The global form:

```sec
SizeOf(TypeName)
```

is removed from the canonical Sec 0.1 source surface.

§ 12(3) It adds no semantic capability because it returns the same owning layout fact as `TypeName.SizeOf`.

§ 12(4) Parser, Sema, compiler-known registry, formatter, LSP, docs, tests, and implementation status must be synchronized to the two-property model.

---

## § 13 Canonical `SizeOf` property shapes

§ 13(1) The compiler-known semantic member shapes are equivalent to the read-only property contracts:

```sec
property SizeOf: uint {
    get
}
```

for a value receiver, and:

```sec
static property SizeOf: uint {
    get
}
```

for a type-associated receiver.

§ 13(2) These blocks specify the canonical Sec property shape used by the compiler-known registry.

§ 13(3) They are not user declarations that must literally be written inside every built-in type.

§ 13(4) Property syntax and getter semantics are owned by `rules/declarations/properties.md`.

§ 13(5) `uint` is the canonical Sec target-sized unsigned integer type defined by `rules/types/types.md`.

---

## § 14 Value-form `SizeOf`

§ 14(1) Source form:

```sec
value.SizeOf
```

§ 14(2) The result type is:

```sec
uint
```

§ 14(3) The semantic result is determined by the exact receiver category and owning `compiler_known_members.md`/layout rule.

§ 14(4) Receiver evaluation follows ordinary Sec evaluation/effect/ownership rules.

§ 14(5) The compiler-known property result does not by itself permit the compiler to erase observable receiver effects.

§ 14(6) User code must not override canonical `SizeOf` on a receiver where the authoritative property applies.

---

## § 15 Type-form `SizeOf`

§ 15(1) Source form:

```sec
TypeName.SizeOf
```

§ 15(2) The result type is:

```sec
uint
```

§ 15(3) It has no value receiver.

§ 15(4) It is available only where the concrete semantic type has complete size semantics under the active `CompilationPlan`.

§ 15(5) Its target-dependent value follows `memory/layout.md`.

§ 15(6) It does not change or reveal a different Sec semantic type merely because representation size differs between targets.

---

## § 16 Shaped `SizeOf`

§ 16(1) Preserve the shaped-type owning distinction already defined by `collections/shaped-types.md`.

§ 16(2) A shaped instance `value.SizeOf` follows the shaped rule's logical represented payload-byte semantics.

§ 16(3) `TypeName.SizeOf` remains the owning type/layout query for the complete concrete type representation where that type form is defined.

§ 16(4) Removing global `SizeOf(TypeName)` does not change the shaped instance semantics.

---

## § 17 `TypeOf` is not introduced

§ 17(1) This correction does not introduce `.TypeOf`, `TypeOf(...)`, `Type`, `TypeInfo`, or another first-class type-information API.

§ 17(2) If a future `TypeOf`-like property is introduced, it must identify Sec semantic type identity rather than silently mean target representation.

§ 17(3) Such a future facility requires its own complete declaration/type contract and concrete use case.

---

## § 18 Compiler-known facts are not generic reflection

§ 18(1) The existence of individually designed properties such as `SizeOf`, rank, shape, or layout does not expose the compiler type/declaration graph as general reflection.

§ 18(2) Compiler-known facts are added when an owning language/library use case justifies them.

§ 18(3) General-purpose structural reflection and declaration metaprogramming remain outside Sec 0.1 unless separately designed.

---

## § 19 Generic lookup

§ 19(1) Compiler-known fundamental availability in generic templates remains constrained by the generic contract.

§ 19(2) A compiler-known member's existence on some concrete types does not allow an unconstrained template to use that member unless the generic/member rules guarantee its availability.

§ 19(3) CTE does not provide an alternate path for discovering undeclared concrete capabilities.

---

## § 20 Core boundary

§ 20(1) A compiler-known member may be implemented:

```text
directly by compiler semantics;
by a privileged core declaration/helper;
by a combination of compiler semantic identity and core implementation.
```

§ 20(2) When an actual privileged core Sec declaration exists, its full canonical declaration and owning module must be documented according to § 4.

§ 20(3) The compiler-known registry must not depend on accidental source spelling alone.

§ 20(4) `library/core-library.md` and `compiler/compiler_known_members.md` must agree on all privileged core declarations implementing compiler-known surface.

---

## § 21 Audit of incomplete compiler-known declarations

§ 21(1) Every current rulebook that declares a compiler-known Sec type or source member only by name, prose shape, or partial pseudodeclaration must be audited.

§ 21(2) For each such item, the owning rulebook must choose exactly one:

```text
A. It is a real Sec declaration:
   provide the complete canonical Sec declaration and owning module.

B. It is compiler-semantic only:
   state explicitly that it is not a Sec declaration and provide the complete
   compiler-semantic structural contract.
```

§ 21(3) The audit applies particularly to rulebooks for shaped/storage/compiler-known metadata but is repository-wide.

§ 21(4) This correction intentionally does not invent missing declarations whose exact Sec design has not yet been locked.

---

## § 22 Registry synchronization

§ 22(1) Update compiler-known registry entries so semantic categories and replacement policy are explicit.

§ 22(2) At minimum:

```text
ToString
    category: CompilerProvidedFallbackMember
    canonical shape: fn ToString() string
    exact user replacement: permitted on eligible user-owned nominal types

SizeOf instance
    category: AuthoritativeCompilerSemanticProperty
    canonical shape: property SizeOf: uint { get }
    user replacement: forbidden where canonical property applies

SizeOf type-associated
    category: AuthoritativeCompilerSemanticProperty
    canonical shape: static property SizeOf: uint { get }
    user replacement: forbidden where canonical property applies
```

§ 22(3) The registry must remove the canonical global `SizeOf(TypeName)` entry.

---

## § 23 Lookup-order correction

§ 23(1) The existing generic lookup order must be interpreted together with member-category policy.

§ 23(2) An exact user member may replace a compiler fallback only when the registry category permits replacement.

§ 23(3) An exact user member must not shadow an authoritative semantic property on a receiver category where that property is reserved.

§ 23(4) This prevents the general lookup order from accidentally making layout truth user-overridable.

---

## § 24 Tooling

§ 24(1) LSP must expose the same canonical compiler-known semantic surface used by Sema.

§ 24(2) Completion/hover/navigation must show property syntax for the two `SizeOf` forms.

§ 24(3) Tooling must not advertise global `SizeOf(TypeName)` after the correction is merged.

§ 24(4) Tooling should identify whether a compiler-known member is:

```text
fallback;
authoritative;
privileged-core-backed;
ordinary user replacement.
```

where such information improves diagnostics/hover.

---

## § 25 Diagnostics

§ 25(1) Invalid attempts to redeclare/override an authoritative compiler semantic property should explain the semantic category.

§ 25(2) Example diagnostic shape:

```text
SizeOf is an authoritative compiler-known property for this type
and cannot be replaced by a user-defined member
```

§ 25(3) A valid exact `ToString() string` on an eligible user-owned nominal type must not produce an ambiguity diagnostic merely because a compiler fallback exists.

§ 25(4) An invalid replacement shape should be diagnosed as an ordinary conflicting/overloaded member according to member rules rather than silently replacing the fallback.

---

## § 26 Required tests

§ 26(1) Required regression coverage includes:

```text
user ToString exact replacement wins over fallback;
different ToString overload does not replace canonical fallback;
compiler-owned built-in type cannot be extended illegally;
value.SizeOf resolves as authoritative property;
TypeName.SizeOf resolves as authoritative static property;
global SizeOf(TypeName) is rejected after migration;
user SizeOf override is rejected where authoritative property applies;
shaped value.SizeOf retains shaped payload semantics;
type SizeOf remains target-plan dependent;
named types preserve nominal layout semantics;
compiler-space categories do not require _ or __ spelling;
real _ / __ source declarations retain ordinary visibility;
compiler metadata does not leak private/sourcefile-only source accessibility;
incomplete compiler-known declarations are rejected by documentation/governance audit.
```

---

## § 27 Cross-rulebook synchronization targets

§ 27(1) Synchronize at least:

```text
rules/compiler/compiler_known_members.md
rules/library/core-library.md
rules/declarations/properties.md
rules/collections/shaped-types.md
rules/memory/layout.md
rules/declarations/generics.md
rules/compiler/compile_time_evaluation.md
rules/tooling/lsp.md
language-rulebook-status.md
implementation-status.yaml
manual/compiler-known-member documentation
compiler/LSP registry tests
```

§ 27(2) Additional rulebooks discovered to contain incomplete compiler-known declarations must be added to the audit rather than ignored.

---

## § 28 Superseding principles

§ 28(1) The canonical compiler-space rule is:

> Compiler-facing versus user-facing is a semantic authority boundary, not an underscore naming convention.

§ 28(2) The canonical declaration rule is:

> A compiler-known Sec declaration must exist normatively in complete canonical form; compiler-only structures must be explicitly documented as compiler-only instead of being represented by incomplete pseudo-Sec declarations.

§ 28(3) The canonical fundamental-member rule is:

> User-defined behavior may replace a compiler-provided fallback only where the registry explicitly permits replacement; authoritative semantic facts remain compiler-owned.

§ 28(4) The canonical `SizeOf` rule is:

```sec
value.SizeOf
TypeName.SizeOf
```

and no canonical global `SizeOf(TypeName)` form exists in Sec 0.1.
