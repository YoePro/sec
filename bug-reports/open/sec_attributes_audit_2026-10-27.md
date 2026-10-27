# Sec – Regelboksgranskning 5: Attributes

| Fält | Värde |
|---|---|
| **Granskat dokument** | `rules/foundations/attributes.md` |
| **Status** | Granskning – föreslagna rättningar, ej implementerade |
| **Datum** | 2026-10-27 |
| **Rapportrevision** | 1.0 |
| **Sec-version** | 0.1 |
| **Repository** | `YoePro/sec` |
| **Baslinje** | `5426af6cf8cce6fe85ff897b97050dfd330ef4fe` |
| **Primär governance-ägare** | `governance/attributes.yaml`, `frontend.attributes-v2` |
| **NSV-serie** | NSV-21–NSV-25 |
| **Granskningsmetod** | Statisk genomgång av regelbok, governance, parser/AST/Sema, LSP och befintliga tester. Inga nya kompilator-/regressionstester har körts. |

## 1. Avgränsning och kontroll mot tidigare arbete

Denna granskning följer granskning 4 (`rules/foundations/operators.md`, NSV-15–NSV-20).
Den tidigare `?`-punkten återintroduceras inte. Sec 0.2:s ännu ej fastställda
villkorliga uttryck ingår inte i denna granskning.

Redan dokumenterade implementationssteg för `@target`, `@when`, fullständig
`@address`-validering, ISR-lowering och verifierade effekter ska **inte** få
duplicerade governance-poster. Här rapporteras specifika avvikelser som inte
täcks av att dessa funktioner är planerade.

