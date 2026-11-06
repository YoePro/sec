# Sec LSP — fyra buggrapporter för import i buggrapportsliggaren

**Rapportdatum:** 2026-11-06  
**Revision:** 1.0  
**Repository:** `YoePro/sec`  
**Granskad baseline:** `main` vid commit `e37b17eeebe33f521234c0c8bf3963b30005a1dd` (2026-11-05)  
**Rapporttyp:** Statisk implementationsgranskning (LSP och Sema-integration)  
**Verifieringsstatus:** Kodvägar verifierade genom läsning av GitHub-källorna. **Inga Go-tester eller runtime-reproduktioner har körts inom denna granskning.**  
**Syfte:** Underlag för Codex 5.6 Terra att registrera **fyra separata öppna buggar** i den kanoniska liggaren, utan dubblerat governance-arbete.

## Instruktion till Codex 5.6 Terra — import, inte implementation

1. Kontrollera **aktuell** `main` och jämför med baseline ovan. Flera av Claudes prestandafixar och arbetet med delad analys kan ha tillkommit efter denna commit. Verifiera varje fynd mot aktuell kod innan registrering och kontrollera om det redan är rättat.
2. Läs `bug-reports/README.md`, `bug-reports/index.yaml` och befintliga rapporter. Leta efter semantiskt motsvarande buggar **före** ID-tilldelning. Befintligt index hade `BUG-000001`–`BUG-000024` vid granskningen; detta är **inte** en reservation av `BUG-000025`–`BUG-000028`.
3. Bevara rapportens stabila `source_id` nedan. Tilldela lediga, monotont stigande `BUG-NNNNNN`-ID enligt den aktuella liggaren. Registrera **en separat post per fynd** i `bug-reports/index.yaml`, med `status: open`, rätt `severity`, `kind`, `area`, `rulebooks`, `governance`, `evidence` och `reported`. Rapportens `source_id` förs över ordagrant. Hantera eventuella tidigare registreringar som dubletter i stället för att skapa nya poster.
4. Repositoryts `bug-reports/README.md` tillåter importerade auditfiler med flera fynd. Spara gärna detta dokument oförändrat som **en** importerad rapport under `bug-reports/open/` och referera till samma rapportfil från fyra indexposter. Alternativt dela upp i fyra separata `open/BUG-NNNNNN-*.md` enligt mallens rubriker. **Skapa inte båda varianterna som parallella buggar.**
5. Använd befintliga governance-integrationer som ägare. Skapa inte nya integrations-ID:n eller dubbla `remaining`-punkter. Lägg eventuell buggreferens/`identified_bugs` där lokal governancepraxis medger det.
6. Låt implementation och fixstatus vara oförändrade i detta importsteg. Skapa först fokuserade regressionstester vid separat buggfixarbete. Förväxla inte statiskt konstaterad felaktig kodväg med körd reproduktion.

## Översikt

| Source ID | Fynd | Allvar | Primär governanceägare | Belägg |
|---|---|---|---|---|
| `LSP-AUD-20261106-01` | Fel analysnivå vid målplattformsanalys | medium | `tooling.lsp-multi-target-diagnostics` | Hårdkodad `AnalysisInteractive` |
| `LSP-AUD-20261106-02` | Cache-nyckel skiljer inte mellan olika overlays | high | `tooling.lsp-responsiveness` | Endast `hasOverlay` jämförs |
| `LSP-AUD-20261106-03` | Importfel döljer oberoende Sema-fel | medium | `tooling.lsp-import-resolution` | Sema-fel undertrycks om `importErrors` inte är tom |
| `LSP-AUD-20261106-04` | Osparad diagnostikpolicy ignoreras | medium | `tooling.lsp-pitfall-presentation` | Konfiguration läses enbart från disk |

---

## LSP-AUD-20261106-01 — Målplattformsanalys ignorerar begärd analysnivå

**Status:** Open / statiskt verifierad kodväg; runtime-test saknas  
**Severity:** `medium`  
**Kind:** `lsp-analysis-depth-configuration-bug`  
**Affected phase:** LSP:s målplattformsanalyser, semantiska snapshots, diagnoser och interface-conformance  
**Area:** `tooling-lsp`  
**Governance integration:** `tooling.lsp-multi-target-diagnostics` (`governance/tooling_lsp.yaml`), relaterat `tooling.lsp-pitfall-presentation`

