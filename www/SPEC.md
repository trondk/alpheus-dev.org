# alpheus-dev.org — specifikation for hjemmesiden

Status: udkast, 2026-10-29. Siden findes som Hugo-site i `www/site/`, men
er ikke udgivet endnu.

Hjemmesiden er Alpheus' offentlige ansigt og dokumentation. Den forklarer,
hvad Alpheus er, hjælper en platform engineer i gang på under ti
minutter, og er hjem for Go-modulets vanity import. Krav-ID'er herfra
starter med `WWW-`.

Projektets repo'er ligger på en selvhostet Forgejo-instans, som er det
primære hjem. GitHub har et spejl, der kun læses fra. Adressen på
Forgejo-instansen er ikke besluttet og står her som `<forgejo>`.

Krav bruger SKAL og BØR. ID'er er stabile: et krav, der udgår, beholder
sit nummer med teksten "Udgået", og nye krav får det næste ledige nummer.


## 1. Formål og målgruppe

Siden skal få to typer læsere til at forstå Alpheus hurtigt:

| Læser | Hvad de vil vide | Hvor de finder det |
|---|---|---|
| Platform engineer | Hvordan installerer jeg det, og hvordan kører jeg et første flow? | Kom i gang, Docs |
| Arkitekt eller beslutningstager, ofte i offentlig sektor | Hvad løser det, er det modent, hvem står bag, hvordan er licens og governance? | Forsiden, Om projektet |

Det, siden ikke er: en markedsføringsside for et produkt. Tonen er
saglig, konkret og uden salgssprog.

| ID | Niveau | Krav |
|---|---|---|
| WWW-1 | SKAL | Sidens sprog er engelsk, så projektet kan bruges internationalt. |
| WWW-2 | BØR | Forsiden og "Om projektet" findes også på dansk, når projektet har danske brugere. |


## 2. Struktur

```
/                         Forside
/docs/                    Dokumentation (pr. version)
  getting-started/        Installation og første flow (findes: fra kilden på testklyngen)
  concepts/               StreamRun, trin, kanter, chunks, agent, signaler
  reference/streamrun/    CRD-reference, genereret fra api/v1alpha1
  cli/                    CLI-reference, genereret fra CLI'ens egen hjælp med make generate (findes)
  components/             Skriv en komponent i Go eller Python: et run trin for trin, skabeloner og eksempler (findes)
  operations/             Drift på Kubernetes og Talos, metrics, sikkerhed
  protocol/               Chunk-protokollen i detaljer
/development/             Udvikling og test: byg, tests og testklyngen i Vagrant
/about/                   Om projektet: historien, governance, licens, kontakt
/security/                Sådan rapporteres sårbarheder (genereres fra SECURITY.md)
/releases/                Udgivelser og release notes (genereres fra udgivelserne på Forgejo)
/privacy/                 Privatliv (se afsnit 7)
/alpheus/                 Vanity import for Go-modulet (se afsnit 5)
```

| ID | Niveau | Krav |
|---|---|---|
| WWW-3 | SKAL | Dokumentationen versioneres pr. minor-udgivelse (`/docs/v0.1/`), og `/docs/` peger på den seneste. Hver version bygges fra det seneste patch-tag i sin minor-serie. `/docs/dev/` bygges fra `main` og har en tydelig markering af, at den beskriver ikke-udgivet kode. Før første udgivelse peger `/docs/` på `/docs/dev/`. |
| WWW-4 | SKAL | CRD- og CLI-referencen genereres ved hvert byg fra det samme tag (eller `main`) som den dokumentationsversion, de hører til, så de aldrig kommer ud af trit med programmet. |
| WWW-5 | SKAL | Hver dokumentationsside har et "Edit this page"-link til kilden i det primære repo på Forgejo, ikke i spejlet. |
| WWW-6 | BØR | Søgning i dokumentationen kører helt i browseren (fx Pagefind), uden en ekstern søgetjeneste. |

Status 2026-10-31: før første udgivelse ligger dokumentationen direkte under `/docs/` med en tydelig markering af, at den beskriver koden på `main`. Versionering efter WWW-3 (`/docs/v0.1/`, `/docs/dev/`) kommer med `v0.1.0`. `/docs/` viser en liste over siderne (temaets `section.html`).


## 3. Indhold på forsiden

I rækkefølge, i én spalte:

1. **Navn og én sætning.** "Alpheus streams files between containers
   on Kubernetes, with nothing but object storage in between."
