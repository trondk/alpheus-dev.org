# Alpheus — implementeringsspecifikation (Go)

Status: udkast, 2026-10-29. Trin 1–3 i afsnit 15 er bygget: API-typer og
CRD, chunks og lager, og komponent-SDK'et med lokal testmode. Agent og
controller findes ikke endnu, så intet kører på Kubernetes.

Kravene GO-88 til GO-124 gør Alpheus formatuafhængigt: chunks er filer,
som Alpheus flytter uden at læse dem, og Parquet og Arrow er ude af
Alpheus. Koden fra trin 1–3 tilpasses disse krav, før agenten bygges.

Denne specifikation beskriver, hvordan Alpheus skrives i Go. Den bygger på
den funktionelle specifikation "Alpheus — specifikation" (StreamRun-CRD,
komponent-kontrakt, chunk-protokol, signal-drivere, integration med MCPipe,
drift og accepttest) og gentager ikke kravene derfra. Krav-ID'er herfra
starter med `GO-`.

Krav bruger SKAL og BØR.


## 1. Overblik

Alpheus består af fire programmer og ét bibliotek, alle i ét Go-modul:

| Del | Binær / pakke | Kører som |
|---|---|---|
| Controller | `alpheus-controller` | Deployment i `alpheus-system` |
| Agent | `alpheus-agent` | Native sidecar i hver trin-pod |
| CLI | `alpheus` | Brugerens maskine eller CI |
| Signal-hub | indbygget i controlleren | Del af `alpheus-controller` |
| Komponent-SDK | `alpheus-dev.org/alpheus/sdk` | Importeres af komponenter |


## 2. Modul, sprog og byggeregler

| ID | Niveau | Krav |
|---|---|---|
| GO-1 | SKAL | Ét Go-modul med stien `alpheus-dev.org/alpheus`. Stien serveres som vanity import fra hjemmesiden (se `www/SPEC.md`), så modulet ikke er bundet til GitHub eller Forgejo. |
| GO-2 | SKAL | Go-versionen fastlåses i `go.mod` (`go` og `toolchain`). Projektet følger den seneste stabile Go-udgivelse og understøtter den forrige. |
| GO-3 | SKAL | Alle binærer bygges med `CGO_ENABLED=0` som statiske binærer. |
| GO-4 | SKAL | Byggeriet er reproducerbart: binærer bygges med `-trimpath` og `-buildvcs=true`, og image-lag og arkiver bruger fast `SOURCE_DATE_EPOCH` (commit-tidspunktet). To byggerier af samme commit giver samme binær og samme image-digest. |
| GO-5 | SKAL | Container-images bygges på `gcr.io/distroless/static` eller `scratch`, kører som non-root (UID 65532) og har read-only rodfilsystem. |
| GO-6 | SKAL | Images bygges til `linux/amd64` og `linux/arm64`. |
| GO-7 | SKAL | Hver udgivelse har en SBOM (SPDX) og signerede images (cosign, keyless via OIDC i CI). |
| GO-8 | SKAL | Versionering følger SemVer. CRD-versionen (`v1alpha1`, `v1beta1`, `v1`) er uafhængig af programversionen. |
| GO-9 | BØR | `goreleaser` bygger binærer, images, SBOM og checksums i én konfiguration. |
| GO-88 | SKAL | `go.mod` ligger i repo'ets rod, så modulstien `alpheus-dev.org/alpheus` peger på roden af repo'et. |
| GO-89 | SKAL | Udgivelser tagges `vX.Y.Z` på `main`. CI kontrollerer efter hver udgivelse, at `go list -m alpheus-dev.org/alpheus@vX.Y.Z` finder den. |


## 3. Repo-struktur

Go-modulet ligger i repo'ets rod (`go.mod` i roden), så modulstien
`alpheus-dev.org/alpheus` svarer direkte til repo'et. Testklyngen ligger i
`vagrant/` og hjemmesiden i `www/`; Go ignorerer mapper uden Go-filer.
Mapper markeret *(findes)* er bygget; resten er planlagt.

```
alpheus/
├── SPEC.md                denne specifikation (findes)
├── Makefile               generate, proto, test, test-crd, test-s3, test-agent-s3 (findes)
├── buf.gen.yaml           kodegenerering fra proto/ (findes)
├── api/v1alpha1/          StreamRun-typer med kubebuilder-markører (findes)
├── cmd/
│   ├── alpheus/           CLI
│   ├── alpheus-controller/ controllerens binær (findes)
│   └── alpheus-agent/     agentens binær (findes)
├── internal/
│   ├── controller/        reconcile-løkke, pod-spec, status, signal-CA og hub (findes)
│   ├── agent/             socket-server, ind- og udgange, genstart (findes)
│   ├── chunk/             navngivning, manifest, ack-filer, checksum (findes)
│   ├── pb/                genereret Go-kode fra proto/ (findes)
│   ├── signal/            driver-interface (findes)
│   │   ├── builtin/       gRPC-hub i controlleren og klient i agenten (findes)
│   │   └── stomp/         STOMP-driver til fx Artemis
│   ├── store/             S3-klient bag et lille interface (findes)
│   └── version/
├── sdk/                   offentligt komponent-SDK, lokal mode og agent-mode (findes)
├── proto/                 component.proto og signal.proto (findes)
├── config/                genererede CRD-, RBAC- og webhook-manifester (CRD og RBAC findes)
│   ├── crd/bases/         StreamRun-CRD'en (findes)
│   ├── manager/           controllerens Deployment, hub-Service og RBAC-binding (findes)
│   ├── default/           CRD, RBAC og controller samlet, til kustomize (findes)
│   └── samples/           eksempler, bl.a. testcase03 (findes)
├── hack/                  boilerplate, test-crd.sh, images.sh og e2e.sh (findes)
├── charts/alpheus/        Helm-chart
├── test/
│   ├── components/        testkomponenter: bench (findes), filter og sum i CSV (findes)
│   ├── e2e/               testcase01 til testcase06 (findes), og fejltests
│   └── internal/table/    CSV og decimaltal for testkomponenterne (findes)
├── docs/                  kilde til dokumentationen på hjemmesiden
├── vagrant/               testklyngen: Talos i en VM med Garage og Zot (findes)
├── www/                   hjemmesidens specifikation og Hugo-site (findes)
├── GOVERNANCE.md
├── SECURITY.md
└── LICENSE                Apache 2.0
```

Alt under `internal/` er privat. Kun `api/` og `sdk/` er offentlig API og
følger SemVer-garantier.


## 4. Afhængigheder

Hver afhængighed skal godkendes, før den tages i brug. Kolonnen Status
siger, om den er godkendt og i brug (med versionen i `go.mod`), eller
stadig kun foreslået.

| Formål | Bibliotek | Licens | Status |
|---|---|---|---|
| Kubernetes-klient og controller | `sigs.k8s.io/controller-runtime`, `k8s.io/client-go` | Apache 2.0 | Godkendt, i brug (v0.25.2; k8s.io v0.37.0) |
| CRD-generering | `sigs.k8s.io/controller-tools` (kun build) | Apache 2.0 | Godkendt, i brug (v0.23.0, `go tool`) |
| S3 | `github.com/minio/minio-go/v7` | Apache 2.0 | Godkendt, i brug (v7.4.1) |
| gRPC | `google.golang.org/grpc`, `google.golang.org/protobuf` | Apache 2.0 / BSD | Godkendt, i brug (v1.83.2 og v1.36.12) |
| Protobuf-generering | `protoc-gen-go`, `protoc-gen-go-grpc` (kun build, `go tool`), kørt af `buf` | BSD / Apache 2.0 | Godkendt sammen med gRPC (v1.36.12 og v1.6.2) |
| STOMP | `github.com/go-stomp/stomp/v3` | Apache 2.0 | Foreslået (trin 9) |
| CLI | `github.com/spf13/cobra` | Apache 2.0 | Foreslået (trin 10) |
| Metrics | `github.com/prometheus/client_golang` | Apache 2.0 | Foreslået |
| Logning | `log/slog` fra standardbiblioteket | BSD | I brug |
| Test | `sigs.k8s.io/controller-runtime/pkg/envtest`, `setup-envtest` (kun build, `go tool`) | Apache 2.0 | Godkendt, i brug (v0.25.2; Kubernetes 1.37.0) |

