# Diagnostics, Mentor Analysis and Localization

- **Status:** Normative
- **Created:** Legacy rulebook; exact original creation date not established
- **Last updated:** 2026-09-30
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/tooling/diagnostics.md`
- **Replaces:** `rules/tooling/diagnostics.txt`
- **Repository baseline reviewed:** `main-reviewed-2026-09-30`
- **Implementation governance:** `governance/errors_diagnostics.yaml` (`errors.diagnostics`)
- **Related rulebooks:** `rules/compiler/compiler_testing.md`, `rules/compiler/parser_recovery.md`, `rules/tooling/lsp.md`, `rules/tooling/formatter.md`, `rules/tooling/testing.md`, `rules/control-flow/discard.md`, `rules/errors/errorhandling.md`, `rules/errors/runtime_checks.md`, `rules/errors/panic.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/destruction.md`, `rules/analysis/effect_analysis.md`, `rules/analysis/escape_analysis.md`, `rules/analysis/call_graph.md`, `rules/analysis/parameter_usage_analysis.md`, `rules/analysis/pitfall_analysis.md`, `rules/platform/platform_model.md`, `rules/foundations/attributes.md`

---

## § 1. Purpose and authority

**Governance tags:** `errors.diagnostics`

§ 1(1) This rulebook is the canonical owner of Sec's programmer-visible diagnostic model.

§ 1(2) It owns:
- stable diagnostic IDs;
- stable symbolic diagnostic names;
- ID namespaces;
- diagnostic topic families;
- default and effective severity;
- mandatory versus configurable classification;
- structured diagnostic definitions;
- structured diagnostic occurrences;
- related locations, notes, help and fixes;
- localization and message catalogs;
- detailed diagnostic lookup;
- project and command-line diagnostic policy;
- machine-readable diagnostic identity;
- LSP diagnostic identity;
- mentor-diagnostic evidence and uncertainty rules.

§ 1(3) A rulebook that owns a language or tool rule owns the condition that triggers a diagnostic. This rulebook owns the identity, classification, transport and presentation contract of that diagnostic.

§ 1(4) Parser recovery behavior is owned by `rules/compiler/parser_recovery.md`. Formatter behavior is owned by `rules/tooling/formatter.md`. Explicit discard semantics are owned by `rules/control-flow/discard.md`. This rulebook must not redefine those semantics.

§ 1(5) Mutable implementation status, migration progress and known implementation bugs are not normative content. They are tracked in `governance/errors_diagnostics.yaml`.

§ 1(6) Revision 2.0 supersedes earlier guidance that:
- used `sec explain`;
- treated numeric subranges as a semantic taxonomy;
- treated `P` as also owning lexer diagnostics;
- implied that every `S` diagnostic must be mandatory;
- left a source-level suppression syntax to be invented during implementation;
- described mutable implementation phases inside the normative rulebook.

---

## § 2. Design principles

**Governance tags:** `errors.diagnostics`

§ 2(1) Invalid Sec programs, invalid projects and invalid requested builds must be rejected.

§ 2(2) Statically proven unreachable code and statically proven dead code are errors when the owning semantic rule defines them as invalid.

§ 2(3) A likely mistake that remains semantically valid may be a warning.

§ 2(4) A possible improvement that requires programmer judgement should normally be information.

§ 2(5) Safety and correctness rules must not be configurable away.

§ 2(6) Every emitted primary diagnostic must have a stable registered identity.

§ 2(7) Diagnostic wording, localization and rendering may improve without changing diagnostic identity.

§ 2(8) Diagnostics should explain:
- what is wrong or noteworthy;
- why it matters;
- the earlier source action or declaration that caused the current state when relevant;
- a concrete valid correction when one is known.

§ 2(9) The compiler may act as a mentor, but must distinguish proven facts from heuristic advice and must not present uncertain analysis as certainty.

§ 2(10) The same semantic diagnostic identity must be used by CLI output, LSP output, tests and machine-readable output.

---

## § 3. Core terminology

**Governance tags:** `errors.diagnostics`

§ 3(1) A **diagnostic definition** is the stable registry entry describing one programmer-visible diagnostic rule.

§ 3(2) A **diagnostic occurrence** is one concrete emission of a diagnostic definition for one compilation, analysis or tool invocation.

§ 3(3) A **primary diagnostic** is one independently actionable error, warning or informational finding.

§ 3(4) A **related location** points to source that materially explains the primary occurrence, such as:
- the earlier move that made a value unavailable;
- the original declaration in a duplicate-name error;
- the covering arm that makes a later arm unreachable;
- the borrow that conflicts with an attempted mutation.

§ 3(5) A **note** adds explanatory context to a primary diagnostic. It is not an independent severity and normally has no diagnostic ID.

§ 3(6) **Help** describes a valid correction, alternative or next action. It is not an independent severity and normally has no diagnostic ID.

§ 3(7) A **fix** is structured source-edit information attached to a primary diagnostic.

§ 3(8) An **ID namespace** is the leading letter of a stable diagnostic ID.

§ 3(9) A **topic family** is the stable registry metadata that groups a diagnostic by semantic subject, such as `lexer`, `parser`, `names`, `operators`, `ownership`, `types`, `performance` or `control-flow`.

§ 3(10) ID namespace and topic family are distinct concepts. The namespace is part of the permanent diagnostic ID. The topic family is registry metadata and must not be inferred from the numeric portion of the ID.

---

## § 4. Severity and policy

**Governance tags:** `errors.diagnostics`

§ 4(1) The canonical diagnostic severities are:

```text
Error
Warning
Information
```

§ 4(2) `Error` means that the affected source, project, requested compilation or requested build has no valid meaning under the applicable Sec rules.

§ 4(3) `Warning` means that the code or project remains valid, but the compiler has strong evidence of a likely unintended result.

§ 4(4) `Information` means that the code or project remains valid and the compiler has identified a possible improvement for which programmer judgement is required.

§ 4(5) The canonical configurable policy states are:

```text
Off
Information
Warning
Error
```

§ 4(6) Every active diagnostic definition declares:
- a default severity;
- whether it is mandatory.

§ 4(7) `Mandatory = true` requires:
- default severity `Error`;
- effective severity `Error`;
- rejection of attempts to disable or demote the diagnostic.

§ 4(8) Namespace alone does not determine severity or configurability.

§ 4(9) In particular, a published `S` diagnostic may be configurable when its existing stable definition says so. `S1014` / `switch.incomplete-enum-coverage` is such a published diagnostic and remains a warning by default.

§ 4(10) New diagnostics that are purely mentor/advisory findings should normally use the `A` namespace, but published IDs must never be renumbered merely to satisfy that convention.

§ 4(11) Notes, help and fixes inherit the occurrence they are attached to and do not receive independent severity.

---

## § 5. Stable diagnostic IDs

**Governance tags:** `errors.diagnostics`

§ 5(1) Every primary diagnostic has exactly one stable diagnostic ID.

§ 5(2) Sec 0.1 diagnostic IDs use one uppercase ASCII namespace letter followed by exactly four decimal digits.

```text
L1001
P2001
S1005
S3001
A2001
B1104
F1002
```

§ 5(3) The initial namespaces are:

```text
L
    lexical analysis