### Report

`analysisForTarget()` skapar först en projektkonfigurerad analysator via `newLSPAnalyzerWithOverlay(...)`. När den begärda målplattformen har en giltig `ScalarPlan` **ersätts** analysatorn med `sema.NewAnalyzerWithScalarPlanAndDepth(plan, sema.AnalysisInteractive)`. Projektets val `standard` eller `deep` ignoreras således för dessa analyser. Ersättningen riskerar dessutom att tappa konfiguration som sattes av `newLSPAnalyzerForInputsAtDepth`, däribland diagnostikpolicy och hardware module context.

Kodpunkt:

```go
analyzer := newLSPAnalyzerWithOverlay(uri, program, overlay)
// ...
analyzer = sema.NewAnalyzerWithScalarPlanAndDepth(plan, sema.AnalysisInteractive)
```

### Reproduction / regressionstest

1. Skapa en projektfixture med `[analysis] lsp_depth = "deep"` och minst två målplattformar där `platformtarget.Find(target)` och `ScalarPlan()` lyckas.
2. Kör `analysisForTarget()` för en icke-aktiv målplattform och kontrollera vilken effektiv analysnivå och vilka valbara rapporter som används. Jämför med motsvarande `standard` och `interactive`.
3. Kontrollera även att projektets `diagnostics.rules` och relevanta projekt-/targetinställningar tillämpas på samma sätt som vid aktiv analys.
4. Testa att växling av nivå ger en ny kompatibel snapshot och inte återanvänder den gamla.

### Expected and actual behavior

- **Expected:** Det effektiva projektvalet (`interactive`, `standard`, `deep`) styr också målplattformsanalyser. Target-specifik scalar plan får inte kringgå övrig analyskonfiguration.
- **Actual (kodmässigt):** En giltig `ScalarPlan` väljer ovillkorligt `AnalysisInteractive`. Eventuella konfigurationsinställningar på den först skapade analysatorn går förlorade vid ersättningen.

### Scope / regelverksstöd

- `rules/tooling/lsp.md` — Configuration, Snapshots, Target-aware analysis.
- `rules/compiler/compiler_analysis.md` §§60, 62 — samma garantier vid jämförbar nivå och plan.
- `rules/projects/projects.md` §23 — analysnivå för LSP.

### Föreslagen åtgärd

Konstruera target-analysatorn med **effektivt djup** och samma nödvändiga konfiguration som den ordinarie LSP-vägen. Undvik att först konfigurera en analysator som sedan kastas bort. Behåll korrekt, konfigurationsberoende snapshot-identitet. Regressionstest före fix.