| ID | Niveau | Krav |
|---|---|---|
| GO-10 | SKAL | Kun licenser, der er forenelige med Apache 2.0. Ingen GPL, AGPL, BSL eller "community"-licenser. |
| GO-11 | SKAL | `govulncheck` kører i CI og blokerer en udgivelse ved kendte sårbarheder i brugt kode. |
| GO-12 | BØR | minio-go vælges frem for AWS SDK, fordi den er mindre og testet mod S3-kompatible lagre som Garage. Begge ligger bag `internal/store`, så valget kan ændres. |

Parquet og Arrow (`github.com/apache/arrow-go`) er ikke en afhængighed
(GO-101). Parquet og DuckDB findes kun i MCPipe.


## 5. API-typer (`api/v1alpha1`)

| ID | Niveau | Krav |
|---|---|---|
| GO-13 | SKAL | API-gruppen er `alpheus-dev.org`, versionen `v1alpha1`, kind `StreamRun`, kort navn `sr`. |
| GO-14 | SKAL | Typerne har kubebuilder-markører for validering (påkrævede felter, mønstre, min/max), så CRD'en genereres med OpenAPI-skema. |
| GO-15 | SKAL | `spec` er uforanderlig efter oprettelse, bortset fra `spec.cancel`. Håndhæves med CEL-validering i CRD'en (`x-kubernetes-validations`), ikke kun i controlleren. |
| GO-16 | SKAL | Status-subresource er slået til, og kun controlleren skriver status. |
| GO-17 | SKAL | Printer-kolonner viser fase, antal trin, alder og eventuel fejl i `kubectl get sr`. |
| GO-18 | BØR | Conditions følger `metav1.Condition` (`Ready`, `Progressing`, `Degraded`). |

Felter i CRD'en fra kravene GO-78 til GO-124:

| Felt | Krav |
|---|---|
| `spec.resultsTTL` (standard 7 dage, `0` = aldrig) | GO-78 |
| `spec.steps[].failurePolicy` (`stop` eller `continue`) og fasen `PartiallySucceeded` | GO-97 |
| `spec.chunks.maxBytes` | GO-105 |
| `spec.steps[].scratch` (standard 10 GiB) | GO-106 |
| `spec.steps[].maxOpen` (standard 8) | GO-110 |
| `spec.steps[].prefetch` (standard 2) | GO-111 |
| `spec.steps[].inputOrder` (`arrival` eller `deterministic`) | GO-112 |
| `spec.edges[].port` (standard `default`) | GO-108 |
| `status.steps[].replicas[].failedChunks` | GO-97 |
| `status.steps[].counts`, `errors`, `warnings`, `errorsDropped`, `errorLog` | GO-115, GO-119, GO-120 |
| `spec.chunks.maxRows` fjernes | GO-86 |

### 5.1 Replikaer

Et trin kan køre i flere kopier, der deler input-chunks mellem sig:

```yaml
steps:
  - name: generate
    image: registry.example/generate@sha256:…
  - name: filter
    image: registry.example/filter@sha256:…
    replicas: 10
    settings: {field: country, in: [DK]}
  - name: sum
    image: registry.example/sum@sha256:…
edges:
  - {from: generate, to: filter}
  - {from: filter, to: sum}
```

Hver replika er sin egen pod med sin egen agent. Hver producent-replika
skriver sin egen strøm af chunks på en kant (7.3), så der stadig kun er
én skriver pr. manifest. Et trin uden `replicas` har én replika.

| ID | Niveau | Krav |
|---|---|---|
| GO-75 | SKAL | `spec.steps[].replicas` (standard 1, højst 64) angiver antallet af replikaer. Replikaerne nummereres 0 til `replicas`-1, og agenten får sit nummer som miljøvariabel. |
| GO-76 | SKAL | Chunk nummer n i strømmen fra producent-replika p går til forbruger-replika (n + p) mod `replicas` hos hvert forbrugende trin. Leddet p spreder arbejdet, når mange producent-replikaer hver laver få chunks; uden det ville alle strømme starte på samme forbruger-replika. Fordelingen er deterministisk, så en genstartet replika får præcis de samme chunks igen. |
| GO-77 | SKAL | `maxRestarts` og forsøgsnummeret (GO-67) gælder pr. replika. Et trin er afsluttet, når alle dets replikaer er afsluttet, og fejlet, når én replika har brugt alle sine genstarter. |
| GO-97 | SKAL | Har en replika brugt alle sine genstarter, og er `failurePolicy` `continue`, lukker controlleren replikaens udgående strømme. Først når pod'en er bekræftet væk (GO-67), skriver controlleren hvert udgående manifest med `eos: true`, uændrede chunks og et `epoch` én højere end replikaens sidste forsøg, så en agent, der mod forventning stadig kører, stoppes af fencing (GO-68). Input-chunks, der var tildelt replikaen, men ikke stod i dens `consumed`, tælles i `status.steps[].replicas[].failedChunks`. Forbrugerne når EOS, og runnet slutter som `PartiallySucceeded`. Med `failurePolicy: stop` stoppes hele runnet i stedet. |

### 5.2 Resultater

En udgang, som ingen kant går fra, er et resultat. Agenten skriver den som
en strøm med manifest, ligesom en kant, men under
`results/<run>/<uid>/<trin>/<udgang>/<replika>/` i stedet for
`runs/<uid>/`. `<uid>` er StreamRun-objektets `metadata.uid`.

| ID | Niveau | Krav |
|---|---|---|
| GO-83 | SKAL | Stier i lageret bruger runnets UID, ikke kun dets navn: `runs/<uid>/` og `results/<run>/<uid>/`. Slettes en StreamRun og oprettes igen med samme navn, får den en ny UID, og gamle og nye data blandes aldrig. Navnet står med i `results/` og i manifestet (`run`), så en person kan finde resultatet. |
| GO-78 | SKAL | Resultater slettes ikke af `ttlAfterFinished` eller finalizeren (GO-20). De slettes, når `spec.resultsTTL` er udløbet, regnet fra `finishedAt`. Standard er 7 dage. Værdien `0` betyder, at controlleren aldrig sletter dem, og skal angives eksplicit. |
| GO-98 | SKAL | Agenten skriver `results/<run>/<uid>/run.json` med runnets navn, namespace, UID, `finishedAt` og `resultsTTL`, før den første resultat-chunk skrives. Så kan resultater findes og ryddes, også efter at StreamRun-objektet er slettet. |
| GO-99 | BØR | `alpheus results gc` finder resultater, hvis TTL er udløbet, eller hvis StreamRun-objektet ikke længere findes, og sletter dem efter bekræftelse. Med `--dry-run` vises kun, hvad der ville blive slettet. |
| GO-79 | SKAL | `status.results` har én post pr. resultat med S3-endpoint, bucket, region, prefix, navnet på Secret'en med nøglerne (`spec.storage.secretRef`) og antal chunks, bytes og komponentens committede tællere (10.1). Det er det, der skal til for at hente resultatet, og `alpheus get` viser det. |

Secret'en i `status.results` er den samme som agenternes, med adgang til
hele bucketen. Det strider mod princippet om, at agentens nøgle kun
giver adgang til runnets eget prefix. I testmiljøet er det acceptabelt;
en særskilt læsenøgle kun til `results/` er et åbent spørgsmål (16).


## 6. Controller (`internal/controller`)

### 6.1 Reconcile-løkken

Controlleren reconciler én StreamRun ad gangen pr. worker og er
idempotent: samme input giver samme ønskede tilstand.

1. Valider grafen: navne unikke, kanter peger på eksisterende trin,
   ingen løkker. Fejl sætter fasen `Failed` med en læsbar årsag.
2. Opret Secret-referencer, ServiceAccount og NetworkPolicy for runnet,
   hvis de mangler.
3. Opret én pod pr. replika af hvert trin, hvis den mangler (se 6.2).
4. Læs pod-status og agenternes rapporter, og opdater `status`.
5. Når alle trin er afsluttet, sæt slutfasen og `finishedAt`.
6. Når `ttlAfterFinished` er udløbet, slet pods og runnets S3-prefix
   (`runs/<uid>/`, ikke `results/`, GO-78).

| ID | Niveau | Krav |
|---|---|---|
| GO-19 | SKAL | Alle objekter, et run opretter, har `ownerReferences` til StreamRun-objektet. |
| GO-20 | SKAL | En finalizer (`alpheus-dev.org/cleanup`) sikrer, at runnets S3-prefix slettes, før objektet forsvinder. |
| GO-21 | SKAL | Controlleren bruger leader election, så to replicas kan køre samtidig. |
| GO-22 | SKAL | Statusopdateringer bruger `Patch` med optimistisk låsning og rate-begrænses (højst én pr. sekund pr. run). |
| GO-23 | BØR | En validating webhook afviser ugyldige grafer, før objektet gemmes. Uden webhook sker samme validering i reconcile-løkken. |