P
    parsing and parser recovery

S
    semantic language and compiler-analysis rules

A
    advisory and mentor analysis

B
    project, manifest, target, variant and build rules

F
    formatter and canonical-source rules
```

§ 5(4) A future namespace requires an explicit normative extension. An implementation must not invent a new namespace merely because no convenient existing prefix appears available.

§ 5(5) Each namespace owns one four-digit numeric space.

§ 5(6) The numeric portion is an opaque stable allocation. Revision 2.0 defines no semantic numeric subranges.

§ 5(7) Implementations must not infer topic, severity, owning compiler pass or configurability from the numeric portion of an ID.

§ 5(8) New IDs are allocated through the central diagnostic registry after checking every active and retired ID.

§ 5(9) Once published, an ID must never be reused for another programmer-visible rule.

§ 5(10) When a diagnostic is removed, its ID remains registered as retired.

§ 5(11) A diagnostic keeps its ID when only the following change:
- English wording;
- translated wording;
- source highlighting quality;
- notes;
- help;
- extended explanation;
- fix quality;
- internal compiler pass that discovers the problem;
- internal implementation type used to transport the occurrence.

§ 5(12) A new ID is required when the programmer-visible rule itself becomes materially different.

§ 5(13) Internal compiler failures are not ordinary source diagnostics and must use a distinct internal-error representation rather than consuming a normal diagnostic ID.

---

## § 6. Symbolic names and topic families

**Governance tags:** `errors.diagnostics`

§ 6(1) Every diagnostic ID has one stable symbolic name.

§ 6(2) Symbolic names are lowercase, dot-separated names. Segments may contain lowercase ASCII letters, decimal digits and `-`.

Examples:

```text
control-flow.unreachable-statement
ownership.non-discardable-value
performance.large-value-parameter
operator.invalid-shift-count
```

§ 6(3) Symbolic names are stable identifiers, not localized text.

§ 6(4) An active symbolic name must map to exactly one diagnostic ID.

§ 6(5) A retired symbolic name remains reserved together with its retired ID.

§ 6(6) Configuration may identify a rule by stable ID or stable symbolic name.

§ 6(7) Normal terminal rendering must include the stable ID. Verbose rendering may additionally include the symbolic name.

Example:

```text
information[A2001 performance.large-value-parameter]:
    parameter "frame" could use a shared reference
