<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

# projectfile/bridge — agent guide

## Purpose

`pf-bridge` — **projections and detection** over a projectfile document. Adapts
the projectfile to external representations and back. Since the §5 split it is a
**git-style multi-binary tool**: a thin `pf-bridge` dispatcher execs one binary
per bridge on PATH.

- `pf-bridge <name> [to|from] [dir]` → execs `pf-bridge-<name>`. Each per-bridge binary round-trips its metadata file(s) (package.json via `npm`, CITATION.cff via `cff`, …) or derive-renders (LICENSE via `license`, .gitignore family via `ignore`, …). Direction is `to`/`from`/none; a single-file binary implies its filename, a multi-file one (`ignore`, `vulnerabilities`) takes the name or `all`. The `license` bridge renders the substituted root LICENSE **and** `LICENSES/<id>.txt` per SPDX term (also substituted by default; `--reuse-canonical` keeps the literal SPDX placeholders to match `reuse download`).
- `pf-bridge all` / `to all` / `from all` — fan out across every installed file bridge.
- `pf-bridge forge|scan|init` → execs `pf-bridge-forge` (push identity to GitHub/GitLab/Forgejo), `pf-bridge-scan` (filesystem/Git scanners), or
    `pf-bridge-init` (scaffold a new projectfile — engine behind `pf-cli init`, which delegates here when installed).

The **document backend** (read/query/mutate/convert/validate) is the sibling
`projectfile/core` module (`pf-cli`). This module was extracted from it in the
Bridge Revolution Phase 2 cut — see [../bridge-revolution.md](../bridge-revolution.md).

## The seam — how bridge consumes core

`go.mod` requires `kiota.ch/projectfile/core` as a pinned module (no `replace`;
core tags releases — currently `v1.0.1`). Every core dependency crosses through
core’s **`pkg/*` façades** — this module
holds **zero** `cli/internal/*` imports. The core API in use: `pkg/projectfile`
(the typed `*Document` — alias identity means it crosses zero-cost),
`pkg/{genlog,rawdoc,userconfig,spdx,selector,pflock,fieldpath}`. Mutable core
toggles are driven via setters (`genlog.SetQuiet`, `projectfile.SetYAMLOutputSorted`,
`spdx.SetEmbedded`, …) exposed on those façades.

## SPDX licence corpus (`internal/bridge/license/spdx/`)

Bridge owns the SPDX boilerplate texts and registers them with core’s resolver as
lookup tier 1 (`spdx.SetEmbedded`, called from the license bridge’s `init`). Core
ships **none** — it is a consumed library, so a `go:embed` asset there would have
to be tracked in a repository that never reads it; when it was untracked instead,
`core/v2@v2.0.0` shipped an empty set and broke every offline consumer.

The texts are **committed**, not fetched in CI. On the forge plane every DAG node
is its own job with its own checkout, and the corpus is needed in three of them
(`source-is-tested`, `binaries-built`, `image-is-built`); `pf-ci` refuses to place
one tool in three unrelated nodes (“node=job cannot place it”). Committing serves
all three with no mechanism at all and keeps a network round-trip out of every
build. `.scripts/download-spdx.sh` is the manual refresh — run it, commit the diff.

Each `<id>.txt` carries a `<id>.txt.license` REUSE sidecar (a licence text cannot
hold an inline header without corrupting the text it is). The tags record what we
received: SPDX publishes `license-list-data` under CC0-1.0 with no asserted holder,
so `LICENSES/CC0-1.0.txt` ships alongside `MIT.txt`. The sidecars are written by
`download-spdx.sh` next to each fetch, so a newly added ID can never arrive
unannotated and break `reuse-lint` in a later, unrelated run. Note the SPDX ID
correction: `BUSL-1.1` is canonical for the Business Source License — `BSL-1.1`
404s upstream and is a different licence (Boost).

Bridge-owned typed shapes of `org.projectfile.*` extension namespaces
(citation, readme, forge, funding, acknowledgements, codeowners, contributing,
support, security, release, conventions, ignores, vulnerabilities,
editors, dei) and their accessors live in `internal/pfmodel` below — moved out of
core in the core-2.0 cut. Bridge code reaches them there, not through
`pkg/projectfile`.

## Layout

Read [docs/readme-generator.md](docs/readme-generator.md) before touching
`internal/bridge/readme/` — it is the canonical description of the block list,
the 4-tier block resolution, the artifact-driven command sections, and the
interpolation/fan-out rules. Two contracts there are easy to break by accident:

