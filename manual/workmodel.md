Ja. Nu när jag går igenom den kanoniska language-rulebook-status.md som helhet ser jag att den gamla planen 32–45 är för liten. Den missar flera stora, redan normativa block: shaped types, allocation/Arena/lifetime/destruction/layout, panic/runtime checks, stora delar av plattformsmodellen och merparten av concurrency. Inventariet listar dessutom testing, formatter, diagnostics och LSP som egna områden.

Jag tycker därför att vi ska sluta försöka få manualen att sluta på 45. Det finns inget värde i det. En komplett språkmanual landar snarare runt 70–72 sidor med den detaljnivå vi nu håller.

## Förslag på den återstående kanoniska manualstrukturen
| Nr | Fil | Kapitel | Läget |
|---|---|---|---|
| 32 | 32-visibility.html | Visibility and Scopes | Skriven |
| 33 | 33-result-option.html | Result and Option | Skriven |
| 34 | 34-error-handling.html | Errors and try | Skriven | 
| 35 | 35-panic.html | Panic, Assertions and Runtime Checks | Partly - Krävs; explicit panic-syntax har fortfarande en öppen detalj |
| 36 | 36-defer.html | Defer and Cleanup | Skriven |
| 37 | 37-attributes.html | Attributes | Skriven | 
| 38 | 38-conditional-compilation.html | Conditional Compilation and Build Selection | Skriven | 
| 39 | 39-compile-time-evaluation.html | Compile-Time Evaluation | Skriven | 
| 40 | 40-allocation.html | Memory Allocation | Skriven | 
| 41 | 41-arena.html | Arenas | Skriven | 
| 42 | 42-lifetimes.html | Lifetimes | Skriven | 
| 43 | 43-destruction.html | Destruction | Skriven | 
| 44 | 44-layout.html | Memory Layout and Representation | Partly - Krävs; några explicita layout-syntaxdetaljer är fortfarande öppna |
| 45 | 45-collections.html | Collections | Skriven, med list, map, set; arrays behöver inte upprepas  | 
| 46 | 46-shaped-types.html | Shaped Types | Absolut nödvändig; några designkanter återstår  | 
| 47 | 47-lambdas-closures.html | Lambdas and Closures | Partly - Krävs; skriv när closure-regelboken/statusen är helt synkad | 
| 48 | 48-effects.html | Effects and Verified Guarantees | Skriven | 
| 49 | 49-platforms.html | Platforms and Target Profiles | Skriven | 
| 50 | 50-hardware-access.html | Fixed Addresses, Volatile and Hardware Access | Skrive behöver uppdateras. | 
| 51 | 51-interrupts.html | Interrupts and ISRs | Skriven | 
| 52 | 52-inline-assembly.html | Inline Assembly | Skriven | 
| 53 | 53-initialization.html | Program Initialization and Shutdown | Skriven | 
| 54 | 54-concurrency.html | Concurrency | Skriven, paraplykapitlet | 
| 55 | 55-tasks.html | Tasks | Skriven | 
| 56 | 56-spawn.html | Spawn | Skriven | 
| 57 | 57-await.html | Await and Task Outcomes | sync-behov | 
| 58 | 58-threads.html | Threads and Thread-Local Storage | Threads är klar; TLS har sync-behov | 
| 59 | 59-cancellation.html | Cancellation and Context | Skriv nu | 
| 60 | 60-scheduling.html | Scheduling, Blocking and Structured Concurrency | Vänta tills sync-required-böckerna är synkade | 
| 61 | 61-channels-select.html | Channels and Select | Select klar, Channels behöver sync | 
| 62 | 62-events.html | Events | Vänta på sync | 
| 63 | 63-mutexes.html | Mutexes and Locks | Skriv nu | 
| 64 | 64-atomics.html | Atomics and Memory Order | Skriv nu | 
| 65 | 65-processes.html | Processes and Command | Skriv nu | 
| 66 | 66-ipc.html | Inter-Process Communication | När IPC-regelboken du arbetar med är färdig | 
| 67 | 67-concurrency-safety.html | Transferability, Data Races and Deadlocks | Skriv nu | 
| 68 | 68-testing.html | Testing | Skriven | 
| 69 | 69-analysis.html | Static Analysis and sec analyse | Skriv nu | 
| 70 | 70-formatting.html | Formatting | Skriv nu | 
| 71 | 71-diagnostics.html | Diagnostics | Krävs; centrala rulebooken är fortfarande sync-required | 
| 72 | 72-lsp.html | Editor and Language Server | Skriv sist; LSP-rulebooken är Living | 
|---|---|---|---|

Det här motsvarar väldigt väl den faktiska uppdelningen i rulebook-inventariet: memory-blocket har separata normativa böcker för allocation, Arena, lifetime analysis, destruction, storage och layout; errors har error handling, panic och runtime checks; concurrency har task/thread/process samt synchronization och safety-analys; platform har target profiles, volatile/hardware, interrupts och inline assembly.

