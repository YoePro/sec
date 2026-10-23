# Uppgift till Codex: MD-077 steg 5a — Sec MLIR-dialekten för register och hårdvaruoperationer

## Bakgrund
MD-077 är integrerad i regelböckerna. Underlaget:
- beslutet: `rules/corrections/applied/md077-register-semantic-ir-final-decision-20261022.md`, särskilt § 9, § 10, § 11(4)–(5) och § 19;
- den normativa dialekttexten: `rules/mlir/sec_mlir_dialect.md`, avsnittet "### v11.41.1 Register values and hardware operations (MD-077 early admission)" samt v12.1-noten om arv.

Claude gör parallellt steg 3–4 i Go, alltså Semas planer, Semantic IR och verifieraren. Din del är bara C++/ODS-sidan i `mlir/`. Emittern, alltså Go-koden som skriver MLIR, ingår inte. Den kommer när Semantic IR-operationerna finns.

## Omfattning

### 1. Typer
- `!sec.register<"module::TypeName">` (detached nominellt registervärde).
- `!sec.hw_instance<"module::TypeName">` (instans/auktoritet).

De ska vara distinkta från varandra och från heltal. Lägg dem i `SecTypes.td`/`SecTypes.cpp` enligt befintlig stil.

### 2. Registerdefinitionstabell och planposter på modulnivå
Följ samma konvention som befintliga struct- och enumdefinitioner i dialekten. Tabellen ska innehålla:
- ordnade fältnamn/ID:n;
- bitbredd och offset;
- representation;
- åtkomstkontrakt;
- nominell identitet.

Hårdvaruoperationernas verifierade planposter ska vara modulomfattande, unika och deterministiskt ordnade.

Om en stavning inte går att härleda ur reglerna eller ur befintlig konvention: stoppa och rapportera frågan. Hitta inte på en egen.

### 3. Operationer
Lägg dem gärna i en egen `SecRegisterOps.td` plus en `RegisterOps.cpp`, enligt mönstret i `SecSliceOps.td`/`SliceOps.cpp`. Det gäller:
- `sec.register.extract`
- `sec.register.update`
- `sec.hw.bind`
- `sec.hw.observe`
- `sec.hw.request_write`
- `sec.hw.shadow_observe`
- `sec.hw.shadow_intent`
- `sec.hw.shadow_invalidate`
- `sec.hw.commit`

Operander, resultat och obligatoriska attribut följer § 19(4)–(8).

### 4. Effekter
- `sec.register.*` ska vara Pure.
- `sec.hw.*` (utom `bind`, som inte gör någon fysisk I/O) ska ha minnes- och sidoeffekter som förhindrar eliminering, duplicering, spekulation, flytt och sammanslagning, även när resultatet är oanvänt (§ 19(11)).

### 5. Verifierare (§ 19(12))
Verifieraren ska kontrollera:
- exakta operand- och resultattyper;
- att typnamnet finns i registertabellen;
- att `sec.field_path` är giltig för typen, med rätt fältbredd, intervall och enum;
- att alla obligatoriska metadata-nycklar finns och att plan- och instansreferenserna löser upp;
- att `sec.place.read`, `sec.place.write` och `sec.ref.borrow_*` direkt på en fysisk resurs eller endpoint avvisas (§ 19(10)).

En saknad plan ger ett diagnostiskt fel (fail-closed).

### 6. Schemagrindar
- Schema 1–10 avvisar alla konstruktioner ovan.
- Schema 11 och 12 accepterar dem.
- Default-schemat ändras inte.

### 7. Lit-tester under `mlir/test/Dialect/`
- Rundturer parse → print → parse för varje typ och operation.
- Ett negativt test per verifieringsregel.
- Ett schema-10-avvisningstest.

## Regler för arbetet
- Regelböckerna är auktoritativa.
- Rör inte `internal/` (Go-koden) eller `cmd/`. Där arbetar Claude med MD-077 steg 3–4.
- Commita, stagea eller kör `git mv` aldrig. Användaren commitar.
- Bygg och testa med `cmake --build build/sec-mlir --target check-sec-mlir -j4`. Baslinjen är 139/139 gröna, och den ska förbli grön plus dina nya tester.
- Uppdatera governance-posten `lowering.sec-mlir-package15` i `governance/` (steg 5) och implementeringsnoten i arkivfilen ovan när delen är klar. Markera inte MD-077 som helt implementerad, eftersom emittern och P15-42/42a återstår.

## Rapport tillbaka
- Vilka filer som har ändrats eller skapats.
- Lit-resultatet.
- Exakt textform för typer, attribut och tabeller, så att Go-emittern kan skrivas mot den.
- Eventuella frågor som behöver ett beslut av användaren.

## Obs: heltal ↔ register
Konvertering heltal ↔ register (`Packet(raw)`) saknar operation i MD-077-vokabulären. Den är loggad som MD-110 i `missing-decisions.yaml`. Implementera ingen sådan operation. Den väntar på användarens beslut.