- **Sections are a LIST of groups**, one per artifact, because installation
  alternatives are not steps. A group whose commands all fail to resolve drops
  whole; a section with no surviving group falls back to its file probe.
- **Install and usage instructions come from `org.projectfile.artifacts`**, not
  from which namespace a project lives in. A shared m6e fragment declares a
  recipe per ecosystem unconditionally and the drop rule selects. Never add a
  Go-side table of package managers or an `if kind == …` in a lowering — `kind`
  stays advisory for lowering by spec.
- **`priority` orders peers within a list** (shields within a row; `links[]`
  within a category/kind, propagating to readme, SUPPORT and CONTRIBUTING).
  Higher renders first; default `50`; the sort is stable so unset items keep
  declaration order. The ONE direction lives in `pfmodel.ByPriorityDesc` — every
  priority sort calls it, never an inline comparator. Link priority rides the
  §139 `Extra` channel (`pfmodel.LinkPriority`), the same path `tags` uses, so
  no schema change. It reorders peers only, never the semantic groupings
  (rows, link categories, the where-to-ask ladder).

```text
bridge/
├── main.go                 pf-bridge dispatcher (PATH discovery + exec; links NO bridges)
├── cmd/pf-bridge-*/         one tiny main per binary (17 bridges + forge/scan/init);
│                            generated by .scripts/gen-bridge-cmds.sh
├── go.mod                  module …/bridge; require …/core (pinned, no replace)
└── internal/
    ├── bridgerun/          bridge command run logic + Main(binName) (registry-driven)
    ├── rootflags/          shared persistent flags + PersistentPreRun + ReadOpts/Offline
    ├── describe/           the --describe probe spelling shared by dispatcher and children
    ├── buildinfo/          Version var (single -ldflags -X target for every binary)
    ├── forgecmd/ scancmd/ initcmd/ cachecmd/   each: a command + Main(binName)
    ├── pfmodel/            bridge-owned projectfile extension shapes (citation, readme,
    │                        forge, funding, ...) — moved out of core in the core-2.0 cut
    ├── bridge/             every projectfile↔external-file bridge + core/ contract + registry
    ├── forge/              forge push (core/ + drivers/{github,gitlab,forgejo} + hostmatch/)
    ├── scanners/           stack + git + forge + sinks scanners (core registry)
    ├── source/             `init` ecosystem auto-detection (reuses bridge parsers)
    ├── scaffold/           interactive `init` TUI (runs scanners)
    ├── derive/             read-time fields (forge remotes, sink refs) + containers (registry pages, run by `scan sinks`)
    └── warn/               warning ledger + end-of-run summary (+ fan-out handoff)
```

### Forge templates (`internal/scanners/forge`)

`links[]` and `repositories[]` are MATERIALIZED, never derived: the file on
disk carries concrete URLs, because a link validator, an indexer or a human
reads it without pf-cli. A fleet fragment declares the grammar once as
templates under `org.projectfile.forge.links[]` / `.repositories[]` — the
spec shapes, with `${path}` / `${flatpath}` / `${name}` read relative to the
forge namespace — and `pf-bridge scan forge` expands each against the merged
document and gap-fills the result through `applyLink` / `applyRepository`
(`--force` rewrites label, preferred, tags and priority). A template that does not
resolve is skipped with a warning, so a `${` never reaches the file. A
template with no `label` gets the same noun placeholder `git-remotes` writes,
which the title promotion rewrites per declared language. Templates never sit
in the spec lists themselves: pf-cli does no interpolation, so a `${…}` there
would reach every consumer of the merged document.

### Registry pages (`internal/scanners/sinks`)

`links[type=package-registry]` is materialized the same way: `pf-bridge scan
sinks` runs `derive/containers` over the merged document and proposes one
landing page per PUBLIC sink (`ghcr.io` → the package under the GitHub
repository, `docker.io` → the Docker Hub repository; kiota, ECR and quay have
no public page), fanned out per matrix cell when the image path names an axis.
The GHCR page hangs off the GitHub repository, so the scanner folds in the
forge links `Materialize` stages in the same run — `scan all` converges in one
pass on a project that just gained its GitHub fragment. Nothing else writes
this link: a bridge sync (`cff`, `shard`, `npm`, …) touches only the fields its
file maps, so `pf-cli del` followed by a sync leaves the link gone until the
scanner runs again.

