# Sec — regelboksgranskning 02: Lexical Structure

**Datum:** 2026-10-27  
**Rapportrevision:** 1.0  
**Regelbok:** `rules/foundations/lexical_structure.md`  
**Repository:** `YoePro/sec`, `main`  
**Granskad commit:** `5426af6cf8cce6fe85ff897b97050dfd330ef4fe`  
**Omfattning:** Normativ konsistens, befintlig governance, konkret implementation, testtäckning.  
**Metod:** Statisk källkodsgranskning via GitHub. Inga lokala Go-tester har körts.  
**Förhållande till tidigare granskning:** NSV-01–NSV-04 hör till granskning 01. Här används NSV-05–NSV-08; befintlig NSV-01 återges inte som ett nytt fel.

## 1. Översikt

Den kanoniska implementationen ägs redan av `frontend.lexical-structure` i `governance/lexer.yaml` (`status: partial`). Posten har mycket detaljerad täckning av lexer, parser, Sema, formatter, LSP och bootstrap. De verkligt återstående bootstrapuppgifterna ligger uttryckligen under samma integration. Relaterade ägare är `frontend.names-scopes-visibility`, `tooling.formatter-v2`, `frontend.parser-recovery-result` och `frontend.grammar-conformance`.

**Resultat:**

| ID | Klass | Allvar | Säkerhet |
|---|---|---|---|
| NSV-05 | Stale governance-referens | Låg | Bekräftad i dokument |
| NSV-06 | Otydlig formatteringsnorm | Låg | Bekräftad skillnad mellan nuvarande kontrakt och formulering |
| NSV-07 | Inkonsekvent litteralexempel | Medel | Bekräftat i regeltext och lexer |
| NSV-08 | Fel diagnoskategori i lexer | Medel | Statiskt härlett ur exakt kodväg; regressionstest behöver köras |

### Redan rapporterat — ingen ny post

Det ogiltigt markerade `set`-exemplet från `lexical_structure.md` §9.3 tillhör **NSV-01** (i föregående granskningsrapport). Det återrapporteras inte som ett nytt NSV. Befintlig governance täcker också bootstraputestående (NFC, L1021, interpolation, `::` och `null`); dessa upprepas inte som buggar.

## 2. Fynd

### NSV-05 — Föråldrad hänvisning till avvecklad implementationsliggare

- **Kategori:** rulebook/governance consistency
- **Severity:** low
- **Status:** confirmed
- **Regelbok:** §§24 och 26
- **Primär governanceägare:** `frontend.lexical-structure`
- **Beskrivning:** §24 anger att aktuell implementation följs i `implementation-status.yaml`. §26 räknar upp samma fil som obligatorisk synkroniseringsdestination. Den filen avvecklades vid migreringen 2026-10-10; den aktuella kanoniska ägaren är `governance/lexer.yaml` och ägarregistret `governance/index.yaml`.
- **Effekt:** En underhållande AI kan försöka skriva till en icke existerande fil eller behandla historisk status som kanonisk.
- **Åtgärd:** Ersätt referenserna med `governance/lexer.yaml` och vid behov `governance/index.yaml`. Ändra inte implementationsstatus eller skapa ny ägarpost.
- **Dublettkontroll:** Befintlig `frontend.lexical-structure` används; detta gäller endast regelbokssynkronisering.