```

§ 6(8) Every definition also has one topic-family string.

§ 6(9) Topic family is registry metadata and is not the same field as ID namespace.

§ 6(10) Changing a topic family must not silently change an ID or symbolic name.

---

## § 7. Canonical logical definition schema

**Governance tags:** `errors.diagnostics`

§ 7(1) The canonical diagnostic model is a logical compiler/tooling contract. The following schema names do not introduce public Sec source-language types.

§ 7(2) An implementation may use different host-language structures internally, but exported catalog data and emitted occurrences must preserve the semantics defined here.

§ 7(3) The logical definition schema is:

```text
DiagnosticDefinition {
    ID: DiagnosticID
    Name: string
    Namespace: DiagnosticNamespace
    Family: string
    Category: optional string
    DefaultSeverity: DiagnosticSeverity
    Mandatory: bool
    Retired: bool
    Arguments: ordered list<DiagnosticArgumentDefinition>
    ShortMessageKey: optional string
    ExplanationKey: optional string
    RelatedIDs: ordered list<DiagnosticID>
}
```

§ 7(4) `DiagnosticID` is the stable identity defined by § 5.

§ 7(5) `DiagnosticNamespace` is one of the namespaces defined by § 5.

§ 7(6) `Family` is the topic-family string defined by § 6.

§ 7(7) `Category` is an optional policy grouping primarily used by configurable diagnostics.

§ 7(8) Active definitions that are emitted to users must eventually provide both:
- an English short-message key;
- an extended-explanation key.

During migration, absence of those keys is implementation debt tracked in governance; it does not permit emission of an unregistered primary diagnostic.

§ 7(9) Retired definitions remain in the registry but are not emitted for new source.

§ 7(10) The canonical argument-definition schema is:

```text
DiagnosticArgumentDefinition {
    Name: string
    Kind: DiagnosticArgumentKind
    Required: bool
    Description: string
}
```

§ 7(11) `DiagnosticArgumentKind` has the initial values:

```text
Text
Identifier
Type
Value
Integer
Boolean
Path
Symbol
Token
Target
Variant
Capability
```

§ 7(12) A future argument kind may be added without changing an existing diagnostic ID when the programmer-visible rule remains the same and existing machine consumers can identify the new kind explicitly.

§ 7(13) A definition must not use unstructured free-form text where a stable named argument can represent the same semantic fact.

---

## § 8. Canonical logical occurrence schema

**Governance tags:** `errors.diagnostics`

§ 8(1) A diagnostic occurrence is structured data. English display text is a rendering of that data, not the canonical semantic representation.

§ 8(2) The logical occurrence schema is:

```text
DiagnosticOccurrence {
    ID: DiagnosticID
    Name: string
    Severity: DiagnosticSeverity
    Arguments: map<string, DiagnosticArgumentValue>
    Primary: optional DiagnosticLocation
    Related: ordered list<RelatedLocation>
    Notes: ordered list<DiagnosticMessage>
    Help: ordered list<DiagnosticMessage>
    Fixes: ordered list<DiagnosticFix>
}
```

§ 8(3) `Name` must match the registered symbolic name for `ID`.

§ 8(4) `Severity` is the effective severity after applying valid diagnostic policy.

§ 8(5) Every argument required by the registered definition must be present.

§ 8(6) An occurrence must not contain an argument name unknown to the registered definition.

§ 8(7) `DiagnosticArgumentValue` is:

```text
DiagnosticArgumentValue =
    Text(string)
    Integer(string)
    Boolean(bool)
```

§ 8(8) `Integer(string)` contains the exact canonical base-10 representation and is not limited by host `int64` or JSON safe-integer ranges.

§ 8(9) The argument definition's `Kind` gives semantic meaning to the underlying occurrence value. For example, a `Type` argument is transported as exact compiler display text but remains distinguishable from ordinary free-form `Text`.

§ 8(10) Source identifiers, type displays, values, paths, symbols, target names and variant names are compiler data and must not be translated by localization.

---

## § 9. Source locations

**Governance tags:** `errors.diagnostics`, `tooling.lsp`

§ 9(1) The logical source-location types are:

```text
SourcePosition {
    Line: unsigned integer
    Column: unsigned integer
}

SourceSpan {
    File: string
    Start: SourcePosition
    End: SourcePosition
}

DiagnosticLocation {
    Span: SourceSpan
}

