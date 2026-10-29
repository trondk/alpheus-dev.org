# Alpheus — implementeringsspecifikation (Go)

Status: udkast, 2026-10-29. Trin 1–3 i afsnit 15 er bygget: API-typer og
CRD, chunks og lager, og komponent-SDK'et med lokal testmode. Agent og
controller findes ikke endnu, så intet kører på Kubernetes.

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


## 3. Repo-struktur

Go-modulet ligger i `code/` i repo'et. Testklyngen ligger i `vagrant/` i
repo'ets rod og hjemmesiden i `www/`. Mapper markeret *(findes)* er
bygget; resten er planlagt.

```
code/
├── Makefile               generate, test, test-crd, test-s3 (findes)
├── api/v1alpha1/          StreamRun-typer med kubebuilder-markører (findes)
├── cmd/
│   ├── alpheus/           CLI
│   ├── alpheus-controller/
│   └── alpheus-agent/
├── internal/
│   ├── controller/        reconcile-løkke, pod-spec, status
│   ├── agent/             socket-server, ind- og udgange, genstart
│   ├── chunk/             navngivning, manifest, ack-filer, checksum (findes)
│   ├── pq/                Parquet-skrivning og -læsning af chunks (findes)
│   ├── signal/            driver-interface
│   │   ├── builtin/       gRPC-hub i controlleren og klient i agenten
│   │   └── stomp/         STOMP-driver til fx Artemis
│   ├── store/             S3-klient bag et lille interface (findes)
│   └── version/
├── sdk/                   offentligt komponent-SDK (findes, uden agent-mode)
├── proto/                 component.proto og signal.proto
├── config/                genererede CRD-, RBAC- og webhook-manifester
│   ├── crd/bases/         StreamRun-CRD'en (findes)
│   └── samples/           eksempler, bl.a. testcase03-replicas (findes)
├── hack/                  boilerplate og test-crd.sh (findes)
├── charts/alpheus/        Helm-chart
├── test/
│   ├── components/        testkomponenter: filter (findes), generate og sum
│   ├── e2e/               testcase01, testcase02, testcase03-replicas og fejltests
│   └── fixtures/          input og golden files
├── docs/                  kilde til dokumentationen på hjemmesiden
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
| Parquet og Arrow | `github.com/apache/arrow-go` (Parquet-pakken) | Apache 2.0 | Godkendt, i brug (v18.8.0) |
| S3 | `github.com/minio/minio-go/v7` | Apache 2.0 | Godkendt, i brug (v7.4.1) |
| gRPC | `google.golang.org/grpc`, `google.golang.org/protobuf` | Apache 2.0 / BSD | Foreslået (trin 4) |
| STOMP | `github.com/go-stomp/stomp/v3` | Apache 2.0 | Foreslået (trin 9) |
| CLI | `github.com/spf13/cobra` | Apache 2.0 | Foreslået (trin 10) |
| Metrics | `github.com/prometheus/client_golang` | Apache 2.0 | Foreslået |
| Logning | `log/slog` fra standardbiblioteket | BSD | I brug |
| Test | `sigs.k8s.io/controller-runtime/pkg/envtest` | Apache 2.0 | Foreslået (trin 5) |

| ID | Niveau | Krav |
|---|---|---|
| GO-10 | SKAL | Kun licenser, der er forenelige med Apache 2.0. Ingen GPL, AGPL, BSL eller "community"-licenser. |
| GO-11 | SKAL | `govulncheck` kører i CI og blokerer en udgivelse ved kendte sårbarheder i brugt kode. |
| GO-12 | BØR | minio-go vælges frem for AWS SDK, fordi den er mindre og testet mod S3-kompatible lagre som Garage. Begge ligger bag `internal/store`, så valget kan ændres. |


## 5. API-typer (`api/v1alpha1`)

| ID | Niveau | Krav |
|---|---|---|
| GO-13 | SKAL | API-gruppen er `alpheus-dev.org`, versionen `v1alpha1`, kind `StreamRun`, kort navn `sr`. |
| GO-14 | SKAL | Typerne har kubebuilder-markører for validering (påkrævede felter, mønstre, min/max), så CRD'en genereres med OpenAPI-skema. |
| GO-15 | SKAL | `spec` er uforanderlig efter oprettelse, bortset fra `spec.cancel`. Håndhæves med CEL-validering i CRD'en (`x-kubernetes-validations`), ikke kun i controlleren. |
| GO-16 | SKAL | Status-subresource er slået til, og kun controlleren skriver status. |
| GO-17 | SKAL | Printer-kolonner viser fase, antal trin, alder og eventuel fejl i `kubectl get sr`. |
| GO-18 | BØR | Conditions følger `metav1.Condition` (`Ready`, `Progressing`, `Degraded`). |

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

### 5.2 Resultater

En udgang, som ingen kant går fra, er et resultat. Agenten skriver den som
en strøm med manifest, ligesom en kant, men under
`results/<run>/<uid>/<trin>/<udgang>/<replika>/` i stedet for
`runs/<uid>/`. `<uid>` er StreamRun-objektets `metadata.uid`.

| ID | Niveau | Krav |
|---|---|---|
| GO-83 | SKAL | Stier i lageret bruger runnets UID, ikke kun dets navn: `runs/<uid>/` og `results/<run>/<uid>/`. Slettes en StreamRun og oprettes igen med samme navn, får den en ny UID, og gamle og nye data blandes aldrig. Navnet står med i `results/` og i manifestet (`run`), så en person kan finde resultatet. |
| GO-78 | SKAL | Resultater slettes ikke af `ttlAfterFinished` eller finalizeren (GO-20), da de er runnets produkt. De slettes af den, der bruger dem. |
| GO-79 | SKAL | `status.results` har én post pr. resultat med S3-endpoint, bucket, prefix, navnet på Secret'en med nøglerne (`spec.storage.secretRef`) og antal chunks og rækker. Det er det, der skal til for at hente resultatet, og `alpheus get` viser det. |

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
- læser input-chunks fra S3 og giver dem til komponenten som Arrow
  record batches,
- samler komponentens output til chunks, skriver dem til S3 og
  opdaterer manifestet,
- sender og modtager `ready`, `ack`, `eos` og `fail`,
- håndhæver backpressure (`maxUnacked`) og genstart fra manifestet,
- rapporterer tællere til controlleren og som Prometheus-metrics.

### 7.2 Komponent-API (`proto/component.proto`)

```proto
service Agent {
  rpc Next(NextRequest) returns (NextResponse);          // næste chunk eller EOS
  rpc Emit(stream EmitRequest) returns (EmitResponse);   // batches til en udgang
  rpc Flush(FlushRequest) returns (FlushResponse);       // afslut chunken på en udgang
  rpc Done(DoneRequest) returns (DoneResponse);          // input-chunk færdig
  rpc Report(ReportRequest) returns (ReportResponse);    // afviste rækker
}
```

Batches sendes som Arrow IPC-stream-bytes i beskederne, så komponenter i
andre sprog kan bruge enhver Arrow-implementering.

| ID | Niveau | Krav |
|---|---|---|
| GO-27 | SKAL | Store chunks streames i batches, så hverken agent eller komponent behøver have en hel chunk i hukommelsen. Hukommelsesforbruget pr. kant er begrænset af row group-størrelsen (GO-33), ikke af chunk-størrelsen. |
| GO-28 | SKAL | Agenten validerer, at output-batches har samme skema inden for en kant, og afviser ellers med en tydelig fejl. |
| GO-29 | SKAL | Hvis komponenten ikke har kaldt `Next` inden for en konfigurerbar tid efter start, markeres trinnet som fejlet. |

### 7.3 Chunks og manifest (`internal/chunk`)

En kant hedder `<fra>-<til>`, eller `<fra>.<udgang>-<til>` for en anden
udgang end `default`. Hver producent-replika skriver sin egen strøm på
kanten, i mappen `runs/<uid>/<kant>/<replika>/` (replika 0 for et trin
uden `replicas`, GO-83). Chunks i en strøm nummereres fra 1 og uden huller.

S3 har ingen rename. Derfor er manifestet commit-punktet i stedet for en
omdøbning:

1. Producenten skriver chunk-objektet direkte under sit endelige navn
   `runs/<uid>/<kant>/<replika>/part-000017.parquet`.
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
  "schema": "sha256:…",
  "segments": ["manifest-0000.json"],
  "chunks": [
    {"n": 10017, "rows": 50000, "bytes": 1832211, "sha256": "…",
     "src": {"edge": "source-read", "replica": 0, "n": 4711}}
  ],
  "consumed": {"source-read/0": 4711},
  "eos": false,
  "last": null
}
```