### Forge capability aliases (`internal/derive/forges`)

`Remotes()` files each `links[type=source-code]` mirror under its slug (first
domain label) **and** under every capability it claims, both addresses sharing
one coordinate map. A shared fragment therefore writes
`${org.projectfile.forge.remotes.badges.host}` instead of naming a forge, and
each project answers with its own mirror.

Claims come from three places: the advisory `tags` list on the link (§139
`Extra`, the channel `priority` uses), `links[].preferred` → `preferred`, and
`repositories[issues=true]` → `issues`, matched to the link naming the same
repository so the tracker is declared once even though the repository entry
holds an unusable SSH URL. `hostOwnerRepo` is what reads that triple out of any
transport, including the scp-style `git@host:o/r.git` that carries no scheme.
`repositories[releases=true]` (a §139 extra key, read like `cffr`) → `releases`
plus `releases-<route>` the same way, except that several entries may carry it
— the origin and every mirror a release lands on — so each claim rides at its
naming link’s priority and the most public mirror takes the bare alias. No
fragment sets it: a forge is not where releases go, a release trait is.

An alias never shadows a slug (a slug is an identity; the claim is refused and
warned), and the highest link `priority` wins a contested alias, ties in
document order — so a fragment’s claim holds however a project orders its
`includes:`. The vocabulary is deliberately open — registering words would put the fleet’s topology back in
this repository, which is the thing the aliases exist to remove.

`hostmatch.Rule.Capabilities` holds the only tags a hostname settles, and
`pf-bridge scan git-remotes` proposes them. Host knowledge lives in the rule
table and nowhere else. Two limits are deliberate: only `public` and the
`badges*` family are host facts (`ci` is a per-project choice, `releases` a repository one),
and every self-hosted prefix rule proposes nothing, since `gitlab.` also
matches an internal instance.

`badges` is host-agnostic; `badges-github` / `badges-gitlab` / `badges-gitea`
name a **shields.io route family**, which is NOT the forge kind — Codeberg is
kind `forgejo` and shields calls its route `gitea`. A fragment declares one
badge per route and lets the drop rule render the one whose mirror exists.
That is how a badge branches in a grammar with no conditional.

### Publish sinks (`internal/derive/ocisinks`)

`org.projectfile.sinks` is the OCI plane: where a project’s IMAGES land. A
**sink** is a named destination whose name is a LABEL — nothing reads meaning
into it, so two sinks may address one host under two accounts, and credentials
therefore key on the sink name rather than the host. A peer of the forge plane,
never derived from it: ECR has no repositories, and Codeberg has an issue
tracker but no build minutes. Do not confuse the package with its sibling
`derive/registries`, which infers PACKAGE index pages (npm, PyPI, crates.io)
from the detected stack.

**The composition is not here.** `core/pkg/sink` owns it, and this package binds
only the COORDINATES — this project’s `image.basename` and `image.tag`. That
split is what lets the same composer answer “where do I push this project” here
and “where does this project’s BASE image live” in `pf-cli sink ref`, where the
coordinates name a foreign project. The reader lives in core for the same
reason: cli, bridge and pf-ci must not each own a list of the destinations.

`Refs()` feeds **`AddVirtual`, never a scanner**. That split is load-bearing: a
scanner proposes entries the command PERSISTS, and the fleet’s sink entries
arrive through an include, so a write would copy include data into every base
document. `AddVirtual` computes into the in-memory merged document only, where
`forge.remotes` lives.

A composed ref is COMPLETE — every `${…}` resolved — or it is DROPPED. A `{AXIS}`
placeholder survives because it carries no `$`, and the readme’s `expandAxes`
runs after interpolation, so one composed ref still fans out into one pull line
per matrix cell. Refusing a half-resolved ref is the load-bearing half: a
reference that silently lost a segment is a push to the wrong repository.

Every composed entry carries a `role`, defaulting to `primary`, because **a bare
`{}` projection admits no trailing field** — core allows only `keys`/`values`
after one, so `.ref` is reachable across a mapping ONLY through a `{k=v}`
selector. An entry with no role would be addressable by nothing.

A document declaring no sinks namespace gets one primary entry synthesized from
the legacy `org.projectfile.readme.registry` scalar, so the ~130 projects that
predate the namespace render an identical pull line with no edit. That fallback
is a readme concern and lives HERE, not in core: `pf-cli sink` reports the
absence instead, because the build plane must refuse an unrecognised destination
rather than guess one.

