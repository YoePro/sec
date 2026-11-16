# Uppgift till Codex: P19 del 1 — arenareferenser når Semantic IR

Datum: 2026-11-16. Ägaren har bett oss stänga P17 som bas för nästa steg och
föreslog att du tar en del av arbetet. Det här går före P20 tills det är klart.

## Bakgrund

P17 har en enda återstående form i P17-41: en `defer` som fångar en
referens-holder med dynamisk epok (`validity = dynamic-epoch`). Kroppen måste
validera holdern när den körs, inte när den registreras.

I Sec 0.1 är arenareferenser de enda dynamic-epoch-referenserna. Sema sätter
dem i `arenaOriginOfPlace` i `internal/sema/reference_plans.go` (provenance
`arena`, `InvalidationDomain`, `EpochDependency`). Ingen sådan referens når
Semantic IR i dag, eftersom `ValidateAllocationLoweringReadiness` stoppar varje
arenaallokering: "the selected allocation context/domain has no preserving
backend representation". Prova själv:

```sec
fn F() Result[void, AllocationError] {
    let mut arena: Arena := Arena {}
    let first := try arena.Alloc[byte](1u)
    discard first
    return Ok()
}
```

Din del är att få arenaskapande och `Alloc` till Semantic IR, med
dynamic-epoch-fakta på referenserna. Claude gör defer-fångsten parallellt
(P19-21-skärningen) och skriver källkodstester ovanpå det du levererar.

## Omfattning (package19-todo.md)

Gör de här punkterna, eller den del av dem som behövs för acceptansen nedan.
Markera bara en punkt som klar när den är klar i sin helhet. Annars skriver du
en daterad uppdateringsnotis under punkten.

1. **P19-01–03** — baslinje, synkgranskning av
   `rules/mlir/normative-sync/sec_arena_allocation_sync_v1.md` och bekräftelse
   mot nuvarande Sema. Kort och faktabaserat.
2. **P19-04–08, P19-12** — de Sema-planer som byggaren behöver: backing-sort,
   typrestriktioner för säker allokering, `AllocationError`/`Result`-kontraktet,
   Reset och Release (beroenderegel, övergång) samt läsfrågor. Återanvänd
   befintlig vokabulär i `internal/sema/storage_relations.go`
   (`BackingRelation`, `ReclamationAuthority`, `InvalidationDomainID`,
   `AllocationIdentityID`). Inga parallella ID-system.
3. **P19-14/15** — Semantic IR-typen Arena och operationerna (package19
   §§ 84–96). Mutation sker via Place och skapandet ger ett ägt värde.
   `ArenaResetOp` och `ArenaReleaseOp` ska avancera eller avsluta arenadomänens
   epok. Använd gärna de allmänna operationerna `storage.advance-epoch` och
   `storage.end-domain` från P18 (`internal/ir/semantic/dynamic_array_storage.go`)
   i stället för egna kopior, om reglerna tillåter det.
4. **P19-18** — arenareferenser från källkod bär dynamic-epoch-fakta och går
   genom den befintliga validitets-CFG:n (`ref.is-valid` → `ref.deref-place`
   med guard, `fail.reference-generation`). Därmed är
   `TestPackage15DynamicReferenceUseValidatesOnce` inte längre bara white-box.
   Lägg till ett källkodstest bredvid.
5. **P19-20** — byggaren lowerar skapande, `New`, `Alloc`, Reset, Release och
   implicit destruktion enbart från P19-planerna. Allt som saknar plan ger ett
   paket-19-fel utan partiell modul. Smalna av readiness-grinden per operation:
   använd `ValidateAllocationLoweringReadinessPreserving` med en egen
   preserve-funktion, kombinerad med den befintliga `IsDynamicArrayAppendGrowth`
   (se `Build` i `internal/ir/semantic/builder.go`). Grinden ska fortfarande
   stoppa allt som inte bevaras.

Inget av följande ingår: schema 15, C++-dialekten, emitter (sektion E),
lowering (F), defer-interaktionen P19-21 (Claude) eller P16/P18-integrationen
P19-19.

## Acceptans

- Exemplet ovan och en variant med `Reset` mellan två allokeringar bygger
  Semantic IR i paket 19 som verifierar.
- Minst en referens från `Alloc` dereferereras eller läses i källkoden, och IR
  visar `ref.is-valid` med guard före `ref.deref-place`. Efter `Reset` ska en
  gammal referens vara ogiltig enligt verifieraren eller Sema.
- Paket 18 och lägre ger fortfarande ett tydligt unsupported-fel för
  arenaoperationer.
- Gröna riktade tester: `internal/sema`, `internal/ir/semantic` och
  `internal/lowering/secmlir`. Kör inte hela `cmd/compiler` eller `cmd/lsp`.
  Använd `go test -p 4` med `GOTMPDIR=$HOME/.cache/sec-gotmp`.

## Samordning

- Posta din working set i `mlir/compiler-packages-handoff.md` innan du börjar
  och resultaten efteråt.
- Claude äger under tiden `internal/ir/semantic/defer_producer.go`,
  `reference_use.go`, `cleanup_ops.go`, `union_empty_state.go` och
  `mlir/lib/Analysis/VerifyCleanups.cpp`. Rör dem inte. Om du behöver en hook
  där, skriv det i handoff-filen.
- I `builder.go` och `verifier.go`: bara små dispatch-hooks.
- Skapa governance-posten `lowering.sec-mlir-package19` i
  `governance/lowering.yaml`, med samma form som paket 18. Varje stängd punkt
  får en egen implemented-post; det som är kvar läggs som remaining-poster.
- Uppfinner du inget språkbeteende. Om reglerna inte räcker: lägg en MD i
  `missing-decisions.yaml` (uppdatera `last_id` och `updated`) och stanna vid
  den punkten.
- Ingen commit.