RelatedLocation {
    Span: SourceSpan
    Message: optional DiagnosticMessage
}
```

§ 9(2) CLI/compiler source coordinates are 1-based.

§ 9(3) `End` is exclusive.

§ 9(4) LSP integration converts canonical compiler locations into the coordinate representation required by the active LSP protocol. That conversion must not change diagnostic identity.

§ 9(5) A source diagnostic has one primary source location.

§ 9(6) A project, build or target diagnostic may lack a source span when no source or manifest location exists.

§ 9(7) When a diagnostic depends on an earlier source event, the occurrence should include that event as a related location rather than forcing the programmer to reconstruct it from prose.

---

## § 10. Notes, help and messages

**Governance tags:** `errors.diagnostics`

§ 10(1) The logical localized-message reference is:

```text
DiagnosticMessage {
    Key: string
    Arguments: map<string, DiagnosticArgumentValue>
}
```

§ 10(2) Notes explain relevant facts but do not replace the primary message.

§ 10(3) Help describes a correction or next action that is valid for the resolved program state.

§ 10(4) The compiler must not suggest a repair that is invalid for the resolved types, ownership state, borrow state, target or effect contract.

§ 10(5) When multiple safe corrections exist, help may present more than one alternative.

§ 10(6) Compiler-theory terminology may appear as secondary detail, but the primary explanation should describe the programmer-visible problem.

---

## § 11. Fixes and semantic code actions

**Governance tags:** `errors.diagnostics`, `tooling.formatter-v2`

§ 11(1) The canonical fix applicability values are:

```text
Automatic
ExplicitSafe
Suggestion
```

§ 11(2) The logical schemas are:

```text
TextEdit {
    Span: SourceSpan
    Replacement: string
}

DiagnosticFix {
    Title: DiagnosticMessage
    Applicability: FixApplicability
    Edits: ordered list<TextEdit>
}
```

§ 11(3) `Automatic` means the transformation is proven semantics-preserving and may be applied without an additional semantic decision by the programmer.

§ 11(4) `ExplicitSafe` means equivalence or safety has been proven, but the transformation changes source in a way that requires an explicit user action.

§ 11(5) `Suggestion` means programmer judgement is required. It must never be applied automatically.

§ 11(6) `Automatic` and `ExplicitSafe` fixes must contain at least one concrete edit.

§ 11(7) A `Suggestion` may omit edits when the correct change depends on programmer intent.

§ 11(8) Formatter canonicalization is not redefined by this rulebook.

§ 11(9) API-changing advice, including changing an owning parameter to `ref T` or `ref mut T`, is not unconditional formatter behavior. It is a mentor diagnostic or explicit semantic code action.

---

## § 12. Diagnostic registry

**Governance tags:** `errors.diagnostics`

§ 12(1) Every primary diagnostic definition is declared exactly once in the central compiler registry.

§ 12(2) No conforming compiler path may emit an unregistered primary diagnostic.

§ 12(3) Migration fallback diagnostics may exist only where an explicitly registered fallback ID owns that compatibility role.

§ 12(4) The registry must reject:
- duplicate active IDs;
- reuse of retired IDs;
- duplicate active symbolic names;
- invalid namespace syntax;
- invalid severity;
- mandatory definitions whose default severity is not `Error`;
- active definitions with inconsistent required argument schemas.

§ 12(5) Registry ordering exposed to tools is stable ID order.

§ 12(6) The registry is the canonical source for whether an ID is active or retired.

§ 12(7) Documentation, LSP metadata, compiler tests, translation catalogs and diagnostic catalog commands should consume the registry rather than maintain parallel identity tables.

---

## § 13. Canonical diagnostic commands

**Governance tags:** `errors.diagnostics`, `testing.compiler-verification-v1`

§ 13(1) The canonical inspection commands are:

```text
sec diagnostics
sec diagnostics --json
sec diagnostics <ID>
```

§ 13(2) `sec diagnostics` lists every registered definition in stable ID order.

§ 13(3) The human-readable listing includes at least:
- ID;
- symbolic name;
- topic family;
- default severity;
- mandatory/configurable status;
- retired status.

§ 13(4) `sec diagnostics --json` emits the corresponding machine-readable registry/catalog representation, including the field schemas exported by the compiler.

§ 13(5) `sec diagnostics <ID>` provides detailed information for one diagnostic identity.

§ 13(6) Detailed lookup includes, when applicable:
- ID;
- symbolic name;
- namespace;
- topic family;
- category;
- default severity;
- mandatory/configurable status;
- retired status;
- short description;
- extended explanation;
- common causes;
- valid alternatives;
- examples;
- related diagnostic IDs;
- relevant language-version information.

§ 13(7) `sec diagnostics <ID>` is the canonical replacement for the obsolete `sec explain <ID>` design.

§ 13(8) `sec explain` is not part of the Sec 0.1 canonical diagnostics interface.

§ 13(9) An unknown ID must produce a normal tool error and must not be confused with a compiler source diagnostic for the inspected project.

---

## § 14. Machine-readable emitted occurrences

**Governance tags:** `errors.diagnostics`

§ 14(1) Diagnostic-producing compiler commands may expose machine-readable emitted occurrences using their defined diagnostic-output option.

Example:

```text
sec check shop --diagnostic-format json
```

§ 14(2) Machine-readable occurrence output uses the logical model in §§ 8–11.

§ 14(3) At minimum, an emitted occurrence exposes:
- ID;
- symbolic name;
- effective severity;
- structured arguments;
- primary location when one exists;
- related locations;
- notes;
- help;
- structured fixes.

§ 14(4) Localized display text may additionally be included, but machine consumers must use stable IDs and structured fields as the compatibility interface.

§ 14(5) Catalog JSON from `sec diagnostics --json` and emitted-occurrence JSON are different surfaces:
- the catalog describes definitions and schemas;
- occurrence output describes findings from one invocation.

§ 14(6) The two surfaces must not silently use incompatible identities for the same diagnostic.

---

## § 15. LSP identity

**Governance tags:** `errors.diagnostics`, `tooling.lsp`

§ 15(1) LSP diagnostics use:

```text
source = "sec"
code   = <stable diagnostic ID>
```

§ 15(2) The LSP code must be the same ID used by CLI output and machine-readable output.

§ 15(3) LSP rendering may combine the short message with notes or help according to LSP presentation constraints, but such rendering must not alter the underlying occurrence.

§ 15(4) LSP quick fixes and code actions use the fix applicability contract in § 11.

§ 15(5) Parser recovery metadata may be transported to LSP, but recovery episodes and parser synchronization remain owned by `rules/compiler/parser_recovery.md`.

---

## § 16. Localization

**Governance tags:** `errors.diagnostics`

§ 16(1) Localization changes presentation, not compiler semantics.

§ 16(2) Localization must not change:
- ID;
- symbolic name;
- severity;
- mandatory/configurable classification;
- source locations;
- structured semantic arguments;
- build result.

§ 16(3) English is the canonical fallback language.

§ 16(4) Localized catalogs are keyed by stable message keys owned by registered diagnostic definitions.

§ 16(5) Templates use named arguments.

§ 16(6) Translation validation must reject:
- an unknown diagnostic/message key;
- a missing required named argument;
- an unknown named argument where the template schema does not permit it;
- malformed templates.

§ 16(7) Language selection is a user-interface preference, not source semantics.

§ 16(8) The preference order is:

```text
1. explicit command-line language
2. user configuration
3. process locale
4. project recommendation when present
5. English fallback
```

§ 16(9) A project may recommend a diagnostic language but must not prevent an individual developer from selecting another language.

---

## § 17. Short messages and extended explanations

**Governance tags:** `errors.diagnostics`

§ 17(1) Normal terminal output should be concise.

Example:

```text
error[S3001]: unreachable statement
```

§ 17(2) Extended rendering may include source, reason, related locations, notes and help.

§ 17(3) Extended explanation is retrieved through:

```text
sec diagnostics S3001
```

§ 17(4) Extended explanations may be localized independently from short messages.

§ 17(5) Tests of semantic diagnostics should not depend on exact English prose unless the test specifically validates the message renderer or localization catalog.

---

## § 18. Diagnostic configuration

**Governance tags:** `errors.diagnostics`

§ 18(1) Diagnostic policy applies only to configurable definitions.

§ 18(2) A mandatory diagnostic cannot be:
- disabled;
- demoted;
- converted to warning;
- converted to information.

§ 18(3) Configurable diagnostics may resolve to:

```text
off
info
warning
error
```

§ 18(4) Rule-specific policy may identify the diagnostic by stable ID or symbolic name.

Example:

```toml
[diagnostics.rules]
"performance.large-value-parameter" = "warning"
"A2004" = "off"
```

§ 18(5) Advisory categories initially include:

```text
suspicious
performance
ownership
portability
maintainability
style
deprecated
```

§ 18(6) Category names are policy groupings, not diagnostic identities.

§ 18(7) Rule-specific configuration overrides category configuration.

§ 18(8) The standard presets are:

```text
minimal
    mandatory errors only