`pfmodel.SetLinkTags` is the sole writer. It treats an absent `tags` key as the
gap, so a declared list — `tags: []` included — wins. `scan --force` overrides
that, and exists because gap-fill alone would make the first fleet-wide scan
irreversible. It applies to `tags` only, and it REPLACES rather than merges: a
link that also declares `ci` or `releases` loses them, because no host proposes
a project decision. Force what the scanner wrote; hand-edit what a human added.

**Do not point a shared m6e fragment at an alias until the fleet declares it.**
Slugs are unchanged and keep working, so the mechanism is purely additive; but a
fragment flipped to `remotes.badges` renders NOTHING for every project that has
not tagged its links, and the drop rule makes that silent. See
[the rollout plan](../../.agents/FORGES-REGISTRIES.md).

### The drift gate (`--check`)

`--check` renders, compares, and writes nothing; a content mismatch AND a
missing file both count as drift, each reported with its unified diff.

**Drift WARNS by default; it does not fail.** `--fail-on-drift` (or
`PF_BRIDGE_FAIL_ON_DRIFT=1`) restores the hard gate. The reason the default sits
on the lenient side is the fleet’s two-step rollout: a bridge fix is published
first and the image every project runs is rebuilt second, and in between every
consumer drifts through no fault of its own. A hard gate at that moment blocks
the commits that carry the fix forward — the gate ends up defending the bug.

The library keeps the strict contract: `core.Options.WarnOnly` defaults to false,
so a caller that does nothing gets an error on drift. Only the CLI opts into
leniency, and the environment variable exists because the `--check` call sites
live in m6e tool rows pinned per project: flipping the fleet back through them
is a library edit plus a pin bump everywhere, while a CI plane exports one
variable.

What makes the lenient default safe is that a warning is no longer quiet:

- `internal/warn` keeps every warning and re-states the set as the LAST thing
  the process writes, with the remedy under it.
- `m6e-run` greps the captured log of a SUCCESSFUL run and quotes the warning
  lines under its green pointer (`M6E_WARN_REVEAL=N` disables). Without that
  half, a warn-only gate under `make` would print nothing at all — the runner
  buffers the whole tool log and shows one line when the command exits 0.

Do not "fix" a drift warning by muting it. `--check` reporting nothing when a
file has drifted is the failure mode both halves exist to prevent.

### Fragment inheritance

`internal/bridge/fragments/` assembles `docs/<name>.d/*.md` into one document
(`FEATURES.md`, `ROADMAP.md`, …) and nests what upstream projects publish under
it. Parents are **repository URLs**, never filesystem paths: a path assumes
every project sits in one working copy, which silently inherited nothing for
every consumer who checked out one project alone.

Two rules hold the design together:

- **Generate fetches; everything else re-reads.** A writing run online reads
  each parent’s published document live and nests it — the committed assembled
  document is the only record of inherited content. Offline runs, and the
  check, preview and dry-run forms, re-read the inherited sections verbatim
  out of that committed document, so the drift gate never needs a network and
  two offline runs always agree.
- **All parents or none, per document.** The fetch resolves each parent’s
  newest release tag (`git ls-remote --sort=-v:refname`, so Git orders the
  versions) and reads the canonical path plus each declared language’s
  `docs/<lang>/<Out>` at that one ref with `git archive --remote`. One
  unreachable parent keeps the whole document on its committed sections — a
  forge outage preserves content instead of deleting it, and fresh and stale
  sections never mix in one file. Everything goes over Git — the transport
  these repositories already use — so there is no forge API, no per-forge
  raw-URL table, and a private parent resolves with the developer’s own
  credentials.

Why no version in the heading: the fetch still resolves and records the tag it
read, but printing it would rewrite every child’s FEATURES.md and README.md on
each parent release while the inherited list itself rarely changes — the fleet
logged 150 commits that changed nothing but the version. The parent’s title
scopes the claim (“Inherited from B19/Ubuntu”, not an anonymous “Inherited
Features”); the tag actually read is the one the logs print.

The parent is named by its own `identity.title`, read at the same ref as the
document. A heading is prose, and `owner/repo` is a path: every prose linter
downstream reads `b19/ubuntu` as a misspelling of the product, which failed
`documentation-passes-lint` in every consumer at once. A parent that publishes
no title falls back to `owner/repo`.