## Några viktiga beslut i den kartan

Shaped Types saknas definitivt idag. Vi har bara introducerat dem i 03-types.html. Den kanoniska rulebooken är stor nog för ett eget kapitel och omfattar vector, matrix, tensor, tensor_view, Shape, Strides, layouts, broadcasting, algebra och contraction. Det bör absolut bli 46-shaped-types.html. Samma sak gäller vanliga collections: list, map och set behöver riktig behandling och kan inte anses dokumenterade bara för att iteration eller typer nämns på andra sidor.

Memory behöver fyra nya tydliga fokus utöver det vi redan gjort. 22-storage gav modellen, och 23–26 gav references/ownership/copy-move/borrowing. Men vi saknar fortfarande den användarsynliga betydelsen av allocation, Arena, lifetimes och destruction, plus layout. De ska inte tryckas in i en enda “Memory”-sida; rulebooks är uttryckligen separerade eftersom frågorna är olika.

Errors behöver tre nivåer. Jag skulle behålla Result[T,E] och Option[T] tillsammans på 33, eftersom de är grundläggande carriers. Sedan 34 för error, typed errors, fallible operations och try. Därefter 35 för panic/assert/runtime checks. panic.md är redan mycket större än “hur skriver jag assert”; den omfattar panic domains, cleanup, containment, task/thread/process outcomes och no-panic-verifiering. Den enda anledningen att inte skriva 35 direkt är att exakt explicit panic-syntax fortfarande är markerad som öppen.

Concurrency behöver vara betydligt större än gamla 39–42. Sec har nu tre helt skilda execution identities: Task är den logiska/gröna exekveringen, Thread den fysiska tråden och Process en separat process-isoleringsdomän. Till detta kommer cancellation/context, scheduling/blocking, channels/select/events, mutexes, atomics/memory order, transferability, data-race/deadlock analysis och IPC. Det går inte längre att göra rättvisa åt detta med fyra sidor.

Och jag skulle ändra den gamla idén “Build Directives”. Det är inte längre ett bra kanoniskt kapitelnamn. Den aktuella språkmodellen är compiler-known attributes, där @target(...) och @when(...) äger source selection och conditional compilation. Den gamla #target(...)-formen finns endast som en implementerad kompatibilitetsform under migration; den kanoniska syntaxen är @target. Därför föreslår jag 37-attributes.html följt av 38-conditional-compilation.html.

Det vi inte behöver skapa separata nya manualkapitel för
| Rulebookområde | Manualbehandling |
|---|---|
| types/default_values.md | Auditera och komplettera 03 Types, 05 Structs och 19 Arrays |
| foundations/operators.md | Auditera 02 Basic Syntax; operators/preference finns redan där |
| declarations/static.md | 16 Impl + module/visibility-materialet |
| control-flow/discard.md | Redan starkt täckt i 03 Types och framför allt 24 Ownership |
| memory/reference_model.md | 23 References |
| memory/memory_model.md / storage.md | 22 Values, Places and Storage |
| memory/raw_pointers.md | 30 Unsafe and Raw Pointers |
| platform/abi.md | 31 FFI + 44 Layout + 49 Platform |
| compiler analyses | Sammanför användarsidan i 69 Static Analysis i stället för ett kapitel per analys |
| core-library/compiler-known-members | Fördelas på språktypkapitlen; API-detaljer hör också hemma i Core/Stdlib-dokumentation |
| Semantic IR, compiler pipeline, monomorphization, Sec-MLIR | Inte språkmanualen; separat compiler/developer documentation |
| stdlib | Separat Stdlib-sektion, som vi redan håller på med |

Det sista är viktigt: den kanoniska inventeringen innehåller mycket mer än vad en språkmanual ska återge sida för sida. Compiler pipeline, Semantic IR, MLIR, parser recovery, monomorphization och intern linking-modell är normativa för implementationen, men de ska inte förvandla användarmanualen till en kompilatorhandbok. rules/README.md skiljer också uttryckligen språk-, memory-, compiler-, MLIR-, platform-, tooling- och library-ansvar åt.

Sedan finns tre dokument som fortfarande är planerade men inte skrivna i canonical inventory: debug_information.md, compiler_testing.md och incremental_compilation.md. Jag skulle inte reservera språkmanualnummer för dem nu. De hör huvudsakligen hemma i framtida compiler/tooling-dokumentation. IPC är också formellt planerad/deferred i inventariet just nu, även om vi vet att du aktivt skriver den, så 66-ipc.html ska helt enkelt vänta tills den normativa IPC-boken är klar.

Så jag tycker vi har hittat en mycket bättre helhet: 01–31 är vår färdiga bas, 32–53 stänger själva språk/memory/error/platform-delen, 54–67 är concurrency-blocket och 68–72 är tooling.

Och jag skulle faktiskt låsa den här kartan nu innan vi skriver 32. Den gör att vi slipper upptäcka vid sida 44 att exempelvis Arena, shaped types eller processes inte längre får plats.