default
    mandatory errors, default warnings and selected high-value information

mentor
    mandatory errors, warnings and all implemented information diagnostics

strict
    mandatory errors, warnings promoted to errors and selected information
    promoted to warnings
```

§ 18(9) Diagnostic policy may be configured at:
- project level;
- target level;
- target-variant level;
- command-line level.

§ 18(10) Policy precedence is:

```text
1. command-line rule override
2. target-variant rule override
3. target rule override
4. project rule override
5. command-line category override
6. target-variant category override
7. target category override
8. project category override
9. selected preset
10. diagnostic default
```

§ 18(11) User configuration may choose language, detail and a fallback preset when project policy does not define one.

§ 18(12) Project diagnostic policy that affects build success must remain reproducible in CI.

---

## § 19. Source-level suppression in Sec 0.1

**Governance tags:** `errors.diagnostics`, `foundations.attributes`

§ 19(1) This revision defines no source-level diagnostic suppression syntax.

§ 19(2) Therefore Sec 0.1 source code cannot suppress a diagnostic merely by:
- underscore-prefixing an identifier;
- writing a comment;
- using an undeclared attribute;
- using `_` outside one of its separately defined grammar roles.

§ 19(3) Configurable advisory diagnostics are controlled through the policy surfaces defined in § 18.

§ 19(4) Mandatory diagnostics remain unsuppressible.

§ 19(5) A future source-level suppression feature requires a separate normative rule defining syntax, scope, interaction with mandatory diagnostics, tooling visibility and auditability. An implementation must not invent such syntax from this rulebook.

---

## § 20. Underscore and discard

**Governance tags:** `errors.diagnostics`

§ 20(1) Context-specific meanings of `_` are defined by their owning rulebooks and are not diagnostic suppression.

§ 20(2) Examples include:
- reserved register fields;
- match discard patterns;
- loop discard binding positions;
- other explicitly defined pattern discard positions.

§ 20(3) Identifiers beginning with `_` or `__` remain real identifiers where visibility rules define them as such. Their prefix does not suppress unused or dead-code analysis.

§ 20(4) General value discard is defined by `rules/control-flow/discard.md`.

§ 20(5) The canonical explicit discard statement is:

```sec
discard expression
```

§ 20(6) This rulebook must not reinterpret `_` as an alternative to `discard`.

---

## § 21. Proven unreachable and dead code

**Governance tags:** `errors.diagnostics`, `analysis.effect-analysis`, `analysis.escape-analysis`

§ 21(1) A diagnostic may classify source as dead only when the owning semantic analysis proves that it cannot affect any valid execution represented by the analyzed CompilationPlan.

§ 21(2) The compiler distinguishes:

```text
proven invalid or proven dead
    Error when the owning language rule makes it invalid