Three behaviours are load-bearing and easy to break:

- The parent’s document already carries what IT inherited, so one hop copies the
  whole chain. There is no ancestry walk and no cycle problem to solve.
- The parent’s H1/H2 headings are dropped so its H3 entries nest — the level the
  readme bridge scrapes. Fenced blocks are exempt, or a `# comment` inside a
  shell example would be eaten.
- The offline re-reader keys on H2 boundaries: the localized “Project …”
  heading is skipped, every other H2 is one parent section captured verbatim.
  That layout is the contract between assembly and the offline run — the
  round-trip test pins it byte for byte.

A parent that cannot be reached warns and keeps the committed sections.
Refusing to nest a document with no SPDX header is deliberate — the
alternative is unlicensed text in this repository over a fault upstream owns.

Two things bite when adding a parent:

- **The nested text is linted by the consumer, not by upstream.** Prose that
  passes upstream can fail this project’s textlint or markdownlint. Fix it in
  the parent and cut a release — generate reads the newest release TAG, so an
  unreleased fix on the parent’s `main` never reaches a child.
- **GitHub cannot serve a parent.** It disables `upload-archive`, so `git
  archive --remote` fails there with “operation not supported by protocol”.
  Point parents at a forge that serves it (kiota.ch, Codeberg), not at a GitHub
  mirror.

### Fan-out verbs

`pf-bridge all` (sync every file bridge) and `pf-bridge check` (writing nothing)
are built into the dispatcher — there is no `pf-bridge-check` binary, because
check is a MODE every bridge already has.

The two verbs choose their set differently, and the difference is the point:

| verb | set | source |
| --- | --- | --- |
| `all`, `to all`, `from all` | every installed file bridge | PATH |
| `check` | the bridges the PROJECT declares | `org.projectfile.ci.tools` rows spelling `pf-bridge … --check` |
| `check <name>…` | only those | the command line |
| `check --all` | every installed file bridge | PATH |

A PATH sweep answers the wrong question for a gate. Every consumer of the m6e
fragments inherits ignore patterns, a release config and yamllint rules, so a
sweep renders `.yamllint`, `.containerignore` and `.releaserc.yaml` for projects
that maintain none of them and calls each one drift — while `funding` fails
outright on a project that declares no sponsors. The CI manifest already names
the exact set (`internal/declared` reads it), so the gate verifies what the
project derives and the whole thing costs the CI plane ONE container instead of
one per bridge. A project with no such rows falls back to the PATH sweep, which
keeps the tool useful outside a projectfile-driven fleet.

A declared row naming a bridge this install does not carry is warned and
skipped, never fatal — a bridge reaches the fleet in two steps (publish the
tool, rebuild the image), and a project legitimately declares a check its image
cannot run yet.

**Every flag typed after a fan-out verb is forwarded to each child.** It was not
always: the child argv was rebuilt from scratch, so `pf-bridge all --check`
reached each bridge as a plain `all` and OVERWROTE every derived file — a
read-only verb doing the one thing it promised not to. `splitNamesFlags` owns
that split now and is table-tested.

Children inherit `PF_BRIDGE_WARNINGS_DIR` and hand their ledgers back instead of
each printing its own block, so a 20-bridge sweep ends in one summary.

### The binary split (§5)

One binary per bridge, git-style dispatched (decision §11.2). Each
`cmd/pf-bridge-<pkg>/main.go` blank-imports **only its own** bridge package and
calls `bridgerun.Main` — so that binary links only its code, and the same
registry-driven command sees only its own filename(s). `pf-bridge` (main.go)
carries no bridges: it discovers `pf-bridge-*` on PATH and `syscall.Exec`s the
match; `all` fans out across the file-bridge set (skipping forge/scan/init).
Discovery skips names ending in a `<goos>-<goarch>` pair — the dist set
installs the cross-compile artifacts next to the plain binaries, and an
unfiltered PATH listed `npm-linux-amd64` as a second bridge, double-ran every
sweep, and exec’d the suffixed dispatcher copy (`pf-bridge-linux-amd64 all`)
into itself forever. `--list` and the usage tail annotate each name through
the `--describe` probe (`internal/describe`): children print one line before
cobra runs (registry filenames + one direction suffix, or a `core.Describer`
line when Filename says nothing, or the cobra `Short` for the tool binaries),
and the dispatcher bolds names on a TTY (NO_COLOR honoured, 2 s shared probe
deadline). Adding a bridge = add the package + re-run `.scripts/gen-bridge-cmds.sh`
(idempotent). Escape hatch (§5): to collapse binaries later, drop the extra
mains — the `pf-bridge <name>` call sites never change.

