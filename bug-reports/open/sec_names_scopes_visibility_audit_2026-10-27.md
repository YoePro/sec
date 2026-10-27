# Sec rulebook audit 01 — Names, Scopes, and Visibility

- **Datum:** 2026-10-27
- **Granskad regelbok:** `rules/foundations/names_scopes_visibility.md`
- **Baseline:** `main` @ `5426af6cf8cce6fe85ff897b97050dfd330ef4fe`
- **Rapportrevision:** 1.0
- **Status:** Källkodsgranskning, utan lokalt körda tester
- **Arbetssätt:** En regelbok i taget. Återanvänd befintliga governance-ID:n. Inga ändringar i repositoryt.

## 1. Sammanfattning

Regelbokens huvudsakliga implementation har redan en canonical governance-ägare:
`frontend.names-scopes-visibility` i `governance/declarations.yaml` (status `partial`).
Kompletterande ägare finns i lexer-, modul-, LSP-, funktions- och debug-fragmenten.

**Fynd:** två dokument-/statusfel, en redan spårad konkret kompilatorbugg
samt en svaghet i ett befintligt regressionstest. Ingen ny governance-integration
behöver skapas enbart på grund av dessa fynd.

## 2. Fynd

### NSV-01 — Ogiltigt exempel markerat som giltigt (regelbok)
- **Allvarlighetsgrad:** Medium
- **Typ:** Normativ självmotsägelse / exempel
- **Status:** Bekräftat i källtext
- **Regler:** §2 (en deklarationsnamespace per scope), §10 (kontextuellt `set`).
- **Problem:** §10 säger att följande är giltigt i ett och samma scope:

```sec
let set: int := 3

fn set(value: int) int {
    return value
}
```

§2 förbjuder uttryckligen att en variabel och en funktion har samma namn
i samma scope. Det är alltså **inte** kontextualiteten hos `set` som är
problemet utan den vanliga deklarationskonflikten.

Det identiska problemet finns i `rules/foundations/lexical_structure.md` §9.3.

- **Åtgärd:** Visa giltiga `set`-deklarationer i *separata* scope, eller visa
  bara en av deklarationerna. Synkronisera båda regelböckerna.
- **Governance:** Befintliga `frontend.names-scopes-visibility` och
  `frontend.lexical-structure`; skapa ingen ny integration.

**Källor:**
- [names_scopes_visibility.md §2 och §10](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/names_scopes_visibility.md)
- [lexical_structure.md §9.3](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/lexical_structure.md)

### NSV-02 — §23 är osynkroniserad och innehåller ett motsatt krav
- **Allvarlighetsgrad:** Medium
- **Typ:** Regelboks-/governance-statusfel
- **Status:** Bekräftat genom jämförelse
- **Problem:** Listan 'Not implemented' kräver bland annat
  *“contextual `set` parsing without allowing `set` declarations”*,
  trots att §10 och lexical_structure.md §9 uttryckligen tillåter
  vanliga deklarationer med namnet `set`.
- **Ytterligare statusavvikelse:** §23 behandlar fortfarande kontextuellt `x`,
  reserverade deklarationsnamn och kapitalisering av nominaltyper som
  generella oimplementerade krav. Motsvarande delimplementationer/tester
  finns redan registrerade i `governance/lexer.yaml` och
  `governance/declarations.yaml`.
- **Åtgärd:** Ta bort den motsatta formuleringen, och uppdatera §23 till
  verkliga *återstående* delar med hänvisning till befintliga governance-ID:n.
  Behåll oavslutade bredare namespace-/synlighetskrav.
- **Governance:** Uppdatera innehållet i de befintliga posterna vid behov,
  inte antalet poster.

**Källor:**
- [Regelbok §23](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/names_scopes_visibility.md)
- [lexer.yaml](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/governance/lexer.yaml)
- [declarations.yaml — frontend.names-scopes-visibility](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/governance/declarations.yaml)

### NSV-03 — Modulvariabler kan kollidera mellan orelaterade moduler (kod)
- **Allvarlighetsgrad:** Hög
- **Typ:** Direkt implementationsbugg — **redan dokumenterad**
- **Status:** Källkoden stöder befintligt governance-fynd; inget nytt fynd-ID i governance
- **Regler:** §§2–3 och §17.
- **Problem:** Den modulindexerade namespace-kontrollen arbetar med
  `a.moduleSurfaces[a.currentModule]`, men `defineSymbol()` använder
  `a.symbols[name]` utan modulidentitet. Denna gemensamma registrering kan
  ge falska dubblettfel och påverka namnupplösning över modulgränser.
- **Redan registrerat:** `frontend.names-scopes-visibility` → `remaining`:
  “module-level variables live in the analyzer's shared symbol table …”.
- **Åtgärd:** Fortsätt på **samma** återstående punkt. Verifiera separat
  modulisolering, korrekta felpositioner och att lokala namn i andra moduler
  inte påverkas.

**Källor:**
- [analyzer.go — module namespace och defineSymbol](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/sema/analyzer.go)
- [declarations.yaml — registrerad bugg](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/governance/declarations.yaml)

### NSV-04 — Kontextuellt `set`-test använder en otillåten skuggning
- **Allvarlighetsgrad:** Låg
- **Typ:** Testkvalitet / bristande negativ kontroll
- **Status:** Bekräftat i testkällan; testkörning ej utförd
- **Problem:** `TestContextualSetRemainsAvailableAsDeclarationName` använder:

```sec
fn set(set: int) int {
    let set := 1
    return set
}
```

Den lokala variabeln skuggar en parameter (§8). Testet kontrollerar dock
enbart att diagnosen för **reserverat namn** uteblir. Därmed kan testet
passera trots andra kompileringsfel i exemplet.
- **Åtgärd:** Dela upp i separata giltiga exempel, eller använd andra namn
  för parameter/lokal. Kontrollera att analysen ger **noll fel**, inte bara
  frånvaro av en viss diagnostik. Lägg gärna ett separat negativt fall för
  parameter/lokal-skuggning.
- **Governance:** Befintligt `frontend.lexical-structure` och
  `frontend.names-scopes-visibility` — lägg vid behov till en `required_tests`
  eller teståtgärd i befintlig integration.

**Källa:** [internal/sema/reserved_names_test.go, TestContextualSetRemainsAvailableAsDeclarationName](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/sema/reserved_names_test.go)

## 3. Spårbarhet: samtliga 24 regelboksavsnitt

Följande är en **ägarkartläggning**, inte ett påstående att varje krav är
färdigimplementerat. För flera avsnitt täcker en bred integration flera
konkreta delkrav; den befintliga `remaining`-listan är normativ för
vad som fortfarande återstår.

| § | Område | Befintlig primär spårning / relaterad ägare | Bedömning |
|---|---|---|---|
| 1 | Identifierare | `frontend.lexical-structure` | Spårad |
| 2 | En namespace per scope | `frontend.names-scopes-visibility` | Spårad, `partial` |
| 3 | Modulnamespace | `frontend.modules` + `frontend.names-scopes-visibility` | Spårad, `partial` |
| 4 | Överlagrade funktioner | `frontend.functions-v2` | Spårad, `partial` |
| 5 | Medlemsnamespace | `frontend.names-scopes-visibility`, `frontend.impl-extensions` | Spårad, `partial` |
| 6 | Nästlade deklarationer | `frontend.impl-extensions` + namespaceägaren | Spårad |
| 7 | Lexikala scope | `frontend.names-scopes-visibility` | Spårad, `partial` |
| 8 | Skuggning | `frontend.names-scopes-visibility` | Spårad, `partial` |
| 9 | Reserverade namn | `frontend.lexical-structure` | Implementerade delkrav; §23 stale |
| 10 | Kontextuellt `set` | `frontend.lexical-structure` | Implementerad del; NSV-01/02 |
| 11 | Kontextuellt `x` | `frontend.lexical-structure`, `frontend.operators-v2` | Implementerad del; §23 stale |
| 12 | Synlighetsprefix | `frontend.names-scopes-visibility`, `frontend.modules` | Spårad, `partial` |
| 13 | Nominaltypers versaler | `frontend.names-scopes-visibility` | Implementerad del; §23 stale |
| 14 | Andra namnstilar | `frontend.names-scopes-visibility` | Inga extra namnkrav |
| 15 | Generiska parametrar | `frontend.names-scopes-visibility` | Spårad, `partial` |
| 16 | Importer och alias | `frontend.modules` | Spårad, `partial` |
| 17 | Lookup-ordning | `frontend.names-scopes-visibility`, `frontend.modules` | Spårad, `partial` |
| 18 | Framåtreferenser | `frontend.modules`, `frontend.functions-v2` | Spårad, `partial` |
| 19 | Namnidentitet | `frontend.modules`, `frontend.function-overload-backend-identity` | Spårad |
| 20 | Diagnostik | `frontend.names-scopes-visibility`, diagnostikregistret | Spårad, `partial` |
| 21 | Parser, AST, Sema | `frontend.lexical-structure`, namespaceägaren | Spårad, `partial` |
| 22 | Formatter och verktyg | `tooling.lsp-rename`, `frontend.lexical-structure`, `compiler.debug-information-v1` | Spårad på områdesnivå |
| 23 | Implementation status | Ägarna ovan | **Osynkroniserad; NSV-02** |
| 24 | Synkronisering | `governance.source-rulebook-traceability`, berörda ägare | Spårad som övergripande policy |

**Avgränsad öppen precisionsfråga:** §22 anger att debugger/diagnostik
ska använda fullständigt kvalificerade namn när tvetydighet annars finns.
`compiler.debug-information-v1` spårar semantiska identiteter och
namnrepresentation, men jag hittade inte en separat uttrycklig
kontrollpunkt för just den presentationsregeln i granskade fragment.
Det bör kontrolleras i respektive implementation/testpaket innan
något nytt governance-objekt skapas.

## 4. Rekommenderad uppföljning (utan dubletter)

1. Rätta giltiga `set`-exempel i de två regelböckerna.
2. Synka §23 med redan genomförda integrationer; behåll kvarvarande verkliga hål.
3. Låt befintligt `frontend.names-scopes-visibility` äga modulvariabelfelet.
4. Stärk regressionstestet för kontextuellt `set` med fel-fritt giltigt program.
5. Kontrollera §22:s specifika debuggerpresentationskrav mot befintliga paket.

## 5. Verifikationsgräns

Denna granskning bygger på låsta GitHub-filer vid angiven SHA.
Ingen lokal checkout, kompileringskörning eller regressionstestkörning
har utförts i denna session. Därför görs inga nya påståenden om att
en viss färsk reproduktion faktiskt har körts eller att alla
kodvägar har täckts.