De normativa källorna är främst [`rules/foundations/attributes.md` L77–L111](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/attributes.md#L77-L111),
[`rules/foundations/attributes.md` L331–L475](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/attributes.md#L331-L475),
[`rules/foundations/attributes.md` L1910–L1970](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/attributes.md#L1910-L1970) och
[`rules/foundations/attributes.md` L2607–L2620](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/attributes.md#L2607-L2620).

## 2. Sammanfattande spårbarhetsmatris

| ID | Klass | Prioritet | Fynd | Primär ägare | Rekommendation |
|---|---|---|---|---|---|
| NSV-21 | Kod-/testbrist, LSP | Medium | MD-030:s nya attributmål saknas i LSP-komplettering | `frontend.attributes-v2` | Ändra målklassificering + test |
| NSV-22 | Kod-/testbrist, parser/AST/Sema | **High** | Tillåtna selektionsattribut tappas tyst för AST-noder utan attributfält | `frontend.attributes-v2` | Fail-closed + AST-kontrakt + test |
| NSV-23 | Kod-/testbrist, parser | Medium | `@target`/`@when` godkänns felaktigt som metodattribut | `frontend.attributes-v2` | Kräv faktiskt toppnivåmål + test |
| NSV-24 | Governance-/statusbrist | Medium | MD-030 saknas i attribut-governance; `@noBlock` har motsägande status | `frontend.attributes-v2` | Synka poster utan att dublera ägare |
| NSV-25 | Regelboksfel | Low | Fem kategorier kallas fyra; appendix och argumentöversikt ofullständiga | `rules/foundations/attributes.md` | Redaktionsfix utan ny språksemantik |

## 3. NSV-21 – LSP kompletterar inte enligt MD-030

**Klass:** Bekräftad kodavvikelse genom källkodsläsning.  
**Allvarlighetsgrad:** Medium.

### 3.1 Normativt kontrakt

MD-030 tillåter `@noPanic` före `init`, `free`, ett enskilt `get`/`set`/
`try set`, samt ett `let` som direkt initierar en lambda – även i lokalt scope.
Se [`rules/foundations/attributes.md` L1910–L1970](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/attributes.md#L1910-L1970).

### 3.2 Observerad kod

- LSP:s `attributeTargetAfter()` känner endast igen `fn`, `extern`, `type`,
  `enum` och `let`: [`cmd/lsp/attributes.go` L123–L163](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/cmd/lsp/attributes.go#L123-L163).
- `AllowedOn(AttributeTargetLet)` kontrollerar enbart `attributeOnLet`.
  `@noPanic` har i parserregistret **endast** `attributeOnLambdaLet` för detta
  fall: [`internal/parser/attributes.go` L58–L72](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/parser/attributes.go#L58-L72) och
  [`internal/parser/attributes.go` L584–L604](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/parser/attributes.go#L584-L604).
- Befintliga LSP-tester saknar dessa MD-030-placeringar:
  [`cmd/lsp/attributes_test.go` L18–L65](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/cmd/lsp/attributes_test.go#L18-L65).
- Parserns särskilda MD-030-stöd finns redan:
  [`internal/parser/attributes.go` L105–L125](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/parser/attributes.go#L105-L125) och
  [`internal/parser/attributes.go` L210–L258](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/parser/attributes.go#L210-L258).

### 3.3 Förväntad effekt

`@` före `init`/`free`/accessor ger okänt mål och kan visa otillåtna attribut.
`@` före ett direkt lambda-initierande `let` föreslår inte `noPanic`.
Detta är ett LSP-fel – inget belägg för att parserns MD-030-semantik saknas.

### 3.4 Föreslagen rättning

Gör klassificeringen syntaktiskt kontextmedveten och återanvänd parserns
attributregister; särskilj `let` som direkt initierar lambda från andra `let`.
LSP-komplettering ska vara en projektion av lagliga mål, inte en fristående
approximativ förteckning.

### 3.5 Nya regressionstester

Komplettering före `init`, `free`, `get`, `set`, `try set`, top-level och
lokal lambda-`let`, samt negativa tester före andra lokala `let` och före
hela `property`-deklarationer.

## 4. NSV-22 – Tyst förlust av selektionsattribut

**Klass:** Bekräftad kodavvikelse genom källkodsläsning.  
**Allvarlighetsgrad:** High.

### 4.1 Normativt kontrakt

`@target` och `@when` selekterar toppnivådeklarationer; bland annat
`import` är uttryckligen tillåtet för `@target`:
[`rules/foundations/attributes.md` L526–L575](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/attributes.md#L526-L575).
Tills selektionsfunktionen implementeras måste accepterade attribut som
saknar effekt avvisas, inte ignoreras:
[`governance/attributes.yaml` L54–L72](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/governance/attributes.yaml#L54-L72).

### 4.2 Observerad kod

`attachAttributes()` godkänner `attributeOnAnyTopLevel` oberoende av
AST-nodens förmåga att bära attribut, men lagrar endast attribut på
`FunctionDeclaration`, `TypeDeclStatement`, `EnumDeclaration` och
`LetStatement`. För `LetGroupStatement` sker bara kontroll av `@address`:
[`internal/parser/attributes.go` L391–L452](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/parser/attributes.go#L391-L452).

`LetGroupStatement` saknar attributfält:
[`internal/ast/ast.go` L949–L958](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/ast/ast.go#L949-L958).

`validateAttributeSemantics()` besöker inte import eller grupperade `let`:
[`internal/sema/attributes.go` L19–L68](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/sema/attributes.go#L19-L68).

### 4.3 Minimala regressionsfall

```sec
module main

@when(config.audit)
let first := 1, second := 2
```

```sec
module main

@target(os: "linux")
import "platform/linux"
```

**Statisk slutsats:** Godkännande i parserns attributvalidering kan följas av
förlust i AST, utan att Sema kan rapportera att selektionssemantiken saknas.
Det första fallet är direkt belagt av `LetGroupStatement`-strukturen. Det
andra följer av att `ImportStatement` saknar fall i `attachAttributes()`;
kompilatorintegrationstest krävs för att fastställa det slutliga utfallet.

### 4.4 Föreslagen rättning

1. Inför invariant: varje känt och syntaktiskt tillåtet attribut måste antingen
   bevaras på sin avsedda nod / compilation unit och valideras, **eller**
   uttryckligen diagnostiseras som ännu ej stödd placering.
2. Implementera attributbärande representation för samtliga tillåtna
   toppnivåmål när urval införs; koppla den till plan-time/active-source-graph.
3. Fram till dess: avvisa oimplementerade `@target`/`@when` på samtliga tillåtna
   syntaktiska former; ingen tyst no-op.
4. Lägg till AST-roundtrip-/Sema-fail-closed-/CLI-negativtester för import,
   grupperad `let`, impl, unit, interface och övriga tillåtna mål.

### 4.5 Governance-deduplicering

Ägarskapet finns redan i `frontend.attributes-v2`, under återstående
`@target` och `@when`. Skapa i så fall en `identified_bugs`-post där, inte ett
nytt parallellt integrations-ID.

## 5. NSV-23 – `@target` och `@when` accepteras på metod

**Klass:** Bekräftad parseravvikelse genom källkodsläsning.  
**Allvarlighetsgrad:** Medium.

### 5.1 Normativt kontrakt

Båda är selektionsattribut för fil/toppnivå och inte godtyckliga metodattribut:
[`rules/foundations/attributes.md` L419–L475](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/attributes.md#L419-L475) och
[`rules/foundations/attributes.md` L477–L575](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/attributes.md#L477-L575).

### 5.2 Observerad kod

`attributeOnAnyTopLevel` fungerar som ett generellt undantag i
`attachAttributes()`, även när `member == true`, varefter funktionen kan
lägga `@target`/`@when` på metodens `Attributes`:
[`internal/parser/attributes.go` L391–L446](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/parser/attributes.go#L391-L446).

LSP:s `AllowedOn()` returnerar dessutom `true` direkt för varje attribut vars
mål inkluderar `attributeOnAnyTopLevel`, även när den tillfrågade måltypen
är `Method`: [`internal/parser/attributes.go` L584–L604](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/parser/attributes.go#L584-L604).

### 5.3 Minimal negativ test

```sec
type Counter struct {
    value: int
}

impl Counter {
    @target(os: "linux")
    fn Tick() void {}
}
```

**Förväntat:** mål-/placeringsdiagnostik på `@target`.  
**Nuvarande källkodsväg:** parsern godkänner placeringen; eftersom `@target`
inte är implementerat kan Sema i stället lämna ett generellt
"not implemented yet". Detta är inte samma sak som korrekt placeringskontroll.

### 5.4 Föreslagen rättning

Validera `attributeOnAnyTopLevel` mot faktisk deklarationsnivå, inte enbart
bitflaggan. Låt samma begränsning gälla parser och LSP; testa metodfallet
negativt och det motsvarande toppnivåfallet positivt.

## 6. NSV-24 – Governance ligger efter MD-030 och motsäger sig om `@noBlock`

**Klass:** Governance-/statusbrist.  
**Allvarlighetsgrad:** Medium.

### 6.1 Observerad avvikelse

`governance/attributes.yaml` är reviderad 2026-10-05 och beskriver bara
de tidigare toppnivå-, metod- och `let`-målen i attributparsersammanhang:
[`governance/attributes.yaml` L5–L12](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/governance/attributes.yaml#L5-L12) och
[`governance/attributes.yaml` L54–L65](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/governance/attributes.yaml#L54-L65).

Den 17 oktober tillkom MD-030 i regelboken med nya
`@noPanic`-placeringar: [`rules/foundations/attributes.md` L1910–L1970](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/attributes.md#L1910-L1970).
Implementation och tester finns bland annat i
[`internal/parser/no_panic_bodies_test.go` L1–L104](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/parser/no_panic_bodies_test.go#L1-L104) och
[`cmd/compiler/no_panic_bodies_test.go` L1–L53](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/cmd/compiler/no_panic_bodies_test.go#L1-L53).
MD-030 har ingen motsvarande synlig punkt i `governance/attributes.yaml`.

Därtill listar en `implemented`-punkt uttryckligen `@noBlock` som
"not implemented", och direkt efter finns en `implemented`-punkt som
beskriver genomförd transitiveffektverifiering för `@noBlock`:
[`governance/attributes.yaml` L56–L60](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/governance/attributes.yaml#L56-L60).

### 6.2 Föreslagen rättning

- Uppdatera samma integrationspost `frontend.attributes-v2` med MD-030:s
  genomförda parser/Sema/tester och återstående LSP-/formatteringsarbete.
- Korrigera den inaktuella `@noBlock`-statusmeningen.
- Ange korrekt verifieringsdatum och baslinje för nya punkter.
- Lägg inte upp ny implementationsägare för redan implementerat MD-030-stöd.

## 7. NSV-25 – Redaktionsbrister i kanonisk attributöversikt

**Klass:** Regelboksfel, ingen ny språkdesign.  
**Allvarlighetsgrad:** Low.

### 7.1 Fynd

1. Dokumentet anger "four semantic categories" men räknar upp **fem**:
   selection, target-binding, verified guarantee, nominal policy och
   foreign-binding: [`rules/foundations/attributes.md` L154–L209](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/attributes.md#L154-L209).
2. `@link_name` finns i den kanoniska initiala listan men saknas i appendix B:
   [`rules/foundations/attributes.md` L77–L111](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/attributes.md#L77-L111) och
   [`rules/foundations/attributes.md` L2607–L2620](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/attributes.md#L2607-L2620).
3. Översikten "Positional and named arguments" saknar explicit rad för
   `@link_name`, trots att semantiken finns i foreign-binding-avsnittet:
   [`rules/foundations/attributes.md` L199–L209](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/attributes.md#L199-L209) och
   [`rules/foundations/attributes.md` L256–L293](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/attributes.md#L256-L293).
4. Appendix B listar `@noPanic` enbart för funktion/metod, inte MD-030:s
   `init`, `free`, accessorer och direkt lambda-initierande `let`. Samma tabell
   utelämnar `extern`-kontrakten för `@noAlloc`, `@noPanic` och `@noBlock`:
   [`rules/foundations/attributes.md` L1910–L1970](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/attributes.md#L1910-L1970),
   [`rules/foundations/attributes.md` L1971–L1990](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/attributes.md#L1971-L1990) och
   [`rules/foundations/attributes.md` L2607–L2620](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/attributes.md#L2607-L2620).
5. Appendix B skriver `@interrupt` och `@isr` som `function`, medan parserns
   register och avsnittet "Allowed targets for verified effects" också talar
   om methods. Synka tabellen till fastställt kontrakt; inför inte en ny regel
   genom en oavsiktlig tabelländring:
   [`rules/foundations/attributes.md` L1883–L1908](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/rules/foundations/attributes.md#L1883-L1908) och
   [`internal/parser/attributes.go` L58–L70](https://github.com/YoePro/sec/blob/5426af6cf8cce6fe85ff897b97050dfd330ef4fe/internal/parser/attributes.go#L58-L70).

### 7.2 Föreslagen rättning

Korrigera kategoritalet, komplettera kanoniska tabellen och argumentöversikten
samt synka målbeskrivningar med MD-030 och de separata foreign-binding-reglerna.
Behåll numrering och befintliga rubriker; inga nya designbeslut krävs för
redaktionella fakta.

## 8. Rekommenderad fixordning

1. **NSV-22:** Fail-closed mot tyst attributförlust (säkerhet i kompilatorns kontrakt).
2. **NSV-23:** Korrekt nivåkontroll för `@target`/`@when`.
3. **NSV-21:** LSP-komplettering enligt giltiga placeringar.
4. **NSV-24:** Governance-sync inklusive befintliga MD-030-tester.
5. **NSV-25:** Regelbokens kategorier och kanoniska tabeller.

### Begränsningar

Detta är en **källkodsgranskning**, inte resultatet av en körd testsvit.
Alla kodpåståenden ovan avser observerad styrning/dataflöde i den angivna
committen. Regressionsexemplen är förslag till verifiering och har inte
körts i denna granskning.

Ingen governance-, regelboks- eller implementationfil har ändrats.