The deep subsystem docs (bridge interfaces, sync/render algorithms, per-bridge
notes, how-to-add-a-bridge, forge/scanner/derive detail) currently live in
[../core/AGENTS.md](../core/AGENTS.md) and are being migrated here.

## Generated file headers

Every generated artefact opens with the same two-part header, composed in
`internal/bridge/core/reuse.go` and nowhere else:

- **Line-comment files** get `REUSEHeader(pf, StyleHash)`: copyright lines, a
  bare `#`, the licence line. That is the `.gitignore` family, `FUNDING.yml`,
  `.releaserc.yaml`, `.yamllint`, `.gitattributes`, `CODEOWNERS`, `CITATION.cff`.
  The bare separator is what `reuse annotate` writes for every line-comment
  dialect — without it a generated file and a hand-annotated source file
  disagree on their own header.
- **Markdown files** get `ManagedREUSEHeader(pf)`: ONE comment, no empty line
  between the tags, `pf-cli-managed: yes` folded in as its last line.

**Every generated file carries the sentinel, whatever its policy.** The line
warns a human not to edit what the next run overwrites regardless of the
`Marker` write gate, so `RenderLocalized` folds it into every file it writes
and no bridge decides. A template that emitted its own marker line is the bug
this replaced —
`SUPPORT.md` carried one and `CONTRIBUTING.md` did not, on identical policies.
`core.MarkerHTML` survives for `HasMarker` only: files generated before the
fold still read as managed.

**Synced (two-way) files keep the user’s header instead of stamping one.**
`pyproject.toml` is hand-edited as often as synced, so its writer splices the
re-rendered `[project]` zone into the original bytes — the header, foreign
tables and their comments survive byte for byte — and only GAP-FILLS the
REUSE header when the leading comment block carries no SPDX tags. A header
that is already there, hand-written or stale, always wins untouched: the
projectfile is not authoritative for the file’s rights-holder.

**A single documentation slot selects by tag, never by type.** An include can
union “a piece of documentation” onto every project (the shared spec site in
`m6e/core/conventions.yaml`), so `links[type=documentation]` alone cannot say
which URL is THE project’s docs. Consumers that own one documentation slot —
pyproject’s `[project.urls]` Documentation, composer’s `support.docs` — go
through `pfmodel.MainDocumentationURL`/`SetMainDocumentationURL`, which
select and stamp the `main-documentation` tag. Listing bridges (SUPPORT.md)
keep type-based selection: they list every piece of documentation on purpose.

## Localized community health files

`org.projectfile.i18n` is the **document-level** localization declaration. Two
keys:

- `languages` — the locale list (BCP 47 tags) honoured by every localizable
  renderer.
- `default-language` — the project’s primary language. A BCP 47 tag; defaults
  to `en`. The canonical root-level files are written in this language; every
  OTHER language renders under `docs/<lang>/`.

Localizable set: readme, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, SUPPORT, DEI,
plus every fragments-bridge document (FEATURES.md, ROADMAP.md, …). LICENSE is
deliberately excluded — a licence’s legal force lives in its canonical text.

**Localized fragments.** A fragment document’s unit of translation is the
fragment DIRECTORY, not a template: each declared language reads
`docs/<lang>/<name>.d/*.md` and assembles `docs/<lang>/<Out>`, keeping the
canonical file byte-stable for single-language projects (bar and terminology
wrap are no-ops without variants). The skip rule carries over — a language
with neither translated fragments nor language-specific inherited content
warns with the path to add and renders nothing under a localized name.
Structural strings (title, "Project …", "Inherited from …") localize from the
Go table in `internal/bridge/fragments/strings.go`, the same shape core’s
`footerStrings` uses — five keys do not justify a YAML catalog. Inherited
sections localize from the parent’s own `docs/<lang>/<Out>`, fetched at the
same ref as the canonical copy; a parent publishing no such language falls
back to its canonical copy in that variant — the child cannot translate text
it does not own. Offline, a variant whose committed document does not exist
nests the canonical sections under localized headings. Inherited-only
documents grow variants from the parent’s localized document alone. The
readme’s features block prefers the same-language document and falls back to
the canonical one with the localized `features.untranslated` note.