**Kodreferenser:**  
- [target_analysis.go L103–133](https://github.com/YoePro/sec/blob/e37b17eeebe33f521234c0c8bf3963b30005a1dd/cmd/lsp/target_analysis.go#L103-L133)  
- [analysis_configuration.go L29–41](https://github.com/YoePro/sec/blob/e37b17eeebe33f521234c0c8bf3963b30005a1dd/cmd/lsp/analysis_configuration.go#L29-L41)  
- [analysis_depth.go](https://github.com/YoePro/sec/blob/e37b17eeebe33f521234c0c8bf3963b30005a1dd/cmd/lsp/analysis_depth.go)

**Dublettkontroll:** Befintlig `tooling.lsp-multi-target-diagnostics` äger funktionen. Ingen ny target-analysmotor ska införas.

---

## LSP-AUD-20261106-02 — Delad analyscache har ofullständig overlay-identitet

**Status:** Open / statiskt verifierad cache-nyckel; runtime-test saknas  
**Severity:** `high` (risk för felaktig återanvändning av semantiska fakta)  
**Kind:** `lsp-cache-correctness-bug`  
**Affected phase:** Cachevalidering före semantisk analys och LSP-frågor  
**Area:** `tooling-lsp`  
**Governance integration:** `tooling.lsp-responsiveness` (`governance/tooling_lsp.yaml`)

### Report

`cachedLSPAnalysis()` återanvänder cacheposten för samma `uri`, `text`, `epoch`, `depth` och booleska `hasOverlay`. `hasOverlay` beräknas enbart från om `overlay.Sources` eller `overlay.Targets` **är icke-tomma**. Den faktiska overlay-versionen, innehållet, importerade källor och vald targetvariant skiljs inte åt i cachevillkoret.

```go
hasOverlay := len(overlay.Sources) > 0 || len(overlay.Targets) > 0
// ...
if ok && (entry.epoch != analysisCache.epoch || entry.text != text ||
    entry.hasOverlay != hasOverlay || entry.depth != depth) {
    ok = false
}
```

Normalt `didChange` avancerar den globala epoken, vilket **minskar frekvensen** i ordinarie VS Code-flöde. Det gör dock inte återanvändningsvillkoret korrekt för två olika overlay-snapshots inom samma epok, eller vid fortsatt utbyggnad av den delade diagnostikcachen. Felet ska därför inte beskrivas som ett observerat VS Code-fel utan ytterligare reproduktion.

### Reproduction / regressionstest

1. Aktivera cachen med `enableAnalysisCache()` i test.
2. Håll aktiv `uri` och `text` oförändrade.
3. Anropa `cachedLSPAnalysis()` med `overlayA`, innehållande en importerad/sammanhörande fil med definition `fn Value() int`, och frigör låset.
4. Anropa igen **utan `advanceAnalysisEpoch()`** med `overlayB`, där samma fil i stället definierar en inkompatibel signatur/typ, men där båda overlays är icke-tomma.
5. Kontrollera att den andra analysen inte är samma snapshot som den första och att resulterande semantiska fakta följer `overlayB`. Upprepa med olika `overlay.Targets` men samma närvaro av overlay.
6. Kontrollera samtidig in-flight-beräkning och test med `-race`.

### Expected and actual behavior

- **Expected:** Semantisk återanvändning kräver ekvivalens för hela den relevanta käll-/konfigurations-/target-snapshoten, inte bara att overlay förekommer.
- **Actual (kodmässigt):** Två olika icke-tomma overlays kan ge samma cacheträff om övriga jämförda fält sammanfaller.

### Scope / regelverksstöd

- `rules/tooling/lsp.md` — Snapshots: dokumentversioner, manifest, aktiv target, plan, policy, core/stdlib och modulgraf ingår i snapshot.
- `rules/compiler/incremental_compilation.md` — kompatibilitet och beroendeinvalidering.

### Föreslagen åtgärd

Använd en deterministisk snapshot-/beroendefingerprint som täcker alla relevanta indata, alternativt bind cachen till en kanonisk workspace-snapshot-identitet. Håll analysresultat åtskilda vid versions-, import-, target- och policyförändringar. Detta ska samordnas med den nyligen införda delade diagnostikanalysen, **inte** bli en andra cache med annan semantisk sanning.

**Kodreferenser:**  
- [analysis_cache.go L40–49, L85–129](https://github.com/YoePro/sec/blob/e37b17eeebe33f521234c0c8bf3963b30005a1dd/cmd/lsp/analysis_cache.go#L40-L129)  
- [lsp.md — Snapshots](https://github.com/YoePro/sec/blob/e37b17eeebe33f521234c0c8bf3963b30005a1dd/rules/tooling/lsp.md#L979-L1010)

**Dublettkontroll:** `tooling.lsp-responsiveness` beskriver redan cache och cachebegränsningar. Detta är en **specifik korrekthetsbugg**, inte en ny generell uppgift om att bygga cache.

---

## LSP-AUD-20261106-03 — Olöst import undertrycker oberoende semantiska fel

**Status:** Open / statiskt verifierad felundertryckning; körd reproduktion återstår  
**Severity:** `medium`  
**Kind:** `lsp-diagnostic-suppression-bug`  
**Affected phase:** LSP-diagnostik (enskild fil och modulbatch)  
**Area:** `tooling-lsp`  
**Governance integration:** `tooling.lsp-import-resolution` och `tooling.lsp-module-diagnostic-scheduling` (`governance/tooling_lsp.yaml`)

### Report

Om `prepareProgramForLSP()` rapporterar ett eller flera importfel slutar `analyze()` i `main.go` innan semantisk analys körs. Modulbatchen i `diagnostics_batch.go` kör `analyzer.Analyze(first.program)` men väljer **att inte lägga till någon** Sema-diagnostik om importfel finns.

Därmed verkar ett olöst importberoende kunna gömma oberoende fel i aktiva deklarationer, exempelvis ett lokalt typfel som inte refererar till den saknade modulen. Att undvika kaskadfel är korrekt; att undertrycka alla oberoende fel gör däremot redigeringsstödet sämre.

### Reproduction / regressionstest

Skapa en testfixture med en omöjlig import och ett oberoende semantiskt fel, t.ex.:

```sec
module main

import "missing/module"

fn Main() {
    let value: bool := 42
}
```

Detta är en **testkandidat**; anpassa fixture till den aktuella parserns regler och registrerade diagnos-ID för typkonflikten. Kör både `analyze(uri, text, overlay)` och `analyzeDiagnosticBatch(...)` på samma indata. Kontrollera att importdiagnosen och det lokala, oberoende typfelet publiceras, men att följdfel från okända typer i den saknade modulen inte multipliceras.

### Expected and actual behavior

- **Expected:** Importfelet rapporteras tillsammans med oberoende semantiska fel i giltigt återstående källträd. Enbart saknade importers beroende följdfel begränsas eller märks som okända.
- **Actual (kodmässigt):** `analyze()` returnerar direkt vid `len(importErrors) != 0`; modulbatchen undertrycker Sema-resultaten under samma villkor.

### Scope / regelverksstöd

- `rules/tooling/lsp.md` — error-tolerant editing och diagnostik från compiler-owned fakta.
- `rules/compiler/parser_recovery.md` — §3(3): tooling får fortsätta partiell semantisk analys av giltiga delträd (denna regel gäller parseråterhämtning, men stödjer kravet på isolerad felrapportering).
- `rules/tooling/diagnostics.md` — diagnostik utan vilseledande kaskader.

### Föreslagen åtgärd

Lägg först till ett regressionstest för oberoende fel. Därefter gör compiler-owned analys av giltiga, frånvarande-import-oberoende delar möjlig och filtrera enbart faktiska importföljdfel. **Ta inte bara bort `len(importErrors)`-spärren utan att kontrollera falska kaskaddiagnoser.** LSP och CLI ska dela semantisk sanning.

**Kodreferenser:**  
- [main.go L3770–3808](https://github.com/YoePro/sec/blob/e37b17eeebe33f521234c0c8bf3963b30005a1dd/cmd/lsp/main.go#L3770-L3808)  
- [diagnostics_batch.go L85–123](https://github.com/YoePro/sec/blob/e37b17eeebe33f521234c0c8bf3963b30005a1dd/cmd/lsp/diagnostics_batch.go#L85-L123)  
- [diagnostics_batch_test.go L103–116](https://github.com/YoePro/sec/blob/e37b17eeebe33f521234c0c8bf3963b30005a1dd/cmd/lsp/diagnostics_batch_test.go#L103-L116) — befintligt test kontrollerar endast importdiagnosen, inte samtidig oberoende Sema-diagnostik.

**Dublettkontroll:** Redan existerande import- och modulbatchintegrationer äger beteendet. Skapa ingen parallell diagnostikimplementation.

---

## LSP-AUD-20261106-04 — Osparad diagnostikpolicy i sec.toml ignoreras av Sema

**Status:** Open / statiskt verifierad konfigurationsväg; runtime-test saknas  
**Severity:** `medium`  
**Kind:** `lsp-unsaved-configuration-bug`  
**Affected phase:** LSP:s diagnosregelkonfiguration i Sema  
**Area:** `tooling-lsp`  
**Governance integration:** `tooling.lsp-pitfall-presentation` (`governance/tooling_lsp.yaml`), relaterat `tooling.lsp-module-diagnostic-scheduling`

### Report

`newLSPAnalyzerForInputsAtDepth(...)` skapar en analysator med aktuell overlay och använder den också för att välja analysdjup. Men `SetDiagnosticRules(lspDiagnosticRules(pathFromURI(uri)))` anropar en funktion som **alltid läser** projektets `.sec/sec.toml` från disk genom `os.ReadFile()`. Funktionen får ingen `sourceOverlay` och kan därför inte läsa det osparade manifest som redan finns i workspace-snapshoten.

Det innebär att en användare som ändrar `[diagnostics.rules]` i en öppen men osparad `sec.toml` kan få analyser på den nya konfigurationens djup men med **den tidigare sparade diagnostikpolicyn**. Detta bryter förväntningen på enhetlig workspace-snapshot.

### Reproduction / regressionstest

1. Skapa ett projekt med en sparad `.sec/sec.toml` som har en giltig `diagnostics.rules`-inställning för en valbar diagnostik (använd en faktisk nyckel och nivå ur `cmd/lsp/pitfall_configuration_test.go`).
2. Öppna manifestet via `sourceOverlay.Sources` med en annan, fortfarande giltig policy, utan att skriva den till disk.
3. Kör en LSP-semantisk analys för en fil som utlöser den valda diagnostiken.
4. Verifiera att den osparade policyn används och att en motsvarande sparad ändring ger samma resultat.
5. Kontrollera återställning vid overlay-stängning och cacheinvalidering vid policybyte.

### Expected and actual behavior

- **Expected:** Alla semantiskt relevanta LSP-konfigurationsvärden kommer från samma aktuella snapshot, med osparad overlay före disk.
- **Actual (kodmässigt):** `lspDiagnosticRules()` läser enbart `os.ReadFile(...)`; den kan inte se osparade `[diagnostics.rules]`.

### Scope / regelverksstöd

- `rules/tooling/lsp.md` — Snapshots, Configuration: diagnostic policy ingår i snapshot och ändringar ska uppdateras utan serveromstart.
- `rules/tooling/diagnostics.md` — projektstyrd diagnostikpolicy.

### Föreslagen åtgärd

Lös diagnostikpolicyn från samma versionerade manifest/overlay som övrig analyskonfiguration och inkludera policyn i analysens cacheidentitet. Behåll precedence- och valideringsregler enligt befintlig policyimplementation. Regressionstest bör täcka osparat manifest, sparat manifest och återgång från overlay.

**Kodreferenser:**  
- [analysis_configuration.go L29–56](https://github.com/YoePro/sec/blob/e37b17eeebe33f521234c0c8bf3963b30005a1dd/cmd/lsp/analysis_configuration.go#L29-L56)  
- [analysis_depth.go L19–45](https://github.com/YoePro/sec/blob/e37b17eeebe33f521234c0c8bf3963b30005a1dd/cmd/lsp/analysis_depth.go#L19-L45) — jämförbar konfigurationsläsning från overlay.  
- [lsp.md — Configuration](https://github.com/YoePro/sec/blob/e37b17eeebe33f521234c0c8bf3963b30005a1dd/rules/tooling/lsp.md#L3460-L3503)

**Dublettkontroll:** Befintlig `tooling.lsp-pitfall-presentation` beskriver config-reload. Denna rapport gäller specifikt **osparat manifest i Sema:s diagnosregelpolicy**, inte generell saknad config-reload.

---

## Importkontroll / definition av klart

Importen är klar först när:

- Alla fyra stabila `source_id` finns i `bug-reports/index.yaml` **eller** är explicit mappade till redan existerande motsvarande buggar utan dubletter.
- Varje ny post har ett kanoniskt `BUG-NNNNNN`-ID, `status: open`, spårbar rapportfil och befintlig governanceägare.
- Det framgår att fynden är **statiskt granskade**, inte körda regressionstester.
- Om aktuell `main` redan har fixat något av dem ska Codex dokumentera den verifierade fixen/testbeviset enligt buggrapportsliggarens lifecycle i stället för att felaktigt lägga in en ny öppen bugg.
- Inga källkodsförändringar, språkbeslut eller nya governance-integrationer görs bara för att importera rapporten.