### 6.2 Pod-spec pr. trin

- Komponenten er den eneste almindelige container.
- Agenten kører som native sidecar: en init-container med
  `restartPolicy: Always`. Den starter før komponenten og stopper efter
  den. Det kræver Kubernetes 1.29 eller nyere (beta og slået til som
  standard fra 1.29, GA fra 1.33).
- Agent og komponent deler en `emptyDir` med Unix-socketten
  (`/run/alpheus/agent.sock`) og en lokal scratch-mappe til chunks.
- Pod'en har `restartPolicy: Never`. Genstart af et fejlet trin
  håndteres af controlleren, så antallet kan tælles og begrænses
  (`spec.steps[].maxRestarts`, standard 3).
- Kun agenten får S3- og signal-legitimation. Komponenten får kun de
  Secrets, der står i `envFrom`.
- securityContext på begge containere: `runAsNonRoot`,
  `readOnlyRootFilesystem`, `allowPrivilegeEscalation: false`, alle
  capabilities fjernet, seccomp `RuntimeDefault`.

| ID | Niveau | Krav |
|---|---|---|
| GO-24 | SKAL | Agent-imaget og dets digest er konfigureret i controlleren, ikke i StreamRun. Alle trin i et run bruger samme agent-version. |
| GO-25 | SKAL | Pod-specs opfylder Pod Security-niveauet `restricted`. |
| GO-26 | SKAL | Controlleren afviser komponent-images uden digest, medmindre installationen eksplicit tillader tags (til udvikling). |
| GO-67 | SKAL | Hvert forsøg på et trin har et forsøgsnummer (`status.steps[].attempt`), som agenten får som miljøvariabel. Controlleren starter først et nyt forsøg, når det forrige forsøgs pod er bekræftet væk: pod'en er slettet, eller alle dens containere er `Terminated`. En pod i fasen `Unknown` (fx en node uden forbindelse) blokerer et nyt forsøg, indtil pod'en er slettet. |


## 7. Agent (`internal/agent`)

### 7.1 Ansvar

Agenten er den eneste del, der taler med S3 og signal-driveren. Den:

- tilbyder komponent-API'et på Unix-socketten (`proto/component.proto`),
- læser input-chunks fra S3 og lægger dem som filer i komponentens
  scratch-mappe,
- uploader komponentens output-filer som chunks til S3 og opdaterer
  manifestet,
- sender og modtager `ready`, `ack`, `eos` og `fail`,
- håndhæver backpressure (`maxUnacked`) og genstart fra manifestet,
- rapporterer tællere til controlleren og som Prometheus-metrics.

### 7.2 Komponent-API (`proto/component.proto`)

Alpheus flytter filer mellem containere uden at læse dem. Data går som
filer i pod'ens scratch-mappe; Unix-socketten bærer kun kontrolkald.

```proto
service Agent {
  rpc Next(NextRequest) returns (NextResponse);        // næste input-chunk som lokal fil, eller EOS
  rpc Create(CreateRequest) returns (CreateResponse);  // ny output-chunk som lokal fil på en udgang
  rpc Close(CloseRequest) returns (CloseResponse);     // output-chunken er skrevet færdig
  rpc Done(DoneRequest) returns (DoneResponse);        // input-chunken er færdigbehandlet
  rpc Observe(stream ObserveRequest) returns (ObserveResponse); // tællere, fejl, fremdrift (10.1)
  rpc Finish(FinishRequest) returns (FinishResponse);  // komponenten er færdig; alle udgange afsluttes
}
```

| ID | Niveau | Krav |
|---|---|---|
| GO-27 | — | Udgået: erstattet af GO-107. |
| GO-28 | — | Udgået: Alpheus kender ikke skemaer. Skemavalidering er komponentens eller MCPipes ansvar. |
| GO-29 | SKAL | Hvis komponenten ikke har kontaktet agenten (et kald eller `Observe`-strømmen) inden for en konfigurerbar tid efter start (standard 5 minutter), markeres trinnet som fejlet. En kilde kalder aldrig `Next`, så kontakt tæller i stedet for `Next`. |
| GO-125 | SKAL | `Finish` siger, at komponenten er færdig: den har læst al sin input, og output fra `OnEOS` er lukket. Agenten afviser `Finish`, hvis der er input-chunks, der ikke er læst eller ikke er `Done`. Derefter skriver agenten `eos` i alle udgående manifester (GO-80, GO-91). En komponent, der stopper uden `Finish`, har fejlet, og dens udgange afsluttes ikke. |
| GO-126 | SKAL | Hver besked på `Observe` har et løbenummer, og `Done`, `Close` og `Finish` angiver nummeret på den sidste besked, komponenten sendte før kaldet. Agenten venter på den besked, før den committer. gRPC garanterer ikke rækkefølgen på tværs af strømme, så uden nummeret kunne tællere fra en chunk komme med i den næste. |
| GO-127 | SKAL | `Next` angiver, om komponenten har registreret `OnEOS` (GO-95). Det må ikke ændre sig efter første `Next`; agenten afviser det med en brugsfejl. Agenten venter med at hente input til første `Next`, fordi genstart af et tilstandsfuldt trin begynder forfra. |
| GO-128 | SKAL | Agenten får sin konfiguration fra controlleren som JSON i `ALPHEUS_AGENT_CONFIG` (run, UID, trin, replika, forsøgsnummer, ind- og udgående kanter og trinnets grænser). S3-legitimation og certifikater til signal-hub'en kommer som separate miljøvariabler fra Secrets. Efter `Finish` kører agenten videre, til den får SIGTERM, som en native sidecar skal. |

#### Filer i stedet for batches

| ID | Niveau | Krav |
|---|---|---|
| GO-101 | SKAL | Alpheus fortolker aldrig indholdet af en chunk. Ingen pakke i Alpheus afhænger af Parquet eller Arrow, og CI kontrollerer med `go list -deps ./...`, at `github.com/apache/arrow-go` ikke er en afhængighed. |
| GO-102 | SKAL | Data mellem agent og komponent går som filer i pod'ens scratch-mappe. Unix-socketten bærer kun kontrolkald. |
| GO-103 | SKAL | Agenten henter en input-chunk fra S3 til scratch-mappen, før `Next` returnerer dens sti, og sletter filen efter `Done`. Output-chunks uploades efter `Close` og committes efter GO-90 og GO-91. |
| GO-104 | SKAL | En output-chunk er én fil fra `Create` til `Close`. Komponenten angiver filendelsen ved `Create` (små bogstaver og tal, højst 16 tegn). Den gemmes i manifestet som `ext` og i objektnavnet `part-000017.<ext>`, men agenten fortolker den ikke. |
| GO-105 | SKAL | `spec.chunks.maxBytes` er en øvre grænse. `Close` på en fil, der er større, giver en fejl med filens størrelse og grænsen. |
| GO-106 | SKAL | `spec.steps[].scratch` (standard 10 GiB) bliver `sizeLimit` på scratch-mappens `emptyDir`. Agenten afviser `Next` og `Create` med en tydelig fejl, før mappen bliver fuld, så pod'en ikke bliver evicted. |
| GO-107 | SKAL | Agenten streamer mellem S3 og disk med en fast buffer, så dens hukommelsesforbrug ikke afhænger af chunk-størrelsen. Erstatter GO-27. |

Dette ændrer den funktionelle specifikations KK-1: komponenter ser lokale
stier i deres egen scratch-mappe, men aldrig S3, beskeder eller andre
pods' filer.

#### Mange indgange

| ID | Niveau | Krav |
|---|---|---|
| GO-108 | SKAL | Hver indgående kant ender i en navngivet indgang på trinnet (`edges[].port`, standard `default`). `Next` returnerer for hver chunk: indgang, producerende trin, producent-replika, chunk-nummer, lokal sti, `ext` og `bytes`. |
| GO-109 | SKAL | `Next` kan kaldes uden indgang (næste chunk fra en hvilken som helst indgang) eller med en indgang (kun fra den). En komponent må blande de to, fx læse en lille opslagstabel helt først og derefter streame en stor indgang. |
| GO-110 | SKAL | En komponent må have op til `spec.steps[].maxOpen` input-chunks åbne ad gangen pr. replika (standard 8). Inden for en strøm afsluttes chunks med `Done` i rækkefølge (GO-71); på tværs af strømme er der ingen krav. `Next` blokerer, når grænsen er nået. |
| GO-111 | SKAL | Agenten henter op til `spec.steps[].prefetch` chunks pr. strøm ned i forvejen (standard 2), inden for scratch-grænsen (GO-106). |
| GO-112 | SKAL | `spec.steps[].inputOrder` er `arrival` (standard) eller `deterministic`. Med `arrival` leverer `Next` uden indgang chunks i den rækkefølge, de bliver klar. Med `deterministic` leverer den skiftevis én chunk fra hver strøm, sorteret efter indgang, trin og replika, og springer strømme over, der har nået EOS. Det giver samme rækkefølge i hver kørsel på bekostning af ventetid. |