- `epoch` er forsøgsnummeret (GO-67) for den agent, der skrev manifestet.
- `segments` lister forseglede manifest-segmenter (GO-32). `chunks`
  indeholder kun chunks, der endnu ikke er forseglet.
- `src` angiver den input-chunk, en output-chunk blev lavet af. Kilder
  (trin uden indgående kanter) udelader feltet.
- `replica` er nummeret på den producent-replika, der ejer strømmen.
- `consumed` angiver pr. indgående strøm (`<kant>/<replika>`) den højeste
  input-chunk, hvis output til denne strøm er fuldt committet, også når
  den ikke gav nogen rækker (se 7.4). En replika ser kun på de chunks,
  der er tildelt den (GO-76).

| ID | Niveau | Krav |
|---|---|---|
| GO-30 | SKAL | Hver strøm har præcis én skriver af manifestet (agenten for den producent-replika, der ejer strømmen), og hver ack-fil har præcis én skriver (forbruger-replikaens agent, GO-36). Derfor kræves ingen låsning eller betingede skrivninger i S3. Enkeltskriveren sikres af GO-67. |
| GO-31 | SKAL | Checksum (SHA-256) beregnes under skrivning og verificeres under læsning. |
| GO-32 | SKAL | Når `chunks` i `manifest.json` når 10.000 chunks, skrives de til et forseglet, uforanderligt segment (`manifest-0000.json`, `manifest-0001.json` osv.). Derefter skrives `manifest.json` med segmentet tilføjet til `segments` og en tom `chunks`. Segmentet skrives altid før `manifest.json`. Så vokser den fil, der omskrives ved hver chunk, ikke ubegrænset. |
| GO-33 | SKAL | Parquet-chunks skrives med Zstd-komprimering. En row group lukkes ved en konfigurerbar størrelse (standard 64 MiB ukomprimeret), så en chunk kan have flere row groups. |
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
| GO-35 | SKAL | Producentens `Emit` blokerer, når strømmen har `maxUnacked` chunks, som ikke er acket af den tildelte replika hos hvert forbrugende trin. Komponenten mærker backpressure som et langsomt kald, ikke som en fejl. |
| GO-36 | SKAL | Hver forbruger-replika har sin egen ack-fil pr. strøm, `runs/<uid>/<kant>/<replika>/acks-<trin>-<forbruger-replika>.json`. Ved fan-out og replikaer har en strøm derfor flere ack-filer, men hver fil har kun én skriver. Filen lister numrene på de ackede chunks: `{"version": 1, "acked": [1, 11, 21]}`. |
| GO-37 | SKAL | En input-chunk på en indgående strøm er behandlet, når alle udgående manifester har `consumed[<kant>/<replika>]` større end eller lig med dens nummer. Ved genstart bruger agenten dette, ikke ack-filen, når der er udgående kanter. Den opdaterer ack-filen, så den passer, og fortsætter fra første ubehandlede chunk. |
| GO-70 | SKAL | Ved genstart sletter agenten output-chunks, der ikke står i manifestet (7.3 trin 4). Hvis en input-chunk genbehandles og et udgående manifest allerede har committet den (`consumed` ≥ nummeret), kasserer agenten komponentens output til netop den kant. Det giver exactly-once pr. kant, også når komponenten ikke er deterministisk. |
| GO-71 | SKAL | Input-chunks på samme indgående kant afsluttes (`Done`) i rækkefølge. Agenten afviser `Done` i forkert rækkefølge. |
| GO-80 | SKAL | En forbruger-replika har fået EOS, når hver indgående strøm har `eos` og alle chunks, der er tildelt replikaen, er behandlet. Først da kaldes komponentens `OnEOS` (GO-48). Derefter skriver agenten `eos` i alle replikaens udgående manifester, også når replikaen ikke har lavet noget output; så har manifestet ingen chunks, men `eos: true`. Ellers venter forbrugerne for evigt. |
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
| `alpheus version` | Vis CLI-, controller- og CRD-version |