**Layout.** The locale lives in the directory, not the filename: the canonical
(default-language) file renders at the repository root, and each other
language renders under `docs/<lang>/` keeping the canonical basename:

```text
README.md              CONTRIBUTING.md              SECURITY.md          ← default (en)
docs/es/README.md      docs/es/CONTRIBUTING.md      docs/es/SECURITY.md
docs/uk/README.md      docs/uk/CONTRIBUTING.md      docs/uk/SECURITY.md
```

When `default-language` is not `en`, the English copies move under `docs/en/`
and the root files render in the declared default language. The default
language is never also a variant (it is the root file), so listing it in
`languages` is a no-op — it is dropped with a diagnostic.

The machinery is `internal/bridge/core/localize.go`: `LocalizedSpec` +
`RenderLocalized` own the naming rule (`LocalizedFilename` → `docs/<lang>/`),
the missing-translation policy, the cross-language bar and the final assembly,
so a bridge opts in by handing over a per-language view instead of calling
`Render` itself. Translated bodies come from sibling templates that keep the
infix form (`SUPPORT.es.md.tmpl`) — only OUTPUT relocated under `docs/<lang>/`,
template lookup did not — registered by walking each bridge’s embedded
`templates/` dir (`core.RegisterTemplateTree`). Adding a locale is one file,
never a `register.go` edit. `es` and `uk` ship embedded.

**textlint terminology vs non-English files.** The shared textlint config’s
`terminology` and `common-misspellings` rules are English-tuned and
false-positive on legitimate non-English text (e.g. Spanish `comando` is not
“commando”). The config enables `filters.comments`, so inline
`<!-- textlint-disable terminology,common-misspellings -->` …
`<!-- textlint-enable -->` directives suppress them — that is the one
supported mechanism, emitted by `core.WrapLocalizedTextlint` for every
non-English render and carried by hand-translated fragment sources
(`docs/<lang>/features.d/`), where `parseFragment` strips the pair as a
source pragma before assembly. Locale dirs never go into `.textlintignore`.

**Default-language re-anchoring.** The render loop’s `""` sentinel is the
canonical root render. `core.ResolveLang(lang, pf)` maps that sentinel to the
default language for STRING resolution (catalog messages, LocalizedString
fields), so a Spanish-first project’s root readme resolves Spanish. PATH
resolution (`LocalizedFilename`, `LocalizedSibling`) keeps the raw sentinel so
the default language renders at the root, not under `docs/<defLang>/`. Each
bridge’s `View(lang)` closure applies this split: string-resolving helpers
receive `ResolveLang(lang, pf)`, path-resolving helpers receive the raw `lang`.

Two rules that shape the code:

- **Prose belongs in templates, data belongs in Go.** Anything a translator
    must be able to reword lives in the `.tmpl`: the SUPPORT “Where to Ask”
    question column (Go emits a stable `Kind`, the template maps it to a
    localized question via a `{{define “want”}}` block), the SECURITY
    disclosure-window fallback, the SUPPORT response-time fallback. Go decides
    *which* rows exist and where they point; the template says what they mean.
- **A missing translation is skipped, never faked.** No default-language
    body ever ships under a localized name; the warning names the template to
    add, and the language is dropped from the cross-language bar of the files
    that did render.

Cross-references stay in-language (`core.LocalizedSibling` →
`docs/<lang>/SECURITY.md`). It cannot verify the sibling — the §5 binary split
hides other bridges’ templates from a given `pf-bridge-*` — so the declared
language set is the contract, and the sibling bridge warns by name about
anything it is missing.

The readme is the one exception to “a locale is a template”, and deliberately
so. Its two dozen blocks are three-line fragments, not prose: a per-language
copy of each would mean 18×N near-identical templates and every structural
change re-applied N times. So the readme localizes its **strings** from one
flat catalog per language (`internal/bridge/readme/messages/<lang>.yaml`,
reached from templates via the `t` function and from Go via `lookupMessage`)
and keeps one structural template per block. A block that needs a different
*shape* — not just different words — in some language still ships
`readme.md/<block>.<lang>.tmpl` (resolved via `LocalizedTemplateInfix`, which
keeps the infix form), which wins over the neutral template within its own
tier.