semantically valid but strongly suspicious
    Warning

possible improvement
    Information

not proven
    no dead-code diagnostic
```

§ 21(3) Statements after an unconditional transfer in the same reachable block are invalid when execution cannot reach them.

§ 21(4) `S3001` is permanently allocated as:

```text
S3001
control-flow.unreachable-statement
mandatory Error
```

§ 21(5) `S3001` must identify the unreachable statement and explain the control-flow fact that makes it unreachable.

§ 21(6) A branch or pattern arm proven impossible is invalid when its owning language rule requires reachability.

§ 21(7) A dead-store diagnostic requires proof that removing the store does not alter:
- evaluation effects;
- property behavior;
- ownership;
- borrowing;
- deterministic destruction;
- volatile behavior;
- target-visible effects.

§ 21(8) Volatile writes are never dead merely because Sec source does not later read the value.

§ 21(9) A call is not assumed pure unless effect analysis proves the relevant absence of effects.

§ 21(10) Source excluded by target or variant selection is not dead merely because it is absent from another CompilationPlan.

---

## § 22. Unused declarations

**Governance tags:** `errors.diagnostics`, `analysis.parameter-usage`, `analysis.call-graph`

§ 22(1) A declaration is semantically unused only when the compiler proves that its binding, value and lifetime have no required semantic effect.

§ 22(2) The analysis accounts for:
- reads;
- moves;
- borrows;
- captures;
- returned values;
- deterministic destruction;
- deferred use;
- interface requirements;
- ABI requirements;
- target-specific use;
- configured variants;
- compiler-recognized entry points;
- generated references required by the language or toolchain.

§ 22(3) An owning local value is not unused merely because its identifier is never read when keeping it alive until destruction has required observable semantics.

§ 22(4) The following IDs remain permanently allocated by this rulebook:

```text
S3101  declarations.unused-local-variable
S3102  declarations.unused-local-constant
S3103  declarations.unused-parameter
S3104  modules.unused-import
S3105  lambdas.unused-capture
S3106  generics.unused-parameter
S3107  declarations.unused-private-symbol
```

§ 22(5) The owning analyses must not emit those diagnostics until the required semantic proof is available.

§ 22(6) An unused parameter in an ordinary function or lambda may be invalid when no contract requires the parameter to exist.

§ 22(7) A parameter required by an interface, callback, extern ABI or other fixed-signature contract is not semantically unused merely because its implementation body does not inspect the value.

§ 22(8) Revision 2.0 introduces no special unused-parameter spelling and does not treat underscore-prefixed names as suppression.

§ 22(9) A public exported library declaration is not unused merely because no current local target calls it.

---

## § 23. Assignment in boolean-condition contexts

**Governance tags:** `errors.diagnostics`

§ 23(1) Assignment is not a value-producing boolean expression in Sec.

§ 23(2) `S3201` is permanently allocated as:

```text
S3201
control-flow.assignment-in-condition
mandatory Error
```

§ 23(3) When an assignment appears where a boolean condition is required, the diagnostic identifies the assignment.

§ 23(4) Help may suggest `==` only when the resolved operand types support equality.

§ 23(5) The rule applies to every ordinary boolean-condition context defined by the language.

---

## § 24. Reserved IDs owned by this revision

**Governance tags:** `errors.diagnostics`

§ 24(1) The following allocations from the replaced rulebook remain reserved and must not be reused:

```text
S3001  control-flow.unreachable-statement
S3002  control-flow.unreachable-branch
S3003  control-flow.unreachable-arm
S3004  data-flow.dead-store
S3005  expressions.unused-pure-result