| ID | Niveau | Krav |
|---|---|---|
| GO-43 | SKAL | CLI'en bruger brugerens kubeconfig og har ingen egen server eller egen login. |
| GO-44 | SKAL | Alle kommandoer understøtter `-o json` og `-o yaml`, så de kan bruges i scripts. |
| GO-45 | SKAL | Fejl giver en exit-kode ≠ 0 og en besked, der siger, hvad der gik galt, og hvad man kan gøre. |


## 10. Komponent-SDK (`sdk/`)

En komponent skrives som en funktion fra batch til batch:

```go
func main() {
    sdk.Run(func(ctx context.Context, in sdk.Input, out sdk.Output) error {
        for batch, err := range in.Batches(ctx) {
            if err != nil {
                return err
            }
            if err := out.Emit(ctx, "default", filter(batch)); err != nil {
                return err
            }
        }
        return nil
    })
}
```

| ID | Niveau | Krav |
|---|---|---|
| GO-46 | SKAL | `sdk.Run` håndterer socket-forbindelse, `Next`/`Done`-løkken, signaler for nedlukning og exit-koder. Komponentens kode ser kun batches. |
| GO-47 | SKAL | SDK'et har en lokal testmode, der læser Parquet-filer fra disk og skriver til disk uden agent, så komponenter kan testes uden Kubernetes. |
| GO-48 | SKAL | Komponenter med tilstand på tværs af chunks får et `in.OnEOS`-hook til at skrive deres resultat til sidst. |
| GO-49 | BØR | Samme komponent-binær kan køre i MCPipes batch-mode på Argo ved at SDK'et genkender miljøet og læser én stor chunk. |
| GO-86 | SKAL | En output-chunk slutter, når den når `maxRows`, når den input-chunk, den blev lavet af, er færdig (`Done`), og når komponenten kalder `Flush` på udgangen. `Flush` er til komponenter, der selv vælger grænserne, fx en kilde, der skriver sine data i dele. |
| GO-87 | SKAL | Lokal testmode vælges med `ALPHEUS_MODE=local` og mapperne `ALPHEUS_LOCAL_INPUT` (input-chunks som `*.parquet`, læst i navneorden) og `ALPHEUS_LOCAL_OUTPUT` (én mappe pr. udgang). `ALPHEUS_SETTINGS` er trinnets settings som JSON, og `ALPHEUS_REPLICA` og `ALPHEUS_REPLICAS` vælger, hvilke chunks replikaen læser (GO-76). Komponenttests kalder `sdk.RunLocal` direkte. |