2. **Floden.** Sidens signatur (se 4.4): en linje, der løber, forsvinder
   under en horisont og dukker op igen, med en kort tekst om myten.
3. **Hvad det gør**, i tre konkrete udsagn:
   - Every step runs at the same time; data moves as chunks in S3, in any file format.
   - No message broker for data, only Kubernetes and an S3 endpoint.
   - The same component runs in batch and in streaming.
4. **Hvordan det virker**: en tegning af et run med tre trin på
   Kubernetes (WWW-47) og derefter en almindelig liste: controlleren
   kører trinnene som pods, komponenten læser og skriver filer, agenten
   flytter dem og committer dem i manifester, signaler går gennem hub'en
   med mTLS, og et trin genstarter fra sine manifester.
5. **Et minimalt eksempel**: en StreamRun på 15–20 linjer og
   `alpheus run -f flow.yaml`.
6. **Installation** i én kommando (Helm) og et link til Kom i gang.
7. **Status**: projektets aktuelle fase (se WWW-8), roadmap'en med de
   færdige trin og det næste (fra `data/roadmap.toml`, efter afsnit 15 i
   hovedrepo'ets `SPEC.md`) og et link til release notes.
8. **Fod**: licens, governance, sikkerhed, repo, kontakt.

| ID | Niveau | Krav |
|---|---|---|
| WWW-7 | SKAL | Ingen påstande uden belæg: ingen ydelsestal, kundelogoer eller citater, før de findes og kan dokumenteres. |
| WWW-8 | SKAL | Projektets modenhed står tydeligt på forsiden, så ingen tror, det er produktionsklart før det er. Fasen er én af *design* (intet kører endnu, selv om der kan være kode), *alpha*, *beta* eller *stable* og sættes ét sted i konfigurationen. I fasen *design* står der også ved eksemplet, hvor langt API'et er (fx at det findes som CRD, men at intet kører det endnu). |


## 4. Visuel identitet

### 4.1 Udgangspunkt

Floden Alpheus løber gennem kalkstenslandskab i Arkadien, forsvinder ned
i undergrunden og dukker op igen. Identiteten bygger på det: lys kalksten
over jorden, dybt grundvand under den, og én linje, der forbinder dem.

### 4.2 Farver

| Navn | Lyst tema | Mørkt tema | Brug |
|---|---|---|---|
| Kalksten | `#EEF0EE` | `#EEF0EE` | Baggrund i lyst tema, brødtekst i mørkt tema |
| Skifer | `#23292D` | `#23292D` | Brødtekst i lyst tema, baggrund i mørkt tema |
| Grundvand | `#0F4A55` | `#7FB8C2` | Overskrifter, floden under jorden |
| Kilde | `#1F6E6C` | `#5FC2BD` | Links og floden over jorden |
| Sediment | `#5B615A` | `#9AA19A` | Sekundær tekst |
| Horisont | `#80857D` | `#80857D` | Kun horisontlinjen og andre ikke-tekst-elementer |
| Flade | `#E4E7E4` | `#1A1F22` | Baggrund for kodeblokke, inline kode og andre flader |

Kontrast mod baggrunden (WCAG): Skifer og Kalksten 12,9:1, Grundvand 8,6
og 6,7, Kilde 5,2 og 7,0, Sediment 5,6 og 5,6. Horisont når 3,3 og 3,9,
nok til grafik (WCAG 1.4.11, mindst 3:1), men ikke til tekst, og bruges
derfor aldrig til tekst.

Kontrast mod Flade (lyst og mørkt tema): Skifer og Kalksten 11,8 og 14,5,
Grundvand 7,9 og 7,6, Kilde 4,8 og 7,9, Sediment 5,1 og 6,3. Alle
tekstfarver kan altså bruges på en flade. I mørkt tema er fladen mørkere
end baggrunden, som grundvand under kalkstenen.

Kilde adskiller sig ikke nok fra brødteksten (2,5:1 og 1,8:1 mod Skifer
og Kalksten), så links kan ikke kendes på farven alene (se WWW-40).

| ID | Niveau | Krav |
|---|---|---|
| WWW-9 | SKAL | Alle tekst- og baggrundskombinationer opfylder WCAG 2.1 AA (kontrast mindst 4,5:1 for brødtekst). Kombinationerne kontrolleres i CI. |
| WWW-10 | SKAL | Lyst og mørkt tema følger `prefers-color-scheme`. Hver farve har sin egen værdi i hvert tema, som i tabellen. |
| WWW-40 | SKAL | Links i løbende tekst er altid understreget. Kun links i navigation og fod, hvor placeringen viser, at det er links, må undlade understregning. |
| WWW-41 | SKAL | Flader, fokus og syntaksfremhævning bruger kun farverne i tabellen. Fokus vises som en 3 px kontur i Kilde (mindst 5,2:1 mod baggrunden). Syntaksfremhævning i kodeblokke bruger Grundvand til nøgler og nøgleord, Kilde til strenge og Sediment til kommentarer og prompt, på Flade. Alle kombinationer kontrolleres i CI som i WWW-9. |

### 4.3 Typografi

| Rolle | Skrift | Begrundelse |
|---|---|---|
| Overskrifter | Spectral | En klassisk serif med græsk-antik klang uden at være kostume |
| Brødtekst | Atkinson Hyperlegible Next | Bygget til læsbarhed; passer til en målgruppe, hvor tilgængelighed er et krav |
| Kode | JetBrains Mono | Tydelig forskel på 0/O og 1/l i YAML og kommandoer |

Alle tre har SIL Open Font License.

| ID | Niveau | Krav |
|---|---|---|
| WWW-11 | SKAL | Skrifter hostes på siden selv (WOFF2, subsettet). Ingen Google Fonts eller anden ekstern skrifttjeneste, fordi det sender besøgendes IP-adresser til tredjepart. |
| WWW-12 | SKAL | Linjelængden i brødtekst er højst 75 tegn. |
| WWW-42 | BØR | Højst fire skriftfiler indlæses på forsiden: Spectral i én vægt (fx 600), Atkinson Hyperlegible Next som variabel skrift (opret og kursiv) og JetBrains Mono i vægt 400. Det holder forsiden under grænsen i WWW-26. |

### 4.4 Signaturen: floden

Én ting på siden må være mindeværdig, og det er floden: en enkelt
SVG-linje i Kilde-farven, der løber ind fra venstre, går ned under en
tynd horisontlinje i Horisont, fortsætter stiplet i Grundvand under
jorden og kommer op igen til højre. Den er en tegning af præcis det,
Alpheus gør med data mellem trin.

```
 Alpheus
 Streams files between containers on Kubernetes, with
 nothing but object storage in between.

   ~~~~~~~~~~~\                                  /~~~~~~~~~>
 ______________\________________________________/____________
                 '- - - - - - - - - - - - - - -'
                       chunks in object storage
```

| ID | Niveau | Krav |
|---|---|---|
| WWW-13 | SKAL | Floden er inline SVG med `role="img"` og en tekstbeskrivelse. |
| WWW-14 | SKAL | Floden tegnes op én gang ved indlæsning (stroke-animation, under 2 sekunder) og står derefter stille. Med `prefers-reduced-motion` vises den færdig uden animation. |
| WWW-15 | SKAL | Ingen anden dekorativ animation på siden. |
| WWW-16 | BØR | Samme linje i lille udgave bruges som logo og favicon. |
| WWW-47 | SKAL | Tegningen under "Hvordan det virker" viser et run på Kubernetes i flodens sprog: StreamRun og controller med signal-hub'en øverst, en pod pr. trin med komponent og agent, og under horisonten objektlageret, hvor data løber som floden fra agent til agent. Den er inline SVG med `role="img"`, titel og beskrivelse, står stille (WWW-15), bruger kun farverne i 4.2 i begge temaer og har større tekst på telefoner. Listen under den siger det samme i tekst. |

### 4.5 Layout

- Én venstrestillet spalte med god luft, også på store skærme.
- Ingen kort-grids eller ikonrækker. De tre udsagn på forsiden står som
  en almindelig liste.
- Dokumentationen har en venstre navigation, der folder sig sammen til
  en menu på telefoner.


## 5. Vanity import for Go

Go-modulet hedder `alpheus-dev.org/alpheus`. Når `go` henter modulet,
spørger den `https://alpheus-dev.org/alpheus?go-get=1` og forventer
meta-tags, der peger på det primære repo på Forgejo.

```html
<meta name="go-import" content="alpheus-dev.org/alpheus git https://<forgejo>/alpheus-dev/alpheus">
<meta name="go-source" content="alpheus-dev.org/alpheus https://<forgejo>/alpheus-dev/alpheus https://<forgejo>/alpheus-dev/alpheus/src/branch/main{/dir} https://<forgejo>/alpheus-dev/alpheus/src/branch/main{/dir}/{file}#L{line}">
```

Er Forgejo-instansen nede, kan `go` stadig hente udgivne versioner
gennem proxy.golang.org, som har dem i sin cache.

| ID | Niveau | Krav |
|---|---|---|
| WWW-17 | SKAL | `/alpheus/` og hver offentlig pakke (`/alpheus/sdk/`, `/alpheus/api/v1alpha1/`) har siden med meta-tags. Listen over pakker genereres ved byg med `go list ./...` i hovedrepo'et, uden `internal/`-pakker, så nye pakker kommer med automatisk. |
| WWW-18 | SKAL | En besøgende i en browser på `/alpheus/` ser en kort side med links til repo'et og til pkg.go.dev, ikke en tom side. |
| WWW-19 | SKAL | Repo-URL'en i `go-import`, "Edit this page" og alle andre links til koden ændres ét sted i konfigurationen, hvis hjemmet flytter. Den peger på det primære repo, aldrig på spejlet. Modulstien ændres aldrig. |
| WWW-20 | SKAL | CI tester efter hver udgivelse af siden, at `go list -m alpheus-dev.org/alpheus@latest` virker mod den udgivne side. Testen kører med `GOPROXY=direct`, `GONOSUMDB=alpheus-dev.org` og en tom modulcache, så den rammer siden og ikke proxy.golang.org's cache. Den tester også én underpakke. |


## 6. Teknik

| ID | Niveau | Krav |
|---|---|---|
| WWW-21 | SKAL | Siden bygges med Hugo (statisk, en enkelt Go-binær). Hugo-versionen fastlåses i CI. |
| WWW-22 | SKAL | Siden ligger i repo'et `alpheus-dev/alpheus-dev.org` på Forgejo. Dokumentationens kilde ligger i `docs/` i hovedrepo'et og hentes ind ved byg (fra `main` og fra hvert release-tag, se WWW-3), så dokumentation og kode ændres i samme pull request. |
| WWW-23 | SKAL | Forgejo Actions bygger og udgiver til projektets egen webserver ved push til `main` i begge repo'er og ved hver udgivelse af Alpheus. Hovedrepo'et udløser bygget via Forgejos API med en token, der kun har adgang til netop det ene repo. Udgivelsen sker med rsync over SSH med en nøgle, der kun kan skrive i sidens mappe på serveren. |
| WWW-44 | SKAL | Spejlene på GitHub opdateres automatisk fra Forgejo og kører ingen workflows. Issues, pull requests og Actions er slået fra, og beskrivelsen peger på det primære repo. |
| WWW-24 | SKAL | Siden virker fuldt ud uden JavaScript. JavaScript bruges kun til søgning og kopier-knapper på kodeblokke. Menuen på telefoner bygges med `<details>`/`<summary>`, og kopier-knapper indsættes af scriptet, så de ikke står døde uden JavaScript. |
| WWW-43 | SKAL | Hver side har en Content-Security-Policy som `<meta http-equiv>`, så den også gælder i forhåndsvisninger, og webserveren sender samme politik som header med `frame-ancestors 'none'` tilføjet (se afsnit 9). Politikken tillader kun ressourcer fra siden selv: `default-src 'self'`, `script-src 'self' 'wasm-unsafe-eval'` (Pagefind bruger WebAssembly), ingen `'unsafe-inline'`. Hugo sættes op med `markup.highlight.noClasses = false`, så syntaksfremhævning bruger klasser i stedet for inline styles. |
| WWW-25 | SKAL | Ingen eksterne ressourcer: ingen CDN'er, scripts, skrifter, billeder eller iframes fra andre domæner. |
| WWW-26 | SKAL | Forsiden vejer under 150 KB samlet ved første indlæsning, inklusive skrifter. |
| WWW-27 | BØR | Lighthouse-score på mindst 95 i alle kategorier, kontrolleret i CI. |


## 7. Privatliv og GDPR

| ID | Niveau | Krav |
|---|---|---|
| WWW-28 | SKAL | Ingen cookies, ingen tracking og ingen tredjeparts-analytics. Derfor er der ingen cookie-banner. |
| WWW-29 | SKAL | En kort privatlivsside forklarer, at siden ikke indsamler data, hvad webserveren logger, og hvor længe loggen gemmes. |
| WWW-45 | SKAL | Webserverens adgangslog gemmer kun forkortede IP-adresser (sidste oktet i IPv4 og de sidste 80 bit i IPv6 sat til nul) og slettes efter højst 14 dage. Fejlloggen må ikke indeholde IP-adresser. |
| WWW-30 | BØR | Hvis besøgstal bliver nødvendige, bruges en selvhostet, cookie-fri løsning, og privatlivssiden opdateres først. |


## 8. Tilgængelighed

| ID | Niveau | Krav |
|---|---|---|
| WWW-31 | SKAL | Siden opfylder WCAG 2.1 AA: semantisk HTML, synligt tastaturfokus, alt-tekst, korrekt overskriftshierarki og "spring til indhold"-link. |
| WWW-32 | SKAL | Alt kan bruges med tastatur alene, også navigation og søgning. |
| WWW-33 | SKAL | Siden kan vises i 320 CSS-pixels bredde (svarende til 400 % zoom, WCAG 1.4.10) uden vandret scroll i brødteksten. Kodeblokke og tabeller må scrolle inden for deres egen boks. |
| WWW-34 | SKAL | En automatisk tilgængelighedstest (axe) kører i CI på forsiden og en dokumentationsside. |


## 9. Domæne og hosting

| ID | Niveau | Krav |
|---|---|---|
| WWW-35 | BØR | Domænet er verificeret i GitHub-org'en `alpheus-dev`, så spejlet viser, at det hører til projektet. Ingen DNS-records peger på GitHub. |
| WWW-36 | SKAL | Apex-domænet peger på projektets webserver med A- og AAAA-records. `www.alpheus-dev.org` peger på samme server, som omdirigerer permanent (301) til apex. |
| WWW-37 | SKAL | Siden svarer kun over HTTPS med TLS 1.2 eller nyere. HTTP omdirigeres permanent til HTTPS. Certifikatet fornyes automatisk med ACME. |
| WWW-46 | SKAL | Webserveren sender `Strict-Transport-Security` (mindst et år), `Content-Security-Policy` (WWW-43), `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer` og `Permissions-Policy`, der slår kamera, mikrofon og position fra. |
| WWW-38 | SKAL | Domænet har auto-fornyelse, transfer lock og DNSSEC, hvis registraren understøtter det. |
| WWW-39 | BØR | En CAA-record begrænser, hvilke certifikatudstedere der må udstede for domænet. |

Siden ligger på projektets egen webserver, så ingen tredjepart ser
besøgendes IP-adresser, og serveren kan sende sikkerhedsheadere.
`frame-ancestors` virker kun som header, ikke i et meta-tag, og derfor
sender serveren CSP'en som header, selv om den også står i hver side.


## 10. Accepttest

- [ ] Forsiden viser navn, én sætning, floden, tre udsagn, et eksempel,
      installation og status uden at scrolle vandret på 320 px bredde.
- [ ] Forsiden vejer under 150 KB ved første indlæsning (WWW-26).
- [ ] `go list -m alpheus-dev.org/alpheus@latest` virker med
      `GOPROXY=direct`, også for en underpakke (WWW-20).
- [ ] `/alpheus/` i en browser viser links til repo og pkg.go.dev.
- [ ] Ingen forespørgsler til andre domæner end alpheus-dev.org, målt i
      browserens netværksfane.
- [ ] Ingen cookies sat.
- [ ] axe finder ingen fejl på forsiden og en dokumentationsside.
- [ ] Floden vises uden animation med `prefers-reduced-motion`.
- [ ] CRD-referencen svarer til `api/v1alpha1` i den seneste udgivelse.
- [ ] Siden virker med JavaScript slået fra, bortset fra søgning.
- [ ] Browserens konsol viser ingen CSP-fejl, heller ikke under søgning.
- [ ] Alle skrifter hentes fra alpheus-dev.org.
- [ ] Hver dokumentationsside har et virkende "Edit this page"-link til
      Forgejo.
- [ ] Headerne fra WWW-46 sendes, og `http://` og `www.` omdirigerer til
      `https://alpheus-dev.org/`.
- [ ] Adgangsloggen indeholder kun forkortede IP-adresser (WWW-45).
- [ ] Spejlet på GitHub har ingen workflows, issues eller pull requests.


## 11. Åbne spørgsmål

- Skal en dansk version af forsiden med fra start?
- Hvad bliver adressen på Forgejo-instansen?
- Hvilken webserver kører siden (fx Caddy eller nginx), og hvem driver
  serveren?
- Hvor skal sårbarheder rapporteres, når GitHubs private
  sårbarhedsrapportering ikke kan bruges på et spejl? Fx en
  e-mailadresse som `security@alpheus-dev.org`.
- Hvem godkender indhold på siden: kun maintainers, eller også
  Redpill Linpro, hvis firmaet står bag?
