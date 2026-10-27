# Sec rulebook audit 03 — Grammar

- **Audit date:** 2026-10-27
- **Report revision:** 1.0
- **Repository:** [`YoePro/sec`](https://github.com/YoePro/sec)
- **Repository reference:** `main` at `5426af6cf8cce6fe85ff897b97050dfd330ef4fe` (HEAD checked 2026-10-27; same revision as the previous audit)
- **Primary rulebook:** [`rules/foundations/grammar.md`](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/grammar.md)
- **Canonical governance owner:** `frontend.grammar-conformance`, [`governance/parser.yaml`](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/governance/parser.yaml#L367-L438)
- **Audit scope:** normative consistency, specific ownership in existing governance, code inspection and regression adequacy. No repository changes and no test execution.
- **ID continuity:** new IDs **NSV-09–NSV-14**. Earlier NSV-01 (`set` example conflict) applies across rulebooks; it is not duplicated.

## Audit result

Six documentation/governance findings are recorded. No previously untracked compiler implementation defect has been established by this review. Existing implementation gaps are explicitly tracked by the corresponding integration and must not be reintroduced as new items. Static source reading is not the same as running Sec tests.

| ID | Severity | Category | Disposition | Existing owner |
|---|---|---|---|---|
| NSV-09 | medium | rulebook/example | new | `frontend.properties` / `frontend.grammar-conformance` |
| NSV-10 | medium | rulebook/grammar | new | `frontend.grammar-conformance` |
| NSV-11 | low–medium | rulebook/example | new | `concurrency.select-v2` / `frontend.grammar-conformance` |
| NSV-12 | medium | rulebook/grammar and cross-book sync | new | `frontend.registers` / `frontend.grammar-conformance` |
| NSV-13 | low | governance metadata | new | `frontend.grammar-conformance` |
| NSV-14 | medium | grammar-production reachability | new documentation inconsistency; implementation gaps already tracked | `frontend.grammar-conformance` and `frontend.ffi-v2` |

---

## identified_bugs

### NSV-09 — `try set` example omits mandatory error type

- **severity:** medium
- **category:** rulebook_internal_inconsistency
- **status:** new
- **affected_rules:** `grammar.md` — “Impl declaration” (example), “Property declaration” (normative production); `declarations/properties.md` §6.
- **description:** The `impl Vehicle` example writes `try set value { ... }`. The same grammar's `FallibleSetter` production is `"try" Contextual("set") Identifier TypeReference Block`; properties.md §6 explicitly requires a named error type. The illustrated program therefore does not match Sec 0.1 syntax.
- **evidence:** [Grammar example](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/grammar.md#L1135-L1164); [grammar production](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/grammar.md#L1238-L1286); [canonical setter](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/declarations/properties.md).
- **existing_governance:** `frontend.properties` in `governance/declarations.yaml`, `frontend.grammar-conformance` in `governance/parser.yaml`.
- **recommended_action:** Correct the example to include a real explicit error type, e.g. `try set value SpeedError { ... }`; ensure the example's return/error handling is coherent if expanded.
- **test:** Add/maintain source examples with `try set value ErrorType`; a negative fixture for the missing error type should produce a focused error. First check existing property tests to avoid duplicates.
- **implementation_bug_confirmed:** no.

### NSV-10 — Event capacity is an expression, not merely `IntegerConstant`

- **severity:** medium
- **category:** rulebook_out_of_sync_with_implementation
- **status:** new
- **affected_rules:** `grammar.md` — “Event types”; parser type-reference logic.
- **description:** `EventType` and `EventStorageType` still specify `[ "," IntegerConstant ]`. The parser AST owns `EventCapacityExpression Expression`, and Sema calls the shared integer constant evaluator. `governance/parser.yaml` under `frontend.grammar-conformance` explicitly says capacity was upgraded to a compile-time expression. Current normative grammar must permit constant expressions, not only literal constants.
- **evidence:** [Grammar production](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/grammar.md#L3477-L3498); [AST](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/ast/ast.go); [Sema](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/sema/type_references.go); [existing implementation record](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/governance/parser.yaml#L405-L419).
- **existing_governance:** `frontend.grammar-conformance` (implemented change; no missing implementation entry).
- **recommended_action:** Use `ConstantExpression` or the canonical compile-time-required expression nonterminal, and preserve >0 semantic validation in Sema.
- **test:** A constant expression such as `Event[T, 2 + 2]` and an invalid non-constant expression; reuse the existing event capacity tests.
- **implementation_bug_confirmed:** no.

### NSV-11 — `select` illustration uses an integer where `duration` is required

- **severity:** low–medium
- **category:** cross_rulebook_example_conflict
- **status:** new
- **affected_rules:** `grammar.md` — “Select statement”; `concurrency/select.md` §30; `types/temporal.md` §5.
- **description:** The grammar's full select example includes `after 10 => { ... }`. The specialized select rulebook requires a `duration`-compatible expression, and its examples use units such as `100<ms>`. An unqualified integer literal has no unit-of-time information. In this context the example should use `after 10<ms>` or another valid time quantity.
- **evidence:** [Grammar](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/grammar.md#L2257-L2312); [Select §30](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/concurrency/select.md#L629-L651); [Temporal §5](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/types/temporal.md#L150-L178).
- **existing_governance:** `concurrency.select-v2` owns timeout semantics; `frontend.grammar-conformance` owns the consolidated grammar example.
- **recommended_action:** Replace the example with a dimensioned quantity (choose representative units, e.g. `after 10<ms>`), not a bare integer.
- **test:** Compile the complete example as a valid fixture; alternatively isolate the select timeout in a valid test. Do not claim an implementation bug without test evidence.
- **implementation_bug_confirmed:** no.

### NSV-12 — Register grammar omits implemented modifiers and misrepresents hyphenated tokenization

- **severity:** medium
- **category:** rulebook_cross_book_and_implementation_sync
- **status:** new
- **affected_rules:** `grammar.md` — “Register declaration”; `declarations/registers.md` §§7–11.
- **description:** The formal `RegisterTypeDeclaration` allows width, allocation order and byte order only. It omits `shadow`/`noshadow`; `RegisterField` omits the access modifier families (`read-only`, `read-clear`, `write-one-clear`, etc.) and field shadow policy. These are already recognized by `internal/parser/register_header.go` and `internal/parser/parser.go`. Also, the notation `Contextual("msb-first")` suggests one contextual identifier token; the lexer actually sees `IDENT("msb")`, `MINUS`, `IDENT("first")` and the parser explicitly reconstructs the modifier from separate tokens.
- **evidence:** [Register EBNF](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/grammar.md#L883-L951); [register header parser](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/parser/register_header.go); [field parsing](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/parser/parser.go#L4317-L4483); [Register rulebook](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/declarations/registers.md).
- **existing_governance:** `frontend.registers` in `governance/declarations.yaml` and `frontend.grammar-conformance` in `governance/parser.yaml`; the existing register implementation work must not be recopied.
- **recommended_action:** Update the EBNF to model valid register-level and field-level modifier slots, order and exclusions; represent hyphenated spellings as a sequence of token spellings, not `Contextual("...")` with a hyphen.
- **test:** Reuse existing register parser tests for `msb-first`, `little-endian`, `shadow`/`noshadow`, specialized access modifiers and conflicting/duplicate options. Add a grammar fixture only if one is absent.
- **implementation_bug_confirmed:** no.

### NSV-13 — Grammar governance baseline note contains a superseded status assertion

- **severity:** low
- **category:** governance_status_accuracy
- **status:** new
- **affected_file:** `governance/parser.yaml`, integration `frontend.grammar-conformance`, `document_baseline.note`.
- **description:** The baseline note still asserts that `grammar.md` “mixes the older reviewed implementation-status snapshot with newer normative decisions” and that later specialized rulebooks take precedence because of that legacy state. Yet the same integration's implemented record says the old implementation-status sections, status matrix and Appendix A were **removed on 2026-10-05**. The current grammar now explicitly delegates status to governance. The note is therefore obsolete as a statement about the current file. Do not alter its historical review date or assert that all specialized rules are implemented.
- **evidence:** [Governance baseline](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/governance/parser.yaml#L367-L389); [governance implementation entry](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/governance/parser.yaml#L405-L435); [grammar header](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/grammar.md#L1-L32).
- **existing_governance:** Update `frontend.grammar-conformance` in place; no new integration.
- **recommended_action:** Keep the history in a dated historical note if needed, but change the current-state field to describe the actual current rulebook; link specialized ownership without declaring a blanket supersession rule.
- **implementation_bug_confirmed:** no.

### NSV-14 — Foreign declaration/type/callback productions are disconnected from the grammar roots

- **severity:** medium
- **category:** grammar_production_reachability
- **status:** new documentation issue; planned implementations already owned
- **affected_rules:** `grammar.md` — “Source file”, “Top-level declarations”, “Extern function declaration”, “Primary expressions”, and “Type-reference grammar”.
- **description:** The grammar defines `ForeignTypeDeclaration`, `ForeignTypeReference`, and `ForeignCallbackExpression` under the extern section, but does not connect them to the `TopLevelDeclaration`, `TypeReference`, or `PrimaryExpression` productions. The top-level declaration inventory includes `ExternFunctionDeclaration` but not `ForeignTypeDeclaration`. The `ForeignTypeDeclaration` EBNF also uses `"C"` where the actual syntax is the `StringLiteral` `"C"` following `extern` (as already modeled in `ExternFunctionDeclaration`). This makes the stated grammar non-derivable from its root, even though foreign forms have specialized planned implementations.
- **evidence:** [Top-level declarations](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/grammar.md#L259-L298); [foreign productions](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/grammar.md#L1410-L1467); [primary grammar](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/grammar.md#L2428-L2450); [type root](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/grammar.md#L3213-L3240); [FFI remaining](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/governance/ffi.yaml#L98-L127).
- **existing_governance:** `frontend.ffi-v2` already owns the planned foreign struct/union/enum/function-pointer/callback implementation. `frontend.grammar-conformance` owns the consolidated grammar contract.
- **recommended_action:** Give foreign forms valid grammar-entry paths or identify them explicitly as future/recognized-but-not-yet-supported syntax; make the extern ABI string spelling consistent with the canonical extern declaration. Do **not** change the status of any FFI implementation merely because its grammar is made reachable.
- **test:** Build a grammar-production reachability check or structured audit for all EBNF rules. Actual FFI parser acceptance should only be required when the existing `frontend.ffi-v2` remaining item is implemented.
- **implementation_bug_confirmed:** no new bug; the missing foreign implementations were already tracked.

---

## Duplicate avoidance and existing known work

1. **NSV-01 is not reissued.** The conflict between same-scope declaration uniqueness and the valid-looking dual `set` declaration example in `lexical_structure.md`/`names_scopes_visibility.md` remains the earlier record. The `grammar.md` “Contextual `set`” section must be synchronized with the eventual correction, not assigned a second audit ID.
2. **FFI not-yet-supported forms are already pending.** `frontend.ffi-v2` in `governance/ffi.yaml` explicitly lists foreign C structs, unions, enums, bitfields, `c::fn`, `c::callback` and varargs under `remaining`. NSV-14 refers to grammar closure, **not** a new compiler implementation backlog.
3. **Parser recovery is already pending.** `frontend.parser-long-term` in `governance/parser.yaml` records invalid-node retention, contexts, causal suppression and nonlinear recovery, plus deferred fuzz campaigns. These are not newly discovered bugs.
4. **Core grammar fixtures are already covered.** `frontend.grammar-conformance` lists 13 named grammar fixtures and a test suite, including invalid diagnostics and surviving subsequent declarations. Avoid duplicating that work.
5. **Register parser features are already built.** NSV-12 is a *documentation synchronization* issue; don't reimplement specialized access and shadow rules.
6. **Event capacity expression handling is already implemented.** NSV-10 requests an EBNF correction, not another parser/Sema implementation.

## Governance / implementation traceability

| Grammar area | Existing canonical owner | Review result |
|---|---|---|
| Compilation unit, module and imports | `frontend.grammar-conformance` / `frontend.modules` | present; no new gap established |
| Token and contextual spelling boundaries | `frontend.lexical-structure`, `frontend.grammar-conformance` | prior NSV-01 remains; no duplicate |
| Named types, defaults and contracts | `frontend.grammar-conformance`, related type integrations | cross-checked against syntax and recorded implementation |
| Struct, enum, union, register | `frontend.structs`, `frontend.enums`, `frontend.unions`, `frontend.registers` | NSV-12 register EBNF sync |
| Implementations, properties, events | `frontend.impl-extensions`, `frontend.properties`, related entries | NSV-09 invalid example; NSV-10 event capacity production |
| Extern, foreign data, callback and unsafe | `frontend.ffi-v2`, related unsafe owner | NSV-14 reachability; implementation gaps already tracked |
| Statements, `match`, `try`, `select` | `frontend.grammar-conformance`, feature-specific owners | NSV-11 invalid timeout example |
| Collections and shaped expressions | `frontend.grammar-conformance` and collection/type owners | no new confirmed code defect |
| Type syntax, units and references | `frontend.grammar-conformance`, unit/type owners | current feature ownership identified |
| Malformed input and recovery | `frontend.parser-long-term`, `frontend.parser-recovery-result` | existing remaining; not duplicated |
| Formatting and source tooling | `tooling.formatter-v2` and tooling owners | no new confirmed code defect |
| Source status / metadata | `frontend.grammar-conformance` | NSV-13 status note |

**Traceability limitation:** Integration ownership was checked by domain and for the concrete findings; this does not assert that each production and every semantic clause has its own individual actionable test. That stricter traceability remains the quality criterion for implementation handoff.

## Reviewed implementation files

- `internal/parser/parser.go` — main statement dispatch, extern function parsing, property/interface setters, register field parsing.
- `internal/parser/register_header.go` — register-level modifiers, hyphenated token combination, shadow policy.
- `internal/ast/ast.go` — event capacity expression field.
- `internal/sema/type_references.go` — integer constant evaluation of event capacities.
- `governance/parser.yaml`, `governance/ffi.yaml`, `governance/declarations.yaml` — ownership and remaining work.

The repository was read remotely at the pinned revision; no Go test or executable Sec snippet was run. A static review does not prove runtime acceptance or rejection for unseen input cases.

## Suggested handoff to the ledger-updating AI

- Add exactly the six IDs `NSV-09` through `NSV-14` to the existing NSV ledger; do not re-add NSV-01.
- Set all six to *open* until the current `main` HEAD is rechecked and the relevant documentation/governance changes have landed.
- Route fixes through the canonical owners listed here, not through duplicate integrations.
- The implementation AI can use the code and tests linked above for validation. None of these six findings by itself requires a new language-design decision.