### 7.3 Chunks og manifest (`internal/chunk`)

En kant hedder `<fra>-<til>`, eller `<fra>.<udgang>-<til>` for en anden
udgang end `default`. Hver producent-replika skriver sin egen strøm på
kanten, i mappen `runs/<uid>/<kant>/<replika>/` (replika 0 for et trin
uden `replicas`, GO-83). Chunks i en strøm nummereres fra 1 og uden huller.

S3 har ingen rename. Derfor er manifestet commit-punktet i stedet for en
omdøbning:

1. Producenten skriver chunk-objektet direkte under sit endelige navn
   `runs/<uid>/<kant>/<replika>/part-000017.<ext>` (GO-104).
2. Producenten skriver derefter manifestet
   `runs/<uid>/<kant>/<replika>/manifest.json` med chunken tilføjet.
3. En chunk findes kun logisk, når den står i manifestet. Forbrugere
   læser aldrig objekter, der ikke er listet.
4. Ved genstart sletter producenten objekter, der ikke står i
   manifestet (forældreløse chunks fra et nedbrud midt i en skrivning).

Det erstatter "midlertidigt navn og omdøbning" i den funktionelle
specifikations CH-1, som bør rettes til denne formulering.

Manifestformat (version 1):

```json
{
  "version": 1,
  "run": "sales-dk-0042",
  "uid": "3f0c9a6e-…",
  "edge": "read-filter",
  "replica": 0,
  "epoch": 2,
  "segments": ["manifest-0000.json"],
  "chunks": [
    {"n": 10017, "ext": "csv", "bytes": 1832211, "sha256": "…",
     "counts": {"rows": 50000},
     "src": {"edge": "source-read", "replica": 0, "n": 4711}}
  ],
  "consumed": {"source-read/0": 4711},
  "eos": false,
  "last": null
}
```

- `epoch` er forsøgsnummeret (GO-67) for den agent, der skrev manifestet.
- `ext` er filendelsen, komponenten angav ved `Create` (GO-104).
- `counts` er komponentens committede tællere for chunken (GO-115).
- `segments` lister forseglede manifest-segmenter (GO-32). `chunks`
  indeholder kun chunks, der endnu ikke er forseglet.
- `src` angiver den input-chunk, en output-chunk blev lavet af. Kilder
  (trin uden indgående kanter) udelader feltet.
- `replica` er nummeret på den producent-replika, der ejer strømmen.
- `consumed` angiver pr. indgående strøm (`<kant>/<replika>`) den højeste
  input-chunk, hvis output til denne strøm er fuldt committet, også når
  den ikke gav noget output (se 7.4). En replika ser kun på de chunks,
  der er tildelt den (GO-76).

| ID | Niveau | Krav |
|---|---|---|
| GO-30 | SKAL | Hver strøm har præcis én skriver af manifestet (agenten for den producent-replika, der ejer strømmen), og hver ack-fil har præcis én skriver (forbruger-replikaens agent, GO-36). Derfor kræves ingen låsning eller betingede skrivninger i S3. Enkeltskriveren sikres af GO-67. Den eneste undtagelse er controllerens lukning af en fejlet replikas strømme (GO-97), som sker efter bekræftet stop og med højere `epoch`. |
| GO-31 | SKAL | Checksum (SHA-256) beregnes under skrivning og verificeres under læsning. |
| GO-32 | SKAL | Når `chunks` i `manifest.json` når 10.000 chunks, skrives de til et forseglet, uforanderligt segment (`manifest-0000.json`, `manifest-0001.json` osv.). Derefter skrives `manifest.json` med segmentet tilføjet til `segments` og en tom `chunks`. Segmentet skrives altid før `manifest.json`. Så vokser den fil, der omskrives ved hver chunk, ikke ubegrænset. |
| GO-33 | — | Udgået: Parquet er ude af Alpheus (GO-101). |
| GO-68 | SKAL | Agenten afviser et manifest med lavere `epoch` end sit eget forsøgsnummer, og et manifest med højere `epoch` end sit eget får agenten til at stoppe straks. Begge dele sætter trinnet til `Failed` med årsagen "fencing". Brud på GO-67 bliver dermed opdaget i stedet for at give stille datatab. |
| GO-69 | SKAL | Lageret skal have stærk read-after-write-konsistens for nye objekter, overskrivninger og listning. Det gælder AWS S3, Garage og MinIO. Installationsdokumentationen nævner kravet, og agenten advarer ved opstart, hvis lageret ikke er på listen over testede lagre. |
| GO-34 | BØR | Lokale kopier af chunks i scratch-mappen slettes, så snart de er ackede eller skrevet. |

### 7.4 Backpressure og genstart

For et trin med udgående kanter er det de udgående manifester, der
er commit-punktet for en input-chunk, ikke ack-filen. Når en input-chunk
er behandlet, gør agenten følgende:

1. Skriver de resterende output-chunks.
2. Opdaterer hvert udgående manifest med chunkene og sætter
   `consumed[<indgående kant>/<replika>]` til input-chunkens nummer.
3. Skriver acken i ack-filen.

Et nedbrud mellem trin 2 og 3 giver derfor ikke dubleret output.

| ID | Niveau | Krav |
|---|---|---|
| GO-35 | SKAL | `maxUnacked` gælder pr. forbruger-replika, ikke for strømmen samlet. Producentens `Emit` blokerer, når den næste chunk er tildelt en forbruger-replika (GO-76), der allerede har `maxUnacked` u-ackede chunks fra strømmen, hos mindst ét forbrugende trin. Med 10 replikaer og `maxUnacked: 4` kan op til 40 chunks fra strømmen være i luften. Komponenten mærker backpressure som et langsomt kald, ikke som en fejl. |
| GO-94 | SKAL | Fordi chunks skrives i rækkefølge, og fordelingen er fast, venter producenten på den replika, næste chunk er tildelt, selv om andre replikaer er ledige. Den langsomste replika sætter derfor tempoet for strømmen. Det er et bevidst valg, der giver deterministisk fordeling og simpel genstart, og dokumentationen nævner det. |
| GO-36 | SKAL | Hver forbruger-replika har sin egen ack-fil pr. strøm, `runs/<uid>/<kant>/<replika>/acks-<trin>-<forbruger-replika>.json`. Ved fan-out og replikaer har en strøm derfor flere ack-filer, men hver fil har kun én skriver. Filen lister numrene på de ackede chunks: `{"version": 1, "acked": [1, 11, 21]}`. |
| GO-37 | SKAL | En input-chunk på en indgående strøm er behandlet, når alle udgående manifester har `consumed[<kant>/<replika>]` større end eller lig med dens nummer. Ved genstart bruger agenten dette, ikke ack-filen, når der er udgående kanter. Den opdaterer ack-filen, så den passer, og fortsætter fra første ubehandlede chunk. |
| GO-70 | SKAL | Ved genstart sletter agenten output-chunks, der ikke står i manifestet (7.3 trin 4). Hvis en input-chunk genbehandles og et udgående manifest allerede har committet den (`consumed` ≥ nummeret), kasserer agenten komponentens output til netop den kant. Det giver exactly-once pr. kant, også når komponenten ikke er deterministisk. |
| GO-71 | SKAL | Input-chunks på samme indgående kant afsluttes (`Done`) i rækkefølge. Agenten afviser `Done` i forkert rækkefølge. |
| GO-80 | SKAL | En forbruger-replika har fået EOS, når hver indgående strøm har `eos` og alle chunks, der er tildelt replikaen, er behandlet. Først da kaldes komponentens `OnEOS` (GO-48). Derefter skriver agenten `eos` i alle replikaens udgående manifester, også når replikaen ikke har lavet noget output; så har manifestet ingen chunks, men `eos: true`. Ellers venter forbrugerne for evigt. |
| GO-90 | SKAL | For et trin med indgående kanter lukker `Close` kun chunken; den committes ikke. Alle output-chunks fra en input-chunk skrives i manifestet samtidig med `consumed`, når input-chunken er `Done` (7.4 trin 2). Et nedbrud midt i en input-chunk giver derfor aldrig synligt, dubleret output. |
| GO-91 | SKAL | En kilde er et trin uden indgående kanter. For en kilde er hver lukket output-chunk et commit-punkt: den skrives i manifestet, så snart den er lukket. Når komponenten returnerer, skriver agenten `eos: true`. |
| GO-92 | SKAL | En kilde skal være deterministisk: samme settings giver de samme chunks i samme rækkefølge. Ved genstart læser agenten manifestet, og hvis det har *k* chunks, kasserer den de første *k* chunks, komponenten lukker, og committer først fra chunk *k*+1. Agenten sammenligner checksum (GO-31) for de kasserede chunks med manifestet. Ved forskel sættes trinnet til `Failed` med årsagen "kilden er ikke deterministisk". |
| GO-93 | SKAL | Kilder, der læser ekstern tilstand (fx et S3-prefix eller et share), er ikke deterministiske og understøttes ikke i v0.1.0. En senere version giver dem en genoptagelsesposition via SDK'et. Dokumentationen siger det tydeligt. |
| GO-95 | SKAL | Et trin, hvis komponent har registreret `OnEOS` (GO-48), er tilstandsfuldt: dets tilstand findes kun i komponentens hukommelse. For et sådant trin flytter agenten ikke `consumed` i de udgående manifester, før `OnEOS`-outputtet er skrevet. Chunks fra `OnEOS`, `consumed` for alle indgående strømme og `eos: true` committes i samme skrivning af hvert udgående manifest. Ved genstart før denne skrivning behandles alle tildelte input-chunks forfra, så tilstanden genopbygges. |
| GO-96 | SKAL | Et tilstandsfuldt trin acker stadig sine input-chunks løbende (GO-36), så producenterne ikke blokeres af backpressure. En ack er derfor ikke en garanti for, at chunken er committet. Input-chunks må først slettes fra lageret, når alle forbrugende replikaer har dem i `consumed`, ikke når de er ackede. |
| GO-72 | SKAL | Resultater (5.2) tæller som udgående manifester, så et trin med resultater får exactly-once som ved kanter. For et trin uden udgående kanter og uden resultater er ack-filen commit-punktet. Effekter uden for Alpheus (fx skrivning til en database) er at-least-once, og dokumentationen for komponent-SDK'et siger det tydeligt. |