**Källor:**
- [lexical_structure.md §§24,26](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/lexical_structure.md#L2025-L2131)
- [governance/README.md — migreringen](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/governance/README.md)
- [governance/index.yaml](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/governance/index.yaml)

### NSV-06 — Framtida LF-undantag i §2 utan stöd i aktuell formatteringskonfiguration

- **Kategori:** rulebook consistency / formatter contract
- **Severity:** low
- **Status:** confirmed wording discrepancy; no runtime error demonstrated
- **Regelbok:** §2
- **Primär governanceägare:** `tooling.formatter-v2` (`governance/formatting.yaml`), med regelboksägaren `frontend.lexical-structure` som relaterad
- **Beskrivning:** §2 säger att formatteraren skriver LF *by default unless an explicit formatter configuration preserves another repository convention*. Men den kanoniska konfigurationen i `rules/tooling/formatter.md` §4(5), §4(18) innehåller inget sådant val, och `internal/formatter/formatter.go` normaliserar uttryckligen CRLF och CR till LF. `governance/lexer.yaml` redovisar detta som implementerat utan en alternativ konfiguration.
- **Effekt:** Formuleringen kan förväxlas med ett redan tillåtet `format_version = 1`-alternativ och leda till ett oavsiktligt implementationsspår. Den är inte bevis för att en sådan konfiguration faktiskt finns.
- **Åtgärd:** Beskriv nuvarande kontrakt direkt: formatteraren skriver LF. Eventuell framtida valbarhet kräver separat normativt beslut och formatteringsversionshantering.
- **Dublettkontroll:** Inget nytt formatteringsprojekt; befintlig `tooling.formatter-v2`.

**Källor:**
- [lexical_structure.md §2](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/lexical_structure.md#L90-L111)
- [formatter.md §§3–4](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/tooling/formatter.md)
- [formatter.go `format`](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/formatter/formatter.go#L151-L156)

### NSV-07 — `_100` betecknas som felaktigt numeriskt literal trots att det är ett giltigt identifierarnamn

- **Kategori:** normative self-consistency
- **Severity:** medium
- **Status:** confirmed
- **Regelbok:** §§6.1 och 12.6
- **Primär governanceägare:** `frontend.lexical-structure`
- **Beskrivning:** §12.6 innehåller `_100` i en lista av ogiltiga exempel på numeriska digit separators. §6.1 anger emellertid att identifierare får börja med `_` följt av ASCII-siffror. `_100` är därför ett giltigt *identifierartoken*, inte ett felaktigt numeriskt literal. `NextToken()` tar understrykning genom identifierarvägen, och `readIdentifier` ger identifieraren `_100`; den är inte lexikalt ett siffertal.
- **Effekt:** Samma sekvens sägs både vara giltig och ogiltig i skilda avsnitt. En implementerare eller testskapare kan felaktigt införa lexikal avvisning av giltiga variabelnamn.
- **Åtgärd:** Ta bort `_100` från den ogiltiga literallistan eller ange uttryckligen att `_100` inte är ett numeriskt literal utan en möjlig identifierare. Bevara enhetlig identifierargrammatik.
- **Föreslaget regressionstest:** `New("_100")` ger `IDENT("_100")` och inga lexerdiagnoser. Testa separat de verkligt ogiltiga digit-separatorformerna, exempelvis `100_` och `1__000`.
- **Dublettkontroll:** Inte samma som NSV-01 och ingen ny governanceintegration.

**Källor:**
- [lexical_structure.md §6.1](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/lexical_structure.md#L343-L407)
- [lexical_structure.md §12.6](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/lexical_structure.md#L1194-L1238)
- [lexer.go `NextToken()`](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/lexer/lexer.go#L166-L215)
- [identifiers.go](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/lexer/identifiers.go)

### NSV-08 — Baslitteral med familjesuffix och felaktig fortsättning ger fel diagnoskategori

- **Kategori:** implementation bug / lexer diagnostic
- **Severity:** medium
- **Status:** statically demonstrated code path, **not execution-verified**
- **Regelbok:** §§12.3, 12.7 och 18
- **Primär governanceägare:** `frontend.lexical-structure`
- **Kod:** `internal/lexer/lexer.go`, `readBasePrefixedInteger`
- **Beskrivning:** En kandidat såsom `0b1u2` innehåller ett giltigt binärt tal `0b1`, därefter det giltiga familjesuffixet `u`, därefter den otillåtna fortsättningen `2`. Detta är en ogiltig suffix-/identifierarfortsättning snarare än en ogiltig binär siffra i själva talet.

**Exakt kodväg:**
1. `readDigitSequence` för binär bas förbrukar `1`.
2. `consumeIntegerSuffix` förbrukar `u`.
3. `invalidDigit` testar **nästa** tecken (`2`) mot basen, utan att skilja mellan att ett suffix redan konsumerats och att man fortfarande läser talets siffror.
4. `invalidDigit != 0` väljer `L1011` (`lexer.invalid-base-digit`).

Det innebär att samma semantiska ogiltighet kan ge en vilseledande diagnos efter ett giltigt suffix. Enligt §12.3 bör en ogiltig identifierarliknande fortsättning behandlas som malformed base literal (`L1010`), medan `0b102` (utan suffix) korrekt ska behålla `L1011` för den ogiltiga binära siffran.

- **Reproduktionskandidater:** `0b1u2`, `0o7u8`, `0b1g2`.
- **Regressionstest att köra innan fix:** Verifiera utfallet för `0b1u2` kontra `0b102`, att båda förblir enskilda `ILLEGAL`-token och att diagnoserna blir `L1010` respektive `L1011`; kontrollera källspann och återhämtning till efterföljande token.
- **Åtgärd:** Håll reda på om en suffixbokstav redan konsumerats. Testa basöverskridande siffror **före** suffix separat från fortsatt text **efter** suffix. Bevara befintlig prioritet för ogiltiga digit separators och äldre migrationssuffix.
- **Dublettkontroll:** Jag hittade ingen träff för testfallen `0b1u2` eller `0o7u8` i repositorysökningen. Den befintliga `frontend.lexical-structure` dokumenterar de allmänna L1010/L1011-testerna men ingen specifik korrigering av detta suffixfall. **Ingen ny integration:** föreslå en `identified_bugs`-punkt under samma ID (eller dess befintliga `remaining` enligt lokal ledgerpraxis), och duplicera inte den som separat projekt.

**Källor:**
- [lexer.go `readBasePrefixedInteger`](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/lexer/lexer.go#L625-L678)
- [lexer_test.go — befintliga baslitteraltest](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/lexer/lexer_test.go#L842-L894)
- [lexical_structure.md §§12.3 och 12.7](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/lexical_structure.md#L1075-L1310)

## 3. Spårbarhetsmatris — samtliga 27 huvudavsnitt

Matriserna visar **existerande ägande och uppföljning**, inte att varje del är färdig eller fullständigt integrationstestad. Där en regel redan är implementerad finns motsvarande test/`implemented` under primärägaren; där den saknas/är partiell hänvisas till den existerande `remaining`-listan.

| § | Normativt ämne | Befintlig actionägare | Bedömning |
|---|---|---|---|
| 1 | UTF-8, BOM, `.sec`, byte- och skalärpositioner | `frontend.lexical-structure` | Spårat; bootstrap byteekvivalens partiell |
| 2 | LF/CRLF/CR och formatterings-LF | `frontend.lexical-structure`, `tooling.formatter-v2` | Spårat; NSV-06 |
| 3 | Whitespace | `frontend.lexical-structure` | Spårat och testat |
| 4 | Statementgränser och semikolon | `frontend.lexical-structure`, `frontend.parser-recovery-result` | Spårat |
| 5 | Kommentarer, nästling, dokumentation, retention | `frontend.lexical-structure`, `tooling.cst-lexical-foundation` | Spårat och testat |
| 6 | Identifierare, NFC, UTS#39, `_` | `frontend.lexical-structure`, `frontend.names-scopes-visibility` | Spårat; bootstrap NFC återstår |
| 7 | Hårda ord, kontextord, kontrakt och modifierare | `frontend.lexical-structure` | Spårat och testat |
| 8 | Inbyggda gemena typnamn | `frontend.lexical-structure`, `frontend.names-scopes-visibility` | Spårat |
| 9 | Kontextuellt `set` | `frontend.lexical-structure` | Spårat; redan NSV-01 |
| 10 | Kontextuellt `x` och `not in` | `frontend.lexical-structure`, `frontend.operators-v2` | Spårat |
| 11 | Boolean-literal, `None` m.fl. | `frontend.lexical-structure` | Spårat |
| 12 | Numeriska former, suffix, range | `frontend.lexical-structure` | Spårat; NSV-07/08 |
| 13 | Teckenliteral och L1009/L1018 | `frontend.lexical-structure` | Spårat och testat |
| 14 | Vanliga, råa, interpolerade strängar | `frontend.lexical-structure`, interpolations-/loweringposter i `governance/sema.yaml` | Spårat; bootstrap interpolation återstår |
| 15 | Escape-inventarium | `frontend.lexical-structure` | Spårat och testat |
| 16 | Operatorer och interpunktion, `::`, `null` | `frontend.lexical-structure` | Spårat; bootstrap `::`/`null` återstår |
| 17 | Longest match | `frontend.lexical-structure` | Spårat |
| 18 | Tokengränser | `frontend.lexical-structure` | Spårat; NSV-08 |
| 19 | Attribut och metadata | `frontend.lexical-structure`, `governance/attributes.yaml` | Spårat |
| 20 | Lexikala fel, återhämtning och ID | `frontend.lexical-structure`, `errors.diagnostics` | Spårat; bootstrap L1021 återstår |
| 21 | Lexer–parser–Sema ansvar | `frontend.lexical-structure`, `frontend.grammar-conformance` | Spårat |
| 22 | Formatteringskrav | `tooling.formatter-v2`, `frontend.lexical-structure` | Spårat, formatter delvis implementerad |
| 23 | Semantic tokens / highlighting | `frontend.lexical-structure`, LSP-integrationer | Spårat och testat |
| 24 | Implementation governance | `frontend.lexical-structure`, `governance.source-rulebook-traceability` | Spårat men hänvisningen fel: NSV-05 |
| 25 | Testmatris | `frontend.lexical-structure` + angivna testsökvägar | Spårat; komplettera suffixgränstest NSV-08 |
| 26 | Synkronisering | `governance.source-rulebook-traceability`, berörda integrationsägare | Spårat men hänvisningen fel: NSV-05 |
| 27 | Designöversikt | `frontend.lexical-structure` | Sammanfattande, ingen fristående implementation |

**Detaljnotering:** Poster med hela regelboken i `rules:` är spårbarhet på områdesnivå. Detta är inte ett bevis för att vartenda enskilt krav har en egen kontrollpunkt. Jag har stämt av omfattningen mot detaljerade `implemented`, `remaining`, testvägar och kodfilssökvägar i `governance/lexer.yaml`. Inget större nytt oägt implementationstema framkom i denna avgränsade genomgång.

## 4. Förslag till liggarinmatning

Använd befintliga NSV-ID:n och peka på **existerande kanoniska integrations-ID:n**:

- `NSV-05` → `frontend.lexical-structure` — documentation sync; ingen ny implementation.
- `NSV-06` → `tooling.formatter-v2` — normative formatter wording sync; ingen ny feature utan beslut.
- `NSV-07` → `frontend.lexical-structure` — documentation consistency + positivt test av `_100`.
- `NSV-08` → `frontend.lexical-structure` — kodbugg, föreslagen `identified_bugs`-post och regressionstest.

**Rekommenderad prioritet:** NSV-08 (diagnostisk riktighet) → NSV-07 (lexikal norm) → NSV-05 (stale governance) → NSV-06 (framtidsförbehåll).

**Inga käll- eller governancefiler har ändrats.** Rapporten är läsunderlag för liggar-AI och separata implementations-/buggfix-verktyg. Ett fullständigt dynamiskt compiler-conformance-påstående görs inte; framför allt måste NSV-08 reproduceras med Go-test före kodfix.