Two consequences worth knowing: the catalog is the home of every user-visible
label the readme derives (policies link text, link-group headings, link-type
names), keyed by a **derived** suffix (a link’s `type`, a health file’s
basename), so adding either is a catalog edit and not a Go edit; and a
`messages/<lang>.yaml` missing a key falls back to English per key rather than
dropping the file — the whole-file skip rule above governs the other five
documents, whose unit of translation is the document.

Scan/derive link labels localize too: `pfmodel.ComposeOnLabel` composes
`"{subject} {connector} {forge}"` per declared language (the connector and the
type noun — Issues/Packages/Source Code — live in `pfmodel/labels.go`, not the
readme catalog, because the catalog’s `link.type.*` values are the full
standalone names). The subject for source-code links is the project title;
`PromoteSourceCodeLabel` is the localized successor to the scan title-rewrite
(the scanner writes a “Source Code” noun placeholder, the rewrite swaps in the
effective title in every language at once). The git-remotes scanner emits
`links[type=bugs]` beside each source-code link as well — the same localized
“Issues on {forge}”, but final in the scanner since its subject is the type
noun, not the title, so no placeholder/rewrite is needed. `pfmodel.SetLinkLabel`
is the sole label writer, gap-fill like `SetLinkTags`. Load-bearing: a producer
MUST clear `Bare` when emitting a `Langs` map, or
`ExtractLocalizedStringForLang` returns `Bare` first and every language renders
the English default.

Remaining limit: labels the bridge humanizes from a filename (`docs/*.md` in
the documentation block) are the file’s own name, so they read the same in
every language. The `docs/<lang>/` locale directories are skipped by the docs
listing (they hold localized health files, surfaced through the language bar,
not generic documentation).

## Build

This is a full m6e project (`projectfile.yaml` + `.makefile/` submodules +
`Makefile`). Bridge consumes core as a **pinned module**
(`kiota.ch/projectfile/core` in `go.mod`) — no sibling `../core` checkout. The
module is private, so a cold fetch needs `GOPRIVATE=kiota.ch/*`, an SSH
`insteadOf` rewrite, and `GOPROXY=direct`.

```sh
make help           # categorised m6e target list
make build-binaries # → dist/pf-bridge + every dist/pf-bridge-* (host-native off-matrix)
make install-binary # build-binaries + copy the whole set into ~/.local/bin
make go-test        # host Go test suite
go build ./... && go test ./...   # plain host toolchain
```

`build-binaries.sh` builds **all** binaries (dispatcher + 17 bridges +
forge/scan/init); the dispatcher finds its siblings on PATH, so the m6e-only
`install-binary` tool (which self-builds via `build-binaries.sh`) drops the whole
set together. There is no hand-written `build-local`/`install-local` — those
retired with the manifest model. `.scripts/`: `build-binaries.sh` (matrix
cross-compile on the forge, host-native when the matrix binds no cell + a Git
version fallback + an unsuffixed `dist/<name>` host copy), `install-binary.sh`
(m6e-only local install), `gen-bridge-cmds.sh` (regenerate the per-binary mains
after adding a bridge), `forgejo-release.sh` / `gh-release.sh`. Version crosses
via `-ldflags -X …/internal/buildinfo.Version` (one target for every binary).

### Container lint/build (needs validation)

Host `go build`/`go test`/`go vet`/`gosec` all pass. The old blocker — a
`replace => ../core` path invisible to the m6e tool container — is gone now that
core is pinned. What a container `make build` / `make source-passes-lint` still
needs is the private-module fetch (`GOPRIVATE`/ssh/`GOPROXY=direct`) reachable
from inside the tool container; that path is not yet validated, and no
Dockerfile ships. CI-workflow generation is gated on the same validation.

## Deferred (Bridge Revolution)

- ~~The rename (`cli → core`, `pf-cli → projectfile`).~~ DONE — module
    `…/cli → …/core`, dir `cli/ → core/`, host/release binary `pf-cli → projectfile`. The container/fleet binary (`/usr/local/bin/pf-cli`, m6e
    `compile-go`) and the forge remote repository stay `cli` until §9.
- Multi-binary release wiring: `forgejo-release.sh`/`gh-release.sh` still upload the single `pf-bridge` asset; the split ships ~20 binaries per cell that a future §7 pass should attach (or bundle) per release.
- §9 release ordering: core is tagged (`v1.0.1`) and bridge pins it — the
    `replace` is gone. Remaining: validate the container build /
    `source-passes-lint` / CI-workflow generation against the pinned private module, and land bridge’s own remote repository + Dockerfile.