## 8. Signal-drivere (`internal/signal`)

```go
type Driver interface {
    Publish(ctx context.Context, sig Signal) error
    Subscribe(ctx context.Context, run, edge string) (<-chan Signal, error)
    Close() error
}
```

| ID | Niveau | Krav |
|---|---|---|
| GO-38 | SKAL | `builtin`: agenter holder én bidirektionel gRPC-stream åben til controllerens hub (`proto/signal.proto`). Hub'en videresender signaler pr. run og kant og gemmer intet varigt. |
| GO-73 | SKAL | Hub'en kører kun i den replica, der er leder (GO-21). Lederen sætter label'en `alpheus-dev.org/signal-hub=active` på sin egen pod og fjerner den, når den mister lederskabet. Hub-Service'en vælger pods med denne label. Ved failover genforbinder agenter med eksponentiel backoff og poller i mellemtiden (GO-40). |
| GO-39 | SKAL | `stomp`: én destination pr. run og kant (`/queue/alpheus.<run>.<kant>`), persistente beskeder, client-ack. |
| GO-40 | SKAL | Begge drivere falder tilbage til at polle manifestet (standard hvert 5. sekund), hvis forbindelsen er nede. Korrektheden afhænger aldrig af, at et signal kommer frem. |
| GO-41 | SKAL | Forbindelsen mellem agent og hub bruger mTLS med certifikater udstedt af controlleren pr. run. CA'en ligger i Secret'en `alpheus-signal-ca` i `alpheus-system`. Den oprettes af den første leder og læses af alle replicas, så certifikater stadig gælder efter failover. Hub'en afviser forbindelser fra en agent, hvis forsøgsnummer er ældre end trinnets aktuelle (GO-67). |
| GO-42 | BØR | En fælles testsuite kører mod begge drivere og kontrollerer samme rækkefølge og semantik. |


## 9. CLI (`cmd/alpheus`)

| Kommando | Formål |
|---|---|
| `alpheus run -f flow.yaml` | Opret en StreamRun fra en fil |
| `alpheus get [run]` | Vis runs eller ét runs status |
| `alpheus watch <run>` | Følg status live, med lag pr. kant |
| `alpheus logs <run> [trin]` | Vis logs fra trinnenes containere |
| `alpheus cancel <run>` | Sæt `spec.cancel` |
| `alpheus chunks <run> <kant>` | Vis manifestet for en kant |
| `alpheus validate -f flow.yaml` | Valider en flow-fil uden at oprette den |
| `alpheus results gc [--dry-run]` | Find og slet resultater med udløbet TTL eller uden StreamRun (GO-99) |
| `alpheus version` | Vis CLI-, controller- og CRD-version |

| ID | Niveau | Krav |
|---|---|---|
| GO-43 | SKAL | CLI'en bruger brugerens kubeconfig og har ingen egen server eller egen login. |
| GO-44 | SKAL | Alle kommandoer understøtter `-o json` og `-o yaml`, så de kan bruges i scripts. |
| GO-45 | SKAL | Fejl giver en exit-kode ≠ 0 og en besked, der siger, hvad der gik galt, og hvad man kan gøre. |


## 10. Komponent-SDK (`sdk/`)

En komponent læser input-chunks som filer og skriver output-chunks som
filer. Formatet er komponentens eget; Alpheus læser det ikke:

```go
func main() {
    sdk.Run(func(ctx context.Context, in sdk.Input, out sdk.Output) error {
        for chunk, err := range in.Chunks(ctx) {
            if err != nil {
                return err
            }
            dst, err := out.Create(ctx, "default", "csv")
            if err != nil {
                return err
            }
            if err := filter(chunk.Path, dst); err != nil {
                return err
            }
            if err := dst.Close(); err != nil {
                return err
            }
        }
        return nil
    })
}
```

`in.Port("lookup").Chunks(ctx)` læser kun fra én indgang (GO-109).

| ID | Niveau | Krav |
|---|---|---|
| GO-46 | SKAL | `sdk.Run` håndterer socket-forbindelse, `Next`/`Done`-løkken, signaler for nedlukning og exit-koder. Komponentens kode ser kun filer i sin egen scratch-mappe og aldrig S3. |
| GO-47 | SKAL | SDK'et har en lokal testmode, der læser input-filer fra disk og skriver til disk uden agent, så komponenter kan testes uden Kubernetes. |
| GO-48 | SKAL | Komponenter med tilstand på tværs af chunks får et `in.OnEOS`-hook til at skrive deres resultat til sidst. |
| GO-49 | BØR | Samme komponent-binær kan køre i MCPipes batch-mode på Argo ved at SDK'et genkender miljøet og læser én stor chunk. |
| GO-86 | SKAL | En output-chunk lukkes, når komponenten kalder `Close` på den, eller automatisk, når den input-chunk, den blev lavet af, er færdig (`Done`). `Flush` og `maxRows` fjernes fra komponent-API'et og fra StreamRun. |
| GO-87 | SKAL | Lokal testmode vælges med `ALPHEUS_MODE=local` og mapperne `ALPHEUS_LOCAL_INPUT` (alle filer, læst i navneorden) og `ALPHEUS_LOCAL_OUTPUT` (én mappe pr. udgang, med manifest-formatet fra 7.3, og filerne `_summary.json` med tællere og antal fejl og `_errors.jsonl` med fejlloggen). `ALPHEUS_SETTINGS` er trinnets settings som JSON, og `ALPHEUS_REPLICA` og `ALPHEUS_REPLICAS` vælger, hvilke chunks replikaen læser (GO-76). Komponenttests kalder `sdk.RunLocal` direkte. |


### 10.1 Tællere, fejl og fremdrift

Komponenter kan rapportere, hvor meget de har behandlet, i en enhed de
selv vælger, og skrive fejl til en fejllog, der kun tilføjes til. Alpheus
tæller og gemmer, men fortolker ikke, hvad der tælles.

```go
sdk.Count(ctx, "rows", 50000)
sdk.Count(ctx, "files", 3)
sdk.Error(ctx, "missing_amount", "amount is empty", sdk.Field("row", 9))
sdk.Warn(ctx, "date_format", "date parsed as DD-MM-YYYY", sdk.Field("row", 14))
sdk.Progress(ctx, "4,2 mio. rækker behandlet")
```