S3101  declarations.unused-local-variable
S3102  declarations.unused-local-constant
S3103  declarations.unused-parameter
S3104  modules.unused-import
S3105  lambdas.unused-capture
S3106  generics.unused-parameter
S3107  declarations.unused-private-symbol

S3201  control-flow.assignment-in-condition
```

§ 24(2) Reservation does not imply that an ID is currently implemented.

§ 24(3) Existing registry entries outside this list, including lexer, parser, semantic, advisory and retired IDs, remain authoritative and stable.

§ 24(4) In particular, revision 2.0 does not renumber existing `L1xxx`, `P2xxx`, `S1xxx`, `S3001` or `A2001` definitions.

§ 24(5) Retired IDs remain reserved. Known published retired entries include the existing registry definitions for retired diagnostics such as `S1020` / `operator.string-runtime-concat` and any other retired entry already present in the registry.

---

## § 25. Discard diagnostic integration

**Governance tags:** `errors.diagnostics`

§ 25(1) Semantic discard conditions are owned by `rules/control-flow/discard.md`.

§ 25(2) The central registry provides stable identities for:
- implicit discard of ordinary discardable call results when advisory policy requests a finding;
- unhandled must-use values;
- non-discardable values;
- use after explicit discard;
- discard while borrowed;
- useless explicit discard;
- redundant explicit discard of `void`.

§ 25(3) Existing published IDs must retain their existing meanings.

§ 25(4) `S1005` / `ownership.unhandled-must-use-result` remains the stable Result-specific implemented diagnostic and must not be silently broadened or renamed.

§ 25(5) `S1006` / `ownership.non-discardable-value` remains the existing non-discardable-value diagnostic.

§ 25(6) A broader must-use diagnostic receives its own registry identity when implemented unless an existing published definition already exactly owns that programmer-visible rule.

§ 25(7) Must-use, ownership, borrow and lifecycle-safety errors are mandatory when their owning rulebook defines the operation as invalid.

---

## § 26. Ownership and error-handling diagnostic quality

**Governance tags:** `errors.diagnostics`

§ 26(1) Ownership diagnostics should identify:
- the affected place or value;
- the earlier move, discard, detach or transfer when relevant;
- the source location of that earlier operation;
- why the requested operation is invalid;
- a valid source-level repair when one is known.

§ 26(2) When omission of the explicit consuming marker is the problem, help should show the canonical `<-` repair when it is valid.

§ 26(3) Error-handling diagnostics should identify expected and actual success/error types where type mismatch is the reason.

§ 26(4) Consuming `Result` or `Option` projections should relate the later invalid use to the consume location.

§ 26(5) Compiler and LSP diagnostics must consume the same resolved semantic facts rather than independently re-deriving ownership or error state in tooling.

---

## § 27. Mentor diagnostics

**Governance tags:** `errors.diagnostics`, `analysis.parameter-usage`, `analysis.pitfall`

§ 27(1) A mentor diagnostic must expose the evidence that supports the recommendation when that evidence is available.

§ 27(2) Relevant evidence may include:
- resolved type size;
- alignment;
- copy/move classification;
- mutation analysis;
- escape analysis;
- allocation analysis;
- call count;
- loop nesting;
- target/variant scope;
- effect analysis.

§ 27(3) The compiler must not claim a fact that its current analysis has not proved.

§ 27(4) `A2001` / `performance.large-value-parameter` remains a published advisory identity.

§ 27(5) Its canonical registered default severity is `Information`.

§ 27(6) Thresholds and implementation heuristics used to decide when `A2001` fires are implementation/policy data tracked in governance unless a separate normative rule makes a particular threshold part of the language/tool contract.

§ 27(7) Reverse parameter-mode advice remains advisory because API intent belongs to the programmer.

---

## § 28. Determinism and diagnostic ordering

**Governance tags:** `errors.diagnostics`, `testing.compiler-verification-v1`

§ 28(1) For identical source, compiler version, relevant configuration and CompilationPlan, diagnostic identity and structured semantic content must be deterministic.

§ 28(2) Parallel analysis must not introduce nondeterministic primary-diagnostic ordering.

§ 28(3) Where more than one diagnostic is emitted, the compiler defines a deterministic order based on source order and stable diagnostic identity, with tool-specific non-source diagnostics placed deterministically.

§ 28(4) Related locations, notes, help and fixes are emitted in deterministic order.

§ 28(5) Registry/catalog listing is stable ID order.

---

## § 29. Parser recovery boundary

**Governance tags:** `errors.diagnostics`

§ 29(1) Parser diagnostics use stable registered IDs.

§ 29(2) `P2001` remains the compatibility fallback for parser errors whose more focused migration is not yet complete.

§ 29(3) A fallback ID is not permission to collapse a focused diagnostic back into generic text after a focused published ID exists.

§ 29(4) Parser recovery may suppress secondary cascades that are consequences of one malformed construct.

§ 29(5) Such recovery suppression is compiler-internal cascade control and is not source-level diagnostic suppression under § 19.

§ 29(6) Recovery metadata may include expected tokens, unexpected token, recovery context and episode identity, but the canonical programmer-visible identity remains the registered diagnostic ID.

---

## § 30. Testing requirements

**Governance tags:** `errors.diagnostics`, `testing.compiler-verification-v1`

§ 30(1) Diagnostic tests primarily assert structured semantics, not exact English prose.

§ 30(2) Tests validate, as applicable:
- diagnostic ID;
- symbolic name;
- default and effective severity;
- mandatory/configurable classification;
- primary source location;
- related locations;
- structured arguments;
- notes/help presence;
- fix applicability and edits;
- retired-ID preservation.

§ 30(3) Registry tests verify:
- active-ID uniqueness;
- retired-ID preservation;
- symbolic-name uniqueness;
- definition-schema validity;
- stable ordering;
- complete catalog representation;
- no unregistered emitted primary diagnostics.

§ 30(4) Localization tests are separate from semantic diagnostic tests.

§ 30(5) Parser recovery tests verify that one malformed construct does not create unrelated diagnostic cascades.

§ 30(6) LSP conformance tests verify that the LSP `code` equals the canonical diagnostic ID.

§ 30(7) Machine-readable occurrence tests verify logical equivalence with the same occurrence rendered for humans.

§ 30(8) Test expectations must not encode an obsolete English sentence when the diagnostic ID and structured facts are sufficient.

---

## § 31. Migration and compatibility

**Governance tags:** `errors.diagnostics`

§ 31(1) Existing published registry IDs take precedence over obsolete allocation guidance.

§ 31(2) Therefore no existing ID is renumbered merely because its number no longer matches a former recommended subject subrange.

§ 31(3) Existing lexer IDs in the `L` namespace remain valid and establish `L` as the lexical namespace.

§ 31(4) Existing `S` diagnostics that are configurable, including `S1014`, remain valid and establish that namespace does not imply mandatory status.

§ 31(5) Existing topic-family metadata such as `lexer`, `parser`, `names`, `operators`, `types`, `testing`, `performance` and `control-flow` remains topic metadata and must not be confused with the ID namespace.

§ 31(6) Host-language structures used during migration, including current parser and semantic diagnostic structures, are adapters to the canonical logical model and are not frozen by this rulebook.

§ 31(7) Migration must move toward the canonical registry and occurrence model without breaking published diagnostic identity.

§ 31(8) `L1021` / `lexer.unescaped-interpolation-closing-brace` is a published mandatory lexer diagnostic for an unmatched single `}` in interpolated-string text. Its primary range is exactly that brace, and its message explains both the error and the `}}` repair (`rules/foundations/lexical_structure.md` § 14.3; MD-005, `rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md` § 6).

---

## § 32. Governance boundary

**Governance tags:** `errors.diagnostics`

§ 32(1) `governance/errors_diagnostics.yaml` tracks:
- which registry-definition fields are implemented;
- which diagnostic-producing subsystems have migrated to stable IDs;
- which reserved diagnostics are implemented;
- catalog command implementation;
- detailed lookup implementation;
- localization implementation;
- policy/preset implementation;
- emitted-occurrence JSON implementation;
- LSP transport implementation;
- identified implementation bugs.

§ 32(2) Implementation thresholds, temporary fallback behavior, migration percentages and current Go structure layouts belong in governance unless a normative paragraph explicitly makes them part of the observable Sec contract.

§ 32(3) An implementation-status change does not revise this rulebook unless the programmer-visible contract changes.

---

## § 33. Normative summary

**Governance tags:** `errors.diagnostics`

§ 33(1) Every emitted primary diagnostic has one stable registered ID and symbolic name.

§ 33(2) Published and retired IDs are never reused.

§ 33(3) ID namespace, topic family, severity and mandatory status are separate registry concepts.

§ 33(4) The numeric part of an ID is opaque allocation, not semantic taxonomy.

§ 33(5) Mandatory diagnostics are errors and cannot be disabled or demoted.

§ 33(6) Configurable diagnostics may resolve to off, information, warning or error without changing identity.

§ 33(7) Diagnostics are structured semantic data rendered through localized messages.

§ 33(8) English is the canonical localization fallback.

§ 33(9) The canonical inspection interface is `sec diagnostics`, `sec diagnostics --json`, and `sec diagnostics <ID>`.

§ 33(10) `sec explain` is obsolete and not canonical Sec 0.1 tooling.

§ 33(11) Sec 0.1 defines no source-level diagnostic suppression syntax.

§ 33(12) Explicit value discard uses the separately defined `discard expression` semantics.

§ 33(13) CLI, LSP, tests and machine-readable output share the same diagnostic identity.

§ 33(14) Mutable implementation status is tracked in `governance/errors_diagnostics.yaml`, not in this normative rulebook.