## 11. Logning, metrics og tracing

| ID | Niveau | Krav |
|---|---|---|
| GO-50 | SKAL | Logs skrives med `log/slog` som JSON til stdout. Faste felter: `run`, `step`, `edge`, `chunk`, `component`. |
| GO-51 | SKAL | Metrics eksponeres på `:9090/metrics`. Navne har præfikset `alpheus_`, fx `alpheus_chunks_total`, `alpheus_rows_total`, `alpheus_edge_lag`, `alpheus_backpressure_seconds`, `alpheus_step_restarts_total`. |
| GO-52 | SKAL | Controller og agent har `/healthz` og `/readyz`. |
| GO-53 | BØR | OpenTelemetry-tracing kan slås til, med et span pr. chunk fra skrivning til ack. |


## 12. Test

| Niveau | Hvad | Hvordan |
|---|---|---|
| Unit | chunk, manifest, backpressure, graf-validering | `go test`, ingen netværk |
| Integration | controller mod API-server | envtest |
| Integration | agent mod S3 | Garage i en container |
| Driver | `builtin` og `stomp` | fælles suite, Artemis i en container |
| E2E | testcase01, testcase02, testcase03-replicas, fejltests | Talos i en VM med Vagrant og libvirt (`vagrant/` i repo'ets rod) med Garage og Zot (12.2) |

| ID | Niveau | Krav |
|---|---|---|
| GO-54 | SKAL | Alle accepttests fra den funktionelle specifikation kører automatisk i CI på hver pull request mod `main`. |
| GO-55 | SKAL | testcase01 og testcase02 sammenlignes byte for byte med golden files i `test/fixtures`. |
| GO-56 | SKAL | En kaos-test slår tilfældige trin-pods ihjel under et run og kontrollerer, at resultatet stadig matcher golden file. |
| GO-57 | SKAL | `go test -race` kører på alle pakker. |
| GO-58 | BØR | Fuzz-tests for manifest-parsing og Arrow-afkodning i agenten. |
| GO-74 | SKAL | Genstartstests slår agenten ihjel mellem hvert trin i commit-rækkefølgen (7.3 og 7.4) og kontrollerer, at output hverken mangler eller er dubleret. |

### 12.1 Testcases

Testkomponenterne ligger i `test/components/` og bygges som images, der
skubbes til Zot i testmiljøet.

testcase01 og testcase02 er dem fra MCPipe-specifikationen og beholder
deres betydning (testcase01 er `sales.csv` med golden file, DK i alt
1846,50). Replikaer testes i en ny testcase med sit eget navn.

**testcase03-replicas: salgsrapport med replikaer.**

```
generate ──> filter (10 replikaer) ──> sum ──> resultat i S3
```

- `generate` laver en salgsrapport i 100 dele (chunk 1 til 100). Hver
  række er et salg: `order_id`, `date`, `country`, `product`,
  `quantity`, `amount`. Dataene laves ud fra et fast seed, så de er ens
  hver gang. Delene 10, 20, 30 … 100 indeholder ingen `DK`-rækker; alle
  andre dele indeholder `DK`-rækker blandt andre lande.
- `filter` kører med `replicas: 10`. Hver replika læser sine egne dele
  (GO-76) og sender kun `DK`-rækkerne videre. `generate` har én
  replika (p = 0), så del n går til replika n mod 10, og replika 0 får
  præcis delene 10, 20 … 100. Replika 0 laver derfor intet output og
  skal alligevel afslutte med EOS (GO-80). Det er testens svære kant.
- `sum` lægger `DK`-salget sammen pr. produkt (antal og beløb) og
  skriver summen til resultatet `result` ved EOS (GO-48, 5.2).
- Runnet er færdigt med fasen `Succeeded`, og `status.results` siger,
  hvor resultatet ligger, og hvilken Secret der giver adgang.

| ID | Niveau | Krav |
|---|---|---|
| GO-81 | SKAL | Testen kontrollerer, at summen i resultatet er lig med summen beregnet direkte fra `generate`s data. |
| GO-84 | SKAL | Testen læser de ti `filter`-replikaers ack-filer (GO-36) og kontrollerer, at mængderne af ackede chunks er disjunkte, at deres forening er præcis 1–100, og at chunk n ligger hos replika n mod 10. Testen afhænger ikke af logs. |
| GO-85 | SKAL | Testen kontrollerer, at replika 0's udgående manifest har nul chunks og `eos: true`, og at `sum` alligevel er nået til EOS. |
| GO-82 | SKAL | Testen henter resultatet med oplysningerne i `status.results` og intet andet, så den også tester, at de er nok. |

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
3. `sdk` med lokal testmode og en filter-komponent. *Færdig*, undtagen
   agent-mode, der kommer med agenten i trin 4.
4. Agenten med `builtin`-signaler, kørt lokalt mod Garage.
5. Controlleren med envtest.
6. E2E på Talos i Vagrant-VM'en med testcase01.
7. Fan-out og fan-in med testcase02, replikaer med testcase03-replicas.
8. Genstart, backpressure og kaos-test.
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