**Tællere**

| ID | Niveau | Krav |
|---|---|---|
| GO-113 | SKAL | `Observe` er en strøm fra komponent til agent med tællere, fejl, advarsler og fremdrift. Den erstatter `Report`. |
| GO-114 | SKAL | En tæller har en enhed, komponenten selv vælger (små bogstaver, tal og `_`, højst 32 tegn), og et antal. Alpheus fortolker ikke enheden. Et trin kan højst have 16 forskellige enheder; flere giver en fejl ved `Observe`. |
| GO-115 | SKAL | Tællere, der rapporteres, mens en input-chunk behandles, committes i den chunks manifest-post (`counts`), når den er `Done`. For en kilde committes de med den output-chunk, der lukkes. `status.steps[].counts` er summen af committede tællere, så tallene er præcise, også efter en genstart. |
| GO-116 | SKAL | Agenten eksponerer tællerne live på `/metrics` som `alpheus_component_count_total` med labels for run, trin, replika og enhed. Live-tallene er best-effort og nulstilles ved genstart; status er det præcise tal. |

**Fejllog**

| ID | Niveau | Krav |
|---|---|---|
| GO-117 | SKAL | Fejl og advarsler har niveau (`error` eller `warn`), kode, besked og valgfrie felter. Agenten skriver dem som JSON Lines med tid, trin, replika og input-chunk tilføjet. |
| GO-118 | SKAL | Fejlloggen er en del af chunk-commit'et: poster fra en input-chunk skrives først, når chunken er `Done`. Efter en genstart står de derfor ikke to gange. Loggen skrives som nummererede segmenter med manifest under `results/<run>/<uid>/errors/<trin>/<replika>/` og har samme TTL som resultater (GO-78). |
| GO-119 | SKAL | `status.steps[].errors` og `warnings` viser antallet, og `errorLog` viser stien. `alpheus get` og `alpheus watch` viser dem. |
| GO-120 | SKAL | En post er højst 4 KB, og en replika kan højst skrive 100.000 poster. Poster ud over grænserne gemmes ikke, men tælles i `errorsDropped`. Dokumentationen anbefaler at henvise til rækkenummer og felt frem for indhold, da fejlbeskeder ellers let kommer til at indeholde persondata. |

**Måling og opsummering**

| ID | Niveau | Krav |
|---|---|---|
| GO-121 | SKAL | Agenten måler selv tiden fra `Next` til `Done` for hver input-chunk og eksponerer den som histogrammet `alpheus_chunk_duration_seconds`. Komponenten skal ikke gøre noget. |
| GO-122 | BØR | Fremdrift er en kort tekst, der vises i `status.steps[].replicas[].message` og i `alpheus watch`, højst én opdatering i sekundet pr. replika (GO-22). Den er best-effort og committes ikke. |
| GO-123 | SKAL | Når runnet er afsluttet, skriver controlleren `results/<run>/<uid>/summary.json` med fase, tællere, antal fejl og advarsler pr. trin og replika. Den overlever, at StreamRun-objektet slettes, og har samme TTL som resultater. |
| GO-124 | SKAL | `Observe` blokerer aldrig komponenten. Tællere og fejl, der skal committes, holdes af agenten indtil `Done`; kun live-metrics og fremdrift kan smides væk, hvis agenten er travl. |


## 11. Logning, metrics og tracing

| ID | Niveau | Krav |
|---|---|---|
| GO-50 | SKAL | Logs skrives med `log/slog` som JSON til stdout. Faste felter: `run`, `step`, `edge`, `chunk`, `component`. |
| GO-51 | SKAL | Metrics eksponeres på `:9090/metrics`. Navne har præfikset `alpheus_`, fx `alpheus_chunks_total`, `alpheus_bytes_total`, `alpheus_edge_lag`, `alpheus_backpressure_seconds`, `alpheus_step_restarts_total`. |
| GO-52 | SKAL | Controller og agent har `/healthz` og `/readyz`. |
| GO-53 | BØR | OpenTelemetry-tracing kan slås til, med et span pr. chunk fra skrivning til ack. |


## 12. Test

| Niveau | Hvad | Hvordan |
|---|---|---|
| Unit | chunk, manifest, backpressure, graf-validering | `go test`, ingen netværk |
| Integration | controller mod API-server | envtest |
| Integration | agent mod S3 | Garage i en container |
| Driver | `builtin` og `stomp` | fælles suite, Artemis i en container |
| E2E | testcase01 til testcase06, fejltests | Talos i en VM med Vagrant og libvirt (`vagrant/` i repo'ets rod) med Garage og Zot (12.2) |

| ID | Niveau | Krav |
|---|---|---|
| GO-54 | SKAL | Alle accepttests fra den funktionelle specifikation kører automatisk i CI på hver pull request mod `main`. |
| GO-55 | SKAL | Hver testcase kontrollerer sit resultat mod noget, der er beregnet uden Alpheus: kildens egne data eller en kendt værdi. |
| GO-56 | SKAL | En kaos-test slår tilfældige trin-pods ihjel under et run og kontrollerer, at resultatet stadig er korrekt (GO-55). |
| GO-57 | SKAL | `go test -race` kører på alle pakker. |
| GO-58 | BØR | Fuzz-tests for manifest-parsing i agenten. |
| GO-74 | SKAL | Genstartstests slår agenten ihjel mellem hvert trin i commit-rækkefølgen (7.3 og 7.4) og kontrollerer, at output hverken mangler eller er dubleret. |
| GO-100 | SKAL | En genstartstest slår det tilstandsfulde `sink`-trin ihjel, efter at det har acket halvdelen af sine input-chunks, men før EOS. Resultatet skal stadig have hver chunk præcis én gang og uændret, og tællerne må ikke være for høje. Testen beviser GO-95. |

### 12.1 Testcases

Testkomponenterne ligger i `test/components/` og bygges som images, der
skubbes til Zot i testmiljøet. De bruger et simpelt format fra Gos
standardbibliotek (CSV), da Alpheus ikke kender Parquet (GO-101).

Testcasene er Alpheus' egne. De tester det, Alpheus gør, og kender ikke
noget bestemt dataformat.

**testcase01: måling af systemet.**

```
source ──> pass ──> sink ──> resultat i S3
```

En simpel testcase, der også viser, hvordan systemet yder. Alle tre trin
bruger testkomponenten `bench`, så hver chunk går to gange gennem
lageret.

- `source` laver `chunks` chunks på `size` bytes. Hver chunk begynder
  med sit nummer og tidspunktet, den blev lavet, efterfulgt af bytes fra
  en ChaCha8-generator med et fast seed.
- `pass` kopierer hver chunk uændret til sit output.
- `sink` kontrollerer hver byte mod generatoren og noterer, hvornår
  chunken kom. Ved EOS skriver den resultatet `n,bytes,written_ns,received_ns`
  med én linje pr. chunk, med indgangen først
  (`port,n,bytes,written_ns,received_ns`). En forkert, dubleret eller
  manglende chunk får trinnet til at fejle.

Testen kontrollerer, at runnet ender i `Succeeded`, at resultatet har
alle chunks, og at alle tre trins tællere passer. Den finder resultatet
med `status.results` alene (GO-82). Den måler:

| Måling | Fra |
|---|---|
| Opstart | Runnet er oprettet, til alle komponenter kører (pod-status, hele sekunder) |
| Strøm | Første chunk er lavet, til sidste er modtaget |
| I alt | Runnet er oprettet, til `Succeeded` |
| Gennemløb | Bytes delt med strømmens varighed |
| Latens | Pr. chunk fra den laves, til `sink` har den; p50, p95 og max |

Størrelserne kan ændres med `BENCH_CHUNKS`, `BENCH_SIZE`,
`BENCH_PASS_REPLICAS` og `BENCH_MAX_UNACKED` (standard 64 chunks på
4 MiB, én `pass`-replika og `maxUnacked: 4`). `make bench` kører
testcase01, testcase02 og testcase03 og gemmer målingerne som JSON i `_out/bench/`.
Alle trin kører på samme node, så tiderne fra `source` og `sink` kan
sammenlignes.

**testcase02: fan-out og fan-in.**

```
            ┌──> left  (pass, 3 replikaer) ──┐ indgang left
source ─────┤                                ├──────────────> sink ──> resultat i S3
            └──> right (pass, 2 replikaer) ──┘ indgang right
```

Samme komponent som testcase01, men strømmen deler sig og samles igen.
`source`s ene udgang har to kanter, så hver chunk går til både `left` og
`right`, der hver fordeler chunks mellem sine egne replikaer. `sink` har
to indgange og indstillingen `ports: [left, right]`: den skal have hver
chunk præcis én gang på hver indgang. Replikaantallene er forskellige,
så de to grene fordeler chunks forskelligt.

Testen kontrollerer ud over testcase01's kontroller:

- Fan-out: `source`s strøm har egne ack-filer for `left` og `right`
  (GO-36). `left`s tre filer indeholder tilsammen 1–64 præcis én gang,
  med chunk n hos replika n mod 3, og `right`s to filer det samme med
  n mod 2 (GO-76).
- Fan-in: `consumed` i `sink`s resultat-manifest har alle fem indgående
  strømme, hver til strømmens sidste chunk, og hver strøm har `eos`
  (GO-80, GO-95). Hver chunk kom på sin grens indgang.
- Tællerne: `source` 64, `left` og `right` 64 hver over replikaerne,
  `sink` 128.

Målingerne er de samme som i testcase01, med latens pr. indgang.
Replikaerne ændres med `BENCH_LEFT_REPLICAS` og `BENCH_RIGHT_REPLICAS`.

**testcase03: replikaer og en replika uden output.**

```
source ──> filter (pass, 10 replikaer, drop: 10) ──> sink ──> resultat i S3
```

Samme komponent som testcase01. `pass` har indstillingen `drop`: den
dropper hver chunk, hvis nummer i dens input-strøm går op i `drop`, og
tæller dem i enheden `dropped`. `sink` har den tilsvarende `skip` og
forventer kun de øvrige chunks.

- `filter` kører med `replicas: 10` og `drop: 10`. `source` har én
  replika (p = 0), så chunk n går til replika n mod 10 (GO-76), og
  replika 0 får præcis chunks 10, 20, 30 …. Den dropper dem alle, laver
  derfor intet output og skal alligevel afslutte med EOS (GO-80). Det er
  testens svære kant.
- `sink` kører med `skip: 10`, kontrollerer hver byte og fejler, hvis en
  chunk, der skulle være droppet, kommer frem.
- Replikaantallet ændres med `BENCH_FILTER_REPLICAS`, og `drop` og
  `skip` følger det, så replika 0 altid får præcis de droppede chunks.

| ID | Niveau | Krav |
|---|---|---|
| GO-81 | SKAL | Testen kontrollerer, at resultatet har præcis de chunks, der ikke går op i 10, hver med det indhold, generatoren laver uden Alpheus, og at `filter` talte 10 % af dem som `dropped`. |
| GO-84 | SKAL | Testen læser de ti `filter`-replikaers ack-filer (GO-36) og kontrollerer, at mængderne af ackede chunks er disjunkte, at deres forening er præcis alle chunks, og at chunk n ligger hos replika n mod 10. Testen afhænger ikke af logs. |
| GO-85 | SKAL | Testen kontrollerer, at replika 0's udgående manifest har nul chunks, `last: 0` og `eos: true`, at de andre replikaer skrev én chunk pr. ackede chunk, og at `sink` alligevel er nået til EOS med alle ti strømme forbrugt. |
| GO-82 | SKAL | Testen henter resultatet med oplysningerne i `status.results` og intet andet, så den også tester, at de er nok. |

**testcase04: genstart af hver slags trin.**

```
source (fixed) ──> pass ──> sink (delay: 100)
```

Tre runs, der hver slår ét trins pod ihjel med det samme (uden
grace period), så agent og komponent dør sammen som ved et
node-nedbrud. Controlleren ser pod'en forsvinde og starter et nyt
forsøg. Testen vælger tidspunktet ud fra manifester og ack-filer i S3:

| Run | Slås ihjel, når | Beviser |
|---|---|---|
| `source` | Den har committet en fjerdedel af sine chunks | En deterministisk kilde fortsætter fra næste chunk efter at have sammenlignet dem, den laver igen (GO-92) |
| `pass` | Den har committet halvdelen af sit output | Den fortsætter fra første ubehandlede chunk, sletter output, der aldrig blev committet, og intet står to gange (GO-37, GO-70) |
| `sink` | Den har acket halvdelen af sit input, før EOS | Et tilstandsfuldt trin læser alt sit input igen (GO-95, GO-100) |

`source` har indstillingen `fixed`, der skriver tiden 0 i chunkens
header, så hvert forsøg laver de samme chunks; ellers ville agenten med
rette afvise kilden som ikke-deterministisk. `sink` har `delay`, så
testen når at handle. Testen kontrollerer, at runnet lykkes, at kun det
dræbte trin har én genstart, at `sink` fik hver chunk præcis én gang og
uændret, at hver strøm har chunk 1–64 og EOS, at der ikke ligger
chunk-objekter, som aldrig blev committet, og at ingen tæller er for
høj. Tællerne kan være for lave, indtil GO-115 i 16 er afgjort.

**testcase05: backpressure.**

```
source ──> slow (pass, 2 replikaer, delay: 500) ──> sink
```

Mens runnet kører, læser testen ca. fem gange i sekundet `source`s
manifest og derefter `slow`s to ack-filer. For hver replika må antallet
af chunks, der er tildelt den, men ikke acket, aldrig overstige
`maxUnacked` (GO-35). Manifestet læses først, så en prøve kan tælle for
få, aldrig for mange. Hver replika skal også nå grænsen, ellers blev
backpressure ikke testet. Den langsomste replika sætter tempoet (GO-94),
så strømmen tager mindst chunks × delay / replikaer. Replikaerne og
ventetiden ændres med `BENCH_SLOW_REPLICAS` og `BENCH_SLOW_MS`.

**testcase06: kaos.**

testcase02's form med `fixed` kilde og `maxRestarts: 100`. Ca. hvert
tredje sekund slår testen en tilfældig pod i runnet ihjel, til runnet er
færdigt (GO-56). Ud over testcase02's kontroller må der ikke ligge
chunk-objekter, som aldrig blev committet. Tællerne må være for lave,
men aldrig for høje (se GO-115 i 16). `BENCH_CHAOS_SEED` vælger
pod'erne, så en fejl kan gentages; testen logger den.

**Agent-test: hvert commit-punkt (GO-74).** En pod, der slås ihjel,
rammer ikke et bestemt sted mellem to skrivninger til S3. Derfor kører
`TestCrashAtEveryWrite` pipelinen i hukommelsen og lader skrivning
nummer k fejle for hvert k i et run, både som en skrivning, der aldrig
nåede frem, og som en, der nåede frem, men hvis svar gik tabt. Herefter
fejler alle skrivninger, som om hele runnet døde. Så køres alle
replikaer igen som forsøg 2, og resultatet skal være uden manglende og
dublerede data.

### 12.2 Testmiljø (Vagrant)

`vagrant/deploy.sh` laver en Talos-VM med de samme tjenester som MCPipe
bruger, i de samme versioner: Garage (S3) og Zot (container-registry).
Hver `deploy.sh` laver nye nøgler, og `vagrant/credentials.sh` viser dem.
Bruges Alpheus uden Vagrant, sættes S3-lager og registry op manuelt.

| Tjeneste | Adresse fra værten | Hvad `credentials.sh` viser |
|---|---|---|
| Garage S3 | `https://<VM IP>:30900`, region `garage`, bucket `alpheus-data` | access key, secret key |
| Garage dashboard | `https://<VM IP>:30901` | admin token (bruges til login) |
| Garage internt | | rpc secret |
| Zot | `https://registry.alpheus.test:30500`, ingen login | Zots egen S3-nøgle til bucket `alpheus-registry` |

HTTPS går gennem en gateway (HAProxy) med et certifikat fra en CA, som
`deploy.sh` laver i `vagrant/_out/`. Talos-noden stoler på CA'en, så den
kan hente images fra Zot.

VM'ens IP kan skifte fra deploy til deploy. Flow-filer bruger derfor det
faste navn `registry.alpheus.test:30500` til images, ikke IP'en. Navnet
står i certifikatet fra CA'en og peger på VM'en via Talos'
statiske værtsnavne (`StaticHostConfig`, tidligere `extraHostEntries`).
På værten skal navnet stå i `/etc/hosts` for at skubbe images;
`credentials.sh` viser linjen.

Zot har ingen login. Det er kun i orden, fordi VM'en ligger på libvirts
host-only/NAT-netværk og ikke kan nås udefra. Det står i
`vagrant/README.md`, så ingen eksponerer den ved et uheld.


## 13. Kodestil og CI

| ID | Niveau | Krav |
|---|---|---|
| GO-59 | SKAL | `gofmt`, `go vet` og `golangci-lint` med en fast konfiguration i repo'et. CI fejler ved fund. |
| GO-60 | SKAL | `context.Context` er første parameter i alle funktioner, der laver I/O, og respekteres for afbrydelse. |
| GO-61 | SKAL | Fejl pakkes med `fmt.Errorf("…: %w", err)`, og sentinel-fejl bruges til tilstande, kalderen skal kunne reagere på. |
| GO-62 | SKAL | Ingen globale mutable variabler, ud over registrering af metrics. |
| GO-63 | SKAL | Genereret kode (CRD'er, protobuf, deepcopy) committes og kontrolleres i CI for at være opdateret. |
| GO-64 | BØR | Offentlige pakker (`api/`, `sdk/`) har godoc-kommentarer på alle eksporterede navne og eksempler. |


## 14. Udgivelse

1. Tag `vX.Y.Z` på `main`.
2. CI bygger binærer og images, genererer SBOM og signerer.
3. Images skubbes til et offentligt registry og spejles til jeres Zot.
4. Helm-chartet udgives som OCI-artefakt.
5. Hjemmesiden genbygges med dokumentation for den nye version.

| ID | Niveau | Krav |
|---|---|---|
| GO-65 | SKAL | Hver udgivelse har release notes med ændringer, opgraderingsnoter og eventuelle CRD-ændringer. |
| GO-66 | SKAL | CRD-opgraderinger er bagudkompatible inden for samme API-version. En ny API-version leveres med en conversion webhook. |


## 15. Rækkefølge for implementeringen

1. `api/v1alpha1` og genererede CRD'er. *Færdig*: 22 CRD-tests mod
   API-serveren i testklyngen (`make test-crd`).
2. `internal/chunk` og `internal/store` med unit-tests og Garage.
   *Færdig*: unit- og fuzz-tests, lagertests mod Garage (`make test-s3`).
3. `sdk` med lokal testmode og en filter-komponent. *Færdig*: fil-API'et
   (GO-101 til GO-112), tællere og fejllog (10.1), og filter i CSV.
   Agent-mode kommer med agenten i trin 4.
4. Agenten med `builtin`-signaler, kørt lokalt mod Garage. *Færdig*:
   `internal/agent`, `internal/signal/builtin`, `cmd/alpheus-agent` og
   agent-mode i SDK'et. Agenter og komponenter kører i samme proces mod
   hukommelseslager og mod Garage (`make test-agent-s3`) med
   testcase03's form, genstart af `filter` og af det tilstandsfulde
   `sum`, kilde-genstart (GO-92), fencing (GO-68) og backpressure
   (GO-35). Fejllog og tællere skrives under `errors/<trin>/<replika>/`
   efter de udgående manifester, så de aldrig står to gange, men kan
   mangle efter et nedbrud lige derimellem (se GO-115 i 16). mTLS til
   hub'en (GO-41) kom med controlleren i trin 5.
5. Controlleren med envtest. *Færdig*: `internal/controller` og
   `cmd/alpheus-controller`. Tests mod en rigtig API-server (`make test`
   henter envtest-binærerne): pod-spec med agenten som native sidecar,
   genstart og `maxRestarts`, `failurePolicy: continue` med lukkede
   strømme og `failedChunks`, ugyldig graf, image uden digest, cancel,
   finalizer, status fra lageret og mTLS til hub'en. CEL-reglerne for
   `resultsTTL` og `ttlAfterFinished` sammenligner nu som varigheder,
   fordi Go-klienten skriver `168h` tilbage som `168h0m0s`. Ikke med
   endnu: ServiceAccount og NetworkPolicy pr. run (6.1 trin 2),
   webhook (GO-23), og at hub'en afviser ældre forsøg (GO-41), da
   certifikaterne er pr. run og ikke pr. forsøg.
6. E2E på Talos i Vagrant-VM'en med testcase01. *Færdig*: `make e2e`
   (`hack/e2e.sh`) bygger agent, controller og testkomponenterne som
   statiske binærer i images på `scratch` (uid 65532, reproducerbare
   digests), skubber dem til Zot, installerer CRD og controller med
   `config/default` og kører `test/e2e`. Den første testcase blev siden
   erstattet af testcase01 i 12.1, som også måler systemet. På testklyngen
   (én node, varmt image) giver standardstørrelsen ca. 1 s opstart, 40
   MB/s gennem to hop og en latens pr. chunk på 300–500 ms. Signalerne
   går gennem hub'en: hvert trin slutter inden for ca. 50 ms efter det
   forrige. Fundet undervejs: hub'en
   fejlede ved første start, fordi signal-CA'en blev oprettet, men
   fejlen fra opslaget blev returneret; controlleren skrev spec'en
   tilbage, når den satte sin finalizer (`168h` blev til `168h0m0s`), og
   bruger nu en patch; og `status.results` manglede regionen, som
   Garage kræver (GO-79).
7. Fan-out og fan-in med testcase02, replikaer med testcase03.
   *Færdig*: begge kører på testklyngen (12.1), og agent-testen
   `TestFanOutFanIn` kører testcase02's form i hukommelsen med
   `make test`. Med standardstørrelsen: testcase02 ca. 23–34 MB/s ind i
   `sink` og en latens pr. chunk på 450–1100 ms (p50–p95), da hver chunk
   læses af to trin; testcase03 ca. 16 MB/s og 570–1000 ms med ti
   `filter`-replikaer på én node. testcase03 blev bygget med
   `bench`-komponenten i stedet for den oprindelige salgsrapport i CSV,
   så testene stadig ikke afhænger af et dataformat.
8. Genstart, backpressure og kaos-test. *Færdig*: testcase04,
   testcase05 og testcase06 kører på testklyngen (12.1), og
   `TestCrashAtEveryWrite` kører med `make test`. Fundet undervejs: en
   genstartet kilde talte de chunks, agenten kasserer (GO-92), med
   igen, og et tilstandsfuldt trin talte alt sit input igen efter en
   genstart; dets fejllog ville også have stået to gange. Et
   tilstandsfuldt trin holder nu tællere og fejllog tilbage til
   `OnEOS`-outputtet er committet, og en kasseret chunk fra en kilde
   committer intet (GO-95, GO-115, GO-118). Tilbage står, at tællere kan
   mangle efter et nedbrud mellem de udgående manifester og tællerne
   (16).
9. `stomp`-driveren.
10. CLI, Helm-chart og første udgivelse `v0.1.0`.


## 16. Åbne spørgsmål

- Hvilket offentligt container-registry: GitHub Container Registry,
  Quay eller et europæisk alternativ?
- Skal SDK'et findes til Python fra start, eller er gRPC-kontrakten nok
  til andre sprog i første omgang?
- Skal manifest-segmentering (GO-32) være med i `v0.1.0`, eller er en
  øvre grænse for antal chunks pr. kant nok til at starte med?
- Skal `status.results` pege på en særskilt nøgle med kun læseadgang til
  `results/<run>/<uid>/` (fx `spec.storage.resultsSecretRef`), i stedet
  for agenternes nøgle til hele bucketen (5.2)?
- Hvor committes tællere og fejllog for et trin med input (GO-115,
  GO-118)? En input-chunk uden output har ingen manifest-post. Agenten
  skriver dem nu i `results/<run>/<uid>/errors/<trin>/<replika>/`
  efter de udgående manifester, så de kan mangle, men aldrig står to
  gange. Præcise tal kræver, at de står i selve commit-skrivningen, fx
  i de udgående manifester ved siden af `consumed`.
  `TestCrashAtEveryWrite` viser det: et nedbrud lige efter de udgående
  manifester mister den chunks tællere, og et nedbrud lige efter et
  tilstandsfuldt trins sidste manifest mister alle dets tællere.
- Et trin, der hedder `errors`, får resultater under
  `results/<run>/<uid>/errors/`, samme sted som fejlloggen. Skal navnet
  afvises i CRD'en, eller skal fejlloggen flyttes (fx til `_errors/`)?
- Med `failurePolicy: continue` acker en fejlet replika aldrig de
  chunks, der fortsat tildeles den. Producenten blokerer så på
  backpressure (GO-35), og runnet går i stå. Skal controlleren skrive
  replikaens ack-fil efter bekræftet stop, som den skriver dens
  manifester (GO-97), eller skal producenten springe fejlede replikaer
  over?
